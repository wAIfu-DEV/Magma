//go:build llvm_object

package llvmobject

import (
	"fmt"

	llvm "tinygo.org/x/go-llvm"
)

const maxLLVMIntegerBits = (1 << 23) - 1

func (m *Module) VoidType() (Type, error) {
	if err := m.live(); err != nil {
		return Type{}, err
	}
	return Type{raw: m.ctx.VoidType(), owner: m}, nil
}

func (m *Module) IntType(bits int) (Type, error) {
	if err := m.live(); err != nil {
		return Type{}, err
	}
	if bits < 1 || bits > maxLLVMIntegerBits {
		return Type{}, fmt.Errorf("LLVM integer width %d is outside [1, %d]", bits, maxLLVMIntegerBits)
	}
	return Type{raw: m.ctx.IntType(bits), owner: m}, nil
}

func (m *Module) PointerType(addressSpace int) (Type, error) {
	if err := m.live(); err != nil {
		return Type{}, err
	}
	if addressSpace < 0 || uint64(addressSpace) > uint64(^uint32(0)) {
		return Type{}, fmt.Errorf("LLVM address space %d is outside [0, %d]", addressSpace, uint64(^uint32(0)))
	}
	return Type{raw: llvm.PointerType(m.ctx.Int8Type(), addressSpace), owner: m}, nil
}

func (m *Module) floatType(bits uint32) (Type, error) {
	if err := m.live(); err != nil {
		return Type{}, err
	}
	var raw llvm.Type
	switch bits {
	case 16:
		// The pinned go-llvm binding does not expose LLVMHalfTypeInContext.
		// Preserve the ABI-sized storage representation until that binding is
		// available; arithmetic on f16 remains a separately tracked gap.
		raw = m.ctx.Int16Type()
	case 32:
		raw = m.ctx.FloatType()
	case 64:
		raw = m.ctx.DoubleType()
	case 128:
		raw = m.ctx.FP128Type()
	default:
		return Type{}, fmt.Errorf("unsupported LLVM floating-point width %d", bits)
	}
	return Type{raw: raw, owner: m}, nil
}

func (m *Module) arrayType(element Type, length uint64) (Type, error) {
	if err := m.live(); err != nil {
		return Type{}, err
	}
	if err := m.ownsType(element, "array element"); err != nil {
		return Type{}, err
	}
	if length > uint64(^uint(0)>>1) {
		return Type{}, fmt.Errorf("LLVM array length %d exceeds host int", length)
	}
	return Type{raw: llvm.ArrayType(element.raw, int(length)), owner: m}, nil
}

func (m *Module) vectorType(element Type, length uint64) (Type, error) {
	if err := m.live(); err != nil {
		return Type{}, err
	}
	if err := m.ownsType(element, "vector element"); err != nil {
		return Type{}, err
	}
	if length == 0 || length > uint64(^uint(0)>>1) {
		return Type{}, fmt.Errorf("LLVM vector length %d is outside the supported host range", length)
	}
	return Type{raw: llvm.VectorType(element.raw, int(length)), owner: m}, nil
}

func (m *Module) literalStructType(elements []Type, packed bool) (Type, error) {
	raw := make([]llvm.Type, len(elements))
	for i, element := range elements {
		if err := m.ownsType(element, "struct element"); err != nil {
			return Type{}, err
		}
		raw[i] = element.raw
	}
	return Type{raw: m.ctx.StructType(raw, packed), owner: m}, nil
}

func (m *Module) namedStructType(name string) (Type, error) {
	if err := m.live(); err != nil {
		return Type{}, err
	}
	if name == "" {
		return Type{}, fmt.Errorf("named LLVM struct has no name")
	}
	return Type{raw: m.ctx.StructCreateNamed(name), owner: m}, nil
}

func (m *Module) setStructBody(target Type, elements []Type, packed bool) error {
	if err := m.ownsType(target, "struct"); err != nil {
		return err
	}
	raw := make([]llvm.Type, len(elements))
	for i, element := range elements {
		if err := m.ownsType(element, "struct element"); err != nil {
			return err
		}
		raw[i] = element.raw
	}
	target.raw.StructSetBody(raw, packed)
	return nil
}

func (m *Module) functionType(result Type, parameters []Type, variadic bool) (Type, error) {
	if err := m.ownsType(result, "function result"); err != nil {
		return Type{}, err
	}
	raw := make([]llvm.Type, len(parameters))
	for i, parameter := range parameters {
		if err := m.ownsType(parameter, "function parameter"); err != nil {
			return Type{}, err
		}
		raw[i] = parameter.raw
	}
	return Type{raw: llvm.FunctionType(result.raw, raw, variadic), owner: m}, nil
}
