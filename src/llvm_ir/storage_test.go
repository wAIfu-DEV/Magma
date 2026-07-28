package llvmir

import (
	mt "Magma/src/types"
	"testing"
)

func TestLocalSlotNamesAreBackendLocalAndStable(t *testing.T) {
	next := 7
	ctx := &IrCtx{moduleIdx: 3, nextSsa: &next, localSlots: map[*mt.NodeExprVarDef]SsaName{}}
	first := &mt.NodeExprVarDef{Storage: mt.VariableStorageLocal}
	second := &mt.NodeExprVarDef{Storage: mt.VariableStorageLocal}

	assignExprIrNames(ctx, first)
	assignExprIrNames(ctx, second)
	assignExprIrNames(ctx, first)

	if got := ctx.localSlots[first].Repr; got != "%.37" {
		t.Fatalf("first slot = %q, want %q", got, "%.37")
	}
	if got := ctx.localSlots[second].Repr; got != "%.38" {
		t.Fatalf("second slot = %q, want %q", got, "%.38")
	}
	if next != 9 {
		t.Fatalf("reassigning a reserved local consumed an SSA id: next = %d", next)
	}
}

func TestArgumentStorageKeepsNamedSlotConvention(t *testing.T) {
	i64 := &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "i64"}}
	variable := &mt.NodeExprVarDef{
		Name:    &mt.NodeNameSingle{Name: "value"},
		Type:    i64,
		Storage: mt.VariableStorageArgument,
	}
	name := &mt.NodeExprName{Name: variable.Name, AssociatedNode: variable, Storage: mt.VariableStorageArgument}

	slot, _, err := irNameVariableStorage(&IrCtx{}, name)
	if err != nil {
		t.Fatal(err)
	}
	if slot.Repr != "%value.addr" {
		t.Fatalf("argument slot = %q, want %%value.addr", slot.Repr)
	}
}
