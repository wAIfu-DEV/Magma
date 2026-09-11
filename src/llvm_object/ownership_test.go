//go:build llvm_object

package llvmobject

import (
	mt "Magma/src/types"
	"errors"
	"strings"
	"testing"
)

func TestTypeConstructorValidation(t *testing.T) {
	if _, err := NewModule(""); err == nil {
		t.Fatal("empty module name was accepted")
	}
	m, err := NewModule("validation")
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	for _, width := range []int{-1, 0, 1 << 23} {
		if _, err := m.IntType(width); err == nil {
			t.Errorf("integer width %d was accepted", width)
		}
	}
	for _, addressSpace := range []int{-1, int(uint64(^uint32(0)) + 1)} {
		if _, err := m.PointerType(addressSpace); err == nil {
			t.Errorf("address space %d was accepted", addressSpace)
		}
	}
}

func TestRejectsCrossModuleHandles(t *testing.T) {
	left, _ := NewModule("left")
	right, _ := NewModule("right")
	defer left.Close()
	defer right.Close()
	leftI64, _ := left.IntType(64)
	rightI64, _ := right.IntType(64)
	if _, err := left.NewFunction("wrong", rightI64); err == nil || !strings.Contains(err.Error(), "different LLVM module") {
		t.Fatalf("cross-module result error = %v", err)
	}
	fn, _ := left.NewFunction("leftFn", leftI64, leftI64)
	if _, err := right.NewBlock(fn, "wrong"); err == nil || !strings.Contains(err.Error(), "different module") {
		t.Fatalf("cross-module function error = %v", err)
	}
	block, _ := left.NewBlock(fn, "entry")
	builder, _ := left.BuilderAt(block)
	foreignFn, _ := right.NewFunction("rightFn", rightI64, rightI64)
	foreign, _ := foreignFn.Param(0)
	if err := builder.Ret(foreign); err == nil || !strings.Contains(err.Error(), "different LLVM module") {
		t.Fatalf("cross-module value error = %v", err)
	}
}

func TestClosedModuleRejectsOperationsAndDoubleClose(t *testing.T) {
	m, _ := NewModule("closed")
	i64, _ := m.IntType(64)
	fn, _ := m.NewFunction("f", i64, i64)
	block, _ := m.NewBlock(fn, "entry")
	builder, _ := m.BuilderAt(block)
	value, _ := fn.Param(0)
	m.Close()
	m.Close()
	if _, err := m.IntType(32); err == nil {
		t.Fatal("type construction after close succeeded")
	}
	if _, err := fn.Param(0); err == nil {
		t.Fatal("parameter access after close succeeded")
	}
	if err := builder.Ret(value); err == nil {
		t.Fatal("instruction construction after close succeeded")
	}
	if _, err := m.String(); err == nil {
		t.Fatal("module printing after close succeeded")
	}
}

func TestRejectsInvalidHandlesBeforeLLVM(t *testing.T) {
	m, _ := NewModule("invalid_handles")
	defer m.Close()
	i64, _ := m.IntType(64)
	if _, err := m.NewFunction("bad", Type{}); err == nil {
		t.Fatal("zero result type was accepted")
	}
	fn, _ := m.NewFunction("f", i64)
	block, _ := m.NewBlock(fn, "entry")
	builder, _ := m.BuilderAt(block)
	if err := builder.Ret(Value{}); err == nil {
		t.Fatal("zero return value was accepted")
	}
}

func TestVerificationFailureIsReturned(t *testing.T) {
	m, _ := NewModule("invalid_ir")
	defer m.Close()
	void, _ := m.VoidType()
	fn, _ := m.NewFunction("unterminated", void)
	if _, err := m.NewBlock(fn, "entry"); err != nil {
		t.Fatal(err)
	}
	token := mt.Token{Repr: "ret", Pos: mt.FilePos{Line: 7, Col: 3}}
	err := m.VerifyAt(ErrorContext{Token: &token, Function: "broken", Block: "entry", Operation: "finalize function"})
	if err == nil {
		t.Fatal("unterminated block passed verification")
	}
	var backend *BackendError
	if !errors.As(err, &backend) {
		t.Fatalf("verification error has type %T, want *BackendError", err)
	}
	if backend.Token != &token || backend.Function != "broken" || backend.Block != "entry" || backend.Operation != "finalize function" || backend.LLVMDiagnostic == "" {
		t.Fatalf("verification context was not preserved: %#v", backend)
	}
}

func TestDirectiveErrorCarriesSourceAndBuilderContext(t *testing.T) {
	m, _ := NewModule("diagnostic")
	defer m.Close()
	void, _ := m.VoidType()
	fn, _ := m.NewFunction("worker", void)
	block, _ := m.NewBlock(fn, "entry")
	builder, _ := m.BuilderAt(block)
	expr := &mt.NodeExprLlvm{Tk: mt.Token{Repr: "@", Pos: mt.FilePos{Line: 4, Col: 9}}, Operation: "missing", ResultType: &mt.NodeType{}}
	_, err := builder.LowerLLVMExpr(expr, nil, void)
	var backend *BackendError
	if !errors.As(err, &backend) {
		t.Fatalf("directive error has type %T, want *BackendError", err)
	}
	if backend.Token != &expr.Tk || backend.Function != "worker" || backend.Block != "entry" || backend.Operation != "lower @llvm(missing)" {
		t.Fatalf("directive context was not preserved: %#v", backend)
	}
}
