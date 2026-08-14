package destroychecker

import (
	"Magma/src/types"
	"strings"
	"testing"
)

func fixture() (*analyzer, *types.NodeType) {
	destructor := &types.NodeFuncDef{IsDestructor: true}
	global := &types.NodeGlobal{StructDefs: map[string]*types.StructDef{
		"Resource": {Module: "test", Name: "Resource", Destructors: []*types.NodeFuncDef{destructor}},
	}}
	file := &types.FileCtx{FilePath: "test.mg", PackageName: "test", GlNode: global}
	shared := &types.SharedState{Files: map[string]*types.FileCtx{"test.mg": file}}
	a := &analyzer{shared: shared, file: file, seen: map[string]bool{}}
	resourceType := &types.NodeType{KindNode: &types.NodeTypeAbsolute{AbsoluteName: "test.Resource"}}
	return a, resourceType
}

func name(variable *types.NodeExprVarDef) *types.NodeExprName {
	return &types.NodeExprName{Name: variable.Name, AssociatedNode: variable, InfType: variable.Type}
}

func aggregateFixture() (*analyzer, *types.NodeType, *types.NodeType, *types.StructDef) {
	destructor := &types.NodeFuncDef{IsDestructor: true}
	resource := &types.StructDef{Module: "test", Name: "Resource", Destructors: []*types.NodeFuncDef{destructor}}
	resourceType := &types.NodeType{KindNode: &types.NodeTypeAbsolute{AbsoluteName: "test.Resource"}}
	container := &types.StructDef{
		Module:      "test",
		Name:        "Container",
		Destructors: []*types.NodeFuncDef{destructor},
		FieldNb:     map[string]int{"left": 0, "right": 1, "count": 2},
		Fields:      map[string]*types.NodeType{"left": resourceType, "right": resourceType, "count": {KindNode: &types.NodeTypeNamed{NameNode: &types.NodeNameSingle{Name: "u64"}}}},
		FieldOrder:  []string{"left", "right", "count"},
	}
	global := &types.NodeGlobal{StructDefs: map[string]*types.StructDef{"Resource": resource, "Container": container}}
	file := &types.FileCtx{FilePath: "test.mg", PackageName: "test", GlNode: global}
	shared := &types.SharedState{Files: map[string]*types.FileCtx{"test.mg": file}}
	a := &analyzer{shared: shared, file: file, seen: map[string]bool{}}
	containerType := &types.NodeType{KindNode: &types.NodeTypeAbsolute{AbsoluteName: "test.Container"}}
	return a, containerType, resourceType, container
}

func field(variable *types.NodeExprVarDef, owner *types.StructDef, fieldType *types.NodeType, fieldName string) *types.NodeExprName {
	return &types.NodeExprName{
		Name:           variable.Name,
		AssociatedNode: variable,
		InfType:        fieldType,
		MemberAccesses: []*types.MemberAccess{{OwnerType: variable.Type, Type: fieldType, OwnerDef: owner, FieldNb: owner.FieldNb[fieldName]}},
	}
}

func TestFieldMoveLeavesSiblingUsableButRejectsWholeAggregateUse(t *testing.T) {
	a, containerType, resourceType, container := aggregateFixture()
	value := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "value"}, Type: containerType}
	out := flow{states: map[*types.NodeExprVarDef]State{value: stateLive}, absent: map[placeKey]types.Token{}, deferred: map[*types.NodeExprVarDef]bool{}}

	a.transferValue(&out, &types.NodeExprMove{Expr: field(value, container, resourceType, "left")})
	a.borrowExpr(&out, field(value, container, resourceType, "right"))
	countType := container.Fields["count"]
	a.borrowExpr(&out, field(value, container, countType, "count"))
	if len(a.diagnostics) != 0 {
		t.Fatalf("sibling use after field move produced diagnostics: %+v", a.diagnostics)
	}
	a.borrowExpr(&out, name(value))
	if len(a.diagnostics) != 1 || !strings.Contains(a.diagnostics[0].Message, "after it was moved") {
		t.Fatalf("whole aggregate use diagnostics = %+v", a.diagnostics)
	}
}

func TestMovedFieldCanBeReinitialized(t *testing.T) {
	a, containerType, resourceType, container := aggregateFixture()
	value := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "value"}, Type: containerType}
	out := flow{states: map[*types.NodeExprVarDef]State{value: stateLive}, absent: map[placeKey]types.Token{}, deferred: map[*types.NodeExprVarDef]bool{}}
	left := field(value, container, resourceType, "left")

	a.transferValue(&out, &types.NodeExprMove{Expr: left})
	a.assignment(&out, &types.NodeExprAssign{Left: left, Right: callReturning(resourceType, true)})
	a.borrowExpr(&out, name(value))

	if len(a.diagnostics) != 0 {
		t.Fatalf("reinitialized aggregate produced diagnostics: %+v", a.diagnostics)
	}
}

func TestMovingEveryOwnedFieldCompletesDestructuring(t *testing.T) {
	a, containerType, resourceType, container := aggregateFixture()
	value := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "value"}, Type: containerType}
	out := flow{states: map[*types.NodeExprVarDef]State{value: stateLive}, absent: map[placeKey]types.Token{}, deferred: map[*types.NodeExprVarDef]bool{}}

	a.transferValue(&out, &types.NodeExprMove{Expr: field(value, container, resourceType, "left")})
	a.transferValue(&out, &types.NodeExprMove{Expr: field(value, container, resourceType, "right")})

	if out.states[value] != stateConsumed {
		t.Fatalf("aggregate state = %v, want consumed after complete owned-field extraction", out.states[value])
	}
}

func TestIndexedOwnershipMoveIsRejected(t *testing.T) {
	a, resourceType := fixture()
	value := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "value"}, Type: resourceType}
	out := flow{states: map[*types.NodeExprVarDef]State{value: stateLive}, absent: map[placeKey]types.Token{}, deferred: map[*types.NodeExprVarDef]bool{}}
	indexed := &types.NodeExprSubscript{Target: name(value), Expr: &types.NodeExprLit{LitType: types.TokLitNum, Value: "0"}, ElemType: resourceType}

	a.transferValue(&out, &types.NodeExprMove{Expr: indexed})

	if len(a.diagnostics) != 1 || !strings.Contains(a.diagnostics[0].Message, "checked container operation") {
		t.Fatalf("indexed move diagnostics = %+v", a.diagnostics)
	}
}

func callReturning(resourceType *types.NodeType, owned bool) *types.NodeExprCall {
	returnType := *resourceType
	returnType.Owned = owned
	definition := &types.NodeFuncDef{ReturnType: &returnType}
	return &types.NodeExprCall{AssociatedFnDef: definition, InfType: &returnType}
}

func TestWholeVariableAssignmentTransfersOwnership(t *testing.T) {
	a, resourceType := fixture()
	source := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "source"}, Type: resourceType}
	destination := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "destination"}, Type: resourceType}
	out := flow{
		states:   map[*types.NodeExprVarDef]State{source: stateLive},
		deferred: map[*types.NodeExprVarDef]bool{},
	}

	a.assignment(&out, &types.NodeExprAssign{Left: name(destination), Right: name(source)})

	if out.states[source] != stateConsumed {
		t.Fatalf("source state = %v, want consumed", out.states[source])
	}
	if out.states[destination] != stateLive {
		t.Fatalf("destination state = %v, want live", out.states[destination])
	}
}

func TestStructConstructorTransfersFieldOwnership(t *testing.T) {
	a, resourceType := fixture()
	source := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "source"}, Type: resourceType}
	out := flow{states: map[*types.NodeExprVarDef]State{source: stateLive}, deferred: map[*types.NodeExprVarDef]bool{}}
	init := &types.NodeExprStructInit{
		Type: resourceType,
		Fields: []types.NodeStructFieldInit{{
			Name:       "resource",
			Expression: name(source),
		}},
	}

	a.transferValue(&out, init)

	if out.states[source] != stateConsumed {
		t.Fatalf("source state = %v, want constructor field transfer to consume it", out.states[source])
	}
}

func TestOwnedReturnEvaluatesConsumingCallArguments(t *testing.T) {
	a, resourceType := fixture()
	source := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "source"}, Type: resourceType}
	ownedParameter := *resourceType
	ownedParameter.Owned = true
	call := &types.NodeExprCall{
		AssociatedFnDef: &types.NodeFuncDef{Class: types.NodeGenericClass{ArgsNode: types.NodeArgList{Args: []types.NodeArg{{Name: "value", TypeNode: &ownedParameter}}}}},
		Args:            []types.NodeExpr{name(source)},
	}
	out := flow{states: map[*types.NodeExprVarDef]State{source: stateLive}, deferred: map[*types.NodeExprVarDef]bool{}}

	a.transferValue(&out, call)

	if out.states[source] != stateConsumed {
		t.Fatalf("source state = %v, want consuming call argument to be evaluated", out.states[source])
	}
}

func TestErrorPredicateRefinesConditionalOwnership(t *testing.T) {
	_, resourceType := fixture()
	value := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "value"}, Type: resourceType}
	errVariable := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "err"}, Type: &types.NodeType{KindNode: &types.NodeTypeNamed{NameNode: &types.NodeNameSingle{Name: "error"}}}}
	out := flow{
		states:     map[*types.NodeExprVarDef]State{value: stateConditional},
		deferred:   map[*types.NodeExprVarDef]bool{},
		conditions: map[*types.NodeExprVarDef]*types.NodeExprVarDef{value: errVariable},
	}
	predicate := &types.NodeExprCall{
		AssociatedFnDef: &types.NodeFuncDef{ErrorPredicate: types.ErrorPredicateNok},
		IsMemberFunc:    true,
		MemberOwnerName: name(errVariable),
	}

	failure, success := predicateFlows(out, predicate)
	if failure.states[value] != stateBorrowed {
		t.Fatalf("nok true state = %v, want borrowed/absent", failure.states[value])
	}
	if success.states[value] != stateLive {
		t.Fatalf("nok false state = %v, want live", success.states[value])
	}
}

func TestImplicitFieldTransferRequiresMove(t *testing.T) {
	a, resourceType := fixture()
	aggregate := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "aggregate"}, Type: resourceType}
	destination := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "destination"}, Type: resourceType}
	out := flow{
		states:   map[*types.NodeExprVarDef]State{aggregate: stateLive},
		deferred: map[*types.NodeExprVarDef]bool{},
	}
	owner := a.shared.Files["test.mg"].GlNode.StructDefs["Resource"]
	owner.FieldNb = map[string]int{"field": 0, "other": 1}
	owner.Fields = map[string]*types.NodeType{"field": resourceType, "other": resourceType}
	owner.FieldOrder = []string{"field", "other"}
	field := &types.NodeExprMemberAccess{Target: name(aggregate), Member: "field", InfType: resourceType, Access: &types.MemberAccess{OwnerType: resourceType, Type: resourceType, OwnerDef: owner, FieldNb: 0}}

	a.assignment(&out, &types.NodeExprAssign{Left: name(destination), Right: field})

	if out.states[aggregate] != stateLive {
		t.Fatalf("aggregate state = %v, moving one field must leave the root partially live", out.states[aggregate])
	}
	if len(a.diagnostics) != 1 || !strings.Contains(a.diagnostics[0].Message, "requires 'move'") {
		t.Fatalf("implicit field transfer diagnostics = %+v", a.diagnostics)
	}
}

func TestOwnedLocalCannotSilentlyClaimField(t *testing.T) {
	a, resourceType := fixture()
	ownedType := *resourceType
	ownedType.Owned = true
	aggregate := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "aggregate"}, Type: resourceType}
	destination := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "destination"}, Type: &ownedType}
	out := flow{states: map[*types.NodeExprVarDef]State{aggregate: stateLive}, deferred: map[*types.NodeExprVarDef]bool{}}
	owner := a.shared.Files["test.mg"].GlNode.StructDefs["Resource"]
	owner.FieldNb = map[string]int{"field": 0, "other": 1}
	owner.Fields = map[string]*types.NodeType{"field": resourceType, "other": resourceType}
	owner.FieldOrder = []string{"field", "other"}
	field := &types.NodeExprMemberAccess{Target: name(aggregate), Member: "field", InfType: resourceType, Access: &types.MemberAccess{OwnerType: resourceType, Type: resourceType, OwnerDef: owner, FieldNb: 0}}

	a.valueInto(&out, destination, field)

	if out.states[destination] != stateLive {
		t.Fatalf("field transfer destination state = %v, want live", out.states[destination])
	}
	if out.states[aggregate] != stateLive {
		t.Fatal("single field transfer consumed the whole aggregate")
	}
	if len(a.diagnostics) != 1 || !strings.Contains(a.diagnostics[0].Message, "requires 'move'") {
		t.Fatalf("implicit field transfer diagnostics = %+v", a.diagnostics)
	}
}

func TestBorrowedParameterIsNotAnOwnershipObligation(t *testing.T) {
	a, resourceType := fixture()
	borrowed := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "borrowed"}, Type: resourceType}
	out := flow{states: map[*types.NodeExprVarDef]State{}, deferred: map[*types.NodeExprVarDef]bool{}}

	a.use(&out, borrowed)
	a.checkExit(&out)

	if len(a.diagnostics) != 0 {
		t.Fatalf("borrow created ownership diagnostics: %+v", a.diagnostics)
	}
}

func TestConsumingBorrowWarns(t *testing.T) {
	a, resourceType := fixture()
	borrowed := &types.NodeExprVarDef{Name: &types.NodeNameSingle{
		Tk:   types.Token{Pos: types.FilePos{Line: 12, Col: 7}},
		Name: "borrowed",
	}, Type: resourceType}
	out := flow{states: map[*types.NodeExprVarDef]State{}, deferred: map[*types.NodeExprVarDef]bool{}}

	a.consume(&out, borrowed, "consuming argument")

	if len(a.diagnostics) != 1 || !strings.Contains(a.diagnostics[0].Message, "borrowed destructible value") {
		t.Fatalf("diagnostics = %+v, want borrowed-value warning", a.diagnostics)
	}
	if a.diagnostics[0].Line != 12 || a.diagnostics[0].Column != 7 {
		t.Fatalf("diagnostic position = %d:%d, want 12:7", a.diagnostics[0].Line, a.diagnostics[0].Column)
	}
}

func TestPlainReturnInitializesBorrowedLocal(t *testing.T) {
	a, resourceType := fixture()
	destination := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "value"}, Type: resourceType}
	out := flow{states: map[*types.NodeExprVarDef]State{}, deferred: map[*types.NodeExprVarDef]bool{}}

	a.valueInto(&out, destination, callReturning(resourceType, false))
	a.checkExit(&out)

	if out.states[destination] != stateBorrowed {
		t.Fatalf("plain return state = %v, want borrowed", out.states[destination])
	}
	if len(a.diagnostics) != 0 {
		t.Fatalf("borrowed return produced diagnostics: %+v", a.diagnostics)
	}
}

func TestOwnedReturnInitializesOwnedLocal(t *testing.T) {
	a, resourceType := fixture()
	destination := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "value"}, Type: resourceType}
	out := flow{states: map[*types.NodeExprVarDef]State{}, deferred: map[*types.NodeExprVarDef]bool{}}

	a.valueInto(&out, destination, callReturning(resourceType, true))

	if out.states[destination] != stateLive {
		t.Fatalf("owned return state = %v, want live", out.states[destination])
	}
}

func TestBorrowAssignmentRemainsBorrowed(t *testing.T) {
	a, resourceType := fixture()
	source := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "source"}, Type: resourceType}
	destination := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "destination"}, Type: resourceType}
	out := flow{states: map[*types.NodeExprVarDef]State{}, deferred: map[*types.NodeExprVarDef]bool{}}

	a.assignment(&out, &types.NodeExprAssign{Left: name(destination), Right: name(source)})

	if out.states[destination] != stateBorrowed {
		t.Fatalf("borrow assignment state = %v, want borrowed", out.states[destination])
	}
	if len(a.diagnostics) != 0 {
		t.Fatalf("borrow-to-borrow assignment produced diagnostics: %+v", a.diagnostics)
	}
}

func TestBorrowedAndOwnedBranchesMergeConservatively(t *testing.T) {
	_, resourceType := fixture()
	value := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "value"}, Type: resourceType}
	borrowed := flow{states: map[*types.NodeExprVarDef]State{value: stateBorrowed}, deferred: map[*types.NodeExprVarDef]bool{}}
	owned := flow{states: map[*types.NodeExprVarDef]State{value: stateLive}, deferred: map[*types.NodeExprVarDef]bool{}}

	merged := mergeFlows(borrowed, owned)

	if merged.states[value] != stateMaybeConsumed {
		t.Fatalf("merged state = %v, want maybe-consumed/conditionally-owned", merged.states[value])
	}
}

func TestDestructorArgumentsAreAllowed(t *testing.T) {
	a, _ := fixture()
	voidType := &types.NodeType{KindNode: &types.NodeTypeNamed{NameNode: &types.NodeNameSingle{Name: "void"}}}
	destructor := &types.NodeFuncDef{
		AbsName:      "test.Resource.free",
		IsDestructor: true,
		ReturnType:   voidType,
	}
	destructor.Class.ArgsNode.Args = []types.NodeArg{{Name: "this"}, {Name: "allocator"}}
	global := &types.NodeGlobal{StructDefs: map[string]*types.StructDef{
		"Resource": {Module: "test", Name: "Resource", Destructors: []*types.NodeFuncDef{destructor}},
	}}

	validateDestructors(a, global)

	if len(a.diagnostics) != 0 {
		t.Fatalf("destructor arguments produced diagnostics: %+v", a.diagnostics)
	}
}

func TestThrowingDestructorIsAllowed(t *testing.T) {
	a, _ := fixture()
	voidType := &types.NodeType{KindNode: &types.NodeTypeNamed{NameNode: &types.NodeNameSingle{Name: "void"}}, Throws: true}
	destructor := &types.NodeFuncDef{AbsName: "test.Resource.close", IsDestructor: true, ReturnType: voidType}
	global := &types.NodeGlobal{StructDefs: map[string]*types.StructDef{
		"Resource": {Module: "test", Name: "Resource", Destructors: []*types.NodeFuncDef{destructor}},
	}}

	validateDestructors(a, global)

	if len(a.diagnostics) != 0 {
		t.Fatalf("throwing destructor produced diagnostics: %+v", a.diagnostics)
	}
}

func TestValueReturningDestructorIsAllowed(t *testing.T) {
	destructor := &types.NodeFuncDef{
		AbsName:      "test.Resource.take",
		IsDestructor: true,
		ReturnType:   &types.NodeType{KindNode: &types.NodeTypeNamed{NameNode: &types.NodeNameSingle{Name: "u64"}}},
	}
	global := &types.NodeGlobal{StructDefs: map[string]*types.StructDef{
		"Resource": {Module: "test", Name: "Resource", Destructors: []*types.NodeFuncDef{destructor}},
	}}
	a := &analyzer{}
	validateDestructors(a, global)
	if len(a.diagnostics) != 0 {
		t.Fatalf("value-returning destructor produced diagnostics: %+v", a.diagnostics)
	}
}

func TestFunctionPointerCanConsumeOwnedArgument(t *testing.T) {
	a, resourceType := fixture()
	ownedType := *resourceType
	ownedType.Owned = true
	value := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "value"}, Type: resourceType}
	out := flow{states: map[*types.NodeExprVarDef]State{value: stateLive}, deferred: map[*types.NodeExprVarDef]bool{}}
	call := &types.NodeExprCall{
		IsFuncPointer: true,
		FuncPtrType: &types.NodeType{KindNode: &types.NodeTypeFunc{
			Args: []*types.NodeType{&ownedType},
		}},
		Args: []types.NodeExpr{name(value)},
	}

	a.call(&out, call)

	if out.states[value] != stateConsumed {
		t.Fatalf("function-pointer argument state = %v, want consumed", out.states[value])
	}
}

func TestFunctionPointerCannotConsumeBorrowedArgument(t *testing.T) {
	a, resourceType := fixture()
	ownedType := *resourceType
	ownedType.Owned = true
	value := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "value"}, Type: resourceType}
	out := flow{states: map[*types.NodeExprVarDef]State{value: stateBorrowed}, deferred: map[*types.NodeExprVarDef]bool{}}
	call := &types.NodeExprCall{
		IsFuncPointer: true,
		FuncPtrType: &types.NodeType{KindNode: &types.NodeTypeFunc{
			Args: []*types.NodeType{&ownedType},
		}},
		Args: []types.NodeExpr{name(value)},
	}

	a.call(&out, call)

	if len(a.diagnostics) != 1 || !strings.Contains(a.diagnostics[0].Message, "borrowed destructible value") {
		t.Fatalf("diagnostics = %+v, want borrowed-value warning", a.diagnostics)
	}
}

func TestBorrowedReturnCannotBeConsumed(t *testing.T) {
	a, resourceType := fixture()
	destination := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "value"}, Type: resourceType}
	out := flow{states: map[*types.NodeExprVarDef]State{}, deferred: map[*types.NodeExprVarDef]bool{}}
	a.valueInto(&out, destination, callReturning(resourceType, false))

	a.consume(&out, destination, "destructor call")

	if len(a.diagnostics) != 1 || !strings.Contains(a.diagnostics[0].Message, "borrowed destructible value") {
		t.Fatalf("diagnostics = %+v, want borrowed-value warning", a.diagnostics)
	}
}

func destructorCall(variable *types.NodeExprVarDef) *types.NodeExprCall {
	return &types.NodeExprCall{
		AssociatedFnDef: &types.NodeFuncDef{IsDestructor: true},
		IsMemberFunc:    true,
		MemberOwnerExpr: name(variable),
	}
}

func TestScopeLocalDeferConsumesLocal(t *testing.T) {
	a, resourceType := fixture()
	value := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "value"}, Type: resourceType}
	out := flow{states: map[*types.NodeExprVarDef]State{}, deferred: map[*types.NodeExprVarDef]bool{}}
	body := types.NodeBody{Statements: []types.NodeStatement{
		&types.NodeStmtExpr{Expression: &types.NodeExprVarDefAssign{VarDef: value, AssignExpr: callReturning(resourceType, true)}},
		&types.NodeStmtDefer{Expression: destructorCall(value)},
	}}

	a.body(&out, &body)

	if len(a.diagnostics) != 0 {
		t.Fatalf("scope-local defer produced diagnostics: %+v", a.diagnostics)
	}
	if _, exists := out.states[value]; exists {
		t.Fatal("scope-local value escaped its scope")
	}
}

func TestLoopDeferRunsOnEveryContinue(t *testing.T) {
	a, resourceType := fixture()
	value := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "value"}, Type: resourceType}
	out := flow{states: map[*types.NodeExprVarDef]State{}, deferred: map[*types.NodeExprVarDef]bool{}, scopes: []deferScope{{locals: map[*types.NodeExprVarDef]bool{}}}}
	loop := &types.NodeStmtWhile{
		CondExpr: &types.NodeExprLit{Value: "true", LitType: types.TokLitBool},
		Body: types.NodeBody{Statements: []types.NodeStatement{
			&types.NodeStmtExpr{Expression: &types.NodeExprVarDefAssign{VarDef: value, AssignExpr: callReturning(resourceType, true)}},
			&types.NodeStmtDefer{Expression: destructorCall(value)},
			&types.NodeStmtContinue{},
		}},
	}

	a.statement(&out, loop)

	if len(a.diagnostics) != 0 {
		t.Fatalf("loop-local defer produced diagnostics: %+v", a.diagnostics)
	}
	if !out.terminated {
		t.Fatal("unbroken while true should not fall through")
	}
}

func TestThrowUnwindsNestedBlockDefer(t *testing.T) {
	a, resourceType := fixture()
	value := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "value"}, Type: resourceType}
	out := flow{states: map[*types.NodeExprVarDef]State{}, deferred: map[*types.NodeExprVarDef]bool{}, scopes: []deferScope{{locals: map[*types.NodeExprVarDef]bool{}}}}
	body := types.NodeBody{Statements: []types.NodeStatement{
		&types.NodeStmtExpr{Expression: &types.NodeExprVarDefAssign{VarDef: value, AssignExpr: callReturning(resourceType, true)}},
		&types.NodeStmtDefer{IsBody: true, Body: types.NodeBody{Statements: []types.NodeStatement{
			&types.NodeStmtExpr{Expression: destructorCall(value)},
		}}},
		&types.NodeStmtThrow{Expression: &types.NodeExprLit{Value: "failure"}},
	}}

	a.body(&out, &body)

	if len(a.diagnostics) != 0 {
		t.Fatalf("throw unwind produced diagnostics: %+v", a.diagnostics)
	}
	if !out.terminated {
		t.Fatal("throw did not terminate the flow")
	}
}

func TestThrowRunsOnErrorCleanup(t *testing.T) {
	a, resourceType := fixture()
	value := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "value"}, Type: resourceType}
	out := flow{states: map[*types.NodeExprVarDef]State{}, deferred: map[*types.NodeExprVarDef]bool{}, scopes: []deferScope{{locals: map[*types.NodeExprVarDef]bool{}}}}
	body := types.NodeBody{Statements: []types.NodeStatement{
		&types.NodeStmtExpr{Expression: &types.NodeExprVarDefAssign{VarDef: value, AssignExpr: callReturning(resourceType, true)}},
		&types.NodeStmtDefer{Expression: destructorCall(value), OnError: true},
		&types.NodeStmtThrow{Expression: &types.NodeExprLit{Value: "failure"}},
	}}

	a.body(&out, &body)

	if len(a.diagnostics) != 0 {
		t.Fatalf("onerror cleanup produced diagnostics on throw: %+v", a.diagnostics)
	}
}

func TestOnErrorCleanupDoesNotRunOnSuccess(t *testing.T) {
	a, resourceType := fixture()
	value := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "value"}, Type: resourceType}
	out := flow{states: map[*types.NodeExprVarDef]State{}, deferred: map[*types.NodeExprVarDef]bool{}}
	body := types.NodeBody{Statements: []types.NodeStatement{
		&types.NodeStmtExpr{Expression: &types.NodeExprVarDefAssign{VarDef: value, AssignExpr: callReturning(resourceType, true)}},
		&types.NodeStmtDefer{Expression: destructorCall(value), OnError: true},
	}}

	a.body(&out, &body)

	if len(a.diagnostics) != 1 || !strings.Contains(a.diagnostics[0].Message, "not consumed") {
		t.Fatalf("diagnostics = %+v, want successful-exit ownership warning", a.diagnostics)
	}
}

func TestOnErrorCleanupAllowsSuccessfulReturnTransfer(t *testing.T) {
	a, resourceType := fixture()
	ownedType := *resourceType
	ownedType.Owned = true
	value := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "value"}, Type: resourceType}
	out := flow{states: map[*types.NodeExprVarDef]State{}, deferred: map[*types.NodeExprVarDef]bool{}, scopes: []deferScope{{locals: map[*types.NodeExprVarDef]bool{}}}}
	body := types.NodeBody{Statements: []types.NodeStatement{
		&types.NodeStmtExpr{Expression: &types.NodeExprVarDefAssign{VarDef: value, AssignExpr: callReturning(resourceType, true)}},
		&types.NodeStmtDefer{Expression: destructorCall(value), OnError: true},
		&types.NodeStmtRet{Expression: &types.NodeExprMove{Expr: name(value)}, OwnerFuncType: &ownedType},
	}}

	a.body(&out, &body)

	if len(a.diagnostics) != 0 {
		t.Fatalf("successful return with onerror cleanup produced diagnostics: %+v", a.diagnostics)
	}
}

func TestUninitializedDestructibleStartsAsZeroValue(t *testing.T) {
	a, resourceType := fixture()
	value := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Name: "value"}, Type: resourceType}
	out := flow{states: map[*types.NodeExprVarDef]State{}, deferred: map[*types.NodeExprVarDef]bool{}, scopes: []deferScope{{locals: map[*types.NodeExprVarDef]bool{}}}}

	a.expression(&out, value)
	a.unwindTo(&out, 0, false)

	if len(a.diagnostics) != 0 {
		t.Fatalf("zero-valued declaration produced diagnostics: %+v", a.diagnostics)
	}
}
