//go:build llvm_object

package loweringast_test

import (
	"strings"
	"testing"

	llvmobject "Magma/src/llvm_object"
	loweringast "Magma/src/lowering_ast"
	lb "Magma/src/lowering_backend"
	loweringtypes "Magma/src/lowering_types"
	mt "Magma/src/types"
)

func primitive(name string) *mt.NodeType {
	return &mt.NodeType{KindNode: &mt.NodeTypeNamed{NameNode: &mt.NodeNameSingle{Name: name}}}
}

func TestScalarASTFunctionLowersCheckedTypesAndStorage(t *testing.T) {
	i32, i64 := primitive("i32"), primitive("i64")
	a := &mt.NodeExprVarDef{Name: &mt.NodeNameSingle{Name: "a"}, Type: i32, Storage: mt.VariableStorageArgument}
	b := &mt.NodeExprVarDef{Name: &mt.NodeNameSingle{Name: "b"}, Type: i64, Storage: mt.VariableStorageArgument}
	local := &mt.NodeExprVarDef{Name: &mt.NodeNameSingle{Name: "value"}, Type: i64, Storage: mt.VariableStorageLocal}
	aName := &mt.NodeExprName{Name: a.Name, InfType: i32, AssociatedNode: a, Storage: mt.VariableStorageArgument}
	bName := &mt.NodeExprName{Name: b.Name, InfType: i64, AssociatedNode: b, Storage: mt.VariableStorageArgument}
	addition := &mt.NodeExprBinary{Operator: mt.KwPlus, Left: aName, Right: bName, OperandType: i64, InfType: i64}
	initialize := &mt.NodeExprVarDefAssign{VarDef: local, AssignExpr: addition}
	result := &mt.NodeExprName{Name: local.Name, InfType: i64, AssociatedNode: local, Storage: mt.VariableStorageLocal}
	definition := &mt.NodeFuncDef{
		Class:      mt.NodeGenericClass{ArgsNode: mt.NodeArgList{Args: []mt.NodeArg{{Name: "a", TypeNode: i32}, {Name: "b", TypeNode: i64}}}},
		ReturnType: i64, AbsName: "app.add", ContextABI: mt.ContextABIContextless,
		Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtExpr{Expression: initialize}, &mt.NodeStmtRet{Expression: result, OwnerFuncType: i64}}},
	}
	ir, err := llvmobject.ExperimentalIR("scalar-ast.mg", func(backend lb.Backend) error {
		typeLowerer, err := loweringtypes.New(backend, &mt.SharedState{Files: map[string]*mt.FileCtx{}, CoreTypes: map[mt.CoreTypeRole]*mt.StructDef{}})
		if err != nil {
			return err
		}
		_, err = loweringast.LowerScalarFunction(backend, typeLowerer, definition)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(ir)
	for _, expected := range []string{"define internal i64 @app.add(i32 %0, i64 %1)", "sext i32", "add i64", "store i64 0"} {
		if !strings.Contains(text, expected) {
			t.Errorf("AST lowering missing %q:\n%s", expected, text)
		}
	}
	firstStore := strings.Index(text, "store i32 %0")
	branchOrBody := strings.Index(text, "sext i32")
	if firstStore < 0 || branchOrBody < 0 {
		t.Fatalf("missing argument materialization or body instruction:\n%s", text)
	}
	lastAlloca := strings.LastIndex(text[:branchOrBody], "alloca ")
	if lastAlloca < 0 || lastAlloca > firstStore {
		t.Fatalf("fixed local storage was not grouped at function entry:\n%s", text)
	}
	if strings.Contains(text, "icmp") || strings.Contains(text, "llvm.lifetime") {
		t.Fatalf("AST scalar lowering introduced runtime behavior absent from textual lowering:\n%s", text)
	}
}

func TestThrowingFunctionSuccessUsesTextualABIEnvelope(t *testing.T) {
	i64, void := primitive("i64"), primitive("void")
	throwingI64 := primitive("i64")
	throwingI64.Throws = true
	throwingVoid := primitive("void")
	throwingVoid.Throws = true
	errorDef := &mt.StructDef{
		Module: "std", Name: "Error", CoreRole: mt.CoreTypeError,
		FieldOrder: []string{"__code"}, FieldNb: map[string]int{"__code": 0},
		Fields: map[string]*mt.NodeType{"__code": primitive("u32")},
	}
	state := &mt.SharedState{Files: map[string]*mt.FileCtx{}, CoreTypes: map[mt.CoreTypeRole]*mt.StructDef{mt.CoreTypeError: errorDef}}
	valueFn := &mt.NodeFuncDef{
		ReturnType: throwingI64, AbsName: "app.throwing_value", ContextABI: mt.ContextABIContextless,
		Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtRet{Expression: literal("7", i64), OwnerFuncType: throwingI64}}},
	}
	voidFn := &mt.NodeFuncDef{
		ReturnType: throwingVoid, AbsName: "app.throwing_void", ContextABI: mt.ContextABIContextless,
		Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtRet{Expression: &mt.NodeExprVoid{VoidType: void}, OwnerFuncType: throwingVoid}}},
	}
	ir, err := llvmobject.ExperimentalIR("throwing-success.mg", func(backend lb.Backend) error {
		typeLowerer, err := loweringtypes.New(backend, state)
		if err != nil {
			return err
		}
		if _, err = loweringast.LowerScalarFunction(backend, typeLowerer, valueFn); err != nil {
			return err
		}
		_, err = loweringast.LowerScalarFunction(backend, typeLowerer, voidFn)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(ir)
	for _, expected := range []string{
		"define internal { %type.error, i64 } @app.throwing_value()",
		"ret { %type.error, i64 } { %type.error zeroinitializer, i64 7 }",
		"define internal { %type.error } @app.throwing_void()",
		"ret { %type.error } zeroinitializer",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("throwing success lowering missing %q:\n%s", expected, text)
		}
	}
	if strings.Contains(text, "icmp") || strings.Contains(text, "br i1") {
		t.Fatalf("successful throwing return introduced a runtime check absent from textual lowering:\n%s", text)
	}
}

func TestThrowingCallsUsePhysicalResultAndPreserveDiscardBehavior(t *testing.T) {
	i64, void := primitive("i64"), primitive("void")
	throwingI64 := primitive("i64")
	throwingI64.Throws = true
	errorDef := &mt.StructDef{
		Module: "std", Name: "Error", CoreRole: mt.CoreTypeError,
		FieldOrder: []string{"__code"}, FieldNb: map[string]int{"__code": 0},
		Fields: map[string]*mt.NodeType{"__code": primitive("u32")},
	}
	state := &mt.SharedState{Files: map[string]*mt.FileCtx{}, CoreTypes: map[mt.CoreTypeRole]*mt.StructDef{mt.CoreTypeError: errorDef}}
	callee := &mt.NodeFuncDef{
		ReturnType: throwingI64, AbsName: "app.may_fail", ContextABI: mt.ContextABIContextless,
		Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtRet{Expression: literal("9", i64), OwnerFuncType: throwingI64}}},
	}
	valueCall := &mt.NodeExprCall{AssociatedFnDef: callee, InfType: i64, ThrowingType: throwingI64}
	valueCaller := &mt.NodeFuncDef{
		ReturnType: i64, AbsName: "app.use_throwing_value", ContextABI: mt.ContextABIContextless,
		Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtRet{Expression: valueCall, OwnerFuncType: i64}}},
	}
	discardCall := &mt.NodeExprCall{AssociatedFnDef: callee, InfType: i64, ThrowingType: throwingI64}
	discardCaller := &mt.NodeFuncDef{
		ReturnType: void, AbsName: "app.discard_throwing_value", ContextABI: mt.ContextABIContextless,
		Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtExpr{Expression: discardCall}, &mt.NodeStmtRet{Expression: &mt.NodeExprVoid{VoidType: void}, OwnerFuncType: void}}},
	}
	ir, err := llvmobject.ExperimentalIR("throwing-calls.mg", func(backend lb.Backend) error {
		typeLowerer, err := loweringtypes.New(backend, state)
		if err != nil {
			return err
		}
		for _, definition := range []*mt.NodeFuncDef{callee, valueCaller, discardCaller} {
			if _, err = loweringast.LowerScalarFunction(backend, typeLowerer, definition); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(ir)
	if !strings.Contains(text, "call { %type.error, i64 } @app.may_fail()") || !strings.Contains(text, "extractvalue { %type.error, i64 }") {
		t.Fatalf("throwing expression call did not use and project the physical result:\n%s", text)
	}
	discardStart := strings.Index(text, "define internal void @app.discard_throwing_value")
	if discardStart < 0 {
		t.Fatalf("discard caller is missing:\n%s", text)
	}
	discardBody := text[discardStart:]
	if !strings.Contains(discardBody, "call { %type.error, i64 } @app.may_fail()") || strings.Contains(discardBody, "extractvalue") {
		t.Fatalf("statement-position throwing call did not preserve textual discard behavior:\n%s", discardBody)
	}
}

func TestThrowingCallDestructureCapturesExistingErrorBranch(t *testing.T) {
	i64 := primitive("i64")
	throwingI64 := primitive("i64")
	throwingI64.Throws = true
	errorType := &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "std.Error", CoreRole: mt.CoreTypeError}}
	errorDef := &mt.StructDef{
		Module: "std", Name: "Error", CoreRole: mt.CoreTypeError,
		FieldOrder: []string{"__code"}, FieldNb: map[string]int{"__code": 0},
		Fields: map[string]*mt.NodeType{"__code": primitive("u32")},
	}
	callee := &mt.NodeFuncDef{
		ReturnType: throwingI64, AbsName: "app.captured", ContextABI: mt.ContextABIContextless,
		Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtRet{Expression: literal("3", i64), OwnerFuncType: throwingI64}}},
	}
	valueDef := mt.NodeExprVarDef{Name: &mt.NodeNameSingle{Name: "value"}, Type: i64, Storage: mt.VariableStorageLocal}
	errorBinding := mt.NodeExprVarDef{Name: &mt.NodeNameSingle{Name: "err"}, Type: errorType, Storage: mt.VariableStorageLocal}
	call := &mt.NodeExprCall{AssociatedFnDef: callee, InfType: i64, ThrowingType: throwingI64, ErrorMode: 2}
	destructure := &mt.NodeExprDestructureAssign{ValueDef: valueDef, ErrDef: errorBinding, Call: call}
	valueName := &mt.NodeExprName{Name: valueDef.Name, InfType: i64, AssociatedNode: &destructure.ValueDef, Storage: mt.VariableStorageLocal}
	function := &mt.NodeFuncDef{
		ReturnType: i64, AbsName: "app.capture_call", ContextABI: mt.ContextABIContextless,
		Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtExpr{Expression: destructure}, &mt.NodeStmtRet{Expression: valueName, OwnerFuncType: i64}}},
	}
	state := &mt.SharedState{Files: map[string]*mt.FileCtx{}, CoreTypes: map[mt.CoreTypeRole]*mt.StructDef{mt.CoreTypeError: errorDef}}
	ir, err := llvmobject.ExperimentalIR("capture-call.mg", func(backend lb.Backend) error {
		typeLowerer, err := loweringtypes.New(backend, state)
		if err != nil {
			return err
		}
		if _, err = loweringast.LowerScalarFunction(backend, typeLowerer, callee); err != nil {
			return err
		}
		_, err = loweringast.LowerScalarFunction(backend, typeLowerer, function)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(ir)
	for _, expected := range []string{
		"call { %type.error, i64 } @app.captured()",
		"extractvalue %type.error",
		"icmp ne i32",
		"capture.failure.",
		"capture.success.",
		`!{!"branch_weights", i32 1, i32 2000}`,
		"store %type.error",
		"store i64",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("captured throwing call missing %q:\n%s", expected, text)
		}
	}
}

func TestOrdinaryDefersRunLIFOAfterReturnValueEvaluation(t *testing.T) {
	i64 := primitive("i64")
	local := &mt.NodeExprVarDef{Name: &mt.NodeNameSingle{Name: "value"}, Type: i64, Storage: mt.VariableStorageLocal}
	name := func() *mt.NodeExprName {
		return &mt.NodeExprName{Name: local.Name, InfType: i64, AssociatedNode: local, Storage: mt.VariableStorageLocal}
	}
	assign := func(value string) *mt.NodeExprAssign {
		return &mt.NodeExprAssign{Left: name(), Right: literal(value, i64), InfType: i64}
	}
	function := &mt.NodeFuncDef{
		ReturnType: i64, AbsName: "app.defer_lifo", ContextABI: mt.ContextABIContextless,
		Body: mt.NodeBody{Statements: []mt.NodeStatement{
			&mt.NodeStmtExpr{Expression: &mt.NodeExprVarDefAssign{VarDef: local, AssignExpr: literal("1", i64)}},
			&mt.NodeStmtDefer{Expression: assign("2")},
			&mt.NodeStmtDefer{Expression: assign("3")},
			&mt.NodeStmtRet{Expression: name(), OwnerFuncType: i64},
		}},
	}
	ir, err := llvmobject.ExperimentalIR("defer-lifo.mg", func(backend lb.Backend) error {
		typeLowerer, err := loweringtypes.New(backend, &mt.SharedState{Files: map[string]*mt.FileCtx{}, CoreTypes: map[mt.CoreTypeRole]*mt.StructDef{}})
		if err != nil {
			return err
		}
		_, err = loweringast.LowerScalarFunction(backend, typeLowerer, function)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(ir)
	load := strings.LastIndex(text, "load i64")
	storeThree := strings.Index(text, "store i64 3")
	storeTwo := strings.Index(text, "store i64 2")
	ret := strings.LastIndex(text, "ret i64")
	if load < 0 || storeThree < load || storeTwo < storeThree || ret < storeTwo {
		t.Fatalf("defer order or return-value evaluation differs from textual lowering:\n%s", text)
	}
}

func TestTryPropagatesThroughTraceAndErrorCleanups(t *testing.T) {
	i64, u32, u16, pointer := primitive("i64"), primitive("u32"), primitive("u16"), primitive("ptr")
	errorType := &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "core.error", CoreRole: mt.CoreTypeError}}
	stringType := &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "core.str", CoreRole: mt.CoreTypeString}}
	errorDef := &mt.StructDef{
		Module: "core", Name: "error", CoreRole: mt.CoreTypeError,
		FieldOrder: []string{"__message", "__code", "__traceSlot", "__messageLength"},
		FieldNb:    map[string]int{"__message": 0, "__code": 1, "__traceSlot": 2, "__messageLength": 3},
		Fields:     map[string]*mt.NodeType{"__message": pointer, "__code": u32, "__traceSlot": u16, "__messageLength": u16},
	}
	stringDef := &mt.StructDef{
		Module: "core", Name: "str", CoreRole: mt.CoreTypeString,
		FieldOrder: []string{"__data", "__byteCount", "__allocatorImpl", "__allocatorVtable"},
		FieldNb:    map[string]int{"__data": 0, "__byteCount": 1, "__allocatorImpl": 2, "__allocatorVtable": 3},
		Fields:     map[string]*mt.NodeType{"__data": pointer, "__byteCount": i64, "__allocatorImpl": pointer, "__allocatorVtable": pointer},
	}
	siteDef := &mt.StructDef{
		Module: "core", Name: "ErrorTraceSite",
		FieldOrder: []string{"functionData", "functionLength", "fileData", "fileLength", "line", "column"},
		FieldNb:    map[string]int{"functionData": 0, "functionLength": 1, "fileData": 2, "fileLength": 3, "line": 4, "column": 5},
		Fields:     map[string]*mt.NodeType{"functionData": pointer, "functionLength": i64, "fileData": pointer, "fileLength": i64, "line": u32, "column": u32},
	}
	errorArg, errorName := argument("value", errorType)
	push := &mt.NodeFuncDef{
		Class:      mt.NodeGenericClass{ArgsNode: mt.NodeArgList{Args: []mt.NodeArg{{Name: "value", TypeNode: errorType}, {Name: "site", TypeNode: pointer}}}},
		ReturnType: errorType, AbsName: "core.errorTracePush", ContextABI: mt.ContextABIContextless,
		Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtRet{Expression: errorName, OwnerFuncType: errorType}}},
	}
	errorName.AssociatedNode = errorArg
	throwingI64 := primitive("i64")
	throwingI64.Throws = true
	callee := &mt.NodeFuncDef{
		ReturnType: throwingI64, AbsName: "app.may_fail_for_try", ContextABI: mt.ContextABIContextless,
		Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtRet{Expression: literal("5", i64), OwnerFuncType: throwingI64}}},
	}
	marker := &mt.NodeExprVarDef{Name: &mt.NodeNameSingle{Name: "marker"}, Type: i64, Storage: mt.VariableStorageLocal}
	markerName := func() *mt.NodeExprName {
		return &mt.NodeExprName{Name: marker.Name, InfType: i64, AssociatedNode: marker, Storage: mt.VariableStorageLocal}
	}
	call := &mt.NodeExprCall{AssociatedFnDef: callee, InfType: i64, ThrowingType: throwingI64, ErrorMode: 1}
	tried := &mt.NodeExprTry{Call: call, Pos: mt.FilePos{Line: 17, Col: 9}, InfType: i64}
	caller := &mt.NodeFuncDef{
		ReturnType: throwingI64, AbsName: "app.try_caller", DisplayName: "tryCaller", ContextABI: mt.ContextABIContextless,
		Body: mt.NodeBody{Statements: []mt.NodeStatement{
			&mt.NodeStmtExpr{Expression: &mt.NodeExprVarDefAssign{VarDef: marker, AssignExpr: literal("0", i64)}},
			&mt.NodeStmtDefer{OnError: true, Expression: &mt.NodeExprAssign{Left: markerName(), Right: literal("99", i64), InfType: i64}},
			&mt.NodeStmtRet{Expression: tried, OwnerFuncType: throwingI64},
		}},
	}
	throwErrorDef, throwErrorName := argument("err", errorType)
	throwStringDef, throwStringName := argument("message", stringType)
	thrower := &mt.NodeFuncDef{
		Class:      mt.NodeGenericClass{ArgsNode: mt.NodeArgList{Args: []mt.NodeArg{{Name: "err", TypeNode: errorType}, {Name: "message", TypeNode: stringType}}}},
		ReturnType: throwingI64, AbsName: "app.throw_error", DisplayName: "throwError", ContextABI: mt.ContextABIContextless,
		Body: mt.NodeBody{Statements: []mt.NodeStatement{
			&mt.NodeStmtThrow{Expression: throwErrorName, Pos: mt.FilePos{Line: 23, Col: 4}},
			&mt.NodeStmtThrow{Expression: throwStringName, Pos: mt.FilePos{Line: 24, Col: 4}},
			&mt.NodeStmtRet{Expression: literal("8", i64), OwnerFuncType: throwingI64},
		}},
	}
	throwErrorName.AssociatedNode = throwErrorDef
	throwStringName.AssociatedNode = throwStringDef
	functionType := &mt.NodeType{KindNode: &mt.NodeTypeFunc{RetType: throwingI64, ContextABI: mt.ContextABIContextless}}
	functionName := &mt.NodeExprName{Name: &mt.NodeNameSingle{Name: "may_fail_for_try"}, InfType: functionType, AssociatedNode: callee, Storage: mt.VariableStorageSSA}
	indirectCall := &mt.NodeExprCall{Callee: functionName, IsFuncPointer: true, FuncPtrType: functionType, InfType: i64, ThrowingType: throwingI64, ErrorMode: 1}
	indirectTry := &mt.NodeExprTry{Call: indirectCall, Pos: mt.FilePos{Line: 31, Col: 6}, InfType: i64}
	indirectCaller := &mt.NodeFuncDef{
		ReturnType: throwingI64, AbsName: "app.indirect_try", DisplayName: "indirectTry", ContextABI: mt.ContextABIContextless,
		Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtRet{Expression: indirectTry, OwnerFuncType: throwingI64}}},
	}
	coreGlobal := &mt.NodeGlobal{StructDefs: map[string]*mt.StructDef{"error": errorDef, "str": stringDef, "ErrorTraceSite": siteDef}, FuncDefs: map[string]*mt.NodeFuncDef{"errorTracePush": push}}
	appGlobal := &mt.NodeGlobal{FuncDefs: map[string]*mt.NodeFuncDef{"may_fail_for_try": callee, "try_caller": caller, "throw_error": thrower, "indirect_try": indirectCaller}}
	state := &mt.SharedState{
		Files: map[string]*mt.FileCtx{
			"core.mg": {FilePath: "/stdlib/core.mg", ModuleName: "core", GlNode: coreGlobal},
			"app.mg":  {FilePath: "/project/app.mg", ModuleName: "app", GlNode: appGlobal},
		},
		CoreTypes: map[mt.CoreTypeRole]*mt.StructDef{mt.CoreTypeError: errorDef, mt.CoreTypeString: stringDef},
	}
	ir, err := llvmobject.ExperimentalIR("try.mg", func(backend lb.Backend) error {
		typeLowerer, err := loweringtypes.New(backend, state)
		if err != nil {
			return err
		}
		for _, definition := range []*mt.NodeFuncDef{push, callee, caller, thrower, indirectCaller} {
			if _, err = loweringast.LowerScalarFunction(backend, typeLowerer, definition); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(ir)
	for _, expected := range []string{
		"private unnamed_addr constant %struct.core.ErrorTraceSite",
		"ptr @.magma.trace.text.",
		"i32 17, i32 9",
		"icmp ne i32",
		"try.failure.",
		"call %type.error @core.errorTracePush",
		"define %type.error @core.errorTracePush",
		"store i64 99",
		"ret { %type.error, i64 }",
		"throw.failure.",
		"throw.continue.",
		"i32 23, i32 4",
		"icmp ugt i64",
		"select i1",
		"trunc i64",
		"i32 1",
		"define internal { %type.error, i64 } @app.indirect_try()",
		"i32 31, i32 6",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("try propagation missing %q:\n%s", expected, text)
		}
	}
	failure := strings.Index(text, "\ntry.failure.")
	cleanup := strings.Index(text, "store i64 99")
	success := strings.Index(text, "\ntry.success.")
	if failure < 0 || cleanup < failure || success < cleanup {
		t.Fatalf("onerror cleanup is not confined to the failure edge:\n%s", text)
	}
}

func TestMemberDefinitionAndValueReceiverCallUseTextualABI(t *testing.T) {
	i64 := primitive("i64")
	recordDef := &mt.StructDef{Module: "app", Name: "Record", FieldOrder: []string{"value"}, FieldNb: map[string]int{"value": 0}, Fields: map[string]*mt.NodeType{"value": i64}, Funcs: map[string]*mt.NodeFuncDef{}}
	recordType := &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "app.Record"}}
	receiverType := &mt.NodeType{KindNode: &mt.NodeTypePointer{Kind: recordType.KindNode}}
	receiverDef := &mt.NodeExprVarDef{Name: &mt.NodeNameSingle{Name: "this"}, Type: receiverType, Storage: mt.VariableStorageArgument}
	receiverValue := &mt.NodeExprName{
		Name: receiverDef.Name, InfType: i64, AssociatedNode: receiverDef, Storage: mt.VariableStorageArgument,
		MemberAccesses: []*mt.MemberAccess{{OwnerType: receiverType, Type: i64, OwnerDef: recordDef, FieldNb: 0, PtrDeref: true}},
	}
	_, deltaName := argument("delta", i64)
	addition := &mt.NodeExprBinary{Operator: mt.KwPlus, Left: receiverValue, Right: deltaName, OperandType: i64, InfType: i64}
	method := &mt.NodeFuncDef{
		Class:      mt.NodeGenericClass{ArgsNode: mt.NodeArgList{Args: []mt.NodeArg{{Name: "this", TypeNode: receiverType}, {Name: "delta", TypeNode: i64}}}},
		ReturnType: i64, AbsName: "app.Record.add", ContextABI: mt.ContextABIContextless, IsMember: true,
		Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtRet{Expression: addition, OwnerFuncType: i64}}},
	}
	recordDef.Funcs["add"] = method
	owner := &mt.NodeExprStructInit{Type: recordType, Fields: []mt.NodeStructFieldInit{{Name: "value", FieldIndex: 0, FieldType: i64, Expression: literal("4", i64)}}}
	call := &mt.NodeExprCall{AssociatedFnDef: method, IsMemberFunc: true, MemberOwnerExpr: owner, MemberOwnerType: recordType, Args: []mt.NodeExpr{literal("6", i64)}, InfType: i64}
	caller := &mt.NodeFuncDef{
		ReturnType: i64, AbsName: "app.member_caller", ContextABI: mt.ContextABIContextless,
		Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtRet{Expression: call, OwnerFuncType: i64}}},
	}
	global := &mt.NodeGlobal{StructDefs: map[string]*mt.StructDef{"Record": recordDef}, FuncDefs: map[string]*mt.NodeFuncDef{"member_caller": caller}}
	state := &mt.SharedState{Files: map[string]*mt.FileCtx{"app.mg": {ModuleName: "app", GlNode: global}}, CoreTypes: map[mt.CoreTypeRole]*mt.StructDef{}}
	ir, err := llvmobject.ExperimentalIR("member.mg", func(backend lb.Backend) error {
		typeLowerer, err := loweringtypes.New(backend, state)
		if err != nil {
			return err
		}
		for _, definition := range []*mt.NodeFuncDef{method, caller} {
			if _, err = loweringast.LowerScalarFunction(backend, typeLowerer, definition); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(ir)
	for _, expected := range []string{
		"define internal i64 @app.Record.add(ptr %0, i64 %1)",
		"store i64 %1",
		"getelementptr inbounds %struct.app.Record, ptr %0",
		"add i64",
		"alloca %struct.app.Record",
		"call i64 @app.Record.add(ptr",
		"i64 6)",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("member ABI lowering missing %q:\n%s", expected, text)
		}
	}
}

func TestPrototypeViewBuildsTypedPrivateVtable(t *testing.T) {
	i64, pointer := primitive("i64"), primitive("ptr")
	recordDef := &mt.StructDef{Module: "app", Name: "Record", FieldOrder: []string{"value"}, FieldNb: map[string]int{"value": 0}, Fields: map[string]*mt.NodeType{"value": i64}, Funcs: map[string]*mt.NodeFuncDef{}}
	recordType := &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "app.Record"}}
	receiverType := &mt.NodeType{KindNode: &mt.NodeTypePointer{Kind: recordType.KindNode}}
	method := &mt.NodeFuncDef{
		Class:      mt.NodeGenericClass{ArgsNode: mt.NodeArgList{Args: []mt.NodeArg{{Name: "this", TypeNode: receiverType}}}},
		ReturnType: i64, AbsName: "app.Record.get", ContextABI: mt.ContextABIContextless, IsMember: true,
		Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtRet{Expression: literal("7", i64), OwnerFuncType: i64}}},
	}
	recordDef.Funcs["get"] = method
	proto := &mt.ProtoDef{Module: "api", Name: "View", VtableName: "ViewVtable"}
	protoMethod := &mt.ProtoMethod{Name: "get", Ret: i64, ContextABI: mt.ContextABIContextless, Slot: 0, FnDef: method, Proto: proto}
	proto.Methods = []*mt.ProtoMethod{protoMethod}
	proto.MethodMap = map[string]*mt.ProtoMethod{"get": protoMethod}
	viewDef := &mt.StructDef{Module: "api", Name: "View", IsProto: true, Proto: proto, FieldOrder: []string{"implementation", "vtable"}, FieldNb: map[string]int{"implementation": 0, "vtable": 1}, Fields: map[string]*mt.NodeType{"implementation": pointer, "vtable": pointer}}
	vtableDef := &mt.StructDef{Module: "api", Name: "ViewVtable", FieldOrder: []string{"get"}, FieldNb: map[string]int{"get": 0}, Fields: map[string]*mt.NodeType{"get": pointer}}
	viewType := &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "api.View"}}
	viewReceiver := &mt.NodeType{KindNode: &mt.NodeTypePointer{Kind: viewType.KindNode}}
	dispatch := &mt.NodeFuncDef{
		Class:      mt.NodeGenericClass{ArgsNode: mt.NodeArgList{Args: []mt.NodeArg{{Name: "this", TypeNode: viewReceiver}}}},
		ReturnType: i64, AbsName: "api.View.get", ContextABI: mt.ContextABIContextless, IsMember: true, ProtoDispatch: protoMethod,
	}
	viewDef.Funcs = map[string]*mt.NodeFuncDef{"get": dispatch}
	implementation := &mt.ProtoImpl{Type: recordType, Proto: proto, Owner: recordDef}
	recordDef.Implements = []*mt.ProtoImpl{implementation}
	local := &mt.NodeExprVarDef{Name: &mt.NodeNameSingle{Name: "record"}, Type: recordType, Storage: mt.VariableStorageLocal}
	localName := &mt.NodeExprName{Name: local.Name, InfType: recordType, AssociatedNode: local, Storage: mt.VariableStorageLocal}
	view := &mt.NodeExprProtoView{Target: localName, ProtoType: viewType, Implementation: implementation, InfType: viewType}
	maker := &mt.NodeFuncDef{
		ReturnType: viewType, AbsName: "app.make_view", ContextABI: mt.ContextABIContextless,
		Body: mt.NodeBody{Statements: []mt.NodeStatement{
			&mt.NodeStmtExpr{Expression: &mt.NodeExprVarDefAssign{VarDef: local, AssignExpr: &mt.NodeExprStructInit{Type: recordType, Fields: []mt.NodeStructFieldInit{{Name: "value", FieldIndex: 0, FieldType: i64, Expression: literal("2", i64)}}}}},
			&mt.NodeStmtRet{Expression: view, OwnerFuncType: viewType},
		}},
	}
	state := &mt.SharedState{Files: map[string]*mt.FileCtx{
		"app.mg": {ModuleName: "app", GlNode: &mt.NodeGlobal{StructDefs: map[string]*mt.StructDef{"Record": recordDef}, FuncDefs: map[string]*mt.NodeFuncDef{"make_view": maker}}},
		"api.mg": {ModuleName: "api", GlNode: &mt.NodeGlobal{StructDefs: map[string]*mt.StructDef{"View": viewDef, "ViewVtable": vtableDef}}},
	}, CoreTypes: map[mt.CoreTypeRole]*mt.StructDef{}}
	ir, err := llvmobject.ExperimentalIR("proto-view.mg", func(backend lb.Backend) error {
		if err := backend.ConfigureModule(lb.ModuleSpec{SourceFile: "proto-view.mg", TargetTriple: "x86_64-unknown-linux-gnu", DataLayout: "e-p:64:64-i64:64"}); err != nil {
			return err
		}
		typeLowerer, err := loweringtypes.New(backend, state)
		if err != nil {
			return err
		}
		for _, definition := range []*mt.NodeFuncDef{method, maker, dispatch} {
			if _, err = loweringast.LowerScalarFunction(backend, typeLowerer, definition); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(ir)
	for _, expected := range []string{
		"@app.Record.__proto.api.View = private constant %struct.api.ViewVtable { ptr @app.Record.get }",
		"store %struct.app.Record",
		"ptr @app.Record.__proto.api.View",
		"ret %struct.api.View",
		"define internal i64 @api.View.get(ptr %0)",
		"getelementptr inbounds %struct.api.ViewVtable",
		"call i64 %",
		"alwaysinline",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("prototype view lowering missing %q:\n%s", expected, text)
		}
	}
}

func TestContextfulFunctionUsesLeadingPointerWithoutMaterializingReadOnlyContext(t *testing.T) {
	i64 := primitive("i64")
	contextDef := &mt.StructDef{
		Module: "context", Name: "Ctx", FieldOrder: []string{"value"}, FieldNb: map[string]int{"value": 0},
		Fields: map[string]*mt.NodeType{"value": i64},
	}
	contextType := &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "context.Ctx"}}
	implicit := &mt.NodeExprVarDef{Name: &mt.NodeNameSingle{Name: "ctx"}, Type: contextType, Storage: mt.VariableStorageLocal, IsImplicitContext: true}
	argumentDef, argumentName := argument("value", i64)
	definition := &mt.NodeFuncDef{
		Class:      mt.NodeGenericClass{ArgsNode: mt.NodeArgList{Args: []mt.NodeArg{{Name: "value", TypeNode: i64}}}},
		ReturnType: i64, AbsName: "app.contextful", ContextABI: mt.ContextABIContextful, ImplicitContext: implicit,
		Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtRet{Expression: argumentName, OwnerFuncType: i64}}},
	}
	argumentName.AssociatedNode = argumentDef
	callerArgument, callerName := argument("input", i64)
	call := &mt.NodeExprCall{AssociatedFnDef: definition, Args: []mt.NodeExpr{callerName}, InfType: i64}
	caller := &mt.NodeFuncDef{
		Class:      mt.NodeGenericClass{ArgsNode: mt.NodeArgList{Args: []mt.NodeArg{{Name: "input", TypeNode: i64}}}},
		ReturnType: i64, AbsName: "app.contextful_caller", ContextABI: mt.ContextABIContextful,
		ImplicitContext: &mt.NodeExprVarDef{Name: &mt.NodeNameSingle{Name: "ctx"}, Type: contextType, Storage: mt.VariableStorageLocal, IsImplicitContext: true},
		Body:            mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtRet{Expression: call, OwnerFuncType: i64}}},
	}
	callerName.AssociatedNode = callerArgument
	state := &mt.SharedState{Files: map[string]*mt.FileCtx{"context.mg": {GlNode: &mt.NodeGlobal{StructDefs: map[string]*mt.StructDef{"Ctx": contextDef}}}}, CoreTypes: map[mt.CoreTypeRole]*mt.StructDef{}}
	ir, err := llvmobject.ExperimentalIR("contextful.mg", func(backend lb.Backend) error {
		typeLowerer, err := loweringtypes.New(backend, state)
		if err != nil {
			return err
		}
		if _, err = loweringast.LowerScalarFunction(backend, typeLowerer, definition); err != nil {
			return err
		}
		_, err = loweringast.LowerScalarFunction(backend, typeLowerer, caller)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(ir)
	for _, expected := range []string{
		"define internal i64 @app.contextful(ptr %0, i64 %1)",
		"store i64 %1",
		"call i64 @app.contextful(ptr %0",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("contextful lowering missing %q:\n%s", expected, text)
		}
	}
	if strings.Contains(text, "alloca %struct.context.Ctx") || strings.Contains(text, "load %struct.context.Ctx, ptr %0") {
		t.Fatalf("read-only implicit context was materialized:\n%s", text)
	}
}

func TestScalarASTLogicalExpressionPreservesShortCircuitCFG(t *testing.T) {
	boolean := primitive("bool")
	leftDef := &mt.NodeExprVarDef{Name: &mt.NodeNameSingle{Name: "left"}, Type: boolean, Storage: mt.VariableStorageArgument}
	rightDef := &mt.NodeExprVarDef{Name: &mt.NodeNameSingle{Name: "right"}, Type: boolean, Storage: mt.VariableStorageArgument}
	left := &mt.NodeExprName{Name: leftDef.Name, InfType: boolean, AssociatedNode: leftDef, Storage: mt.VariableStorageArgument}
	right := &mt.NodeExprName{Name: rightDef.Name, InfType: boolean, AssociatedNode: rightDef, Storage: mt.VariableStorageArgument}
	logical := &mt.NodeExprBinary{Operator: mt.KwAndAnd, Left: left, Right: right, OperandType: boolean, InfType: boolean}
	definition := &mt.NodeFuncDef{
		Class: mt.NodeGenericClass{ArgsNode: mt.NodeArgList{Args: []mt.NodeArg{
			{Name: "left", TypeNode: boolean}, {Name: "right", TypeNode: boolean},
		}}},
		ReturnType: boolean, AbsName: "app.logical", ContextABI: mt.ContextABIContextless,
		Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtRet{Expression: logical, OwnerFuncType: boolean}}},
	}
	ir, err := llvmobject.ExperimentalIR("logical-ast.mg", func(backend lb.Backend) error {
		typeLowerer, err := loweringtypes.New(backend, &mt.SharedState{Files: map[string]*mt.FileCtx{}, CoreTypes: map[mt.CoreTypeRole]*mt.StructDef{}})
		if err != nil {
			return err
		}
		_, err = loweringast.LowerScalarFunction(backend, typeLowerer, definition)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(ir)
	for _, expected := range []string{"br i1", "logical.rhs.1:", "logical.short.2:", "logical.end.3:", "store i1 false", "load i1"} {
		if !strings.Contains(text, expected) {
			t.Errorf("short-circuit lowering missing %q:\n%s", expected, text)
		}
	}
	entryBranch := strings.Index(text, "br i1")
	if entryBranch < 0 {
		t.Fatalf("logical entry branch is missing:\n%s", text)
	}
	resultSlot := strings.LastIndex(text[:entryBranch], "alloca i1")
	if resultSlot < 0 {
		t.Fatalf("logical result slot was not placed in entry before branching:\n%s", text)
	}
	if strings.Contains(text, " and i1 ") {
		t.Fatalf("logical RHS was made eager instead of preserving textual short-circuiting:\n%s", text)
	}
}

func literal(value string, typ *mt.NodeType) *mt.NodeExprLit {
	return &mt.NodeExprLit{Value: value, LitType: mt.TokLitNum, InfType: typ}
}

func argument(name string, typ *mt.NodeType) (*mt.NodeExprVarDef, *mt.NodeExprName) {
	definition := &mt.NodeExprVarDef{Name: &mt.NodeNameSingle{Name: name}, Type: typ, Storage: mt.VariableStorageArgument}
	return definition, &mt.NodeExprName{Name: definition.Name, InfType: typ, AssociatedNode: definition, Storage: mt.VariableStorageArgument}
}

func TestASTControlFlowStatementsBuildTextualEquivalentCFG(t *testing.T) {
	boolean, i64 := primitive("bool"), primitive("i64")
	_, ifCondition := argument("condition", boolean)
	ifDefinition := &mt.NodeFuncDef{
		Class: mt.NodeGenericClass{ArgsNode: mt.NodeArgList{Args: []mt.NodeArg{{Name: "condition", TypeNode: boolean}}}}, ReturnType: i64, AbsName: "app.choose", ContextABI: mt.ContextABIContextless,
		Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtIf{CondExpr: ifCondition, Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtRet{Expression: literal("1", i64), OwnerFuncType: i64}}}, NextCondStmt: &mt.NodeStmtElse{Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtRet{Expression: literal("2", i64), OwnerFuncType: i64}}}}}}},
	}
	_, whileCondition := argument("condition", boolean)
	whileDefinition := &mt.NodeFuncDef{
		Class: mt.NodeGenericClass{ArgsNode: mt.NodeArgList{Args: []mt.NodeArg{{Name: "condition", TypeNode: boolean}}}}, ReturnType: i64, AbsName: "app.while_break", ContextABI: mt.ContextABIContextless,
		Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtWhile{CondExpr: whileCondition, Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtBreak{}}}}, &mt.NodeStmtRet{Expression: literal("0", i64), OwnerFuncType: i64}}},
	}
	_, continueCondition := argument("condition", boolean)
	continueDefinition := &mt.NodeFuncDef{
		Class: mt.NodeGenericClass{ArgsNode: mt.NodeArgList{Args: []mt.NodeArg{{Name: "condition", TypeNode: boolean}}}}, ReturnType: i64, AbsName: "app.while_continue", ContextABI: mt.ContextABIContextless,
		Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtWhile{CondExpr: continueCondition, Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtContinue{}}}}, &mt.NodeStmtRet{Expression: literal("0", i64), OwnerFuncType: i64}}},
	}
	index := &mt.NodeExprVarDef{Name: &mt.NodeNameSingle{Name: "index"}, Type: i64, Storage: mt.VariableStorageLocal}
	forDefinition := &mt.NodeFuncDef{
		ReturnType: i64, AbsName: "app.for_break", ContextABI: mt.ContextABIContextless,
		Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtFor{DeclExpr: &mt.NodeExprVarDefAssign{VarDef: index, AssignExpr: literal("0", i64)}, BoundExpr: literal("3", i64), Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtBreak{}}}}, &mt.NodeStmtRet{Expression: &mt.NodeExprName{Name: index.Name, InfType: i64, AssociatedNode: index, Storage: mt.VariableStorageLocal}, OwnerFuncType: i64}}},
	}
	_, boundedPredicate := argument("allowed", boolean)
	boundedDefinition := &mt.NodeFuncDef{
		Class: mt.NodeGenericClass{ArgsNode: mt.NodeArgList{Args: []mt.NodeArg{{Name: "allowed", TypeNode: boolean}}}}, ReturnType: i64, AbsName: "app.bounded", ContextABI: mt.ContextABIContextless,
		Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtBounded{Predicates: []mt.NodeExpr{boundedPredicate, boundedPredicate}, Proofs: []*mt.RangeProof{{ID: 1, Guarded: true}}, Body: mt.NodeBody{}}, &mt.NodeStmtRet{Expression: literal("0", i64), OwnerFuncType: i64}}},
	}
	ir, err := llvmobject.ExperimentalIR("control-flow.mg", func(backend lb.Backend) error {
		lowerer, err := loweringtypes.New(backend, &mt.SharedState{Files: map[string]*mt.FileCtx{}, CoreTypes: map[mt.CoreTypeRole]*mt.StructDef{}})
		if err != nil {
			return err
		}
		for _, definition := range []*mt.NodeFuncDef{ifDefinition, whileDefinition, continueDefinition, forDefinition, boundedDefinition} {
			if _, err := loweringast.LowerScalarFunction(backend, lowerer, definition); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(ir)
	for _, expected := range []string{"if.then", "if.else", "while.cond", "while.exit", "for.cond", "for.increment", "for.break.increment"} {
		if !strings.Contains(text, expected) {
			t.Errorf("control-flow lowering missing %q:\n%s", expected, text)
		}
	}
}

func TestASTMatchLowersTagChainAndPayloadBindings(t *testing.T) {
	i64 := primitive("i64")
	representation := &mt.StructDef{Module: "app", Name: "Choice", FieldOrder: []string{"tag", "first", "second"}, FieldNb: map[string]int{"tag": 0, "first": 1, "second": 2}, Fields: map[string]*mt.NodeType{"tag": i64, "first": i64, "second": i64}}
	choiceType := &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "app.Choice"}}
	choice, choiceName := argument("choice", choiceType)
	_ = choice
	owner := &mt.UnionDef{Module: "app", Name: "Choice"}
	firstVariant := &mt.UnionVariant{Name: "first", Tag: 0, Owner: owner}
	secondVariant := &mt.UnionVariant{Name: "second", Tag: 1, Owner: owner}
	firstBinding := &mt.NodeExprVarDef{Name: &mt.NodeNameSingle{Name: "first"}, Type: i64, Storage: mt.VariableStorageLocal}
	secondBinding := &mt.NodeExprVarDef{Name: &mt.NodeNameSingle{Name: "second"}, Type: i64, Storage: mt.VariableStorageLocal}
	returnBinding := func(binding *mt.NodeExprVarDef) mt.NodeStatement {
		return &mt.NodeStmtRet{Expression: &mt.NodeExprName{Name: binding.Name, InfType: i64, AssociatedNode: binding, Storage: mt.VariableStorageLocal}, OwnerFuncType: i64}
	}
	definition := &mt.NodeFuncDef{
		Class: mt.NodeGenericClass{ArgsNode: mt.NodeArgList{Args: []mt.NodeArg{{Name: "choice", TypeNode: choiceType}}}}, ReturnType: i64, AbsName: "app.match", ContextABI: mt.ContextABIContextless,
		Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtMatch{Expression: choiceName, Cases: []*mt.NodeMatchCase{{Variant: firstVariant, Binding: firstBinding, Body: mt.NodeBody{Statements: []mt.NodeStatement{returnBinding(firstBinding)}}}, {Variant: secondVariant, Binding: secondBinding, Body: mt.NodeBody{Statements: []mt.NodeStatement{returnBinding(secondBinding)}}}}, ElseBody: &mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtRet{Expression: literal("0", i64), OwnerFuncType: i64}}}}}},
	}
	state := &mt.SharedState{Files: map[string]*mt.FileCtx{"choice.mg": {GlNode: &mt.NodeGlobal{StructDefs: map[string]*mt.StructDef{"Choice": representation}}}}, CoreTypes: map[mt.CoreTypeRole]*mt.StructDef{}}
	ir, err := llvmobject.ExperimentalIR("match.mg", func(backend lb.Backend) error {
		lowerer, err := loweringtypes.New(backend, state)
		if err != nil {
			return err
		}
		_, err = loweringast.LowerScalarFunction(backend, lowerer, definition)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(ir)
	for _, expected := range []string{"extractvalue %struct.app.Choice", "match.case", "match.next", "icmp eq i64"} {
		if !strings.Contains(text, expected) {
			t.Errorf("match lowering missing %q:\n%s", expected, text)
		}
	}
}

func TestASTDirectAndIndirectCallsShareStructuralDeclarations(t *testing.T) {
	i64 := primitive("i64")
	leftDef, left := argument("left", i64)
	rightDef, right := argument("right", i64)
	_ = leftDef
	_ = rightDef
	add := &mt.NodeFuncDef{
		Class: mt.NodeGenericClass{ArgsNode: mt.NodeArgList{Args: []mt.NodeArg{{Name: "left", TypeNode: i64}, {Name: "right", TypeNode: i64}}}}, ReturnType: i64, AbsName: "app.add.callable", ContextABI: mt.ContextABIContextless,
		Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtRet{Expression: &mt.NodeExprBinary{Operator: mt.KwPlus, Left: left, Right: right, OperandType: i64, InfType: i64}, OwnerFuncType: i64}}},
	}
	directCall := &mt.NodeExprCall{AssociatedFnDef: add, Args: []mt.NodeExpr{literal("20", i64), literal("22", i64)}, InfType: i64}
	direct := &mt.NodeFuncDef{ReturnType: i64, AbsName: "app.direct", ContextABI: mt.ContextABIContextless, Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtRet{Expression: directCall, OwnerFuncType: i64}}}}
	functionTypeKind := &mt.NodeTypeFunc{Args: []*mt.NodeType{i64, i64}, RetType: i64, ContextABI: mt.ContextABIContextless}
	functionType := &mt.NodeType{KindNode: functionTypeKind}
	functionName := &mt.NodeExprName{Name: &mt.NodeNameSingle{Name: "add"}, InfType: functionType, AssociatedNode: add, Storage: mt.VariableStorageSSA}
	indirectCall := &mt.NodeExprCall{Callee: functionName, Args: []mt.NodeExpr{literal("1", i64), literal("2", i64)}, IsFuncPointer: true, FuncPtrType: functionType, InfType: i64}
	indirect := &mt.NodeFuncDef{ReturnType: i64, AbsName: "app.indirect", ContextABI: mt.ContextABIContextless, Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtRet{Expression: indirectCall, OwnerFuncType: i64}}}}
	ir, err := llvmobject.ExperimentalIR("calls.mg", func(backend lb.Backend) error {
		lowerer, err := loweringtypes.New(backend, &mt.SharedState{Files: map[string]*mt.FileCtx{}, CoreTypes: map[mt.CoreTypeRole]*mt.StructDef{}})
		if err != nil {
			return err
		}
		// Lower callers before the definition to exercise declaration-to-definition promotion.
		for _, definition := range []*mt.NodeFuncDef{direct, indirect, add} {
			if _, err := loweringast.LowerScalarFunction(backend, lowerer, definition); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(ir)
	if strings.Count(text, "define internal i64 @app.add.callable") != 1 {
		t.Fatalf("callable was not structurally declared once and then defined:\n%s", text)
	}
	if strings.Count(text, "call i64 @app.add.callable") != 2 {
		t.Fatalf("direct and indirect function-address calls were not emitted:\n%s", text)
	}
}

func TestASTStructFieldsAndProvenSliceIndexing(t *testing.T) {
	i8, i64 := primitive("u8"), primitive("u64")
	record := &mt.StructDef{Module: "app", Name: "Record", FieldOrder: []string{"first", "second"}, FieldNb: map[string]int{"first": 0, "second": 1}, Fields: map[string]*mt.NodeType{"first": i64, "second": i64}}
	recordType := &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "app.Record"}}
	local := &mt.NodeExprVarDef{Name: &mt.NodeNameSingle{Name: "record"}, Type: recordType, Storage: mt.VariableStorageLocal}
	initialize := &mt.NodeExprVarDefAssign{VarDef: local, AssignExpr: &mt.NodeExprStructInit{Type: recordType, Fields: []mt.NodeStructFieldInit{{Name: "first", FieldIndex: 0, FieldType: i64, Expression: literal("1", i64)}, {Name: "second", FieldIndex: 1, FieldType: i64, Expression: literal("2", i64)}}}}
	secondAccess := &mt.MemberAccess{OwnerType: recordType, OwnerDef: record, Type: i64, FieldNb: 1}
	second := func() *mt.NodeExprName {
		return &mt.NodeExprName{Name: local.Name, InfType: i64, AssociatedNode: local, Storage: mt.VariableStorageLocal, MemberAccesses: []*mt.MemberAccess{secondAccess}}
	}
	structFunction := &mt.NodeFuncDef{ReturnType: i64, AbsName: "app.struct_fields", ContextABI: mt.ContextABIContextless, Body: mt.NodeBody{Statements: []mt.NodeStatement{
		&mt.NodeStmtExpr{Expression: initialize},
		&mt.NodeStmtExpr{Expression: &mt.NodeExprAssign{Left: second(), Right: literal("7", i64), InfType: i64}},
		&mt.NodeStmtRet{Expression: second(), OwnerFuncType: i64},
	}}}
	sliceDef := &mt.StructDef{Module: "std", Name: "Slice", CoreRole: mt.CoreTypeSlice, FieldOrder: []string{"__data", "__count"}, FieldNb: map[string]int{"__data": 0, "__count": 1}, Fields: map[string]*mt.NodeType{"__data": primitive("ptr"), "__count": i64}}
	sliceType := &mt.NodeType{KindNode: &mt.NodeTypeSlice{ElemKind: i8.KindNode}}
	_, sliceName := argument("values", sliceType)
	_, indexName := argument("index", i64)
	subscript := &mt.NodeExprSubscript{Target: sliceName, Expr: indexName, IsTargetSsa: false, BoxType: sliceType, ElemType: i8, IndexType: i64, RangeProof: &mt.RangeProof{ID: 1, Guarded: true}}
	sliceFunction := &mt.NodeFuncDef{Class: mt.NodeGenericClass{ArgsNode: mt.NodeArgList{Args: []mt.NodeArg{{Name: "values", TypeNode: sliceType}, {Name: "index", TypeNode: i64}}}}, ReturnType: i8, AbsName: "app.slice_at", ContextABI: mt.ContextABIContextless, Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtRet{Expression: subscript, OwnerFuncType: i8}}}}
	state := &mt.SharedState{Files: map[string]*mt.FileCtx{"record.mg": {GlNode: &mt.NodeGlobal{StructDefs: map[string]*mt.StructDef{"Record": record}}}}, CoreTypes: map[mt.CoreTypeRole]*mt.StructDef{mt.CoreTypeSlice: sliceDef}}
	ir, err := llvmobject.ExperimentalIR("aggregate-expressions.mg", func(backend lb.Backend) error {
		lowerer, err := loweringtypes.New(backend, state)
		if err != nil {
			return err
		}
		for _, definition := range []*mt.NodeFuncDef{structFunction, sliceFunction} {
			if _, err := loweringast.LowerScalarFunction(backend, lowerer, definition); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(ir)
	for _, expected := range []string{"%struct.app.Record { i64 1, i64 2 }", "getelementptr inbounds %struct.app.Record", "getelementptr i8"} {
		if !strings.Contains(text, expected) {
			t.Errorf("aggregate lowering missing %q:\n%s", expected, text)
		}
	}
	if strings.Contains(text, "getelementptr inbounds i8") {
		t.Fatalf("proven slice indexing gained stronger inbounds semantics than textual IR:\n%s", text)
	}
}

func TestASTArraysAndStringsPreserveTextualInitialization(t *testing.T) {
	u8, u64, pointer := primitive("u8"), primitive("u64"), primitive("ptr")
	sliceDef := &mt.StructDef{Module: "std", Name: "Slice", CoreRole: mt.CoreTypeSlice, FieldOrder: []string{"__data", "__count"}, FieldNb: map[string]int{"__data": 0, "__count": 1}, Fields: map[string]*mt.NodeType{"__data": pointer, "__count": u64}}
	stringDef := &mt.StructDef{Module: "std", Name: "String", CoreRole: mt.CoreTypeString, FieldOrder: []string{"__data", "__byteCount", "__allocatorImpl", "__allocatorVtable"}, FieldNb: map[string]int{"__data": 0, "__byteCount": 1, "__allocatorImpl": 2, "__allocatorVtable": 3}, Fields: map[string]*mt.NodeType{"__data": pointer, "__byteCount": u64, "__allocatorImpl": pointer, "__allocatorVtable": pointer}}
	sliceType := &mt.NodeType{KindNode: &mt.NodeTypeSlice{ElemKind: u8.KindNode}}
	array := &mt.NodeExprArray{ElemType: u8, Length: literal("3", u64), LengthType: u64, InfType: sliceType, Entries: []mt.NodeArrayInitEntry{{Value: literal("9", u8), ResolvedIndex: 1}}}
	arrayFunction := &mt.NodeFuncDef{ReturnType: sliceType, AbsName: "app.array", ContextABI: mt.ContextABIContextless, Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtRet{Expression: array, OwnerFuncType: sliceType}}}}
	stringType := primitive("str")
	stringFunction := &mt.NodeFuncDef{ReturnType: stringType, AbsName: "app.string", ContextABI: mt.ContextABIContextless, Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtRet{Expression: &mt.NodeExprLit{Value: "hello", LitType: mt.TokLitStr, InfType: stringType}, OwnerFuncType: stringType}}}}
	state := &mt.SharedState{Files: map[string]*mt.FileCtx{}, CoreTypes: map[mt.CoreTypeRole]*mt.StructDef{mt.CoreTypeSlice: sliceDef, mt.CoreTypeString: stringDef}}
	ir, err := llvmobject.ExperimentalIR("arrays-strings.mg", func(backend lb.Backend) error {
		if err := backend.ConfigureModule(lb.ModuleSpec{SourceFile: "arrays-strings.mg", TargetTriple: "x86_64-unknown-linux-gnu", DataLayout: "e-p:64:64-i64:64"}); err != nil {
			return err
		}
		lowerer, err := loweringtypes.New(backend, state)
		if err != nil {
			return err
		}
		for _, definition := range []*mt.NodeFuncDef{arrayFunction, stringFunction} {
			if _, err := loweringast.LowerScalarFunction(backend, lowerer, definition); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(ir)
	for _, expected := range []string{"alloca i8, i64 3", "call void @llvm.memset.p0.i64", "getelementptr i8", "private unnamed_addr constant [6 x i8] c\"hello\\00\"", "{ ptr, i64, ptr, ptr }"} {
		if !strings.Contains(text, expected) {
			t.Errorf("array/string lowering missing %q:\n%s", expected, text)
		}
	}
}

func TestASTEmbeddedAssetBuildsExternalStorageSlice(t *testing.T) {
	pointer, u64, u8 := primitive("ptr"), primitive("u64"), primitive("u8")
	sliceDef := &mt.StructDef{
		Module: "std", Name: "Slice", CoreRole: mt.CoreTypeSlice,
		FieldOrder: []string{"__data", "__count"}, FieldNb: map[string]int{"__data": 0, "__count": 1},
		Fields: map[string]*mt.NodeType{"__data": pointer, "__count": u64},
	}
	sliceType := &mt.NodeType{KindNode: &mt.NodeTypeSlice{ElemKind: u8.KindNode}}
	embed := &mt.NodeExprEmbed{Path: "payload.bin", Symbol: "magma_embed_payload", Size: 3, InfType: sliceType}
	definition := &mt.NodeFuncDef{
		ReturnType: sliceType, AbsName: "app.asset", ContextABI: mt.ContextABIContextless,
		Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtRet{Expression: embed, OwnerFuncType: sliceType}}},
	}
	state := &mt.SharedState{Files: map[string]*mt.FileCtx{}, CoreTypes: map[mt.CoreTypeRole]*mt.StructDef{mt.CoreTypeSlice: sliceDef}}
	ir, err := llvmobject.ExperimentalIR("embedded-asset.mg", func(backend lb.Backend) error {
		typeLowerer, err := loweringtypes.New(backend, state)
		if err != nil {
			return err
		}
		_, err = loweringast.LowerScalarFunction(backend, typeLowerer, definition)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(ir)
	for _, expected := range []string{"@magma_embed_payload = external constant [3 x i8]", "ptr @magma_embed_payload", "i64 3"} {
		if !strings.Contains(text, expected) {
			t.Errorf("embedded asset lowering missing %q:\n%s", expected, text)
		}
	}
}

func TestASTTypedLLVMDispatcherUsesNeutralOperations(t *testing.T) {
	pointer, u8, u64, void := primitive("ptr"), primitive("u8"), primitive("u64"), primitive("void")
	_, address := argument("address", pointer)
	ptrToInt := &mt.NodeExprLlvm{Operation: "ptrtoint", Args: []mt.NodeExpr{address}, ResultType: u64, InfType: u64}
	convert := &mt.NodeFuncDef{Class: mt.NodeGenericClass{ArgsNode: mt.NodeArgList{Args: []mt.NodeArg{{Name: "address", TypeNode: pointer}}}}, ReturnType: u64, AbsName: "app.ptr_to_int", ContextABI: mt.ContextABIContextless, Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtRet{Expression: ptrToInt, OwnerFuncType: u64}}}}
	_, loadAddress := argument("address", pointer)
	load := &mt.NodeExprLlvm{Operation: "load_volatile", Args: []mt.NodeExpr{loadAddress, literal("1", u64)}, ResultType: u8, InfType: u8}
	loadFunction := &mt.NodeFuncDef{Class: mt.NodeGenericClass{ArgsNode: mt.NodeArgList{Args: []mt.NodeArg{{Name: "address", TypeNode: pointer}}}}, ReturnType: u8, AbsName: "app.volatile_load", ContextABI: mt.ContextABIContextless, Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtRet{Expression: load, OwnerFuncType: u8}}}}
	_, storeAddress := argument("address", pointer)
	_, storeValue := argument("value", u8)
	store := &mt.NodeExprLlvm{Operation: "store_volatile", Args: []mt.NodeExpr{storeAddress, storeValue, literal("1", u64)}, ResultType: void, InfType: void}
	storeFunction := &mt.NodeFuncDef{Class: mt.NodeGenericClass{ArgsNode: mt.NodeArgList{Args: []mt.NodeArg{{Name: "address", TypeNode: pointer}, {Name: "value", TypeNode: u8}}}}, ReturnType: void, AbsName: "app.volatile_store", ContextABI: mt.ContextABIContextless, Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtExpr{Expression: store}, &mt.NodeStmtRet{Expression: &mt.NodeExprVoid{VoidType: void}, OwnerFuncType: void}}}}
	ir, err := llvmobject.ExperimentalIR("typed-llvm.mg", func(backend lb.Backend) error {
		lowerer, err := loweringtypes.New(backend, &mt.SharedState{Files: map[string]*mt.FileCtx{}, CoreTypes: map[mt.CoreTypeRole]*mt.StructDef{}})
		if err != nil {
			return err
		}
		for _, definition := range []*mt.NodeFuncDef{convert, loadFunction, storeFunction} {
			if _, err := loweringast.LowerScalarFunction(backend, lowerer, definition); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(ir)
	for _, expected := range []string{"ptrtoint ptr", "load volatile i8", "store volatile i8"} {
		if !strings.Contains(text, expected) {
			t.Errorf("typed LLVM lowering missing %q:\n%s", expected, text)
		}
	}
	if strings.Contains(text, "@llvm(\"") {
		t.Fatalf("directive text leaked into object IR:\n%s", text)
	}
}

func TestASTTypedLLVMAtomicRecipesMatchTextualInstructions(t *testing.T) {
	pointer, u64, void := primitive("ptr"), primitive("u64"), primitive("void")
	config := func(value string) mt.NodeExpr {
		return &mt.NodeExprLit{Value: value, LitType: mt.TokLitStr, InfType: primitive("str")}
	}
	_, loadPointer := argument("pointer", pointer)
	load := &mt.NodeExprLlvm{Operation: "atomic_load", Args: []mt.NodeExpr{loadPointer, config("acquire"), literal("8", u64)}, ResultType: u64, InfType: u64}
	loadFunction := &mt.NodeFuncDef{Class: mt.NodeGenericClass{ArgsNode: mt.NodeArgList{Args: []mt.NodeArg{{Name: "pointer", TypeNode: pointer}}}}, ReturnType: u64, AbsName: "app.atomic_load", ContextABI: mt.ContextABIContextless, Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtRet{Expression: load, OwnerFuncType: u64}}}}
	_, operationPointer := argument("pointer", pointer)
	_, expected := argument("expected", u64)
	_, desired := argument("desired", u64)
	store := &mt.NodeExprLlvm{Operation: "atomic_store", Args: []mt.NodeExpr{operationPointer, desired, config("release"), literal("8", u64)}, ResultType: void, InfType: void}
	rmw := &mt.NodeExprLlvm{Operation: "atomic_rmw", Args: []mt.NodeExpr{operationPointer, desired, config("add"), config("acq_rel"), literal("8", u64)}, ResultType: u64, InfType: u64}
	compareExchange := &mt.NodeExprLlvm{Operation: "cmpxchg_old", Args: []mt.NodeExpr{operationPointer, expected, desired, config("seq_cst"), config("acquire"), literal("8", u64)}, ResultType: u64, InfType: u64}
	operations := &mt.NodeFuncDef{Class: mt.NodeGenericClass{ArgsNode: mt.NodeArgList{Args: []mt.NodeArg{{Name: "pointer", TypeNode: pointer}, {Name: "expected", TypeNode: u64}, {Name: "desired", TypeNode: u64}}}}, ReturnType: u64, AbsName: "app.atomic_ops", ContextABI: mt.ContextABIContextless, Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtExpr{Expression: store}, &mt.NodeStmtExpr{Expression: rmw}, &mt.NodeStmtRet{Expression: compareExchange, OwnerFuncType: u64}}}}
	ir, err := llvmobject.ExperimentalIR("atomics.mg", func(backend lb.Backend) error {
		lowerer, err := loweringtypes.New(backend, &mt.SharedState{Files: map[string]*mt.FileCtx{}, CoreTypes: map[mt.CoreTypeRole]*mt.StructDef{}})
		if err != nil {
			return err
		}
		for _, definition := range []*mt.NodeFuncDef{loadFunction, operations} {
			if _, err := loweringast.LowerScalarFunction(backend, lowerer, definition); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(ir)
	for _, expected := range []string{"load atomic i64", "acquire, align 8", "store atomic i64", "release, align 8", "atomicrmw add", "acq_rel, align 8", "cmpxchg ptr", "seq_cst acquire, align 8", "extractvalue { i64, i1 }"} {
		if !strings.Contains(text, expected) {
			t.Errorf("atomic lowering missing %q:\n%s", expected, text)
		}
	}
}

func TestASTTypedLLVMIntrinsicAndSideEffectRecipes(t *testing.T) {
	u64, boolean, void := primitive("u64"), primitive("bool"), primitive("void")
	_, input := argument("input", u64)
	population := &mt.NodeExprLlvm{Operation: "ctpop", Args: []mt.NodeExpr{input}, ResultType: u64, InfType: u64}
	bitFunction := &mt.NodeFuncDef{Class: mt.NodeGenericClass{ArgsNode: mt.NodeArgList{Args: []mt.NodeArg{{Name: "input", TypeNode: u64}}}}, ReturnType: u64, AbsName: "app.population", ContextABI: mt.ContextABIContextless, Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtRet{Expression: population, OwnerFuncType: u64}}}}
	_, condition := argument("condition", boolean)
	assume := &mt.NodeExprLlvm{Operation: "assume", Args: []mt.NodeExpr{condition}, ResultType: void, InfType: void}
	sideEffect := &mt.NodeExprLlvm{Operation: "sideeffect", ResultType: void, InfType: void}
	trap := &mt.NodeExprLlvm{Operation: "trap", ResultType: void, InfType: void}
	assembly := &mt.NodeExprLlvm{Operation: "asm_sideeffect", Args: []mt.NodeExpr{&mt.NodeExprLit{Value: "pause", LitType: mt.TokLitStr, InfType: primitive("str")}}, ResultType: void, InfType: void}
	barrierFunction := &mt.NodeFuncDef{Class: mt.NodeGenericClass{ArgsNode: mt.NodeArgList{Args: []mt.NodeArg{{Name: "condition", TypeNode: boolean}}}}, ReturnType: void, AbsName: "app.barriers", ContextABI: mt.ContextABIContextless, Body: mt.NodeBody{Statements: []mt.NodeStatement{&mt.NodeStmtExpr{Expression: assume}, &mt.NodeStmtExpr{Expression: sideEffect}, &mt.NodeStmtExpr{Expression: assembly}, &mt.NodeStmtExpr{Expression: trap}, &mt.NodeStmtRet{Expression: &mt.NodeExprVoid{VoidType: void}, OwnerFuncType: void}}}}
	ir, err := llvmobject.ExperimentalIR("intrinsics.mg", func(backend lb.Backend) error {
		if err := backend.ConfigureModule(lb.ModuleSpec{SourceFile: "intrinsics.mg", TargetTriple: "x86_64-unknown-linux-gnu"}); err != nil {
			return err
		}
		lowerer, err := loweringtypes.New(backend, &mt.SharedState{Files: map[string]*mt.FileCtx{}, CoreTypes: map[mt.CoreTypeRole]*mt.StructDef{}})
		if err != nil {
			return err
		}
		for _, definition := range []*mt.NodeFuncDef{bitFunction, barrierFunction} {
			if _, err := loweringast.LowerScalarFunction(backend, lowerer, definition); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(ir)
	for _, expected := range []string{"call i64 @llvm.ctpop.i64", "call void @llvm.assume", "call void @llvm.sideeffect", "asm sideeffect \"pause\", \"~{memory}\"", "call void @llvm.trap"} {
		if !strings.Contains(text, expected) {
			t.Errorf("intrinsic lowering missing %q:\n%s", expected, text)
		}
	}
}
