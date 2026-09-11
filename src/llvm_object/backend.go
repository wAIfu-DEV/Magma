//go:build llvm_object

package llvmobject

import (
	lb "Magma/src/lowering_backend"
	"fmt"
	"math/big"
	"strings"

	llvm "tinygo.org/x/go-llvm"
)

type neutralValue struct {
	value    Value
	function lb.FunctionID
	typeID   lb.TypeID
	pointee  lb.TypeID
	constant lb.ConstantID
}

type neutralPhi struct {
	block    lb.BlockID
	incoming map[lb.BlockID]struct{}
}

// LoweringBackend adapts the backend-neutral opaque-ID contract to go-llvm.
// It is intentionally small; feature ports expand it without leaking LLVM
// handles or semantic AST nodes through loweringbackend.Backend.
type LoweringBackend struct {
	module    *Module
	registry  *lb.Registry
	cfg       *lb.CFG
	types     map[lb.TypeID]Type
	constants map[lb.ConstantID]Value
	globals   map[lb.GlobalID]Value
	functions map[lb.FunctionID]Function
	blocks    map[lb.BlockID]Block
	values    map[lb.ValueID]neutralValue
	nextValue lb.ValueID
	phis      map[lb.ValueID]*neutralPhi
}

var _ lb.Backend = (*LoweringBackend)(nil)

func NewLoweringBackend(name string) (*LoweringBackend, error) {
	module, err := NewModule(name)
	if err != nil {
		return nil, err
	}
	return &LoweringBackend{
		module: module, registry: lb.NewRegistry(), cfg: lb.NewCFG(),
		types: make(map[lb.TypeID]Type), functions: make(map[lb.FunctionID]Function),
		constants: make(map[lb.ConstantID]Value), globals: make(map[lb.GlobalID]Value),
		blocks: make(map[lb.BlockID]Block), values: make(map[lb.ValueID]neutralValue),
		phis: make(map[lb.ValueID]*neutralPhi),
	}, nil
}

func (b *LoweringBackend) ConfigureModule(spec lb.ModuleSpec) error {
	if spec.SourceFile != "" && spec.SourceFile != b.module.name {
		return fmt.Errorf("go-llvm cannot change source filename independently; create the backend with %q", spec.SourceFile)
	}
	return b.module.configure(spec.TargetTriple, spec.DataLayout)
}

func (b *LoweringBackend) TypeLayout(id lb.TypeID) (lb.TypeLayout, error) {
	typeValue, ok := b.types[id]
	if !ok {
		return lb.TypeLayout{}, fmt.Errorf("type %d is not materialized", id)
	}
	if err := b.requireSizedType(id); err != nil {
		return lb.TypeLayout{}, err
	}
	if b.module.dataLayout == "" {
		return lb.TypeLayout{}, fmt.Errorf("module data layout is not configured")
	}
	data := llvm.NewTargetData(b.module.dataLayout)
	defer data.Dispose()
	return lb.TypeLayout{SizeBits: data.TypeSizeInBits(typeValue.raw), StoreSize: data.TypeStoreSize(typeValue.raw), AllocationSize: data.TypeAllocSize(typeValue.raw), ABIAlignment: uint32(data.ABITypeAlignment(typeValue.raw)), PreferredAlignment: uint32(data.PrefTypeAlignment(typeValue.raw))}, nil
}

func (b *LoweringBackend) StructFieldOffset(id lb.TypeID, field uint32) (uint64, error) {
	structure, err := b.registry.Struct(id)
	if err != nil {
		return 0, err
	}
	if structure.Opaque {
		return 0, fmt.Errorf("opaque struct %q has no field layout", structure.Name)
	}
	if uint64(field) >= uint64(len(structure.Elements)) {
		return 0, fmt.Errorf("struct field %d is out of range", field)
	}
	if b.module.dataLayout == "" {
		return 0, fmt.Errorf("module data layout is not configured")
	}
	data := llvm.NewTargetData(b.module.dataLayout)
	defer data.Dispose()
	return data.ElementOffset(b.types[id].raw, int(field)), nil
}

func (b *LoweringBackend) requireSizedType(id lb.TypeID) error {
	spec, err := b.registry.Type(id)
	if err != nil {
		return err
	}
	if spec.Kind == lb.TypeVoid || spec.Kind == lb.TypeFunction {
		return fmt.Errorf("type %d is not sized", id)
	}
	if spec.Kind == lb.TypeStruct {
		structure, _ := b.registry.Struct(id)
		if structure.Opaque {
			return fmt.Errorf("opaque struct %q is not sized", structure.Name)
		}
	}
	return nil
}

func (b *LoweringBackend) InternStruct(spec lb.StructSpec) (lb.TypeID, error) {
	id, err := b.registry.InternStruct(spec)
	if err != nil {
		return 0, err
	}
	if _, ok := b.types[id]; ok {
		return id, nil
	}
	if spec.Name != "" {
		object, err := b.module.namedStructType(spec.Name)
		if err != nil {
			return 0, err
		}
		b.types[id] = object
		if !spec.Opaque {
			if err := b.materializeStructBody(id, spec.Elements, spec.Packed); err != nil {
				return 0, err
			}
		}
		return id, nil
	}
	elements, err := b.materializedTypes(spec.Elements)
	if err != nil {
		return 0, err
	}
	object, err := b.module.literalStructType(elements, spec.Packed)
	if err != nil {
		return 0, err
	}
	b.types[id] = object
	return id, nil
}

func (b *LoweringBackend) DefineStruct(id lb.TypeID, elements []lb.TypeID, packed bool) error {
	if err := b.registry.DefineStruct(id, elements, packed); err != nil {
		return err
	}
	return b.materializeStructBody(id, elements, packed)
}

func (b *LoweringBackend) materializeStructBody(id lb.TypeID, ids []lb.TypeID, packed bool) error {
	target, ok := b.types[id]
	if !ok {
		return fmt.Errorf("struct type %d is not materialized", id)
	}
	elements, err := b.materializedTypes(ids)
	if err != nil {
		return err
	}
	return b.module.setStructBody(target, elements, packed)
}

func (b *LoweringBackend) InternFunctionType(spec lb.FunctionTypeSpec) (lb.TypeID, error) {
	id, err := b.registry.InternFunctionType(spec)
	if err != nil {
		return 0, err
	}
	if _, ok := b.types[id]; ok {
		return id, nil
	}
	result, ok := b.types[spec.Result]
	if !ok {
		return 0, fmt.Errorf("function result type %d is not materialized", spec.Result)
	}
	parameters, err := b.materializedTypes(spec.Parameters)
	if err != nil {
		return 0, err
	}
	object, err := b.module.functionType(result, parameters, spec.Variadic)
	if err != nil {
		return 0, err
	}
	b.types[id] = object
	return id, nil
}

func (b *LoweringBackend) materializedTypes(ids []lb.TypeID) ([]Type, error) {
	result := make([]Type, len(ids))
	for i, id := range ids {
		value, ok := b.types[id]
		if !ok {
			return nil, fmt.Errorf("type %d is not materialized", id)
		}
		result[i] = value
	}
	return result, nil
}

func (b *LoweringBackend) InternConstant(spec lb.ConstantSpec) (lb.ConstantID, error) {
	id, err := b.registry.InternConstant(spec)
	if err != nil {
		return 0, err
	}
	if _, exists := b.constants[id]; exists {
		return id, nil
	}
	typeValue, ok := b.types[spec.Type]
	if !ok {
		return 0, fmt.Errorf("constant type %d is not materialized", spec.Type)
	}
	canonical, _ := b.registry.Constant(id)
	var raw llvm.Value
	switch canonical.Kind {
	case lb.ConstantInteger:
		raw = llvm.ConstIntFromString(typeValue.raw, canonical.Integer, 10)
	case lb.ConstantNull:
		raw = llvm.ConstPointerNull(typeValue.raw)
	case lb.ConstantZero:
		raw = llvm.ConstNull(typeValue.raw)
	case lb.ConstantFloat:
		raw = llvm.ConstFloatFromString(typeValue.raw, canonical.Float)
	case lb.ConstantUndef:
		raw = llvm.Undef(typeValue.raw)
	case lb.ConstantString:
		raw = b.module.ctx.ConstString(canonical.Bytes, canonical.NullTerminated)
	case lb.ConstantGlobalAddress:
		global, ok := b.globals[canonical.Global]
		if !ok {
			return 0, fmt.Errorf("global-address constant references unmaterialized global %d", canonical.Global)
		}
		raw = global.raw
	case lb.ConstantFunctionAddress:
		function, ok := b.functions[canonical.Function]
		if !ok {
			return 0, fmt.Errorf("function-address constant references unmaterialized function %d", canonical.Function)
		}
		raw = function.value.raw
	case lb.ConstantAggregate:
		elements := make([]llvm.Value, len(canonical.Elements))
		for i, elementID := range canonical.Elements {
			element, ok := b.constants[elementID]
			if !ok {
				return 0, fmt.Errorf("aggregate element %d is not materialized", elementID)
			}
			elements[i] = element.raw
		}
		typeSpec, _ := b.registry.Type(canonical.Type)
		if typeSpec.Kind == lb.TypeArray {
			raw = llvm.ConstArray(b.types[typeSpec.Element].raw, elements)
		} else {
			structure, _ := b.registry.Struct(canonical.Type)
			if structure.Name != "" {
				raw = llvm.ConstNamedStruct(typeValue.raw, elements)
			} else {
				raw = b.module.ctx.ConstStruct(elements, structure.Packed)
			}
		}
	default:
		return 0, fmt.Errorf("unsupported constant kind %d", canonical.Kind)
	}
	b.constants[id] = Value{raw: raw, owner: b.module}
	return id, nil
}

func (b *LoweringBackend) DeclareGlobal(spec lb.GlobalSpec) (lb.GlobalID, error) {
	id, err := b.registry.DeclareGlobal(spec)
	if err != nil {
		return 0, err
	}
	global, exists := b.globals[id]
	if !exists {
		typeValue, ok := b.types[spec.Type]
		if !ok {
			return 0, fmt.Errorf("global type %d is not materialized", spec.Type)
		}
		raw := llvm.AddGlobalInAddressSpace(b.module.raw, typeValue.raw, spec.Symbol, int(spec.AddressSpace))
		global = Value{raw: raw, owner: b.module}
		b.globals[id] = global
	}
	if spec.Definition {
		initializer, ok := b.constants[spec.Initializer]
		if !ok {
			return 0, fmt.Errorf("global initializer %d is not materialized", spec.Initializer)
		}
		global.raw.SetInitializer(initializer.raw)
	}
	switch spec.Linkage {
	case lb.LinkageInternal:
		global.raw.SetLinkage(llvm.InternalLinkage)
	case lb.LinkageExternal:
		global.raw.SetLinkage(llvm.ExternalLinkage)
	case lb.LinkagePrivate:
		global.raw.SetLinkage(llvm.PrivateLinkage)
	}
	switch spec.Visibility {
	case lb.VisibilityDefault:
		global.raw.SetVisibility(llvm.DefaultVisibility)
	case lb.VisibilityHidden:
		global.raw.SetVisibility(llvm.HiddenVisibility)
	case lb.VisibilityProtected:
		global.raw.SetVisibility(llvm.ProtectedVisibility)
	}
	global.raw.SetThreadLocal(spec.ThreadLocal)
	global.raw.SetGlobalConstant(spec.Constant)
	global.raw.SetUnnamedAddr(spec.UnnamedAddress)
	if spec.Alignment != 0 {
		global.raw.SetAlignment(int(spec.Alignment))
	}
	if spec.Section != "" {
		global.raw.SetSection(spec.Section)
	}
	return id, nil
}

func (b *LoweringBackend) Close() {
	if b != nil {
		b.module.Close()
	}
}

func (b *LoweringBackend) InternType(spec lb.TypeSpec) (lb.TypeID, error) {
	id, err := b.registry.InternType(spec)
	if err != nil {
		return 0, err
	}
	if _, exists := b.types[id]; exists {
		return id, nil
	}
	var object Type
	switch spec.Kind {
	case lb.TypeVoid:
		object, err = b.module.VoidType()
	case lb.TypeInteger:
		object, err = b.module.IntType(int(spec.Bits))
	case lb.TypeFloat:
		object, err = b.module.floatType(spec.Bits)
	case lb.TypePointer:
		object, err = b.module.PointerType(int(spec.AddressSpace))
	case lb.TypeArray:
		element, ok := b.types[spec.Element]
		if !ok {
			return 0, fmt.Errorf("array element type %d has not been materialized", spec.Element)
		}
		object, err = b.module.arrayType(element, spec.Length)
	case lb.TypeVector:
		element, ok := b.types[spec.Element]
		if !ok {
			return 0, fmt.Errorf("vector element type %d has not been materialized", spec.Element)
		}
		object, err = b.module.vectorType(element, spec.Length)
	default:
		err = fmt.Errorf("unsupported object type kind %d", spec.Kind)
	}
	if err != nil {
		return 0, backendError(ErrorContext{Operation: "materialize backend type"}, err)
	}
	b.types[id] = object
	return id, nil
}

func (b *LoweringBackend) DeclareFunction(spec lb.FunctionSpec) (lb.FunctionID, error) {
	id, err := b.registry.DeclareFunction(spec)
	if err != nil {
		return 0, err
	}
	if _, exists := b.functions[id]; exists {
		return id, nil
	}
	result, ok := b.types[spec.Result]
	if !ok {
		return 0, fmt.Errorf("function result type %d is not materialized", spec.Result)
	}
	params := make([]Type, len(spec.Parameters))
	for i, parameter := range spec.Parameters {
		var found bool
		params[i], found = b.types[parameter]
		if !found {
			return 0, fmt.Errorf("function parameter type %d is not materialized", parameter)
		}
	}
	function, err := b.module.newFunction(spec.Symbol, result, params, spec.Variadic)
	if err != nil {
		return 0, err
	}
	switch spec.Linkage {
	case lb.LinkageInternal:
		function.value.raw.SetLinkage(llvm.InternalLinkage)
	case lb.LinkageExternal:
		function.value.raw.SetLinkage(llvm.ExternalLinkage)
	case lb.LinkagePrivate:
		function.value.raw.SetLinkage(llvm.PrivateLinkage)
	default:
		return 0, fmt.Errorf("unsupported function linkage %d", spec.Linkage)
	}
	b.functions[id] = function
	canonical, _ := b.registry.Function(id)
	if err := b.applyFunctionABI(function, canonical); err != nil {
		return 0, err
	}
	return id, nil
}

func (b *LoweringBackend) applyFunctionABI(function Function, spec lb.FunctionSpec) error {
	convention, err := objectCallingConvention(spec.CallingConvention)
	if err != nil {
		return err
	}
	function.value.raw.SetFunctionCallConv(convention)
	for _, spec := range spec.Attributes {
		attribute, err := b.objectAttribute(spec)
		if err != nil {
			return err
		}
		index := -1
		if spec.Placement == lb.AttributeReturn {
			index = 0
		} else if spec.Placement == lb.AttributeParameter {
			index = int(spec.Parameter) + 1
		}
		function.value.raw.AddAttributeAtIndex(index, attribute)
	}
	return nil
}

func objectCallingConvention(value lb.CallingConvention) (llvm.CallConv, error) {
	switch value {
	case lb.CallingConventionC:
		return llvm.CCallConv, nil
	case lb.CallingConventionFast:
		return llvm.FastCallConv, nil
	case lb.CallingConventionCold:
		return llvm.ColdCallConv, nil
	default:
		return 0, fmt.Errorf("unsupported calling convention %d", value)
	}
}

func (b *LoweringBackend) objectAttribute(spec lb.AttributeSpec) (llvm.Attribute, error) {
	names := map[lb.AttributeKind]string{
		lb.AttributeNoReturn: "noreturn", lb.AttributeNoUnwind: "nounwind", lb.AttributeReadOnly: "readonly",
		lb.AttributeAlwaysInline: "alwaysinline", lb.AttributeNoInline: "noinline", lb.AttributeCold: "cold", lb.AttributeZeroExtend: "zeroext",
		lb.AttributeSignExtend: "signext", lb.AttributeNonNull: "nonnull", lb.AttributeNoAlias: "noalias",
		lb.AttributeAlignment: "align", lb.AttributeDereferenceable: "dereferenceable",
		lb.AttributeStructReturn: "sret", lb.AttributeByValue: "byval",
	}
	name := names[spec.Kind]
	kind := llvm.AttributeKindID(name)
	if kind == 0 {
		return llvm.Attribute{}, fmt.Errorf("LLVM does not support attribute %q", name)
	}
	if spec.Type != 0 {
		value, ok := b.types[spec.Type]
		if !ok {
			return llvm.Attribute{}, fmt.Errorf("attribute type %d is not materialized", spec.Type)
		}
		return b.module.ctx.CreateTypeAttribute(kind, value.raw), nil
	}
	return b.module.ctx.CreateEnumAttribute(kind, spec.Value), nil
}

func (b *LoweringBackend) AppendBlock(functionID lb.FunctionID, name string) (lb.BlockID, error) {
	function, ok := b.functions[functionID]
	if !ok {
		return 0, fmt.Errorf("unknown object function %d", functionID)
	}
	id, err := b.cfg.AppendBlock(functionID, name)
	if err != nil {
		return 0, err
	}
	block, err := b.module.NewBlock(function, name)
	if err != nil {
		return 0, err
	}
	b.blocks[id] = block
	return id, nil
}

func (b *LoweringBackend) Parameter(functionID lb.FunctionID, index int) (lb.ValueID, error) {
	function, ok := b.functions[functionID]
	if !ok {
		return 0, fmt.Errorf("unknown object function %d", functionID)
	}
	value, err := function.Param(index)
	if err != nil {
		return 0, err
	}
	spec, _ := b.registry.Function(functionID)
	return b.remember(value, functionID, spec.Parameters[index]), nil
}

func (b *LoweringBackend) ConstantValue(id lb.ConstantID) (lb.ValueID, error) {
	value, ok := b.constants[id]
	if !ok {
		return 0, fmt.Errorf("constant %d is not materialized", id)
	}
	spec, err := b.registry.Constant(id)
	if err != nil {
		return 0, err
	}
	result := b.remember(value, 0, spec.Type)
	tracked := b.values[result]
	tracked.constant = id
	b.values[result] = tracked
	return result, nil
}

func (b *LoweringBackend) GlobalAddress(id lb.GlobalID) (lb.ValueID, error) {
	value, ok := b.globals[id]
	if !ok {
		return 0, fmt.Errorf("global %d is not materialized", id)
	}
	spec, err := b.registry.Global(id)
	if err != nil {
		return 0, err
	}
	pointer, err := b.InternType(lb.TypeSpec{Kind: lb.TypePointer, AddressSpace: spec.AddressSpace})
	if err != nil {
		return 0, err
	}
	result := b.remember(value, 0, pointer)
	tracked := b.values[result]
	tracked.pointee = spec.Type
	b.values[result] = tracked
	return result, nil
}

func (b *LoweringBackend) FunctionAddress(id lb.FunctionID) (lb.ValueID, error) {
	function, ok := b.functions[id]
	if !ok {
		return 0, fmt.Errorf("function %d is not materialized", id)
	}
	spec, err := b.registry.Function(id)
	if err != nil {
		return 0, err
	}
	functionType, err := b.InternFunctionType(lb.FunctionTypeSpec{Result: spec.Result, Parameters: spec.Parameters, Variadic: spec.Variadic})
	if err != nil {
		return 0, err
	}
	pointer, err := b.InternType(lb.TypeSpec{Kind: lb.TypePointer})
	if err != nil {
		return 0, err
	}
	result := b.remember(function.value, 0, pointer)
	tracked := b.values[result]
	tracked.pointee = functionType
	b.values[result] = tracked
	return result, nil
}

func (b *LoweringBackend) Alloca(blockID lb.BlockID, allocatedID lb.TypeID, alignment uint32) (lb.ValueID, error) {
	builder, functionID, err := b.openBuilder(blockID)
	if err != nil {
		return 0, err
	}
	allocated, ok := b.types[allocatedID]
	if !ok {
		return 0, fmt.Errorf("alloca type %d is not materialized", allocatedID)
	}
	spec, _ := b.registry.Type(allocatedID)
	if spec.Kind == lb.TypeVoid || spec.Kind == lb.TypeFunction {
		return 0, fmt.Errorf("type %d cannot be allocated", allocatedID)
	}
	pointerID, err := b.InternType(lb.TypeSpec{Kind: lb.TypePointer})
	if err != nil {
		return 0, err
	}
	value, err := builder.Alloca(allocated, alignment, "")
	if err != nil {
		return 0, err
	}
	result := b.remember(value, functionID, pointerID)
	tracked := b.values[result]
	tracked.pointee = allocatedID
	b.values[result] = tracked
	return result, nil
}

// StaticAlloca mirrors textual lowering's fixed-size local storage rule: the
// slot is created in the function entry even when initialization occurs in a
// nested scope. It intentionally emits no lifetime markers or runtime checks.
func (b *LoweringBackend) StaticAlloca(functionID lb.FunctionID, allocatedID lb.TypeID, alignment uint32) (lb.ValueID, error) {
	if _, ok := b.functions[functionID]; !ok {
		return 0, fmt.Errorf("unknown object function %d", functionID)
	}
	entryID, err := b.cfg.EntryBlock(functionID)
	if err != nil {
		return 0, err
	}
	entry, ok := b.blocks[entryID]
	if !ok {
		return 0, fmt.Errorf("entry block %d is not materialized", entryID)
	}
	allocated, ok := b.types[allocatedID]
	if !ok {
		return 0, fmt.Errorf("alloca type %d is not materialized", allocatedID)
	}
	spec, _ := b.registry.Type(allocatedID)
	if spec.Kind == lb.TypeVoid || spec.Kind == lb.TypeFunction {
		return 0, fmt.Errorf("type %d cannot be allocated", allocatedID)
	}
	pointerID, err := b.InternType(lb.TypeSpec{Kind: lb.TypePointer})
	if err != nil {
		return 0, err
	}
	builder, err := b.module.BuilderAfterAllocas(entry)
	if err != nil {
		return 0, err
	}
	value, err := builder.Alloca(allocated, alignment, "")
	if err != nil {
		return 0, err
	}
	result := b.remember(value, functionID, pointerID)
	tracked := b.values[result]
	tracked.pointee = allocatedID
	b.values[result] = tracked
	return result, nil
}

func (b *LoweringBackend) DynamicAlloca(blockID lb.BlockID, allocatedID lb.TypeID, countID lb.ValueID, alignment uint32) (lb.ValueID, error) {
	builder, functionID, err := b.openBuilder(blockID)
	if err != nil {
		return 0, err
	}
	allocated, ok := b.types[allocatedID]
	if !ok {
		return 0, fmt.Errorf("dynamic alloca type %d is not materialized", allocatedID)
	}
	spec, _ := b.registry.Type(allocatedID)
	if spec.Kind == lb.TypeVoid || spec.Kind == lb.TypeFunction {
		return 0, fmt.Errorf("type %d cannot be dynamically allocated", allocatedID)
	}
	count, err := b.valueForFunction(countID, functionID, "dynamic alloca count")
	if err != nil {
		return 0, err
	}
	countSpec, _ := b.registry.Type(count.typeID)
	if countSpec.Kind != lb.TypeInteger {
		return 0, fmt.Errorf("dynamic alloca count must be an integer")
	}
	pointerID, err := b.InternType(lb.TypeSpec{Kind: lb.TypePointer})
	if err != nil {
		return 0, err
	}
	value, err := builder.DynamicAlloca(allocated, count.value, alignment, "")
	if err != nil {
		return 0, err
	}
	result := b.remember(value, functionID, pointerID)
	tracked := b.values[result]
	tracked.pointee = allocatedID
	b.values[result] = tracked
	return result, nil
}

func (b *LoweringBackend) Load(blockID lb.BlockID, loadedID lb.TypeID, pointerID lb.ValueID, alignment uint32, volatile bool) (lb.ValueID, error) {
	builder, functionID, err := b.openBuilder(blockID)
	if err != nil {
		return 0, err
	}
	loaded, ok := b.types[loadedID]
	if !ok {
		return 0, fmt.Errorf("load type %d is not materialized", loadedID)
	}
	pointer, err := b.valueForFunction(pointerID, functionID, "load pointer")
	if err != nil {
		return 0, err
	}
	if err := b.requirePointer(pointer.typeID, "load"); err != nil {
		return 0, err
	}
	if pointer.pointee != 0 && pointer.pointee != loadedID {
		return 0, fmt.Errorf("load type does not match pointer element type")
	}
	value, err := builder.Load(loaded, pointer.value, alignment, volatile, "")
	if err != nil {
		return 0, err
	}
	return b.remember(value, functionID, loadedID), nil
}

func (b *LoweringBackend) Store(blockID lb.BlockID, valueID, pointerID lb.ValueID, alignment uint32, volatile bool) (lb.ValueID, error) {
	builder, functionID, err := b.openBuilder(blockID)
	if err != nil {
		return 0, err
	}
	value, err := b.valueForFunction(valueID, functionID, "store value")
	if err != nil {
		return 0, err
	}
	pointer, err := b.valueForFunction(pointerID, functionID, "store pointer")
	if err != nil {
		return 0, err
	}
	if err := b.requirePointer(pointer.typeID, "store"); err != nil {
		return 0, err
	}
	if pointer.pointee != 0 && pointer.pointee != value.typeID {
		return 0, fmt.Errorf("store value type does not match pointer element type")
	}
	stored, err := builder.Store(value.value, pointer.value, alignment, volatile)
	if err != nil {
		return 0, err
	}
	voidID, err := b.InternType(lb.TypeSpec{Kind: lb.TypeVoid})
	if err != nil {
		return 0, err
	}
	return b.remember(stored, functionID, voidID), nil
}

func (b *LoweringBackend) ReinterpretPointer(valueID lb.ValueID) (lb.ValueID, error) {
	value, ok := b.values[valueID]
	if !ok {
		return 0, fmt.Errorf("unknown pointer value %d", valueID)
	}
	if err := b.requirePointer(value.typeID, "pointer reinterpretation"); err != nil {
		return 0, err
	}
	return b.remember(value.value, value.function, value.typeID), nil
}

func (b *LoweringBackend) AtomicLoad(blockID lb.BlockID, loadedID lb.TypeID, pointerID lb.ValueID, ordering lb.AtomicOrdering, alignment uint32) (lb.ValueID, error) {
	if ordering != lb.AtomicMonotonic && ordering != lb.AtomicAcquire && ordering != lb.AtomicSequentiallyConsistent {
		return 0, fmt.Errorf("invalid atomic load ordering %d", ordering)
	}
	value, err := b.Load(blockID, loadedID, pointerID, alignment, false)
	if err != nil {
		return 0, err
	}
	objectOrdering, err := objectAtomicOrdering(ordering)
	if err != nil {
		return 0, err
	}
	b.values[value].value.raw.SetOrdering(objectOrdering)
	return value, nil
}

func (b *LoweringBackend) AtomicStore(blockID lb.BlockID, valueID, pointerID lb.ValueID, ordering lb.AtomicOrdering, alignment uint32) (lb.ValueID, error) {
	if ordering != lb.AtomicMonotonic && ordering != lb.AtomicRelease && ordering != lb.AtomicSequentiallyConsistent {
		return 0, fmt.Errorf("invalid atomic store ordering %d", ordering)
	}
	value, err := b.Store(blockID, valueID, pointerID, alignment, false)
	if err != nil {
		return 0, err
	}
	objectOrdering, err := objectAtomicOrdering(ordering)
	if err != nil {
		return 0, err
	}
	b.values[value].value.raw.SetOrdering(objectOrdering)
	return value, nil
}

func (b *LoweringBackend) AtomicRMW(blockID lb.BlockID, operation lb.AtomicRMWOp, pointerID, valueID lb.ValueID, ordering lb.AtomicOrdering, alignment uint32) (lb.ValueID, error) {
	builder, functionID, err := b.openBuilder(blockID)
	if err != nil {
		return 0, err
	}
	if err := validateMemoryAlignment(alignment); err != nil {
		return 0, err
	}
	pointer, err := b.valueForFunction(pointerID, functionID, "atomicrmw pointer")
	if err != nil {
		return 0, err
	}
	if err := b.requirePointer(pointer.typeID, "atomicrmw"); err != nil {
		return 0, err
	}
	value, err := b.valueForFunction(valueID, functionID, "atomicrmw value")
	if err != nil {
		return 0, err
	}
	if pointer.pointee != 0 && pointer.pointee != value.typeID {
		return 0, fmt.Errorf("atomicrmw value type does not match pointer element type")
	}
	valueSpec, _ := b.registry.Type(value.typeID)
	var objectOperation llvm.AtomicRMWBinOp
	switch operation {
	case lb.AtomicRMWExchange:
		if valueSpec.Kind != lb.TypeInteger && valueSpec.Kind != lb.TypePointer {
			return 0, fmt.Errorf("atomic exchange requires an integer or pointer value")
		}
		objectOperation = llvm.AtomicRMWBinOpXchg
	case lb.AtomicRMWAdd:
		if valueSpec.Kind != lb.TypeInteger {
			return 0, fmt.Errorf("atomic add requires an integer value")
		}
		objectOperation = llvm.AtomicRMWBinOpAdd
	case lb.AtomicRMWSub:
		if valueSpec.Kind != lb.TypeInteger {
			return 0, fmt.Errorf("atomic sub requires an integer value")
		}
		objectOperation = llvm.AtomicRMWBinOpSub
	default:
		return 0, fmt.Errorf("invalid atomicrmw operation %d", operation)
	}
	objectOrdering, err := objectAtomicOrdering(ordering)
	if err != nil {
		return 0, err
	}
	raw := builder.module.builder.CreateAtomicRMW(objectOperation, pointer.value.raw, value.value.raw, objectOrdering, false)
	if alignment != 0 {
		raw.SetAlignment(int(alignment))
	}
	return b.remember(Value{raw: raw, owner: b.module}, functionID, value.typeID), nil
}

func (b *LoweringBackend) CompareExchangeOld(blockID lb.BlockID, pointerID, expectedID, desiredID lb.ValueID, success, failure lb.AtomicOrdering, alignment uint32) (lb.ValueID, error) {
	if !validCmpXchgOrderings(success, failure) {
		return 0, fmt.Errorf("invalid cmpxchg ordering pair %d/%d", success, failure)
	}
	builder, functionID, err := b.openBuilder(blockID)
	if err != nil {
		return 0, err
	}
	if err := validateMemoryAlignment(alignment); err != nil {
		return 0, err
	}
	pointer, err := b.valueForFunction(pointerID, functionID, "cmpxchg pointer")
	if err != nil {
		return 0, err
	}
	if err := b.requirePointer(pointer.typeID, "cmpxchg"); err != nil {
		return 0, err
	}
	expected, err := b.valueForFunction(expectedID, functionID, "cmpxchg expected value")
	if err != nil {
		return 0, err
	}
	desired, err := b.valueForFunction(desiredID, functionID, "cmpxchg desired value")
	if err != nil {
		return 0, err
	}
	if expected.typeID != desired.typeID || pointer.pointee != 0 && pointer.pointee != expected.typeID {
		return 0, fmt.Errorf("cmpxchg operand types do not match")
	}
	successOrdering, _ := objectAtomicOrdering(success)
	failureOrdering, _ := objectAtomicOrdering(failure)
	pair := builder.module.builder.CreateAtomicCmpXchg(pointer.value.raw, expected.value.raw, desired.value.raw, successOrdering, failureOrdering, false)
	if alignment != 0 {
		pair.SetAlignment(int(alignment))
	}
	old := builder.module.builder.CreateExtractValue(pair, 0, "")
	return b.remember(Value{raw: old, owner: b.module}, functionID, expected.typeID), nil
}

func validCmpXchgOrderings(success, failure lb.AtomicOrdering) bool {
	if failure != lb.AtomicMonotonic && failure != lb.AtomicAcquire && failure != lb.AtomicSequentiallyConsistent {
		return false
	}
	switch success {
	case lb.AtomicMonotonic, lb.AtomicRelease:
		return failure == lb.AtomicMonotonic
	case lb.AtomicAcquire, lb.AtomicAcquireRelease:
		return failure == lb.AtomicMonotonic || failure == lb.AtomicAcquire
	case lb.AtomicSequentiallyConsistent:
		return true
	default:
		return false
	}
}

func (b *LoweringBackend) InlineAssemblySideEffect(blockID lb.BlockID, instruction string) (lb.ValueID, error) {
	if instruction != "pause" && instruction != "yield" && instruction != "fence.seq_cst" {
		return 0, fmt.Errorf("unsupported inline assembly %q", instruction)
	}
	architecture := strings.ToLower(strings.SplitN(b.module.targetTriple, "-", 2)[0])
	if architecture == "" {
		return 0, fmt.Errorf("inline assembly %q requires a configured target triple", instruction)
	}
	validTarget := false
	assemblyInstruction := instruction
	if instruction == "pause" {
		validTarget = architecture == "x86_64" || architecture == "amd64" || architecture == "i386" || architecture == "i486" || architecture == "i586" || architecture == "i686"
	} else if instruction == "yield" {
		validTarget = architecture == "aarch64" || architecture == "arm64" || strings.HasPrefix(architecture, "arm") || strings.HasPrefix(architecture, "thumb")
	} else if architecture == "x86_64" || architecture == "amd64" || architecture == "i386" || architecture == "i486" || architecture == "i586" || architecture == "i686" {
		validTarget = true
		assemblyInstruction = "mfence"
	} else if architecture == "aarch64" || architecture == "arm64" || strings.HasPrefix(architecture, "arm") || strings.HasPrefix(architecture, "thumb") {
		validTarget = true
		assemblyInstruction = "dmb ish"
	}
	if !validTarget {
		return 0, fmt.Errorf("inline assembly %q is not supported for target %q", instruction, b.module.targetTriple)
	}
	builder, functionID, err := b.openBuilder(blockID)
	if err != nil {
		return 0, err
	}
	voidID, err := b.InternType(lb.TypeSpec{Kind: lb.TypeVoid})
	if err != nil {
		return 0, err
	}
	functionType, err := b.module.functionType(b.types[voidID], nil, false)
	if err != nil {
		return 0, err
	}
	assembly := llvm.InlineAsm(functionType.raw, assemblyInstruction, "~{memory}", true, false, llvm.InlineAsmDialectATT, false)
	call := builder.module.builder.CreateCall(functionType.raw, assembly, nil, "")
	return b.remember(Value{raw: call, owner: b.module}, functionID, voidID), nil
}

func (b *LoweringBackend) GEP(blockID lb.BlockID, elementID lb.TypeID, pointerID lb.ValueID, indexIDs []lb.ValueID, inbounds bool) (lb.ValueID, error) {
	builder, functionID, err := b.openBuilder(blockID)
	if err != nil {
		return 0, err
	}
	element, ok := b.types[elementID]
	if !ok {
		return 0, fmt.Errorf("GEP source element type %d is not materialized", elementID)
	}
	pointer, err := b.valueForFunction(pointerID, functionID, "GEP pointer")
	if err != nil {
		return 0, err
	}
	if err := b.requirePointer(pointer.typeID, "GEP"); err != nil {
		return 0, err
	}
	if pointer.pointee != 0 && pointer.pointee != elementID {
		return 0, fmt.Errorf("GEP source element type does not match pointer provenance")
	}
	indices := make([]Value, len(indexIDs))
	for i, id := range indexIDs {
		value, err := b.valueForFunction(id, functionID, "GEP index")
		if err != nil {
			return 0, err
		}
		spec, _ := b.registry.Type(value.typeID)
		if spec.Kind != lb.TypeInteger {
			return 0, fmt.Errorf("GEP index %d is not an integer", i)
		}
		indices[i] = value.value
	}
	pointee, err := b.gepResultType(elementID, indexIDs)
	if err != nil {
		return 0, err
	}
	value, err := builder.GEP(element, pointer.value, indices, inbounds, "")
	if err != nil {
		return 0, err
	}
	result := b.remember(value, functionID, pointer.typeID)
	tracked := b.values[result]
	tracked.pointee = pointee
	b.values[result] = tracked
	return result, nil
}

func (b *LoweringBackend) gepResultType(element lb.TypeID, indices []lb.ValueID) (lb.TypeID, error) {
	current := element
	for position := 1; position < len(indices); position++ {
		spec, err := b.registry.Type(current)
		if err != nil {
			return 0, err
		}
		switch spec.Kind {
		case lb.TypeArray:
			current = spec.Element
		case lb.TypeStruct:
			value := b.values[indices[position]]
			if value.constant == 0 {
				return 0, fmt.Errorf("struct GEP index %d is not constant", position)
			}
			constant, _ := b.registry.Constant(value.constant)
			if constant.Kind != lb.ConstantInteger {
				return 0, fmt.Errorf("struct GEP index %d is not an integer constant", position)
			}
			index, ok := new(big.Int).SetString(constant.Integer, 10)
			if !ok || !index.IsUint64() {
				return 0, fmt.Errorf("struct GEP index %d is out of range", position)
			}
			structure, _ := b.registry.Struct(current)
			n := index.Uint64()
			if n >= uint64(len(structure.Elements)) {
				return 0, fmt.Errorf("struct GEP index %d is out of range", n)
			}
			current = structure.Elements[n]
		default:
			return 0, fmt.Errorf("GEP index %d descends into a non-aggregate type", position)
		}
	}
	return current, nil
}

func (b *LoweringBackend) BuildAggregate(blockID lb.BlockID, typeID lb.TypeID, valueIDs []lb.ValueID) (lb.ValueID, error) {
	builder, functionID, err := b.openBuilder(blockID)
	if err != nil {
		return 0, err
	}
	arity, err := b.registry.AggregateArity(typeID)
	if err != nil {
		return 0, err
	}
	if uint64(len(valueIDs)) != arity {
		return 0, fmt.Errorf("aggregate type %d requires %d values, got %d", typeID, arity, len(valueIDs))
	}
	typeValue, ok := b.types[typeID]
	if !ok {
		return 0, fmt.Errorf("aggregate type %d is not materialized", typeID)
	}
	raw := llvm.Undef(typeValue.raw)
	for i, id := range valueIDs {
		value, err := b.valueForFunction(id, functionID, "aggregate element")
		if err != nil {
			return 0, err
		}
		expected, _ := b.registry.AggregateElement(typeID, []uint32{uint32(i)})
		if value.typeID != expected {
			return 0, fmt.Errorf("aggregate element %d has the wrong type", i)
		}
		raw = builder.module.builder.CreateInsertValue(raw, value.value.raw, i, "")
	}
	return b.remember(Value{raw: raw, owner: b.module}, functionID, typeID), nil
}

func (b *LoweringBackend) ExtractValue(blockID lb.BlockID, aggregateID lb.ValueID, path []uint32) (lb.ValueID, error) {
	builder, functionID, err := b.openBuilder(blockID)
	if err != nil {
		return 0, err
	}
	aggregate, err := b.valueForFunction(aggregateID, functionID, "extract aggregate")
	if err != nil {
		return 0, err
	}
	resultType, err := b.registry.AggregateElement(aggregate.typeID, path)
	if err != nil {
		return 0, err
	}
	raw := aggregate.value.raw
	current := aggregate.typeID
	for _, index := range path {
		if uint64(index) > uint64(^uint(0)>>1) {
			return 0, fmt.Errorf("aggregate index %d exceeds host int", index)
		}
		raw = builder.module.builder.CreateExtractValue(raw, int(index), "")
		current, _ = b.registry.AggregateElement(current, []uint32{index})
	}
	return b.remember(Value{raw: raw, owner: b.module}, functionID, resultType), nil
}

func (b *LoweringBackend) InsertValue(blockID lb.BlockID, aggregateID, valueID lb.ValueID, path []uint32) (lb.ValueID, error) {
	builder, functionID, err := b.openBuilder(blockID)
	if err != nil {
		return 0, err
	}
	aggregate, err := b.valueForFunction(aggregateID, functionID, "insert aggregate")
	if err != nil {
		return 0, err
	}
	value, err := b.valueForFunction(valueID, functionID, "insert value")
	if err != nil {
		return 0, err
	}
	expected, err := b.registry.AggregateElement(aggregate.typeID, path)
	if err != nil {
		return 0, err
	}
	if value.typeID != expected {
		return 0, fmt.Errorf("inserted value has the wrong type")
	}
	raw, err := b.insertValuePath(builder, aggregate.value.raw, value.value.raw, aggregate.typeID, path)
	if err != nil {
		return 0, err
	}
	return b.remember(Value{raw: raw, owner: b.module}, functionID, aggregate.typeID), nil
}

func (b *LoweringBackend) insertValuePath(builder *Builder, aggregate, value llvm.Value, aggregateType lb.TypeID, path []uint32) (llvm.Value, error) {
	index := path[0]
	if uint64(index) > uint64(^uint(0)>>1) {
		return llvm.Value{}, fmt.Errorf("aggregate index %d exceeds host int", index)
	}
	if len(path) == 1 {
		return builder.module.builder.CreateInsertValue(aggregate, value, int(index), ""), nil
	}
	inner := builder.module.builder.CreateExtractValue(aggregate, int(index), "")
	innerType, _ := b.registry.AggregateElement(aggregateType, []uint32{index})
	updated, err := b.insertValuePath(builder, inner, value, innerType, path[1:])
	if err != nil {
		return llvm.Value{}, err
	}
	return builder.module.builder.CreateInsertValue(aggregate, updated, int(index), ""), nil
}

func (b *LoweringBackend) StructFieldAddress(blockID lb.BlockID, structID lb.TypeID, pointerID lb.ValueID, field uint32) (lb.ValueID, error) {
	structure, err := b.registry.Struct(structID)
	if err != nil {
		return 0, err
	}
	if structure.Opaque {
		return 0, fmt.Errorf("opaque struct %q has no fields", structure.Name)
	}
	if int(field) >= len(structure.Elements) {
		return 0, fmt.Errorf("struct field %d is out of range", field)
	}
	i32, err := b.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 32})
	if err != nil {
		return 0, err
	}
	zeroConstant, err := b.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: i32, Integer: "0"})
	if err != nil {
		return 0, err
	}
	fieldConstant, err := b.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: i32, Integer: fmt.Sprint(field)})
	if err != nil {
		return 0, err
	}
	zero, _ := b.ConstantValue(zeroConstant)
	index, _ := b.ConstantValue(fieldConstant)
	return b.GEP(blockID, structID, pointerID, []lb.ValueID{zero, index}, true)
}

func (b *LoweringBackend) Binary(blockID lb.BlockID, operation lb.BinaryOp, leftID, rightID lb.ValueID) (lb.ValueID, error) {
	builder, functionID, err := b.openBuilder(blockID)
	if err != nil {
		return 0, err
	}
	left, err := b.valueForFunction(leftID, functionID, "binary left operand")
	if err != nil {
		return 0, err
	}
	right, err := b.valueForFunction(rightID, functionID, "binary right operand")
	if err != nil {
		return 0, err
	}
	if left.typeID != right.typeID {
		return 0, fmt.Errorf("binary operands have different types")
	}
	typeSpec, _ := b.registry.Type(left.typeID)
	integerOnly := operation >= lb.BinaryUnsignedDiv && operation <= lb.BinarySignedDiv || operation >= lb.BinaryUnsignedRem && operation <= lb.BinarySignedRem || operation >= lb.BinaryAnd
	floatOnly := operation == lb.BinaryFloatDiv || operation == lb.BinaryFloatRem
	if integerOnly && typeSpec.Kind != lb.TypeInteger {
		return 0, fmt.Errorf("binary operation %d requires integer operands", operation)
	}
	if floatOnly && typeSpec.Kind != lb.TypeFloat {
		return 0, fmt.Errorf("binary operation %d requires floating operands", operation)
	}
	if operation <= lb.BinaryMul && typeSpec.Kind != lb.TypeInteger && typeSpec.Kind != lb.TypeFloat {
		return 0, fmt.Errorf("binary arithmetic requires numeric operands")
	}
	var raw llvm.Value
	switch operation {
	case lb.BinaryAdd:
		if typeSpec.Kind == lb.TypeFloat {
			raw = builder.module.builder.CreateFAdd(left.value.raw, right.value.raw, "")
		} else {
			raw = builder.module.builder.CreateAdd(left.value.raw, right.value.raw, "")
		}
	case lb.BinarySub:
		if typeSpec.Kind == lb.TypeFloat {
			raw = builder.module.builder.CreateFSub(left.value.raw, right.value.raw, "")
		} else {
			raw = builder.module.builder.CreateSub(left.value.raw, right.value.raw, "")
		}
	case lb.BinaryMul:
		if typeSpec.Kind == lb.TypeFloat {
			raw = builder.module.builder.CreateFMul(left.value.raw, right.value.raw, "")
		} else {
			raw = builder.module.builder.CreateMul(left.value.raw, right.value.raw, "")
		}
	case lb.BinaryUnsignedDiv:
		raw = builder.module.builder.CreateUDiv(left.value.raw, right.value.raw, "")
	case lb.BinarySignedDiv:
		raw = builder.module.builder.CreateSDiv(left.value.raw, right.value.raw, "")
	case lb.BinaryFloatDiv:
		raw = builder.module.builder.CreateFDiv(left.value.raw, right.value.raw, "")
	case lb.BinaryUnsignedRem:
		raw = builder.module.builder.CreateURem(left.value.raw, right.value.raw, "")
	case lb.BinarySignedRem:
		raw = builder.module.builder.CreateSRem(left.value.raw, right.value.raw, "")
	case lb.BinaryFloatRem:
		raw = builder.module.builder.CreateFRem(left.value.raw, right.value.raw, "")
	case lb.BinaryAnd:
		raw = builder.module.builder.CreateAnd(left.value.raw, right.value.raw, "")
	case lb.BinaryOr:
		raw = builder.module.builder.CreateOr(left.value.raw, right.value.raw, "")
	case lb.BinaryXor:
		raw = builder.module.builder.CreateXor(left.value.raw, right.value.raw, "")
	case lb.BinaryShiftLeft:
		raw = builder.module.builder.CreateShl(left.value.raw, right.value.raw, "")
	case lb.BinaryLogicalShiftRight:
		raw = builder.module.builder.CreateLShr(left.value.raw, right.value.raw, "")
	case lb.BinaryArithmeticShiftRight:
		raw = builder.module.builder.CreateAShr(left.value.raw, right.value.raw, "")
	default:
		return 0, fmt.Errorf("unknown binary operation %d", operation)
	}
	return b.remember(Value{raw: raw, owner: b.module}, functionID, left.typeID), nil
}

func (b *LoweringBackend) Compare(blockID lb.BlockID, operation lb.CompareOp, leftID, rightID lb.ValueID) (lb.ValueID, error) {
	builder, functionID, err := b.openBuilder(blockID)
	if err != nil {
		return 0, err
	}
	left, err := b.valueForFunction(leftID, functionID, "comparison left operand")
	if err != nil {
		return 0, err
	}
	right, err := b.valueForFunction(rightID, functionID, "comparison right operand")
	if err != nil {
		return 0, err
	}
	if left.typeID != right.typeID {
		return 0, fmt.Errorf("comparison operands have different types")
	}
	typeSpec, _ := b.registry.Type(left.typeID)
	var raw llvm.Value
	if operation >= lb.CompareFloatOrderedEqual {
		if typeSpec.Kind != lb.TypeFloat {
			return 0, fmt.Errorf("floating comparison requires floating operands")
		}
		predicates := map[lb.CompareOp]llvm.FloatPredicate{lb.CompareFloatOrderedEqual: llvm.FloatOEQ, lb.CompareFloatOrderedNotEqual: llvm.FloatONE, lb.CompareFloatOrderedGreater: llvm.FloatOGT, lb.CompareFloatOrderedGreaterEqual: llvm.FloatOGE, lb.CompareFloatOrderedLess: llvm.FloatOLT, lb.CompareFloatOrderedLessEqual: llvm.FloatOLE, lb.CompareFloatUnorderedEqual: llvm.FloatUEQ, lb.CompareFloatUnorderedNotEqual: llvm.FloatUNE}
		predicate, ok := predicates[operation]
		if !ok {
			return 0, fmt.Errorf("unknown floating comparison %d", operation)
		}
		raw = builder.module.builder.CreateFCmp(predicate, left.value.raw, right.value.raw, "")
	} else {
		if typeSpec.Kind != lb.TypeInteger && typeSpec.Kind != lb.TypePointer {
			return 0, fmt.Errorf("integer comparison requires integer or pointer operands")
		}
		if typeSpec.Kind == lb.TypePointer && operation != lb.CompareEqual && operation != lb.CompareNotEqual {
			return 0, fmt.Errorf("pointer comparison only supports equality")
		}
		predicates := map[lb.CompareOp]llvm.IntPredicate{lb.CompareEqual: llvm.IntEQ, lb.CompareNotEqual: llvm.IntNE, lb.CompareUnsignedGreater: llvm.IntUGT, lb.CompareUnsignedGreaterEqual: llvm.IntUGE, lb.CompareUnsignedLess: llvm.IntULT, lb.CompareUnsignedLessEqual: llvm.IntULE, lb.CompareSignedGreater: llvm.IntSGT, lb.CompareSignedGreaterEqual: llvm.IntSGE, lb.CompareSignedLess: llvm.IntSLT, lb.CompareSignedLessEqual: llvm.IntSLE}
		predicate, ok := predicates[operation]
		if !ok {
			return 0, fmt.Errorf("unknown integer comparison %d", operation)
		}
		raw = builder.module.builder.CreateICmp(predicate, left.value.raw, right.value.raw, "")
	}
	i1, err := b.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 1})
	if err != nil {
		return 0, err
	}
	return b.remember(Value{raw: raw, owner: b.module}, functionID, i1), nil
}

func (b *LoweringBackend) Cast(blockID lb.BlockID, operation lb.CastOp, valueID lb.ValueID, resultID lb.TypeID) (lb.ValueID, error) {
	builder, functionID, err := b.openBuilder(blockID)
	if err != nil {
		return 0, err
	}
	value, err := b.valueForFunction(valueID, functionID, "cast operand")
	if err != nil {
		return 0, err
	}
	result, ok := b.types[resultID]
	if !ok {
		return 0, fmt.Errorf("cast result type %d is not materialized", resultID)
	}
	sourceSpec, _ := b.registry.Type(value.typeID)
	resultSpec, _ := b.registry.Type(resultID)
	valid := false
	switch operation {
	case lb.CastTruncate:
		valid = sourceSpec.Kind == lb.TypeInteger && resultSpec.Kind == lb.TypeInteger && sourceSpec.Bits > resultSpec.Bits
	case lb.CastZeroExtend, lb.CastSignExtend:
		valid = sourceSpec.Kind == lb.TypeInteger && resultSpec.Kind == lb.TypeInteger && sourceSpec.Bits < resultSpec.Bits
	case lb.CastUnsignedIntToFloat, lb.CastSignedIntToFloat:
		valid = sourceSpec.Kind == lb.TypeInteger && resultSpec.Kind == lb.TypeFloat
	case lb.CastFloatToUnsignedInt, lb.CastFloatToSignedInt:
		valid = sourceSpec.Kind == lb.TypeFloat && resultSpec.Kind == lb.TypeInteger
	case lb.CastFloatTruncate:
		valid = sourceSpec.Kind == lb.TypeFloat && resultSpec.Kind == lb.TypeFloat && sourceSpec.Bits > resultSpec.Bits
	case lb.CastFloatExtend:
		valid = sourceSpec.Kind == lb.TypeFloat && resultSpec.Kind == lb.TypeFloat && sourceSpec.Bits < resultSpec.Bits
	case lb.CastBit:
		valid = sourceSpec.Kind == lb.TypePointer && resultSpec.Kind == lb.TypePointer && sourceSpec.AddressSpace == resultSpec.AddressSpace || (sourceSpec.Kind == lb.TypeInteger || sourceSpec.Kind == lb.TypeFloat) && (resultSpec.Kind == lb.TypeInteger || resultSpec.Kind == lb.TypeFloat) && sourceSpec.Bits == resultSpec.Bits
	default:
		return 0, fmt.Errorf("unknown cast operation %d", operation)
	}
	if !valid {
		return 0, fmt.Errorf("cast operation %d is invalid from type %d to %d", operation, value.typeID, resultID)
	}
	var raw llvm.Value
	switch operation {
	case lb.CastBit:
		raw = builder.module.builder.CreateBitCast(value.value.raw, result.raw, "")
	case lb.CastTruncate:
		raw = builder.module.builder.CreateTrunc(value.value.raw, result.raw, "")
	case lb.CastZeroExtend:
		raw = builder.module.builder.CreateZExt(value.value.raw, result.raw, "")
	case lb.CastSignExtend:
		raw = builder.module.builder.CreateSExt(value.value.raw, result.raw, "")
	case lb.CastUnsignedIntToFloat:
		raw = builder.module.builder.CreateUIToFP(value.value.raw, result.raw, "")
	case lb.CastSignedIntToFloat:
		raw = builder.module.builder.CreateSIToFP(value.value.raw, result.raw, "")
	case lb.CastFloatToUnsignedInt:
		raw = builder.module.builder.CreateFPToUI(value.value.raw, result.raw, "")
	case lb.CastFloatToSignedInt:
		raw = builder.module.builder.CreateFPToSI(value.value.raw, result.raw, "")
	case lb.CastFloatTruncate:
		raw = builder.module.builder.CreateFPTrunc(value.value.raw, result.raw, "")
	case lb.CastFloatExtend:
		raw = builder.module.builder.CreateFPExt(value.value.raw, result.raw, "")
	}
	return b.remember(Value{raw: raw, owner: b.module}, functionID, resultID), nil
}

func (b *LoweringBackend) Select(blockID lb.BlockID, conditionID, thenID, elseID lb.ValueID) (lb.ValueID, error) {
	builder, functionID, err := b.openBuilder(blockID)
	if err != nil {
		return 0, err
	}
	condition, err := b.valueForFunction(conditionID, functionID, "select condition")
	if err != nil {
		return 0, err
	}
	conditionType, _ := b.registry.Type(condition.typeID)
	if conditionType.Kind != lb.TypeInteger || conditionType.Bits != 1 {
		return 0, fmt.Errorf("select condition must be i1")
	}
	thenValue, err := b.valueForFunction(thenID, functionID, "select true value")
	if err != nil {
		return 0, err
	}
	elseValue, err := b.valueForFunction(elseID, functionID, "select false value")
	if err != nil {
		return 0, err
	}
	if thenValue.typeID != elseValue.typeID {
		return 0, fmt.Errorf("select result operands have different types")
	}
	typeSpec, _ := b.registry.Type(thenValue.typeID)
	if typeSpec.Kind == lb.TypeVoid || typeSpec.Kind == lb.TypeFunction {
		return 0, fmt.Errorf("select result type is not a first-class value")
	}
	raw := builder.module.builder.CreateSelect(condition.value.raw, thenValue.value.raw, elseValue.value.raw, "")
	return b.remember(Value{raw: raw, owner: b.module}, functionID, thenValue.typeID), nil
}

func (b *LoweringBackend) Call(blockID lb.BlockID, calleeID lb.FunctionID, argumentIDs []lb.ValueID) (lb.ValueID, error) {
	builder, functionID, err := b.openBuilder(blockID)
	if err != nil {
		return 0, err
	}
	callee, ok := b.functions[calleeID]
	if !ok {
		return 0, fmt.Errorf("unknown callee function %d", calleeID)
	}
	spec, err := b.registry.Function(calleeID)
	if err != nil {
		return 0, err
	}
	if len(argumentIDs) < len(spec.Parameters) || !spec.Variadic && len(argumentIDs) != len(spec.Parameters) {
		return 0, fmt.Errorf("call to %q has %d arguments; expected %d", spec.Symbol, len(argumentIDs), len(spec.Parameters))
	}
	arguments := make([]llvm.Value, len(argumentIDs))
	for i, id := range argumentIDs {
		argument, err := b.valueForFunction(id, functionID, "call argument")
		if err != nil {
			return 0, err
		}
		if i < len(spec.Parameters) && argument.typeID != spec.Parameters[i] {
			return 0, fmt.Errorf("call to %q argument %d has the wrong type (got %d, expected %d)", spec.Symbol, i, argument.typeID, spec.Parameters[i])
		}
		argumentType, _ := b.registry.Type(argument.typeID)
		if argumentType.Kind == lb.TypeVoid || argumentType.Kind == lb.TypeFunction {
			return 0, fmt.Errorf("call argument %d is not a first-class value", i)
		}
		arguments[i] = argument.value.raw
	}
	result := b.types[spec.Result]
	parameters, err := b.materializedTypes(spec.Parameters)
	if err != nil {
		return 0, err
	}
	functionType, err := b.module.functionType(result, parameters, spec.Variadic)
	if err != nil {
		return 0, err
	}
	raw := builder.module.builder.CreateCall(functionType.raw, callee.value.raw, arguments, "")
	call := Value{raw: raw, owner: b.module}
	if err := b.applyCallABI(call, spec); err != nil {
		return 0, err
	}
	return b.remember(call, functionID, spec.Result), nil
}

func (b *LoweringBackend) IndirectCall(blockID lb.BlockID, functionTypeID lb.TypeID, calleeID lb.ValueID, argumentIDs []lb.ValueID, callSpec lb.CallSpec) (lb.ValueID, error) {
	builder, functionID, err := b.openBuilder(blockID)
	if err != nil {
		return 0, err
	}
	functionType, err := b.registry.FunctionType(functionTypeID)
	if err != nil {
		return 0, err
	}
	objectType, ok := b.types[functionTypeID]
	if !ok {
		return 0, fmt.Errorf("function type %d is not materialized", functionTypeID)
	}
	callee, err := b.valueForFunction(calleeID, functionID, "indirect callee")
	if err != nil {
		return 0, err
	}
	if err := b.requirePointer(callee.typeID, "indirect call"); err != nil {
		return 0, err
	}
	if callee.pointee != 0 && callee.pointee != functionTypeID {
		return 0, fmt.Errorf("indirect callee signature does not match requested function type")
	}
	convention, attributes, err := b.registry.NormalizeCallABI(callSpec.CallingConvention, callSpec.Attributes, len(functionType.Parameters), functionType.Variadic)
	if err != nil {
		return 0, fmt.Errorf("indirect call ABI: %w", err)
	}
	if len(argumentIDs) < len(functionType.Parameters) || !functionType.Variadic && len(argumentIDs) != len(functionType.Parameters) {
		return 0, fmt.Errorf("indirect call has %d arguments; expected %d", len(argumentIDs), len(functionType.Parameters))
	}
	arguments := make([]llvm.Value, len(argumentIDs))
	for i, id := range argumentIDs {
		argument, err := b.valueForFunction(id, functionID, "indirect call argument")
		if err != nil {
			return 0, err
		}
		if i < len(functionType.Parameters) && argument.typeID != functionType.Parameters[i] {
			return 0, fmt.Errorf("indirect call argument %d has the wrong type", i)
		}
		argumentType, _ := b.registry.Type(argument.typeID)
		if argumentType.Kind == lb.TypeVoid || argumentType.Kind == lb.TypeFunction {
			return 0, fmt.Errorf("indirect call argument %d is not a first-class value", i)
		}
		arguments[i] = argument.value.raw
	}
	raw := builder.module.builder.CreateCall(objectType.raw, callee.value.raw, arguments, "")
	call := Value{raw: raw, owner: b.module}
	if err := b.applyCallABI(call, lb.FunctionSpec{CallingConvention: convention, Attributes: attributes}); err != nil {
		return 0, err
	}
	return b.remember(call, functionID, functionType.Result), nil
}

func (b *LoweringBackend) applyCallABI(call Value, spec lb.FunctionSpec) error {
	convention, err := objectCallingConvention(spec.CallingConvention)
	if err != nil {
		return err
	}
	call.raw.SetInstructionCallConv(convention)
	for _, spec := range spec.Attributes {
		attribute, err := b.objectAttribute(spec)
		if err != nil {
			return err
		}
		index := -1
		if spec.Placement == lb.AttributeReturn {
			index = 0
		} else if spec.Placement == lb.AttributeParameter {
			index = int(spec.Parameter) + 1
		}
		call.raw.AddCallSiteAttribute(index, attribute)
	}
	return nil
}

func (b *LoweringBackend) valueForFunction(id lb.ValueID, function lb.FunctionID, role string) (neutralValue, error) {
	value, ok := b.values[id]
	if !ok {
		return neutralValue{}, fmt.Errorf("unknown %s value %d", role, id)
	}
	if value.function != 0 && value.function != function {
		return neutralValue{}, fmt.Errorf("%s value %d belongs to a different function", role, id)
	}
	return value, nil
}

func (b *LoweringBackend) requirePointer(id lb.TypeID, operation string) error {
	spec, err := b.registry.Type(id)
	if err != nil {
		return err
	}
	if spec.Kind != lb.TypePointer {
		return fmt.Errorf("%s requires a pointer operand", operation)
	}
	return nil
}

func (b *LoweringBackend) PtrToInt(blockID lb.BlockID, valueID lb.ValueID, resultID lb.TypeID) (lb.ValueID, error) {
	return b.conversion(blockID, valueID, resultID, "ptrtoint")
}

func (b *LoweringBackend) IntToPtr(blockID lb.BlockID, valueID lb.ValueID, resultID lb.TypeID) (lb.ValueID, error) {
	return b.conversion(blockID, valueID, resultID, "inttoptr")
}

func (b *LoweringBackend) conversion(blockID lb.BlockID, valueID lb.ValueID, resultID lb.TypeID, operation string) (lb.ValueID, error) {
	builder, functionID, err := b.openBuilder(blockID)
	if err != nil {
		return 0, err
	}
	value, ok := b.values[valueID]
	if !ok || (value.function != 0 && value.function != functionID) {
		return 0, fmt.Errorf("value %d does not belong to block function", valueID)
	}
	result, ok := b.types[resultID]
	if !ok {
		return 0, fmt.Errorf("unknown object result type %d", resultID)
	}
	var converted Value
	if operation == "ptrtoint" {
		converted, err = builder.PtrToInt(value.value, result, "")
	} else {
		converted, err = builder.IntToPtr(value.value, result, "")
	}
	if err != nil {
		return 0, err
	}
	return b.remember(converted, functionID, resultID), nil
}

func (b *LoweringBackend) Phi(blockID lb.BlockID, typeID lb.TypeID) (lb.ValueID, error) {
	builder, functionID, err := b.openBuilder(blockID)
	if err != nil {
		return 0, err
	}
	typeValue, ok := b.types[typeID]
	if !ok {
		return 0, fmt.Errorf("phi type %d is not materialized", typeID)
	}
	raw := Value{raw: builder.module.builder.CreatePHI(typeValue.raw, ""), owner: b.module}
	id := b.remember(raw, functionID, typeID)
	b.phis[id] = &neutralPhi{block: blockID, incoming: make(map[lb.BlockID]struct{})}
	return id, nil
}

func (b *LoweringBackend) AddPhiIncoming(phiID lb.ValueID, valueIDs []lb.ValueID, blocks []lb.BlockID) error {
	phi, ok := b.phis[phiID]
	if !ok {
		return fmt.Errorf("value %d is not a phi", phiID)
	}
	if len(valueIDs) == 0 || len(valueIDs) != len(blocks) {
		return fmt.Errorf("phi incoming values and blocks must have equal non-zero length")
	}
	phiValue := b.values[phiID]
	rawValues := make([]llvm.Value, len(valueIDs))
	rawBlocks := make([]llvm.BasicBlock, len(blocks))
	for i, blockID := range blocks {
		if !b.cfg.IsPredecessor(phi.block, blockID) {
			return fmt.Errorf("block %d is not a predecessor of phi block", blockID)
		}
		if _, duplicate := phi.incoming[blockID]; duplicate {
			return fmt.Errorf("phi already has incoming value for block %d", blockID)
		}
		value, ok := b.values[valueIDs[i]]
		if !ok || value.typeID != phiValue.typeID || (value.function != 0 && value.function != phiValue.function) {
			return fmt.Errorf("phi incoming value %d has wrong type or function", valueIDs[i])
		}
		rawValues[i] = value.value.raw
		rawBlocks[i] = b.blocks[blockID].raw
	}
	phiValue.value.raw.AddIncoming(rawValues, rawBlocks)
	for _, blockID := range blocks {
		phi.incoming[blockID] = struct{}{}
	}
	return nil
}

func (b *LoweringBackend) AttachSource(valueID lb.ValueID, location lb.SourceLocation) error {
	value, ok := b.values[valueID]
	if !ok {
		return fmt.Errorf("unknown value %d", valueID)
	}
	if value.value.raw.IsAInstruction().IsNil() {
		return fmt.Errorf("source metadata can only be attached to instructions")
	}
	if location.File == "" || location.Line == 0 {
		return fmt.Errorf("source location requires a file and non-zero line")
	}
	text := fmt.Sprintf("%s:%d:%d", location.File, location.Line, location.Column)
	node := b.module.ctx.MDNode([]llvm.Metadata{b.module.ctx.MDString(text)})
	value.value.raw.SetMetadata(b.module.ctx.MDKindID("magma.source"), node)
	return nil
}

func (b *LoweringBackend) Branch(from, target lb.BlockID) error { return b.branch(from, 0, target) }
func (b *LoweringBackend) CondBranch(from lb.BlockID, condition lb.ValueID, thenBlock, elseBlock lb.BlockID) error {
	return b.condBranch(from, condition, thenBlock, elseBlock, nil)
}
func (b *LoweringBackend) CondBranchWeighted(from lb.BlockID, condition lb.ValueID, thenBlock, elseBlock lb.BlockID, weights lb.BranchWeights) error {
	return b.condBranch(from, condition, thenBlock, elseBlock, &weights)
}
func (b *LoweringBackend) condBranch(from lb.BlockID, condition lb.ValueID, thenBlock, elseBlock lb.BlockID, weights *lb.BranchWeights) error {
	if err := b.cfg.ValidateTargets(from, thenBlock, elseBlock); err != nil {
		return err
	}
	builder, functionID, err := b.openBuilder(from)
	if err != nil {
		return err
	}
	value, ok := b.values[condition]
	if !ok || (value.function != 0 && value.function != functionID) {
		return fmt.Errorf("branch condition %d does not belong to block function", condition)
	}
	typeSpec, _ := b.registry.Type(value.typeID)
	if typeSpec.Kind != lb.TypeInteger || typeSpec.Bits != 1 {
		return fmt.Errorf("conditional branch requires an i1 condition")
	}
	branch := builder.module.builder.CreateCondBr(value.value.raw, b.blocks[thenBlock].raw, b.blocks[elseBlock].raw)
	if weights != nil {
		i32 := b.module.ctx.Int32Type()
		metadata := b.module.ctx.MDNode([]llvm.Metadata{
			b.module.ctx.MDString("branch_weights"),
			llvm.ConstInt(i32, uint64(weights.Then), false).ConstantAsMetadata(),
			llvm.ConstInt(i32, uint64(weights.Else), false).ConstantAsMetadata(),
		})
		branch.SetMetadata(b.module.ctx.MDKindID("prof"), metadata)
	}
	return b.cfg.TerminateWithTargets(from, thenBlock, elseBlock)
}
func (b *LoweringBackend) branch(from lb.BlockID, condition lb.ValueID, targets ...lb.BlockID) error {
	if err := b.cfg.ValidateTargets(from, targets...); err != nil {
		return err
	}
	builder, functionID, err := b.openBuilder(from)
	if err != nil {
		return err
	}
	if len(targets) == 1 {
		builder.module.builder.CreateBr(b.blocks[targets[0]].raw)
	} else {
		value, ok := b.values[condition]
		if !ok || (value.function != 0 && value.function != functionID) {
			return fmt.Errorf("branch condition %d does not belong to block function", condition)
		}
		typeSpec, _ := b.registry.Type(value.typeID)
		if typeSpec.Kind != lb.TypeInteger || typeSpec.Bits != 1 {
			return fmt.Errorf("conditional branch requires an i1 condition")
		}
		builder.module.builder.CreateCondBr(value.value.raw, b.blocks[targets[0]].raw, b.blocks[targets[1]].raw)
	}
	return b.cfg.TerminateWithTargets(from, targets...)
}

func (b *LoweringBackend) Unreachable(blockID lb.BlockID) error {
	builder, _, err := b.openBuilder(blockID)
	if err != nil {
		return err
	}
	builder.module.builder.CreateUnreachable()
	return b.cfg.Terminate(blockID)
}

func (b *LoweringBackend) Return(blockID lb.BlockID, valueID lb.ValueID) error {
	builder, functionID, err := b.openBuilder(blockID)
	if err != nil {
		return err
	}
	value, ok := b.values[valueID]
	if !ok || (value.function != 0 && value.function != functionID) {
		return fmt.Errorf("return value %d does not belong to block function", valueID)
	}
	if err := builder.Ret(value.value); err != nil {
		return err
	}
	return b.cfg.Terminate(blockID)
}

func (b *LoweringBackend) ReturnVoid(blockID lb.BlockID) error {
	builder, _, err := b.openBuilder(blockID)
	if err != nil {
		return err
	}
	if err := builder.RetVoid(); err != nil {
		return err
	}
	return b.cfg.Terminate(blockID)
}

func (b *LoweringBackend) FinalizeFunction(function lb.FunctionID) error {
	if err := b.cfg.Finalize(function); err != nil {
		return err
	}
	for _, phi := range b.phis {
		owner, _ := b.cfg.Function(phi.block)
		if owner == function && len(phi.incoming) != b.cfg.PredecessorCount(phi.block) {
			return fmt.Errorf("phi in block %d has %d incoming values for %d predecessors", phi.block, len(phi.incoming), b.cfg.PredecessorCount(phi.block))
		}
	}
	return nil
}
func (b *LoweringBackend) Verify() error { return b.module.Verify() }

func (b *LoweringBackend) openBuilder(blockID lb.BlockID) (*Builder, lb.FunctionID, error) {
	if err := b.cfg.RequireOpen(blockID); err != nil {
		return nil, 0, err
	}
	block, ok := b.blocks[blockID]
	if !ok {
		return nil, 0, fmt.Errorf("unknown object block %d", blockID)
	}
	function, err := b.cfg.Function(blockID)
	if err != nil {
		return nil, 0, err
	}
	builder, err := b.module.BuilderAt(block)
	return builder, function, err
}

func (b *LoweringBackend) remember(value Value, function lb.FunctionID, typeID lb.TypeID) lb.ValueID {
	b.nextValue++
	b.values[b.nextValue] = neutralValue{value: value, function: function, typeID: typeID}
	return b.nextValue
}
