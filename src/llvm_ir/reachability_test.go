package llvmir

import (
	"testing"

	t "Magma/src/types"
)

func TestReachableFunctionsFollowsCallsAndPrunesUnusedDefinitions(test *testing.T) {
	used := &t.NodeFuncDef{AbsName: "main.used"}
	unused := &t.NodeFuncDef{AbsName: "main.unused"}
	entry := &t.NodeFuncDef{
		AbsName:      "main.main",
		IsEntryPoint: true,
		Body: t.NodeBody{Statements: []t.NodeStatement{
			&t.NodeStmtExpr{Expression: &t.NodeExprCall{
				Callee:          &t.NodeExprName{AssociatedNode: used},
				AssociatedFnDef: used,
			}},
		}},
	}
	global := &t.NodeGlobal{Declarations: []t.NodeGlobalDecl{entry, used, unused}}
	files := map[string]*t.FileCtx{
		"main.mg": {
			ModuleName:   "main",
			PackageName:  "main",
			MainPckgName: "main",
			GlNode:       global,
		},
	}

	reachable, _ := reachableFunctions(files, false)
	if !reachable[entry] || !reachable[used] {
		test.Fatalf("reachable functions = %#v, want entry and its direct callee", reachable)
	}
	if reachable[unused] {
		test.Fatal("unused function was retained")
	}
}

func TestReachableFunctionsPreservesRootlessIrWriteInputs(test *testing.T) {
	function := &t.NodeFuncDef{AbsName: "library.function"}
	files := map[string]*t.FileCtx{
		"library.mg": {GlNode: &t.NodeGlobal{Declarations: []t.NodeGlobalDecl{function}}},
	}
	reachable, _ := reachableFunctions(files, false)
	if !reachable[function] {
		test.Fatal("rootless library function was pruned")
	}
}
