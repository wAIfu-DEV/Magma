// Package loweringcabi classifies checked Magma values for the platform C ABI.
// It contains no LLVM text and is shared by object-built calls and exports.
package loweringcabi

import (
	"fmt"

	lb "Magma/src/lowering_backend"
	loweringtypes "Magma/src/lowering_types"
	magmatypes "Magma/src/magma_types"
	t "Magma/src/types"
)

type Class uint8

const (
	Direct Class = iota
	Indirect
	Coerce
)

type Part struct {
	Type   lb.TypeID
	Offset uint64
}

type Value struct {
	Class     Class
	Logical   lb.TypeID
	Size      uint64
	Alignment uint32
	Parts     []Part
	ByValue   bool
	StructRet bool
	Aggregate bool
}

type leaf struct {
	offset  uint64
	size    uint64
	isFloat bool
}

func Classify(backend lb.Backend, types *loweringtypes.Lowerer, state *t.SharedState, node *t.NodeType, isReturn bool) (Value, error) {
	if backend == nil || types == nil || state == nil || node == nil || node.Throws {
		return Value{}, fmt.Errorf("C ABI classification requires a non-throwing checked type")
	}
	logical, err := types.Lower(node)
	if err != nil {
		return Value{}, err
	}
	if named, ok := node.KindNode.(*t.NodeTypeNamed); ok {
		if name, ok := named.NameNode.(*t.NodeNameSingle); ok && name.Name == "void" {
			return Value{Class: Direct, Logical: logical}, nil
		}
	}
	layout, err := backend.TypeLayout(logical)
	if err != nil {
		return Value{}, err
	}
	leaves, aggregate, err := valueLeaves(backend, types, state, node, 0)
	if err != nil {
		return Value{}, err
	}
	value := Value{Class: Direct, Logical: logical, Size: layout.StoreSize, Alignment: layout.ABIAlignment, Aggregate: aggregate}
	if !aggregate {
		return value, nil
	}
	arch, os := string(state.Target.Arch), string(state.Target.OS)
	if arch == "x86_64" && os == "windows" {
		switch value.Size {
		case 1, 2, 4, 8:
			value.Class = Coerce
			part, err := integerType(backend, value.Size)
			if err != nil {
				return Value{}, err
			}
			value.Parts = []Part{{Type: part}}
		default:
			value.Class, value.StructRet = Indirect, isReturn
		}
		return value, nil
	}
	if arch == "x86_64" && (os == "linux" || os == "darwin") {
		if value.Size > 16 {
			value.Class, value.ByValue, value.StructRet = Indirect, !isReturn, isReturn
			return value, nil
		}
		value.Class = Coerce
		value.Parts, err = sysVParts(backend, value.Size, leaves)
		return value, err
	}
	if (arch == "aarch64" || arch == "arm64") && (os == "linux" || os == "darwin") {
		if value.Size > 16 && !isHFA(leaves) {
			value.Class, value.StructRet = Indirect, isReturn
		}
		return value, nil
	}
	return Value{}, fmt.Errorf("aggregate C ABI for target %s-%s is not implemented", arch, os)
}

func PhysicalResult(backend lb.Backend, value Value) (lb.TypeID, error) {
	if value.Class == Indirect {
		return backend.InternType(lb.TypeSpec{Kind: lb.TypeVoid})
	}
	if value.Class != Coerce {
		return value.Logical, nil
	}
	if len(value.Parts) == 1 {
		return value.Parts[0].Type, nil
	}
	elements := make([]lb.TypeID, len(value.Parts))
	for index, part := range value.Parts {
		elements[index] = part.Type
	}
	return backend.InternStruct(lb.StructSpec{Elements: elements})
}

func sysVParts(backend lb.Backend, size uint64, leaves []leaf) ([]Part, error) {
	count := (size + 7) / 8
	parts := make([]Part, 0, count)
	for chunk := uint64(0); chunk < count; chunk++ {
		start, end := chunk*8, (chunk+1)*8
		if end > size {
			end = size
		}
		allFloat, floatBytes, hasLeaf := true, uint64(0), false
		isDouble := false
		for _, item := range leaves {
			if item.offset >= end || item.offset+item.size <= start {
				continue
			}
			hasLeaf = true
			if !item.isFloat {
				allFloat = false
				break
			}
			floatBytes += item.size
			isDouble = isDouble || item.size == 8
		}
		var typ lb.TypeID
		var err error
		if hasLeaf && allFloat && floatBytes == end-start && floatBytes == 4 {
			typ, err = backend.InternType(lb.TypeSpec{Kind: lb.TypeFloat, Bits: 32})
		} else if hasLeaf && allFloat && floatBytes == 8 {
			f32, typeErr := backend.InternType(lb.TypeSpec{Kind: lb.TypeFloat, Bits: 32})
			if typeErr != nil {
				return nil, typeErr
			}
			if isDouble {
				typ, err = backend.InternType(lb.TypeSpec{Kind: lb.TypeFloat, Bits: 64})
			} else {
				typ, err = backend.InternType(lb.TypeSpec{Kind: lb.TypeVector, Element: f32, Length: 2})
			}
		} else {
			typ, err = integerType(backend, end-start)
		}
		if err != nil {
			return nil, err
		}
		parts = append(parts, Part{Type: typ, Offset: start})
	}
	return parts, nil
}

func integerType(backend lb.Backend, bytes uint64) (lb.TypeID, error) {
	if bytes == 0 {
		bytes = 1
	}
	return backend.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: uint32(bytes * 8)})
}

func isHFA(leaves []leaf) bool {
	if len(leaves) == 0 || len(leaves) > 4 || !leaves[0].isFloat {
		return false
	}
	size := leaves[0].size
	if size != 2 && size != 4 && size != 8 && size != 16 {
		return false
	}
	for _, item := range leaves[1:] {
		if !item.isFloat || item.size != size {
			return false
		}
	}
	return true
}

func valueLeaves(backend lb.Backend, types *loweringtypes.Lowerer, state *t.SharedState, node *t.NodeType, base uint64) ([]leaf, bool, error) {
	switch kind := node.KindNode.(type) {
	case *t.NodeTypePointer, *t.NodeTypeRfc, *t.NodeTypeFunc:
		id, err := types.Lower(node)
		if err != nil {
			return nil, false, err
		}
		layout, err := backend.TypeLayout(id)
		if err != nil {
			return nil, false, err
		}
		return []leaf{{offset: base, size: layout.StoreSize, isFloat: false}}, false, nil
	case *t.NodeTypeSlice:
		definition := state.CoreTypes[t.CoreTypeSlice]
		return structLeaves(backend, types, state, node, definition, base)
	case *t.NodeTypeAbsolute:
		definition := findDefinition(state, kind)
		if definition == nil {
			return nil, false, fmt.Errorf("C ABI cannot resolve struct %s", kind.AbsoluteName)
		}
		return structLeaves(backend, types, state, node, definition, base)
	case *t.NodeTypeNamed:
		name, ok := kind.NameNode.(*t.NodeNameSingle)
		if !ok {
			return nil, false, fmt.Errorf("C ABI has unresolved named type")
		}
		if role := t.CoreTypeRoleForName(name.Name); role != t.CoreTypeNone {
			return structLeaves(backend, types, state, node, state.CoreTypes[role], base)
		}
		id, err := types.Lower(node)
		if err != nil {
			return nil, false, err
		}
		layout, err := backend.TypeLayout(id)
		if err != nil {
			return nil, false, err
		}
		description, numeric := magmatypes.NumberTypes[name.Name]
		return []leaf{{offset: base, size: layout.StoreSize, isFloat: numeric && description.IsFloat}}, false, nil
	default:
		return nil, false, fmt.Errorf("C ABI unsupported type %T", node.KindNode)
	}
}

func structLeaves(backend lb.Backend, types *loweringtypes.Lowerer, state *t.SharedState, node *t.NodeType, definition *t.StructDef, base uint64) ([]leaf, bool, error) {
	if definition == nil {
		return nil, false, fmt.Errorf("C ABI struct definition is missing")
	}
	id, err := types.Lower(node)
	if err != nil {
		return nil, false, err
	}
	for _, file := range state.Files {
		if file == nil || file.GlNode == nil {
			continue
		}
		union := file.GlNode.UnionDefs[definition.Name]
		if union == nil || union.Module != definition.Module {
			continue
		}
		layout, err := backend.TypeLayout(id)
		if err != nil {
			return nil, false, err
		}
		payloadOffset, err := backend.StructFieldOffset(id, 2)
		if err != nil {
			return nil, false, err
		}
		result := []leaf{{offset: base, size: 8}}
		for offset := payloadOffset; offset < layout.StoreSize; offset += 8 {
			size := layout.StoreSize - offset
			if size > 8 {
				size = 8
			}
			result = append(result, leaf{offset: base + offset, size: size})
		}
		return result, true, nil
	}
	result := make([]leaf, 0)
	for index, name := range definition.FieldOrder {
		offset, err := backend.StructFieldOffset(id, uint32(index))
		if err != nil {
			return nil, false, err
		}
		children, _, err := valueLeaves(backend, types, state, definition.Fields[name], base+offset)
		if err != nil {
			return nil, false, err
		}
		result = append(result, children...)
	}
	return result, true, nil
}

func findDefinition(state *t.SharedState, absolute *t.NodeTypeAbsolute) *t.StructDef {
	if absolute.CoreRole != t.CoreTypeNone {
		return state.CoreTypes[absolute.CoreRole]
	}
	for _, file := range state.Files {
		if file == nil || file.GlNode == nil {
			continue
		}
		for _, definition := range file.GlNode.StructDefs {
			if definition.Module+"."+definition.Name == absolute.AbsoluteName {
				return definition
			}
		}
	}
	return nil
}
