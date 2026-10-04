package compilerpipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	incrementalcache "Magma/src/incremental_cache"
	llvmobject "Magma/src/llvm_object"
	"Magma/src/makeabs"
	moduleinterface "Magma/src/module_interface"
	"Magma/src/pipeline"
	"Magma/src/types"
)

var sourceImportPattern = regexp.MustCompile(`(?m)^\s*(?:pub\s+)?use\s+"([^"]+)"`)

// DiscoverCachedInterfaces walks import spellings without tokenizing or parsing
// dependency bodies and returns compatible warm-cache interfaces. Missing or
// stale entries are omitted so the ordinary source pipeline can rebuild them.
func DiscoverCachedInterfaces(state *types.SharedState, rootPath, cacheRoot, compilerVersion string) (map[string]*moduleinterface.Interface, error) {
	cache, err := incrementalcache.New(cacheRoot, state.Cwd, nil)
	if err != nil {
		return nil, err
	}
	result := map[string]*moduleinterface.Interface{}
	seen := map[string]bool{}
	var addCached func(types.ModuleID, string) bool
	addCached = func(id types.ModuleID, path string) bool {
		// Public interfaces do not encode private implementation changes. Check
		// the ordinary-unit source hash before replacing a present source module
		// with its cached interface; otherwise final-artifact lookup can return an
		// object containing the previous private implementation.
		if source, readErr := os.ReadFile(path); readErr == nil {
			inputs, ok := cache.LatestInputs(string(id))
			if !ok ||
				inputs.SourceHash != incrementalcache.SourceHash(ordinaryImplementationBytes(source)) ||
				inputs.CompilerVersion != compilerVersion ||
				inputs.InterfaceSchema != fmt.Sprintf("mgi-v%d", moduleinterface.SchemaVersion) ||
				inputs.BackendVersion != ObjectCacheBackendVersion ||
				inputs.LLVMVersion != llvmobject.LLVMCacheVersion ||
				inputs.TargetTriple != state.Target.Triple ||
				inputs.DataLayout != state.Target.DataLayout {
				return false
			}
		}
		data, ok, _ := cache.LatestInterface(string(id))
		if !ok {
			return false
		}
		value, err := moduleinterface.Decode(data)
		if err != nil || moduleinterface.ValidateCompiler(value, compilerVersion) != nil {
			return false
		}
		result[filepath.Clean(path)] = value
		for _, dependency := range value.Dependencies {
			dependencyID := types.ModuleID(dependency)
			dependencyPath := pathForModuleID(state, dependencyID)
			if dependencyPath == "" || result[dependencyPath] != nil {
				continue
			}
			if !addCached(dependencyID, dependencyPath) {
				delete(result, filepath.Clean(path))
				return false
			}
		}
		return true
	}
	var walk func(string) error
	walk = func(path string) error {
		path, err = filepath.Abs(path)
		if err != nil {
			return err
		}
		path = filepath.Clean(path)
		if seen[path] {
			return nil
		}
		seen[path] = true
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range sourceImportPattern.FindAllSubmatch(content, -1) {
			specifier := string(match[1])
			dependency, err := makeabs.ResolveImport(specifier, path, state.StdRoot)
			if err != nil {
				dependency = unresolvedImportPath(specifier, path, state.StdRoot)
			}
			id, err := types.ResolveModuleID(dependency, state.Cwd, state.StdRoot)
			if err != nil {
				return err
			}
			if addCached(id, dependency) {
				continue
			}
			if err := walk(dependency); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(rootPath); err != nil {
		return nil, err
	}
	delete(result, filepath.Clean(rootPath))
	for changed := true; changed; {
		changed = false
		ids := make(map[string]bool, len(result))
		for _, value := range result {
			ids[value.ModuleID] = true
		}
		for path, value := range result {
			for _, dependency := range value.Dependencies {
				if !ids[dependency] {
					delete(result, path)
					changed = true
					break
				}
			}
		}
	}
	return result, nil
}

// ordinaryImplementationBytes excludes generic templates because concrete
// specializations have independent cache entries. It is shared by warm source
// validation and ordinary module cache-key construction.
func ordinaryImplementationBytes(source []byte) []byte {
	lines := strings.Split(string(source), "\n")
	out := make([]string, 0, len(lines))
	for index := 0; index < len(lines); {
		line := lines[index]
		trimmed := strings.TrimSpace(line)
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		openParen := strings.IndexByte(trimmed, '(')
		openBracket := strings.IndexByte(trimmed, '[')
		closeBracket := strings.IndexByte(trimmed, ']')
		genericDeclaration := openBracket >= 0 && closeBracket > openBracket && openParen > closeBracket
		if indent == 0 && genericDeclaration && strings.HasSuffix(trimmed, ":") {
			index++
			for index < len(lines) {
				candidate := strings.TrimSpace(lines[index])
				candidateIndent := len(lines[index]) - len(strings.TrimLeft(lines[index], " \t"))
				index++
				if candidate == ".." && candidateIndent == 0 {
					break
				}
			}
			continue
		}
		out = append(out, line)
		index++
	}
	return []byte(strings.Join(out, "\n"))
}

func unresolvedImportPath(specifier, from, stdRoot string) string {
	var path string
	if strings.HasPrefix(specifier, "std:") {
		path = filepath.Join(stdRoot, strings.TrimPrefix(specifier, "std:"))
	} else {
		path = filepath.Join(filepath.Dir(from), specifier)
	}
	if filepath.Ext(path) == "" {
		path += ".mg"
	}
	absolute, _ := filepath.Abs(path)
	return filepath.Clean(absolute)
}

func pathForModuleID(state *types.SharedState, id types.ModuleID) string {
	value := string(id)
	const stdPrefix = "magma-module-v1:std:v1:"
	const workspacePrefix = "magma-module-v1:workspace:"
	if strings.HasPrefix(value, stdPrefix) {
		return filepath.Clean(filepath.Join(state.StdRoot, filepath.FromSlash(strings.TrimPrefix(value, stdPrefix))))
	}
	if strings.HasPrefix(value, workspacePrefix) {
		return filepath.Clean(filepath.Join(state.Cwd, filepath.FromSlash(strings.TrimPrefix(value, workspacePrefix))))
	}
	const externalPrefix = "magma-module-v1:external:"
	if strings.HasPrefix(value, externalPrefix) {
		return filepath.Clean(filepath.FromSlash(strings.TrimPrefix(value, externalPrefix)))
	}
	return ""
}

// UseCachedSpecializations lets monomorphization avoid reparsing provider
// source when the exact concrete symbol already has cached bitcode.
func UseCachedSpecializations(state *types.SharedState, cacheRoot, compilerVersion, safetyMode string) error {
	// A concrete specialization can consume layouts from modules which are not
	// dependencies of its generic provider (for example Future[world.MeshTask]).
	// Selecting the provider's latest entry here cannot validate those argument
	// layouts. Always materialize the specialization in the current program;
	// object lowering still reuses it when its complete key matches.
	state.GenericSpecializationHit = nil
	return nil
}

// ParseWithInterfaces parses the local root while satisfying selected source
// imports from declaration-only module interfaces. The map key is the source
// path used to identify that module; the source file itself need not exist.
func ParseWithInterfaces(state *types.SharedState, rootPath, compilerVersion string, interfaces map[string]*moduleinterface.Interface) (ParsedProgram, error) {
	if state == nil {
		return ParsedProgram{}, fmt.Errorf("interface-backed parsing requires shared state")
	}
	byID := make(map[string]*moduleinterface.Interface, len(interfaces))
	pathByID := make(map[string]string, len(interfaces))
	canonical := make(map[string]*moduleinterface.Interface, len(interfaces))
	for path, value := range interfaces {
		if err := moduleinterface.ValidateCompiler(value, compilerVersion); err != nil {
			return ParsedProgram{}, fmt.Errorf("interface %q: %w", path, err)
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			return ParsedProgram{}, err
		}
		absolute = filepath.Clean(absolute)
		if previous := pathByID[value.ModuleID]; previous != "" && previous != absolute {
			return ParsedProgram{}, fmt.Errorf("module interface %q is provided by both %q and %q", value.ModuleID, previous, absolute)
		}
		canonical[absolute] = value
		byID[value.ModuleID] = value
		pathByID[value.ModuleID] = absolute
	}
	for path, value := range canonical {
		file, err := moduleinterface.Materialize(value, byID, path)
		if err != nil {
			return ParsedProgram{}, fmt.Errorf("materialize interface %q: %w", path, err)
		}
		for _, dependency := range value.Dependencies {
			if dependencyPath := pathByID[dependency]; dependencyPath != "" {
				file.Imports = append(file.Imports, dependencyPath)
			}
		}
		state.InterfaceFiles[path] = file
		state.Files[path] = file
		state.ModuleNamesM.Lock()
		if existing, found := state.ModuleNames[file.PackageName]; found && existing != file.ModuleID {
			state.ModuleNamesM.Unlock()
			return ParsedProgram{}, fmt.Errorf("stable module-name collision %q", file.PackageName)
		}
		state.ModuleNames[file.PackageName] = file.ModuleID
		state.ModuleNamesM.Unlock()
	}
	state.PipelineFunc = func(shared *types.SharedState, filePath, alias, fromAbs string, fromGl *types.NodeGlobal) <-chan error {
		path := filepath.Clean(filePath)
		if file := shared.InterfaceFiles[path]; file != nil {
			if fromGl != nil {
				fromGl.ImportAlias[alias] = file.PackageName
			}
			result := make(chan error, 1)
			result <- nil
			close(result)
			return result
		}
		return pipeline.DoAsync(shared, filePath, alias, fromAbs, fromGl)
	}
	state.GenericProviderLoader = func(packageName string) (*types.FileCtx, error) {
		var path string
		var previous *types.FileCtx
		for candidate, file := range state.InterfaceFiles {
			if file != nil && file.PackageName == packageName {
				path, previous = candidate, file
				break
			}
		}
		if path == "" {
			return nil, fmt.Errorf("generic provider %q has no interface backing path", packageName)
		}
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("generic provider source %q is required on specialization cache miss: %w", path, err)
		}
		state.FilesM.Lock()
		for candidate, file := range state.Files {
			if candidate == path || (file != nil && file.ModuleID == previous.ModuleID) {
				delete(state.Files, candidate)
			}
		}
		state.FilesM.Unlock()
		for candidate, file := range state.InterfaceFiles {
			if candidate == path || (file != nil && file.ModuleID == previous.ModuleID) {
				delete(state.InterfaceFiles, candidate)
			}
		}
		state.ImportedFilesM.Lock()
		delete(state.ImportedFiles, path)
		state.ImportedFilesM.Unlock()
		owner := &types.NodeGlobal{ImportAlias: map[string]string{}}
		if err := pipeline.Do(state, path, "__generic_provider", path, owner); err != nil {
			return nil, err
		}
		state.WaitGroup.Wait()
		file := state.Files[path]
		if file == nil || file.InterfaceOnly || file.GlNode == nil {
			return nil, fmt.Errorf("generic provider %q did not materialize source", packageName)
		}
		file.MainPckgName = previous.MainPckgName
		return file, nil
	}
	return Parse(state, rootPath)
}
