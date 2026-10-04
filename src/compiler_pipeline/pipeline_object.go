//go:build llvm_object

package compilerpipeline

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"Magma/src/comp_err"
	incrementalcache "Magma/src/incremental_cache"
	llvmobject "Magma/src/llvm_object"
	moduleinterface "Magma/src/module_interface"
	types "Magma/src/types"
)

const ObjectCacheBackendVersion = "magma-object-bitcode-v6"

func dependencyContains(pairs []incrementalcache.Pair, name, value string) bool {
	for _, pair := range pairs {
		if pair.Name == name && pair.Value == value {
			return true
		}
	}
	return false
}

func protoLayoutFingerprint(values map[types.ModuleID]*moduleinterface.Interface) string {
	var entries []string
	for id, value := range values {
		if value == nil {
			continue
		}
		for _, implementation := range value.Structs {
			for _, proto := range implementation.Implements {
				entries = append(entries, fmt.Sprintf("%s|%s|%s|%d|%d", id, implementation.Symbol, proto.Name, implementation.StorageSize, implementation.StorageAlignment))
			}
		}
	}
	sort.Strings(entries)
	sum := sha256.Sum256([]byte(strings.Join(entries, "\n")))
	return fmt.Sprintf("%x", sum)
}

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
// lowered module and reuses a whole-program optimized artifact when possible.
func LowerCachedIncrementalObjectBytes(program SafetyCheckedProgram, options llvmobject.TargetOptions, cacheRoot, compilerVersion, safetyMode string, explain func(string)) ([]byte, error) {
	outputs, err := lowerCachedIncrementalObjects(program, options, cacheRoot, compilerVersion, safetyMode, cachedOutputWhole, explain)
	if err != nil {
		return nil, err
	}
	if len(outputs) != 1 {
		return nil, fmt.Errorf("whole-program cache produced %d objects", len(outputs))
	}
	return outputs[0], nil
}

// LowerCachedThinLTOBitcode returns independently summarized bitcode units.
// LLD performs the ThinLTO index, parallel backends, native caching and link.
func LowerCachedThinLTOBitcode(program SafetyCheckedProgram, options llvmobject.TargetOptions, cacheRoot, compilerVersion, safetyMode string, explain func(string)) ([][]byte, error) {
	return lowerCachedIncrementalObjects(program, options, cacheRoot, compilerVersion, safetyMode, cachedOutputThinLTO, explain)
}

type cachedOutputMode uint8

const (
	cachedOutputWhole cachedOutputMode = iota
	cachedOutputThinLTO
)

func lowerCachedIncrementalObjects(program SafetyCheckedProgram, options llvmobject.TargetOptions, cacheRoot, compilerVersion, safetyMode string, mode cachedOutputMode, explain func(string)) ([][]byte, error) {
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
	interfaceHashes := make(map[types.ModuleID]string, len(files))
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
		interfaces[file.ModuleID], interfaceHashes[file.ModuleID], values[file.ModuleID] = data, hash, value
	}
	dependencyHashes, err := effectiveInterfaceHashes(values, interfaceHashes)
	if err != nil {
		return nil, err
	}
	protoFingerprint := protoLayoutFingerprint(values)
	const protoLayoutKey types.ModuleID = "magma-proto-layout-v1"
	results := make([]incrementalcache.Result, 0, len(files))
	for _, file := range files {
		if file.InterfaceOnly {
			latest, err := cache.LatestResult(string(file.ModuleID))
			compatible := err == nil && latest.Hit && latest.Metadata.InterfaceHash == interfaceHashes[file.ModuleID] &&
				dependencyContains(latest.Metadata.Inputs.Dependencies, string(protoLayoutKey), protoFingerprint) &&
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
		deps[protoLayoutKey] = protoFingerprint
		for _, dependency := range values[file.ModuleID].Dependencies {
			id := types.ModuleID(dependency)
			hash, ok := dependencyHashes[id]
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
			deps := specializationDependencyHashes(file.ModuleID, function.AbsName, files, values, dependencyHashes)
			deps[protoLayoutKey] = protoFingerprint
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
	// A nested specialization may have been produced by a different root which
	// loaded more concrete type owners than this one. Keep those recorded layout
	// hashes, and reject a cached unit only when it conflicts with a layout the
	// current program or another linked unit actually uses.
	linkedLayouts := make(map[types.ModuleID]string, len(dependencyHashes))
	linkedLayouts[protoLayoutKey] = protoFingerprint
	for id, hash := range dependencyHashes {
		linkedLayouts[id] = hash
	}
	for _, result := range results {
		if !mergeSpecializationLayouts(linkedLayouts, result.Metadata.Inputs.Dependencies) {
			return nil, fmt.Errorf("cached unit %s has conflicting concrete layout dependencies", result.Metadata.Inputs.ModuleID)
		}
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
			expected := specializationDependencyHashes(provider, symbol, files, values, dependencyHashes)
			if !nestedDependenciesCompatible(result.Metadata.Inputs.Dependencies, expected, linkedLayouts) {
				return nil, fmt.Errorf("nested specialization %s has stale concrete layout dependencies", symbol)
			}
			mergeSpecializationLayouts(linkedLayouts, result.Metadata.Inputs.Dependencies)
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
	if mode == cachedOutputWhole {
		if artifact, ok := cache.LookupArtifact(graphKey); ok {
			if explain != nil {
				explain("final object: hit")
			}
			return [][]byte{artifact}, nil
		}
	}
	if mode == cachedOutputThinLTO {
		return cachedThinLTOBitcode(cache, results, explain)
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
	return [][]byte{object}, nil
}

func dependenciesCover(cached []incrementalcache.Pair, current map[types.ModuleID]string) bool {
	if len(cached) != len(current) {
		return false
	}
	for _, pair := range cached {
		if current[types.ModuleID(pair.Name)] != pair.Value {
			return false
		}
	}
	return true
}

// Nested bitcode can carry dependencies on concrete types that are absent
// from the current source graph. Their recorded hashes remain valid unless a
// current module or another linked bitcode unit supplies a conflicting layout.
func nestedDependenciesCompatible(cached []incrementalcache.Pair, required, linked map[types.ModuleID]string) bool {
	seen := make(map[types.ModuleID]bool, len(cached))
	for _, pair := range cached {
		id := types.ModuleID(pair.Name)
		if seen[id] {
			return false
		}
		seen[id] = true
		if hash, ok := linked[id]; ok && hash != pair.Value {
			return false
		}
	}
	for id, hash := range required {
		if !seen[id] {
			return false
		}
		if linkedHash, ok := linked[id]; !ok || linkedHash != hash {
			return false
		}
	}
	return true
}

func mergeSpecializationLayouts(linked map[types.ModuleID]string, dependencies []incrementalcache.Pair) bool {
	for _, pair := range dependencies {
		id := types.ModuleID(pair.Name)
		if hash, ok := linked[id]; ok && hash != pair.Value {
			return false
		}
		linked[id] = pair.Value
	}
	return true
}

// specializationDependencyHashes includes the provider's semantic closure and
// every module which owns a concrete type encoded in the mangled symbol. This
// captures external argument layouts without coupling entries to unrelated
// modules present only because of a different executable root.
func specializationDependencyHashes(provider types.ModuleID, symbol string, files []*types.FileCtx, values map[types.ModuleID]*moduleinterface.Interface, effective map[types.ModuleID]string) map[types.ModuleID]string {
	result := map[types.ModuleID]string{provider: effective[provider]}
	if value := values[provider]; value != nil {
		for _, dependency := range value.Dependencies {
			id := types.ModuleID(dependency)
			result[id] = effective[id]
		}
	}
	for _, candidate := range files {
		if candidate != nil && candidate.PackageName != "" && strings.Contains(symbol, candidate.PackageName) {
			result[candidate.ModuleID] = effective[candidate.ModuleID]
		}
	}
	return result
}

func cachedThinLTOBitcode(cache *incrementalcache.Cache, results []incrementalcache.Result, explain func(string)) ([][]byte, error) {
	units := make([][]byte, len(results))
	for index, result := range results {
		key := incrementalcache.SourceHash([]byte("magma-thinlto-bitcode-v1\n" + result.Key + "\n"))
		if artifact, ok := cache.LookupArtifact(key); ok {
			units[index] = artifact
			if explain != nil {
				explain(result.Metadata.Inputs.ModuleID + ": ThinLTO bitcode hit")
			}
			continue
		}
		unit, err := llvmobject.ThinLTOBitcode(result.Metadata.Inputs.ModuleID, result.Bitcode)
		if err != nil {
			return nil, fmt.Errorf("emit ThinLTO bitcode %s: %w", result.Metadata.Inputs.ModuleID, err)
		}
		if err := cache.PublishArtifact(key, unit); err != nil {
			return nil, err
		}
		units[index] = unit
		if explain != nil {
			explain(result.Metadata.Inputs.ModuleID + ": ThinLTO bitcode miss")
		}
	}
	if len(units) == 0 {
		return nil, fmt.Errorf("ThinLTO graph is empty")
	}
	return units, nil
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

// effectiveInterfaceHashes fingerprints the complete public-layout dependency
// closure of each module. A module interface records imported type names by
// module ID, so hashing only its own encoded bytes misses layout changes in a
// transitive provider. Reusing such a consumer bitcode unit can make two linked
// modules disagree about the body of the same named LLVM struct.
func effectiveInterfaceHashes(values map[types.ModuleID]*moduleinterface.Interface, local map[types.ModuleID]string) (map[types.ModuleID]string, error) {
	effective := make(map[types.ModuleID]string, len(local))
	visiting := make(map[types.ModuleID]bool, len(local))
	var visit func(types.ModuleID) (string, error)
	visit = func(id types.ModuleID) (string, error) {
		if hash := effective[id]; hash != "" {
			return hash, nil
		}
		base := local[id]
		value := values[id]
		if base == "" || value == nil {
			return "", fmt.Errorf("module %s has no interface for dependency fingerprinting", id)
		}
		if visiting[id] {
			return "", fmt.Errorf("module interface dependency cycle includes %s", id)
		}
		visiting[id] = true
		dependencies := append([]string(nil), value.Dependencies...)
		sort.Strings(dependencies)
		var fingerprint strings.Builder
		fingerprint.WriteString("magma-effective-interface-v1\n")
		fingerprint.WriteString(base)
		fingerprint.WriteByte('\n')
		for _, dependency := range dependencies {
			dependencyID := types.ModuleID(dependency)
			hash, err := visit(dependencyID)
			if err != nil {
				return "", err
			}
			fingerprint.WriteString(dependency)
			fingerprint.WriteByte(' ')
			fingerprint.WriteString(hash)
			fingerprint.WriteByte('\n')
		}
		visiting[id] = false
		hash := incrementalcache.SourceHash([]byte(fingerprint.String()))
		effective[id] = hash
		return hash, nil
	}
	for id := range local {
		if _, err := visit(id); err != nil {
			return nil, err
		}
	}
	return effective, nil
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
