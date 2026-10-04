//go:build llvm_object

package loweringtypes_test

import (
	"testing"

	llvmobject "Magma/src/llvm_object"
	lb "Magma/src/lowering_backend"
	loweringtypes "Magma/src/lowering_types"
	mt "Magma/src/types"
)

func TestUnionUsesLargestPayloadAndStrongestAlignment(t *testing.T) {
	backend, err := llvmobject.NewLoweringBackend("union.mg")
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	if err := backend.ConfigureModule(lb.ModuleSpec{SourceFile: "union.mg", TargetTriple: "x86_64-unknown-linux-gnu", DataLayout: "e-p:64:64-i64:64-i128:128"}); err != nil {
		t.Fatal(err)
	}
	state, _ := semanticState()
	small := &mt.StructDef{Module: "app", Name: "__union_Choice_Small", FieldOrder: []string{"value"}, Fields: map[string]*mt.NodeType{"value": primitive("u8")}}
	large := &mt.StructDef{Module: "app", Name: "__union_Choice_Large", FieldOrder: []string{"left", "right"}, Fields: map[string]*mt.NodeType{"left": primitive("u64"), "right": primitive("u64")}}
	wide := &mt.StructDef{Module: "app", Name: "__union_Choice_Wide", FieldOrder: []string{"value"}, Fields: map[string]*mt.NodeType{"value": primitive("u128")}}
	long := &mt.StructDef{Module: "app", Name: "__union_Choice_Long", FieldOrder: []string{"a", "b", "c"}, Fields: map[string]*mt.NodeType{"a": primitive("u64"), "b": primitive("u64"), "c": primitive("u64")}}
	choice := &mt.StructDef{Module: "app", Name: "Choice", FieldOrder: []string{"__tag", "__Small", "__Large", "__Wide", "__Long"}, Fields: map[string]*mt.NodeType{
		"__tag":   primitive("u64"),
		"__Small": {KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "app.__union_Choice_Small"}},
		"__Large": {KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "app.__union_Choice_Large"}},
		"__Wide":  {KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "app.__union_Choice_Wide"}},
		"__Long":  {KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "app.__union_Choice_Long"}},
	}}
	global := state.Files["record.mg"].GlNode
	global.StructDefs[small.Name] = small
	global.StructDefs[large.Name] = large
	global.StructDefs[wide.Name] = wide
	global.StructDefs[long.Name] = long
	global.StructDefs[choice.Name] = choice
	global.UnionDefs = map[string]*mt.UnionDef{"Choice": {Module: "app", Name: "Choice", Variants: []*mt.UnionVariant{{Name: "Small"}, {Name: "Large"}, {Name: "Wide"}, {Name: "Long"}}}}
	lowerer, err := loweringtypes.New(backend, state)
	if err != nil {
		t.Fatal(err)
	}
	id, err := lowerer.Lower(&mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "app.Choice"}})
	if err != nil {
		t.Fatal(err)
	}
	layout, err := backend.TypeLayout(id)
	if err != nil {
		t.Fatal(err)
	}
	if layout.AllocationSize != 48 || layout.ABIAlignment != 16 {
		t.Fatalf("union layout = %#v, want 48 bytes aligned to 16", layout)
	}
	offset, err := backend.StructFieldOffset(id, 2)
	if err != nil {
		t.Fatal(err)
	}
	if offset != 16 {
		t.Fatalf("payload offset = %d, want 16", offset)
	}
}
