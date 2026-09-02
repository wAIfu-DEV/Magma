package lsp

import (
	"Magma/src/types"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

type inlayHint struct {
	Position     position       `json:"position"`
	Label        string         `json:"label"`
	Kind         int            `json:"kind,omitempty"`
	Tooltip      map[string]any `json:"tooltip,omitempty"`
	PaddingLeft  bool           `json:"paddingLeft,omitempty"`
	PaddingRight bool           `json:"paddingRight,omitempty"`
}

func (s *server) handleInlayHints(msg message) error {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Range rangePosition `json:"range"`
	}
	if err := json.Unmarshal(msg.Params, &p); err != nil {
		return err
	}
	d := s.documents[p.TextDocument.URI]
	if d == nil {
		return s.respond(msg.ID, []inlayHint{})
	}
	if d.result == nil {
		d.result = analyzePolicy(d.URI, d.Text, s.stdRoot, s.safetyWarnings)
	}
	if d.result == nil || d.result.file == nil || d.result.docs == nil {
		return s.respond(msg.ID, []inlayHint{})
	}
	hints := inferredTypeHints(d.Text, d.result.file.PackageName, d.result.docs, p.Range)
	return s.respond(msg.ID, hints)
}

var inferredBindingPattern = regexp.MustCompile(`([\p{L}_][\p{L}\p{N}_]*)\s*(?:,\s*([\p{L}_][\p{L}\p{N}_]*))?\s*:=`)

func inferredTypeHints(source, module string, docs *docIndex, requested rangePosition) []inlayHint {
	if docs == nil {
		return []inlayHint{}
	}
	lines := strings.Split(source, "\n")
	byPosition := map[string]inlayHint{}
	for _, binding := range docs.completionBindings {
		if binding.module != module || binding.valueType == nil || binding.declarationLine == 0 {
			continue
		}
		lineNumber := binding.declarationLine - 1
		if lineNumber < requested.Start.Line || lineNumber > requested.End.Line || int(lineNumber) >= len(lines) {
			continue
		}
		line := strings.TrimSuffix(lines[lineNumber], "\r")
		match := inferredBindingPattern.FindStringSubmatchIndex(line)
		if len(match) == 0 {
			continue
		}
		start, end, ok := inferredBindingSpan(match, line, binding.name)
		if !ok {
			continue
		}
		character := uint32(utf8.RuneCountInString(line[:end]))
		if lineNumber == requested.Start.Line && character < requested.Start.Character {
			continue
		}
		if lineNumber == requested.End.Line && character > requested.End.Character {
			continue
		}
		typeName := formatInlayType(binding.valueType)
		if typeName == "" {
			continue
		}
		// Pre-specialization and post-checker indexes can both describe the same
		// source binding. Key by source identity, not type spelling, so the later
		// (more semantically resolved) binding replaces the earlier one.
		key := fmt.Sprintf("%d:%d:%s", lineNumber, start, binding.name)
		byPosition[key] = inlayHint{
			Position:    position{Line: lineNumber, Character: character},
			Label:       typeName,
			Kind:        1, // InlayHintKind.Type
			Tooltip:     markdownContent("Inferred type: `" + typeName + "`"),
			PaddingLeft: true,
		}
	}
	result := make([]inlayHint, 0, len(byPosition))
	for _, hint := range byPosition {
		result = append(result, hint)
	}
	sortInlayHints(result)
	return result
}

func formatInlayType(valueType *types.NodeType) string {
	if valueType == nil {
		return ""
	}
	// A local binding contains the successful value, never the throwing call
	// envelope used to infer it. Preserve ownership in Magma source syntax.
	copy := *valueType
	copy.Throws = false
	value := formatType(&copy)
	return value
}

func inferredBindingSpan(match []int, line, name string) (int, int, bool) {
	for group := 1; group <= 2; group++ {
		startIndex := group * 2
		if startIndex+1 >= len(match) || match[startIndex] < 0 {
			continue
		}
		start, end := match[startIndex], match[startIndex+1]
		if line[start:end] == name {
			return start, end, true
		}
	}
	return 0, 0, false
}

func sortInlayHints(hints []inlayHint) {
	sort.Slice(hints, func(i, j int) bool {
		if hints[i].Position.Line != hints[j].Position.Line {
			return hints[i].Position.Line < hints[j].Position.Line
		}
		return hints[i].Position.Character < hints[j].Position.Character
	})
}
