// Package loweringprogram assembles a checked Magma program through the
// backend-neutral object-lowering boundary.
package loweringprogram

import (
	"crypto/sha256"
	"fmt"
	"math/big"
	"sort"
	"strings"

	llvmir "Magma/src/llvm_ir"
	loweringast "Magma/src/lowering_ast"
	lb "Magma/src/lowering_backend"
	loweringcabi "Magma/src/lowering_cabi"
	loweringruntime "Magma/src/lowering_runtime"
	loweringtypes "Magma/src/lowering_types"
	magmatypes "Magma/src/magma_types"
	t "Magma/src/types"
)

// Build lowers all concrete checked functions in deterministic symbol order
// and installs the canonical runtime utilities in the same object module.
// Unsupported constructs return an error; callers must not fall back silently.
func Build(backend lb.Backend, state *t.SharedState) error {
	if backend == nil || state == nil {
		return fmt.Errorf("whole-program lowering requires a backend and checked state")
	}
	types, err := loweringtypes.New(backend, state)
	if err != nil {
		return err
	}
	if err := types.ValidateProtoLayouts(); err != nil {
		return err
	}
	state.FilesM.Lock()
	files := make(map[string]*t.FileCtx, len(state.Files))
	for path, file := range state.Files {
		files[path] = file
	}
	state.FilesM.Unlock()
	coreMethodRoots := make([]*t.NodeFuncDef, 0, len(state.CoreMethods))
	// Some source operations lower to compiler-known core method calls (for
	// example string equality to str.compare) without an explicit call node in
	// the checked AST. Keep those synthetic callees as object roots.
	for _, method := range state.CoreMethods {
		if method != nil {
			coreMethodRoots = append(coreMethodRoots, method)
		}
	}
	// Taking a contextless function as a contextful function value creates an
	// adapter whose body calls the original function. That synthetic edge is
	// not present as an ordinary call expression, so retain its target here.
	for _, file := range files {
		if file == nil || file.GlNode == nil {
			continue
		}
		for _, declaration := range file.GlNode.Declarations {
			if function, ok := declaration.(*t.NodeFuncDef); ok && function.NeedsContextAdapter {
				coreMethodRoots = append(coreMethodRoots, function)
			}
		}
	}
	reachable := llvmir.ObjectReachableFunctions(files, state.NullContext, coreMethodRoots...)
	functions := make([]*t.NodeFuncDef, 0)
	contextAdapters := make([]*t.NodeFuncDef, 0)
	exports := make([]*t.NodeFuncDef, 0)
	globalDefinitions := make([]*t.NodeExprVarDef, 0)
	var entryPoint, contextInitializer, printUncaught *t.NodeFuncDef
	for _, file := range files {
		if file == nil || file.GlNode == nil {
			continue
		}
		for _, declaration := range file.GlNode.Declarations {
			switch node := declaration.(type) {
			case *t.NodeFuncDef:
				if node.ExportName != "" {
					exports = append(exports, node)
				}
				if node.NeedsContextAdapter {
					contextAdapters = append(contextAdapters, node)
				}
				if node.IsEntryPoint && file.PackageName == file.MainPckgName {
					entryPoint = node
				}
				if !node.IsExternal && reachable[node] {
					functions = append(functions, node)
				}
			case *t.NodeExprVarDef:
				globalDefinitions = append(globalDefinitions, node)
			case *t.NodeConstDef:
				// Constant declarations keep their initializer on the declaration
				// node. Mirror it onto the associated variable identity consumed by
				// expression lowering so constants are materialized at their use.
				if node.VarDef != nil {
					node.VarDef.IsConst = true
					node.VarDef.Initializer = node.Initializer
					globalDefinitions = append(globalDefinitions, node.VarDef)
				}
			}
		}
		if file.ModuleName == "context_default" {
			name := "newDefault"
			if state.NullContext {
				name = "newNull"
			}
			contextInitializer = file.GlNode.FuncDefs[name]
		}
		if file.ModuleName == "errors" {
			printUncaught = file.GlNode.FuncDefs["printUncaught"]
		}
	}
	sort.SliceStable(globalDefinitions, func(i, j int) bool { return globalSymbol(globalDefinitions[i]) < globalSymbol(globalDefinitions[j]) })
	globals := make(map[*t.NodeExprVarDef]lb.ValueID, len(globalDefinitions))
	for _, variable := range globalDefinitions {
		typeID, err := types.Lower(variable.Type)
		if err != nil {
			return fmt.Errorf("global %s: %w", globalSymbol(variable), err)
		}
		symbol := globalSymbol(variable)
		if symbol == "" {
			return fmt.Errorf("global variable has no resolved symbol")
		}
		if array, ok := variable.Initializer.(*t.NodeExprArray); ok && !variable.IsExternal {
			global, err := declareArrayGlobal(backend, types, state, variable, array, symbol, typeID)
			if err != nil {
				return fmt.Errorf("global %s: %w", symbol, err)
			}
			globals[variable], err = backend.GlobalAddress(global)
			if err != nil {
				return err
			}
			continue
		}
		spec := lb.GlobalSpec{Symbol: symbol, Type: typeID, Linkage: lb.LinkagePrivate, Visibility: lb.VisibilityDefault, ThreadLocal: !variable.IsProcessGlobal && !variable.IsConst, Constant: variable.IsConst}
		if variable.IsExternal {
			spec.Linkage = lb.LinkageExternal
		} else {
			initializer, err := globalConstant(backend, types, variable.Type, variable.Initializer)
			if err != nil {
				return fmt.Errorf("global %s: %w", symbol, err)
			}
			spec.Initializer = initializer
			spec.Definition = true
		}
		global, err := backend.DeclareGlobal(spec)
		if err != nil {
			return err
		}
		globals[variable], err = backend.GlobalAddress(global)
		if err != nil {
			return err
		}
	}
	var rootAddress lb.ValueID
	needsRoot := entryPoint != nil
	for _, function := range functions {
		needsRoot = needsRoot || function.NeedsNativeContextThunk
	}
	for _, function := range exports {
		needsRoot = needsRoot || function.ContextABI == t.ContextABIContextful
	}
	if needsRoot {
		rootAddress, err = declareContextRoot(backend, types, contextInitializer)
		if err != nil {
			return fmt.Errorf("declare root context: %w", err)
		}
	}
	sort.SliceStable(functions, func(i, j int) bool {
		return functions[i].AbsName < functions[j].AbsName
	})
	functionIDs := make(map[*t.NodeFuncDef]lb.FunctionID, len(functions))
	for _, function := range functions {
		if function.AbsName == "" {
			return fmt.Errorf("checked function has no resolved symbol")
		}
		functionIDs[function], err = loweringast.LowerFunctionWithGlobals(backend, types, function, globals)
		if err != nil {
			return fmt.Errorf("lower %s: %w", function.AbsName, err)
		}
	}
	sort.SliceStable(contextAdapters, func(i, j int) bool { return contextAdapters[i].AbsName < contextAdapters[j].AbsName })
	for _, function := range contextAdapters {
		if err := buildContextAdapter(backend, types, function, functionIDs); err != nil {
			return fmt.Errorf("build context adapter for %s: %w", function.AbsName, err)
		}
	}
	for _, function := range functions {
		if function.NeedsNativeContextThunk {
			if err := buildNativeContextThunk(backend, types, state, function, contextInitializer, printUncaught, functionIDs, rootAddress); err != nil {
				return fmt.Errorf("build native context thunk for %s: %w", function.AbsName, err)
			}
		}
	}
	sort.SliceStable(exports, func(i, j int) bool { return exports[i].ExportName < exports[j].ExportName })
	for _, function := range exports {
		if err := buildCExportWrapper(backend, types, state, function, contextInitializer, printUncaught, functionIDs, rootAddress); err != nil {
			return fmt.Errorf("build C export %s: %w", function.ExportName, err)
		}
	}
	utils, err := loweringruntime.BuildUtils(backend)
	if err != nil {
		return fmt.Errorf("build runtime utilities: %w", err)
	}
	if entryPoint != nil {
		var windowsArgs *loweringruntime.WindowsArgs
		if state.Target.OS == "windows" && len(entryPoint.Class.ArgsNode.Args) == 1 {
			helpers, err := loweringruntime.BuildWindowsArgs(backend, types)
			if err != nil {
				return fmt.Errorf("build Windows argument helpers: %w", err)
			}
			windowsArgs = &helpers
		}
		if err := buildEntryWrapper(backend, types, state, entryPoint, contextInitializer, printUncaught, functionIDs, utils, windowsArgs, rootAddress); err != nil {
			return fmt.Errorf("build native entry point: %w", err)
		}
	}
	return backend.Verify()
}

func declareContextRoot(backend lb.Backend, types *loweringtypes.Lowerer, initializer *t.NodeFuncDef) (lb.ValueID, error) {
	if initializer == nil || initializer.ReturnType == nil {
		return 0, fmt.Errorf("root context initializer is missing")
	}
	ordinaryContext := *initializer.ReturnType
	ordinaryContext.Throws = false
	contextType, err := types.Lower(&ordinaryContext)
	if err != nil {
		return 0, err
	}
	zeroContext, err := backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantZero, Type: contextType})
	if err != nil {
		return 0, err
	}
	root, err := backend.DeclareGlobal(lb.GlobalSpec{Symbol: "magma.context.root", Type: contextType, Initializer: zeroContext, Linkage: lb.LinkageInternal, Visibility: lb.VisibilityDefault, ThreadLocal: true, Definition: true})
	if err != nil {
		return 0, err
	}
	return backend.GlobalAddress(root)
}

func buildEntryWrapper(backend lb.Backend, types *loweringtypes.Lowerer, state *t.SharedState, entry, initializer, printUncaught *t.NodeFuncDef, functions map[*t.NodeFuncDef]lb.FunctionID, utils loweringruntime.Utils, windowsArgs *loweringruntime.WindowsArgs, rootAddress lb.ValueID) error {
	if initializer == nil || functions[initializer] == 0 {
		return fmt.Errorf("root context initializer is not reachable")
	}
	i32, err := backend.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 32})
	if err != nil {
		return err
	}
	pointer, err := backend.InternType(lb.TypeSpec{Kind: lb.TypePointer})
	if err != nil {
		return err
	}
	entrySymbol := "main"
	if state.Target.OS == "windows" {
		entrySymbol = "wmain"
	}
	wrapper, err := backend.DeclareFunction(lb.FunctionSpec{Symbol: entrySymbol, Result: i32, Parameters: []lb.TypeID{i32, pointer}, Linkage: lb.LinkageExternal, CallingConvention: lb.CallingConventionC, Definition: true})
	if err != nil {
		return err
	}
	block, err := backend.AppendBlock(wrapper, "entry")
	if err != nil {
		return err
	}
	argc, err := backend.Parameter(wrapper, 0)
	if err != nil {
		return err
	}
	argv, err := backend.Parameter(wrapper, 1)
	if err != nil {
		return err
	}
	if state.Target.OS == "windows" {
		setConsoleOutputCP, err := backend.DeclareFunction(lb.FunctionSpec{Symbol: "SetConsoleOutputCP", Result: i32, Parameters: []lb.TypeID{i32}, Linkage: lb.LinkageExternal, CallingConvention: lb.CallingConventionC})
		if err != nil {
			return err
		}
		utf8CodePage, err := integerValue(backend, i32, 65001)
		if err != nil {
			return err
		}
		if _, err := backend.Call(block, setConsoleOutputCP, []lb.ValueID{utf8CodePage}); err != nil {
			return err
		}
	}
	initialized, err := backend.Call(block, functions[initializer], nil)
	if err != nil {
		return err
	}
	if initializer.ReturnType.Throws {
		errorValue, err := backend.ExtractValue(block, initialized, []uint32{0})
		if err != nil {
			return err
		}
		codeField, err := coreFieldIndex(state, t.CoreTypeError, "__code")
		if err != nil {
			return err
		}
		code, err := backend.ExtractValue(block, errorValue, []uint32{codeField})
		if err != nil {
			return err
		}
		zero, err := integerValue(backend, i32, 0)
		if err != nil {
			return err
		}
		failed, err := backend.Compare(block, lb.CompareNotEqual, code, zero)
		if err != nil {
			return err
		}
		failure, err := backend.AppendBlock(wrapper, "ctx.init.failed")
		if err != nil {
			return err
		}
		ready, err := backend.AppendBlock(wrapper, "ctx.init.ready")
		if err != nil {
			return err
		}
		if err := backend.CondBranch(block, failed, failure, ready); err != nil {
			return err
		}
		if printUncaught == nil || functions[printUncaught] == 0 {
			return fmt.Errorf("uncaught-error reporter is not reachable")
		}
		if _, err := backend.Call(failure, functions[printUncaught], []lb.ValueID{errorValue}); err != nil {
			return err
		}
		if err := backend.Return(failure, code); err != nil {
			return err
		}
		initialized, err = backend.ExtractValue(ready, initialized, []uint32{1})
		if err != nil {
			return err
		}
		block = ready
	}
	if _, err := backend.Store(block, initialized, rootAddress, 0, false); err != nil {
		return err
	}
	arguments := make([]lb.ValueID, 0, len(entry.Class.ArgsNode.Args)+1)
	if entry.ContextABI == t.ContextABIContextful {
		arguments = append(arguments, rootAddress)
	}
	if len(entry.Class.ArgsNode.Args) > 1 {
		return fmt.Errorf("native entry point has more than one explicit argument")
	}
	var windowsBuffer lb.ValueID
	if len(entry.Class.ArgsNode.Args) == 1 {
		buffer, err := backend.DynamicAlloca(block, utils.String, argc, 0)
		if err != nil {
			return err
		}
		var argumentSlice lb.ValueID
		if state.Target.OS == "windows" {
			if windowsArgs == nil {
				return fmt.Errorf("Windows argument helpers are missing")
			}
			converted, err := backend.Call(block, windowsArgs.FromUTF16, []lb.ValueID{argc, argv, buffer})
			if err != nil {
				return err
			}
			failed, err := backend.AppendBlock(wrapper, "args.failed")
			if err != nil {
				return err
			}
			ready, err := backend.AppendBlock(wrapper, "args.ready")
			if err != nil {
				return err
			}
			if err := backend.CondBranch(block, converted, ready, failed); err != nil {
				return err
			}
			failureExit, err := integerValue(backend, i32, 1)
			if err != nil {
				return err
			}
			if err := backend.Return(failed, failureExit); err != nil {
				return err
			}
			i64, err := backend.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 64})
			if err != nil {
				return err
			}
			count, err := backend.Cast(ready, lb.CastSignExtend, argc, i64)
			if err != nil {
				return err
			}
			argumentSlice, err = types.BuildSlice(ready, buffer, count)
			if err != nil {
				return err
			}
			windowsBuffer = buffer
			block = ready
		} else {
			argumentSlice, err = backend.Call(block, utils.ArgsToSlice, []lb.ValueID{argc, argv, buffer})
			if err != nil {
				return err
			}
		}
		arguments = append(arguments, argumentSlice)
	}
	result, err := backend.Call(block, functions[entry], arguments)
	if err != nil {
		return err
	}
	zeroExit, err := integerValue(backend, i32, 0)
	if err != nil {
		return err
	}
	if !entry.ReturnType.Throws {
		if windowsBuffer != 0 {
			if _, err := backend.Call(block, windowsArgs.FreeUTF8, []lb.ValueID{argc, windowsBuffer}); err != nil {
				return err
			}
		}
		if err := backend.Return(block, zeroExit); err != nil {
			return err
		}
		return backend.FinalizeFunction(wrapper)
	}
	errorValue, err := backend.ExtractValue(block, result, []uint32{0})
	if err != nil {
		return err
	}
	codeField, err := coreFieldIndex(state, t.CoreTypeError, "__code")
	if err != nil {
		return err
	}
	code, err := backend.ExtractValue(block, errorValue, []uint32{codeField})
	if err != nil {
		return err
	}
	failed, err := backend.Compare(block, lb.CompareNotEqual, code, zeroExit)
	if err != nil {
		return err
	}
	failure, err := backend.AppendBlock(wrapper, "main.failed")
	if err != nil {
		return err
	}
	success, err := backend.AppendBlock(wrapper, "main.success")
	if err != nil {
		return err
	}
	if err := backend.CondBranch(block, failed, failure, success); err != nil {
		return err
	}
	if printUncaught == nil || functions[printUncaught] == 0 {
		return fmt.Errorf("uncaught-error reporter is not reachable")
	}
	if _, err := backend.Call(failure, functions[printUncaught], []lb.ValueID{errorValue}); err != nil {
		return err
	}
	if windowsBuffer != 0 {
		if _, err := backend.Call(failure, windowsArgs.FreeUTF8, []lb.ValueID{argc, windowsBuffer}); err != nil {
			return err
		}
	}
	if err := backend.Return(failure, code); err != nil {
		return err
	}
	if windowsBuffer != 0 {
		if _, err := backend.Call(success, windowsArgs.FreeUTF8, []lb.ValueID{argc, windowsBuffer}); err != nil {
			return err
		}
	}
	if err := backend.Return(success, zeroExit); err != nil {
		return err
	}
	return backend.FinalizeFunction(wrapper)
}

func buildContextAdapter(backend lb.Backend, types *loweringtypes.Lowerer, function *t.NodeFuncDef, functions map[*t.NodeFuncDef]lb.FunctionID) error {
	if function == nil || function.ReturnType == nil || function.AbsName == "" {
		return fmt.Errorf("context adapter target is incomplete")
	}
	if function.ContextABI != t.ContextABIContextless {
		return fmt.Errorf("context adapter target must be noctx")
	}
	result, err := types.Lower(function.ReturnType)
	if err != nil {
		return err
	}
	pointer, err := backend.InternType(lb.TypeSpec{Kind: lb.TypePointer})
	if err != nil {
		return err
	}
	targetParameters := make([]lb.TypeID, 0, len(function.Class.ArgsNode.Args))
	argumentStart := 0
	if function.IsMember {
		targetParameters = append(targetParameters, pointer)
		argumentStart = 1
	}
	for _, argument := range function.Class.ArgsNode.Args[argumentStart:] {
		parameter, err := types.Lower(argument.TypeNode)
		if err != nil {
			return err
		}
		targetParameters = append(targetParameters, parameter)
	}
	target := functions[function]
	if target == 0 {
		symbol := function.AbsName
		linkage := lb.LinkageInternal
		if function.IsExternal || function.NoAliasName != "" {
			linkage = lb.LinkageExternal
			if function.NoAliasName != "" {
				symbol = function.NoAliasName
			}
		}
		target, err = backend.DeclareFunction(lb.FunctionSpec{Symbol: symbol, Result: result, Parameters: targetParameters, Linkage: linkage, CallingConvention: lb.CallingConventionC})
		if err != nil {
			return err
		}
	}
	adapterParameters := append([]lb.TypeID{pointer}, targetParameters...)
	adapter, err := backend.DeclareFunction(lb.FunctionSpec{
		Symbol:            function.AbsName + ".__ctx_adapter",
		Result:            result,
		Parameters:        adapterParameters,
		Linkage:           lb.LinkageInternal,
		CallingConvention: lb.CallingConventionC,
		Attributes:        []lb.AttributeSpec{{Kind: lb.AttributeAlwaysInline, Placement: lb.AttributeFunction}},
		Definition:        true,
	})
	if err != nil {
		return err
	}
	entry, err := backend.AppendBlock(adapter, "entry")
	if err != nil {
		return err
	}
	arguments := make([]lb.ValueID, 0, len(targetParameters))
	for index := range targetParameters {
		argument, err := backend.Parameter(adapter, index+1)
		if err != nil {
			return err
		}
		arguments = append(arguments, argument)
	}
	value, err := backend.Call(entry, target, arguments)
	if err != nil {
		return err
	}
	if isVoidResult(function.ReturnType) {
		if err := backend.ReturnVoid(entry); err != nil {
			return err
		}
	} else if err := backend.Return(entry, value); err != nil {
		return err
	}
	return backend.FinalizeFunction(adapter)
}

func buildCExportWrapper(backend lb.Backend, types *loweringtypes.Lowerer, state *t.SharedState, function, initializer, printUncaught *t.NodeFuncDef, functions map[*t.NodeFuncDef]lb.FunctionID, rootAddress lb.ValueID) error {
	if function == nil || function.ExportName == "" || function.ExportABI != "C" || functions[function] == 0 {
		return fmt.Errorf("export definition is incomplete or has an unsupported ABI")
	}
	returnPlan, err := loweringcabi.Classify(backend, types, state, function.ReturnType, true)
	if err != nil {
		return fmt.Errorf("return type: %w", err)
	}
	result, err := loweringcabi.PhysicalResult(backend, returnPlan)
	if err != nil {
		return err
	}
	pointer, err := backend.InternType(lb.TypeSpec{Kind: lb.TypePointer})
	if err != nil {
		return err
	}
	argumentPlans := make([]loweringcabi.Value, len(function.Class.ArgsNode.Args))
	argumentStarts := make([]int, len(argumentPlans))
	parameterTypes := make([]lb.TypeID, 0, len(argumentPlans)+2)
	attributes := make([]lb.AttributeSpec, 0)
	returnStorageParameter := -1
	if returnPlan.Class == loweringcabi.Indirect {
		returnStorageParameter = len(parameterTypes)
		parameterTypes = append(parameterTypes, pointer)
		attributes = append(attributes,
			lb.AttributeSpec{Kind: lb.AttributeStructReturn, Placement: lb.AttributeParameter, Parameter: uint32(returnStorageParameter), Type: returnPlan.Logical},
			lb.AttributeSpec{Kind: lb.AttributeAlignment, Placement: lb.AttributeParameter, Parameter: uint32(returnStorageParameter), Value: uint64(returnPlan.Alignment)},
		)
	}
	for index, argument := range function.Class.ArgsNode.Args {
		argumentPlans[index], err = loweringcabi.Classify(backend, types, state, argument.TypeNode, false)
		if err != nil {
			return fmt.Errorf("argument %d: %w", index+1, err)
		}
		argumentStarts[index] = len(parameterTypes)
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
	wrapper, err := backend.DeclareFunction(lb.FunctionSpec{Symbol: function.ExportName, Result: result, Parameters: parameterTypes, Linkage: lb.LinkageExternal, CallingConvention: lb.CallingConventionC, Attributes: attributes, Definition: true})
	if err != nil {
		return err
	}
	block, err := backend.AppendBlock(wrapper, "entry")
	if err != nil {
		return err
	}
	if function.ContextABI == t.ContextABIContextful {
		if rootAddress == 0 || initializer == nil || initializer.ContextABI != t.ContextABIContextless || functions[initializer] == 0 {
			return fmt.Errorf("contextful export requires a reachable noctx root initializer")
		}
		initialized, err := backend.Call(block, functions[initializer], nil)
		if err != nil {
			return err
		}
		if initializer.ReturnType.Throws {
			errorValue, err := backend.ExtractValue(block, initialized, []uint32{0})
			if err != nil {
				return err
			}
			codeField, err := coreFieldIndex(state, t.CoreTypeError, "__code")
			if err != nil {
				return err
			}
			code, err := backend.ExtractValue(block, errorValue, []uint32{codeField})
			if err != nil {
				return err
			}
			i32, err := backend.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 32})
			if err != nil {
				return err
			}
			zero, err := integerValue(backend, i32, 0)
			if err != nil {
				return err
			}
			failed, err := backend.Compare(block, lb.CompareNotEqual, code, zero)
			if err != nil {
				return err
			}
			failure, err := backend.AppendBlock(wrapper, "ctx.init.failed")
			if err != nil {
				return err
			}
			ready, err := backend.AppendBlock(wrapper, "ctx.init.ready")
			if err != nil {
				return err
			}
			if err := backend.CondBranch(block, failed, failure, ready); err != nil {
				return err
			}
			if printUncaught == nil || functions[printUncaught] == 0 {
				return fmt.Errorf("uncaught-error reporter is not reachable")
			}
			if _, err := backend.Call(failure, functions[printUncaught], []lb.ValueID{errorValue}); err != nil {
				return err
			}
			voidType, err := types.Lower(namedType("void"))
			if err != nil {
				return err
			}
			abort, err := backend.DeclareFunction(lb.FunctionSpec{Symbol: "abort", Result: voidType, Linkage: lb.LinkageExternal, CallingConvention: lb.CallingConventionC, Attributes: []lb.AttributeSpec{{Kind: lb.AttributeNoReturn, Placement: lb.AttributeFunction}}})
			if err != nil {
				return err
			}
			if _, err := backend.Call(failure, abort, nil); err != nil {
				return err
			}
			if err := backend.Unreachable(failure); err != nil {
				return err
			}
			initialized, err = backend.ExtractValue(ready, initialized, []uint32{1})
			if err != nil {
				return err
			}
			block = ready
		}
		if _, err := backend.Store(block, initialized, rootAddress, 0, false); err != nil {
			return err
		}
	}
	arguments := make([]lb.ValueID, 0, len(argumentPlans)+1)
	if function.ContextABI == t.ContextABIContextful {
		arguments = append(arguments, rootAddress)
	}
	for index, plan := range argumentPlans {
		start := argumentStarts[index]
		var argument lb.ValueID
		switch plan.Class {
		case loweringcabi.Indirect:
			address, err := backend.Parameter(wrapper, start)
			if err != nil {
				return err
			}
			argument, err = backend.Load(block, plan.Logical, address, plan.Alignment, false)
			if err != nil {
				return err
			}
		case loweringcabi.Coerce:
			argument, err = reconstructCABIValue(backend, block, wrapper, plan, start)
			if err != nil {
				return err
			}
		default:
			argument, err = backend.Parameter(wrapper, start)
		}
		if err != nil {
			return err
		}
		arguments = append(arguments, argument)
	}
	value, err := backend.Call(block, functions[function], arguments)
	if err != nil {
		return err
	}
	if returnPlan.Class == loweringcabi.Indirect {
		address, err := backend.Parameter(wrapper, returnStorageParameter)
		if err != nil {
			return err
		}
		if _, err := backend.Store(block, value, address, returnPlan.Alignment, false); err != nil {
			return err
		}
		if err := backend.ReturnVoid(block); err != nil {
			return err
		}
	} else if returnPlan.Class == loweringcabi.Coerce {
		physical, err := coerceCABIResult(backend, block, returnPlan, value)
		if err != nil {
			return err
		}
		if err := backend.Return(block, physical); err != nil {
			return err
		}
	} else if isVoidResult(function.ReturnType) {
		if err := backend.ReturnVoid(block); err != nil {
			return err
		}
	} else if err := backend.Return(block, value); err != nil {
		return err
	}
	return backend.FinalizeFunction(wrapper)
}

func reconstructCABIValue(backend lb.Backend, block lb.BlockID, function lb.FunctionID, plan loweringcabi.Value, parameterStart int) (lb.ValueID, error) {
	storage, err := backend.Alloca(block, plan.Logical, plan.Alignment)
	if err != nil {
		return 0, err
	}
	rawStorage, err := backend.ReinterpretPointer(storage)
	if err != nil {
		return 0, err
	}
	i8, err := backend.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 8})
	if err != nil {
		return 0, err
	}
	i64, err := backend.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 64})
	if err != nil {
		return 0, err
	}
	for index, part := range plan.Parts {
		address := rawStorage
		if part.Offset != 0 {
			offset, err := integerValue(backend, i64, part.Offset)
			if err != nil {
				return 0, err
			}
			address, err = backend.GEP(block, i8, rawStorage, []lb.ValueID{offset}, false)
			if err != nil {
				return 0, err
			}
		}
		address, err = backend.ReinterpretPointer(address)
		if err != nil {
			return 0, err
		}
		value, err := backend.Parameter(function, parameterStart+index)
		if err != nil {
			return 0, err
		}
		if _, err := backend.Store(block, value, address, 1, false); err != nil {
			return 0, err
		}
	}
	return backend.Load(block, plan.Logical, storage, plan.Alignment, false)
}

func coerceCABIResult(backend lb.Backend, block lb.BlockID, plan loweringcabi.Value, value lb.ValueID) (lb.ValueID, error) {
	storage, err := backend.Alloca(block, plan.Logical, plan.Alignment)
	if err != nil {
		return 0, err
	}
	if _, err := backend.Store(block, value, storage, plan.Alignment, false); err != nil {
		return 0, err
	}
	rawStorage, err := backend.ReinterpretPointer(storage)
	if err != nil {
		return 0, err
	}
	i8, err := backend.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 8})
	if err != nil {
		return 0, err
	}
	i64, err := backend.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 64})
	if err != nil {
		return 0, err
	}
	parts := make([]lb.ValueID, len(plan.Parts))
	for index, part := range plan.Parts {
		address := rawStorage
		if part.Offset != 0 {
			offset, err := integerValue(backend, i64, part.Offset)
			if err != nil {
				return 0, err
			}
			address, err = backend.GEP(block, i8, rawStorage, []lb.ValueID{offset}, false)
			if err != nil {
				return 0, err
			}
		}
		address, err = backend.ReinterpretPointer(address)
		if err != nil {
			return 0, err
		}
		parts[index], err = backend.Load(block, part.Type, address, 1, false)
		if err != nil {
			return 0, err
		}
	}
	if len(parts) == 1 {
		return parts[0], nil
	}
	physical, err := loweringcabi.PhysicalResult(backend, plan)
	if err != nil {
		return 0, err
	}
	return backend.BuildAggregate(block, physical, parts)
}

func buildNativeContextThunk(backend lb.Backend, types *loweringtypes.Lowerer, state *t.SharedState, function, initializer, printUncaught *t.NodeFuncDef, functions map[*t.NodeFuncDef]lb.FunctionID, rootAddress lb.ValueID) error {
	if rootAddress == 0 {
		return fmt.Errorf("native callback requires the thread-local root context")
	}
	if initializer == nil || initializer.ContextABI != t.ContextABIContextless || functions[initializer] == 0 {
		return fmt.Errorf("root context initializer must be a reachable noctx function")
	}
	resultType, err := types.Lower(function.ReturnType)
	if err != nil {
		return err
	}
	pointer, err := backend.InternType(lb.TypeSpec{Kind: lb.TypePointer})
	if err != nil {
		return err
	}
	parameterTypes := make([]lb.TypeID, 0, len(function.Class.ArgsNode.Args))
	argumentStart := 0
	if function.IsMember {
		parameterTypes = append(parameterTypes, pointer)
		argumentStart = 1
	}
	for _, argument := range function.Class.ArgsNode.Args[argumentStart:] {
		parameterType, err := types.Lower(argument.TypeNode)
		if err != nil {
			return err
		}
		parameterTypes = append(parameterTypes, parameterType)
	}
	thunk, err := backend.DeclareFunction(lb.FunctionSpec{Symbol: function.AbsName + ".__native_ctx_thunk", Result: resultType, Parameters: parameterTypes, Linkage: lb.LinkageInternal, CallingConvention: lb.CallingConventionC, Definition: true})
	if err != nil {
		return err
	}
	block, err := backend.AppendBlock(thunk, "entry")
	if err != nil {
		return err
	}
	initialized, err := backend.Call(block, functions[initializer], nil)
	if err != nil {
		return err
	}
	if initializer.ReturnType.Throws {
		errorValue, err := backend.ExtractValue(block, initialized, []uint32{0})
		if err != nil {
			return err
		}
		codeField, err := coreFieldIndex(state, t.CoreTypeError, "__code")
		if err != nil {
			return err
		}
		code, err := backend.ExtractValue(block, errorValue, []uint32{codeField})
		if err != nil {
			return err
		}
		i32, err := backend.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 32})
		if err != nil {
			return err
		}
		zero, err := integerValue(backend, i32, 0)
		if err != nil {
			return err
		}
		failed, err := backend.Compare(block, lb.CompareNotEqual, code, zero)
		if err != nil {
			return err
		}
		failure, err := backend.AppendBlock(thunk, "ctx.init.failed")
		if err != nil {
			return err
		}
		ready, err := backend.AppendBlock(thunk, "ctx.init.ready")
		if err != nil {
			return err
		}
		if err := backend.CondBranch(block, failed, failure, ready); err != nil {
			return err
		}
		if printUncaught == nil || functions[printUncaught] == 0 {
			return fmt.Errorf("uncaught-error reporter is not reachable")
		}
		if _, err := backend.Call(failure, functions[printUncaught], []lb.ValueID{errorValue}); err != nil {
			return err
		}
		voidType, err := types.Lower(namedType("void"))
		if err != nil {
			return err
		}
		abort, err := backend.DeclareFunction(lb.FunctionSpec{Symbol: "abort", Result: voidType, Linkage: lb.LinkageExternal, CallingConvention: lb.CallingConventionC, Attributes: []lb.AttributeSpec{{Kind: lb.AttributeNoReturn, Placement: lb.AttributeFunction}}})
		if err != nil {
			return err
		}
		if _, err := backend.Call(failure, abort, nil); err != nil {
			return err
		}
		if err := backend.Unreachable(failure); err != nil {
			return err
		}
		initialized, err = backend.ExtractValue(ready, initialized, []uint32{1})
		if err != nil {
			return err
		}
		block = ready
	}
	if _, err := backend.Store(block, initialized, rootAddress, 0, false); err != nil {
		return err
	}
	arguments := []lb.ValueID{rootAddress}
	for index := range parameterTypes {
		parameter, err := backend.Parameter(thunk, index)
		if err != nil {
			return err
		}
		arguments = append(arguments, parameter)
	}
	result, err := backend.Call(block, functions[function], arguments)
	if err != nil {
		return err
	}
	if isVoidResult(function.ReturnType) {
		if err := backend.ReturnVoid(block); err != nil {
			return err
		}
	} else if err := backend.Return(block, result); err != nil {
		return err
	}
	return backend.FinalizeFunction(thunk)
}

func namedType(name string) *t.NodeType {
	return &t.NodeType{KindNode: &t.NodeTypeNamed{NameNode: &t.NodeNameSingle{Name: name}}}
}

func isVoidResult(node *t.NodeType) bool {
	if node == nil || node.Throws {
		return false
	}
	named, ok := node.KindNode.(*t.NodeTypeNamed)
	if !ok {
		return false
	}
	single, ok := named.NameNode.(*t.NodeNameSingle)
	return ok && single.Name == "void"
}

func integerValue(backend lb.Backend, typeID lb.TypeID, value uint64) (lb.ValueID, error) {
	constant, err := backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: typeID, Integer: fmt.Sprint(value)})
	if err != nil {
		return 0, err
	}
	return backend.ConstantValue(constant)
}

func coreFieldIndex(state *t.SharedState, role t.CoreTypeRole, name string) (uint32, error) {
	definition := state.CoreTypes[role]
	if definition == nil {
		return 0, fmt.Errorf("missing core type %s", role.Name())
	}
	for index, field := range definition.FieldOrder {
		if field == name {
			return uint32(index), nil
		}
	}
	return 0, fmt.Errorf("core type %s has no field %s", role.Name(), name)
}

func declareArrayGlobal(backend lb.Backend, types *loweringtypes.Lowerer, state *t.SharedState, variable *t.NodeExprVarDef, array *t.NodeExprArray, symbol string, descriptorType lb.TypeID) (lb.GlobalID, error) {
	length, ok := constantUint(array.Length)
	if !ok {
		return 0, fmt.Errorf("constant array length is not an integer constant")
	}
	elementType, err := types.Lower(array.ElemType)
	if err != nil {
		return 0, err
	}
	backingType, err := backend.InternType(lb.TypeSpec{Kind: lb.TypeArray, Element: elementType, Length: length})
	if err != nil {
		return 0, err
	}
	entries := make(map[uint64]t.NodeExpr, len(array.Entries))
	for _, entry := range array.Entries {
		entries[entry.ResolvedIndex] = entry.Value
	}
	elements := make([]lb.ConstantID, length)
	for index := uint64(0); index < length; index++ {
		elements[index], err = globalConstant(backend, types, array.ElemType, entries[index])
		if err != nil {
			return 0, fmt.Errorf("array element %d: %w", index, err)
		}
	}
	backingInitializer, err := backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantAggregate, Type: backingType, Elements: elements})
	if err != nil {
		return 0, err
	}
	backing, err := backend.DeclareGlobal(lb.GlobalSpec{Symbol: symbol + ".data", Type: backingType, Initializer: backingInitializer, Linkage: lb.LinkagePrivate, Visibility: lb.VisibilityDefault, Definition: true})
	if err != nil {
		return 0, err
	}
	sliceDefinition := state.CoreTypes[t.CoreTypeSlice]
	if sliceDefinition == nil {
		return 0, fmt.Errorf("missing core slice definition")
	}
	descriptor := make([]lb.ConstantID, len(sliceDefinition.FieldOrder))
	for index, fieldName := range sliceDefinition.FieldOrder {
		fieldType := sliceDefinition.Fields[fieldName]
		fieldTypeID, err := types.Lower(fieldType)
		if err != nil {
			return 0, err
		}
		switch fieldName {
		case "__data":
			descriptor[index], err = backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantGlobalAddress, Type: fieldTypeID, Global: backing})
		case "__count":
			descriptor[index], err = backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: fieldTypeID, Integer: fmt.Sprint(length)})
		default:
			descriptor[index], err = backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantZero, Type: fieldTypeID})
		}
		if err != nil {
			return 0, err
		}
	}
	descriptorInitializer, err := backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantAggregate, Type: descriptorType, Elements: descriptor})
	if err != nil {
		return 0, err
	}
	linkage := lb.LinkagePrivate
	if types.CrossModule && variable.IsPublic {
		linkage = lb.LinkageExternal
	}
	return backend.DeclareGlobal(lb.GlobalSpec{Symbol: symbol, Type: descriptorType, Initializer: descriptorInitializer, Linkage: linkage, Visibility: lb.VisibilityDefault, ThreadLocal: !variable.IsProcessGlobal, Definition: true})
}

func constantUint(expression t.NodeExpr) (uint64, bool) {
	switch node := expression.(type) {
	case *t.NodeExprLit:
		var value uint64
		if _, err := fmt.Sscan(node.Value, &value); err == nil {
			return value, true
		}
	case *t.NodeExprName:
		if variable, ok := node.AssociatedNode.(*t.NodeExprVarDef); ok && variable.IsConst {
			return constantUint(variable.Initializer)
		}
	}
	return 0, false
}

func declareFunctionReference(backend lb.Backend, types *loweringtypes.Lowerer, function *t.NodeFuncDef) (lb.FunctionID, error) {
	if function == nil || function.ReturnType == nil || function.AbsName == "" {
		return 0, fmt.Errorf("global function constant is unresolved")
	}
	result, err := types.Lower(function.ReturnType)
	if err != nil {
		return 0, err
	}
	parameters := make([]lb.TypeID, 0, len(function.Class.ArgsNode.Args)+1)
	if function.ContextABI == t.ContextABIContextful {
		pointer, err := backend.InternType(lb.TypeSpec{Kind: lb.TypePointer})
		if err != nil {
			return 0, err
		}
		parameters = append(parameters, pointer)
	}
	argumentStart := 0
	if function.IsMember {
		pointer, err := backend.InternType(lb.TypeSpec{Kind: lb.TypePointer})
		if err != nil {
			return 0, err
		}
		parameters = append(parameters, pointer)
		argumentStart = 1
	}
	for _, argument := range function.Class.ArgsNode.Args[argumentStart:] {
		parameter, err := types.Lower(argument.TypeNode)
		if err != nil {
			return 0, err
		}
		parameters = append(parameters, parameter)
	}
	symbol := function.AbsName
	linkage := lb.LinkageInternal
	if types.IsTracePush(function) {
		linkage = lb.LinkageExternal
	}
	// Independently lowered units may carry function addresses in constant
	// globals. Those references must use the same externally linkable
	// declaration as ordinary cross-module calls and the eventual definition.
	if types.CrossModule || function.IsExternal || function.NoAliasName != "" {
		linkage = lb.LinkageExternal
		if function.NoAliasName != "" {
			symbol = function.NoAliasName
		}
	}
	return backend.DeclareFunction(lb.FunctionSpec{Symbol: symbol, Result: result, Parameters: parameters, Linkage: linkage, CallingConvention: lb.CallingConventionC})
}

func globalConstant(backend lb.Backend, types *loweringtypes.Lowerer, expected *t.NodeType, expression t.NodeExpr) (lb.ConstantID, error) {
	typeID, err := types.Lower(expected)
	if err != nil {
		return 0, err
	}
	if expression == nil {
		return backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantZero, Type: typeID})
	}
	switch node := expression.(type) {
	case *t.NodeExprLit:
		switch node.LitType {
		case t.TokLitNone:
			return backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantNull, Type: typeID})
		case t.TokLitBool:
			value := "0"
			if node.Value == "true" {
				value = "1"
			}
			return backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: typeID, Integer: value})
		case t.TokLitNum:
			if name, ok := primitiveTypeName(expected); ok && (name == "f16" || name == "f32" || name == "f64" || name == "f128") {
				return backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantFloat, Type: typeID, Float: node.Value})
			}
			integer, err := globalIntegerLiteral(expected, node.Value)
			if err != nil {
				return 0, err
			}
			return backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: typeID, Integer: integer})
		case t.TokLitStr:
			return stringGlobalConstant(backend, types, typeID, node.Value)
		default:
			return 0, fmt.Errorf("unsupported literal initializer %q", node.Value)
		}
	case *t.NodeExprName:
		if function, ok := node.AssociatedNode.(*t.NodeFuncDef); ok {
			functionID, err := declareFunctionReference(backend, types, function)
			if err != nil {
				return 0, err
			}
			return backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantFunctionAddress, Type: typeID, Function: functionID})
		}
		if variable, ok := node.AssociatedNode.(*t.NodeExprVarDef); ok && variable.IsConst {
			return globalConstant(backend, types, expected, variable.Initializer)
		}
		return 0, fmt.Errorf("global constant name is not a compile-time constant")
	case *t.NodeExprStructInit:
		fields := append([]t.NodeStructFieldInit(nil), node.Fields...)
		sort.Slice(fields, func(i, j int) bool { return fields[i].FieldIndex < fields[j].FieldIndex })
		elements := make([]lb.ConstantID, len(fields))
		for i, field := range fields {
			elements[i], err = globalConstant(backend, types, field.FieldType, field.Expression)
			if err != nil {
				return 0, fmt.Errorf("field %s: %w", field.Name, err)
			}
		}
		return backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantAggregate, Type: typeID, Elements: elements})
	default:
		return 0, fmt.Errorf("unsupported initializer expression %T", expression)
	}
}

func globalIntegerLiteral(expected *t.NodeType, value string) (string, error) {
	representation := strings.ReplaceAll(value, "_", "")
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
		return "", fmt.Errorf("invalid integer literal %q", value)
	}
	if integer.Sign() < 0 {
		name, named := primitiveTypeName(expected)
		description, numeric := magmatypes.NumberTypes[name]
		if !named || !numeric || description.IsFloat {
			return "", fmt.Errorf("negative integer literal %q has no integer type", value)
		}
		integer.Add(integer, new(big.Int).Lsh(big.NewInt(1), uint(description.ByteSize)))
	}
	return integer.String(), nil
}

func stringGlobalConstant(backend lb.Backend, types *loweringtypes.Lowerer, stringType lb.TypeID, value string) (lb.ConstantID, error) {
	i8, err := backend.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 8})
	if err != nil {
		return 0, err
	}
	array, err := backend.InternType(lb.TypeSpec{Kind: lb.TypeArray, Element: i8, Length: uint64(len(value) + 1)})
	if err != nil {
		return 0, err
	}
	bytes, err := backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantString, Type: array, Bytes: value, NullTerminated: true})
	if err != nil {
		return 0, err
	}
	digest := sha256.Sum256([]byte(value))
	global, err := backend.DeclareGlobal(lb.GlobalSpec{Symbol: fmt.Sprintf(".magma.str.%x", digest), Type: array, Initializer: bytes, Linkage: lb.LinkagePrivate, Visibility: lb.VisibilityDefault, Constant: true, UnnamedAddress: true, Definition: true})
	if err != nil {
		return 0, err
	}
	pointer, err := backend.InternType(lb.TypeSpec{Kind: lb.TypePointer})
	if err != nil {
		return 0, err
	}
	data, err := backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantGlobalAddress, Type: pointer, Global: global})
	if err != nil {
		return 0, err
	}
	definition := types.State().CoreTypes[t.CoreTypeString]
	if definition == nil {
		return 0, fmt.Errorf("string core type is missing")
	}
	elements := make([]lb.ConstantID, len(definition.FieldOrder))
	for index, field := range definition.FieldOrder {
		fieldType, err := types.Lower(definition.Fields[field])
		if err != nil {
			return 0, err
		}
		switch field {
		case "__data":
			elements[index] = data
		case "__byteCount":
			elements[index], err = backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: fieldType, Integer: fmt.Sprint(len(value))})
		default:
			elements[index], err = backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantZero, Type: fieldType})
		}
		if err != nil {
			return 0, err
		}
	}
	return backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantAggregate, Type: stringType, Elements: elements})
}

func primitiveTypeName(node *t.NodeType) (string, bool) {
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

func globalSymbol(variable *t.NodeExprVarDef) string {
	if variable == nil {
		return ""
	}
	if variable.IsExternal {
		return variable.ExternalName
	}
	return variable.AbsName
}
