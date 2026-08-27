package lsp

import (
	"Magma/src/types"
	"encoding/json"
	"strings"
	"unicode"
)

type signatureHelp struct {
	Signatures      []signatureInformation `json:"signatures"`
	ActiveSignature uint32                 `json:"activeSignature"`
	ActiveParameter uint32                 `json:"activeParameter"`
}

type signatureInformation struct {
	Label         string                 `json:"label"`
	Documentation map[string]any         `json:"documentation,omitempty"`
	Parameters    []parameterInformation `json:"parameters,omitempty"`
}

type parameterInformation struct {
	Label         string         `json:"label"`
	Documentation map[string]any `json:"documentation,omitempty"`
}

func (s *server) handleSignatureHelp(msg message) error {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Position position `json:"position"`
	}
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		return err
	}
	d := s.documents[p.TextDocument.URI]
	if d == nil {
		return s.respond(msg.ID, nil)
	}
	name, active, ok := callContextAt(d.Text, p.Position)
	if !ok {
		return s.respond(msg.ID, nil)
	}
	if d.result == nil {
		d.result = analyze(d.URI, d.Text, s.stdRoot)
	}
	fn, docs := d.result.signatureFunction(name)
	if fn == nil {
		return s.respond(msg.ID, nil)
	}
	parameters := make([]parameterInformation, 0, len(fn.Class.ArgsNode.Args))
	for _, arg := range fn.Class.ArgsNode.Args {
		parameters = append(parameters, parameterInformation{Label: arg.Name + " " + formatType(arg.TypeNode)})
	}
	if len(parameters) != 0 && int(active) >= len(parameters) {
		active = uint32(len(parameters) - 1)
	}
	return s.respond(msg.ID, signatureHelp{Signatures: []signatureInformation{{Label: formatFunction(fn), Documentation: markdownContent(docs), Parameters: parameters}}, ActiveParameter: active})
}

func (a *analysis) signatureFunction(sourceName string) (*types.NodeFuncDef, string) {
	if a == nil || a.file == nil || a.docs == nil {
		return nil, ""
	}
	parts := strings.Split(sourceName, ".")
	keys := []string{}
	if len(parts) == 1 {
		keys = append(keys, a.file.PackageName+"\x00"+parts[0])
	} else if module := a.importedPackage(parts[0]); module != "" {
		keys = append(keys, module+"\x00"+strings.Join(parts[1:], "."))
	}
	if len(parts) > 1 {
		suffix := "\x00" + parts[len(parts)-1]
		for key := range a.docs.functionDefs {
			if strings.HasSuffix(key, suffix) || strings.HasSuffix(key, "."+parts[len(parts)-1]) {
				keys = append(keys, key)
			}
		}
	}
	for _, key := range keys {
		if fn := a.docs.functionDefs[key]; fn != nil {
			return fn, a.docs.hoverSymbols[key]
		}
	}
	return nil, ""
}

func callContextAt(source string, pos position) (string, uint32, bool) {
	offset, ok := sourceOffset(source, pos)
	if !ok {
		return "", 0, false
	}
	depth, active, inString, escaped := 0, uint32(0), false, false
	for i := offset - 1; i >= 0; i-- {
		c := source[i]
		if inString {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			continue
		}
		switch c {
		case ')':
			depth++
		case '(':
			if depth > 0 {
				depth--
				continue
			}
			end := i
			start := end
			for start > 0 {
				r := rune(source[start-1])
				if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.') {
					break
				}
				start--
			}
			name := strings.Trim(source[start:end], ".")
			return name, active, name != ""
		case ',':
			if depth == 0 {
				active++
			}
		case '\n':
			if depth == 0 {
				return "", 0, false
			}
		}
	}
	return "", 0, false
}

func sourceOffset(source string, pos position) (int, bool) {
	lines := strings.SplitAfter(source, "\n")
	if int(pos.Line) >= len(lines) {
		return 0, false
	}
	offset := 0
	for i := 0; i < int(pos.Line); i++ {
		offset += len(lines[i])
	}
	runes := []rune(strings.TrimSuffix(strings.TrimSuffix(lines[pos.Line], "\n"), "\r"))
	if int(pos.Character) > len(runes) {
		return 0, false
	}
	return offset + len(string(runes[:pos.Character])), true
}
