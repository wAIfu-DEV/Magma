//go:build llvm_object

package llvmobject

import (
	"errors"
	"fmt"

	llvm "tinygo.org/x/go-llvm"
)

// Opaque handles retain the module identity that owns their LLVM context.
type Type struct {
	raw   llvm.Type
	owner *Module
}
type Value struct {
	raw   llvm.Value
	owner *Module
}
type Function struct {
	value Value
	owner *Module
	name  string
}
type Block struct {
	raw      llvm.BasicBlock
	owner    *Module
	name     string
	function string
}

// Builder appends typed instructions to one module's current block.
type Builder struct {
	module   *Module
	function string
	block    string
}

func (b *Builder) valid() error {
	if b == nil || b.module == nil {
		return errors.New("LLVM builder has no owner")
	}
	return b.module.live()
}

func (m *Module) ownsType(value Type, role string) error {
	if value.owner == nil || value.raw.IsNil() {
		return fmt.Errorf("%s is an invalid LLVM type", role)
	}
	if value.owner != m {
		return fmt.Errorf("%s belongs to a different LLVM module", role)
	}
	return nil
}

func (m *Module) ownsValue(value Value, role string) error {
	if value.owner == nil || value.raw.IsNil() {
		return fmt.Errorf("%s is an invalid LLVM value", role)
	}
	if value.owner != m {
		return fmt.Errorf("%s belongs to a different LLVM module", role)
	}
	return nil
}
