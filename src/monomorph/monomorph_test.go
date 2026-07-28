package monomorph

import (
	"reflect"
	"testing"

	t "Magma/src/types"
)

func TestCanonicalTypeSignatureAndMangleNestedTypes(test *testing.T) {
	stringType := &t.NodeType{KindNode: &t.NodeTypeAbsolute{AbsoluteName: "std.String"}}
	nested := &t.NodeType{KindNode: &t.NodeTypeNamed{
		NameNode: &t.NodeNameSingle{Name: "Box"},
		GenericArgs: []*t.NodeType{{KindNode: &t.NodeTypePointer{
			Kind: (&t.NodeTypeSlice{ElemKind: stringType.KindNode}),
		}}},
	}}

	const expected = "N_Box__G__P__S__A_std__String"
	if got := CanonicalTypeSignature(nested); got != expected {
		test.Fatalf("canonical signature = %q, want %q", got, expected)
	}
	if got := MangleSpecializedName("wrap", []*t.NodeType{nested}); got != "wrap__g__"+expected {
		test.Fatalf("specialized name = %q", got)
	}
}

func TestCloneStructDefPreservesResolvedSymbol(test *testing.T) {
	original := &t.NodeStructDef{AbsName: "app.Box"}
	if got := cloneStructDef(original).AbsName; got != original.AbsName {
		test.Fatalf("cloned struct symbol = %q, want %q", got, original.AbsName)
	}
}

func TestSubstituteTypeRewritesNestedFunctionTypeWithoutMutatingTemplate(test *testing.T) {
	typeParameter := func() *t.NodeType {
		return &t.NodeType{KindNode: &t.NodeTypeNamed{NameNode: &t.NodeNameSingle{Name: "T"}}}
	}
	template := &t.NodeType{Throws: true, KindNode: &t.NodeTypeFunc{
		Args:    []*t.NodeType{{KindNode: &t.NodeTypeSlice{ElemKind: typeParameter().KindNode}}},
		RetType: &t.NodeType{KindNode: &t.NodeTypePointer{Kind: typeParameter().KindNode}},
	}}
	concrete := &t.NodeType{KindNode: &t.NodeTypeAbsolute{AbsoluteName: "app.Widget"}}

	result := substituteType(template, map[string]*t.NodeType{"T": concrete})
	if got := CanonicalTypeSignature(result); got != "F__S__A_app__Widget__RET__P__A_app__Widget" {
		test.Fatalf("substituted signature = %q", got)
	}
	if !result.Throws {
		test.Fatal("substitution discarded the throwing qualifier")
	}
	if reflect.DeepEqual(template, result) {
		test.Fatal("substitution unexpectedly retained the template structure")
	}
	if got := CanonicalTypeSignature(template); got != "F__S__N_T__RET__P__N_T" {
		test.Fatalf("template was mutated: %q", got)
	}
}

func TestPruneTemplatesRemovesGenericMemberFromStructDef(test *testing.T) {
	generic := &t.NodeFuncDef{Class: t.NodeGenericClass{TypeParams: []string{"T"}}}
	concrete := &t.NodeFuncDef{}
	owner := &t.StructDef{
		Funcs: map[string]*t.NodeFuncDef{
			"generic":  generic,
			"concrete": concrete,
		},
	}
	gl := &t.NodeGlobal{
		Declarations: []t.NodeGlobalDecl{generic, concrete},
		StructDefs:   map[string]*t.StructDef{"Owner": owner},
		FuncDefs: map[string]*t.NodeFuncDef{
			"Owner.generic":  generic,
			"Owner.concrete": concrete,
		},
	}

	ctx := &monoCtx{modules: map[string]*t.NodeGlobal{"module": gl}}
	ctx.pruneTemplates()

	if _, ok := owner.Funcs["generic"]; ok {
		test.Fatal("generic member template remained attached to its struct")
	}
	if owner.Funcs["concrete"] != concrete {
		test.Fatal("concrete member was incorrectly pruned")
	}
	if _, ok := gl.FuncDefs["Owner.generic"]; ok {
		test.Fatal("generic member template remained in the module function map")
	}
	if len(gl.Declarations) != 1 || gl.Declarations[0] != concrete {
		test.Fatal("module declarations were not pruned consistently")
	}
}

func TestSubstituteTypePreservesPositionOwnership(test *testing.T) {
	typeParameter := &t.NodeType{
		Owned: true,
		KindNode: &t.NodeTypeNamed{
			NameNode: &t.NodeNameSingle{Name: "T"},
		},
	}
	concrete := &t.NodeType{
		KindNode: &t.NodeTypeAbsolute{AbsoluteName: "test.Resource"},
	}

	result := substituteType(typeParameter, map[string]*t.NodeType{"T": concrete})

	if !result.Owned {
		test.Fatal("generic substitution discarded the $ ownership qualifier")
	}
}

func TestRegisterGenericMemberTemplate(test *testing.T) {
	member := &t.NodeFuncDef{Class: t.NodeGenericClass{
		NameNode:   &t.NodeNameComposite{Parts: []string{"Allocator", "allocT"}},
		TypeParams: []string{"T"},
	}}
	ctx := &monoCtx{
		funcTemplates:   map[string]*t.NodeFuncDef{},
		memberTemplates: map[string]*t.NodeFuncDef{},
	}

	ctx.registerFuncTemplate("allocator", "Allocator.allocT", member)

	key := makeMemberTemplateKey("allocator", "Allocator", "allocT")
	if ctx.memberTemplates[key] != member {
		test.Fatalf("generic member template was not registered as %q", key)
	}
	if len(ctx.funcTemplates) != 0 {
		test.Fatal("generic member template was incorrectly registered as a free function")
	}
}
