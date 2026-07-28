package loweringvalidate

import (
	"Magma/src/comp_err"
	mt "Magma/src/types"
	"errors"
	"testing"
)

func stateWithExpression(expression mt.NodeExpr) *mt.SharedState {
	file := &mt.FileCtx{FilePath: "test.mg", GlNode: &mt.NodeGlobal{Declarations: []mt.NodeGlobalDecl{
		&mt.NodeFuncDef{
			ReturnType: &mt.NodeType{KindNode: &mt.NodeTypeNamed{NameNode: &mt.NodeNameSingle{Name: "void"}}},
			Body:       mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtExpr{Expression: expression}}},
		},
	}}}
	return &mt.SharedState{Files: map[string]*mt.FileCtx{"test.mg": file}}
}

func stateWithStatements(statements ...mt.NodeStatement) *mt.SharedState {
	voidType := &mt.NodeType{KindNode: &mt.NodeTypeNamed{NameNode: &mt.NodeNameSingle{Name: "void"}}}
	file := &mt.FileCtx{FilePath: "test.mg", GlNode: &mt.NodeGlobal{Declarations: []mt.NodeGlobalDecl{
		&mt.NodeFuncDef{ReturnType: voidType, Body: mt.NodeBody{Statements: statements}},
	}}}
	return &mt.SharedState{Files: map[string]*mt.FileCtx{"test.mg": file}}
}

func requireStatementStageDiagnostic(test *testing.T, statement mt.NodeStatement) {
	test.Helper()
	err := Validate(stateWithStatements(statement))
	var diagnostic *comp_err.CompilationError
	if !errors.As(err, &diagnostic) {
		test.Fatalf("expected structured stage diagnostic, got %T: %v", err, err)
	}
}

func TestRejectsIncompleteNestedType(t *testing.T) {
	file := &mt.FileCtx{FilePath: "test.mg", GlNode: &mt.NodeGlobal{Declarations: []mt.NodeGlobalDecl{
		&mt.NodeExprVarDef{Type: &mt.NodeType{KindNode: &mt.NodeTypePointer{}}},
	}}}
	err := Validate(&mt.SharedState{Files: map[string]*mt.FileCtx{"test.mg": file}})
	var diagnostic *comp_err.CompilationError
	if !errors.As(err, &diagnostic) {
		t.Fatalf("expected structured stage diagnostic, got %T: %v", err, err)
	}
}

func TestValidatesResolvedMemberFunctionMetadata(t *testing.T) {
	voidType := &mt.NodeType{KindNode: &mt.NodeTypeNamed{NameNode: &mt.NodeNameSingle{Name: "void"}}}
	ownerType := &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "test.Owner"}}
	member := &mt.NodeFuncDef{
		Class: mt.NodeGenericClass{
			NameNode: &mt.NodeNameComposite{Parts: []string{"Owner", "value"}},
			ArgsNode: mt.NodeArgList{Args: []mt.NodeArg{{
				Name:     "this",
				TypeNode: &mt.NodeType{KindNode: &mt.NodeTypePointer{Kind: ownerType.KindNode}},
			}}},
		},
		ReturnType: voidType,
		IsMember:   true,
	}
	file := &mt.FileCtx{FilePath: "test.mg", GlNode: &mt.NodeGlobal{Declarations: []mt.NodeGlobalDecl{member}}}
	state := &mt.SharedState{Files: map[string]*mt.FileCtx{"test.mg": file}}
	if err := Validate(state); err != nil {
		t.Fatalf("resolved member metadata rejected: %v", err)
	}

	member.IsMember = false
	if err := Validate(state); err == nil {
		t.Fatal("expected inconsistent member metadata to be rejected")
	}
}

func TestValidatesResolvedStructSymbol(t *testing.T) {
	declaration := &mt.NodeStructDef{
		Class:   mt.NodeGenericClass{NameNode: &mt.NodeNameSingle{Name: "Item"}},
		AbsName: "test.Item",
	}
	global := &mt.NodeGlobal{
		Declarations: []mt.NodeGlobalDecl{declaration},
		StructDefs: map[string]*mt.StructDef{
			"Item": {Module: "test", Name: "Item"},
		},
	}
	state := &mt.SharedState{Files: map[string]*mt.FileCtx{
		"test.mg": {FilePath: "test.mg", GlNode: global},
	}}
	if err := Validate(state); err != nil {
		t.Fatalf("resolved struct symbol rejected: %v", err)
	}

	declaration.AbsName = "test.Other"
	if err := Validate(state); err == nil {
		t.Fatal("expected inconsistent struct symbol to be rejected")
	}
}

func TestRejectsResidualGenericAndCompilerKnownTypes(t *testing.T) {
	voidType := &mt.NodeType{KindNode: &mt.NodeTypeNamed{NameNode: &mt.NodeNameSingle{Name: "void"}}}
	for _, invalidType := range []*mt.NodeType{
		{KindNode: &mt.NodeTypeNamed{NameNode: &mt.NodeNameSingle{Name: "slice"}, GenericArgs: []*mt.NodeType{voidType}}},
		{KindNode: &mt.NodeTypeCompilerKnown{Name: "T"}},
	} {
		file := &mt.FileCtx{FilePath: "test.mg", GlNode: &mt.NodeGlobal{Declarations: []mt.NodeGlobalDecl{
			&mt.NodeExprVarDef{Type: invalidType},
		}}}
		if err := Validate(&mt.SharedState{Files: map[string]*mt.FileCtx{"test.mg": file}}); err == nil {
			t.Fatalf("expected type %T to be rejected", invalidType.KindNode)
		}
	}
}

func TestRejectsVoidValueType(t *testing.T) {
	voidType := &mt.NodeType{KindNode: &mt.NodeTypeNamed{NameNode: &mt.NodeNameSingle{Name: "void"}}}
	variable := &mt.NodeExprVarDef{
		Name:    &mt.NodeNameSingle{Name: "value"},
		Type:    voidType,
		Storage: mt.VariableStorageLocal,
	}
	if err := Validate(stateWithStatements(&mt.NodeStmtExpr{Expression: variable})); err == nil {
		t.Fatal("expected lowering validation to reject a void local variable")
	}
}

func TestAcceptsCompleteRecursiveTypesWithoutMutation(t *testing.T) {
	i64 := &mt.NodeType{KindNode: &mt.NodeTypeNamed{NameNode: &mt.NodeNameSingle{Name: "i64"}}}
	fn := &mt.NodeType{KindNode: &mt.NodeTypeFunc{Args: []*mt.NodeType{i64}, RetType: i64}}
	ptr := &mt.NodeType{KindNode: &mt.NodeTypePointer{Kind: fn.KindNode}}
	variable := &mt.NodeExprVarDef{
		Name:     &mt.NodeNameSingle{Name: "callback"},
		Type:     ptr,
		AbsName:  "test.callback",
		IsGlobal: true,
		Storage:  mt.VariableStorageGlobal,
	}
	file := &mt.FileCtx{FilePath: "test.mg", GlNode: &mt.NodeGlobal{Declarations: []mt.NodeGlobalDecl{variable}}}
	if err := Validate(&mt.SharedState{Files: map[string]*mt.FileCtx{"test.mg": file}}); err != nil {
		t.Fatalf("valid recursive type rejected: %v", err)
	}
	if variable.Type != ptr || ptr.KindNode.(*mt.NodeTypePointer).Kind != fn.KindNode {
		t.Fatal("validation mutated the type tree")
	}
}

func requireStageDiagnostic(test *testing.T, expression mt.NodeExpr) {
	test.Helper()
	err := Validate(stateWithExpression(expression))
	var diagnostic *comp_err.CompilationError
	if !errors.As(err, &diagnostic) {
		test.Fatalf("expected structured stage diagnostic, got %T: %v", err, err)
	}
}

func TestRejectsUnresolvedDirectCall(t *testing.T) {
	requireStageDiagnostic(t, &mt.NodeExprCall{Callee: &mt.NodeExprVoid{}, InfType: &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "void"}}})
}

func TestRejectsInvalidFunctionPointerType(t *testing.T) {
	requireStageDiagnostic(t, &mt.NodeExprCall{Callee: &mt.NodeExprVoid{}, InfType: &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "void"}}, IsFuncPointer: true, FuncPtrType: &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "i64"}}})
}

func TestRejectsFunctionValueSignatureMismatch(t *testing.T) {
	i64 := &mt.NodeType{KindNode: &mt.NodeTypeNamed{NameNode: &mt.NodeNameSingle{Name: "i64"}}}
	boolean := &mt.NodeType{KindNode: &mt.NodeTypeNamed{NameNode: &mt.NodeNameSingle{Name: "bool"}}}
	definition := &mt.NodeFuncDef{
		Class:      mt.NodeGenericClass{ArgsNode: mt.NodeArgList{Args: []mt.NodeArg{{Name: "value", TypeNode: i64}}}},
		ReturnType: i64,
	}
	requireStageDiagnostic(t, &mt.NodeExprName{
		Name:           &mt.NodeNameSingle{Name: "identity"},
		AssociatedNode: definition,
		InfType: &mt.NodeType{KindNode: &mt.NodeTypeFunc{
			Args:    []*mt.NodeType{boolean},
			RetType: i64,
		}},
	})
}

func TestRejectsNonCallTryOperand(t *testing.T) {
	requireStageDiagnostic(t, &mt.NodeExprTry{Call: &mt.NodeExprVoid{}})
}

func TestRejectsUnresolvedName(t *testing.T) {
	requireStageDiagnostic(t, &mt.NodeExprName{
		Name:    &mt.NodeNameSingle{Name: "missing"},
		InfType: &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "i64"}},
	})
}

func TestRejectsGlobalNameWithoutResolvedSymbol(t *testing.T) {
	valueType := &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "i64"}}
	variable := &mt.NodeExprVarDef{
		Name:     &mt.NodeNameSingle{Name: "globalValue"},
		Type:     valueType,
		IsGlobal: true,
	}
	requireStageDiagnostic(t, &mt.NodeExprName{
		Name:           variable.Name,
		InfType:        valueType,
		AssociatedNode: variable,
	})
}

func TestRejectsUnresolvedAndMismatchedVariableStorage(t *testing.T) {
	valueType := &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "i64"}}
	unresolved := &mt.NodeExprVarDef{Name: &mt.NodeNameSingle{Name: "value"}, Type: valueType}
	requireStageDiagnostic(t, &mt.NodeExprName{
		Name: unresolved.Name, InfType: valueType, AssociatedNode: unresolved,
	})

	local := &mt.NodeExprVarDef{Name: &mt.NodeNameSingle{Name: "value"}, Type: valueType, Storage: mt.VariableStorageLocal}
	requireStageDiagnostic(t, &mt.NodeExprName{
		Name: local.Name, InfType: valueType, AssociatedNode: local, Storage: mt.VariableStorageArgument,
	})
}

func TestRejectsIncompleteMemberAccess(t *testing.T) {
	rootType := &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "root"}}
	variable := &mt.NodeExprVarDef{Name: &mt.NodeNameSingle{Name: "value"}, Type: rootType}
	requireStageDiagnostic(t, &mt.NodeExprMemberAccess{
		Target:  &mt.NodeExprName{Name: variable.Name, InfType: rootType, AssociatedNode: variable},
		Member:  "field",
		InfType: rootType,
	})
}

func TestRejectsIncompleteSubscript(t *testing.T) {
	requireStageDiagnostic(t, &mt.NodeExprSubscript{
		Target: &mt.NodeExprVoid{},
		Expr:   &mt.NodeExprLit{},
	})
}

func TestAcceptsResolvedNameMemberAndSubscript(t *testing.T) {
	elementType := &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "i64"}}
	sliceType := &mt.NodeType{KindNode: &mt.NodeTypeSlice{ElemKind: elementType.KindNode}}
	variable := &mt.NodeExprVarDef{Name: &mt.NodeNameSingle{Name: "items"}, Type: sliceType, Storage: mt.VariableStorageLocal}
	name := &mt.NodeExprName{Name: variable.Name, InfType: sliceType, AssociatedNode: variable, Storage: mt.VariableStorageLocal}
	subscript := &mt.NodeExprSubscript{
		Target: name,
		Expr: &mt.NodeExprLit{
			InfType: &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "i64"}},
		},
		BoxType:   sliceType,
		ElemType:  elementType,
		IndexType: elementType,
	}
	if err := Validate(stateWithExpression(subscript)); err != nil {
		t.Fatalf("resolved subscript rejected: %v", err)
	}

	fieldType := &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "field"}}
	member := &mt.NodeExprMemberAccess{
		Target:  name,
		Member:  "field",
		Access:  &mt.MemberAccess{OwnerType: sliceType, Type: fieldType, FieldNb: 0},
		InfType: fieldType,
	}
	if err := Validate(stateWithExpression(member)); err != nil {
		t.Fatalf("resolved member access rejected: %v", err)
	}
}

func TestRejectsInconsistentMemberPathTypes(t *testing.T) {
	rootType := &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "test.Root"}}
	wrongOwner := &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "test.Other"}}
	fieldType := &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "test.Field"}}
	variable := &mt.NodeExprVarDef{Name: &mt.NodeNameSingle{Name: "root"}, Type: rootType}
	requireStageDiagnostic(t, &mt.NodeExprName{
		Name:           variable.Name,
		AssociatedNode: variable,
		InfType:        fieldType,
		MemberAccesses: []*mt.MemberAccess{{OwnerType: wrongOwner, Type: fieldType, FieldNb: 0}},
	})

	pointerRoot := &mt.NodeType{KindNode: &mt.NodeTypePointer{Kind: rootType.KindNode}}
	pointerVariable := &mt.NodeExprVarDef{Name: &mt.NodeNameSingle{Name: "pointer"}, Type: pointerRoot}
	requireStageDiagnostic(t, &mt.NodeExprName{
		Name:           pointerVariable.Name,
		AssociatedNode: pointerVariable,
		InfType:        fieldType,
		MemberAccesses: []*mt.MemberAccess{{OwnerType: pointerRoot, Type: fieldType, FieldNb: 0, PtrDeref: false}},
	})
}

func TestAcceptsCompleteDirectCallWithoutMutation(t *testing.T) {
	returnType := &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "void"}}
	definition := &mt.NodeFuncDef{ReturnType: returnType}
	call := &mt.NodeExprCall{Callee: &mt.NodeExprVoid{VoidType: returnType}, AssociatedFnDef: definition, InfType: returnType}
	if err := Validate(stateWithExpression(call)); err != nil {
		t.Fatalf("valid call rejected: %v", err)
	}
	if call.AssociatedFnDef != definition || call.InfType != returnType {
		t.Fatal("validation mutated the call")
	}
}

func TestRejectsIncompleteValueExpressionMetadata(t *testing.T) {
	i64 := &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "i64"}}
	void := &mt.NodeExprVoid{VoidType: &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "void"}}}
	tests := []mt.NodeExpr{
		&mt.NodeExprLit{},
		&mt.NodeExprUnary{Operand: void},
		&mt.NodeExprBinary{Left: void, Right: void},
		&mt.NodeExprArray{Length: &mt.NodeExprLit{InfType: i64}},
		&mt.NodeExprAddrof{Expr: void},
		&mt.NodeExprStructInit{Type: i64, Fields: []mt.NodeStructFieldInit{{Name: "field", Expression: void}}},
		&mt.NodeExprAssign{Left: void, Right: void},
		&mt.NodeExprSizeof{InfType: i64},
		&mt.NodeExprVoid{},
	}
	for _, expression := range tests {
		requireStageDiagnostic(t, expression)
	}
}

func TestRejectsReturnWithoutOwnerType(t *testing.T) {
	voidType := &mt.NodeType{KindNode: &mt.NodeTypeNamed{NameNode: &mt.NodeNameSingle{Name: "void"}}}
	file := &mt.FileCtx{FilePath: "test.mg", GlNode: &mt.NodeGlobal{Declarations: []mt.NodeGlobalDecl{
		&mt.NodeFuncDef{ReturnType: voidType, Body: mt.NodeBody{Statements: []mt.NodeStatement{
			&mt.NodeStmtRet{Expression: &mt.NodeExprVoid{VoidType: voidType}},
		}}},
	}}}
	err := Validate(&mt.SharedState{Files: map[string]*mt.FileCtx{"test.mg": file}})
	var diagnostic *comp_err.CompilationError
	if !errors.As(err, &diagnostic) {
		t.Fatalf("expected structured stage diagnostic, got %T: %v", err, err)
	}
}

func TestRejectsIncompleteDestructuringMetadata(t *testing.T) {
	i64 := &mt.NodeType{KindNode: &mt.NodeTypeNamed{NameNode: &mt.NodeNameSingle{Name: "i64"}}}
	errType := &mt.NodeType{KindNode: &mt.NodeTypeNamed{NameNode: &mt.NodeNameSingle{Name: "error"}}}
	throwingI64 := *i64
	throwingI64.Throws = true
	callee := &mt.NodeFuncDef{ReturnType: &throwingI64}
	completeCall := func() *mt.NodeExprCall {
		return &mt.NodeExprCall{
			Callee:          &mt.NodeExprVoid{VoidType: i64},
			AssociatedFnDef: callee,
			InfType:         &throwingI64,
		}
	}
	tests := []*mt.NodeExprDestructureAssign{
		{ErrDef: mt.NodeExprVarDef{Name: &mt.NodeNameSingle{Name: "err"}, Type: errType}, Call: completeCall()},
		{ValueDef: mt.NodeExprVarDef{Name: &mt.NodeNameSingle{Name: "value"}, Type: i64}, ErrDef: mt.NodeExprVarDef{Name: &mt.NodeNameSingle{Name: "err"}, Type: i64}, Call: completeCall()},
		{ValueDef: mt.NodeExprVarDef{Name: &mt.NodeNameSingle{Name: "value"}, Type: i64}, ErrDef: mt.NodeExprVarDef{Name: &mt.NodeNameSingle{Name: "err"}, Type: errType}},
	}
	for _, expression := range tests {
		requireStageDiagnostic(t, expression)
	}
}

func TestRejectsIncompleteControlFlowMetadata(t *testing.T) {
	i64 := &mt.NodeType{KindNode: &mt.NodeTypeNamed{NameNode: &mt.NodeNameSingle{Name: "i64"}}}
	voidType := &mt.NodeType{KindNode: &mt.NodeTypeNamed{NameNode: &mt.NodeNameSingle{Name: "void"}}}
	voidExpr := &mt.NodeExprVoid{VoidType: voidType}
	tests := []mt.NodeStatement{
		&mt.NodeStmtIf{CondExpr: &mt.NodeExprLit{InfType: i64}},
		&mt.NodeStmtWhile{CondExpr: &mt.NodeExprLit{InfType: i64}},
		&mt.NodeStmtIf{CondExpr: &mt.NodeExprLit{InfType: &mt.NodeType{KindNode: &mt.NodeTypeNamed{NameNode: &mt.NodeNameSingle{Name: "bool"}}}}, NextCondStmt: &mt.NodeStmtRet{Expression: voidExpr, OwnerFuncType: voidType}},
		&mt.NodeStmtDefer{IsBody: true, Expression: voidExpr},
		&mt.NodeStmtDefer{Expression: voidExpr, Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtExpr{Expression: voidExpr}}}},
		&mt.NodeStmtBreak{},
		&mt.NodeStmtContinue{},
	}
	for _, statement := range tests {
		requireStatementStageDiagnostic(t, statement)
	}
}

func TestAcceptsCompleteSpecializedControlFlowWithoutMutation(t *testing.T) {
	boolType := &mt.NodeType{KindNode: &mt.NodeTypeNamed{NameNode: &mt.NodeNameSingle{Name: "bool"}}}
	condition := &mt.NodeExprLit{InfType: boolType}
	loop := &mt.NodeStmtWhile{CondExpr: condition, Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtBreak{}}}}
	deferred := &mt.NodeStmtDefer{IsBody: true, Body: mt.NodeBody{Statements: []mt.NodeStatement{loop}}}
	if err := Validate(stateWithStatements(deferred)); err != nil {
		t.Fatalf("valid specialized control flow rejected: %v", err)
	}
	if loop.CondExpr != condition || deferred.Body.Statements[0] != loop {
		t.Fatal("validation mutated specialized control flow")
	}
}
