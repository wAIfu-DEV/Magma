package checker

import (
	mt "Magma/src/types"
	"testing"
)

func absoluteType(name string) *mt.NodeType {
	return &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: name}}
}

func TestMemberPathPreservesActualPointerFieldType(t *testing.T) {
	innerType := absoluteType("test.Inner")
	innerPointer := &mt.NodeType{KindNode: &mt.NodeTypePointer{Kind: innerType.KindNode}}
	outerType := absoluteType("test.Outer")
	u64Type := &mt.NodeType{KindNode: &mt.NodeTypeNamed{NameNode: &mt.NodeNameSingle{Name: "u64"}}}

	global := &mt.NodeGlobal{StructDefs: map[string]*mt.StructDef{
		"Outer": {Module: "test", Name: "Outer", Fields: map[string]*mt.NodeType{"inner": innerPointer}, FieldNb: map[string]int{"inner": 0}},
		"Inner": {Module: "test", Name: "Inner", Fields: map[string]*mt.NodeType{"value": u64Type}, FieldNb: map[string]int{"value": 0}},
	}}
	context := &ctx{
		GlobalNode:   global,
		ModuleBundle: &mt.ModuleBundle{Main: global, Modules: map[string]*mt.NodeGlobal{"test": global}},
		FileCtx:      &mt.FileCtx{ModuleName: "test"},
	}
	source := &mt.NodeExprName{Name: &mt.NodeNameComposite{Parts: []string{"root", "inner", "value"}}}
	parsed := parseName(source.Name)
	_, accesses, err := clVarNameChainValid(context, nil, source, &parsed, "root", outerType, false)
	if err != nil {
		t.Fatalf("resolve member path: %v", err)
	}
	if len(accesses) != 2 {
		t.Fatalf("access count = %d, want 2", len(accesses))
	}
	if !sameType(accesses[0].OwnerType, outerType) || !sameType(accesses[0].Type, innerPointer) || accesses[0].PtrDeref {
		t.Fatal("first access did not preserve its owner and pointer-valued field types")
	}
	if !sameType(accesses[1].OwnerType, innerPointer) || !sameType(accesses[1].Type, u64Type) || !accesses[1].PtrDeref {
		t.Fatal("second access did not record the pointer transition")
	}

	variable := &mt.NodeExprVarDef{Name: &mt.NodeNameSingle{Name: "root"}, Type: outerType}
	name := &mt.NodeExprName{AssociatedNode: variable, MemberAccesses: accesses}
	result, err := memberPathResult(name)
	if err != nil {
		t.Fatalf("type member path: %v", err)
	}
	if !sameType(result, u64Type) {
		t.Fatal("member path result is not the final field's actual type")
	}
}
