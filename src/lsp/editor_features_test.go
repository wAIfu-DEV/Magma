package lsp

import (
	"Magma/src/types"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"
)

func TestCallContextAtTracksNestedArguments(t *testing.T) {
	source := "mod main\nmain() void:\n    output := make(1, nested(2, 3), val\n..\n"
	line := "    output := make(1, nested(2, 3), val"
	name, active, ok := callContextAt(source, position{Line: 2, Character: uint32(utf8.RuneCountInString(line))})
	if !ok || name != "make" || active != 2 {
		t.Fatalf("call context = %q, %d, %v", name, active, ok)
	}
}

func TestSignatureFunctionFindsImportedFunction(t *testing.T) {
	dir := t.TempDir()
	dep := filepath.Join(dir, "dep.mg")
	if err := os.WriteFile(dep, []byte("mod dep\npub open(path str, mode u64) !u64:\n    ret 0\n..\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(dir, "main.mg")
	source := "mod main\nuse \"dep\" dep\nmain() !void:\n    value := try dep.open(\"x\", 0)\n..\n"
	if err := os.WriteFile(mainPath, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	a := analyze(fileURI(mainPath), source, testStdRoot())
	fn, _ := a.signatureFunction("dep.open")
	if fn == nil || len(fn.Class.ArgsNode.Args) != 2 || fn.Class.ArgsNode.Args[1].Name != "mode" {
		t.Fatalf("signature function = %#v", fn)
	}
}

func TestTopLevelAndExpressionKeywordCompletion(t *testing.T) {
	items := topLevelKeywordCompletions("pr")
	if len(items) == 0 || items[0].Label != "proto" {
		t.Fatalf("top-level completions = %#v", items)
	}
	a := &analysis{file: &types.FileCtx{PackageName: "main"}, docs: &docIndex{expressionSymbols: map[string]map[string]completionItem{}, modules: map[string]string{}}}
	labels := map[string]bool{}
	for _, item := range a.expressionCompletions("t", 1) {
		labels[item.Label] = true
	}
	if !labels["try"] || !labels["throw"] {
		t.Fatalf("expression keyword labels = %#v", labels)
	}
}

func TestExpectedTypeRanksMatchingCompletionFirst(t *testing.T) {
	a := &analysis{file: &types.FileCtx{PackageName: "main"}, docs: &docIndex{
		expressionSymbols: map[string]map[string]completionItem{"main": {
			"number": {Label: "number", Detail: "number u64"},
			"text":   {Label: "text", Detail: "text str"},
		}},
		modules: map[string]string{},
	}}
	items := a.expressionCompletions("", 1, "str")
	if len(items) == 0 || items[0].Label != "text" {
		t.Fatalf("ranked completions = %#v", items)
	}
}
