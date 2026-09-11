//go:build llvm_object

package llvmobject

import (
	mt "Magma/src/types"
	"bytes"
	"testing"
)

func buildPtrToInt(t *testing.T) *Module {
	t.Helper()
	m, err := NewModule("magma.object.poc")
	if err != nil {
		t.Fatal(err)
	}
	ptr, err := m.PointerType(0)
	if err != nil {
		t.Fatal(err)
	}
	i64, err := m.IntType(64)
	if err != nil {
		t.Fatal(err)
	}
	fn, err := m.NewFunction("ptrToInt", i64, ptr)
	if err != nil {
		m.Close()
		t.Fatal(err)
	}
	entry, err := m.NewBlock(fn, "entry")
	if err != nil {
		m.Close()
		t.Fatal(err)
	}
	builder, err := m.BuilderAt(entry)
	if err != nil {
		m.Close()
		t.Fatal(err)
	}
	param, err := fn.Param(0)
	if err != nil {
		m.Close()
		t.Fatal(err)
	}
	result, err := builder.PtrToInt(param, i64, "result")
	if err != nil {
		m.Close()
		t.Fatal(err)
	}
	if err := builder.Ret(result); err != nil {
		m.Close()
		t.Fatal(err)
	}
	return m
}

func TestPtrToIntBuildsAndVerifies(t *testing.T) {
	m := buildPtrToInt(t)
	defer m.Close()
	if err := m.Verify(); err != nil {
		t.Fatal(err)
	}
	ir, err := m.String()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains([]byte(ir), []byte("ptrtoint ptr %0 to i64")) {
		t.Fatalf("missing ptrtoint instruction:\n%s", ir)
	}
}

func TestPtrToIntEmitsObjectBytes(t *testing.T) {
	m := buildPtrToInt(t)
	defer m.Close()
	object, err := m.EmitObject(TargetOptions{Optimization: OptimizationNone})
	if err != nil {
		t.Fatal(err)
	}
	if len(object) < 16 {
		t.Fatalf("object output is unexpectedly small: %d bytes", len(object))
	}
}

func TestSharedLLVMExpressionLowersToObject(t *testing.T) {
	m, err := NewModule("magma.object.directive")
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	ptr, _ := m.PointerType(0)
	i64, _ := m.IntType(64)
	fn, _ := m.NewFunction("ptrToInt", i64, ptr)
	entry, _ := m.NewBlock(fn, "entry")
	builder, _ := m.BuilderAt(entry)
	operand, _ := fn.Param(0)
	expr := &mt.NodeExprLlvm{Operation: "ptrtoint", Args: []mt.NodeExpr{&mt.NodeExprName{}}, ResultType: &mt.NodeType{}}
	result, err := builder.LowerLLVMExpr(expr, []Value{operand}, i64)
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.Ret(result); err != nil {
		t.Fatal(err)
	}
	if err := m.Verify(); err != nil {
		t.Fatal(err)
	}
}
