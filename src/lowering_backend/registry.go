package loweringbackend

import (
	"fmt"
	"math/big"
	"sort"
)

// Registry owns backend-neutral identities and structural invariants for one
// compilation context. It is intentionally independent of any LLVM binding.
type Registry struct {
	types         map[TypeSpec]TypeID
	typeSpecs     []TypeSpec
	functions     []FunctionSpec
	functionIDs   map[string]FunctionID
	defined       map[string]bool
	constants     map[string]ConstantID
	constantSpecs []ConstantSpec
	globals       []GlobalSpec
	globalIDs     map[string]GlobalID
	globalDefined map[string]bool
	structs       map[TypeID]StructSpec
	namedStructs  map[string]TypeID
	compoundTypes map[string]TypeID
	functionTypes map[TypeID]FunctionTypeSpec
}

func NewRegistry() *Registry {
	return &Registry{
		types:         make(map[TypeSpec]TypeID),
		functionIDs:   make(map[string]FunctionID),
		defined:       make(map[string]bool),
		constants:     make(map[string]ConstantID),
		globalIDs:     make(map[string]GlobalID),
		globalDefined: make(map[string]bool),
		structs:       make(map[TypeID]StructSpec), namedStructs: make(map[string]TypeID),
		compoundTypes: make(map[string]TypeID), functionTypes: make(map[TypeID]FunctionTypeSpec),
	}
}

func (r *Registry) InternStruct(spec StructSpec) (TypeID, error) {
	if spec.Name == "" && spec.Opaque {
		return 0, fmt.Errorf("literal struct cannot be opaque")
	}
	if spec.Opaque && len(spec.Elements) != 0 {
		return 0, fmt.Errorf("opaque struct %q has a body", spec.Name)
	}
	if err := r.validateTypes(spec.Elements, "struct element"); err != nil {
		return 0, err
	}
	if spec.Name != "" {
		if id := r.namedStructs[spec.Name]; id != 0 {
			existing := r.structs[id]
			if !existing.Opaque || spec.Opaque {
				if structKey(existing) != structKey(spec) {
					return 0, fmt.Errorf("incompatible definition of struct %q", spec.Name)
				}
			}
			return id, nil
		}
	}
	key := structKey(spec)
	if spec.Name == "" {
		if id := r.compoundTypes[key]; id != 0 {
			return id, nil
		}
	}
	id := r.addCompoundType(TypeStruct)
	copySpec := spec
	copySpec.Elements = append([]TypeID(nil), spec.Elements...)
	r.structs[id] = copySpec
	if spec.Name != "" {
		r.namedStructs[spec.Name] = id
	} else {
		r.compoundTypes[key] = id
	}
	return id, nil
}

func (r *Registry) DefineStruct(id TypeID, elements []TypeID, packed bool) error {
	spec, ok := r.structs[id]
	if !ok || spec.Name == "" {
		return fmt.Errorf("type %d is not a named struct", id)
	}
	if !spec.Opaque {
		return fmt.Errorf("struct %q already has a body", spec.Name)
	}
	if err := r.validateTypes(elements, "struct element"); err != nil {
		return err
	}
	spec.Elements = append([]TypeID(nil), elements...)
	spec.Packed = packed
	spec.Opaque = false
	r.structs[id] = spec
	return nil
}

func (r *Registry) Struct(id TypeID) (StructSpec, error) {
	spec, ok := r.structs[id]
	if !ok {
		return StructSpec{}, fmt.Errorf("type %d is not a struct", id)
	}
	spec.Elements = append([]TypeID(nil), spec.Elements...)
	return spec, nil
}

func (r *Registry) AggregateArity(id TypeID) (uint64, error) {
	spec, err := r.Type(id)
	if err != nil {
		return 0, err
	}
	switch spec.Kind {
	case TypeArray:
		return spec.Length, nil
	case TypeStruct:
		structure, err := r.Struct(id)
		if err != nil {
			return 0, err
		}
		if structure.Opaque {
			return 0, fmt.Errorf("opaque struct %q has no elements", structure.Name)
		}
		return uint64(len(structure.Elements)), nil
	default:
		return 0, fmt.Errorf("type %d is not an aggregate", id)
	}
}

func (r *Registry) AggregateElement(id TypeID, path []uint32) (TypeID, error) {
	if len(path) == 0 {
		return 0, fmt.Errorf("aggregate path is empty")
	}
	current := id
	for depth, index := range path {
		spec, err := r.Type(current)
		if err != nil {
			return 0, err
		}
		switch spec.Kind {
		case TypeArray:
			if uint64(index) >= spec.Length {
				return 0, fmt.Errorf("aggregate index %d at depth %d is out of range", index, depth)
			}
			current = spec.Element
		case TypeStruct:
			structure, err := r.Struct(current)
			if err != nil {
				return 0, err
			}
			if structure.Opaque {
				return 0, fmt.Errorf("opaque struct %q has no elements", structure.Name)
			}
			if int(index) >= len(structure.Elements) {
				return 0, fmt.Errorf("aggregate index %d at depth %d is out of range", index, depth)
			}
			current = structure.Elements[index]
		default:
			return 0, fmt.Errorf("aggregate path descends into non-aggregate type %d at depth %d", current, depth)
		}
	}
	return current, nil
}

func (r *Registry) InternFunctionType(spec FunctionTypeSpec) (TypeID, error) {
	if !r.ValidType(spec.Result) {
		return 0, fmt.Errorf("function type has unknown result type %d", spec.Result)
	}
	if err := r.validateTypes(spec.Parameters, "function parameter"); err != nil {
		return 0, err
	}
	key := functionTypeKey(spec)
	if id := r.compoundTypes[key]; id != 0 {
		return id, nil
	}
	id := r.addCompoundType(TypeFunction)
	copySpec := spec
	copySpec.Parameters = append([]TypeID(nil), spec.Parameters...)
	r.functionTypes[id] = copySpec
	r.compoundTypes[key] = id
	return id, nil
}

func (r *Registry) FunctionType(id TypeID) (FunctionTypeSpec, error) {
	spec, ok := r.functionTypes[id]
	if !ok {
		return FunctionTypeSpec{}, fmt.Errorf("type %d is not a function type", id)
	}
	spec.Parameters = append([]TypeID(nil), spec.Parameters...)
	return spec, nil
}

func (r *Registry) addCompoundType(kind TypeKind) TypeID {
	id := TypeID(len(r.typeSpecs) + 1)
	r.typeSpecs = append(r.typeSpecs, TypeSpec{Kind: kind})
	return id
}

func (r *Registry) validateTypes(ids []TypeID, role string) error {
	for i, id := range ids {
		if !r.ValidType(id) {
			return fmt.Errorf("%s %d has unknown type %d", role, i, id)
		}
	}
	return nil
}

func structKey(s StructSpec) string {
	return fmt.Sprintf("s:%s:%t:%t:%v", s.Name, s.Packed, s.Opaque, s.Elements)
}
func functionTypeKey(s FunctionTypeSpec) string {
	return fmt.Sprintf("f:%d:%t:%v", s.Result, s.Variadic, s.Parameters)
}

func (r *Registry) InternConstant(spec ConstantSpec) (ConstantID, error) {
	typeSpec, err := r.Type(spec.Type)
	if err != nil {
		return 0, fmt.Errorf("constant has %w", err)
	}
	switch spec.Kind {
	case ConstantInteger:
		if spec.Float != "" || len(spec.Elements) != 0 || spec.Global != 0 || spec.Function != 0 || spec.Bytes != "" || spec.NullTerminated {
			return 0, fmt.Errorf("integer constant has an unrelated payload")
		}
		if typeSpec.Kind != TypeInteger {
			return 0, fmt.Errorf("integer constant requires an integer type")
		}
		value, ok := new(big.Int).SetString(spec.Integer, 10)
		if !ok || value.Sign() < 0 {
			return 0, fmt.Errorf("invalid unsigned integer constant %q", spec.Integer)
		}
		if value.BitLen() > int(typeSpec.Bits) {
			return 0, fmt.Errorf("integer constant %q does not fit i%d", spec.Integer, typeSpec.Bits)
		}
		spec.Integer = value.String()
	case ConstantNull:
		if typeSpec.Kind != TypePointer {
			return 0, fmt.Errorf("null constant requires a pointer type")
		}
		if spec.Integer != "" || spec.Float != "" || len(spec.Elements) != 0 || spec.Global != 0 || spec.Function != 0 || spec.Bytes != "" || spec.NullTerminated {
			return 0, fmt.Errorf("null constant has a payload")
		}
	case ConstantZero:
		if typeSpec.Kind == TypeVoid {
			return 0, fmt.Errorf("void has no zero constant")
		}
		if spec.Integer != "" || spec.Float != "" || len(spec.Elements) != 0 || spec.Global != 0 || spec.Function != 0 || spec.Bytes != "" || spec.NullTerminated {
			return 0, fmt.Errorf("zero constant has a payload")
		}
	case ConstantFloat:
		if spec.Integer != "" || len(spec.Elements) != 0 || spec.Global != 0 || spec.Function != 0 || spec.Bytes != "" || spec.NullTerminated {
			return 0, fmt.Errorf("floating constant has an unrelated payload")
		}
		if typeSpec.Kind != TypeFloat {
			return 0, fmt.Errorf("floating constant requires a floating-point type")
		}
		precision := uint(113)
		if typeSpec.Bits == 16 {
			precision = 11
		} else if typeSpec.Bits == 32 {
			precision = 24
		} else if typeSpec.Bits == 64 {
			precision = 53
		}
		value, _, err := big.ParseFloat(spec.Float, 10, precision, big.ToNearestEven)
		if err != nil {
			return 0, fmt.Errorf("invalid floating constant %q", spec.Float)
		}
		spec.Float = value.Text('g', -1)
	case ConstantUndef:
		if spec.Integer != "" || spec.Float != "" || len(spec.Elements) != 0 || spec.Global != 0 || spec.Function != 0 || spec.Bytes != "" || spec.NullTerminated {
			return 0, fmt.Errorf("undef constant has a payload")
		}
		if typeSpec.Kind == TypeVoid {
			return 0, fmt.Errorf("void has no undef constant")
		}
	case ConstantAggregate:
		if spec.Integer != "" || spec.Float != "" || spec.Global != 0 || spec.Function != 0 || spec.Bytes != "" || spec.NullTerminated {
			return 0, fmt.Errorf("aggregate constant has an unrelated payload")
		}
		if err := r.validateAggregate(typeSpec, spec); err != nil {
			return 0, err
		}
	case ConstantString:
		if spec.Integer != "" || spec.Float != "" || len(spec.Elements) != 0 || spec.Global != 0 || spec.Function != 0 {
			return 0, fmt.Errorf("string constant has an unrelated payload")
		}
		if typeSpec.Kind != TypeArray {
			return 0, fmt.Errorf("string constant requires an array type")
		}
		element, err := r.Type(typeSpec.Element)
		if err != nil || element.Kind != TypeInteger || element.Bits != 8 {
			return 0, fmt.Errorf("string constant requires an i8 array")
		}
		length := uint64(len(spec.Bytes))
		if spec.NullTerminated {
			length++
		}
		if typeSpec.Length != length {
			return 0, fmt.Errorf("string constant length %d does not match array length %d", length, typeSpec.Length)
		}
	case ConstantGlobalAddress:
		if spec.Integer != "" || spec.Float != "" || len(spec.Elements) != 0 || spec.Function != 0 || spec.Bytes != "" || spec.NullTerminated {
			return 0, fmt.Errorf("global-address constant has an unrelated payload")
		}
		if typeSpec.Kind != TypePointer {
			return 0, fmt.Errorf("global-address constant requires a pointer type")
		}
		global, err := r.Global(spec.Global)
		if err != nil {
			return 0, fmt.Errorf("global-address constant: %w", err)
		}
		if typeSpec.AddressSpace != global.AddressSpace {
			return 0, fmt.Errorf("global-address constant address space does not match global %q", global.Symbol)
		}
	case ConstantFunctionAddress:
		if spec.Integer != "" || spec.Float != "" || len(spec.Elements) != 0 || spec.Global != 0 || spec.Bytes != "" || spec.NullTerminated {
			return 0, fmt.Errorf("function-address constant has an unrelated payload")
		}
		if typeSpec.Kind != TypePointer {
			return 0, fmt.Errorf("function-address constant requires a pointer type")
		}
		if _, err := r.Function(spec.Function); err != nil {
			return 0, fmt.Errorf("function-address constant: %w", err)
		}
	default:
		return 0, fmt.Errorf("unknown constant kind %d", spec.Kind)
	}
	key := constantKey(spec)
	if id := r.constants[key]; id != 0 {
		return id, nil
	}
	id := ConstantID(len(r.constantSpecs) + 1)
	r.constants[key] = id
	spec.Elements = append([]ConstantID(nil), spec.Elements...)
	r.constantSpecs = append(r.constantSpecs, spec)
	return id, nil
}

func (r *Registry) Constant(id ConstantID) (ConstantSpec, error) {
	if id == 0 || int(id) > len(r.constantSpecs) {
		return ConstantSpec{}, fmt.Errorf("unknown backend constant %d", id)
	}
	spec := r.constantSpecs[id-1]
	spec.Elements = append([]ConstantID(nil), spec.Elements...)
	return spec, nil
}

func constantKey(spec ConstantSpec) string {
	return fmt.Sprintf("%d:%d:%s:%s:%q:%t:%v:%d:%d", spec.Kind, spec.Type, spec.Integer, spec.Float, spec.Bytes, spec.NullTerminated, spec.Elements, spec.Global, spec.Function)
}

func (r *Registry) validateAggregate(typeSpec TypeSpec, spec ConstantSpec) error {
	actual := make([]TypeID, len(spec.Elements))
	for i, id := range spec.Elements {
		child, err := r.Constant(id)
		if err != nil {
			return fmt.Errorf("aggregate element %d: %w", i, err)
		}
		actual[i] = child.Type
	}
	switch typeSpec.Kind {
	case TypeArray:
		if uint64(len(actual)) != typeSpec.Length {
			return fmt.Errorf("aggregate has %d elements; array requires %d", len(actual), typeSpec.Length)
		}
		for i, id := range actual {
			if id != typeSpec.Element {
				return fmt.Errorf("aggregate element %d has the wrong type", i)
			}
		}
	case TypeStruct:
		structure, err := r.Struct(spec.Type)
		if err != nil {
			return err
		}
		if structure.Opaque {
			return fmt.Errorf("opaque struct %q has no aggregate constants", structure.Name)
		}
		if len(actual) != len(structure.Elements) {
			return fmt.Errorf("aggregate has %d elements; struct requires %d", len(actual), len(structure.Elements))
		}
		for i, id := range actual {
			if id != structure.Elements[i] {
				return fmt.Errorf("aggregate element %d has the wrong type", i)
			}
		}
	default:
		return fmt.Errorf("aggregate constant requires an array or struct type")
	}
	return nil
}

func (r *Registry) DeclareGlobal(spec GlobalSpec) (GlobalID, error) {
	if spec.Symbol == "" {
		return 0, fmt.Errorf("global symbol must not be empty")
	}
	if !r.ValidType(spec.Type) {
		return 0, fmt.Errorf("global %q has unknown type %d", spec.Symbol, spec.Type)
	}
	if !validLinkage(spec.Linkage) {
		return 0, fmt.Errorf("global %q has invalid linkage %d", spec.Symbol, spec.Linkage)
	}
	if !validVisibility(spec.Visibility) {
		return 0, fmt.Errorf("global %q has invalid visibility %d", spec.Symbol, spec.Visibility)
	}
	if spec.Alignment != 0 && spec.Alignment&(spec.Alignment-1) != 0 {
		return 0, fmt.Errorf("global %q alignment %d is not a power of two", spec.Symbol, spec.Alignment)
	}
	if spec.Definition {
		constant, err := r.Constant(spec.Initializer)
		if err != nil {
			return 0, fmt.Errorf("global %q initializer: %w", spec.Symbol, err)
		}
		if constant.Type != spec.Type {
			return 0, fmt.Errorf("global %q initializer type does not match", spec.Symbol)
		}
	} else if spec.Initializer != 0 {
		return 0, fmt.Errorf("global declaration %q has an initializer", spec.Symbol)
	}
	if id := r.globalIDs[spec.Symbol]; id != 0 {
		existing := r.globals[id-1]
		if !sameGlobal(existing, spec) {
			return 0, fmt.Errorf("incompatible declaration of global %q", spec.Symbol)
		}
		if spec.Definition && r.globalDefined[spec.Symbol] {
			if existing.Definition && existing.Initializer == spec.Initializer {
				return id, nil
			}
			return 0, fmt.Errorf("duplicate definition of global %q", spec.Symbol)
		}
		if spec.Definition {
			r.globalDefined[spec.Symbol] = true
			r.globals[id-1] = spec
		}
		return id, nil
	}
	id := GlobalID(len(r.globals) + 1)
	r.globals = append(r.globals, spec)
	r.globalIDs[spec.Symbol] = id
	r.globalDefined[spec.Symbol] = spec.Definition
	return id, nil
}

func sameGlobal(a, b GlobalSpec) bool {
	return a.Symbol == b.Symbol && a.Type == b.Type && a.Linkage == b.Linkage && a.Visibility == b.Visibility &&
		a.AddressSpace == b.AddressSpace && a.Alignment == b.Alignment && a.Section == b.Section &&
		a.ThreadLocal == b.ThreadLocal && a.Constant == b.Constant && a.UnnamedAddress == b.UnnamedAddress
}

func (r *Registry) Global(id GlobalID) (GlobalSpec, error) {
	if id == 0 || int(id) > len(r.globals) {
		return GlobalSpec{}, fmt.Errorf("unknown backend global %d", id)
	}
	return r.globals[id-1], nil
}

func (r *Registry) InternType(spec TypeSpec) (TypeID, error) {
	if err := spec.Validate(); err != nil {
		return 0, err
	}
	if (spec.Kind == TypeArray || spec.Kind == TypeVector) && !r.ValidType(spec.Element) {
		return 0, fmt.Errorf("aggregate element type %d does not belong to this context", spec.Element)
	}
	if id := r.types[spec]; id != 0 {
		return id, nil
	}
	id := TypeID(len(r.typeSpecs) + 1)
	r.types[spec] = id
	r.typeSpecs = append(r.typeSpecs, spec)
	return id, nil
}

func (r *Registry) ValidType(id TypeID) bool { return id > 0 && int(id) <= len(r.typeSpecs) }

func (r *Registry) Type(id TypeID) (TypeSpec, error) {
	if !r.ValidType(id) {
		return TypeSpec{}, fmt.Errorf("unknown backend type %d", id)
	}
	return r.typeSpecs[id-1], nil
}

func (r *Registry) DeclareFunction(spec FunctionSpec) (FunctionID, error) {
	if spec.Symbol == "" {
		return 0, fmt.Errorf("function symbol must not be empty")
	}
	if !r.ValidType(spec.Result) {
		return 0, fmt.Errorf("function %q has unknown result type %d", spec.Symbol, spec.Result)
	}
	for i, parameter := range spec.Parameters {
		if !r.ValidType(parameter) {
			return 0, fmt.Errorf("function %q parameter %d has unknown type %d", spec.Symbol, i, parameter)
		}
	}
	if !validLinkage(spec.Linkage) {
		return 0, fmt.Errorf("function %q has invalid linkage %d", spec.Symbol, spec.Linkage)
	}
	convention, attributes, err := r.NormalizeCallABI(spec.CallingConvention, spec.Attributes, len(spec.Parameters), spec.Variadic)
	if err != nil {
		return 0, fmt.Errorf("function %q: %w", spec.Symbol, err)
	}
	spec.CallingConvention, spec.Attributes = convention, attributes
	if id := r.functionIDs[spec.Symbol]; id != 0 {
		existing := r.functions[id-1]
		if !sameFunction(existing, spec) {
			return 0, fmt.Errorf("incompatible declaration of function %q: previous=%+v new=%+v", spec.Symbol, existing, spec)
		}
		if spec.Definition && r.defined[spec.Symbol] {
			return 0, fmt.Errorf("duplicate definition of function %q", spec.Symbol)
		}
		if spec.Definition {
			r.defined[spec.Symbol] = true
		}
		return id, nil
	}
	id := FunctionID(len(r.functions) + 1)
	copySpec := spec
	copySpec.Parameters = append([]TypeID(nil), spec.Parameters...)
	copySpec.Attributes = append([]AttributeSpec(nil), spec.Attributes...)
	r.functions = append(r.functions, copySpec)
	r.functionIDs[spec.Symbol] = id
	r.defined[spec.Symbol] = spec.Definition
	return id, nil
}

func validLinkage(linkage Linkage) bool {
	return linkage == LinkageInternal || linkage == LinkageExternal || linkage == LinkagePrivate
}

func validVisibility(visibility Visibility) bool {
	return visibility == VisibilityDefault || visibility == VisibilityHidden || visibility == VisibilityProtected
}

func validCallingConvention(value CallingConvention) bool {
	return value == CallingConventionC || value == CallingConventionFast || value == CallingConventionCold
}

func (r *Registry) NormalizeCallABI(convention CallingConvention, attributes []AttributeSpec, parameterCount int, variadic bool) (CallingConvention, []AttributeSpec, error) {
	if convention == 0 {
		convention = CallingConventionC
	}
	if !validCallingConvention(convention) {
		return 0, nil, fmt.Errorf("invalid calling convention %d", convention)
	}
	if variadic && convention != CallingConventionC {
		return 0, nil, fmt.Errorf("calling convention does not support variadic arguments")
	}
	attributes = append([]AttributeSpec(nil), attributes...)
	for i, attribute := range attributes {
		if err := r.validateAttribute(attribute, parameterCount); err != nil {
			return 0, nil, fmt.Errorf("attribute %d: %w", i, err)
		}
	}
	sort.Slice(attributes, func(i, j int) bool { return attributeKey(attributes[i]) < attributeKey(attributes[j]) })
	for i := 1; i < len(attributes); i++ {
		if attributeIdentity(attributes[i-1]) == attributeIdentity(attributes[i]) {
			return 0, nil, fmt.Errorf("conflicting duplicate attribute")
		}
	}
	if hasAttribute(attributes, AttributeAlwaysInline) && hasAttribute(attributes, AttributeNoInline) {
		return 0, nil, fmt.Errorf("cannot be both alwaysinline and noinline")
	}
	return convention, attributes, nil
}

func attributeKey(a AttributeSpec) string {
	return fmt.Sprintf("%03d:%03d:%010d:%020d:%010d", a.Placement, a.Kind, a.Parameter, a.Value, a.Type)
}

func attributeIdentity(a AttributeSpec) string {
	return fmt.Sprintf("%03d:%03d:%010d", a.Placement, a.Kind, a.Parameter)
}
func hasAttribute(attributes []AttributeSpec, kind AttributeKind) bool {
	for _, attribute := range attributes {
		if attribute.Kind == kind {
			return true
		}
	}
	return false
}

func (r *Registry) validateAttribute(a AttributeSpec, parameterCount int) error {
	if a.Placement < AttributeFunction || a.Placement > AttributeParameter {
		return fmt.Errorf("invalid placement %d", a.Placement)
	}
	if a.Placement == AttributeParameter {
		if int(a.Parameter) >= parameterCount {
			return fmt.Errorf("parameter %d is out of range", a.Parameter)
		}
	} else if a.Parameter != 0 {
		return fmt.Errorf("non-parameter attribute has a parameter index")
	}
	if a.Kind < AttributeNoReturn || a.Kind > AttributeByValue {
		return fmt.Errorf("invalid kind %d", a.Kind)
	}
	allowed := false
	switch a.Placement {
	case AttributeFunction:
		allowed = a.Kind == AttributeNoReturn || a.Kind == AttributeNoUnwind || a.Kind == AttributeReadOnly || a.Kind == AttributeAlwaysInline || a.Kind == AttributeNoInline || a.Kind == AttributeCold
	case AttributeReturn:
		allowed = a.Kind == AttributeZeroExtend || a.Kind == AttributeSignExtend || a.Kind == AttributeNonNull || a.Kind == AttributeNoAlias
	case AttributeParameter:
		allowed = a.Kind == AttributeZeroExtend || a.Kind == AttributeSignExtend || a.Kind == AttributeNonNull || a.Kind == AttributeNoAlias || a.Kind == AttributeAlignment || a.Kind == AttributeDereferenceable || a.Kind == AttributeStructReturn || a.Kind == AttributeByValue
	}
	if !allowed {
		return fmt.Errorf("kind %d is invalid at placement %d", a.Kind, a.Placement)
	}
	needsValue := a.Kind == AttributeAlignment || a.Kind == AttributeDereferenceable
	if needsValue != (a.Value != 0) {
		return fmt.Errorf("attribute kind %d has invalid value %d", a.Kind, a.Value)
	}
	if a.Kind == AttributeAlignment && a.Value&(a.Value-1) != 0 {
		return fmt.Errorf("alignment %d is not a power of two", a.Value)
	}
	needsType := a.Kind == AttributeStructReturn || a.Kind == AttributeByValue
	if needsType {
		if !r.ValidType(a.Type) {
			return fmt.Errorf("attribute kind %d has unknown type %d", a.Kind, a.Type)
		}
	} else if a.Type != 0 {
		return fmt.Errorf("attribute kind %d has an unexpected type", a.Kind)
	}
	return nil
}

func (r *Registry) Function(id FunctionID) (FunctionSpec, error) {
	if id == 0 || int(id) > len(r.functions) {
		return FunctionSpec{}, fmt.Errorf("unknown backend function %d", id)
	}
	spec := r.functions[id-1]
	spec.Parameters = append([]TypeID(nil), spec.Parameters...)
	spec.Attributes = append([]AttributeSpec(nil), spec.Attributes...)
	return spec, nil
}

func sameFunction(a, b FunctionSpec) bool {
	if a.Symbol != b.Symbol || a.Result != b.Result || a.Variadic != b.Variadic || a.Linkage != b.Linkage || a.CallingConvention != b.CallingConvention || len(a.Parameters) != len(b.Parameters) || len(a.Attributes) != len(b.Attributes) {
		return false
	}
	for i := range a.Parameters {
		if a.Parameters[i] != b.Parameters[i] {
			return false
		}
	}
	for i := range a.Attributes {
		if a.Attributes[i] != b.Attributes[i] {
			return false
		}
	}
	return true
}
