//go:build llvm_object

package compilerpipeline

import (
	"fmt"
	"sort"
	"strings"

	"Magma/src/comp_err"
	incrementalcache "Magma/src/incremental_cache"
	llvmobject "Magma/src/llvm_object"
	moduleinterface "Magma/src/module_interface"
	types "Magma/src/types"
)

const ObjectCacheBackendVersion = "magma-object-bitcode-v3"

// LowerObjectIR lowers a safety-checked program through the object model and
// prints the constructed module. It is retained for whole-program LLVM text
// output and backend parity checks; native compilation uses cached bitcode.
func LowerObjectIR(program SafetyCheckedProgram) ([]byte, error) {
	if program.state == nil {
		return nil, comp_err.AtStage("LLVM object lowering", fmt.Errorf("checked program state is nil"))
	}
	ir, err := llvmobject.ProgramIR("magma-program", program.state)
	return ir, comp_err.AtStage("LLVM object lowering", err)
}

// LowerObjectBytes lowers and optimizes a safety-checked program, then emits a
// native object directly through LLVM's target machine.
func LowerObjectBytes(program SafetyCheckedProgram, options llvmobject.TargetOptions) ([]byte, error) {
	if program.state == nil {
		return nil, comp_err.AtStage("LLVM object lowering", fmt.Errorf("checked program state is nil"))
	}
	object, err := llvmobject.ProgramObject("magma-program", program.state, options)
	return object, comp_err.AtStage("LLVM object lowering", err)
}

// LowerIncrementalObjectBytes exercises independent module lowering and the
// deterministic final bitcode link. Persistent lookup/publish is layered on
// ModuleBitcode rather than changing the ordinary object path.
func LowerIncrementalObjectBytes(program SafetyCheckedProgram, options llvmobject.TargetOptions) ([]byte, error) {
	if program.state == nil {
		return nil, comp_err.AtStage("incremental LLVM object lowering", fmt.Errorf("checked program state is nil"))
	}
	object, err := llvmobject.IncrementalProgramObject("magma-incremental-program", program.state, options)
	return object, comp_err.AtStage("incremental LLVM object lowering", err)
}

// LowerCachedIncrementalObjectBytes loads or publishes each independently
// lowered module and always relinks/reoptimizes the complete graph.
func LowerCachedIncrementalObjectBytes(program SafetyCheckedProgram, options llvmobject.TargetOptions, cacheRoot, compilerVersion, safetyMode string, explain func(string)) ([]byte, error) {
	state := program.state
	if state == nil {
		return nil, comp_err.AtStage("incremental cache", fmt.Errorf("checked program state is nil"))
	}
	cache, err := incrementalcache.New(cacheRoot, state.Cwd, explain)
	if err != nil {
		return nil, comp_err.AtStage("incremental cache", err)
	}
	if state.Target.Triple == "" || state.Target.DataLayout == "" {
		triple, layout, err := llvmobject.ResolveTargetMetadata(options)
		if err != nil {
			return nil, err
		}
		state.Target.Triple, state.Target.DataLayout = triple, layout
		if options.Triple == "" {
			options.Triple = triple
		}
	}
	if options.Triple == "" {
		options.Triple = state.Target.Triple
	}
	state.FilesM.Lock()
	byModule := make(map[types.ModuleID]*types.FileCtx, len(state.Files))
	for _, file := range state.Files {
		if file == nil || file.ModuleID == "" {
			continue
		}
		previous := byModule[file.ModuleID]
		if previous == nil || (previous.InterfaceOnly && !file.InterfaceOnly) ||
			(previous.InterfaceOnly == file.InterfaceOnly && file.FilePath < previous.FilePath) {
			byModule[file.ModuleID] = file
		}
	}
	state.FilesM.Unlock()
	files := make([]*types.FileCtx, 0, len(byModule))
	for _, file := range byModule {
		files = append(files, file)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].ModuleID < files[j].ModuleID })
	interfaces := make(map[types.ModuleID][]byte, len(files))
	hashes := make(map[types.ModuleID]string, len(files))
	values := make(map[types.ModuleID]*moduleinterface.Interface, len(files))
	for _, file := range files {
		var value *moduleinterface.Interface
		var err error
		if len(file.InterfaceSnapshot) != 0 {
			value, err = moduleinterface.Decode(file.InterfaceSnapshot)
			if err == nil {
				value.Compiler = compilerVersion
			}
		} else {
			value, err = moduleinterface.Generate(state, file, compilerVersion)
		}
		if err != nil {
			return nil, err
		}
		data, err := moduleinterface.Encode(value)
		if err != nil {
			return nil, err
		}
		hash, err := moduleinterface.Hash(value)
		if err != nil {
			return nil, err
		}
		interfaces[file.ModuleID], hashes[file.ModuleID], values[file.ModuleID] = data, hash, value
	}
	results := make([]incrementalcache.Result, 0, len(files))
	for _, file := range files {
		if file.InterfaceOnly {
			latest, err := cache.LatestResult(string(file.ModuleID))
			compatible := err == nil && latest.Hit && latest.Metadata.InterfaceHash == hashes[file.ModuleID] &&
				latest.Metadata.Inputs.CompilerVersion == compilerVersion &&
				latest.Metadata.Inputs.InterfaceSchema == fmt.Sprintf("mgi-v%d", moduleinterface.SchemaVersion) &&
				latest.Metadata.Inputs.BackendVersion == ObjectCacheBackendVersion &&
				latest.Metadata.Inputs.LLVMVersion == llvmobject.LLVMCacheVersion &&
				latest.Metadata.Inputs.TargetTriple == state.Target.Triple &&
				latest.Metadata.Inputs.DataLayout == state.Target.DataLayout &&
				latest.Metadata.Inputs.SafetyMode == safetyMode
			if compatible {
				if explain != nil {
					explain(string(file.ModuleID) + ": hit")
				}
				results = append(results, latest)
				continue
			}
			if state.GenericProviderLoader == nil {
				return nil, fmt.Errorf("source is required to rebuild cache miss for %s", file.ModuleID)
			}
			loaded, loadErr := state.GenericProviderLoader(file.PackageName)
			if loadErr != nil {
				return nil, loadErr
			}
			file = loaded
		}
		deps := make(map[types.ModuleID]string)
		for _, dependency := range values[file.ModuleID].Dependencies {
			id := types.ModuleID(dependency)
			hash, ok := hashes[id]
			if !ok {
				return nil, fmt.Errorf("module %s depends on unavailable interface %s", file.ModuleID, id)
			}
			deps[id] = hash
		}
		ordinaryFile := *file
		ordinaryFile.Content = ordinaryImplementationBytes(file.Content)
		input, err := ObjectCacheInputs(&ordinaryFile, state, compilerVersion, llvmobject.LLVMCacheVersion, safetyMode, deps, options)
		if err != nil {
			return nil, err
		}
		result, err := cache.Lookup(input)
		if err != nil {
			return nil, err
		}
		if !result.Hit {
			if explain != nil {
				explain(string(file.ModuleID) + ": miss: " + result.Reason)
			}
			bitcode, err := llvmobject.ModuleBitcode(state, file.ModuleID)
			if err != nil {
				return nil, err
			}
			if _, err := cache.Publish(input, interfaces[file.ModuleID], bitcode); err != nil {
				return nil, err
			}
			result, err = cache.Lookup(input)
			if err != nil || !result.Hit {
				return nil, fmt.Errorf("published cache entry for %s could not be loaded: %v (%s)", file.ModuleID, err, result.Reason)
			}
		} else if explain != nil {
			explain(string(file.ModuleID) + ": hit")
		}
		results = append(results, result)
	}
	// Concrete generic instances are cached independently from ordinary provider
	// bitcode, so requesting a new type does not invalidate the provider unit.
	for _, file := range files {
		functions := make([]*types.NodeFuncDef, 0)
		seenSymbols := map[string]bool{}
		for _, declaration := range file.GlNode.Declarations {
			function, ok := declaration.(*types.NodeFuncDef)
			if !ok || !strings.Contains(function.AbsName, "__g__") || seenSymbols[function.AbsName] {
				continue
			}
			seenSymbols[function.AbsName] = true
			functions = append(functions, function)
		}
		sort.Slice(functions, func(i, j int) bool { return functions[i].AbsName < functions[j].AbsName })
		for _, function := range functions {
			if function.CachedSpecialization {
				id := string(file.ModuleID) + ":specialization:" + function.AbsName
				result, err := cache.LatestResult(id)
				if err != nil || !result.Hit {
					return nil, fmt.Errorf("cached specialization %s became unavailable", function.AbsName)
				}
				if explain != nil {
					explain(id + ": hit")
				}
				results = append(results, result)
				continue
			}
			template := genericTemplateBytes(file.Content, function.AbsName)
			pseudo := *file
			pseudo.ModuleID = types.ModuleID(string(file.ModuleID) + ":specialization:" + function.AbsName)
			pseudo.Content = template
			deps := map[types.ModuleID]string{file.ModuleID: hashes[file.ModuleID]}
			for _, dependency := range values[file.ModuleID].Dependencies {
				id := types.ModuleID(dependency)
				deps[id] = hashes[id]
			}
			input, err := ObjectCacheInputs(&pseudo, state, compilerVersion, llvmobject.LLVMCacheVersion, safetyMode, deps, options)
			if err != nil {
				return nil, err
			}
			result, err := cache.Lookup(input)
			if err != nil {
				return nil, err
			}
			if !result.Hit {
				if explain != nil {
					explain(string(pseudo.ModuleID) + ": miss: " + result.Reason)
				}
				bitcode, err := llvmobject.SpecializationBitcode(state, file.ModuleID, []string{function.AbsName})
				if err != nil {
					return nil, err
				}
				manifest := []byte("specialization-v1\n" + function.AbsName + "\n" + incrementalcache.SourceHash(template) + "\n")
				if _, err := cache.Publish(input, manifest, bitcode); err != nil {
					return nil, err
				}
				result, err = cache.Lookup(input)
				if err != nil || !result.Hit {
					return nil, fmt.Errorf("published specialization %s could not be loaded: %v", function.AbsName, err)
				}
			} else if explain != nil {
				explain(string(pseudo.ModuleID) + ": hit")
			}
			results = append(results, result)
		}
	}
	// Cached bodies may call further concrete instances which are absent from
	// interface signatures. Follow bitcode declaration edges to a fixed point.
	packageModules := map[string]types.ModuleID{}
	for _, file := range files {
		packageModules[file.PackageName] = file.ModuleID
	}
	loadedUnits := map[string]bool{}
	for _, result := range results {
		loadedUnits[result.Metadata.Inputs.ModuleID] = true
	}
	for index := 0; index < len(results); index++ {
		required, err := llvmobject.UndefinedFunctions(results[index].Bitcode)
		if err != nil {
			return nil, err
		}
		for _, symbol := range required {
			if !strings.Contains(symbol, "__g__") {
				continue
			}
			packageName, _, ok := strings.Cut(symbol, ".")
			if !ok {
				continue
			}
			provider := packageModules[packageName]
			if provider == "" {
				continue
			}
			id := string(provider) + ":specialization:" + symbol
			if loadedUnits[id] {
				continue
			}
			result, err := cache.LatestResult(id)
			if err != nil || !result.Hit {
				return nil, fmt.Errorf("nested specialization %s is unavailable", symbol)
			}
			if result.Metadata.Inputs.CompilerVersion != compilerVersion ||
				result.Metadata.Inputs.InterfaceSchema != fmt.Sprintf("mgi-v%d", moduleinterface.SchemaVersion) ||
				result.Metadata.Inputs.BackendVersion != ObjectCacheBackendVersion ||
				result.Metadata.Inputs.LLVMVersion != llvmobject.LLVMCacheVersion ||
				result.Metadata.Inputs.TargetTriple != state.Target.Triple ||
				result.Metadata.Inputs.DataLayout != state.Target.DataLayout ||
				result.Metadata.Inputs.SafetyMode != safetyMode {
				return nil, fmt.Errorf("nested specialization %s is incompatible with this build", symbol)
			}
			loadedUnits[id] = true
			results = append(results, result)
			if len(results) > 10000 {
				return nil, fmt.Errorf("cached specialization graph did not reach a fixed point after 10000 units")
			}
		}
	}
	preserve := []string{"main", "wmain"}
	for _, file := range files {
		for _, declaration := range file.GlNode.Declarations {
			if function, ok := declaration.(*types.NodeFuncDef); ok && function.ExportName != "" {
				preserve = append(preserve, function.ExportName)
			}
		}
	}
	var graph strings.Builder
	graph.WriteString("magma-final-object-v2\n")
	for _, result := range results {
		graph.WriteString(result.Key)
		graph.WriteByte('\n')
	}
	graph.WriteString(fmt.Sprintf("%s\n%s\n%s\n%d\n%t\n", options.Triple, options.CPU, options.Features, options.Optimization, options.PIC))
	graphKey := incrementalcache.SourceHash([]byte(graph.String()))
	if artifact, ok := cache.LookupArtifact(graphKey); ok {
		if explain != nil {
			explain("final object: hit")
		}
		return artifact, nil
	}
	object, err := linkCachedObjectPreserving("magma-cached-program", results, options, preserve)
	if err != nil {
		return nil, err
	}
	if err := cache.PublishArtifact(graphKey, object); err != nil {
		return nil, err
	}
	if explain != nil {
		explain("final object: miss")
	}
	return object, nil
}

func genericTemplateBytes(source []byte, specializedSymbol string) []byte {
	name := specializedSymbol
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		name = name[dot+1:]
	}
	if generic := strings.Index(name, "__g__"); generic >= 0 {
		name = name[:generic]
	}
	lines := strings.Split(string(source), "\n")
	start, indent := -1, 0
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, name+"[") && strings.HasSuffix(trimmed, ":") {
			start = index
			indent = len(line) - len(strings.TrimLeft(line, " \t"))
			break
		}
	}
	if start < 0 {
		return append([]byte(nil), source...)
	}
	end := len(lines)
	for index := start + 1; index < len(lines); index++ {
		trimmed := strings.TrimSpace(lines[index])
		if trimmed == "" {
			continue
		}
		current := len(lines[index]) - len(strings.TrimLeft(lines[index], " \t"))
		if current <= indent && trimmed != ".." {
			end = index
			break
		}
	}
	return []byte(strings.Join(lines[start:end], "\n"))
}

// LinkObjectBitcode links independently cached compilation units and performs
// optimization/object emission only after the whole program has been formed.
// It is kept separate from LowerObjectBytes so the textual and ordinary object
// paths retain their existing whole-program behavior.
func LinkObjectBitcode(name string, units []llvmobject.BitcodeUnit, options llvmobject.TargetOptions) ([]byte, error) {
	object, err := llvmobject.LinkBitcodeObject(name, units, options)
	return object, comp_err.AtStage("incremental LLVM bitcode link", err)
}

// ObjectCacheInputs constructs the canonical module-bitcode key. dependency
// hashes must be public .mgi hashes; implementation hashes are intentionally
// not accepted by this API.
func ObjectCacheInputs(file *types.FileCtx, state *types.SharedState, compilerVersion, llvmVersion, safetyMode string, dependencyInterfaces map[types.ModuleID]string, options llvmobject.TargetOptions) (incrementalcache.Inputs, error) {
	if file == nil || state == nil {
		return incrementalcache.Inputs{}, fmt.Errorf("object cache key requires a source module and shared state")
	}
	dependencies := make([]incrementalcache.Pair, 0, len(dependencyInterfaces))
	for module, hash := range dependencyInterfaces {
		dependencies = append(dependencies, incrementalcache.Pair{Name: string(module), Value: hash})
	}
	args := make([]incrementalcache.Pair, 0, len(state.CompilerArgs))
	for name, value := range state.CompilerArgs {
		args = append(args, incrementalcache.Pair{Name: name, Value: value})
	}
	sort.Slice(dependencies, func(i, j int) bool { return dependencies[i].Name < dependencies[j].Name })
	sort.Slice(args, func(i, j int) bool { return args[i].Name < args[j].Name })
	triple := options.Triple
	if triple == "" {
		triple = state.Target.Triple
	}
	return incrementalcache.Inputs{
		ModuleID: string(file.ModuleID), SourceHash: incrementalcache.SourceHash(file.Content), CompilerVersion: compilerVersion,
		InterfaceSchema: fmt.Sprintf("mgi-v%d", moduleinterface.SchemaVersion), BackendVersion: ObjectCacheBackendVersion, LLVMVersion: llvmVersion,
		TargetTriple: triple, DataLayout: state.Target.DataLayout, SafetyMode: safetyMode,
		Dependencies: dependencies, CompilerArgs: args,
		// Per-module bitcode is unoptimized and relocation-independent. CPU,
		// optimization, and PIC choices apply only after the final link.
		Codegen: []incrementalcache.Pair{{Name: "unit-mode", Value: "unoptimized"}},
	}, nil
}

// LinkCachedObject performs no final-artifact caching: every invocation links
// the complete selected graph and runs the requested whole-program optimizer.
func LinkCachedObject(name string, results []incrementalcache.Result, options llvmobject.TargetOptions) ([]byte, error) {
	units := make([]llvmobject.BitcodeUnit, 0, len(results))
	for _, result := range results {
		if !result.Hit {
			return nil, comp_err.AtStage("incremental LLVM bitcode link", fmt.Errorf("module cache miss %s: %s", result.Key, result.Reason))
		}
		units = append(units, llvmobject.BitcodeUnit{Name: result.Metadata.Inputs.ModuleID, Data: result.Bitcode})
	}
	return LinkObjectBitcode(name, units, options)
}

func linkCachedObjectPreserving(name string, results []incrementalcache.Result, options llvmobject.TargetOptions, preserve []string) ([]byte, error) {
	units := make([]llvmobject.BitcodeUnit, 0, len(results))
	for _, result := range results {
		if !result.Hit {
			return nil, fmt.Errorf("module cache miss %s: %s", result.Key, result.Reason)
		}
		units = append(units, llvmobject.BitcodeUnit{Name: result.Metadata.Inputs.ModuleID, Data: result.Bitcode})
	}
	return llvmobject.LinkBitcodeObjectPreserving(name, units, options, preserve)
}
