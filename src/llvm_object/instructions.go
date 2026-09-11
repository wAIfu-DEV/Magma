//go:build llvm_object

package llvmobject

import (
	lb "Magma/src/lowering_backend"
	"fmt"

	llvm "tinygo.org/x/go-llvm"
)

func validateMemoryAlignment(alignment uint32) error {
	if alignment != 0 && alignment&(alignment-1) != 0 {
		return fmt.Errorf("memory alignment %d is not a power of two", alignment)
	}
	return nil
}

func (b *Builder) Alloca(allocated Type, alignment uint32, name string) (Value, error) {
	if err := b.valid(); err != nil {
		return Value{}, err
	}
	if err := b.module.ownsType(allocated, "alloca type"); err != nil {
		return Value{}, err
	}
	if err := validateMemoryAlignment(alignment); err != nil {
		return Value{}, err
	}
	raw := b.module.builder.CreateAlloca(allocated.raw, name)
	if alignment != 0 {
		raw.SetAlignment(int(alignment))
	}
	return Value{raw: raw, owner: b.module}, nil
}

func (b *Builder) DynamicAlloca(allocated Type, count Value, alignment uint32, name string) (Value, error) {
	if err := b.valid(); err != nil {
		return Value{}, err
	}
	if err := b.module.ownsType(allocated, "dynamic alloca type"); err != nil {
		return Value{}, err
	}
	if err := b.module.ownsValue(count, "dynamic alloca count"); err != nil {
		return Value{}, err
	}
	if err := validateMemoryAlignment(alignment); err != nil {
		return Value{}, err
	}
	raw := b.module.builder.CreateArrayAlloca(allocated.raw, count.raw, name)
	if alignment != 0 {
		raw.SetAlignment(int(alignment))
	}
	return Value{raw: raw, owner: b.module}, nil
}

func (b *Builder) Load(loaded Type, pointer Value, alignment uint32, volatile bool, name string) (Value, error) {
	if err := b.valid(); err != nil {
		return Value{}, err
	}
	if err := b.module.ownsType(loaded, "load type"); err != nil {
		return Value{}, err
	}
	if err := b.module.ownsValue(pointer, "load pointer"); err != nil {
		return Value{}, err
	}
	if err := validateMemoryAlignment(alignment); err != nil {
		return Value{}, err
	}
	raw := b.module.builder.CreateLoad(loaded.raw, pointer.raw, name)
	raw.SetVolatile(volatile)
	if alignment != 0 {
		raw.SetAlignment(int(alignment))
	}
	return Value{raw: raw, owner: b.module}, nil
}

func (b *Builder) Store(value, pointer Value, alignment uint32, volatile bool) (Value, error) {
	if err := b.valid(); err != nil {
		return Value{}, err
	}
	if err := b.module.ownsValue(value, "store value"); err != nil {
		return Value{}, err
	}
	if err := b.module.ownsValue(pointer, "store pointer"); err != nil {
		return Value{}, err
	}
	if err := validateMemoryAlignment(alignment); err != nil {
		return Value{}, err
	}
	raw := b.module.builder.CreateStore(value.raw, pointer.raw)
	raw.SetVolatile(volatile)
	if alignment != 0 {
		raw.SetAlignment(int(alignment))
	}
	return Value{raw: raw, owner: b.module}, nil
}

func objectAtomicOrdering(ordering lb.AtomicOrdering) (llvm.AtomicOrdering, error) {
	switch ordering {
	case lb.AtomicMonotonic:
		return llvm.AtomicOrderingMonotonic, nil
	case lb.AtomicAcquire:
		return llvm.AtomicOrderingAcquire, nil
	case lb.AtomicRelease:
		return llvm.AtomicOrderingRelease, nil
	case lb.AtomicAcquireRelease:
		return llvm.AtomicOrderingAcquireRelease, nil
	case lb.AtomicSequentiallyConsistent:
		return llvm.AtomicOrderingSequentiallyConsistent, nil
	default:
		return 0, fmt.Errorf("invalid atomic ordering %d", ordering)
	}
}

func (b *Builder) GEP(element Type, pointer Value, indices []Value, inbounds bool, name string) (Value, error) {
	if err := b.valid(); err != nil {
		return Value{}, err
	}
	if err := b.module.ownsType(element, "GEP source element"); err != nil {
		return Value{}, err
	}
	if err := b.module.ownsValue(pointer, "GEP pointer"); err != nil {
		return Value{}, err
	}
	if len(indices) == 0 {
		return Value{}, fmt.Errorf("GEP requires at least one index")
	}
	rawIndices := make([]llvm.Value, len(indices))
	for i, index := range indices {
		if err := b.module.ownsValue(index, fmt.Sprintf("GEP index %d", i)); err != nil {
			return Value{}, err
		}
		rawIndices[i] = index.raw
	}
	var raw llvm.Value
	if inbounds {
		raw = b.module.builder.CreateInBoundsGEP(element.raw, pointer.raw, rawIndices, name)
	} else {
		raw = b.module.builder.CreateGEP(element.raw, pointer.raw, rawIndices, name)
	}
	return Value{raw: raw, owner: b.module}, nil
}

func (b *Builder) PtrToInt(value Value, result Type, name string) (output Value, err error) {
	defer func() { err = backendError(b.errorContext("build ptrtoint", nil), err) }()
	if err := b.valid(); err != nil {
		return Value{}, err
	}
	if err := b.module.ownsValue(value, "ptrtoint operand"); err != nil {
		return Value{}, err
	}
	if err := b.module.ownsType(result, "ptrtoint result"); err != nil {
		return Value{}, err
	}
	return Value{raw: b.module.builder.CreatePtrToInt(value.raw, result.raw, name), owner: b.module}, nil
}

func (b *Builder) IntToPtr(value Value, result Type, name string) (output Value, err error) {
	defer func() { err = backendError(b.errorContext("build inttoptr", nil), err) }()
	if err := b.valid(); err != nil {
		return Value{}, err
	}
	if err := b.module.ownsValue(value, "inttoptr operand"); err != nil {
		return Value{}, err
	}
	if err := b.module.ownsType(result, "inttoptr result"); err != nil {
		return Value{}, err
	}
	return Value{raw: b.module.builder.CreateIntToPtr(value.raw, result.raw, name), owner: b.module}, nil
}

func (b *Builder) Ret(value Value) (err error) {
	defer func() { err = backendError(b.errorContext("build return", nil), err) }()
	if err := b.valid(); err != nil {
		return err
	}
	if err := b.module.ownsValue(value, "return value"); err != nil {
		return err
	}
	b.module.builder.CreateRet(value.raw)
	return nil
}

func (b *Builder) RetVoid() (err error) {
	defer func() { err = backendError(b.errorContext("build void return", nil), err) }()
	if err := b.valid(); err != nil {
		return err
	}
	b.module.builder.CreateRetVoid()
	return nil
}
