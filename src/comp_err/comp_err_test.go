package comp_err

import (
	"Magma/src/types"
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestDiagnosticsPreserveOrderAndInheritStage(t *testing.T) {
	ctxA := &types.FileCtx{FilePath: "a.mg"}
	ctxB := &types.FileCtx{FilePath: "b.mg"}
	err := AtStage("linking", Join(
		CompilationErrorToken(ctxA, &types.Token{}, "first", ""),
		CompilationErrorToken(ctxB, &types.Token{}, "second", ""),
	))
	diagnostics := Diagnostics(err)
	if len(diagnostics) != 2 {
		t.Fatalf("got %d diagnostics", len(diagnostics))
	}
	if diagnostics[0].Message != "first" || diagnostics[1].Message != "second" {
		t.Fatalf("diagnostics reordered: %#v", diagnostics)
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Stage != "linking" {
			t.Fatalf("stage = %q", diagnostic.Stage)
		}
	}
}

func TestDiagnosticsFindWrappedSourceError(t *testing.T) {
	ctx := &types.FileCtx{FilePath: "wrapped.mg"}
	err := fmt.Errorf("outer context: %w", CompilationErrorToken(ctx, &types.Token{}, "source failure", ""))
	diagnostics := Diagnostics(err)
	if len(diagnostics) != 1 || diagnostics[0].Message != "source failure" {
		t.Fatalf("wrapped diagnostic was lost: %#v", diagnostics)
	}
}

func TestFprintHandlesSourceAndInternalFailures(t *testing.T) {
	ctx := &types.FileCtx{FilePath: "main.mg", Content: []byte("bad\n")}
	err := Join(
		AtStage("parsing", CompilationErrorToken(ctx, &types.Token{Pos: types.FilePos{Line: 1, Col: 1}}, "invalid token", "fix it")),
		AtStage("linking", errors.New("broken invariant")),
	)
	var output bytes.Buffer
	if !Fprint(&output, err) {
		t.Fatal("error was not rendered")
	}
	text := output.String()
	for _, expected := range []string{"error [parsing]: invalid token", "1| bad", "fix it", "fatal error [linking]: broken invariant"} {
		if !strings.Contains(text, expected) {
			t.Errorf("output missing %q:\n%s", expected, text)
		}
	}
}

func TestEnsureDiagnosticAddsSourceAndPreservesExistingDiagnostic(t *testing.T) {
	ctx := &types.FileCtx{FilePath: "main.mg", Content: []byte("broken\n")}
	token := &types.Token{Pos: types.FilePos{Line: 1, Col: 3}}
	wrapped := EnsureDiagnostic(ctx, token, errors.New("opaque failure"))
	diagnostics := Diagnostics(wrapped)
	if len(diagnostics) != 1 || diagnostics[0].FilePath != "main.mg" || diagnostics[0].Token.Pos != token.Pos {
		t.Fatalf("source diagnostic = %#v", diagnostics)
	}

	existing := CompilationErrorToken(ctx, token, "specific failure", "specific help")
	if got := EnsureDiagnostic(ctx, &types.Token{}, existing); got != existing {
		t.Fatal("existing source diagnostic was replaced")
	}
}
