package loweringprogram

import (
	"fmt"
	"sort"
	"strings"

	loweringast "Magma/src/lowering_ast"
	lb "Magma/src/lowering_backend"
	loweringruntime "Magma/src/lowering_runtime"
	loweringtypes "Magma/src/lowering_types"
	t "Magma/src/types"
)

// BuildUnit emits one independently linkable source-owned compilation unit.
// It declares public dependency symbols, defines every local non-generic
// symbol without reachability pruning, and emits process wrappers/runtime only
// in the unit which owns the native entry point.
func BuildUnit(backend lb.Backend, state *t.SharedState, moduleID t.ModuleID) error {
	return buildUnit(backend, state, moduleID, func(function *t.NodeFuncDef) bool { return !isSpecializedFunction(function) }, true)
}

// BuildSpecializationUnit emits only the requested provider-owned concrete
// functions. Ordinary provider code and other instances remain separate cache
// entries.
func BuildSpecializationUnit(backend lb.Backend, state *t.SharedState, moduleID t.ModuleID, symbols map[string]bool) error {
	if len(symbols) == 0 {
		return fmt.Errorf("specialization unit has no requested symbols")
	}
	return buildUnit(backend, state, moduleID, func(function *t.NodeFuncDef) bool { return symbols[function.AbsName] }, false)
}

func isSpecializedFunction(function *t.NodeFuncDef) bool {
	return function != nil && strings.Contains(function.AbsName, "__g__")
}

func buildUnit(backend lb.Backend, state *t.SharedState, moduleID t.ModuleID, selectFunction func(*t.NodeFuncDef) bool, support bool) error {
	if backend == nil || state == nil || moduleID == "" {
		return fmt.Errorf("module lowering requires a backend, checked state, and stable module identity")
	}
	types, err := loweringtypes.New(backend, state)
	if err != nil {
		return err
	}
	if err := types.ValidateProtoLayouts(); err != nil {
		return err
	}
	types.CrossModule = true

	state.FilesM.Lock()
	byModule := make(map[t.ModuleID]*t.FileCtx, len(state.Files))
	for _, file := range state.Files {
		if file == nil || file.ModuleID == "" {
			continue
		}
		previous := byModule[file.ModuleID]
		// A source-backed file is authoritative over a materialized interface.
		// Stable path ordering makes malformed duplicate source registrations
		// deterministic rather than dependent on Go map iteration order.
		if previous == nil || (previous.InterfaceOnly && !file.InterfaceOnly) ||
			(previous.InterfaceOnly == file.InterfaceOnly && file.FilePath < previous.FilePath) {
			byModule[file.ModuleID] = file
		}
	}
	state.FilesM.Unlock()
	files := make([]*t.FileCtx, 0, len(byModule))
	for _, file := range byModule {
		files = append(files, file)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].ModuleID < files[j].ModuleID })
	var local *t.FileCtx
	for _, file := range files {
		if file.ModuleID == moduleID && !file.InterfaceOnly {
			local = file
			break
		}
	}
	if local == nil || local.GlNode == nil {
		return fmt.Errorf("source module %q is unavailable for lowering", moduleID)
	}

	allFunctions := make([]*t.NodeFuncDef, 0)
	localFunctions := make([]*t.NodeFuncDef, 0)
	localAdapters := make([]*t.NodeFuncDef, 0)
	localExports := make([]*t.NodeFuncDef, 0)
	globalOwners := make(map[*t.NodeExprVarDef]*t.FileCtx)
	var entryPoint, contextInitializer, printUncaught *t.NodeFuncDef
	for _, file := range files {
		if file.GlNode == nil {
			continue
		}
		for _, declaration := range file.GlNode.Declarations {
			switch node := declaration.(type) {
			case *t.NodeFuncDef:
				if file == local || node.IsPublic || node.IsExternal {
					allFunctions = append(allFunctions, node)
				}
				if file == local && !node.IsExternal && len(node.Class.TypeParams) == 0 && len(node.Class.OwnerTypeParams) == 0 && selectFunction(node) {
					localFunctions = append(localFunctions, node)
					if node.NeedsContextAdapter {
						localAdapters = append(localAdapters, node)
					}
					if node.ExportName != "" {
						localExports = append(localExports, node)
					}
					if node.IsEntryPoint && file.PackageName == file.MainPckgName {
						entryPoint = node
					}
				}
			case *t.NodeExprVarDef:
				if file == local || node.IsPublic || node.IsExternal {
					globalOwners[node] = file
				}
			case *t.NodeConstDef:
				if node.VarDef != nil && (file == local || node.VarDef.IsPublic || node.VarDef.IsExternal) {
					node.VarDef.IsConst, node.VarDef.Initializer = true, node.Initializer
					globalOwners[node.VarDef] = file
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

	globals := make(map[*t.NodeExprVarDef]lb.ValueID, len(globalOwners))
	variables := make([]*t.NodeExprVarDef, 0, len(globalOwners))
	for variable := range globalOwners {
		variables = append(variables, variable)
	}
	sort.Slice(variables, func(i, j int) bool { return globalSymbol(variables[i]) < globalSymbol(variables[j]) })
	for _, variable := range variables {
		typeID, err := types.Lower(variable.Type)
		if err != nil {
			return err
		}
		symbol := globalSymbol(variable)
		if symbol == "" {
			return fmt.Errorf("global variable has no resolved symbol")
		}
		definition := support && globalOwners[variable] == local && !variable.IsExternal
		if array, ok := variable.Initializer.(*t.NodeExprArray); ok && definition {
			global, err := declareArrayGlobal(backend, types, state, variable, array, symbol, typeID)
			if err != nil {
				return err
			}
			globals[variable], err = backend.GlobalAddress(global)
			if err != nil {
				return err
			}
			continue
		}
		spec := lb.GlobalSpec{Symbol: symbol, Type: typeID, Linkage: lb.LinkageExternal, Visibility: lb.VisibilityDefault, ThreadLocal: !variable.IsProcessGlobal && !variable.IsConst, Constant: variable.IsConst}
		if definition {
			initializer, err := globalConstant(backend, types, variable.Type, variable.Initializer)
			if err != nil {
				return err
			}
			spec.Initializer, spec.Definition = initializer, true
			if !variable.IsPublic {
				spec.Linkage = lb.LinkagePrivate
			}
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

	sort.Slice(allFunctions, func(i, j int) bool { return allFunctions[i].AbsName < allFunctions[j].AbsName })
	functionIDs := make(map[*t.NodeFuncDef]lb.FunctionID, len(allFunctions))
	for _, function := range allFunctions {
		id, err := loweringast.DeclareFunction(backend, types, function)
		if err != nil {
			return fmt.Errorf("declare %s: %w", function.AbsName, err)
		}
		functionIDs[function] = id
	}
	sort.Slice(localFunctions, func(i, j int) bool { return localFunctions[i].AbsName < localFunctions[j].AbsName })
	for _, function := range localFunctions {
		id, err := loweringast.LowerFunctionWithGlobals(backend, types, function, globals)
		if err != nil {
			return fmt.Errorf("lower %s: %w", function.AbsName, err)
		}
		functionIDs[function] = id
	}

	needsRoot := entryPoint != nil
	for _, function := range localFunctions {
		needsRoot = needsRoot || function.NeedsNativeContextThunk
	}
	for _, function := range localExports {
		needsRoot = needsRoot || function.ContextABI == t.ContextABIContextful
	}
	var rootAddress lb.ValueID
	if support && needsRoot {
		rootAddress, err = declareContextRoot(backend, types, contextInitializer)
		if err != nil {
			return err
		}
	}
	for _, function := range localAdapters {
		if err := buildContextAdapter(backend, types, function, functionIDs); err != nil {
			return err
		}
	}
	for _, function := range localFunctions {
		if function.NeedsNativeContextThunk {
			if err := buildNativeContextThunk(backend, types, state, function, contextInitializer, printUncaught, functionIDs, rootAddress); err != nil {
				return err
			}
		}
	}
	for _, function := range localExports {
		if err := buildCExportWrapper(backend, types, state, function, contextInitializer, printUncaught, functionIDs, rootAddress); err != nil {
			return err
		}
	}
	if support && entryPoint != nil {
		utils, err := loweringruntime.BuildUtils(backend)
		if err != nil {
			return err
		}
		var windowsArgs *loweringruntime.WindowsArgs
		if state.Target.OS == "windows" && len(entryPoint.Class.ArgsNode.Args) == 1 {
			helpers, err := loweringruntime.BuildWindowsArgs(backend, types)
			if err != nil {
				return err
			}
			windowsArgs = &helpers
		}
		if err := buildEntryWrapper(backend, types, state, entryPoint, contextInitializer, printUncaught, functionIDs, utils, windowsArgs, rootAddress); err != nil {
			return err
		}
	}
	return backend.Verify()
}
