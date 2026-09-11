// Package loweringbackend defines the backend-neutral object-lowering
// contract. It deliberately contains no Magma AST or semantic type nodes.
package loweringbackend

import "fmt"

type TypeID uint32
type ConstantID uint32
type GlobalID uint32
type ValueID uint32
type FunctionID uint32
type BlockID uint32

type TypeKind uint8

const (
	TypeVoid TypeKind = iota + 1
	TypeInteger
	TypeFloat
	TypePointer
	TypeArray
	TypeVector
	TypeStruct
	TypeFunction
)

// TypeSpec is structural and comparable so a context can intern it directly.
// Named recursive aggregates will use the separate opaque-type API added with
// aggregate lowering rather than embedding source-level declarations here.
type TypeSpec struct {
	Kind         TypeKind
	Bits         uint32
	AddressSpace uint32
	Element      TypeID
	Length       uint64
}

func (s TypeSpec) Validate() error {
	switch s.Kind {
	case TypeVoid:
		if s.Bits != 0 || s.AddressSpace != 0 || s.Element != 0 || s.Length != 0 {
			return fmt.Errorf("void type has unexpected parameters")
		}
	case TypeInteger:
		if s.Bits == 0 || s.Bits >= 1<<23 {
			return fmt.Errorf("integer width %d is outside [1, %d]", s.Bits, (1<<23)-1)
		}
	case TypeFloat:
		if s.Bits != 16 && s.Bits != 32 && s.Bits != 64 && s.Bits != 128 {
			return fmt.Errorf("unsupported floating-point width %d", s.Bits)
		}
	case TypePointer:
		if s.Bits != 0 || s.Element != 0 || s.Length != 0 {
			return fmt.Errorf("opaque pointer type has unexpected parameters")
		}
	case TypeArray, TypeVector:
		if s.Element == 0 {
			return fmt.Errorf("aggregate type has no element type")
		}
		if s.Kind == TypeVector && s.Length == 0 {
			return fmt.Errorf("vector type has no elements")
		}
	case TypeStruct, TypeFunction:
		return fmt.Errorf("compound types must be created through their dedicated builder")
	default:
		return fmt.Errorf("unknown backend type kind %d", s.Kind)
	}
	return nil
}

type ModuleSpec struct {
	SourceFile   string
	TargetTriple string
	DataLayout   string
}

type TypeLayout struct {
	SizeBits           uint64
	StoreSize          uint64
	AllocationSize     uint64
	ABIAlignment       uint32
	PreferredAlignment uint32
}

type StructSpec struct {
	Name     string
	Elements []TypeID
	Packed   bool
	Opaque   bool
}

type FunctionTypeSpec struct {
	Result     TypeID
	Parameters []TypeID
	Variadic   bool
}

type SourceLocation struct {
	File   string
	Line   uint32
	Column uint32
}

// BranchWeights are relative execution frequencies for a conditional branch.
// Then applies to the true successor and Else to the false successor.
type BranchWeights struct {
	Then uint32
	Else uint32
}

var UnlikelyThen = BranchWeights{Then: 1, Else: 2000}
var UnlikelyElse = BranchWeights{Then: 2000, Else: 1}

type BinaryOp uint8

const (
	BinaryAdd BinaryOp = iota + 1
	BinarySub
	BinaryMul
	BinaryUnsignedDiv
	BinarySignedDiv
	BinaryFloatDiv
	BinaryUnsignedRem
	BinarySignedRem
	BinaryFloatRem
	BinaryAnd
	BinaryOr
	BinaryXor
	BinaryShiftLeft
	BinaryLogicalShiftRight
	BinaryArithmeticShiftRight
)

type CompareOp uint8

const (
	CompareEqual CompareOp = iota + 1
	CompareNotEqual
	CompareUnsignedGreater
	CompareUnsignedGreaterEqual
	CompareUnsignedLess
	CompareUnsignedLessEqual
	CompareSignedGreater
	CompareSignedGreaterEqual
	CompareSignedLess
	CompareSignedLessEqual
	CompareFloatOrderedEqual
	CompareFloatOrderedNotEqual
	CompareFloatOrderedGreater
	CompareFloatOrderedGreaterEqual
	CompareFloatOrderedLess
	CompareFloatOrderedLessEqual
	CompareFloatUnorderedEqual
	CompareFloatUnorderedNotEqual
)

type CastOp uint8

const (
	CastBit CastOp = iota + 1
	CastTruncate
	CastZeroExtend
	CastSignExtend
	CastUnsignedIntToFloat
	CastSignedIntToFloat
	CastFloatToUnsignedInt
	CastFloatToSignedInt
	CastFloatTruncate
	CastFloatExtend
)

type AtomicOrdering uint8

const (
	AtomicMonotonic AtomicOrdering = iota + 1
	AtomicAcquire
	AtomicRelease
	AtomicAcquireRelease
	AtomicSequentiallyConsistent
)

type AtomicRMWOp uint8

const (
	AtomicRMWExchange AtomicRMWOp = iota + 1
	AtomicRMWAdd
	AtomicRMWSub
)

type Linkage uint8

const (
	LinkageInternal Linkage = iota + 1
	LinkageExternal
	LinkagePrivate
)

type Visibility uint8

const (
	VisibilityDefault Visibility = iota + 1
	VisibilityHidden
	VisibilityProtected
)

type CallingConvention uint8

const (
	CallingConventionC CallingConvention = iota + 1
	CallingConventionFast
	CallingConventionCold
)

type AttributePlacement uint8

const (
	AttributeFunction AttributePlacement = iota + 1
	AttributeReturn
	AttributeParameter
)

type AttributeKind uint8

const (
	AttributeNoReturn AttributeKind = iota + 1
	AttributeNoUnwind
	AttributeReadOnly
	AttributeAlwaysInline
	AttributeNoInline
	AttributeCold
	AttributeZeroExtend
	AttributeSignExtend
	AttributeNonNull
	AttributeNoAlias
	AttributeAlignment
	AttributeDereferenceable
	AttributeStructReturn
	AttributeByValue
)

type AttributeSpec struct {
	Kind      AttributeKind
	Placement AttributePlacement
	Parameter uint32
	Value     uint64
	Type      TypeID
}

type CallSpec struct {
	CallingConvention CallingConvention
	Attributes        []AttributeSpec
}

type ConstantKind uint8

const (
	ConstantInteger ConstantKind = iota + 1
	ConstantNull
	ConstantZero
	ConstantFloat
	ConstantUndef
	ConstantAggregate
	ConstantString
	ConstantGlobalAddress
	ConstantFunctionAddress
)

// Integer is an unsigned decimal bit pattern. Keeping it as text prevents
// arbitrary-width constants from narrowing through a Go integer.
type ConstantSpec struct {
	Kind           ConstantKind
	Type           TypeID
	Integer        string
	Float          string
	Elements       []ConstantID
	Global         GlobalID
	Function       FunctionID
	Bytes          string
	NullTerminated bool
}

type GlobalSpec struct {
	Symbol         string
	Type           TypeID
	Initializer    ConstantID
	Linkage        Linkage
	Visibility     Visibility
	AddressSpace   uint32
	Alignment      uint32
	Section        string
	ThreadLocal    bool
	Constant       bool
	UnnamedAddress bool
	Definition     bool
}

type FunctionSpec struct {
	Symbol            string
	Result            TypeID
	Parameters        []TypeID
	Variadic          bool
	Linkage           Linkage
	CallingConvention CallingConvention
	Attributes        []AttributeSpec
	Definition        bool
}

// Backend is the first scalar/function subset of the object lowering
// boundary. New language features extend this contract using opaque IDs,
// never library handles or AST nodes.
type Backend interface {
	ConfigureModule(ModuleSpec) error
	TypeLayout(TypeID) (TypeLayout, error)
	StructFieldOffset(TypeID, uint32) (uint64, error)
	InternType(TypeSpec) (TypeID, error)
	InternStruct(StructSpec) (TypeID, error)
	DefineStruct(TypeID, []TypeID, bool) error
	InternFunctionType(FunctionTypeSpec) (TypeID, error)
	InternConstant(ConstantSpec) (ConstantID, error)
	DeclareGlobal(GlobalSpec) (GlobalID, error)
	DeclareFunction(FunctionSpec) (FunctionID, error)
	AppendBlock(FunctionID, string) (BlockID, error)
	Parameter(FunctionID, int) (ValueID, error)
	ConstantValue(ConstantID) (ValueID, error)
	GlobalAddress(GlobalID) (ValueID, error)
	FunctionAddress(FunctionID) (ValueID, error)
	ReinterpretPointer(ValueID) (ValueID, error)
	Alloca(BlockID, TypeID, uint32) (ValueID, error)
	StaticAlloca(FunctionID, TypeID, uint32) (ValueID, error)
	DynamicAlloca(BlockID, TypeID, ValueID, uint32) (ValueID, error)
	Load(BlockID, TypeID, ValueID, uint32, bool) (ValueID, error)
	Store(BlockID, ValueID, ValueID, uint32, bool) (ValueID, error)
	AtomicLoad(BlockID, TypeID, ValueID, AtomicOrdering, uint32) (ValueID, error)
	AtomicStore(BlockID, ValueID, ValueID, AtomicOrdering, uint32) (ValueID, error)
	AtomicRMW(BlockID, AtomicRMWOp, ValueID, ValueID, AtomicOrdering, uint32) (ValueID, error)
	CompareExchangeOld(BlockID, ValueID, ValueID, ValueID, AtomicOrdering, AtomicOrdering, uint32) (ValueID, error)
	InlineAssemblySideEffect(BlockID, string) (ValueID, error)
	GEP(BlockID, TypeID, ValueID, []ValueID, bool) (ValueID, error)
	BuildAggregate(BlockID, TypeID, []ValueID) (ValueID, error)
	ExtractValue(BlockID, ValueID, []uint32) (ValueID, error)
	InsertValue(BlockID, ValueID, ValueID, []uint32) (ValueID, error)
	StructFieldAddress(BlockID, TypeID, ValueID, uint32) (ValueID, error)
	Binary(BlockID, BinaryOp, ValueID, ValueID) (ValueID, error)
	Compare(BlockID, CompareOp, ValueID, ValueID) (ValueID, error)
	Cast(BlockID, CastOp, ValueID, TypeID) (ValueID, error)
	Select(BlockID, ValueID, ValueID, ValueID) (ValueID, error)
	Call(BlockID, FunctionID, []ValueID) (ValueID, error)
	IndirectCall(BlockID, TypeID, ValueID, []ValueID, CallSpec) (ValueID, error)
	PtrToInt(BlockID, ValueID, TypeID) (ValueID, error)
	IntToPtr(BlockID, ValueID, TypeID) (ValueID, error)
	Phi(BlockID, TypeID) (ValueID, error)
	AddPhiIncoming(ValueID, []ValueID, []BlockID) error
	AttachSource(ValueID, SourceLocation) error
	Branch(BlockID, BlockID) error
	CondBranch(BlockID, ValueID, BlockID, BlockID) error
	CondBranchWeighted(BlockID, ValueID, BlockID, BlockID, BranchWeights) error
	Unreachable(BlockID) error
	Return(BlockID, ValueID) error
	ReturnVoid(BlockID) error
	FinalizeFunction(FunctionID) error
	Verify() error
}
