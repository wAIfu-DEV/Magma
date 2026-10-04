// Package loweringast translates checked Magma nodes into backend-neutral
// operations. It is intentionally incremental; unsupported nodes fail instead
// of falling back to textual IR.
package loweringast

import (
	"crypto/sha256"
	"fmt"
	"math/big"
	"path/filepath"
	"strconv"
	"strings"

	lb "Magma/src/lowering_backend"
	loweringcabi "Magma/src/lowering_cabi"
	loweringtypes "Magma/src/lowering_types"
	magmatypes "Magma/src/magma_types"
	t "Magma/src/types"
)

type scalarFunction struct {
	backend      lb.Backend
	types        *loweringtypes.Lowerer
	fn           lb.FunctionID
	entry        lb.BlockID
	block        lb.BlockID
	slots        map[*t.NodeExprVarDef]lb.ValueID
	args         map[string]lb.ValueID
	directArgs   map[string]bool
	globals      map[*t.NodeExprVarDef]lb.ValueID
	blocks       uint64
	loops        []loopTargets
	context      lb.ValueID
	cleanups     [][]*t.NodeStmtDefer
	inCleanup    int
	cleanupDepth int
	cleanupIndex int
	definition   *t.NodeFuncDef
}

type loopTargets struct {
	continueBlock lb.BlockID
	breakBlock    lb.BlockID
	cleanupDepth  int
}

// LowerScalarFunction lowers the first production-shaped AST subset. The
// accepted function must be non-member. Successful returns from
// throwing functions use the textual ABI envelope; cleanup and error-flow
// statements remain outside this scalar/control-flow stage.
func LowerScalarFunction(backend lb.Backend, typeLowerer *loweringtypes.Lowerer, definition *t.NodeFuncDef) (lb.FunctionID, error) {
	return LowerFunctionWithGlobals(backend, typeLowerer, definition, nil)
}

// LowerFunctionWithGlobals lowers a checked function with the program-owned
// address identities of resolved global variables.
func LowerFunctionWithGlobals(backend lb.Backend, typeLowerer *loweringtypes.Lowerer, definition *t.NodeFuncDef, globals map[*t.NodeExprVarDef]lb.ValueID) (lb.FunctionID, error) {
	return lowerFunctionWithGlobals(backend, typeLowerer, definition, globals, true)
}

// DeclareFunction materializes the ABI-exact declaration used by independent
// units and by later definition lowering.
func DeclareFunction(backend lb.Backend, typeLowerer *loweringtypes.Lowerer, definition *t.NodeFuncDef) (lb.FunctionID, error) {
	return lowerFunctionWithGlobals(backend, typeLowerer, definition, nil, false)
}

func lowerFunctionWithGlobals(backend lb.Backend, typeLowerer *loweringtypes.Lowerer, definition *t.NodeFuncDef, globals map[*t.NodeExprVarDef]lb.ValueID, define bool) (lb.FunctionID, error) {
	if backend == nil || typeLowerer == nil || definition == nil || definition.ReturnType == nil {
		return 0, fmt.Errorf("scalar function lowering requires a backend, type lowerer, and complete function")
	}
	if definition.IsExternal || definition.NoAliasName != "" {
		function, _, _, err := declareExternalFunction(backend, typeLowerer, definition)
		return function, err
	}
	result, err := typeLowerer.Lower(definition.ReturnType)
	if err != nil {
		return 0, err
	}
	parameterOffset := 0
	parameters := make([]lb.TypeID, 0, len(definition.Class.ArgsNode.Args)+1)
	if definition.ContextABI == t.ContextABIContextful {
		parameterOffset = 1
		pointer, err := backend.InternType(lb.TypeSpec{Kind: lb.TypePointer})
		if err != nil {
			return 0, err
		}
		parameters = append(parameters, pointer)
	}
	argumentStart := 0
	if definition.IsMember {
		argumentStart = 1
		pointer, err := backend.InternType(lb.TypeSpec{Kind: lb.TypePointer})
		if err != nil {
			return 0, err
		}
		parameters = append(parameters, pointer)
	}
	for i := argumentStart; i < len(definition.Class.ArgsNode.Args); i++ {
		argument := definition.Class.ArgsNode.Args[i]
		parameter, err := typeLowerer.Lower(argument.TypeNode)
		if err != nil {
			return 0, fmt.Errorf("argument %q: %w", argument.Name, err)
		}
		parameters = append(parameters, parameter)
	}
	symbol := definition.AbsName
	if symbol == "" {
		return 0, fmt.Errorf("function has no resolved symbol")
	}
	attributes, err := functionAttributes(typeLowerer, definition)
	if err != nil {
		return 0, err
	}
	linkage := lb.LinkageInternal
	if typeLowerer.IsTracePush(definition) {
		// Keep the canonical error return type as an ABI boundary. Internal
		// functions are eligible for LLVM return promotion, which otherwise
		// rewrites this helper to an anonymous aggregate at higher opt levels.
		linkage = lb.LinkageExternal
	}
	if typeLowerer.CrossModule || definition.IsExternal || definition.NoAliasName != "" {
		linkage = lb.LinkageExternal
		if definition.NoAliasName != "" {
			symbol = definition.NoAliasName
		}
	}
	function, err := backend.DeclareFunction(lb.FunctionSpec{Symbol: symbol, Result: result, Parameters: parameters, Linkage: linkage, CallingConvention: lb.CallingConventionC, Attributes: attributes, Definition: define && !definition.IsExternal})
	if err != nil {
		return 0, err
	}
	if !define || definition.IsExternal {
		return function, nil
	}
	entry, err := backend.AppendBlock(function, "entry")
	if err != nil {
		return 0, err
	}
	if definition.ProtoDispatch != nil {
		if err := lowerProtoDispatch(backend, typeLowerer, definition, function, entry, result, parameters); err != nil {
			return 0, err
		}
		if err := backend.FinalizeFunction(function); err != nil {
			return 0, err
		}
		return function, nil
	}
	context := &scalarFunction{backend: backend, types: typeLowerer, fn: function, entry: entry, block: entry, slots: make(map[*t.NodeExprVarDef]lb.ValueID), args: make(map[string]lb.ValueID), directArgs: make(map[string]bool), globals: globals, definition: definition}
	if definition.ImplicitContext != nil {
		if definition.ImplicitContext.Type == nil {
			return 0, fmt.Errorf("implicit context has no resolved type")
		}
		if _, err := typeLowerer.Lower(definition.ImplicitContext.Type); err != nil {
			return 0, fmt.Errorf("implicit context: %w", err)
		}
		if definition.ContextABI == t.ContextABIContextful {
			contextPointer, err := backend.Parameter(function, 0)
			if err != nil {
				return 0, err
			}
			context.context = contextPointer
		}
		if definition.ImplicitContextMutable {
			if err := context.materializeContext(definition.ImplicitContext); err != nil {
				return 0, err
			}
		}
	}
	if definition.IsMember {
		receiver := definition.Class.ArgsNode.Args[0]
		context.args[receiver.Name], err = backend.Parameter(function, parameterOffset)
		if err != nil {
			return 0, err
		}
		context.directArgs[receiver.Name] = true
	}
	for i := argumentStart; i < len(definition.Class.ArgsNode.Args); i++ {
		argument := definition.Class.ArgsNode.Args[i]
		context.args[argument.Name], err = typeLowerer.MaterializeArgument(function, entry, i+parameterOffset, argument.TypeNode)
		if err != nil {
			return 0, err
		}
	}
	terminated, err := context.statements(definition.Body.Statements, definition.ReturnType)
	if err != nil {
		return 0, err
	}
	if !terminated {
		ordinary := *definition.ReturnType
		ordinary.Throws = false
		if name, ok := scalarTypeName(&ordinary); ok && name == "void" && !definition.ReturnType.Throws {
			if err := backend.ReturnVoid(context.block); err != nil {
				return 0, err
			}
		} else {
			zero, err := backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantZero, Type: result})
			if err != nil {
				return 0, err
			}
			value, err := backend.ConstantValue(zero)
			if err != nil {
				return 0, err
			}
			if err := backend.Return(context.block, value); err != nil {
				return 0, err
			}
		}
	}
	if err := backend.FinalizeFunction(function); err != nil {
		return 0, err
	}
	return function, nil
}

func lowerProtoDispatch(backend lb.Backend, types *loweringtypes.Lowerer, definition *t.NodeFuncDef, function lb.FunctionID, entry lb.BlockID, result lb.TypeID, parameters []lb.TypeID) error {
	method := definition.ProtoDispatch
	if method == nil || method.Proto == nil || method.Slot < 0 || len(definition.Class.ArgsNode.Args) == 0 {
		return fmt.Errorf("prototype dispatch wrapper lacks method metadata")
	}
	contextOffset := 0
	arguments := make([]lb.ValueID, 0, len(parameters))
	if definition.ContextABI == t.ContextABIContextful {
		contextOffset = 1
		context, err := backend.Parameter(function, 0)
		if err != nil {
			return err
		}
		arguments = append(arguments, context)
	}
	receiver, err := backend.Parameter(function, contextOffset)
	if err != nil {
		return err
	}
	viewType := aggregateOwner(definition.Class.ArgsNode.Args[0].TypeNode)
	view, err := types.Lower(viewType)
	if err != nil {
		return err
	}
	pointer, err := backend.InternType(lb.TypeSpec{Kind: lb.TypePointer})
	if err != nil {
		return err
	}
	implementation, err := backend.StructFieldAddress(entry, view, receiver, 2)
	if err != nil {
		return err
	}
	vtableAddress, err := backend.StructFieldAddress(entry, view, receiver, 0)
	if err != nil {
		return err
	}
	vtable, err := backend.Load(entry, pointer, vtableAddress, 0, false)
	if err != nil {
		return err
	}
	vtableType, err := types.Lower(&t.NodeType{KindNode: &t.NodeTypeAbsolute{AbsoluteName: method.Proto.Module + "." + method.Proto.VtableName}})
	if err != nil {
		return err
	}
	slotAddress, err := backend.StructFieldAddress(entry, vtableType, vtable, uint32(method.Slot))
	if err != nil {
		return err
	}
	callee, err := backend.Load(entry, pointer, slotAddress, 0, false)
	if err != nil {
		return err
	}
	arguments = append(arguments, implementation)
	for index := 1; index < len(definition.Class.ArgsNode.Args); index++ {
		value, err := backend.Parameter(function, index+contextOffset)
		if err != nil {
			return err
		}
		arguments = append(arguments, value)
	}
	signature, err := backend.InternFunctionType(lb.FunctionTypeSpec{Result: result, Parameters: parameters})
	if err != nil {
		return err
	}
	call, err := backend.IndirectCall(entry, signature, callee, arguments, lb.CallSpec{CallingConvention: lb.CallingConventionC})
	if err != nil {
		return err
	}
	ordinary := *definition.ReturnType
	ordinary.Throws = false
	name, named := scalarTypeName(&ordinary)
	if named && name == "void" && !definition.ReturnType.Throws {
		return backend.ReturnVoid(entry)
	}
	return backend.Return(entry, call)
}

func (c *scalarFunction) statements(statements []t.NodeStatement, returnType *t.NodeType) (bool, error) {
	depth := len(c.cleanups)
	c.cleanups = append(c.cleanups, nil)
	defer func() { c.cleanups = c.cleanups[:depth] }()
	terminated := false
	for _, statement := range statements {
		if terminated {
			return false, fmt.Errorf("statement follows a terminating control-flow statement")
		}
		var err error
		switch node := statement.(type) {
		case *t.NodeStmtExpr:
			if call, ok := node.Expression.(*t.NodeExprCall); ok {
				_, err = c.call(call, true, false)
			} else {
				_, err = c.expression(node.Expression, node.Expression.GetInferredType())
			}
		case *t.NodeStmtRet:
			terminated, err = c.returnStatement(node, returnType)
		case *t.NodeStmtThrow:
			err = c.throwStatement(node)
		case *t.NodeStmtIf:
			terminated, err = c.ifStatement(node, returnType)
		case *t.NodeStmtWhile:
			err = c.whileStatement(node, returnType)
		case *t.NodeStmtFor:
			err = c.forStatement(node, returnType)
		case *t.NodeStmtBounded:
			terminated, err = c.boundedStatement(node, returnType)
		case *t.NodeStmtUnsafe:
			terminated, err = c.statements(node.Body.Statements, returnType)
		case *t.NodeStmtBreak:
			terminated, err = c.loopBranch(false)
		case *t.NodeStmtContinue:
			terminated, err = c.loopBranch(true)
		case *t.NodeStmtMatch:
			terminated, err = c.matchStatement(node, returnType)
		case *t.NodeStmtDefer:
			c.cleanups[depth] = append(c.cleanups[depth], node)
		case *t.NodeLlvm:
			terminated, err = c.legacyLLVMStatement(node, returnType)
		default:
			err = fmt.Errorf("unsupported scalar statement %T", statement)
		}
		if err != nil {
			return false, err
		}
	}
	if !terminated {
		if err := c.runCleanups(depth, false); err != nil {
			return false, err
		}
	}
	return terminated, nil
}

// legacyLLVMStatement keeps the pre-object one-line return form working while
// inline operations migrate to typed @llvm objects. It deliberately does not
// parse general LLVM IR fragments.
func (c *scalarFunction) legacyLLVMStatement(node *t.NodeLlvm, returnType *t.NodeType) (bool, error) {
	fields := strings.Fields(node.Text)
	if len(fields) == 2 && fields[0] == "ret" && fields[1] == "void" {
		return true, c.backend.ReturnVoid(c.block)
	}
	if len(fields) != 3 || fields[0] != "ret" || !strings.HasPrefix(fields[2], "%") {
		return false, fmt.Errorf("legacy inline LLVM supports only a single return instruction; migrate this fragment to @llvm")
	}
	name := strings.TrimPrefix(fields[2], "%")
	storage := c.args[name]
	if storage == 0 {
		return false, fmt.Errorf("legacy inline LLVM return references unknown parameter %q", name)
	}
	ordinary := *returnType
	ordinary.Throws = false
	typeID, err := c.types.Lower(&ordinary)
	if err != nil {
		return false, err
	}
	value, err := c.backend.Load(c.block, typeID, storage, 0, false)
	if err != nil {
		return false, err
	}
	return true, c.backend.Return(c.block, value)
}

func (c *scalarFunction) throwStatement(node *t.NodeStmtThrow) error {
	if node == nil || node.Expression == nil || c.definition == nil || c.definition.ReturnType == nil || !c.definition.ReturnType.Throws {
		return fmt.Errorf("throw requires a throwing function and error expression")
	}
	role := t.CoreTypeRoleOf(node.Expression.GetInferredType())
	var errorValue lb.ValueID
	var err error
	if role == t.CoreTypeString {
		errorValue, err = c.errorFromString(node.Expression)
	} else if role == t.CoreTypeError {
		errorValue, err = c.expression(node.Expression, node.Expression.GetInferredType())
	} else {
		return fmt.Errorf("throw requires an error or string expression")
	}
	if err != nil {
		return err
	}
	return c.emitThrowError(errorValue, node.Pos)
}

func (c *scalarFunction) errorFromString(expression t.NodeExpr) (lb.ValueID, error) {
	value, err := c.expression(expression, expression.GetInferredType())
	if err != nil {
		return 0, err
	}
	message, err := c.types.ExtractCoreField(c.block, value, t.CoreTypeString, "__data")
	if err != nil {
		return 0, err
	}
	length, err := c.types.ExtractCoreField(c.block, value, t.CoreTypeString, "__byteCount")
	if err != nil {
		return 0, err
	}
	lengthType, err := c.types.CoreFieldType(t.CoreTypeString, "__byteCount")
	if err != nil {
		return 0, err
	}
	maximumConstant, err := c.backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: lengthType, Integer: "65535"})
	if err != nil {
		return 0, err
	}
	maximum, err := c.backend.ConstantValue(maximumConstant)
	if err != nil {
		return 0, err
	}
	tooLong, err := c.backend.Compare(c.block, lb.CompareUnsignedGreater, length, maximum)
	if err != nil {
		return 0, err
	}
	boundedLength, err := c.backend.Select(c.block, tooLong, maximum, length)
	if err != nil {
		return 0, err
	}
	messageLengthType, err := c.types.CoreFieldType(t.CoreTypeError, "__messageLength")
	if err != nil {
		return 0, err
	}
	messageLength, err := c.backend.Cast(c.block, lb.CastTruncate, boundedLength, messageLengthType)
	if err != nil {
		return 0, err
	}
	codeType, err := c.types.CoreFieldType(t.CoreTypeError, "__code")
	if err != nil {
		return 0, err
	}
	codeConstant, err := c.backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: codeType, Integer: "1"})
	if err != nil {
		return 0, err
	}
	code, err := c.backend.ConstantValue(codeConstant)
	if err != nil {
		return 0, err
	}
	return c.types.BuildCoreValueWithDefaults(c.block, t.CoreTypeError, map[string]lb.ValueID{"__message": message, "__code": code, "__messageLength": messageLength})
}

func (c *scalarFunction) emitThrowError(errorValue lb.ValueID, position t.FilePos) error {
	code, err := c.types.ExtractCoreField(c.block, errorValue, t.CoreTypeError, "__code")
	if err != nil {
		return err
	}
	codeType, err := c.types.CoreFieldType(t.CoreTypeError, "__code")
	if err != nil {
		return err
	}
	zeroConstant, err := c.backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantZero, Type: codeType})
	if err != nil {
		return err
	}
	zero, err := c.backend.ConstantValue(zeroConstant)
	if err != nil {
		return err
	}
	failed, err := c.backend.Compare(c.block, lb.CompareNotEqual, code, zero)
	if err != nil {
		return err
	}
	failure, err := c.newBlock("throw.failure")
	if err != nil {
		return err
	}
	continuation, err := c.newBlock("throw.continue")
	if err != nil {
		return err
	}
	if err := c.backend.CondBranchWeighted(c.block, failed, failure, continuation, lb.UnlikelyThen); err != nil {
		return err
	}
	c.block = failure
	site, push, err := c.traceSite(position)
	if err != nil {
		return err
	}
	tracedError, err := c.backend.Call(c.block, push, []lb.ValueID{errorValue, site})
	if err != nil {
		return err
	}
	failureValue, err := c.types.BuildThrowingFailure(c.block, c.definition.ReturnType, tracedError)
	if err != nil {
		return err
	}
	if err := c.runErrorCleanups(); err != nil {
		return err
	}
	if err := c.backend.Return(c.block, failureValue); err != nil {
		return err
	}
	c.block = continuation
	return nil
}

func (c *scalarFunction) returnStatement(node *t.NodeStmtRet, returnType *t.NodeType) (bool, error) {
	if c.inCleanup != 0 {
		return false, fmt.Errorf("terminating control flow inside defer lowering is not yet supported")
	}
	if _, ok := node.Expression.(*t.NodeExprVoid); ok {
		if returnType.Throws {
			value, err := c.types.BuildThrowingSuccess(c.block, returnType, 0)
			if err != nil {
				return false, err
			}
			if err := c.runCleanups(0, false); err != nil {
				return false, err
			}
			return true, c.backend.Return(c.block, value)
		}
		if err := c.runCleanups(0, false); err != nil {
			return false, err
		}
		return true, c.backend.ReturnVoid(c.block)
	}
	ordinaryReturn := *returnType
	ordinaryReturn.Throws = false
	value, err := c.expression(node.Expression, &ordinaryReturn)
	if err != nil {
		return false, err
	}
	value, err = c.coerce(value, node.Expression.GetInferredType(), &ordinaryReturn)
	if err != nil {
		return false, err
	}
	if returnType.Throws {
		value, err = c.types.BuildThrowingSuccess(c.block, returnType, value)
		if err != nil {
			return false, err
		}
	}
	if err := c.runCleanups(0, false); err != nil {
		return false, err
	}
	return true, c.backend.Return(c.block, value)
}

func (c *scalarFunction) runCleanups(first int, onError bool) error {
	depth := len(c.cleanups) - 1
	index := -1
	if depth >= first {
		index = len(c.cleanups[depth]) - 1
	}
	return c.runCleanupRange(first, depth, index, onError)
}

func (c *scalarFunction) runErrorCleanups() error {
	if c.inCleanup == 0 {
		return c.runCleanups(0, true)
	}
	return c.runCleanupRange(0, c.cleanupDepth, c.cleanupIndex-1, true)
}

func (c *scalarFunction) runCleanupRange(first, startDepth, startIndex int, onError bool) error {
	previousDepth, previousIndex := c.cleanupDepth, c.cleanupIndex
	defer func() {
		c.cleanupDepth, c.cleanupIndex = previousDepth, previousIndex
	}()
	for depth := startDepth; depth >= first; depth-- {
		deferred := c.cleanups[depth]
		last := len(deferred) - 1
		if depth == startDepth {
			last = startIndex
		}
		for index := last; index >= 0; index-- {
			if deferred[index].OnError && !onError {
				continue
			}
			c.cleanupDepth, c.cleanupIndex = depth, index
			if err := c.runDeferred(deferred[index]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *scalarFunction) runDeferred(deferred *t.NodeStmtDefer) error {
	if deferred == nil {
		return fmt.Errorf("incomplete defer statement")
	}
	c.inCleanup++
	defer func() { c.inCleanup-- }()
	if deferred.IsBody {
		terminated, err := c.statements(deferred.Body.Statements, nil)
		if err != nil {
			return err
		}
		if terminated {
			return fmt.Errorf("terminating defer body is not yet supported")
		}
		return nil
	}
	if deferred.Expression == nil {
		return fmt.Errorf("defer statement has no expression")
	}
	_, err := c.expression(deferred.Expression, deferred.Expression.GetInferredType())
	return err
}

func (c *scalarFunction) ifStatement(node *t.NodeStmtIf, returnType *t.NodeType) (bool, error) {
	condition, err := c.expression(node.CondExpr, node.CondExpr.GetInferredType())
	if err != nil {
		return false, err
	}
	thenBlock, err := c.newBlock("if.then")
	if err != nil {
		return false, err
	}
	elseBlock, err := c.newBlock("if.else")
	if err != nil {
		return false, err
	}
	if weights, ok := errorPredicateBranchWeights(node.CondExpr); ok {
		err = c.backend.CondBranchWeighted(c.block, condition, thenBlock, elseBlock, weights)
	} else {
		err = c.backend.CondBranch(c.block, condition, thenBlock, elseBlock)
	}
	if err != nil {
		return false, err
	}
	c.block = thenBlock
	thenTerminated, err := c.statements(node.Body.Statements, returnType)
	if err != nil {
		return false, err
	}
	thenExit := c.block
	c.block = elseBlock
	elseTerminated := false
	if node.NextCondStmt != nil {
		switch next := node.NextCondStmt.(type) {
		case *t.NodeStmtIf:
			elseTerminated, err = c.ifStatement(next, returnType)
		case *t.NodeStmtElse:
			elseTerminated, err = c.statements(next.Body.Statements, returnType)
		default:
			err = fmt.Errorf("invalid conditional continuation %T", node.NextCondStmt)
		}
		if err != nil {
			return false, err
		}
	}
	elseExit := c.block
	if thenTerminated && elseTerminated {
		return true, nil
	}
	end, err := c.newBlock("if.end")
	if err != nil {
		return false, err
	}
	if !thenTerminated {
		if err := c.backend.Branch(thenExit, end); err != nil {
			return false, err
		}
	}
	if !elseTerminated {
		if err := c.backend.Branch(elseExit, end); err != nil {
			return false, err
		}
	}
	c.block = end
	return false, nil
}

func errorPredicateBranchWeights(expression t.NodeExpr) (lb.BranchWeights, bool) {
	call, ok := expression.(*t.NodeExprCall)
	if !ok || call.AssociatedFnDef == nil {
		return lb.BranchWeights{}, false
	}
	switch call.AssociatedFnDef.ErrorPredicate {
	case t.ErrorPredicateNok:
		return lb.UnlikelyThen, true
	case t.ErrorPredicateOk:
		return lb.UnlikelyElse, true
	default:
		return lb.BranchWeights{}, false
	}
}

func (c *scalarFunction) whileStatement(node *t.NodeStmtWhile, returnType *t.NodeType) error {
	conditionBlock, err := c.newBlock("while.cond")
	if err != nil {
		return err
	}
	bodyBlock, err := c.newBlock("while.body")
	if err != nil {
		return err
	}
	exitBlock, err := c.newBlock("while.exit")
	if err != nil {
		return err
	}
	if err := c.backend.Branch(c.block, conditionBlock); err != nil {
		return err
	}
	c.block = conditionBlock
	condition, err := c.expression(node.CondExpr, node.CondExpr.GetInferredType())
	if err != nil {
		return err
	}
	if err := c.backend.CondBranch(c.block, condition, bodyBlock, exitBlock); err != nil {
		return err
	}
	c.loops = append(c.loops, loopTargets{continueBlock: conditionBlock, breakBlock: exitBlock, cleanupDepth: len(c.cleanups)})
	c.block = bodyBlock
	terminated, err := c.statements(node.Body.Statements, returnType)
	c.loops = c.loops[:len(c.loops)-1]
	if err != nil {
		return err
	}
	if !terminated {
		if err := c.backend.Branch(c.block, conditionBlock); err != nil {
			return err
		}
	}
	c.block = exitBlock
	return nil
}

func (c *scalarFunction) loopBranch(continued bool) (bool, error) {
	if c.inCleanup != 0 {
		return false, fmt.Errorf("terminating control flow inside defer lowering is not yet supported")
	}
	if len(c.loops) == 0 {
		return false, fmt.Errorf("loop control statement used outside a loop")
	}
	targets := c.loops[len(c.loops)-1]
	target := targets.breakBlock
	if continued {
		target = targets.continueBlock
	}
	if err := c.runCleanups(targets.cleanupDepth, false); err != nil {
		return false, err
	}
	return true, c.backend.Branch(c.block, target)
}

func (c *scalarFunction) boundedStatement(node *t.NodeStmtBounded, returnType *t.NodeType) (bool, error) {
	if len(node.Proofs) == 0 {
		return false, fmt.Errorf("bounded statement lacks validated range facts")
	}
	terminated, err := c.statements(node.Body.Statements, returnType)
	if err != nil {
		return false, err
	}
	if terminated {
		// Later statements are lowered in an unreachable continuation. A bounded
		// assertion itself emits no branch or runtime check.
		c.block, err = c.newBlock("bounded.cont")
		if err != nil {
			return false, err
		}
	}
	return false, nil
}

func (c *scalarFunction) forStatement(node *t.NodeStmtFor, returnType *t.NodeType) error {
	declaration, ok := node.DeclExpr.(*t.NodeExprVarDefAssign)
	if !ok || declaration.VarDef == nil {
		return fmt.Errorf("for loop has no initialized index declaration")
	}
	indexType := declaration.VarDef.Type
	if _, err := c.expression(declaration, indexType); err != nil {
		return err
	}
	bound, err := c.expression(node.BoundExpr, indexType)
	if err != nil {
		return err
	}
	bound, err = c.coerce(bound, node.BoundExpr.GetInferredType(), indexType)
	if err != nil {
		return err
	}
	conditionBlock, err := c.newBlock("for.cond")
	if err != nil {
		return err
	}
	bodyBlock, err := c.newBlock("for.body")
	if err != nil {
		return err
	}
	incrementBlock, err := c.newBlock("for.increment")
	if err != nil {
		return err
	}
	breakIncrementBlock, err := c.newBlock("for.break.increment")
	if err != nil {
		return err
	}
	exitBlock, err := c.newBlock("for.exit")
	if err != nil {
		return err
	}
	if err := c.backend.Branch(c.block, conditionBlock); err != nil {
		return err
	}
	c.block = conditionBlock
	indexName := &t.NodeExprName{Name: declaration.VarDef.Name, InfType: indexType, AssociatedNode: declaration.VarDef, Storage: declaration.VarDef.Storage}
	index, err := c.expression(indexName, indexType)
	if err != nil {
		return err
	}
	condition, err := c.types.Compare(c.block, t.KwCmpLt, index, bound, indexType)
	if err != nil {
		return err
	}
	if err := c.backend.CondBranch(c.block, condition, bodyBlock, exitBlock); err != nil {
		return err
	}
	c.loops = append(c.loops, loopTargets{continueBlock: incrementBlock, breakBlock: breakIncrementBlock, cleanupDepth: len(c.cleanups)})
	c.block = bodyBlock
	terminated, err := c.statements(node.Body.Statements, returnType)
	c.loops = c.loops[:len(c.loops)-1]
	if err != nil {
		return err
	}
	if !terminated {
		if err := c.backend.Branch(c.block, incrementBlock); err != nil {
			return err
		}
	}
	c.block = incrementBlock
	if err := c.incrementForIndex(declaration.VarDef); err != nil {
		return err
	}
	if err := c.backend.Branch(c.block, conditionBlock); err != nil {
		return err
	}
	// Textual lowering models the increment as a synthetic defer, so it also
	// runs when break leaves the loop body.
	c.block = breakIncrementBlock
	if err := c.incrementForIndex(declaration.VarDef); err != nil {
		return err
	}
	if err := c.backend.Branch(c.block, exitBlock); err != nil {
		return err
	}
	c.block = exitBlock
	return nil
}

func (c *scalarFunction) incrementForIndex(variable *t.NodeExprVarDef) error {
	name := &t.NodeExprName{Name: variable.Name, InfType: variable.Type, AssociatedNode: variable, Storage: variable.Storage}
	value, err := c.expression(name, variable.Type)
	if err != nil {
		return err
	}
	typeID, err := c.types.Lower(variable.Type)
	if err != nil {
		return err
	}
	oneConstant, err := c.backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: typeID, Integer: "1"})
	if err != nil {
		return err
	}
	one, err := c.backend.ConstantValue(oneConstant)
	if err != nil {
		return err
	}
	incremented, err := c.types.NumericBinary(c.block, t.KwPlus, value, one, variable.Type)
	if err != nil {
		return err
	}
	storage, _, err := c.nameStorage(name)
	if err != nil {
		return err
	}
	_, err = c.backend.Store(c.block, incremented, storage, 0, false)
	return err
}

func (c *scalarFunction) matchStatement(node *t.NodeStmtMatch, returnType *t.NodeType) (bool, error) {
	matched, err := c.expression(node.Expression, node.Expression.GetInferredType())
	if err != nil {
		return false, err
	}
	tag, err := c.backend.ExtractValue(c.block, matched, []uint32{0})
	if err != nil {
		return false, err
	}
	i64Node := &t.NodeType{KindNode: &t.NodeTypeNamed{NameNode: &t.NodeNameSingle{Name: "u64"}}}
	i64, err := c.types.Lower(i64Node)
	if err != nil {
		return false, err
	}
	exits := make([]lb.BlockID, 0, len(node.Cases)+1)
	for _, arm := range node.Cases {
		if arm == nil || arm.Variant == nil || arm.Binding == nil {
			return false, fmt.Errorf("match arm is incomplete")
		}
		body, err := c.newBlock("match.case")
		if err != nil {
			return false, err
		}
		next, err := c.newBlock("match.next")
		if err != nil {
			return false, err
		}
		tagConstant, err := c.backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: i64, Integer: fmt.Sprint(arm.Variant.Tag)})
		if err != nil {
			return false, err
		}
		tagValue, err := c.backend.ConstantValue(tagConstant)
		if err != nil {
			return false, err
		}
		equal, err := c.backend.Compare(c.block, lb.CompareEqual, tag, tagValue)
		if err != nil {
			return false, err
		}
		if err := c.backend.CondBranch(c.block, equal, body, next); err != nil {
			return false, err
		}
		c.block = body
		storage, err := c.local(arm.Binding)
		if err != nil {
			return false, err
		}
		unionType, err := c.types.Lower(node.Expression.GetInferredType())
		if err != nil {
			return false, err
		}
		unionAddress, err := c.backend.Alloca(c.block, unionType, 0)
		if err != nil {
			return false, err
		}
		if _, err = c.backend.Store(c.block, matched, unionAddress, 0, false); err != nil {
			return false, err
		}
		payloadAddress, err := c.backend.StructFieldAddress(c.block, unionType, unionAddress, 2)
		if err != nil {
			return false, err
		}
		payloadAddress, err = c.backend.ReinterpretPointer(payloadAddress)
		if err != nil {
			return false, err
		}
		payloadType, err := c.types.Lower(arm.Binding.Type)
		if err != nil {
			return false, err
		}
		payload, err := c.backend.Load(c.block, payloadType, payloadAddress, 1, false)
		if err != nil {
			return false, err
		}
		if _, err := c.backend.Store(c.block, payload, storage, 0, false); err != nil {
			return false, err
		}
		terminated, err := c.statements(arm.Body.Statements, returnType)
		if err != nil {
			return false, err
		}
		if !terminated {
			exits = append(exits, c.block)
		}
		c.block = next
	}
	elseTerminated := false
	if node.ElseBody != nil {
		elseTerminated, err = c.statements(node.ElseBody.Statements, returnType)
		if err != nil {
			return false, err
		}
	}
	if !elseTerminated {
		exits = append(exits, c.block)
	}
	if len(exits) == 0 {
		return true, nil
	}
	end, err := c.newBlock("match.end")
	if err != nil {
		return false, err
	}
	for _, block := range exits {
		if err := c.backend.Branch(block, end); err != nil {
			return false, err
		}
	}
	c.block = end
	return false, nil
}

func (c *scalarFunction) expression(expression t.NodeExpr, expected *t.NodeType) (lb.ValueID, error) {
	switch node := expression.(type) {
	case *t.NodeExprLit:
		return c.literal(node, expected)
	case *t.NodeExprEmbed:
		return c.embeddedAsset(node)
	case *t.NodeExprArray:
		return c.array(node)
	case *t.NodeExprSizeof:
		if node.Type == nil {
			return 0, fmt.Errorf("sizeof: missing type")
		}
		resultType, err := c.types.Lower(node.InfType)
		if err != nil {
			return 0, err
		}
		size := uint64(0)
		ordinary := *node.Type
		ordinary.Throws = false
		if name, named := scalarTypeName(&ordinary); !named || name != "void" {
			measured, err := c.types.Lower(node.Type)
			if err != nil {
				return 0, err
			}
			layout, err := c.backend.TypeLayout(measured)
			if err != nil {
				return 0, fmt.Errorf("sizeof: %w", err)
			}
			size = layout.AllocationSize
		}
		constant, err := c.backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: resultType, Integer: strconv.FormatUint(size, 10)})
		if err != nil {
			return 0, err
		}
		return c.backend.ConstantValue(constant)
	case *t.NodeExprLlvm:
		return c.llvmDirective(node)
	case *t.NodeExprName:
		if function, ok := node.AssociatedNode.(*t.NodeFuncDef); ok {
			return c.namedFunctionAddress(node, function)
		}
		return c.name(node)
	case *t.NodeExprCall:
		return c.call(node, false, false)
	case *t.NodeExprTry:
		return c.tryExpression(node)
	case *t.NodeExprDestructureAssign:
		return c.destructureCall(node)
	case *t.NodeExprMove:
		return c.expression(node.Expr, expected)
	case *t.NodeExprVarDef:
		return c.local(node)
	case *t.NodeExprVarDefAssign:
		value, err := c.expression(node.AssignExpr, node.VarDef.Type)
		if err != nil {
			return 0, err
		}
		value, err = c.coerce(value, node.AssignExpr.GetInferredType(), node.VarDef.Type)
		if err != nil {
			return 0, err
		}
		storage, err := c.local(node.VarDef)
		if err != nil {
			return 0, err
		}
		if _, err := c.backend.Store(c.block, value, storage, 0, false); err != nil {
			return 0, err
		}
		return value, nil
	case *t.NodeExprAssign:
		if variable := implicitContextVariable(node.Left); variable != nil {
			if err := c.materializeContext(variable); err != nil {
				return 0, err
			}
		}
		storage, err := c.lvalue(node.Left)
		if err != nil {
			return 0, err
		}
		value, err := c.expression(node.Right, node.Left.GetInferredType())
		if err != nil {
			return 0, err
		}
		value, err = c.coerce(value, node.Right.GetInferredType(), node.Left.GetInferredType())
		if err != nil {
			return 0, err
		}
		if _, err := c.backend.Store(c.block, value, storage, 0, false); err != nil {
			return 0, err
		}
		return value, nil
	case *t.NodeExprBinary:
		return c.binary(node)
	case *t.NodeExprStructInit:
		return c.structInit(node)
	case *t.NodeExprProtoView:
		return c.protoView(node)
	case *t.NodeExprMemberAccess:
		return c.member(node)
	case *t.NodeExprSubscript:
		address, err := c.subscriptAddress(node)
		if err != nil {
			return 0, err
		}
		element, err := c.types.Lower(node.ElemType)
		if err != nil {
			return 0, err
		}
		return c.backend.Load(c.block, element, address, 0, false)
	case *t.NodeExprAddrof:
		address, err := c.lvalue(node.Expr)
		if err == nil {
			return address, nil
		}
		value, valueErr := c.expression(node.Expr, node.Expr.GetInferredType())
		if valueErr != nil {
			return 0, valueErr
		}
		typeID, typeErr := c.types.Lower(node.Expr.GetInferredType())
		if typeErr != nil {
			return 0, typeErr
		}
		storage, typeErr := c.backend.StaticAlloca(c.fn, typeID, 0)
		if typeErr != nil {
			return 0, typeErr
		}
		if _, typeErr = c.backend.Store(c.block, value, storage, 0, false); typeErr != nil {
			return 0, typeErr
		}
		return storage, nil
	case *t.NodeExprUnary:
		value, err := c.expression(node.Operand, node.Operand.GetInferredType())
		if err != nil {
			return 0, err
		}
		switch node.Operator {
		case t.KwTilde, t.KwNot:
			return c.types.UnaryNot(c.block, value, node.Operand.GetInferredType())
		case t.KwAsterisk:
			if !node.ProvenanceChecked {
				return 0, fmt.Errorf("pointer dereference lacks provenance analysis")
			}
			result, err := c.types.Lower(node.InfType)
			if err != nil {
				return 0, err
			}
			return c.backend.Load(c.block, result, value, 0, false)
		default:
			return 0, fmt.Errorf("unsupported scalar unary operator %q", t.KwTypeToRepr[node.Operator])
		}
	default:
		return 0, fmt.Errorf("unsupported scalar expression %T", expression)
	}
}

func (c *scalarFunction) embeddedAsset(node *t.NodeExprEmbed) (lb.ValueID, error) {
	if node == nil || node.Symbol == "" {
		return 0, fmt.Errorf("embedded asset has no linker symbol")
	}
	i8, err := c.backend.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 8})
	if err != nil {
		return 0, err
	}
	storageSize := node.Size
	if storageSize == 0 {
		storageSize = 1
	}
	array, err := c.backend.InternType(lb.TypeSpec{Kind: lb.TypeArray, Element: i8, Length: storageSize})
	if err != nil {
		return 0, err
	}
	global, err := c.backend.DeclareGlobal(lb.GlobalSpec{
		Symbol:     node.Symbol,
		Type:       array,
		Linkage:    lb.LinkageExternal,
		Visibility: lb.VisibilityDefault,
		Constant:   true,
	})
	if err != nil {
		return 0, err
	}
	data, err := c.backend.GlobalAddress(global)
	if err != nil {
		return 0, err
	}
	i64, err := c.backend.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 64})
	if err != nil {
		return 0, err
	}
	count, err := integerValue(c.backend, i64, node.Size)
	if err != nil {
		return 0, err
	}
	return c.types.BuildSlice(c.block, data, count)
}

func (c *scalarFunction) llvmDirective(node *t.NodeExprLlvm) (lb.ValueID, error) {
	if node == nil || node.ResultType == nil {
		return 0, fmt.Errorf("incomplete @llvm directive")
	}
	lower := func(index int) (lb.ValueID, error) {
		if index < 0 || index >= len(node.Args) {
			return 0, fmt.Errorf("@llvm %s operand %d is missing", node.Operation, index)
		}
		return c.expression(node.Args[index], node.Args[index].GetInferredType())
	}
	alignment := func(index int) (uint32, error) {
		if index < 0 || index >= len(node.Args) {
			return 0, fmt.Errorf("@llvm %s alignment is missing", node.Operation)
		}
		literal, ok := node.Args[index].(*t.NodeExprLit)
		if !ok || literal.LitType != t.TokLitNum {
			return 0, fmt.Errorf("@llvm %s alignment must be an integer literal", node.Operation)
		}
		value, err := strconv.ParseUint(strings.ReplaceAll(literal.Value, "_", ""), 10, 32)
		if err != nil || (value != 1 && value != 2 && value != 4 && value != 8 && value != 16) {
			return 0, fmt.Errorf("@llvm %s alignment %q is not a positive power of two", node.Operation, literal.Value)
		}
		return uint32(value), nil
	}
	configuration := func(index int) (string, error) {
		if index < 0 || index >= len(node.Args) {
			return "", fmt.Errorf("@llvm %s configuration %d is missing", node.Operation, index)
		}
		literal, ok := node.Args[index].(*t.NodeExprLit)
		if !ok {
			return "", fmt.Errorf("@llvm %s configuration %d must be a literal", node.Operation, index)
		}
		return literal.Value, nil
	}
	ordering := func(index int) (lb.AtomicOrdering, error) {
		value, err := configuration(index)
		if err != nil {
			return 0, err
		}
		orderings := map[string]lb.AtomicOrdering{"monotonic": lb.AtomicMonotonic, "acquire": lb.AtomicAcquire, "release": lb.AtomicRelease, "acq_rel": lb.AtomicAcquireRelease, "seq_cst": lb.AtomicSequentiallyConsistent}
		result := orderings[value]
		if result == 0 {
			return 0, fmt.Errorf("invalid LLVM atomic ordering %q", value)
		}
		return result, nil
	}
	resultType, err := c.types.Lower(node.ResultType)
	if err != nil {
		return 0, err
	}
	switch node.Operation {
	case "reinterpret":
		if len(node.Args) != 1 {
			return 0, fmt.Errorf("@llvm reinterpret expects one operand")
		}
		return lower(0)
	case "offset":
		if len(node.Args) != 2 {
			return 0, fmt.Errorf("@llvm offset expects two operands")
		}
		pointer, err := lower(0)
		if err != nil {
			return 0, err
		}
		offset, err := lower(1)
		if err != nil {
			return 0, err
		}
		i8, err := c.types.Lower(&t.NodeType{KindNode: &t.NodeTypeNamed{NameNode: &t.NodeNameSingle{Name: "u8"}}})
		if err != nil {
			return 0, err
		}
		return c.backend.GEP(c.block, i8, pointer, []lb.ValueID{offset}, false)
	case "ptrtoint", "inttoptr":
		if len(node.Args) != 1 {
			return 0, fmt.Errorf("@llvm %s expects one operand", node.Operation)
		}
		operand, err := lower(0)
		if err != nil {
			return 0, err
		}
		if node.Operation == "ptrtoint" {
			return c.backend.PtrToInt(c.block, operand, resultType)
		}
		return c.backend.IntToPtr(c.block, operand, resultType)
	case "bitcast", "sext", "zext", "trunc", "sitofp", "uitofp", "fptosi", "fptoui":
		if len(node.Args) != 1 {
			return 0, fmt.Errorf("@llvm %s expects one operand", node.Operation)
		}
		operand, err := lower(0)
		if err != nil {
			return 0, err
		}
		operations := map[string]lb.CastOp{"bitcast": lb.CastBit, "sext": lb.CastSignExtend, "zext": lb.CastZeroExtend, "trunc": lb.CastTruncate, "sitofp": lb.CastSignedIntToFloat, "uitofp": lb.CastUnsignedIntToFloat, "fptosi": lb.CastFloatToSignedInt, "fptoui": lb.CastFloatToUnsignedInt}
		return c.backend.Cast(c.block, operations[node.Operation], operand, resultType)
	case "load_volatile":
		if len(node.Args) != 2 {
			return 0, fmt.Errorf("@llvm load_volatile expects pointer and alignment")
		}
		pointer, err := lower(0)
		if err != nil {
			return 0, err
		}
		align, err := alignment(1)
		if err != nil {
			return 0, err
		}
		return c.backend.Load(c.block, resultType, pointer, align, true)
	case "store_volatile":
		if len(node.Args) != 3 {
			return 0, fmt.Errorf("@llvm store_volatile expects pointer, value, and alignment")
		}
		pointer, err := lower(0)
		if err != nil {
			return 0, err
		}
		value, err := lower(1)
		if err != nil {
			return 0, err
		}
		align, err := alignment(2)
		if err != nil {
			return 0, err
		}
		return c.backend.Store(c.block, value, pointer, align, true)
	case "atomic_load":
		if len(node.Args) != 3 {
			return 0, fmt.Errorf("@llvm atomic_load expects pointer, ordering, and alignment")
		}
		pointer, err := lower(0)
		if err != nil {
			return 0, err
		}
		order, err := ordering(1)
		if err != nil {
			return 0, err
		}
		align, err := alignment(2)
		if err != nil {
			return 0, err
		}
		return c.backend.AtomicLoad(c.block, resultType, pointer, order, align)
	case "atomic_store":
		if len(node.Args) != 4 {
			return 0, fmt.Errorf("@llvm atomic_store expects pointer, value, ordering, and alignment")
		}
		pointer, err := lower(0)
		if err != nil {
			return 0, err
		}
		value, err := lower(1)
		if err != nil {
			return 0, err
		}
		order, err := ordering(2)
		if err != nil {
			return 0, err
		}
		align, err := alignment(3)
		if err != nil {
			return 0, err
		}
		return c.backend.AtomicStore(c.block, value, pointer, order, align)
	case "atomic_rmw":
		if len(node.Args) != 5 {
			return 0, fmt.Errorf("@llvm atomic_rmw expects pointer, value, operation, ordering, and alignment")
		}
		pointer, err := lower(0)
		if err != nil {
			return 0, err
		}
		value, err := lower(1)
		if err != nil {
			return 0, err
		}
		operationName, err := configuration(2)
		if err != nil {
			return 0, err
		}
		operations := map[string]lb.AtomicRMWOp{"xchg": lb.AtomicRMWExchange, "add": lb.AtomicRMWAdd, "sub": lb.AtomicRMWSub}
		operation := operations[operationName]
		if operation == 0 {
			return 0, fmt.Errorf("unsupported atomicrmw operation %q", operationName)
		}
		order, err := ordering(3)
		if err != nil {
			return 0, err
		}
		align, err := alignment(4)
		if err != nil {
			return 0, err
		}
		return c.backend.AtomicRMW(c.block, operation, pointer, value, order, align)
	case "cmpxchg_old":
		if len(node.Args) != 6 {
			return 0, fmt.Errorf("@llvm cmpxchg_old expects pointer, expected, desired, two orderings, and alignment")
		}
		pointer, err := lower(0)
		if err != nil {
			return 0, err
		}
		expected, err := lower(1)
		if err != nil {
			return 0, err
		}
		desired, err := lower(2)
		if err != nil {
			return 0, err
		}
		success, err := ordering(3)
		if err != nil {
			return 0, err
		}
		failure, err := ordering(4)
		if err != nil {
			return 0, err
		}
		align, err := alignment(5)
		if err != nil {
			return 0, err
		}
		return c.backend.CompareExchangeOld(c.block, pointer, expected, desired, success, failure, align)
	case "bswap", "bitreverse", "ctpop", "ctlz", "cttz":
		if len(node.Args) != 1 {
			return 0, fmt.Errorf("@llvm %s expects one operand", node.Operation)
		}
		operand, err := lower(0)
		if err != nil {
			return 0, err
		}
		name, ok := scalarTypeName(node.ResultType)
		if !ok || len(name) < 2 || (name[0] != 'i' && name[0] != 'u') {
			return 0, fmt.Errorf("@llvm %s requires an integer result type", node.Operation)
		}
		intrinsic := "llvm." + node.Operation + ".i" + name[1:]
		arguments := []lb.ValueID{operand}
		parameters := []lb.TypeID{resultType}
		if node.Operation == "ctlz" || node.Operation == "cttz" {
			boolean, err := c.types.Lower(&t.NodeType{KindNode: &t.NodeTypeNamed{NameNode: &t.NodeNameSingle{Name: "bool"}}})
			if err != nil {
				return 0, err
			}
			zero, err := c.backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: boolean, Integer: "0"})
			if err != nil {
				return 0, err
			}
			undefinedIsZero, err := c.backend.ConstantValue(zero)
			if err != nil {
				return 0, err
			}
			parameters = append(parameters, boolean)
			arguments = append(arguments, undefinedIsZero)
		}
		return c.intrinsicCall(intrinsic, resultType, parameters, arguments)
	case "expect":
		if len(node.Args) != 2 {
			return 0, fmt.Errorf("@llvm expect expects value and expected value")
		}
		value, err := lower(0)
		if err != nil {
			return 0, err
		}
		expected, err := lower(1)
		if err != nil {
			return 0, err
		}
		return c.intrinsicCall("llvm.expect.i1", resultType, []lb.TypeID{resultType, resultType}, []lb.ValueID{value, expected})
	case "assume":
		if len(node.Args) != 1 {
			return 0, fmt.Errorf("@llvm assume expects one condition")
		}
		condition, err := lower(0)
		if err != nil {
			return 0, err
		}
		boolean, err := c.types.Lower(node.Args[0].GetInferredType())
		if err != nil {
			return 0, err
		}
		return c.intrinsicCall("llvm.assume", resultType, []lb.TypeID{boolean}, []lb.ValueID{condition})
	case "fence":
		if len(node.Args) != 1 {
			return 0, fmt.Errorf("@llvm fence expects one ordering literal")
		}
		order, err := configuration(0)
		if err != nil {
			return 0, err
		}
		if order != "seq_cst" {
			return 0, fmt.Errorf("object fence currently supports only seq_cst ordering")
		}
		return c.backend.InlineAssemblySideEffect(c.block, "fence.seq_cst")
	case "sideeffect", "trap":
		if len(node.Args) != 0 {
			return 0, fmt.Errorf("@llvm %s expects no operands", node.Operation)
		}
		return c.intrinsicCall("llvm."+node.Operation, resultType, nil, nil)
	case "asm_sideeffect":
		if len(node.Args) != 1 {
			return 0, fmt.Errorf("@llvm asm_sideeffect expects one instruction literal")
		}
		instruction, err := configuration(0)
		if err != nil {
			return 0, err
		}
		return c.backend.InlineAssemblySideEffect(c.block, instruction)
	default:
		return 0, fmt.Errorf("unsupported object @llvm operation %q", node.Operation)
	}
}

func (c *scalarFunction) intrinsicCall(symbol string, result lb.TypeID, parameters []lb.TypeID, arguments []lb.ValueID) (lb.ValueID, error) {
	function, err := c.backend.DeclareFunction(lb.FunctionSpec{Symbol: symbol, Result: result, Parameters: parameters, Linkage: lb.LinkageExternal, CallingConvention: lb.CallingConventionC})
	if err != nil {
		return 0, err
	}
	return c.backend.Call(c.block, function, arguments)
}

func (c *scalarFunction) structInit(node *t.NodeExprStructInit) (lb.ValueID, error) {
	if node == nil || node.Type == nil {
		return 0, fmt.Errorf("struct initializer has no resolved type")
	}
	typeID, err := c.types.Lower(node.Type)
	if err != nil {
		return 0, err
	}
	zero, err := c.backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantZero, Type: typeID})
	if err != nil {
		return 0, err
	}
	current, err := c.backend.ConstantValue(zero)
	if err != nil {
		return 0, err
	}
	if node.UnionVariant != nil {
		tagType, err := c.types.Lower(&t.NodeType{KindNode: &t.NodeTypeNamed{NameNode: &t.NodeNameSingle{Name: "u64"}}})
		if err != nil {
			return 0, err
		}
		tagConstant, err := c.backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: tagType, Integer: fmt.Sprint(node.UnionVariant.Tag)})
		if err != nil {
			return 0, err
		}
		tag, err := c.backend.ConstantValue(tagConstant)
		if err != nil {
			return 0, err
		}
		current, err = c.backend.InsertValue(c.block, current, tag, []uint32{0})
		if err != nil {
			return 0, err
		}
		variantType := &t.NodeType{KindNode: &t.NodeTypeAbsolute{AbsoluteName: node.UnionVariant.Owner.Module + ".__union_" + node.UnionVariant.Owner.Name + "_" + node.UnionVariant.Name}}
		variantID, err := c.types.Lower(variantType)
		if err != nil {
			return 0, err
		}
		variantZero, err := c.backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantZero, Type: variantID})
		if err != nil {
			return 0, err
		}
		payload, err := c.backend.ConstantValue(variantZero)
		if err != nil {
			return 0, err
		}
		for _, field := range node.Fields {
			value, err := c.expression(field.Expression, field.FieldType)
			if err != nil {
				return 0, err
			}
			value, err = c.coerce(value, field.Expression.GetInferredType(), field.FieldType)
			if err != nil {
				return 0, err
			}
			payload, err = c.backend.InsertValue(c.block, payload, value, []uint32{uint32(field.FieldIndex)})
			if err != nil {
				return 0, err
			}
		}
		unionType, err := c.types.Lower(node.Type)
		if err != nil {
			return 0, err
		}
		address, err := c.backend.Alloca(c.block, unionType, 0)
		if err != nil {
			return 0, err
		}
		if _, err = c.backend.Store(c.block, current, address, 0, false); err != nil {
			return 0, err
		}
		payloadAddress, err := c.backend.StructFieldAddress(c.block, unionType, address, 2)
		if err != nil {
			return 0, err
		}
		payloadAddress, err = c.backend.ReinterpretPointer(payloadAddress)
		if err != nil {
			return 0, err
		}
		if _, err = c.backend.Store(c.block, payload, payloadAddress, 1, false); err != nil {
			return 0, err
		}
		return c.backend.Load(c.block, unionType, address, 0, false)
	}
	for _, field := range node.Fields {
		if field.FieldIndex < 0 {
			return 0, fmt.Errorf("struct field %q has no resolved index", field.Name)
		}
		value, err := c.expression(field.Expression, field.FieldType)
		if err != nil {
			return 0, err
		}
		value, err = c.coerce(value, field.Expression.GetInferredType(), field.FieldType)
		if err != nil {
			return 0, err
		}
		current, err = c.backend.InsertValue(c.block, current, value, []uint32{uint32(field.FieldIndex)})
		if err != nil {
			return 0, err
		}
	}
	return current, nil
}

func (c *scalarFunction) protoView(node *t.NodeExprProtoView) (lb.ValueID, error) {
	if node == nil || node.Target == nil || node.ProtoType == nil || node.Implementation == nil || node.Implementation.Owner == nil || node.Implementation.Proto == nil {
		return 0, fmt.Errorf("cannot lower unresolved prototype view")
	}
	var implementation lb.ValueID
	var err error
	if node.TargetIsPointer {
		implementation, err = c.expression(node.Target, node.Target.GetInferredType())
	} else if node.Borrowed {
		implementation, err = c.lvalue(node.Target)
	} else {
		implementation, err = c.expression(node.Target, node.Target.GetInferredType())
	}
	if err != nil {
		return 0, err
	}
	vtable, err := c.protoVtable(node.Implementation, node.Borrowed)
	if err != nil {
		return 0, err
	}
	viewType, err := c.types.Lower(node.ProtoType)
	if err != nil {
		return 0, err
	}
	zero, err := c.backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantZero, Type: viewType})
	if err != nil {
		return 0, err
	}
	initial, err := c.backend.ConstantValue(zero)
	if err != nil {
		return 0, err
	}
	address, err := c.backend.Alloca(c.block, viewType, 0)
	if err != nil {
		return 0, err
	}
	if _, err = c.backend.Store(c.block, initial, address, 0, false); err != nil {
		return 0, err
	}
	vtableAddress, err := c.backend.StructFieldAddress(c.block, viewType, address, 0)
	if err != nil {
		return 0, err
	}
	if _, err = c.backend.Store(c.block, vtable, vtableAddress, 0, false); err != nil {
		return 0, err
	}
	storageAddress, err := c.backend.StructFieldAddress(c.block, viewType, address, 2)
	if err != nil {
		return 0, err
	}
	storageAddress, err = c.backend.ReinterpretPointer(storageAddress)
	if err != nil {
		return 0, err
	}
	if _, err = c.backend.Store(c.block, implementation, storageAddress, 1, false); err != nil {
		return 0, err
	}
	return c.backend.Load(c.block, viewType, address, 0, false)
}

func (c *scalarFunction) protoVtable(implementation *t.ProtoImpl, borrowed bool) (lb.ValueID, error) {
	if implementation == nil || implementation.Owner == nil || implementation.Proto == nil || implementation.Proto.VtableName == "" {
		return 0, fmt.Errorf("prototype implementation has incomplete vtable metadata")
	}
	proto := implementation.Proto
	symbol := t.ProtoVtableSymbol(implementation.Owner, proto)
	if borrowed {
		symbol = t.ProtoBorrowVtableSymbol(implementation.Owner, proto)
	}
	if existing := c.types.ProtoVtableGlobals[symbol]; existing != 0 {
		return c.backend.GlobalAddress(existing)
	}
	vtableType, err := c.types.Lower(&t.NodeType{KindNode: &t.NodeTypeAbsolute{AbsoluteName: proto.Module + "." + proto.VtableName}})
	if err != nil {
		return 0, err
	}
	pointer, err := c.backend.InternType(lb.TypeSpec{Kind: lb.TypePointer})
	if err != nil {
		return 0, err
	}
	entries := make([]lb.ConstantID, len(proto.Methods))
	for index, method := range proto.Methods {
		if method == nil || method.Slot != index {
			return 0, fmt.Errorf("prototype %s.%s has inconsistent method slot %d", proto.Module, proto.Name, index)
		}
		concrete := implementation.Owner.Funcs[method.Name]
		if concrete == nil {
			return 0, fmt.Errorf("missing resolved prototype method %s.%s", implementation.Owner.Name, method.Name)
		}
		var function lb.FunctionID
		if borrowed {
			function, err = c.protoBorrowThunk(implementation, method, concrete)
		} else {
			function, err = c.declareCallable(concrete)
		}
		if err != nil {
			return 0, err
		}
		entries[index], err = c.backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantFunctionAddress, Type: pointer, Function: function})
		if err != nil {
			return 0, err
		}
	}
	initializer, err := c.backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantAggregate, Type: vtableType, Elements: entries})
	if err != nil {
		return 0, err
	}
	global, err := c.backend.DeclareGlobal(lb.GlobalSpec{Symbol: symbol, Type: vtableType, Initializer: initializer, Linkage: lb.LinkagePrivate, Visibility: lb.VisibilityDefault, Constant: true, Definition: true})
	if err != nil {
		return 0, err
	}
	c.types.ProtoVtableGlobals[symbol] = global
	return c.backend.GlobalAddress(global)
}

func (c *scalarFunction) protoBorrowThunk(implementation *t.ProtoImpl, method *t.ProtoMethod, concrete *t.NodeFuncDef) (lb.FunctionID, error) {
	symbol := t.ProtoBorrowThunkSymbol(implementation.Owner, implementation.Proto, method)
	if existing := c.types.ProtoBorrowFunctions[symbol]; existing != 0 {
		return existing, nil
	}
	pointer, err := c.backend.InternType(lb.TypeSpec{Kind: lb.TypePointer})
	if err != nil {
		return 0, err
	}
	result, err := c.types.Lower(method.Ret)
	if err != nil {
		return 0, err
	}
	parameters := make([]lb.TypeID, 0, len(method.Args)+2)
	if method.ContextABI == t.ContextABIContextful {
		parameters = append(parameters, pointer)
	}
	parameters = append(parameters, pointer)
	for _, argument := range method.Args {
		parameter, err := c.types.Lower(argument.TypeNode)
		if err != nil {
			return 0, err
		}
		parameters = append(parameters, parameter)
	}
	function, err := c.backend.DeclareFunction(lb.FunctionSpec{Symbol: symbol, Result: result, Parameters: parameters, Linkage: lb.LinkagePrivate, CallingConvention: lb.CallingConventionC, Definition: true})
	if err != nil {
		return 0, err
	}
	entry, err := c.backend.AppendBlock(function, "entry")
	if err != nil {
		return 0, err
	}
	arguments := make([]lb.ValueID, 0, len(parameters))
	index := 0
	if method.ContextABI == t.ContextABIContextful {
		context, err := c.backend.Parameter(function, index)
		if err != nil {
			return 0, err
		}
		arguments = append(arguments, context)
		index++
	}
	storage, err := c.backend.Parameter(function, index)
	if err != nil {
		return 0, err
	}
	receiver, err := c.backend.Load(entry, pointer, storage, 1, false)
	if err != nil {
		return 0, err
	}
	arguments = append(arguments, receiver)
	for i := index + 1; i < len(parameters); i++ {
		value, err := c.backend.Parameter(function, i)
		if err != nil {
			return 0, err
		}
		arguments = append(arguments, value)
	}
	callee, err := c.declareCallable(concrete)
	if err != nil {
		return 0, err
	}
	value, err := c.backend.Call(entry, callee, arguments)
	if err != nil {
		return 0, err
	}
	ordinary := *method.Ret
	ordinary.Throws = false
	if name, named := scalarTypeName(&ordinary); named && name == "void" && !method.Ret.Throws {
		if err = c.backend.ReturnVoid(entry); err != nil {
			return 0, err
		}
	} else if err = c.backend.Return(entry, value); err != nil {
		return 0, err
	}
	if err = c.backend.FinalizeFunction(function); err != nil {
		return 0, err
	}
	c.types.ProtoBorrowFunctions[symbol] = function
	return function, nil
}

func (c *scalarFunction) member(node *t.NodeExprMemberAccess) (lb.ValueID, error) {
	if node == nil || node.Target == nil || node.Access == nil || node.Access.Type == nil {
		return 0, fmt.Errorf("member access is incomplete")
	}
	if node.MethodDef != nil {
		return c.functionAddress(node.MethodDef)
	}
	target, err := c.expression(node.Target, node.Target.GetInferredType())
	if err != nil {
		return 0, err
	}
	if node.Access.PtrDeref {
		owner, err := c.types.Lower(node.Access.OwnerType)
		if err != nil {
			return 0, err
		}
		target, err = c.backend.Load(c.block, owner, target, 0, false)
		if err != nil {
			return 0, err
		}
	}
	return c.backend.ExtractValue(c.block, target, []uint32{uint32(node.Access.FieldNb)})
}

func (c *scalarFunction) subscriptAddress(node *t.NodeExprSubscript) (lb.ValueID, error) {
	if node == nil || node.Target == nil || node.Expr == nil || node.BoxType == nil || node.ElemType == nil || node.IndexType == nil {
		return 0, fmt.Errorf("subscript expression is incomplete")
	}
	index, err := c.expression(node.Expr, node.IndexType)
	if err != nil {
		return 0, err
	}
	index, err = c.coerce(index, node.Expr.GetInferredType(), node.IndexType)
	if err != nil {
		return 0, err
	}
	target, err := c.expression(node.Target, node.BoxType)
	if err != nil {
		return 0, err
	}
	element, err := c.types.Lower(node.ElemType)
	if err != nil {
		return 0, err
	}
	switch node.BoxType.KindNode.(type) {
	case *t.NodeTypeSlice:
		return c.types.ProvenSliceElementAddress(c.block, target, index, element, node.RangeProof)
	case *t.NodeTypePointer:
		if node.RangeProof == nil {
			return 0, fmt.Errorf("pointer subscript lacks validated range proof")
		}
		return c.backend.GEP(c.block, element, target, []lb.ValueID{index}, false)
	case *t.NodeTypeRfc:
		if node.RangeProof == nil {
			return 0, fmt.Errorf("reference subscript lacks validated range proof")
		}
		return c.backend.GEP(c.block, element, target, []lb.ValueID{index}, false)
	default:
		return 0, fmt.Errorf("unsupported subscript container %T", node.BoxType.KindNode)
	}
}

func (c *scalarFunction) call(node *t.NodeExprCall, discard, keepPhysical bool) (lb.ValueID, error) {
	if node == nil {
		return 0, fmt.Errorf("cannot lower a missing call")
	}
	if node.IsMemberFunc {
		return c.memberCall(node, discard, keepPhysical)
	}
	if node.IsFuncPointer {
		return c.indirectCall(node, discard, keepPhysical)
	}
	callee := node.AssociatedFnDef
	if callee == nil {
		return 0, fmt.Errorf("direct call has no resolved function")
	}
	if callee.IsExternal || callee.NoAliasName != "" {
		return c.externalCall(node, discard)
	}
	if callee.ReturnType.Throws && node.ErrorMode == 2 && !keepPhysical {
		return 0, fmt.Errorf("nested throwing-call capture is not yet supported")
	}
	calleeID, err := c.declareCallable(callee)
	if err != nil {
		return 0, err
	}
	if len(node.Args) != len(callee.Class.ArgsNode.Args) {
		return 0, fmt.Errorf("call to %q has %d arguments; expected %d", callee.AbsName, len(node.Args), len(callee.Class.ArgsNode.Args))
	}
	arguments := make([]lb.ValueID, 0, len(node.Args)+1)
	if callee.ContextABI == t.ContextABIContextful {
		if c.context == 0 {
			return 0, fmt.Errorf("contextful call to %q has no initialized implicit context", callee.AbsName)
		}
		arguments = append(arguments, c.context)
	}
	for index, expression := range node.Args {
		expected := callee.Class.ArgsNode.Args[index].TypeNode
		argument, err := c.expression(expression, expected)
		if err != nil {
			return 0, err
		}
		argument, err = c.coerce(argument, expression.GetInferredType(), expected)
		if err != nil {
			return 0, err
		}
		arguments = append(arguments, argument)
	}
	result, err := c.backend.Call(c.block, calleeID, arguments)
	if err == nil && callee.ReturnType.Throws && node.ErrorMode == 1 && !keepPhysical {
		return c.propagateNestedCall(result, callee.ReturnType, node.Tk.Pos)
	}
	return c.finishCallResult(result, callee.ReturnType, discard, keepPhysical, err)
}

func (c *scalarFunction) memberCall(node *t.NodeExprCall, discard, keepPhysical bool) (lb.ValueID, error) {
	callee := node.AssociatedFnDef
	if callee == nil || !callee.IsMember || len(callee.Class.ArgsNode.Args) == 0 {
		return 0, fmt.Errorf("member call has no resolved member function")
	}
	if callee.IsExternal || callee.NoAliasName != "" {
		return 0, fmt.Errorf("external member call %q is unsupported", callee.AbsName)
	}
	if callee.ReturnType.Throws && node.ErrorMode == 2 && !keepPhysical {
		return 0, fmt.Errorf("nested throwing member-call capture is not yet supported")
	}
	expectedCount := len(callee.Class.ArgsNode.Args) - 1
	if len(node.Args) != expectedCount {
		return 0, fmt.Errorf("member call to %q has %d arguments; expected %d", callee.AbsName, len(node.Args), expectedCount)
	}
	var receiver lb.ValueID
	var err error
	if node.MemberOwnerExpr != nil {
		if callee.ProtoDispatch != nil {
			var viewExpression t.NodeExpr
			switch owner := node.MemberOwnerExpr.(type) {
			case *t.NodeExprName:
				copy := *owner
				copy.MemberAccesses = nil
				viewExpression = &copy
			case *t.NodeExprMemberAccess:
				viewExpression = owner.Target
			}
			if viewExpression != nil {
				receiver, err = c.lvalue(viewExpression)
			}
		}
		if receiver != 0 {
			if err != nil {
				return 0, err
			}
			// A protocol dispatch wrapper always receives the address of the
			// complete view, including its vtable pointer.
			goto receiverReady
		}
		receiver, err = c.expression(node.MemberOwnerExpr, node.MemberOwnerType)
		if err != nil {
			return 0, err
		}
		if !node.MemberOwnerIsPtr {
			ownerType, err := c.types.Lower(node.MemberOwnerType)
			if err != nil {
				return 0, err
			}
			storage, err := c.backend.Alloca(c.block, ownerType, 0)
			if err != nil {
				return 0, err
			}
			if _, err := c.backend.Store(c.block, receiver, storage, 0, false); err != nil {
				return 0, err
			}
			receiver = storage
		}
	} else if node.MemberOwnerName != nil {
		ownerName := node.MemberOwnerName
		receiver, err = c.lvalue(ownerName)
		if err != nil {
			return 0, err
		}
		isSSAOwner := ownerName.Storage.IsSSA()
		ownerHasMemberPath := len(ownerName.MemberAccesses) > 0
		pointerFieldOwner := false
		if ownerHasMemberPath {
			lastAccess := ownerName.MemberAccesses[len(ownerName.MemberAccesses)-1]
			pointerFieldOwner = lastAccess.ResultIsPtr
		}
		pointerLocalOwner := !isSSAOwner && isPointerSemantic(node.MemberOwnerType)
		if pointerFieldOwner || pointerLocalOwner {
			pointer, err := c.backend.InternType(lb.TypeSpec{Kind: lb.TypePointer})
			if err != nil {
				return 0, err
			}
			receiver, err = c.backend.Load(c.block, pointer, receiver, 0, false)
			if err != nil {
				return 0, err
			}
		}
	} else {
		return 0, fmt.Errorf("member call to %q has no receiver", callee.AbsName)
	}

receiverReady:
	arguments := make([]lb.ValueID, 0, len(node.Args)+2)
	if callee.ContextABI == t.ContextABIContextful {
		if c.context == 0 {
			return 0, fmt.Errorf("contextful member call to %q has no initialized implicit context", callee.AbsName)
		}
		arguments = append(arguments, c.context)
	}
	arguments = append(arguments, receiver)
	for index, expression := range node.Args {
		expected := callee.Class.ArgsNode.Args[index+1].TypeNode
		argument, err := c.expression(expression, expected)
		if err != nil {
			return 0, err
		}
		argument, err = c.coerce(argument, expression.GetInferredType(), expected)
		if err != nil {
			return 0, err
		}
		arguments = append(arguments, argument)
	}
	calleeID, err := c.declareCallable(callee)
	if err != nil {
		return 0, err
	}
	result, err := c.backend.Call(c.block, calleeID, arguments)
	if err == nil && callee.ReturnType.Throws && node.ErrorMode == 1 && !keepPhysical {
		return c.propagateNestedCall(result, callee.ReturnType, node.Tk.Pos)
	}
	return c.finishCallResult(result, callee.ReturnType, discard, keepPhysical, err)
}

func (c *scalarFunction) propagateNestedCall(physical lb.ValueID, throwingType *t.NodeType, position t.FilePos) (lb.ValueID, error) {
	if c.definition == nil || c.definition.ReturnType == nil || !c.definition.ReturnType.Throws {
		return 0, fmt.Errorf("throwing-call propagation requires a throwing enclosing function")
	}
	errorValue, err := c.backend.ExtractValue(c.block, physical, []uint32{0})
	if err != nil {
		return 0, err
	}
	code, err := c.types.ExtractCoreField(c.block, errorValue, t.CoreTypeError, "__code")
	if err != nil {
		return 0, err
	}
	codeType, err := c.types.CoreFieldType(t.CoreTypeError, "__code")
	if err != nil {
		return 0, err
	}
	zeroConstant, err := c.backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantZero, Type: codeType})
	if err != nil {
		return 0, err
	}
	zero, err := c.backend.ConstantValue(zeroConstant)
	if err != nil {
		return 0, err
	}
	failed, err := c.backend.Compare(c.block, lb.CompareNotEqual, code, zero)
	if err != nil {
		return 0, err
	}
	failure, err := c.newBlock("nested.try.failure")
	if err != nil {
		return 0, err
	}
	success, err := c.newBlock("nested.try.success")
	if err != nil {
		return 0, err
	}
	if err := c.backend.CondBranchWeighted(c.block, failed, failure, success, lb.UnlikelyThen); err != nil {
		return 0, err
	}
	c.block = failure
	site, push, err := c.traceSite(position)
	if err != nil {
		return 0, err
	}
	tracedError, err := c.backend.Call(c.block, push, []lb.ValueID{errorValue, site})
	if err != nil {
		return 0, err
	}
	failureValue, err := c.types.BuildThrowingFailure(c.block, c.definition.ReturnType, tracedError)
	if err != nil {
		return 0, err
	}
	if err := c.runErrorCleanups(); err != nil {
		return 0, err
	}
	if err := c.backend.Return(c.block, failureValue); err != nil {
		return 0, err
	}
	c.block = success
	ordinary := *throwingType
	ordinary.Throws = false
	if name, named := scalarTypeName(&ordinary); named && name == "void" {
		return 0, nil
	}
	return c.backend.ExtractValue(c.block, physical, []uint32{1})
}

func (c *scalarFunction) externalCall(node *t.NodeExprCall, discard bool) (lb.ValueID, error) {
	callee := node.AssociatedFnDef
	if callee == nil || callee.ReturnType == nil || callee.ReturnType.Throws {
		return 0, fmt.Errorf("external C call has an invalid checked signature")
	}
	function, returnPlan, argumentPlans, err := declareExternalFunction(c.backend, c.types, callee)
	if err != nil {
		return 0, err
	}
	parameterCount := 0
	if returnPlan.Class == loweringcabi.Indirect {
		parameterCount++
	}
	for _, plan := range argumentPlans {
		if plan.Class == loweringcabi.Coerce {
			parameterCount += len(plan.Parts)
		} else {
			parameterCount++
		}
	}
	arguments := make([]lb.ValueID, 0, parameterCount)
	var resultStorage lb.ValueID
	if returnPlan.Class == loweringcabi.Indirect {
		resultStorage, err = c.backend.Alloca(c.block, returnPlan.Logical, returnPlan.Alignment)
		if err != nil {
			return 0, err
		}
		arguments = append(arguments, resultStorage)
	}
	for index, expression := range node.Args {
		expected := callee.Class.ArgsNode.Args[index].TypeNode
		argument, err := c.expression(expression, expected)
		if err != nil {
			return 0, err
		}
		argument, err = c.coerce(argument, expression.GetInferredType(), expected)
		if err != nil {
			return 0, err
		}
		plan := argumentPlans[index]
		switch plan.Class {
		case loweringcabi.Indirect:
			storage, err := c.backend.Alloca(c.block, plan.Logical, plan.Alignment)
			if err != nil {
				return 0, err
			}
			if _, err := c.backend.Store(c.block, argument, storage, plan.Alignment, false); err != nil {
				return 0, err
			}
			arguments = append(arguments, storage)
		case loweringcabi.Coerce:
			parts, err := c.cABIValueParts(plan, argument)
			if err != nil {
				return 0, err
			}
			arguments = append(arguments, parts...)
		default:
			arguments = append(arguments, argument)
		}
	}
	raw, err := c.backend.Call(c.block, function, arguments)
	if err != nil || discard {
		return raw, err
	}
	if returnPlan.Class == loweringcabi.Indirect {
		return c.backend.Load(c.block, returnPlan.Logical, resultStorage, returnPlan.Alignment, false)
	}
	if returnPlan.Class != loweringcabi.Coerce {
		return raw, nil
	}
	storage, err := c.backend.Alloca(c.block, returnPlan.Logical, returnPlan.Alignment)
	if err != nil {
		return 0, err
	}
	address, err := c.backend.ReinterpretPointer(storage)
	if err != nil {
		return 0, err
	}
	if _, err := c.backend.Store(c.block, raw, address, returnPlan.Alignment, false); err != nil {
		return 0, err
	}
	return c.backend.Load(c.block, returnPlan.Logical, storage, returnPlan.Alignment, false)
}

func declareExternalFunction(backend lb.Backend, types *loweringtypes.Lowerer, callee *t.NodeFuncDef) (lb.FunctionID, loweringcabi.Value, []loweringcabi.Value, error) {
	if backend == nil || types == nil || callee == nil || callee.ReturnType == nil || callee.ReturnType.Throws {
		return 0, loweringcabi.Value{}, nil, fmt.Errorf("external C function has an invalid checked signature")
	}
	state := types.State()
	returnPlan, err := loweringcabi.Classify(backend, types, state, callee.ReturnType, true)
	if err != nil {
		return 0, loweringcabi.Value{}, nil, fmt.Errorf("external return type: %w", err)
	}
	physicalResult, err := loweringcabi.PhysicalResult(backend, returnPlan)
	if err != nil {
		return 0, loweringcabi.Value{}, nil, err
	}
	pointer, err := backend.InternType(lb.TypeSpec{Kind: lb.TypePointer})
	if err != nil {
		return 0, loweringcabi.Value{}, nil, err
	}
	parameterTypes := make([]lb.TypeID, 0, len(callee.Class.ArgsNode.Args)+2)
	attributes := make([]lb.AttributeSpec, 0)
	if returnPlan.Class == loweringcabi.Indirect {
		parameterTypes = append(parameterTypes, pointer)
		attributes = append(attributes,
			lb.AttributeSpec{Kind: lb.AttributeStructReturn, Placement: lb.AttributeParameter, Parameter: 0, Type: returnPlan.Logical},
			lb.AttributeSpec{Kind: lb.AttributeAlignment, Placement: lb.AttributeParameter, Parameter: 0, Value: uint64(returnPlan.Alignment)},
		)
	}
	argumentPlans := make([]loweringcabi.Value, len(callee.Class.ArgsNode.Args))
	for index, argument := range callee.Class.ArgsNode.Args {
		argumentPlans[index], err = loweringcabi.Classify(backend, types, state, argument.TypeNode, false)
		if err != nil {
			return 0, loweringcabi.Value{}, nil, fmt.Errorf("external argument %d: %w", index+1, err)
		}
		switch plan := argumentPlans[index]; plan.Class {
		case loweringcabi.Indirect:
			parameter := len(parameterTypes)
			parameterTypes = append(parameterTypes, pointer)
			if plan.ByValue {
				attributes = append(attributes,
					lb.AttributeSpec{Kind: lb.AttributeByValue, Placement: lb.AttributeParameter, Parameter: uint32(parameter), Type: plan.Logical},
					lb.AttributeSpec{Kind: lb.AttributeAlignment, Placement: lb.AttributeParameter, Parameter: uint32(parameter), Value: uint64(plan.Alignment)},
				)
			}
		case loweringcabi.Coerce:
			for _, part := range plan.Parts {
				parameterTypes = append(parameterTypes, part.Type)
			}
		default:
			parameterTypes = append(parameterTypes, plan.Logical)
		}
	}
	symbol := callee.NoAliasName
	if symbol == "" {
		symbol = callee.AbsName
	}
	function, err := backend.DeclareFunction(lb.FunctionSpec{Symbol: symbol, Result: physicalResult, Parameters: parameterTypes, Linkage: lb.LinkageExternal, CallingConvention: lb.CallingConventionC, Attributes: attributes})
	if err != nil {
		return 0, loweringcabi.Value{}, nil, err
	}
	return function, returnPlan, argumentPlans, nil
}

func (c *scalarFunction) cABIValueParts(plan loweringcabi.Value, value lb.ValueID) ([]lb.ValueID, error) {
	storage, err := c.backend.Alloca(c.block, plan.Logical, plan.Alignment)
	if err != nil {
		return nil, err
	}
	if _, err := c.backend.Store(c.block, value, storage, plan.Alignment, false); err != nil {
		return nil, err
	}
	rawStorage, err := c.backend.ReinterpretPointer(storage)
	if err != nil {
		return nil, err
	}
	i8, err := c.backend.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 8})
	if err != nil {
		return nil, err
	}
	i64, err := c.backend.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 64})
	if err != nil {
		return nil, err
	}
	parts := make([]lb.ValueID, len(plan.Parts))
	for index, part := range plan.Parts {
		address := rawStorage
		if part.Offset != 0 {
			offset, err := integerValue(c.backend, i64, part.Offset)
			if err != nil {
				return nil, err
			}
			address, err = c.backend.GEP(c.block, i8, rawStorage, []lb.ValueID{offset}, false)
			if err != nil {
				return nil, err
			}
		}
		address, err = c.backend.ReinterpretPointer(address)
		if err != nil {
			return nil, err
		}
		parts[index], err = c.backend.Load(c.block, part.Type, address, 1, false)
		if err != nil {
			return nil, err
		}
	}
	return parts, nil
}

func integerValue(backend lb.Backend, typ lb.TypeID, value uint64) (lb.ValueID, error) {
	constant, err := backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: typ, Integer: fmt.Sprint(value)})
	if err != nil {
		return 0, err
	}
	return backend.ConstantValue(constant)
}

func (c *scalarFunction) finishCallResult(result lb.ValueID, returnType *t.NodeType, discard, keepPhysical bool, err error) (lb.ValueID, error) {
	if err != nil || discard || returnType == nil || !returnType.Throws || keepPhysical {
		return result, err
	}
	ordinary := *returnType
	ordinary.Throws = false
	name, named := scalarTypeName(&ordinary)
	if named && name == "void" {
		return result, nil
	}
	return c.backend.ExtractValue(c.block, result, []uint32{1})
}

func (c *scalarFunction) destructureCall(node *t.NodeExprDestructureAssign) (lb.ValueID, error) {
	if node == nil || node.Call == nil || node.Call.ThrowingType == nil || !node.Call.ThrowingType.Throws {
		return 0, fmt.Errorf("destructuring requires a resolved throwing call")
	}
	valueSlot, err := c.local(&node.ValueDef)
	if err != nil {
		return 0, err
	}
	errorSlot, err := c.local(&node.ErrDef)
	if err != nil {
		return 0, err
	}
	physical, err := c.call(node.Call, false, true)
	if err != nil {
		return 0, err
	}
	errorValue, err := c.backend.ExtractValue(c.block, physical, []uint32{0})
	if err != nil {
		return 0, err
	}
	code, err := c.types.ExtractCoreField(c.block, errorValue, t.CoreTypeError, "__code")
	if err != nil {
		return 0, err
	}
	codeType, err := c.types.CoreFieldType(t.CoreTypeError, "__code")
	if err != nil {
		return 0, err
	}
	zeroConstant, err := c.backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantZero, Type: codeType})
	if err != nil {
		return 0, err
	}
	zero, err := c.backend.ConstantValue(zeroConstant)
	if err != nil {
		return 0, err
	}
	failed, err := c.backend.Compare(c.block, lb.CompareNotEqual, code, zero)
	if err != nil {
		return 0, err
	}
	failure, err := c.newBlock("capture.failure")
	if err != nil {
		return 0, err
	}
	success, err := c.newBlock("capture.success")
	if err != nil {
		return 0, err
	}
	end, err := c.newBlock("capture.end")
	if err != nil {
		return 0, err
	}
	if err := c.backend.CondBranchWeighted(c.block, failed, failure, success, lb.UnlikelyThen); err != nil {
		return 0, err
	}
	c.block = failure
	if _, err := c.backend.Store(c.block, errorValue, errorSlot, 0, false); err != nil {
		return 0, err
	}
	if err := c.backend.Branch(c.block, end); err != nil {
		return 0, err
	}
	c.block = success
	ordinary := *node.Call.ThrowingType
	ordinary.Throws = false
	name, named := scalarTypeName(&ordinary)
	if !named || name != "void" {
		value, err := c.backend.ExtractValue(c.block, physical, []uint32{1})
		if err != nil {
			return 0, err
		}
		if _, err := c.backend.Store(c.block, value, valueSlot, 0, false); err != nil {
			return 0, err
		}
	}
	if err := c.backend.Branch(c.block, end); err != nil {
		return 0, err
	}
	c.block = end
	return valueSlot, nil
}

func (c *scalarFunction) tryExpression(node *t.NodeExprTry) (lb.ValueID, error) {
	call, ok := node.Call.(*t.NodeExprCall)
	if !ok || call.ThrowingType == nil || !call.ThrowingType.Throws || (call.AssociatedFnDef == nil && !call.IsFuncPointer) {
		return 0, fmt.Errorf("try requires a resolved throwing call")
	}
	if c.definition == nil || c.definition.ReturnType == nil || !c.definition.ReturnType.Throws {
		return 0, fmt.Errorf("try requires a throwing enclosing function")
	}
	physical, err := c.call(call, false, true)
	if err != nil {
		return 0, err
	}
	errorValue, err := c.backend.ExtractValue(c.block, physical, []uint32{0})
	if err != nil {
		return 0, err
	}
	code, err := c.types.ExtractCoreField(c.block, errorValue, t.CoreTypeError, "__code")
	if err != nil {
		return 0, err
	}
	codeType, err := c.types.CoreFieldType(t.CoreTypeError, "__code")
	if err != nil {
		return 0, err
	}
	zeroConstant, err := c.backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantZero, Type: codeType})
	if err != nil {
		return 0, err
	}
	zero, err := c.backend.ConstantValue(zeroConstant)
	if err != nil {
		return 0, err
	}
	failed, err := c.backend.Compare(c.block, lb.CompareNotEqual, code, zero)
	if err != nil {
		return 0, err
	}
	failure, err := c.newBlock("try.failure")
	if err != nil {
		return 0, err
	}
	success, err := c.newBlock("try.success")
	if err != nil {
		return 0, err
	}
	if err := c.backend.CondBranchWeighted(c.block, failed, failure, success, lb.UnlikelyThen); err != nil {
		return 0, err
	}
	c.block = failure
	site, push, err := c.traceSite(node.Pos)
	if err != nil {
		return 0, err
	}
	tracedError, err := c.backend.Call(c.block, push, []lb.ValueID{errorValue, site})
	if err != nil {
		return 0, err
	}
	failureValue, err := c.types.BuildThrowingFailure(c.block, c.definition.ReturnType, tracedError)
	if err != nil {
		return 0, err
	}
	if err := c.runErrorCleanups(); err != nil {
		return 0, err
	}
	if err := c.backend.Return(c.block, failureValue); err != nil {
		return 0, err
	}
	c.block = success
	ordinary := *call.ThrowingType
	ordinary.Throws = false
	name, named := scalarTypeName(&ordinary)
	if named && name == "void" {
		return 0, nil
	}
	return c.backend.ExtractValue(c.block, physical, []uint32{1})
}

func (c *scalarFunction) traceSite(position t.FilePos) (lb.ValueID, lb.FunctionID, error) {
	siteDefinition, pushDefinition, err := c.types.TraceABI()
	if err != nil {
		return 0, 0, err
	}
	source, err := c.types.FunctionSource(c.definition)
	if err != nil {
		return 0, 0, err
	}
	functionName := traceFunctionName(c.definition)
	fileName := filepath.Base(source)
	pointer, err := c.backend.InternType(lb.TypeSpec{Kind: lb.TypePointer})
	if err != nil {
		return 0, 0, err
	}
	functionData, err := c.traceTextAddress(functionName, pointer)
	if err != nil {
		return 0, 0, err
	}
	fileData, err := c.traceTextAddress(fileName, pointer)
	if err != nil {
		return 0, 0, err
	}
	constants := make([]lb.ConstantID, len(siteDefinition.FieldOrder))
	for index, field := range siteDefinition.FieldOrder {
		recorded, ok := siteDefinition.FieldNb[field]
		if !ok || recorded != index || siteDefinition.Fields[field] == nil {
			return 0, 0, fmt.Errorf("trace site field %q has inconsistent layout metadata", field)
		}
		fieldType, err := c.types.Lower(siteDefinition.Fields[field])
		if err != nil {
			return 0, 0, err
		}
		spec := lb.ConstantSpec{Kind: lb.ConstantInteger, Type: fieldType}
		switch field {
		case "functionData":
			constants[index] = functionData
			continue
		case "fileData":
			constants[index] = fileData
			continue
		case "functionLength":
			spec.Integer = strconv.Itoa(len(functionName))
		case "fileLength":
			spec.Integer = strconv.Itoa(len(fileName))
		case "line":
			spec.Integer = strconv.FormatUint(uint64(position.Line), 10)
		case "column":
			spec.Integer = strconv.FormatUint(uint64(position.Col), 10)
		default:
			return 0, 0, fmt.Errorf("unsupported trace site field %q", field)
		}
		constants[index], err = c.backend.InternConstant(spec)
		if err != nil {
			return 0, 0, err
		}
	}
	siteTypeNode := &t.NodeType{KindNode: &t.NodeTypeAbsolute{AbsoluteName: siteDefinition.Module + "." + siteDefinition.Name}}
	siteType, err := c.types.Lower(siteTypeNode)
	if err != nil {
		return 0, 0, err
	}
	initializer, err := c.backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantAggregate, Type: siteType, Elements: constants})
	if err != nil {
		return 0, 0, err
	}
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%d\x00%d", functionName, fileName, position.Line, position.Col)))
	global, err := c.backend.DeclareGlobal(lb.GlobalSpec{Symbol: fmt.Sprintf(".magma.trace.site.%x", digest), Type: siteType, Initializer: initializer, Linkage: lb.LinkagePrivate, Visibility: lb.VisibilityDefault, Constant: true, UnnamedAddress: true, Definition: true})
	if err != nil {
		return 0, 0, err
	}
	address, err := c.backend.GlobalAddress(global)
	if err != nil {
		return 0, 0, err
	}
	push, err := c.declareCallable(pushDefinition)
	return address, push, err
}

func traceFunctionName(definition *t.NodeFuncDef) string {
	if definition != nil {
		switch name := definition.Class.NameNode.(type) {
		case *t.NodeNameSingle:
			return t.SourceName(name.Name)
		case *t.NodeNameComposite:
			return t.SourceName(strings.Join(name.Parts, "."))
		}
		if definition.DisplayName != "" {
			return t.SourceName(definition.DisplayName)
		}
		return t.SourceName(definition.AbsName)
	}
	return "<global>"
}

func (c *scalarFunction) traceTextAddress(value string, pointer lb.TypeID) (lb.ConstantID, error) {
	i8, err := c.types.Lower(&t.NodeType{KindNode: &t.NodeTypeNamed{NameNode: &t.NodeNameSingle{Name: "u8"}}})
	if err != nil {
		return 0, err
	}
	array, err := c.backend.InternType(lb.TypeSpec{Kind: lb.TypeArray, Element: i8, Length: uint64(len(value))})
	if err != nil {
		return 0, err
	}
	text, err := c.backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantString, Type: array, Bytes: value})
	if err != nil {
		return 0, err
	}
	digest := sha256.Sum256([]byte(value))
	global, err := c.backend.DeclareGlobal(lb.GlobalSpec{Symbol: fmt.Sprintf(".magma.trace.text.%x", digest), Type: array, Initializer: text, Linkage: lb.LinkagePrivate, Visibility: lb.VisibilityDefault, Constant: true, UnnamedAddress: true, Definition: true})
	if err != nil {
		return 0, err
	}
	return c.backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantGlobalAddress, Type: pointer, Global: global})
}

func (c *scalarFunction) indirectCall(node *t.NodeExprCall, discard, keepPhysical bool) (lb.ValueID, error) {
	if node.FuncPtrType == nil {
		return 0, fmt.Errorf("indirect call has no resolved function type")
	}
	functionType, ok := node.FuncPtrType.KindNode.(*t.NodeTypeFunc)
	if !ok {
		return 0, fmt.Errorf("indirect call target is not a function type")
	}
	if functionType.RetType == nil {
		return 0, fmt.Errorf("indirect call has no return type")
	}
	if functionType.RetType.Throws && node.ErrorMode != 0 && !keepPhysical {
		return 0, fmt.Errorf("throwing indirect call propagation requires cleanup and trace lowering")
	}
	if len(node.Args) != len(functionType.Args) {
		return 0, fmt.Errorf("indirect call has %d arguments; expected %d", len(node.Args), len(functionType.Args))
	}
	arguments := make([]lb.ValueID, 0, len(node.Args)+1)
	if functionType.ContextABI == t.ContextABIContextful {
		if c.context == 0 {
			return 0, fmt.Errorf("contextful indirect call has no initialized implicit context")
		}
		arguments = append(arguments, c.context)
	}
	var err error
	for index, expression := range node.Args {
		argument, err := c.expression(expression, functionType.Args[index])
		if err != nil {
			return 0, err
		}
		argument, err = c.coerce(argument, expression.GetInferredType(), functionType.Args[index])
		if err != nil {
			return 0, err
		}
		arguments = append(arguments, argument)
	}
	callee, err := c.expression(node.Callee, node.FuncPtrType)
	if err != nil {
		return 0, err
	}
	signature, err := c.types.Signature(functionType)
	if err != nil {
		return 0, err
	}
	result, err := c.backend.IndirectCall(c.block, signature, callee, arguments, lb.CallSpec{CallingConvention: lb.CallingConventionC})
	if err != nil || discard || !functionType.RetType.Throws || keepPhysical {
		return result, err
	}
	ordinary := *functionType.RetType
	ordinary.Throws = false
	name, named := scalarTypeName(&ordinary)
	if named && name == "void" {
		return result, nil
	}
	return c.backend.ExtractValue(c.block, result, []uint32{1})
}

func (c *scalarFunction) functionAddress(definition *t.NodeFuncDef) (lb.ValueID, error) {
	function, err := c.declareCallable(definition)
	if err != nil {
		return 0, err
	}
	return c.backend.FunctionAddress(function)
}

func (c *scalarFunction) namedFunctionAddress(name *t.NodeExprName, definition *t.NodeFuncDef) (lb.ValueID, error) {
	if !name.NativeContextThunk && !name.ContextAdapter {
		return c.functionAddress(definition)
	}
	shim := *definition
	shim.NoAliasName = ""
	shim.IsExternal = false
	shim.NeedsNativeContextThunk = false
	if name.NativeContextThunk {
		shim.AbsName = definition.AbsName + ".__native_ctx_thunk"
		shim.ContextABI = t.ContextABIContextless
	} else {
		shim.AbsName = definition.AbsName + ".__ctx_adapter"
		shim.ContextABI = t.ContextABIContextful
	}
	if name.ContextAdapter {
		function, err := c.declareCallableWithAttributes(&shim, []lb.AttributeSpec{{Kind: lb.AttributeAlwaysInline, Placement: lb.AttributeFunction}})
		if err != nil {
			return 0, err
		}
		return c.backend.FunctionAddress(function)
	}
	return c.functionAddress(&shim)
}

func (c *scalarFunction) declareCallable(definition *t.NodeFuncDef) (lb.FunctionID, error) {
	return c.declareCallableWithAttributes(definition, nil)
}

func (c *scalarFunction) declareCallableWithAttributes(definition *t.NodeFuncDef, additional []lb.AttributeSpec) (lb.FunctionID, error) {
	if definition == nil || definition.ReturnType == nil || definition.AbsName == "" {
		return 0, fmt.Errorf("call target is incomplete")
	}
	result, err := c.types.Lower(definition.ReturnType)
	if err != nil {
		return 0, err
	}
	parameters := make([]lb.TypeID, 0, len(definition.Class.ArgsNode.Args)+1)
	if definition.ContextABI == t.ContextABIContextful {
		pointer, err := c.backend.InternType(lb.TypeSpec{Kind: lb.TypePointer})
		if err != nil {
			return 0, err
		}
		parameters = append(parameters, pointer)
	}
	argumentStart := 0
	if definition.IsMember {
		if len(definition.Class.ArgsNode.Args) == 0 {
			return 0, fmt.Errorf("member call target %q has no receiver type", definition.AbsName)
		}
		pointer, err := c.backend.InternType(lb.TypeSpec{Kind: lb.TypePointer})
		if err != nil {
			return 0, err
		}
		parameters = append(parameters, pointer)
		argumentStart = 1
	}
	for _, argument := range definition.Class.ArgsNode.Args[argumentStart:] {
		parameter, err := c.types.Lower(argument.TypeNode)
		if err != nil {
			return 0, err
		}
		parameters = append(parameters, parameter)
	}
	attributes, err := functionAttributes(c.types, definition)
	if err != nil {
		return 0, err
	}
	attributes = append(attributes, additional...)
	symbol := definition.AbsName
	linkage := lb.LinkageInternal
	if c.types.IsTracePush(definition) {
		// Keep the canonical error return type as an ABI boundary. Internal
		// functions are eligible for LLVM return promotion, which otherwise
		// rewrites this helper to an anonymous aggregate at higher opt levels.
		linkage = lb.LinkageExternal
	}
	if c.types.CrossModule || definition.IsExternal || definition.NoAliasName != "" {
		linkage = lb.LinkageExternal
		if definition.NoAliasName != "" {
			symbol = definition.NoAliasName
		}
	}
	return c.backend.DeclareFunction(lb.FunctionSpec{Symbol: symbol, Result: result, Parameters: parameters, Linkage: linkage, CallingConvention: lb.CallingConventionC, Attributes: attributes})
}

func functionAttributes(types *loweringtypes.Lowerer, definition *t.NodeFuncDef) ([]lb.AttributeSpec, error) {
	attributes := []lb.AttributeSpec(nil)
	if definition.ProtoDispatch != nil {
		attributes = append(attributes, lb.AttributeSpec{Kind: lb.AttributeAlwaysInline, Placement: lb.AttributeFunction})
	}
	if name, ok := definition.Class.NameNode.(*t.NodeNameSingle); ok && name.Name == "errorTracePush" {
		_, tracePush, err := types.TraceABI()
		if err != nil {
			return nil, err
		}
		if definition == tracePush {
			attributes = append(attributes,
				lb.AttributeSpec{Kind: lb.AttributeNoInline, Placement: lb.AttributeFunction},
				lb.AttributeSpec{Kind: lb.AttributeCold, Placement: lb.AttributeFunction},
			)
		}
	}
	return attributes, nil
}

func (c *scalarFunction) binary(node *t.NodeExprBinary) (lb.ValueID, error) {
	if node.Operator == t.KwAndAnd || node.Operator == t.KwOrOr {
		return c.logical(node)
	}
	if isStringType(node.Left.GetInferredType()) && isStringType(node.Right.GetInferredType()) {
		if node.Operator != t.KwCmpEq && node.Operator != t.KwCmpNeq {
			return 0, fmt.Errorf("unsupported string comparison operator")
		}
		compare := c.types.State().CoreMethods["str.compare"]
		if compare == nil {
			return 0, fmt.Errorf("core method str.compare is required for string equality")
		}
		call := &t.NodeExprCall{
			Tk: node.Tk, Args: []t.NodeExpr{node.Right}, AssociatedFnDef: compare,
			InfType: node.InfType, IsMemberFunc: true,
			MemberOwnerType: node.Left.GetInferredType(), MemberOwnerExpr: node.Left,
		}
		equal, err := c.memberCall(call, false, false)
		if err != nil || node.Operator == t.KwCmpEq {
			return equal, err
		}
		return c.types.UnaryNot(c.block, equal, node.InfType)
	}
	operandType := node.OperandType
	if operandType == nil && (node.Operator == t.KwCmpEq || node.Operator == t.KwCmpNeq) {
		operandType = node.Left.GetInferredType()
	}
	if operandType == nil {
		return 0, fmt.Errorf("binary expression has no checked operand type")
	}
	left, err := c.expression(node.Left, operandType)
	if err != nil {
		return 0, err
	}
	left, err = c.coerce(left, node.Left.GetInferredType(), operandType)
	if err != nil {
		return 0, err
	}
	right, err := c.expression(node.Right, operandType)
	if err != nil {
		return 0, err
	}
	right, err = c.coerce(right, node.Right.GetInferredType(), operandType)
	if err != nil {
		return 0, err
	}
	switch node.Operator {
	case t.KwCmpEq, t.KwCmpNeq, t.KwCmpGt, t.KwCmpGtEq, t.KwCmpLt, t.KwCmpLtEq:
		return c.types.Compare(c.block, node.Operator, left, right, operandType)
	default:
		return c.types.NumericBinary(c.block, node.Operator, left, right, operandType)
	}
}

func isStringType(node *t.NodeType) bool {
	if node == nil {
		return false
	}
	name, ok := scalarTypeName(node)
	if ok {
		return name == "str"
	}
	absolute, ok := node.KindNode.(*t.NodeTypeAbsolute)
	return ok && absolute.CoreRole == t.CoreTypeString
}

// logical mirrors textual irExprBinLogical, including its stack-slot result.
// The right operand is emitted only in the RHS block, preserving short-circuit
// side effects and avoiding any new runtime checks.
func (c *scalarFunction) logical(node *t.NodeExprBinary) (lb.ValueID, error) {
	if !isBooleanType(node.Left.GetInferredType()) || !isBooleanType(node.Right.GetInferredType()) {
		return 0, fmt.Errorf("logical operator requires bool operands")
	}
	left, err := c.expression(node.Left, node.Left.GetInferredType())
	if err != nil {
		return 0, err
	}
	boolean, err := c.types.Lower(node.Left.GetInferredType())
	if err != nil {
		return 0, err
	}
	result, err := c.backend.StaticAlloca(c.fn, boolean, 0)
	if err != nil {
		return 0, err
	}
	rhs, err := c.newBlock("logical.rhs")
	if err != nil {
		return 0, err
	}
	shortCircuit, err := c.newBlock("logical.short")
	if err != nil {
		return 0, err
	}
	end, err := c.newBlock("logical.end")
	if err != nil {
		return 0, err
	}
	if node.Operator == t.KwAndAnd {
		err = c.backend.CondBranch(c.block, left, rhs, shortCircuit)
	} else {
		err = c.backend.CondBranch(c.block, left, shortCircuit, rhs)
	}
	if err != nil {
		return 0, err
	}
	c.block = rhs
	right, err := c.expression(node.Right, node.Right.GetInferredType())
	if err != nil {
		return 0, err
	}
	if _, err := c.backend.Store(c.block, right, result, 0, false); err != nil {
		return 0, err
	}
	if err := c.backend.Branch(c.block, end); err != nil {
		return 0, err
	}
	c.block = shortCircuit
	shortValue := "0"
	if node.Operator == t.KwOrOr {
		shortValue = "1"
	}
	constant, err := c.backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: boolean, Integer: shortValue})
	if err != nil {
		return 0, err
	}
	value, err := c.backend.ConstantValue(constant)
	if err != nil {
		return 0, err
	}
	if _, err := c.backend.Store(c.block, value, result, 0, false); err != nil {
		return 0, err
	}
	if err := c.backend.Branch(c.block, end); err != nil {
		return 0, err
	}
	c.block = end
	return c.backend.Load(c.block, boolean, result, 0, false)
}

func (c *scalarFunction) newBlock(prefix string) (lb.BlockID, error) {
	c.blocks++
	return c.backend.AppendBlock(c.fn, fmt.Sprintf("%s.%d", prefix, c.blocks))
}

func isBooleanType(node *t.NodeType) bool {
	name, ok := scalarTypeName(node)
	return ok && name == "bool"
}

func (c *scalarFunction) coerce(value lb.ValueID, from, to *t.NodeType) (lb.ValueID, error) {
	if from == nil || to == nil || sameScalarType(from, to) {
		return value, nil
	}
	fromID, fromErr := c.types.Lower(from)
	toID, toErr := c.types.Lower(to)
	if fromErr == nil && toErr == nil && fromID == toID {
		return value, nil
	}
	return c.types.CoerceNumeric(c.block, value, from, to)
}

func sameScalarType(left, right *t.NodeType) bool {
	leftID, leftOK := scalarTypeName(left)
	rightID, rightOK := scalarTypeName(right)
	return leftOK && rightOK && leftID == rightID
}

func scalarTypeName(node *t.NodeType) (string, bool) {
	if node == nil {
		return "", false
	}
	named, ok := node.KindNode.(*t.NodeTypeNamed)
	if !ok {
		return "", false
	}
	name, ok := named.NameNode.(*t.NodeNameSingle)
	if !ok {
		return "", false
	}
	return name.Name, true
}

func (c *scalarFunction) literal(node *t.NodeExprLit, expected *t.NodeType) (lb.ValueID, error) {
	if node.LitType == t.TokLitStr {
		return c.stringLiteral(node.Value)
	}
	typeNode := node.InfType
	if typeNode == nil {
		typeNode = expected
	}
	typeID, err := c.types.Lower(typeNode)
	if err != nil {
		return 0, err
	}
	spec := lb.ConstantSpec{Type: typeID}
	switch node.LitType {
	case t.TokLitBool:
		spec.Kind = lb.ConstantInteger
		if node.Value == "true" || node.Value == "1" {
			spec.Integer = "1"
		} else if node.Value == "false" || node.Value == "0" {
			spec.Integer = "0"
		} else {
			return 0, fmt.Errorf("invalid bool literal %q", node.Value)
		}
	case t.TokLitNone:
		spec.Kind = lb.ConstantNull
	case t.TokLitNum:
		name, ok := scalarTypeName(typeNode)
		description, numeric := magmatypes.NumberTypes[name]
		if !ok || !numeric {
			return 0, fmt.Errorf("numeric literal has no resolved numeric type")
		}
		if description.IsFloat {
			spec.Kind, spec.Float = lb.ConstantFloat, node.Value
		} else {
			spec.Kind = lb.ConstantInteger
			representation := strings.ReplaceAll(node.Value, "_", "")
			if strings.HasPrefix(representation, "u") {
				representation = strings.TrimPrefix(representation, "u")
			}
			base := 10
			unsigned := strings.TrimPrefix(representation, "-")
			if strings.HasPrefix(unsigned, "0x") || strings.HasPrefix(unsigned, "0X") || strings.HasPrefix(unsigned, "0b") || strings.HasPrefix(unsigned, "0B") || strings.HasPrefix(unsigned, "0o") || strings.HasPrefix(unsigned, "0O") {
				base = 0
			}
			integer, ok := new(big.Int).SetString(representation, base)
			if !ok {
				return 0, fmt.Errorf("invalid integer literal %q", node.Value)
			}
			if integer.Sign() < 0 {
				integer.Add(integer, new(big.Int).Lsh(big.NewInt(1), uint(description.ByteSize)))
			}
			spec.Integer = integer.String()
		}
	default:
		return 0, fmt.Errorf("unsupported scalar literal kind %s", t.TokTypeToRepr[node.LitType])
	}
	constant, err := c.backend.InternConstant(spec)
	if err != nil {
		return 0, err
	}
	return c.backend.ConstantValue(constant)
}

func (c *scalarFunction) stringLiteral(value string) (lb.ValueID, error) {
	i8, err := c.types.Lower(&t.NodeType{KindNode: &t.NodeTypeNamed{NameNode: &t.NodeNameSingle{Name: "u8"}}})
	if err != nil {
		return 0, err
	}
	array, err := c.backend.InternType(lb.TypeSpec{Kind: lb.TypeArray, Element: i8, Length: uint64(len(value) + 1)})
	if err != nil {
		return 0, err
	}
	constant, err := c.backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantString, Type: array, Bytes: value, NullTerminated: true})
	if err != nil {
		return 0, err
	}
	digest := sha256.Sum256([]byte(value))
	global, err := c.backend.DeclareGlobal(lb.GlobalSpec{Symbol: fmt.Sprintf(".magma.str.%x", digest), Type: array, Initializer: constant, Linkage: lb.LinkagePrivate, Visibility: lb.VisibilityDefault, Constant: true, UnnamedAddress: true, Definition: true})
	if err != nil {
		return 0, err
	}
	data, err := c.backend.GlobalAddress(global)
	if err != nil {
		return 0, err
	}
	u64, err := c.types.Lower(&t.NodeType{KindNode: &t.NodeTypeNamed{NameNode: &t.NodeNameSingle{Name: "u64"}}})
	if err != nil {
		return 0, err
	}
	lengthConstant, err := c.backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: u64, Integer: fmt.Sprint(len(value))})
	if err != nil {
		return 0, err
	}
	length, err := c.backend.ConstantValue(lengthConstant)
	if err != nil {
		return 0, err
	}
	return c.types.BuildCoreValueWithDefaults(c.block, t.CoreTypeString, map[string]lb.ValueID{"__data": data, "__byteCount": length})
}

func (c *scalarFunction) array(node *t.NodeExprArray) (lb.ValueID, error) {
	if node == nil || node.ElemType == nil || node.Length == nil || node.LengthType == nil {
		return 0, fmt.Errorf("array expression is incomplete")
	}
	u64Node := &t.NodeType{KindNode: &t.NodeTypeNamed{NameNode: &t.NodeNameSingle{Name: "u64"}}}
	u64, err := c.types.Lower(u64Node)
	if err != nil {
		return 0, err
	}
	length, err := c.expression(node.Length, node.LengthType)
	if err != nil {
		return 0, err
	}
	length, err = c.coerce(length, node.Length.GetInferredType(), u64Node)
	if err != nil {
		return 0, err
	}
	element, err := c.types.Lower(node.ElemType)
	if err != nil {
		return 0, err
	}
	data, err := c.backend.DynamicAlloca(c.block, element, length, 0)
	if err != nil {
		return 0, err
	}
	layout, err := c.backend.TypeLayout(element)
	if err != nil {
		return 0, fmt.Errorf("array element layout: %w", err)
	}
	sizeConstant, err := c.backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: u64, Integer: fmt.Sprint(layout.AllocationSize)})
	if err != nil {
		return 0, err
	}
	size, err := c.backend.ConstantValue(sizeConstant)
	if err != nil {
		return 0, err
	}
	total, err := c.backend.Binary(c.block, lb.BinaryMul, size, length)
	if err != nil {
		return 0, err
	}
	if err := c.zeroMemory(data, total); err != nil {
		return 0, err
	}
	for _, entry := range node.Entries {
		value, err := c.expression(entry.Value, node.ElemType)
		if err != nil {
			return 0, err
		}
		value, err = c.coerce(value, entry.Value.GetInferredType(), node.ElemType)
		if err != nil {
			return 0, err
		}
		indexConstant, err := c.backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: u64, Integer: fmt.Sprint(entry.ResolvedIndex)})
		if err != nil {
			return 0, err
		}
		index, err := c.backend.ConstantValue(indexConstant)
		if err != nil {
			return 0, err
		}
		address, err := c.backend.GEP(c.block, element, data, []lb.ValueID{index}, false)
		if err != nil {
			return 0, err
		}
		if _, err := c.backend.Store(c.block, value, address, 0, false); err != nil {
			return 0, err
		}
	}
	return c.types.BuildSlice(c.block, data, length)
}

func (c *scalarFunction) zeroMemory(pointer, size lb.ValueID) error {
	void, err := c.types.Lower(&t.NodeType{KindNode: &t.NodeTypeNamed{NameNode: &t.NodeNameSingle{Name: "void"}}})
	if err != nil {
		return err
	}
	i8, err := c.types.Lower(&t.NodeType{KindNode: &t.NodeTypeNamed{NameNode: &t.NodeNameSingle{Name: "u8"}}})
	if err != nil {
		return err
	}
	u64, err := c.types.Lower(&t.NodeType{KindNode: &t.NodeTypeNamed{NameNode: &t.NodeNameSingle{Name: "u64"}}})
	if err != nil {
		return err
	}
	boolean, err := c.types.Lower(&t.NodeType{KindNode: &t.NodeTypeNamed{NameNode: &t.NodeNameSingle{Name: "bool"}}})
	if err != nil {
		return err
	}
	pointerType, err := c.types.Lower(&t.NodeType{KindNode: &t.NodeTypeNamed{NameNode: &t.NodeNameSingle{Name: "ptr"}}})
	if err != nil {
		return err
	}
	zeroConstant, err := c.backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: i8, Integer: "0"})
	if err != nil {
		return err
	}
	zero, err := c.backend.ConstantValue(zeroConstant)
	if err != nil {
		return err
	}
	falseConstant, err := c.backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: boolean, Integer: "0"})
	if err != nil {
		return err
	}
	nonVolatile, err := c.backend.ConstantValue(falseConstant)
	if err != nil {
		return err
	}
	intrinsic, err := c.backend.DeclareFunction(lb.FunctionSpec{Symbol: "llvm.memset.p0.i64", Result: void, Parameters: []lb.TypeID{pointerType, i8, u64, boolean}, Linkage: lb.LinkageExternal, CallingConvention: lb.CallingConventionC})
	if err != nil {
		return err
	}
	_, err = c.backend.Call(c.block, intrinsic, []lb.ValueID{pointer, zero, size, nonVolatile})
	return err
}

func (c *scalarFunction) local(variable *t.NodeExprVarDef) (lb.ValueID, error) {
	if variable == nil || variable.Storage != t.VariableStorageLocal {
		return 0, fmt.Errorf("scalar local has unresolved storage")
	}
	if storage := c.slots[variable]; storage != 0 {
		return storage, nil
	}
	storage, err := c.types.CreateLocal(c.fn, c.block, variable.Type)
	if err == nil {
		c.slots[variable] = storage
	}
	return storage, err
}

func (c *scalarFunction) name(node *t.NodeExprName) (lb.ValueID, error) {
	if variable, ok := node.AssociatedNode.(*t.NodeExprVarDef); ok {
		initializer := variable.Initializer
		constantInitializer := c.globalConstantInitializer(variable)
		if initializer == nil {
			initializer = constantInitializer
		}
		if initializer != nil && (variable.IsConst || constantInitializer != nil) {
			value, err := c.expression(initializer, variable.Type)
			if err != nil {
				return 0, err
			}
			return c.coerce(value, initializer.GetInferredType(), variable.Type)
		}
	}
	if constant, ok := node.AssociatedNode.(*t.NodeConstDef); ok && constant.VarDef != nil && constant.Initializer != nil {
		value, err := c.expression(constant.Initializer, constant.VarDef.Type)
		if err != nil {
			return 0, err
		}
		return c.coerce(value, constant.Initializer.GetInferredType(), constant.VarDef.Type)
	}
	if len(node.MemberAccesses) != 0 {
		address, err := c.lvalue(node)
		if err != nil {
			return 0, err
		}
		resultType := node.MemberAccesses[len(node.MemberAccesses)-1].Type
		typeID, err := c.types.Lower(resultType)
		if err != nil {
			return 0, err
		}
		return c.backend.Load(c.block, typeID, address, 0, false)
	}
	storage, semanticType, err := c.nameStorage(node)
	if err != nil {
		return 0, err
	}
	if name, ok := variableName(node); ok && c.directArgs[name] {
		return storage, nil
	}
	typeID, err := c.types.Lower(semanticType)
	if err != nil {
		return 0, err
	}
	return c.backend.Load(c.block, typeID, storage, 0, false)
}

func (c *scalarFunction) globalConstantInitializer(variable *t.NodeExprVarDef) t.NodeExpr {
	if variable == nil {
		return nil
	}
	state := c.types.State()
	state.FilesM.Lock()
	defer state.FilesM.Unlock()
	for _, file := range state.Files {
		if file == nil || file.GlNode == nil {
			continue
		}
		for _, declaration := range file.GlNode.Declarations {
			if constant, ok := declaration.(*t.NodeConstDef); ok && constant.VarDef != nil {
				if constant.VarDef == variable || variable.AbsName != "" && constant.VarDef.AbsName == variable.AbsName {
					return constant.Initializer
				}
				variableName, variableNamed := variable.Name.(*t.NodeNameSingle)
				constantName, constantNamed := constant.VarDef.Name.(*t.NodeNameSingle)
				if variableNamed && constantNamed && variableName.Name == constantName.Name && c.definition != nil && strings.HasPrefix(c.definition.AbsName, file.PackageName+".") {
					return constant.Initializer
				}
			}
		}
	}
	return nil
}

func (c *scalarFunction) lvalue(expression t.NodeExpr) (lb.ValueID, error) {
	switch node := expression.(type) {
	case *t.NodeExprName:
		storage, semanticType, err := c.nameStorage(node)
		if err != nil {
			return 0, err
		}
		current := storage
		currentType := semanticType
		currentIsPointerValue := false
		if name, ok := variableName(node); ok {
			currentIsPointerValue = c.directArgs[name]
		}
		// Pointer locals are materialized as slots containing the pointer.  A
		// member path starts from the pointee, so recover the stored pointer
		// before taking the first field address.  Direct SSA receivers already
		// are pointer values and must not be loaded again.
		if len(node.MemberAccesses) > 0 && isPointerSemantic(currentType) && !currentIsPointerValue {
			pointerType, err := c.types.Lower(currentType)
			if err != nil {
				return 0, err
			}
			current, err = c.backend.Load(c.block, pointerType, current, 0, false)
			if err != nil {
				return 0, err
			}
			currentIsPointerValue = true
		}
		for _, access := range node.MemberAccesses {
			if access == nil || access.OwnerType == nil || access.Type == nil || access.FieldNb < 0 {
				return 0, fmt.Errorf("name has incomplete member access metadata")
			}
			if access.PtrDeref && !currentIsPointerValue {
				pointerType, err := c.types.Lower(currentType)
				if err != nil {
					return 0, err
				}
				current, err = c.backend.Load(c.block, pointerType, current, 0, false)
				if err != nil {
					return 0, err
				}
			}
			owner, err := c.types.Lower(aggregateOwner(access.OwnerType))
			if err != nil {
				return 0, err
			}
			current, err = c.backend.StructFieldAddress(c.block, owner, current, uint32(access.FieldNb))
			if err != nil {
				return 0, err
			}
			currentType = access.Type
			currentIsPointerValue = false
		}
		return current, nil
	case *t.NodeExprMemberAccess:
		if node.Access == nil || node.Target == nil || node.Access.FieldNb < 0 {
			return 0, fmt.Errorf("member lvalue is incomplete")
		}
		base, err := c.lvalue(node.Target)
		if err != nil {
			value, valueErr := c.expression(node.Target, node.Target.GetInferredType())
			if valueErr != nil {
				return 0, err
			}
			typeID, typeErr := c.types.Lower(node.Target.GetInferredType())
			if typeErr != nil {
				return 0, typeErr
			}
			base, typeErr = c.backend.StaticAlloca(c.fn, typeID, 0)
			if typeErr != nil {
				return 0, typeErr
			}
			if _, typeErr = c.backend.Store(c.block, value, base, 0, false); typeErr != nil {
				return 0, typeErr
			}
		}
		if isPointerSemantic(node.Target.GetInferredType()) {
			pointerType, err := c.types.Lower(node.Target.GetInferredType())
			if err != nil {
				return 0, err
			}
			base, err = c.backend.Load(c.block, pointerType, base, 0, false)
			if err != nil {
				return 0, err
			}
		}
		owner, err := c.types.Lower(aggregateOwner(node.Access.OwnerType))
		if err != nil {
			return 0, err
		}
		return c.backend.StructFieldAddress(c.block, owner, base, uint32(node.Access.FieldNb))
	case *t.NodeExprSubscript:
		return c.subscriptAddress(node)
	case *t.NodeExprUnary:
		if node.Operator != t.KwAsterisk || !node.ProvenanceChecked {
			return 0, fmt.Errorf("pointer lvalue lacks provenance analysis")
		}
		return c.expression(node.Operand, node.Operand.GetInferredType())
	default:
		return 0, fmt.Errorf("expression %T is not an lvalue", expression)
	}
}

func aggregateOwner(node *t.NodeType) *t.NodeType {
	if node != nil {
		if pointer, ok := node.KindNode.(*t.NodeTypePointer); ok {
			return &t.NodeType{KindNode: pointer.Kind}
		}
	}
	return node
}

func isPointerSemantic(node *t.NodeType) bool {
	if node == nil {
		return false
	}
	switch value := node.KindNode.(type) {
	case *t.NodeTypePointer, *t.NodeTypeRfc:
		return true
	case *t.NodeTypeNamed:
		name, ok := value.NameNode.(*t.NodeNameSingle)
		return ok && name.Name == "ptr"
	default:
		return false
	}
}

func (c *scalarFunction) nameStorage(node *t.NodeExprName) (lb.ValueID, *t.NodeType, error) {
	variable := resolvedVariable(node)
	if variable == nil {
		return 0, nil, fmt.Errorf("name expression has no resolved variable")
	}
	if variable.IsImplicitContext && c.context != 0 {
		return c.context, variable.Type, nil
	}
	ssaReceiver := false
	if name, nameOK := variable.Name.(*t.NodeNameSingle); nameOK {
		ssaReceiver = variable.Storage == t.VariableStorageSSA && c.directArgs[name.Name]
	}
	if variable.Storage == t.VariableStorageArgument || ssaReceiver {
		name, nameOK := variable.Name.(*t.NodeNameSingle)
		if !nameOK || c.args[name.Name] == 0 {
			return 0, nil, fmt.Errorf("argument has no materialized slot")
		}
		return c.args[name.Name], variable.Type, nil
	}
	if variable.Storage == t.VariableStorageLocal && c.slots[variable] != 0 {
		return c.slots[variable], variable.Type, nil
	}
	if variable.Storage == t.VariableStorageGlobal && c.globals[variable] != 0 {
		return c.globals[variable], variable.Type, nil
	}
	if variable.Storage == t.VariableStorageGlobal {
		for declared, address := range c.globals {
			if declared != nil && address != 0 && variable.AbsName != "" && declared.AbsName == variable.AbsName {
				return address, variable.Type, nil
			}
		}
	}
	name, _ := variable.Name.(*t.NodeNameSingle)
	resolvedName := "<unnamed>"
	if name != nil {
		resolvedName = name.Name
	}
	return 0, nil, fmt.Errorf("unsupported or undefined storage %d for variable %q", variable.Storage, resolvedName)
}

func resolvedVariable(node *t.NodeExprName) *t.NodeExprVarDef {
	if node == nil {
		return nil
	}
	if variable, ok := node.AssociatedNode.(*t.NodeExprVarDef); ok {
		return variable
	}
	if assignment, ok := node.AssociatedNode.(*t.NodeExprVarDefAssign); ok {
		return assignment.VarDef
	}
	return nil
}

func implicitContextVariable(expression t.NodeExpr) *t.NodeExprVarDef {
	switch node := expression.(type) {
	case *t.NodeExprName:
		if variable := resolvedVariable(node); variable != nil && variable.IsImplicitContext {
			return variable
		}
	case *t.NodeExprMemberAccess:
		return implicitContextVariable(node.Target)
	}
	return nil
}

func (c *scalarFunction) materializeContext(variable *t.NodeExprVarDef) error {
	if variable == nil || variable.Type == nil {
		return fmt.Errorf("implicit context has no resolved type")
	}
	if slot := c.slots[variable]; slot != 0 {
		c.context = slot
		return nil
	}
	contextType, err := c.types.Lower(variable.Type)
	if err != nil {
		return err
	}
	slot, err := c.backend.StaticAlloca(c.fn, contextType, 0)
	if err != nil {
		return err
	}
	if c.context != 0 {
		value, err := c.backend.Load(c.entry, contextType, c.context, 0, false)
		if err != nil {
			return err
		}
		if _, err := c.backend.Store(c.entry, value, slot, 0, false); err != nil {
			return err
		}
	}
	c.slots[variable] = slot
	c.context = slot
	return nil
}

func variableName(node *t.NodeExprName) (string, bool) {
	if node == nil {
		return "", false
	}
	variable, ok := node.AssociatedNode.(*t.NodeExprVarDef)
	if !ok {
		if assignment, assignmentOK := node.AssociatedNode.(*t.NodeExprVarDefAssign); assignmentOK && assignment.VarDef != nil {
			variable, ok = assignment.VarDef, true
		}
	}
	if !ok || variable == nil {
		return "", false
	}
	name, ok := variable.Name.(*t.NodeNameSingle)
	if !ok {
		return "", false
	}
	return name.Name, true
}
