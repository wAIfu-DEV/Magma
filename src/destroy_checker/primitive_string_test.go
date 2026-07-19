package destroychecker

import (
	"Magma/src/checker"
	"Magma/src/join"
	"Magma/src/monomorph"
	"Magma/src/pipeline"
	"Magma/src/shared"
	"Magma/src/types"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func checkSource(t *testing.T, source string) []Diagnostic {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.mg")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	state, err := shared.MakeShared(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = pipeline.DoMain(state, path); err != nil {
		t.Fatal(err)
	}
	if err = join.JoinCompilationUnits(state, nil); err != nil {
		t.Fatal(err)
	}
	if err = monomorph.Run(state); err != nil {
		t.Fatal(err)
	}
	if err = checker.CheckLinks(state); err != nil {
		t.Fatal(err)
	}
	if err = checker.TypeChecker(state); err != nil {
		t.Fatal(err)
	}
	return diagnosticsForFile(Check(state), path)
}

func diagnosticsForFile(all []Diagnostic, path string) []Diagnostic {
	out := []Diagnostic{}
	for _, diagnostic := range all {
		if diagnostic.FilePath == path {
			out = append(out, diagnostic)
		}
	}
	return out
}

func TestOwnedPrimitiveStringCanBeDestroyedByMethod(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test source")
	}
	allocatorPath := filepath.Clean(filepath.Join(filepath.Dir(testFile), "..", "..", "std", "allocator.mg"))
	diagnostics := checkSource(t, `mod main
use "`+filepath.ToSlash(allocatorPath)+`" alc

release(value $str, allocator alc.Allocator) void:
    value.free(allocator)
..
`)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v, want none", diagnostics)
	}
}

func TestOwnedPrimitiveStringMustBeConsumed(t *testing.T) {
	diagnostics := checkSource(t, `mod main

leak(value $str) void:
..
`)
	if len(diagnostics) != 1 || !strings.Contains(diagnostics[0].Message, "not consumed") {
		t.Fatalf("diagnostics = %+v, want unconsumed owned string warning", diagnostics)
	}
}

func TestBorrowedStringLiteralNeedsNoDestruction(t *testing.T) {
	diagnostics := checkSource(t, `mod main

borrowed() void:
    value str = "literal"
..
`)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v, want no obligation for borrowed string literal", diagnostics)
	}
}

func TestBorrowedPrimitiveStringCannotBeDestroyed(t *testing.T) {
	// Build this case directly because the allocator type is irrelevant to the
	// ownership rule and the call analyser only needs destructor metadata.
	a, _ := fixture()
	stringType := &types.NodeType{KindNode: &types.NodeTypeNamed{NameNode: &types.NodeNameSingle{Name: "str"}}}
	borrowed := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "value"}, Type: stringType}
	a.shared.Files["core.mg"] = &types.FileCtx{GlNode: &types.NodeGlobal{
		PrimitiveDestructors: map[string][]*types.NodeFuncDef{"str": {{IsDestructor: true}}},
	}}
	out := flow{states: map[*types.NodeExprVarDef]State{}, deferred: map[*types.NodeExprVarDef]bool{}}
	a.consume(&out, borrowed, "destructor call")
	if len(a.diagnostics) != 1 || !strings.Contains(a.diagnostics[0].Message, "borrowed destructible value") {
		t.Fatalf("diagnostics = %+v, want borrowed string warning", a.diagnostics)
	}
}
