//go:build llvm_object

package llvmobject

import (
	"bytes"
	"fmt"
	"os"
	"sort"

	llvm "tinygo.org/x/go-llvm"
)

// BitcodeUnit is an immutable, cache-friendly LLVM compilation unit. Name is
// the stable module identity (not an importer alias) and Data contains LLVM
// bitcode produced by Module.Bitcode.
type BitcodeUnit struct {
	Name string
	Data []byte
}

var llvmBitcodeMagic = []byte{'B', 'C', 0xc0, 0xde}

// Bitcode serializes a verified module. The returned slice owns its storage and
// can safely outlive the Module.
func (m *Module) Bitcode() ([]byte, error) {
	if err := m.VerifyAt(ErrorContext{Operation: "verify LLVM module before bitcode serialization"}); err != nil {
		return nil, err
	}
	file, err := os.CreateTemp("", "magma-llvm-write-*.bc")
	if err != nil {
		return nil, backendError(ErrorContext{Operation: "create LLVM bitcode output"}, err)
	}
	path := file.Name()
	defer os.Remove(path)
	if err := llvm.WriteBitcodeToFile(m.raw, file); err != nil {
		file.Close()
		return nil, backendError(ErrorContext{Operation: "write LLVM bitcode"}, err)
	}
	if err := file.Close(); err != nil {
		return nil, backendError(ErrorContext{Operation: "close LLVM bitcode output"}, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, backendError(ErrorContext{Operation: "read LLVM bitcode output"}, err)
	}
	return data, nil
}

// MergeBitcode deterministically parses and links cached bitcode without
// requiring the source Module objects to remain alive.
func MergeBitcode(name string, units []BitcodeUnit) (*Module, error) {
	if len(units) == 0 {
		return nil, fmt.Errorf("cannot merge an empty bitcode set")
	}
	ordered := append([]BitcodeUnit(nil), units...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Name < ordered[j].Name })
	for i, unit := range ordered {
		if unit.Name == "" {
			return nil, fmt.Errorf("bitcode unit %d has no stable name", i)
		}
		if len(unit.Data) == 0 {
			return nil, fmt.Errorf("bitcode unit %q is empty", unit.Name)
		}
		// ParseBitcodeFile reports malformed input through LLVM's fatal diagnostic
		// path in the pinned binding, so reject obvious cache corruption first.
		if len(unit.Data) < len(llvmBitcodeMagic) || !bytes.Equal(unit.Data[:len(llvmBitcodeMagic)], llvmBitcodeMagic) {
			return nil, fmt.Errorf("bitcode unit %q has an invalid signature", unit.Name)
		}
		if i > 0 && ordered[i-1].Name == unit.Name {
			return nil, fmt.Errorf("duplicate compilation-unit name %q", unit.Name)
		}
	}
	destination, err := NewModule(name)
	if err != nil {
		return nil, err
	}
	var triple, layout string
	for index, unit := range ordered {
		unitTriple, unitLayout, err := linkBitcode(destination, unit, index == 0, triple, layout)
		if err != nil {
			destination.Close()
			return nil, err
		}
		if index == 0 {
			triple, layout = unitTriple, unitLayout
			destination.raw.SetTarget(triple)
			destination.raw.SetDataLayout(layout)
		}
	}
	destination.targetTriple = triple
	destination.dataLayout = layout
	if err := destination.VerifyAt(ErrorContext{Operation: "verify deterministically merged LLVM bitcode"}); err != nil {
		destination.Close()
		return nil, err
	}
	return destination, nil
}

func linkBitcode(destination *Module, unit BitcodeUnit, first bool, triple, layout string) (string, string, error) {
	file, err := os.CreateTemp("", "magma-llvm-read-*.bc")
	if err != nil {
		return "", "", backendError(ErrorContext{Operation: "create LLVM bitcode input for " + unit.Name}, err)
	}
	path := file.Name()
	defer os.Remove(path)
	if _, err := file.Write(unit.Data); err != nil {
		file.Close()
		return "", "", backendError(ErrorContext{Operation: "write cached LLVM bitcode for " + unit.Name}, err)
	}
	if err := file.Close(); err != nil {
		return "", "", backendError(ErrorContext{Operation: "close cached LLVM bitcode for " + unit.Name}, err)
	}
	parsed, err := destination.ctx.ParseBitcodeFile(path)
	if err != nil {
		return "", "", backendError(ErrorContext{Operation: "parse cached LLVM bitcode for " + unit.Name}, err)
	}
	unitTriple, unitLayout := parsed.Target(), parsed.DataLayout()
	if !first && (unitTriple != triple || unitLayout != layout) {
		parsed.Dispose()
		return "", "", fmt.Errorf("bitcode unit %q has incompatible target metadata", unit.Name)
	}
	if err := llvm.LinkModules(destination.raw, parsed); err != nil {
		return "", "", backendError(ErrorContext{Operation: "link cached LLVM bitcode for " + unit.Name}, err)
	}
	return unitTriple, unitLayout, nil
}

// UndefinedFunctions returns deterministic external function requirements from
// one verified bitcode unit. The specialization graph uses these edges to load
// nested concrete instances to a fixed point on warm builds.
func UndefinedFunctions(data []byte) ([]string, error) {
	if len(data) < len(llvmBitcodeMagic) || !bytes.Equal(data[:len(llvmBitcodeMagic)], llvmBitcodeMagic) {
		return nil, fmt.Errorf("invalid LLVM bitcode signature")
	}
	file, err := os.CreateTemp("", "magma-llvm-requirements-*.bc")
	if err != nil {
		return nil, err
	}
	path := file.Name()
	defer os.Remove(path)
	if _, err := file.Write(data); err != nil {
		file.Close()
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	ctx := llvm.NewContext()
	defer ctx.Dispose()
	module, err := ctx.ParseBitcodeFile(path)
	if err != nil {
		return nil, err
	}
	defer module.Dispose()
	result := []string{}
	for function := module.FirstFunction(); !function.IsNil(); function = llvm.NextFunction(function) {
		if function.IsDeclaration() {
			result = append(result, function.Name())
		}
	}
	sort.Strings(result)
	return result, nil
}

// MergeModules deterministically links independently-owned compilation units.
// Units may be built concurrently, but must be quiescent while snapshotted.
func MergeModules(name string, units []*Module) (*Module, error) {
	if len(units) == 0 {
		return nil, fmt.Errorf("cannot merge an empty module set")
	}
	ordered := append([]*Module(nil), units...)
	for _, unit := range ordered {
		if err := unit.live(); err != nil {
			return nil, err
		}
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].name < ordered[j].name })
	for i := 1; i < len(ordered); i++ {
		if ordered[i-1].name == ordered[i].name {
			return nil, fmt.Errorf("duplicate compilation-unit name %q", ordered[i].name)
		}
	}
	triple, layout := ordered[0].targetTriple, ordered[0].dataLayout
	for _, unit := range ordered[1:] {
		if unit.targetTriple != triple || unit.dataLayout != layout {
			return nil, fmt.Errorf("module %q has incompatible target metadata", unit.name)
		}
	}
	destination, err := NewModule(name)
	if err != nil {
		return nil, err
	}
	if err := destination.configure(triple, layout); err != nil {
		destination.Close()
		return nil, err
	}
	for _, unit := range ordered {
		if err := linkSnapshot(destination, unit); err != nil {
			destination.Close()
			return nil, err
		}
	}
	if err := destination.VerifyAt(ErrorContext{Operation: "verify deterministically merged LLVM module"}); err != nil {
		destination.Close()
		return nil, err
	}
	return destination, nil
}

func linkSnapshot(destination, source *Module) error {
	file, err := os.CreateTemp("", "magma-llvm-*.bc")
	if err != nil {
		return backendError(ErrorContext{Operation: "create LLVM bitcode snapshot"}, err)
	}
	path := file.Name()
	defer os.Remove(path)
	if err := llvm.WriteBitcodeToFile(source.raw, file); err != nil {
		file.Close()
		return backendError(ErrorContext{Operation: "write LLVM bitcode snapshot"}, err)
	}
	if err := file.Close(); err != nil {
		return backendError(ErrorContext{Operation: "close LLVM bitcode snapshot"}, err)
	}
	parsed, err := destination.ctx.ParseBitcodeFile(path)
	if err != nil {
		return backendError(ErrorContext{Operation: "parse LLVM bitcode snapshot"}, err)
	}
	if err := llvm.LinkModules(destination.raw, parsed); err != nil {
		return backendError(ErrorContext{Operation: "link LLVM compilation unit"}, err)
	}
	return nil
}
