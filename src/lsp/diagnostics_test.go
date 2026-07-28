package lsp

import (
	"Magma/src/comp_err"
	"Magma/src/types"
	"bufio"
	"bytes"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestDiagnosticsForFileConvertsCompilerLocations(t *testing.T) {
	ctx := &types.FileCtx{FilePath: `C:\project\main.mg`}
	err := comp_err.AtStage("type checking", comp_err.Join(
		comp_err.CompilationErrorToken(ctx, &types.Token{Pos: types.FilePos{Line: 3, Col: 5}, Repr: "name"}, "unknown name", "declare it first"),
		comp_err.CompilationErrorToken(&types.FileCtx{FilePath: `C:\project\other.mg`}, &types.Token{}, "other", ""),
	))
	diagnostics := diagnosticsForFile(err, nil, ctx.FilePath)
	if len(diagnostics) != 1 {
		t.Fatalf("got %d diagnostics", len(diagnostics))
	}
	got := diagnostics[0]
	if got.Range.Start.Line != 2 || got.Range.Start.Character != 4 || got.Range.End.Character != 8 {
		t.Fatalf("unexpected range: %#v", got.Range)
	}
	if got.Severity != 1 || got.Source != "magma" || got.Message != "unknown name: declare it first" {
		t.Fatalf("unexpected diagnostic: %#v", got)
	}
}

func TestDiagnosticsForFileIncludesWarnings(t *testing.T) {
	path := `C:\project\main.mg`
	warnings := []types.Diagnostic{{
		Severity: types.SeverityWarning, FilePath: path,
		Token:   types.Token{Pos: types.FilePos{Line: 2, Col: 3}, Repr: "value"},
		Message: "narrowing conversion",
	}}
	diagnostics := diagnosticsForFile(nil, warnings, path)
	if len(diagnostics) != 1 || diagnostics[0].Severity != 2 || diagnostics[0].Message != "narrowing conversion" {
		t.Fatalf("unexpected warning diagnostic: %#v", diagnostics)
	}
}

func TestDidOpenPublishesCompilerDiagnostics(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "broken.mg")
	source := "mod broken\nrun() void:\n    missing()\n..\n"
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
	params, err := json.Marshal(map[string]any{"textDocument": map[string]any{
		"uri": uri, "text": source, "version": 7,
	}})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	s := &server{in: bufio.NewReader(nil), out: &output, stdRoot: testStdRoot(), documents: map[string]*document{}}
	if err := s.handle(message{Method: "textDocument/didOpen", Params: params}); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	if !bytes.Contains(output.Bytes(), []byte(`"method":"textDocument/publishDiagnostics"`)) ||
		!bytes.Contains(output.Bytes(), []byte(`"severity":1`)) ||
		!bytes.Contains(output.Bytes(), []byte(`"version":7`)) ||
		!bytes.Contains(output.Bytes(), []byte("unknown function")) {
		t.Fatalf("unexpected LSP output: %s", got)
	}
}
