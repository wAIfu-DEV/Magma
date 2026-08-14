package place

import (
	"Magma/src/types"
	"testing"
)

func fixture() (*types.NodeExprVarDef, *types.StructDef, *types.MemberAccess) {
	root := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "value"}}
	owner := &types.StructDef{Name: "Pair"}
	access := &types.MemberAccess{OwnerDef: owner, FieldNb: 0, Type: &types.NodeType{}}
	return root, owner, access
}

func rootName(root *types.NodeExprVarDef) *types.NodeExprName {
	return &types.NodeExprName{AssociatedNode: root}
}

func TestCanonicalFieldEqualityIgnoresSpelling(t *testing.T) {
	root, owner, _ := fixture()
	left, err := FromExpr(&types.NodeExprMemberAccess{Target: rootName(root), Member: "left", Access: &types.MemberAccess{OwnerDef: owner, FieldNb: 1, Type: &types.NodeType{}}})
	if err != nil {
		t.Fatal(err)
	}
	right, err := FromExpr(&types.NodeExprName{AssociatedNode: root, MemberAccesses: []*types.MemberAccess{{OwnerDef: owner, FieldNb: 1, Type: &types.NodeType{}}}})
	if err != nil {
		t.Fatal(err)
	}
	if !left.Equal(right) {
		t.Fatal("resolved forms of the same field are not equal")
	}
}

func TestPrefixAndWholePlaceOverlap(t *testing.T) {
	root, _, access := fixture()
	whole, _ := FromExpr(rootName(root))
	field, _ := FromExpr(&types.NodeExprMemberAccess{Target: rootName(root), Access: access})
	if !whole.IsPrefixOf(field) || !whole.Overlaps(field) {
		t.Fatal("whole place must prefix and overlap its field")
	}
	if field.IsPrefixOf(whole) {
		t.Fatal("field must not prefix its root")
	}
}

func TestDistinctFieldsAndConstantIndicesAreDisjoint(t *testing.T) {
	root, owner, _ := fixture()
	field := func(index int) Place {
		p, _ := FromExpr(&types.NodeExprMemberAccess{Target: rootName(root), Access: &types.MemberAccess{OwnerDef: owner, FieldNb: index, Type: &types.NodeType{}}})
		return p
	}
	index := func(value string) Place {
		p, _ := FromExpr(&types.NodeExprSubscript{Target: rootName(root), Expr: &types.NodeExprLit{LitType: types.TokLitNum, Value: value}})
		return p
	}
	if field(0).Overlaps(field(1)) {
		t.Fatal("distinct fields overlap")
	}
	if index("2").Overlaps(index("3")) {
		t.Fatal("distinct constant indices overlap")
	}
	if !index("2").Equal(index("0x2")) {
		t.Fatal("equivalent numeric indices are not canonical")
	}
}

func TestDynamicIndexOverlapsEveryIndex(t *testing.T) {
	root, _, _ := fixture()
	firstExpr, secondExpr := rootName(root), rootName(root)
	dynamic, _ := FromExpr(&types.NodeExprSubscript{Target: rootName(root), Expr: firstExpr})
	otherDynamic, _ := FromExpr(&types.NodeExprSubscript{Target: rootName(root), Expr: secondExpr})
	constant, _ := FromExpr(&types.NodeExprSubscript{Target: rootName(root), Expr: &types.NodeExprLit{LitType: types.TokLitNum, Value: "9"}})
	if !dynamic.Overlaps(constant) {
		t.Fatal("dynamic index must overlap constant index")
	}
	if dynamic.Equal(otherDynamic) || !dynamic.Overlaps(otherDynamic) {
		t.Fatal("distinct dynamic indices must be unequal but conservatively overlap")
	}
}

func TestDereferenceIsRepresented(t *testing.T) {
	root, _, _ := fixture()
	deref, err := FromExpr(&types.NodeExprUnary{Operator: types.KwAsterisk, Operand: rootName(root)})
	if err != nil {
		t.Fatal(err)
	}
	if len(deref.Projections) != 1 || deref.Projections[0].Kind != Dereference {
		t.Fatalf("projections = %+v", deref.Projections)
	}
}

func TestDereferencesThroughDistinctRootsConservativelyOverlap(t *testing.T) {
	leftRoot, _, _ := fixture()
	rightRoot := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "other"}}
	left, _ := FromExpr(&types.NodeExprUnary{Operator: types.KwAsterisk, Operand: rootName(leftRoot)})
	right, _ := FromExpr(&types.NodeExprUnary{Operator: types.KwAsterisk, Operand: rootName(rightRoot)})
	if !left.Overlaps(right) {
		t.Fatal("dereferences with unresolved provenance must conservatively overlap")
	}
	if !left.Overlaps(Place{Root: rightRoot}) {
		t.Fatal("a dereference with unresolved provenance may overlap a direct place")
	}
	if (Place{Root: leftRoot}).Overlaps(Place{Root: rightRoot}) {
		t.Fatal("distinct direct declaration roots must remain disjoint")
	}
}

func TestLinkedPointerTransitionAddsDereference(t *testing.T) {
	root, owner, _ := fixture()
	resolved, err := FromExpr(&types.NodeExprName{
		AssociatedNode: root,
		MemberAccesses: []*types.MemberAccess{{
			OwnerDef: owner,
			FieldNb:  0,
			Type:     &types.NodeType{},
			PtrDeref: true,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved.Projections) != 2 || resolved.Projections[0].Kind != Dereference || resolved.Projections[1].Kind != Field {
		t.Fatalf("projections = %+v", resolved.Projections)
	}
}

func TestConstantDeclarationIndexIsCanonical(t *testing.T) {
	root, _, _ := fixture()
	constant := &types.NodeExprVarDef{IsConst: true, Initializer: &types.NodeExprLit{LitType: types.TokLitNum, Value: "0x10"}}
	byName, err := FromExpr(&types.NodeExprSubscript{Target: rootName(root), Expr: &types.NodeExprName{AssociatedNode: constant}})
	if err != nil {
		t.Fatal(err)
	}
	byLiteral, _ := FromExpr(&types.NodeExprSubscript{Target: rootName(root), Expr: &types.NodeExprLit{LitType: types.TokLitNum, Value: "16"}})
	if !byName.Equal(byLiteral) {
		t.Fatal("constant declaration and equivalent literal index are not canonical")
	}
}

func TestFailuresCarrySourceToken(t *testing.T) {
	token := types.Token{Pos: types.FilePos{Line: 7, Col: 4}}
	_, err := FromExpr(&types.NodeExprName{Tk: token})
	buildErr, ok := err.(*BuildError)
	if !ok || buildErr.Token.Pos.Line != 7 {
		t.Fatalf("error = %#v", err)
	}
}
