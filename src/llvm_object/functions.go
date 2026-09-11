//go:build llvm_object

package llvmobject

import (
	"errors"
	"fmt"

	llvm "tinygo.org/x/go-llvm"
)

func (m *Module) NewFunction(name string, result Type, params ...Type) (Function, error) {
	return m.newFunction(name, result, params, false)
}

func (m *Module) newFunction(name string, result Type, params []Type, variadic bool) (Function, error) {
	if err := m.live(); err != nil {
		return Function{}, err
	}
	if name == "" {
		return Function{}, errors.New("LLVM function name must not be empty")
	}
	if err := m.ownsType(result, "function result"); err != nil {
		return Function{}, err
	}
	rawParams := make([]llvm.Type, len(params))
	for i := range params {
		if err := m.ownsType(params[i], fmt.Sprintf("function parameter %d", i)); err != nil {
			return Function{}, err
		}
		rawParams[i] = params[i].raw
	}
	fnType := llvm.FunctionType(result.raw, rawParams, variadic)
	return Function{value: Value{raw: llvm.AddFunction(m.raw, name, fnType), owner: m}, owner: m, name: name}, nil
}

func (f Function) Param(index int) (Value, error) {
	if f.owner == nil {
		return Value{}, errors.New("LLVM function has no owner")
	}
	if err := f.owner.live(); err != nil {
		return Value{}, err
	}
	if index < 0 || index >= f.value.raw.ParamsCount() {
		return Value{}, fmt.Errorf("LLVM parameter index %d out of range", index)
	}
	return Value{raw: f.value.raw.Param(index), owner: f.owner}, nil
}

func (m *Module) NewBlock(fn Function, name string) (Block, error) {
	if err := m.live(); err != nil {
		return Block{}, err
	}
	if fn.owner != m {
		return Block{}, errors.New("LLVM function belongs to a different module")
	}
	return Block{raw: m.ctx.AddBasicBlock(fn.value.raw, name), owner: m, name: name, function: fn.name}, nil
}

func (m *Module) BuilderAt(block Block) (*Builder, error) {
	if err := m.live(); err != nil {
		return nil, err
	}
	if block.owner != m {
		return nil, errors.New("LLVM block belongs to a different module")
	}
	m.builder.SetInsertPointAtEnd(block.raw)
	return &Builder{module: m, function: block.function, block: block.name}, nil
}

func (m *Module) BuilderAfterAllocas(block Block) (*Builder, error) {
	if err := m.live(); err != nil {
		return nil, err
	}
	if block.owner != m {
		return nil, errors.New("LLVM block belongs to a different module")
	}
	for instruction := block.raw.FirstInstruction(); !instruction.IsNil(); instruction = llvm.NextInstruction(instruction) {
		if instruction.IsAAllocaInst().IsNil() {
			m.builder.SetInsertPointBefore(instruction)
			return &Builder{module: m, function: block.function, block: block.name}, nil
		}
	}
	m.builder.SetInsertPointAtEnd(block.raw)
	return &Builder{module: m, function: block.function, block: block.name}, nil
}
