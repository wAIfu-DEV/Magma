package checker

import (
	"Magma/src/comp_err"
	mt "Magma/src/types"
	"errors"
	"strings"
	"testing"
)

func compositeName(parts ...string) *mt.NodeNameComposite {
	tokens := make([]mt.Token, len(parts))
	for i, part := range parts {
		tokens[i] = mt.Token{Repr: part, Pos: mt.FilePos{Line: 1, Col: uint32(i + 1)}}
	}
	return &mt.NodeNameComposite{Parts: parts, Tokens: tokens}
}

func requireLookupDiagnostic(t *testing.T, err error, token, message string) {
	t.Helper()
	var diagnostic *comp_err.CompilationError
	if !errors.As(err, &diagnostic) {
		t.Fatalf("expected structured lookup diagnostic, got %T: %v", err, err)
	}
	if diagnostic.Token.Repr != token {
		t.Fatalf("diagnostic token = %q, want %q", diagnostic.Token.Repr, token)
	}
	if !strings.Contains(diagnostic.Message, message) {
		t.Fatalf("diagnostic = %q, want it to contain %q", diagnostic.Message, message)
	}
}

func TestFunctionLookupReportsUnknownAliasAtAliasToken(t *testing.T) {
	c := &ctx{
		FileCtx:      &mt.FileCtx{FilePath: "test.mg"},
		GlobalNode:   &mt.NodeGlobal{ImportAlias: map[string]string{}, FuncDefs: map[string]*mt.NodeFuncDef{}},
		ModuleBundle: &mt.ModuleBundle{Modules: map[string]*mt.NodeGlobal{}},
	}

	_, err := clGetFuncDefFromName(c, compositeName("missing", "run"))
	requireLookupDiagnostic(t, err, "missing", "unknown module alias 'missing'")
}

func TestFunctionLookupReportsMissingImportedFunctionAtSymbolToken(t *testing.T) {
	c := &ctx{
		FileCtx: &mt.FileCtx{FilePath: "test.mg"},
		GlobalNode: &mt.NodeGlobal{
			ImportAlias: map[string]string{"dep": "dependency"},
			FuncDefs:    map[string]*mt.NodeFuncDef{},
		},
		ModuleBundle: &mt.ModuleBundle{Modules: map[string]*mt.NodeGlobal{
			"dependency": {FuncDefs: map[string]*mt.NodeFuncDef{}},
		}},
	}

	_, err := clGetFuncDefFromName(c, compositeName("dep", "missing"))
	requireLookupDiagnostic(t, err, "missing", "unknown function 'dep.missing'")
}
