// Package loweringtypes converts checked Magma semantic types into the opaque
// backend-neutral type identities consumed by either lowering implementation.
package loweringtypes

import (
	"fmt"

	lb "Magma/src/lowering_backend"
	magmatypes "Magma/src/magma_types"
	t "Magma/src/types"
)

type Lowerer struct {
	backend              lb.Backend
	state                *t.SharedState
	definitions          map[string]*t.StructDef
	structIDs            map[*t.StructDef]lb.TypeID
	defined              map[*t.StructDef]bool
	defining             map[*t.StructDef]bool
	structErrors         map[*t.StructDef]error
	ProtoBorrowFunctions map[string]lb.FunctionID
	ProtoVtableGlobals   map[string]lb.GlobalID
	// CrossModule makes ordinary Magma definitions externally linkable while
	// independently lowering bitcode units. Whole-program textual/object
	// lowering leaves this false and retains internal linkage.
	CrossModule bool
}

func (l *Lowerer) State() *t.SharedState {
	if l == nil {
		return nil
	}
	return l.state
}

func New(backend lb.Backend, state *t.SharedState) (*Lowerer, error) {
	if backend == nil || state == nil {
		return nil, fmt.Errorf("type lowerer requires a backend and shared state")
	}
	result := &Lowerer{backend: backend, state: state, definitions: make(map[string]*t.StructDef), structIDs: make(map[*t.StructDef]lb.TypeID), defined: make(map[*t.StructDef]bool), defining: make(map[*t.StructDef]bool), structErrors: make(map[*t.StructDef]error), ProtoBorrowFunctions: make(map[string]lb.FunctionID), ProtoVtableGlobals: make(map[string]lb.GlobalID)}
	state.FilesM.Lock()
	defer state.FilesM.Unlock()
	for _, file := range state.Files {
		if file == nil || file.GlNode == nil {
			continue
		}
		for _, definition := range file.GlNode.StructDefs {
			if err := result.register(definition); err != nil {
				return nil, err
			}
		}
	}
	for _, definition := range state.CoreTypes {
		if definition != nil {
			if err := result.register(definition); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

// ValidateProtoLayouts forces every proto's inline storage to a finite target layout.
func (l *Lowerer) ValidateProtoLayouts() error {
	for _, definition := range l.definitions {
		if definition == nil || !definition.IsProto {
			continue
		}
		if err := l.validateProtoStorage(definition, map[*t.StructDef]bool{}); err != nil {
			return err
		}
		id, err := l.lowerStruct(definition)
		if err != nil {
			return err
		}
		if _, err := l.backend.TypeLayout(id); err != nil {
			return fmt.Errorf("recursive proto storage for %s has no finite inline size; store a pointer to the proto in the implementation instead: %w", definition.Module+"."+definition.Name, err)
		}
	}
	return nil
}

func (l *Lowerer) validateProtoStorage(definition *t.StructDef, path map[*t.StructDef]bool) error {
	if path[definition] {
		return fmt.Errorf("recursive proto storage through %s.%s has no finite inline size; store a pointer to the proto in the implementation instead", definition.Module, definition.Name)
	}
	path[definition] = true
	defer delete(path, definition)
	if definition.IsProto && definition.Proto != nil {
		for _, candidate := range l.definitions {
			for _, relation := range candidate.Implements {
				if relation != nil && relation.Proto != nil && relation.Proto.Module == definition.Proto.Module && relation.Proto.Name == definition.Proto.Name {
					if err := l.validateProtoStorage(candidate, path); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	for _, fieldName := range definition.FieldOrder {
		field := definition.Fields[fieldName]
		if field == nil {
			continue
		}
		dependent := l.storageDependency(definition, field.KindNode)
		if dependent == nil {
			continue
		}
		if err := l.validateProtoStorage(dependent, path); err != nil {
			return fmt.Errorf("field %s.%s: %w", definition.Name, fieldName, err)
		}
	}
	return nil
}

func (l *Lowerer) storageDependency(owner *t.StructDef, kind t.NodeTypeKind) *t.StructDef {
	switch node := kind.(type) {
	case *t.NodeTypeAbsolute:
		if node.CoreRole != t.CoreTypeNone {
			return l.state.CoreTypes[node.CoreRole]
		}
		return l.definitions[node.AbsoluteName]
	case *t.NodeTypeNamed:
		switch name := node.NameNode.(type) {
		case *t.NodeNameSingle:
			if role := t.CoreTypeRoleForName(name.Name); role != t.CoreTypeNone {
				return l.state.CoreTypes[role]
			}
			return l.definitions[owner.Module+"."+name.Name]
		case *t.NodeNameComposite:
			if len(name.Parts) != 2 {
				return nil
			}
			for _, file := range l.state.Files {
				if file == nil || file.PackageName != owner.Module || file.GlNode == nil {
					continue
				}
				packageName := file.GlNode.ImportAlias[name.Parts[0]]
				return l.definitions[packageName+"."+name.Parts[1]]
			}
		}
	}
	return nil
}

func (l *Lowerer) register(definition *t.StructDef) error {
	name := definition.Module + "." + definition.Name
	if existing := l.definitions[name]; existing != nil && existing != definition {
		return fmt.Errorf("duplicate semantic struct identity %q", name)
	}
	l.definitions[name] = definition
	return nil
}

func (l *Lowerer) Lower(node *t.NodeType) (lb.TypeID, error) {
	if node == nil || node.KindNode == nil {
		return 0, fmt.Errorf("cannot lower a missing Magma type")
	}
	base, err := l.lowerKind(node.KindNode)
	if err != nil {
		return 0, err
	}
	if !node.Throws {
		return base, nil
	}
	errorType, err := l.lowerCore(t.CoreTypeError)
	if err != nil {
		return 0, err
	}
	baseSpec, _ := l.backendType(base)
	if baseSpec.Kind == lb.TypeVoid {
		return l.backend.InternStruct(lb.StructSpec{Elements: []lb.TypeID{errorType}})
	}
	return l.backend.InternStruct(lb.StructSpec{Elements: []lb.TypeID{errorType, base}})
}

// Signature lowers a stored function type into its physical call signature.
// Contextful functions receive the implicit context pointer first.
func (l *Lowerer) Signature(function *t.NodeTypeFunc) (lb.TypeID, error) {
	if function == nil || function.RetType == nil {
		return 0, fmt.Errorf("cannot lower an incomplete function type")
	}
	parameters := make([]lb.TypeID, 0, len(function.Args)+1)
	if function.ContextABI == t.ContextABIContextful {
		pointer, err := l.backend.InternType(lb.TypeSpec{Kind: lb.TypePointer})
		if err != nil {
			return 0, err
		}
		parameters = append(parameters, pointer)
	}
	for i, argument := range function.Args {
		lowered, err := l.Lower(argument)
		if err != nil {
			return 0, fmt.Errorf("function argument %d: %w", i, err)
		}
		parameters = append(parameters, lowered)
	}
	result, err := l.Lower(function.RetType)
	if err != nil {
		return 0, fmt.Errorf("function result: %w", err)
	}
	return l.backend.InternFunctionType(lb.FunctionTypeSpec{Result: result, Parameters: parameters})
}

func (l *Lowerer) lowerKind(kind t.NodeTypeKind) (lb.TypeID, error) {
	switch value := kind.(type) {
	case *t.NodeTypePointer, *t.NodeTypeRfc, *t.NodeTypeFunc:
		return l.backend.InternType(lb.TypeSpec{Kind: lb.TypePointer})
	case *t.NodeTypeSlice:
		return l.lowerCore(t.CoreTypeSlice)
	case *t.NodeTypeAbsolute:
		if value.CoreRole != t.CoreTypeNone {
			return l.lowerCore(value.CoreRole)
		}
		definition := l.definitions[value.AbsoluteName]
		if definition == nil {
			return 0, fmt.Errorf("unknown absolute Magma type %q", value.AbsoluteName)
		}
		return l.lowerStruct(definition)
	case *t.NodeTypeNamed:
		name, ok := value.NameNode.(*t.NodeNameSingle)
		if !ok {
			return 0, fmt.Errorf("unresolved composite named type")
		}
		if role := t.CoreTypeRoleForName(name.Name); role != t.CoreTypeNone {
			return l.lowerCore(role)
		}
		return l.lowerPrimitive(name.Name)
	case *t.NodeTypeCompilerKnown:
		return 0, fmt.Errorf("compiler-known type %q was not resolved before lowering", value.Name)
	default:
		return 0, fmt.Errorf("unsupported Magma type kind %T", kind)
	}
}

func (l *Lowerer) lowerPrimitive(name string) (lb.TypeID, error) {
	switch name {
	case "void":
		return l.backend.InternType(lb.TypeSpec{Kind: lb.TypeVoid})
	case "bool":
		return l.backend.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 1})
	case "ptr":
		return l.backend.InternType(lb.TypeSpec{Kind: lb.TypePointer})
	}
	number, ok := magmatypes.NumberTypes[name]
	if !ok {
		return 0, fmt.Errorf("unresolved named Magma type %q", name)
	}
	kind := lb.TypeInteger
	if number.IsFloat {
		kind = lb.TypeFloat
	}
	return l.backend.InternType(lb.TypeSpec{Kind: kind, Bits: uint32(number.ByteSize)})
}

func (l *Lowerer) lowerCore(role t.CoreTypeRole) (lb.TypeID, error) {
	definition := l.state.CoreTypes[role]
	if definition == nil {
		return 0, fmt.Errorf("missing core type definition for %q", role.Name())
	}
	return l.lowerStruct(definition)
}

func (l *Lowerer) lowerStruct(definition *t.StructDef) (lb.TypeID, error) {
	if id := l.structIDs[definition]; id != 0 {
		if err := l.structErrors[definition]; err != nil {
			return 0, err
		}
		if l.defined[definition] || l.defining[definition] {
			return id, nil
		}
		return 0, fmt.Errorf("struct %q is in an invalid lowering state", definition.Module+"."+definition.Name)
	}
	name := "struct." + definition.Module + "." + definition.Name
	if definition.CoreRole != t.CoreTypeNone {
		name = "type." + definition.CoreRole.Name()
	}
	id, err := l.backend.InternStruct(lb.StructSpec{Name: name, Opaque: true})
	if err != nil {
		return 0, err
	}
	l.structIDs[definition] = id
	l.defining[definition] = true
	fail := func(err error) (lb.TypeID, error) {
		l.defining[definition] = false
		l.structErrors[definition] = err
		return 0, err
	}
	if definition.IsProto && definition.Proto != nil {
		pointer, err := l.backend.InternType(lb.TypeSpec{Kind: lb.TypePointer})
		if err != nil {
			return fail(err)
		}
		pointerLayout, err := l.backend.TypeLayout(pointer)
		if err != nil {
			return fail(err)
		}
		maxSize, maxAlign, alignType := pointerLayout.AllocationSize, pointerLayout.ABIAlignment, pointer
		for _, candidate := range l.definitions {
			for _, implementation := range candidate.Implements {
				if implementation == nil || implementation.Proto == nil || implementation.Proto.Module != definition.Proto.Module || implementation.Proto.Name != definition.Proto.Name {
					continue
				}
				concrete, err := l.lowerStruct(candidate)
				if err != nil {
					return fail(err)
				}
				layout, err := l.backend.TypeLayout(concrete)
				if err != nil {
					return fail(fmt.Errorf("recursive proto storage for %s has no finite inline size; store a pointer to the proto in the implementation instead: %w", definition.Module+"."+definition.Name, err))
				}
				if layout.AllocationSize > maxSize {
					maxSize = layout.AllocationSize
				}
				if layout.ABIAlignment > maxAlign {
					maxAlign, alignType = layout.ABIAlignment, concrete
				}
			}
		}
		byteType, err := l.backend.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 8})
		if err != nil {
			return fail(err)
		}
		aligner, err := l.backend.InternType(lb.TypeSpec{Kind: lb.TypeArray, Element: alignType, Length: 0})
		if err != nil {
			return fail(err)
		}
		storage, err := l.backend.InternType(lb.TypeSpec{Kind: lb.TypeArray, Element: byteType, Length: maxSize})
		if err != nil {
			return fail(err)
		}
		if err := l.backend.DefineStruct(id, []lb.TypeID{pointer, aligner, storage}, false); err != nil {
			return fail(err)
		}
		l.defining[definition], l.defined[definition] = false, true
		return id, nil
	}
	// A union's semantic fields name all variants for checking and matching, but
	// its physical representation overlays their payloads.
	for _, file := range l.state.Files {
		if file == nil || file.GlNode == nil || file.GlNode.UnionDefs[definition.Name] == nil || file.GlNode.UnionDefs[definition.Name].Module != definition.Module {
			continue
		}
		union := file.GlNode.UnionDefs[definition.Name]
		tag, err := l.backend.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 64})
		if err != nil {
			return fail(err)
		}
		byteType, err := l.backend.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 8})
		if err != nil {
			return fail(err)
		}
		var maxSize uint64
		var maxAlign uint32 = 1
		var alignType lb.TypeID
		for _, variant := range union.Variants {
			variantDef := l.definitions[definition.Module+".__union_"+definition.Name+"_"+variant.Name]
			if variantDef == nil {
				return fail(fmt.Errorf("missing union variant %s.%s", definition.Name, variant.Name))
			}
			variantType, err := l.lowerStruct(variantDef)
			if err != nil {
				return fail(err)
			}
			layout, err := l.backend.TypeLayout(variantType)
			if err != nil {
				return fail(err)
			}
			if layout.AllocationSize > maxSize {
				maxSize = layout.AllocationSize
			}
			if layout.ABIAlignment > maxAlign {
				maxAlign, alignType = layout.ABIAlignment, variantType
			}
		}
		if alignType == 0 {
			alignType = byteType
		}
		aligner, err := l.backend.InternType(lb.TypeSpec{Kind: lb.TypeArray, Element: alignType, Length: 0})
		if err != nil {
			return fail(err)
		}
		bytes, err := l.backend.InternType(lb.TypeSpec{Kind: lb.TypeArray, Element: byteType, Length: maxSize})
		if err != nil {
			return fail(err)
		}
		if err := l.backend.DefineStruct(id, []lb.TypeID{tag, aligner, bytes}, false); err != nil {
			return fail(err)
		}
		l.defining[definition] = false
		l.defined[definition] = true
		return id, nil
	}
	elements := make([]lb.TypeID, len(definition.FieldOrder))
	for i, field := range definition.FieldOrder {
		fieldType := definition.Fields[field]
		if fieldType == nil {
			return fail(fmt.Errorf("struct %q field %q has no type", name, field))
		}
		elements[i], err = l.Lower(fieldType)
		if err != nil {
			return fail(fmt.Errorf("struct %q field %q: %w", name, field, err))
		}
	}
	if err := l.backend.DefineStruct(id, elements, false); err != nil {
		return fail(err)
	}
	l.defining[definition] = false
	l.defined[definition] = true
	return id, nil
}

// backendType uses layout-independent knowledge available from semantic
// primitives. It is intentionally narrow; throwing void is the only caller.
func (l *Lowerer) backendType(id lb.TypeID) (lb.TypeSpec, error) {
	void, err := l.backend.InternType(lb.TypeSpec{Kind: lb.TypeVoid})
	if err != nil {
		return lb.TypeSpec{}, err
	}
	if id == void {
		return lb.TypeSpec{Kind: lb.TypeVoid}, nil
	}
	return lb.TypeSpec{}, nil
}
