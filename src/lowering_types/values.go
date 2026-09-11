package loweringtypes

import (
	"fmt"

	lb "Magma/src/lowering_backend"
	t "Magma/src/types"
)

func (l *Lowerer) coreField(role t.CoreTypeRole, name string) (lb.TypeID, uint32, lb.TypeID, error) {
	definition := l.state.CoreTypes[role]
	if definition == nil {
		return 0, 0, 0, fmt.Errorf("missing core type definition for %q", role.Name())
	}
	index, ok := definition.FieldNb[name]
	if !ok || index < 0 || index >= len(definition.FieldOrder) || definition.FieldOrder[index] != name {
		return 0, 0, 0, fmt.Errorf("core type %q has inconsistent or missing field %q", role.Name(), name)
	}
	field := definition.Fields[name]
	if field == nil {
		return 0, 0, 0, fmt.Errorf("core type %q field %q has no type", role.Name(), name)
	}
	structure, err := l.lowerCore(role)
	if err != nil {
		return 0, 0, 0, err
	}
	fieldType, err := l.Lower(field)
	if err != nil {
		return 0, 0, 0, err
	}
	return structure, uint32(index), fieldType, nil
}

func (l *Lowerer) BuildCoreValue(block lb.BlockID, role t.CoreTypeRole, fields map[string]lb.ValueID) (lb.ValueID, error) {
	definition := l.state.CoreTypes[role]
	if definition == nil {
		return 0, fmt.Errorf("missing core type definition for %q", role.Name())
	}
	if len(fields) != len(definition.FieldOrder) {
		return 0, fmt.Errorf("core type %q requires %d fields, got %d", role.Name(), len(definition.FieldOrder), len(fields))
	}
	values := make([]lb.ValueID, len(definition.FieldOrder))
	for index, name := range definition.FieldOrder {
		value, ok := fields[name]
		if !ok {
			return 0, fmt.Errorf("core type %q is missing field %q", role.Name(), name)
		}
		recorded, ok := definition.FieldNb[name]
		if !ok || recorded != index {
			return 0, fmt.Errorf("core type %q field index for %q is inconsistent", role.Name(), name)
		}
		values[index] = value
	}
	typeID, err := l.lowerCore(role)
	if err != nil {
		return 0, err
	}
	return l.backend.BuildAggregate(block, typeID, values)
}

// BuildCoreValueWithDefaults constructs compiler-known values whose omitted
// fields are the textual zeroinitializer defaults.
func (l *Lowerer) BuildCoreValueWithDefaults(block lb.BlockID, role t.CoreTypeRole, fields map[string]lb.ValueID) (lb.ValueID, error) {
	definition := l.state.CoreTypes[role]
	if definition == nil {
		return 0, fmt.Errorf("missing core type definition for %q", role.Name())
	}
	values := make(map[string]lb.ValueID, len(definition.FieldOrder))
	for _, name := range definition.FieldOrder {
		if value, ok := fields[name]; ok {
			values[name] = value
			continue
		}
		_, _, lowered, err := l.coreField(role, name)
		if err != nil {
			return 0, err
		}
		constant, err := l.backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantZero, Type: lowered})
		if err != nil {
			return 0, err
		}
		values[name], err = l.backend.ConstantValue(constant)
		if err != nil {
			return 0, err
		}
	}
	for name := range fields {
		if _, ok := definition.Fields[name]; !ok {
			return 0, fmt.Errorf("core type %q has no field %q", role.Name(), name)
		}
	}
	return l.BuildCoreValue(block, role, values)
}

func (l *Lowerer) ExtractCoreField(block lb.BlockID, value lb.ValueID, role t.CoreTypeRole, field string) (lb.ValueID, error) {
	_, index, _, err := l.coreField(role, field)
	if err != nil {
		return 0, err
	}
	return l.backend.ExtractValue(block, value, []uint32{index})
}

// CoreFieldType exposes the already-validated physical type of a compiler-known
// field without leaking semantic definitions through the backend boundary.
func (l *Lowerer) CoreFieldType(role t.CoreTypeRole, field string) (lb.TypeID, error) {
	_, _, fieldType, err := l.coreField(role, field)
	return fieldType, err
}

func (l *Lowerer) CoreFieldAddress(block lb.BlockID, pointer lb.ValueID, role t.CoreTypeRole, field string) (lb.ValueID, error) {
	structure, index, _, err := l.coreField(role, field)
	if err != nil {
		return 0, err
	}
	return l.backend.StructFieldAddress(block, structure, pointer, index)
}

func (l *Lowerer) BuildSlice(block lb.BlockID, data, count lb.ValueID) (lb.ValueID, error) {
	return l.BuildCoreValue(block, t.CoreTypeSlice, map[string]lb.ValueID{"__data": data, "__count": count})
}

// BuildThrowingSuccess constructs the physical return value used by textual
// lowering: a zero Error followed by the ordinary result when it is non-void.
// No control-flow or runtime check is introduced here.
func (l *Lowerer) BuildThrowingSuccess(block lb.BlockID, resultType *t.NodeType, value lb.ValueID) (lb.ValueID, error) {
	if resultType == nil || !resultType.Throws {
		return 0, fmt.Errorf("throwing success requires a throwing result type")
	}
	errorType, err := l.lowerCore(t.CoreTypeError)
	if err != nil {
		return 0, err
	}
	errorZero, err := l.backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantZero, Type: errorType})
	if err != nil {
		return 0, err
	}
	errorValue, err := l.backend.ConstantValue(errorZero)
	if err != nil {
		return 0, err
	}
	physicalType, err := l.Lower(resultType)
	if err != nil {
		return 0, err
	}
	fields := []lb.ValueID{errorValue}
	ordinary := *resultType
	ordinary.Throws = false
	ordinaryType, err := l.Lower(&ordinary)
	if err != nil {
		return 0, err
	}
	voidType, err := l.backend.InternType(lb.TypeSpec{Kind: lb.TypeVoid})
	if err != nil {
		return 0, err
	}
	if ordinaryType != voidType {
		if value == 0 {
			return 0, fmt.Errorf("non-void throwing success has no result value")
		}
		fields = append(fields, value)
	} else if value != 0 {
		return 0, fmt.Errorf("throwing void success has an unexpected result value")
	}
	return l.backend.BuildAggregate(block, physicalType, fields)
}

// BuildThrowingFailure mirrors textual irMakeThrowingRetVal: preserve the
// supplied Error and zero-initialize the ordinary result field, if present.
func (l *Lowerer) BuildThrowingFailure(block lb.BlockID, resultType *t.NodeType, errorValue lb.ValueID) (lb.ValueID, error) {
	if resultType == nil || !resultType.Throws || errorValue == 0 {
		return 0, fmt.Errorf("throwing failure requires a throwing result type and error value")
	}
	physicalType, err := l.Lower(resultType)
	if err != nil {
		return 0, err
	}
	fields := []lb.ValueID{errorValue}
	ordinary := *resultType
	ordinary.Throws = false
	ordinaryType, err := l.Lower(&ordinary)
	if err != nil {
		return 0, err
	}
	voidType, err := l.backend.InternType(lb.TypeSpec{Kind: lb.TypeVoid})
	if err != nil {
		return 0, err
	}
	if ordinaryType != voidType {
		zero, err := l.backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantZero, Type: ordinaryType})
		if err != nil {
			return 0, err
		}
		zeroValue, err := l.backend.ConstantValue(zero)
		if err != nil {
			return 0, err
		}
		fields = append(fields, zeroValue)
	}
	return l.backend.BuildAggregate(block, physicalType, fields)
}

// ProvenSliceElementAddress mirrors textual lowering: the dominating runtime
// guard is emitted by the bounded statement, and each authorized access emits
// only the address calculation. It deliberately does not use inbounds because
// the textual backend currently emits a plain GEP here.
func (l *Lowerer) ProvenSliceElementAddress(block lb.BlockID, slice, index lb.ValueID, element lb.TypeID, proof *t.RangeProof) (lb.ValueID, error) {
	if proof == nil {
		return 0, fmt.Errorf("slice element address requires a validated range proof")
	}
	data, err := l.ExtractCoreField(block, slice, t.CoreTypeSlice, "__data")
	if err != nil {
		return 0, err
	}
	return l.backend.GEP(block, element, data, []lb.ValueID{index}, false)
}
