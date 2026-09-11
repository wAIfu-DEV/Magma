//go:build llvm_object

package loweringprogram_test

import (
	"bytes"
	"strings"
	"testing"

	llvmobject "Magma/src/llvm_object"
	mt "Magma/src/types"
)

func TestBuildAssemblesFunctionsAndRuntimeDeterministically(t *testing.T) {
	u64 := &mt.NodeType{KindNode: &mt.NodeTypeNamed{NameNode: &mt.NodeNameSingle{Name: "u64"}}}
	function := &mt.NodeFuncDef{
		AbsName: "app.answer", ReturnType: u64, ContextABI: mt.ContextABIContextless,
		Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtRet{
			Expression: &mt.NodeExprLit{Value: "42", LitType: mt.TokLitNum, InfType: u64}, OwnerFuncType: u64,
		}}},
	}
	state := &mt.SharedState{Files: map[string]*mt.FileCtx{
		"app.mg": {GlNode: &mt.NodeGlobal{Declarations: []mt.NodeGlobalDecl{function}}},
	}, CoreTypes: make(map[mt.CoreTypeRole]*mt.StructDef)}
	ir, err := llvmobject.ProgramIR("program.mg", state)
	if err != nil {
		t.Fatal(err)
	}
	text := string(ir)
	if !strings.Contains(text, "define internal i64 @app.answer") || !strings.Contains(text, "define internal %type.slice @magma.argsToSlice") {
		t.Fatalf("whole-program module is incomplete:\n%s", text)
	}
}

func TestProgramObjectEmitsNativeObjectBytes(t *testing.T) {
	u64 := &mt.NodeType{KindNode: &mt.NodeTypeNamed{NameNode: &mt.NodeNameSingle{Name: "u64"}}}
	function := &mt.NodeFuncDef{
		AbsName: "app.value", ReturnType: u64, ContextABI: mt.ContextABIContextless,
		Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtRet{
			Expression: &mt.NodeExprLit{Value: "7", LitType: mt.TokLitNum, InfType: u64}, OwnerFuncType: u64,
		}}},
	}
	state := &mt.SharedState{Files: map[string]*mt.FileCtx{
		"app.mg": {GlNode: &mt.NodeGlobal{Declarations: []mt.NodeGlobalDecl{function}}},
	}, CoreTypes: make(map[mt.CoreTypeRole]*mt.StructDef)}
	object, err := llvmobject.ProgramObject("program-object.mg", state, llvmobject.TargetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(object) < 4 {
		t.Fatalf("object emission returned only %d bytes", len(object))
	}
	if !bytes.Equal(object[:4], []byte{0x7f, 'E', 'L', 'F'}) {
		t.Fatalf("host object lacks ELF header: % x", object[:4])
	}
}
