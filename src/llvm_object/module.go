//go:build llvm_object

// Package llvmobject provides the typed LLVM object backend.
//
// It is isolated behind the llvm_object build tag. The production compiler
// uses package llvm_ir when the textual strategy is selected.
package llvmobject

import (
	"errors"
	"strings"
	"sync"

	llvm "tinygo.org/x/go-llvm"
)

// LLVMCacheVersion is part of every persistent bitcode key. Cached bitcode is
// never assumed compatible across LLVM library versions.
const LLVMCacheVersion = llvm.Version

// Module owns an LLVM context, module, and builder. A module must not be used
// after Close. It is intentionally not safe for concurrent mutation; parallel
// lowering should build independent modules and link them at a deterministic
// merge point.
type Module struct {
	name         string
	targetTriple string
	dataLayout   string
	ctx          llvm.Context
	raw          llvm.Module
	builder      llvm.Builder
	mu           sync.Mutex
	closed       bool
}

// internalizeDefinitions closes the independently linked Magma program before
// whole-program optimization. Native declarations remain external and named C
// exports supplied by the caller are preserved.
func (m *Module) internalizeDefinitions(preserve map[string]bool) error {
	if err := m.live(); err != nil {
		return err
	}
	for function := m.raw.FirstFunction(); !function.IsNil(); function = llvm.NextFunction(function) {
		name := function.Name()
		if function.IsDeclaration() || preserve[name] || strings.HasPrefix(name, "llvm.") {
			continue
		}
		function.SetLinkage(llvm.InternalLinkage)
	}
	for global := m.raw.FirstGlobal(); !global.IsNil(); global = llvm.NextGlobal(global) {
		if global.IsDeclaration() || preserve[global.Name()] {
			continue
		}
		if global.Linkage() == llvm.ExternalLinkage {
			global.SetLinkage(llvm.InternalLinkage)
		}
	}
	return m.VerifyAt(ErrorContext{Operation: "verify internalized linked LLVM module"})
}

func NewModule(name string) (*Module, error) {
	if name == "" {
		return nil, errors.New("LLVM module name must not be empty")
	}
	ctx := llvm.NewContext()
	return &Module{name: name, ctx: ctx, raw: ctx.NewModule(name), builder: ctx.NewBuilder()}, nil
}

func (m *Module) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return
	}
	m.builder.Dispose()
	m.raw.Dispose()
	m.ctx.Dispose()
	m.closed = true
}

func (m *Module) live() error {
	if m == nil || m.closed {
		return errors.New("LLVM module is closed")
	}
	return nil
}

func (m *Module) Verify() error {
	return m.VerifyAt(ErrorContext{Operation: "verify LLVM module"})
}

// VerifyAt verifies a module while retaining the lowering context that made
// verification necessary.
func (m *Module) VerifyAt(context ErrorContext) error {
	if err := m.live(); err != nil {
		return backendError(context, err)
	}
	return verificationError(context, llvm.VerifyModule(m.raw, llvm.ReturnStatusAction))
}

func (m *Module) String() (string, error) {
	if err := m.live(); err != nil {
		return "", err
	}
	return m.raw.String(), nil
}

func (m *Module) configure(targetTriple, dataLayout string) error {
	if err := m.live(); err != nil {
		return err
	}
	m.raw.SetTarget(targetTriple)
	m.raw.SetDataLayout(dataLayout)
	m.targetTriple = targetTriple
	m.dataLayout = dataLayout
	return nil
}
