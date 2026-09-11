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
	backend      lb.Backend
	state        *t.SharedState
	definitions  map[string]*t.StructDef
	structIDs    map[*t.StructDef]lb.TypeID
	defined      map[*t.StructDef]bool
	defining     map[*t.StructDef]bool
	structErrors map[*t.StructDef]error
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
	result := &Lowerer{backend: backend, state: state, definitions: make(map[string]*t.StructDef), structIDs: make(map[*t.StructDef]lb.TypeID), defined: make(map[*t.StructDef]bool), defining: make(map[*t.StructDef]bool), structErrors: make(map[*t.StructDef]error)}
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
