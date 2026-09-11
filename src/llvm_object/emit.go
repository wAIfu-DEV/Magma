//go:build llvm_object

package llvmobject

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	llvm "tinygo.org/x/go-llvm"
)

var initializeTargets sync.Once

type OptimizationLevel uint8

const (
	OptimizationNone OptimizationLevel = iota
	OptimizationLess
	OptimizationDefault
	OptimizationAggressive
)

type TargetOptions struct {
	Triple       string
	CPU          string
	Features     string
	Optimization OptimizationLevel
	PIC          bool
}

// ResolveTargetMetadata returns LLVM's canonical triple and data layout for a
// cache key before any module is lowered.
func ResolveTargetMetadata(options TargetOptions) (string, string, error) {
	module, err := NewModule("magma-target-metadata")
	if err != nil {
		return "", "", err
	}
	defer module.Close()
	if err := module.ConfigureTarget(options); err != nil {
		return "", "", err
	}
	return module.targetTriple, module.dataLayout, nil
}

// ConfigureTarget resolves a target machine and installs its canonical triple
// and data layout before any layout-dependent lowering occurs.
func (m *Module) ConfigureTarget(options TargetOptions) error {
	if err := m.live(); err != nil {
		return err
	}
	initializeTargets.Do(func() {
		llvm.InitializeAllTargetInfos()
		llvm.InitializeAllTargets()
		llvm.InitializeAllTargetMCs()
		llvm.InitializeAllAsmParsers()
		llvm.InitializeAllAsmPrinters()
	})
	triple := options.Triple
	if triple == "" {
		triple = llvm.DefaultTargetTriple()
	}
	target, err := llvm.GetTargetFromTriple(triple)
	if err != nil {
		return backendError(ErrorContext{Operation: "resolve LLVM target", Target: triple}, err)
	}
	levels := [...]llvm.CodeGenOptLevel{llvm.CodeGenLevelNone, llvm.CodeGenLevelLess, llvm.CodeGenLevelDefault, llvm.CodeGenLevelAggressive}
	if int(options.Optimization) >= len(levels) {
		return errors.New("invalid LLVM optimization level")
	}
	reloc := llvm.RelocDefault
	if options.PIC {
		reloc = llvm.RelocPIC
	}
	machine := target.CreateTargetMachine(triple, options.CPU, options.Features, levels[options.Optimization], reloc, llvm.CodeModelDefault)
	defer machine.Dispose()
	data := machine.CreateTargetData()
	defer data.Dispose()
	return m.configure(triple, data.String())
}

// EmitObject verifies the module and compiles it directly to object-file
// bytes. Linking remains a separate driver concern, just as it is today.
func (m *Module) EmitObject(options TargetOptions) ([]byte, error) {
	if err := m.live(); err != nil {
		return nil, backendError(ErrorContext{Operation: "emit LLVM object", Target: options.Triple}, err)
	}
	verifyStart := time.Now()
	if err := m.Verify(); err != nil {
		return nil, fmt.Errorf("verify LLVM module: %w", err)
	}
	verifyDuration := time.Since(verifyStart)

	targetStart := time.Now()
	initializeTargets.Do(func() {
		llvm.InitializeAllTargetInfos()
		llvm.InitializeAllTargets()
		llvm.InitializeAllTargetMCs()
		llvm.InitializeAllAsmParsers()
		llvm.InitializeAllAsmPrinters()
	})
	triple := options.Triple
	if triple == "" {
		triple = llvm.DefaultTargetTriple()
	}
	target, err := llvm.GetTargetFromTriple(triple)
	if err != nil {
		return nil, backendError(ErrorContext{Operation: "resolve LLVM target", Target: triple}, err)
	}
	levels := [...]llvm.CodeGenOptLevel{
		llvm.CodeGenLevelNone,
		llvm.CodeGenLevelLess,
		llvm.CodeGenLevelDefault,
		llvm.CodeGenLevelAggressive,
	}
	if int(options.Optimization) >= len(levels) {
		return nil, errors.New("invalid LLVM optimization level")
	}
	reloc := llvm.RelocDefault
	if options.PIC {
		reloc = llvm.RelocPIC
	}
	machine := target.CreateTargetMachine(triple, options.CPU, options.Features, levels[options.Optimization], reloc, llvm.CodeModelDefault)
	defer machine.Dispose()

	m.raw.SetTarget(triple)
	data := machine.CreateTargetData()
	defer data.Dispose()
	m.raw.SetDataLayout(data.String())
	m.targetTriple, m.dataLayout = triple, data.String()
	targetDuration := time.Since(targetStart)

	pipelines := [...]string{"default<O0>,globaldce", "default<O1>,globaldce", "default<O2>,globaldce", "default<O3>,globaldce"}
	passOptions := llvm.NewPassBuilderOptions()
	defer passOptions.Dispose()
	optimizeStart := time.Now()
	if err := m.raw.RunPasses(pipelines[options.Optimization], machine, passOptions); err != nil {
		return nil, backendError(ErrorContext{Operation: "optimize LLVM module", Target: triple}, err)
	}
	// LLVM's default pipeline may leave now-unreferenced private constants from
	// functions removed by whole-program DCE. They have no observable identity
	// after internalization and only bloat the ELF symbol/string tables.
	for global := m.raw.FirstGlobal(); !global.IsNil(); {
		next := llvm.NextGlobal(global)
		if global.Linkage() != llvm.ExternalLinkage && global.FirstUse().IsNil() {
			global.EraseFromParentAsGlobal()
		}
		global = next
	}
	optimizeDuration := time.Since(optimizeStart)
	postVerifyStart := time.Now()
	if err := m.VerifyAt(ErrorContext{Operation: "verify optimized LLVM module", Target: triple}); err != nil {
		return nil, err
	}
	postVerifyDuration := time.Since(postVerifyStart)

	emitStart := time.Now()
	buffer, err := machine.EmitToMemoryBuffer(m.raw, llvm.ObjectFile)
	if err != nil {
		return nil, backendError(ErrorContext{Operation: "emit LLVM object", Target: triple}, err)
	}
	emitDuration := time.Since(emitStart)
	defer buffer.Dispose()
	if os.Getenv("MAGMA_LLVM_TIMINGS") != "" {
		fmt.Fprintf(os.Stderr, "LLVM_TIMING verify_ns=%d target_ns=%d optimize_ns=%d postverify_ns=%d emit_ns=%d\n", verifyDuration.Nanoseconds(), targetDuration.Nanoseconds(), optimizeDuration.Nanoseconds(), postVerifyDuration.Nanoseconds(), emitDuration.Nanoseconds())
	}
	return append([]byte(nil), buffer.Bytes()...), nil
}
