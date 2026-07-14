package monomorph

import (
	"testing"

	t "Magma/src/types"
)

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
