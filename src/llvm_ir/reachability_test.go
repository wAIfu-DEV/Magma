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

	reachable, _ := reachableFunctions(files, false, true)
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
	reachable, _ := reachableFunctions(files, false, true)
	if !reachable[function] {
		test.Fatal("rootless library function was pruned")
	}
}

func TestObjectReachabilityNeverScansLegacyLLVMText(test *testing.T) {
	legacyOnly := &t.NodeFuncDef{AbsName: "main.legacyOnly"}
	entry := &t.NodeFuncDef{AbsName: "main.main", IsEntryPoint: true}
	files := map[string]*t.FileCtx{
		"main.mg": {
			ModuleName: "main", PackageName: "main", MainPckgName: "main",
			GlNode: &t.NodeGlobal{Declarations: []t.NodeGlobalDecl{entry, legacyOnly, &t.NodeLlvm{Text: "call void @main.legacyOnly()"}}},
		},
	}
	textual, _ := reachableFunctions(files, false, true)
	object, _ := reachableFunctions(files, false, false)
	if !textual[legacyOnly] {
		test.Fatal("textual reachability stopped honoring its legacy LLVM compatibility input")
	}
	if object[legacyOnly] {
		test.Fatal("object reachability scanned an opaque LLVM text symbol")
	}
}
