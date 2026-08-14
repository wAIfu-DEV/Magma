package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"Magma/src/types"
)

func TestSafetyKeywordCompletion(t *testing.T) {
	a := &analysis{file: &types.FileCtx{PackageName: "main"}, docs: &docIndex{expressionSymbols: map[string]map[string]completionItem{}}}
	for _, prefix := range []string{"mo", "bou", "uns"} {
		items := a.expressionCompletions(prefix, 1)
		if len(items) != 1 || !strings.HasPrefix(items[0].Label, prefix) || items[0].Kind != 14 {
			t.Fatalf("completion for %q = %#v", prefix, items)
		}
	}
}

func TestSafetySemanticTokensExcludeMoveFunctionCall(t *testing.T) {
	source := "mod main\nmain(value $str) void:\n    unsafe:\n        bounded 0 < 1:\n            consume(move value)\n            move(\"call\")\n        ..\n    ..\n..\n"
	uri := "file:///tmp/safety_tokens.mg"
	var output bytes.Buffer
	s := &server{in: bufio.NewReader(nil), out: &output, documents: map[string]*document{uri: {URI: uri, Text: source}}}
	params, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": uri}})
	if err := s.handleSemanticTokens(message{ID: json.RawMessage("1"), Params: params}); err != nil {
		t.Fatal(err)
	}
	// unsafe, bounded, and the ownership-transfer move produce three tokens;
	// the ordinary move(...) call does not.
	if got := strings.Count(output.String(), ",0,0"); got != 3 {
		t.Fatalf("semantic token response = %s, token endings = %d", output.String(), got)
	}
}

func TestSafetyCodeActions(t *testing.T) {
	move := diagnostic{Code: "missing-move", Range: rangePosition{Start: position{Line: 2, Character: 12}, End: position{Line: 2, Character: 17}}}
	uri := "file:///tmp/actions.mg"
	var output bytes.Buffer
	s := &server{out: &output, documents: map[string]*document{uri: {URI: uri, Text: "mod main\nmain(values u8[], i u64) void:\n    values[i] = 0\n..\n"}}}
	params, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": uri}, "context": map[string]any{"diagnostics": []diagnostic{move, {Code: "unproven-bounds", Range: rangePosition{Start: position{Line: 2, Character: 10}}}}}})
	if err := s.handleCodeAction(message{ID: json.RawMessage("1"), Params: params}); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	if !strings.Contains(got, "Insert `move`") || !strings.Contains(got, "Wrap statement in `bounded` proof") || strings.Contains(got, "unsafe:") {
		t.Fatalf("code actions = %s", got)
	}
}

func TestWarningPolicyChangesOnlySafetySeverityForUnsavedBuffer(t *testing.T) {
	// The disk contains a different document; analysis must use the unsaved
	// source override supplied by the editor.
	directory := t.TempDir()
	path := filepath.Join(directory, "policy.mg")
	if err := os.WriteFile(path, []byte("mod main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
	source := "mod main\nResource(value str)\ndestr Resource.close() void:\n..\nmakeResource() $Resource:\n    ret Resource(value=\"x\")\n..\nconsume(value $Resource) void:\n    value.close()\n..\nmain() void:\n    value $Resource = makeResource()\n    consume(value)\n..\n"
	fatal := analyzePolicy(uri, source, testStdRoot(), false)
	warn := analyzePolicy(uri, source, testStdRoot(), true)
	if fatal.err == nil || warn.err != nil {
		t.Fatalf("fatal err=%v warning err=%v", fatal.err, warn.err)
	}
	fd := diagnosticsForFile(fatal.err, fatal.warnings, path)
	wd := diagnosticsForFile(warn.err, warn.warnings, path)
	if len(fd) != len(wd) || len(fd) == 0 {
		t.Fatalf("fatal=%#v warning=%#v", fd, wd)
	}
	if fd[0].Code != wd[0].Code || fd[0].Range != wd[0].Range || fd[0].Message != wd[0].Message || fd[0].Severity != 1 || wd[0].Severity != 2 {
		t.Fatalf("policy changed more than severity: fatal=%#v warning=%#v", fd[0], wd[0])
	}
}
