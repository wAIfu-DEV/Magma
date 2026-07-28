package join

import (
	"Magma/src/comp_err"
	"Magma/src/types"
	"testing"
)

func completed(err error) <-chan error {
	ch := make(chan error, 1)
	ch <- err
	return ch
}

func TestJoinCompilationUnitsOrdersImportedErrorsByPath(t *testing.T) {
	state := &types.SharedState{ImportedFiles: map[string]<-chan error{
		"z.mg": completed(comp_err.CompilationErrorToken(&types.FileCtx{FilePath: "z.mg"}, &types.Token{}, "z", "")),
		"a.mg": completed(comp_err.CompilationErrorToken(&types.FileCtx{FilePath: "a.mg"}, &types.Token{}, "a", "")),
	}}
	diagnostics := comp_err.Diagnostics(JoinCompilationUnits(state, nil))
	if len(diagnostics) != 2 || diagnostics[0].Message != "a" || diagnostics[1].Message != "z" {
		t.Fatalf("unexpected diagnostic order: %#v", diagnostics)
	}
}
