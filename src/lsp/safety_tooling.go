package lsp

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode/utf8"
)

type semanticTokens struct {
	Data []uint32 `json:"data"`
}

// handleSemanticTokens highlights memory-safety syntax. `move` is contextual,
// so an ordinary function call named move is deliberately not classified as a
// keyword.
func (s *server) handleSemanticTokens(msg message) error {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		return err
	}
	d := s.documents[p.TextDocument.URI]
	if d == nil {
		return s.respond(msg.ID, semanticTokens{Data: []uint32{}})
	}
	words := regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*|\S`).FindAllStringIndex(d.Text, -1)
	line, lineByte, lastLine, lastChar := uint32(0), 0, uint32(0), uint32(0)
	data := []uint32{}
	for i, span := range words {
		for lineByte < span[0] {
			n := strings.IndexByte(d.Text[lineByte:span[0]], '\n')
			if n < 0 {
				break
			}
			line++
			lineByte += n + 1
		}
		word := d.Text[span[0]:span[1]]
		keyword := word == "bounded" || word == "unsafe"
		if word == "move" {
			next := ""
			if i+1 < len(words) {
				next = d.Text[words[i+1][0]:words[i+1][1]]
			}
			keyword = next != "(" && next != "." && next != "=" && next != ":="
		}
		if !keyword {
			continue
		}
		character := uint32(utf8.RuneCountInString(d.Text[lineByte:span[0]]))
		deltaLine, deltaChar := line-lastLine, character
		if deltaLine == 0 {
			deltaChar = character - lastChar
		}
		data = append(data, deltaLine, deltaChar, uint32(utf8.RuneCountInString(word)), 0, 0)
		lastLine, lastChar = line, character
	}
	return s.respond(msg.ID, semanticTokens{Data: data})
}

type codeAction struct {
	Title       string        `json:"title"`
	Kind        string        `json:"kind"`
	Diagnostics []diagnostic  `json:"diagnostics,omitempty"`
	Edit        workspaceEdit `json:"edit"`
}
type workspaceEdit struct {
	Changes map[string][]textEdit `json:"changes"`
}
type textEdit struct {
	Range   rangePosition `json:"range"`
	NewText string        `json:"newText"`
}

func (s *server) handleCodeAction(msg message) error {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Context struct {
			Diagnostics []diagnostic `json:"diagnostics"`
		} `json:"context"`
	}
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		return err
	}
	d := s.documents[p.TextDocument.URI]
	actions := []codeAction{}
	for _, diag := range p.Context.Diagnostics {
		switch diag.Code {
		case "missing-move":
			actions = append(actions, codeAction{Title: "Insert `move`", Kind: "quickfix", Diagnostics: []diagnostic{diag}, Edit: workspaceEdit{Changes: map[string][]textEdit{p.TextDocument.URI: {{Range: rangePosition{Start: diag.Range.Start, End: diag.Range.Start}, NewText: "move "}}}}})
		case "unproven-bounds":
			if d != nil {
				if edit, ok := boundedLineEdit(d.Text, diag.Range.Start.Line); ok {
					actions = append(actions, codeAction{Title: "Wrap statement in `bounded` proof", Kind: "quickfix", Diagnostics: []diagnostic{diag}, Edit: workspaceEdit{Changes: map[string][]textEdit{p.TextDocument.URI: {edit}}}})
				}
			}
		}
	}
	return s.respond(msg.ID, actions)
}

var obviousSubscript = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_\.]*)\[([A-Za-z_][A-Za-z0-9_]*)\]`)

func boundedLineEdit(source string, lineNumber uint32) (textEdit, bool) {
	lines := strings.SplitAfter(source, "\n")
	if int(lineNumber) >= len(lines) {
		return textEdit{}, false
	}
	raw := strings.TrimSuffix(strings.TrimSuffix(lines[lineNumber], "\n"), "\r")
	match := obviousSubscript.FindStringSubmatch(raw)
	if len(match) != 3 {
		return textEdit{}, false
	}
	indent := raw[:len(raw)-len(strings.TrimLeft(raw, " \t"))]
	body := strings.TrimSpace(raw)
	replacement := indent + "bounded " + match[2] + " < " + match[1] + ".count():\n" + indent + "    " + body + "\n" + indent + "..\n"
	return textEdit{Range: rangePosition{Start: position{Line: lineNumber}, End: position{Line: lineNumber + 1}}, NewText: replacement}, true
}
