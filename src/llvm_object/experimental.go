//go:build llvm_object

package llvmobject

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	lb "Magma/src/lowering_backend"
	loweringprogram "Magma/src/lowering_program"
	loweringruntime "Magma/src/lowering_runtime"
	t "Magma/src/types"
)

// ExperimentalIR is the direct object-lowering entry point for tests and
// embedders. It remains isolated from the production textual pipeline.
func ExperimentalIR(name string, build func(lb.Backend) error) ([]byte, error) {
	backend, err := NewLoweringBackend(name)
	if err != nil {
		return nil, err
	}
	defer backend.Close()
	if build == nil {
		return nil, backendError(ErrorContext{Operation: "experimental object lowering"}, errors.New("build callback is nil"))
	}
	if err := build(backend); err != nil {
		return nil, err
	}
	if err := backend.Verify(); err != nil {
		return nil, err
	}
	ir, err := backend.module.String()
	return []byte(ir), err
}

// ProgramIR assembles a checked program as objects and prints the resulting
// LLVM module. It is the object implementation behind the eventual
// --emit=llvm cutover, not a textual-lowering fallback.
func ProgramIR(name string, state *t.SharedState) ([]byte, error) {
	backend, err := buildProgram(name, state)
	if err != nil {
		return nil, err
	}
	defer backend.Close()
	ir, err := backend.module.String()
	return []byte(ir), err
}

// ProgramObject assembles a checked program and emits native object bytes
// directly from LLVM's target machine.
func ProgramObject(name string, state *t.SharedState, options TargetOptions) ([]byte, error) {
	buildStart := time.Now()
	backend, err := buildProgram(name, state)
	if err != nil {
		return nil, err
	}
	defer backend.Close()
	if options.Triple == "" && state != nil {
		options.Triple = state.Target.Triple
	}
	if os.Getenv("MAGMA_LLVM_TIMINGS") != "" {
		fmt.Fprintf(os.Stderr, "LLVM_TIMING build_ns=%d\n", time.Since(buildStart).Nanoseconds())
	}
	return backend.module.EmitObject(options)
}

// LinkBitcodeObject is the final incremental synchronization point. Units are
// linked without per-unit optimization; EmitObject then runs the requested
// whole-program pipeline exactly once on the merged module.
func LinkBitcodeObject(name string, units []BitcodeUnit, options TargetOptions) ([]byte, error) {
	return linkBitcodeObject(name, units, options, nil)
}

// LinkBitcodeObjectPreserving closes the linked world while retaining native
// entry points and explicit C exports named by preserve.
func LinkBitcodeObjectPreserving(name string, units []BitcodeUnit, options TargetOptions, preserve []string) ([]byte, error) {
	keep := make(map[string]bool, len(preserve))
	for _, symbol := range preserve {
		keep[symbol] = true
	}
	return linkBitcodeObject(name, units, options, keep)
}

func linkBitcodeObject(name string, units []BitcodeUnit, options TargetOptions, preserve map[string]bool) ([]byte, error) {
	merged, err := MergeBitcode(name, units)
	if err != nil {
		return nil, err
	}
	defer merged.Close()
	if preserve != nil {
		if err := merged.internalizeDefinitions(preserve); err != nil {
			return nil, err
		}
	}
	return merged.EmitObject(options)
}

func buildProgram(name string, state *t.SharedState) (*LoweringBackend, error) {
	backend, err := NewLoweringBackend(name)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*LoweringBackend, error) {
		backend.Close()
		return nil, err
	}
	if state == nil {
		return fail(backendError(ErrorContext{Operation: "object program lowering"}, errors.New("checked program state is nil")))
	}
	if err := backend.ConfigureModule(lb.ModuleSpec{SourceFile: name, TargetTriple: state.Target.Triple, DataLayout: state.Target.DataLayout}); err != nil {
		return fail(err)
	}
	if state.Target.DataLayout == "" {
		if err := backend.module.ConfigureTarget(TargetOptions{Triple: state.Target.Triple}); err != nil {
			return fail(err)
		}
	}
	if err := loweringprogram.Build(backend, state); err != nil {
		return fail(err)
	}
	return backend, nil
}

// ModuleBitcode lowers exactly one source-owned module without per-unit
// optimization and returns verified cacheable bitcode.
func ModuleBitcode(state *t.SharedState, moduleID t.ModuleID) ([]byte, error) {
	return moduleBitcode(state, moduleID, nil)
}

// SpecializationBitcode emits requested concrete symbols as an independent
// provider-owned unit.
func SpecializationBitcode(state *t.SharedState, moduleID t.ModuleID, symbols []string) ([]byte, error) {
	selected := make(map[string]bool, len(symbols))
	for _, symbol := range symbols {
		selected[symbol] = true
	}
	return moduleBitcode(state, moduleID, selected)
}

func moduleBitcode(state *t.SharedState, moduleID t.ModuleID, symbols map[string]bool) ([]byte, error) {
	backend, err := NewLoweringBackend(string(moduleID))
	if err != nil {
		return nil, err
	}
	defer backend.Close()
	if state == nil {
		return nil, errors.New("checked program state is nil")
	}
	if err := backend.ConfigureModule(lb.ModuleSpec{SourceFile: string(moduleID), TargetTriple: state.Target.Triple, DataLayout: state.Target.DataLayout}); err != nil {
		return nil, err
	}
	if state.Target.DataLayout == "" {
		if err := backend.module.ConfigureTarget(TargetOptions{Triple: state.Target.Triple}); err != nil {
			return nil, err
		}
	}
	var buildErr error
	if symbols == nil {
		buildErr = loweringprogram.BuildUnit(backend, state, moduleID)
	} else {
		buildErr = loweringprogram.BuildSpecializationUnit(backend, state, moduleID, symbols)
	}
	if buildErr != nil {
		return nil, buildErr
	}
	return backend.module.Bitcode()
}

// IncrementalProgramObject independently lowers all source modules, links the
// resulting unoptimized bitcode, then performs whole-program optimization.
func IncrementalProgramObject(name string, state *t.SharedState, options TargetOptions) ([]byte, error) {
	if state == nil {
		return nil, errors.New("checked program state is nil")
	}
	state.FilesM.Lock()
	ids := make([]t.ModuleID, 0, len(state.Files))
	seen := make(map[t.ModuleID]bool)
	for _, file := range state.Files {
		if file != nil && !file.InterfaceOnly && !seen[file.ModuleID] {
			seen[file.ModuleID] = true
			ids = append(ids, file.ModuleID)
		}
	}
	state.FilesM.Unlock()
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	units := make([]BitcodeUnit, 0, len(ids))
	for _, id := range ids {
		data, err := ModuleBitcode(state, id)
		if err != nil {
			return nil, fmt.Errorf("lower module %s: %w", id, err)
		}
		units = append(units, BitcodeUnit{Name: string(id), Data: data})
	}
	type specialization struct {
		module t.ModuleID
		symbol string
	}
	specializations := []specialization{}
	state.FilesM.Lock()
	for _, file := range state.Files {
		if file == nil || file.InterfaceOnly || file.GlNode == nil {
			continue
		}
		for _, declaration := range file.GlNode.Declarations {
			function, ok := declaration.(*t.NodeFuncDef)
			if !ok || !strings.Contains(function.AbsName, "__g__") || function.IsExternal {
				continue
			}
			specializations = append(specializations, specialization{file.ModuleID, function.AbsName})
		}
	}
	state.FilesM.Unlock()
	sort.Slice(specializations, func(i, j int) bool { return specializations[i].symbol < specializations[j].symbol })
	for _, item := range specializations {
		data, err := SpecializationBitcode(state, item.module, []string{item.symbol})
		if err != nil {
			return nil, fmt.Errorf("lower specialization %s: %w", item.symbol, err)
		}
		units = append(units, BitcodeUnit{Name: string(item.module) + ":specialization:" + item.symbol, Data: data})
	}
	if options.Triple == "" {
		options.Triple = state.Target.Triple
	}
	preserve := map[string]bool{"main": true, "wmain": true}
	state.FilesM.Lock()
	for _, file := range state.Files {
		if file != nil && file.GlNode != nil {
			for _, declaration := range file.GlNode.Declarations {
				if function, ok := declaration.(*t.NodeFuncDef); ok && function.ExportName != "" {
					preserve[function.ExportName] = true
				}
			}
		}
	}
	state.FilesM.Unlock()
	return linkBitcodeObject(name, units, options, preserve)
}

// RuntimeUtilsIR constructs and prints the canonical object-defined runtime
// utility module. Textual lowering consumes this only as a generated
// compatibility artifact; it is never parsed to recover object definitions.
func RuntimeUtilsIR() ([]byte, error) {
	return ExperimentalIR("magma-runtime-utils", func(backend lb.Backend) error {
		_, err := loweringruntime.BuildUtils(backend)
		return err
	})
}

// RuntimeUtilsFragmentIR serializes the canonical runtime object definitions
// into the fragment form consumed by the temporary textual lowering. Named
// core type definitions remain owned by that lowering's module header.
func RuntimeUtilsFragmentIR() ([]byte, error) {
	module, err := RuntimeUtilsIR()
	if err != nil {
		return nil, err
	}
	var fragment bytes.Buffer
	fragment.WriteString("; Code generated from lowering_runtime.BuildUtils; DO NOT EDIT.\n\n")
	started := false
	for _, line := range strings.Split(string(module), "\n") {
		if strings.HasPrefix(line, "declare ") || strings.HasPrefix(line, "define ") || strings.HasPrefix(line, "attributes ") {
			started = true
		}
		if started && !strings.HasPrefix(line, "%type.") {
			fragment.WriteString(line)
			fragment.WriteByte('\n')
		}
	}
	return fragment.Bytes(), nil
}
