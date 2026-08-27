package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"Magma/src/types"
)

func TestInferredTypeHintsIncludeSimpleAndDestructuredBindings(t *testing.T) {
	source := "mod main\nmain() !void:\n    café := makeText()\n    value, failure := load()\n    explicit u64 = 1\n..\n"
	docs := &docIndex{completionBindings: []completionBinding{
		{module: "main", name: "café", valueType: namedHintType("str"), declarationLine: 3},
		{module: "main", name: "value", valueType: namedHintType("u64"), declarationLine: 4},
		{module: "main", name: "failure", valueType: namedHintType("error"), declarationLine: 4},
		// Refreshing semantic bindings may add the same source declaration again.
		{module: "main", name: "value", valueType: namedHintType("u64"), declarationLine: 4},
	}}
	hints := inferredTypeHints(source, "main", docs, rangePosition{Start: position{}, End: position{Line: 10, Character: 100}})
	if len(hints) != 3 {
		t.Fatalf("hints = %#v", hints)
	}
	want := []struct {
		line, character uint32
		label           string
	}{{2, 8, "str"}, {3, 9, "u64"}, {3, 18, "error"}}
	for i, expected := range want {
		if hints[i].Position.Line != expected.line || hints[i].Position.Character != expected.character || hints[i].Label != expected.label || hints[i].Kind != 1 || !hints[i].PaddingLeft {
			t.Errorf("hint %d = %#v, want line=%d character=%d label=%q", i, hints[i], expected.line, expected.character, expected.label)
		}
	}
}

func TestInferredTypeHintsRespectRequestedRange(t *testing.T) {
	docs := &docIndex{completionBindings: []completionBinding{
		{module: "main", name: "first", valueType: namedHintType("u64"), declarationLine: 1},
		{module: "main", name: "second", valueType: namedHintType("str"), declarationLine: 2},
	}}
	hints := inferredTypeHints("first := 1\nsecond := text()\n", "main", docs, rangePosition{Start: position{Line: 1}, End: position{Line: 1, Character: 100}})
	if len(hints) != 1 || hints[0].Label != "str" {
		t.Fatalf("range hints = %#v", hints)
	}
}

func TestInlayHintCollapsesCompilerBindingPhases(t *testing.T) {
	precheck := &types.NodeType{KindNode: &types.NodeTypeNamed{NameNode: &types.NodeNameSingle{Name: "Value"}}, Throws: true, Owned: true}
	resolved := &types.NodeType{KindNode: &types.NodeTypeAbsolute{AbsoluteName: "main_id.Value", DisplayName: "Value"}, Owned: true}
	docs := &docIndex{completionBindings: []completionBinding{
		{module: "main", name: "value", valueType: precheck, declarationLine: 1},
		{module: "main", name: "value", valueType: resolved, declarationLine: 1},
	}}
	hints := inferredTypeHints("value := try makeValue()\n", "main", docs, rangePosition{End: position{Line: 1, Character: 100}})
	if len(hints) != 1 || hints[0].Label != "$Value" {
		t.Fatalf("phase-collapsed hints = %#v", hints)
	}
}

func TestFormatAbsoluteTypeDoesNotDuplicateModifiers(t *testing.T) {
	valueType := &types.NodeType{KindNode: &types.NodeTypeAbsolute{AbsoluteName: "main_id.Value", DisplayName: "Value"}, Throws: true, Owned: true}
	if got, want := formatType(valueType), "!owned Value"; got != want {
		t.Fatalf("formatType() = %q, want %q", got, want)
	}
}

func TestPointerInlayUsesPostfixMagmaSyntax(t *testing.T) {
	valueType := &types.NodeType{
		KindNode: &types.NodeTypePointer{Kind: &types.NodeTypeNamed{NameNode: &types.NodeNameSingle{Name: "Value"}}},
		Owned:    true,
	}
	if got, want := formatInlayType(valueType), "$Value*"; got != want {
		t.Fatalf("formatInlayType() = %q, want %q", got, want)
	}
}

func TestInitializeAdvertisesInlayHints(t *testing.T) {
	var output bytes.Buffer
	s := &server{in: bufio.NewReader(nil), out: &output, documents: map[string]*document{}}
	if err := s.handle(message{JSONRPC: "2.0", ID: json.RawMessage("1"), Method: "initialize", Params: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"inlayHintProvider":true`) {
		t.Fatalf("initialize response = %s", output.String())
	}
}

func TestAnalyzedInferenceProducesInlayHint(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "main.mg")
	source := "mod main\nmakeValue() u64:\n    ret 1\n..\nmain() void:\n    value := makeValue()\n..\n"
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	a := analyze(fileURI(path), source, testStdRoot())
	if a.file == nil || a.docs == nil {
		t.Fatalf("analysis = %#v", a)
	}
	hints := inferredTypeHints(source, a.file.PackageName, a.docs, rangePosition{End: position{Line: 20, Character: 100}})
	if len(hints) != 1 || hints[0].Label != "u64" || hints[0].Position.Line != 5 {
		t.Fatalf("analyzed hints = %#v", hints)
	}
}

func namedHintType(name string) *types.NodeType {
	return &types.NodeType{KindNode: &types.NodeTypeNamed{NameNode: &types.NodeNameSingle{Name: name}}}
}
