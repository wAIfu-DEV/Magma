//go:build llvm_object

package llvmobject

import (
	lb "Magma/src/lowering_backend"
	"strings"
	"testing"
)

func TestNeutralBackendBuildsPtrToInt(t *testing.T) {
	b, err := NewLoweringBackend("neutral.ptrtoint")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	ptr, _ := b.InternType(lb.TypeSpec{Kind: lb.TypePointer})
	i64, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 64})
	fn, err := b.DeclareFunction(lb.FunctionSpec{
		Symbol: "ptrToInt", Result: i64, Parameters: []lb.TypeID{ptr},
		Linkage: lb.LinkageInternal, Definition: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	entry, _ := b.AppendBlock(fn, "entry")
	parameter, _ := b.Parameter(fn, 0)
	result, err := b.PtrToInt(entry, parameter, i64)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Return(entry, result); err != nil {
		t.Fatal(err)
	}
	if err := b.FinalizeFunction(fn); err != nil {
		t.Fatal(err)
	}
	if err := b.Verify(); err != nil {
		t.Fatal(err)
	}
	ir, _ := b.module.String()
	if !strings.Contains(ir, "ptrtoint ptr %0 to i64") {
		t.Fatalf("missing conversion:\n%s", ir)
	}
}

func TestNeutralBackendBuildsFixedVectorType(t *testing.T) {
	b, err := NewLoweringBackend("neutral.vector")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if err := b.ConfigureModule(lb.ModuleSpec{DataLayout: "e-p:64:64"}); err != nil {
		t.Fatal(err)
	}
	f32, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeFloat, Bits: 32})
	vector, err := b.InternType(lb.TypeSpec{Kind: lb.TypeVector, Element: f32, Length: 2})
	if err != nil {
		t.Fatal(err)
	}
	layout, err := b.TypeLayout(vector)
	if err != nil {
		t.Fatal(err)
	}
	if layout.StoreSize != 8 {
		t.Fatalf("<2 x float> store size = %d, want 8", layout.StoreSize)
	}
}

func TestNeutralBackendRejectsInstructionAfterTerminator(t *testing.T) {
	b, _ := NewLoweringBackend("neutral.terminator")
	defer b.Close()
	void, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeVoid})
	fn, _ := b.DeclareFunction(lb.FunctionSpec{Symbol: "done", Result: void, Linkage: lb.LinkageInternal, Definition: true})
	entry, _ := b.AppendBlock(fn, "entry")
	if err := b.ReturnVoid(entry); err != nil {
		t.Fatal(err)
	}
	if err := b.ReturnVoid(entry); err == nil || !strings.Contains(err.Error(), "already has a terminator") {
		t.Fatalf("second terminator error = %v", err)
	}
}

func TestNeutralBackendEmitsCanonicalGlobal(t *testing.T) {
	b, _ := NewLoweringBackend("neutral.global")
	defer b.Close()
	i128, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 128})
	initial, err := b.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: i128, Integer: "340282366920938463463374607431768211455"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.DeclareGlobal(lb.GlobalSpec{
		Symbol: "maximum", Type: i128, Initializer: initial, Definition: true,
		Linkage: lb.LinkageInternal, Visibility: lb.VisibilityHidden,
		Constant: true, UnnamedAddress: true, Alignment: 16, Section: ".magma.constants",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Verify(); err != nil {
		t.Fatal(err)
	}
	ir, _ := b.module.String()
	for _, want := range []string{"@maximum = internal hidden unnamed_addr constant i128 -1", "section \".magma.constants\"", "align 16"} {
		if !strings.Contains(ir, want) {
			t.Errorf("global IR missing %q:\n%s", want, ir)
		}
	}
}

func TestNeutralBackendBuildsCompoundTypesAndMetadata(t *testing.T) {
	b, err := NewLoweringBackend("types.mg")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if err := b.ConfigureModule(lb.ModuleSpec{SourceFile: "types.mg", TargetTriple: "x86_64-unknown-linux-gnu", DataLayout: "e-p:64:64"}); err != nil {
		t.Fatal(err)
	}
	i32, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 32})
	ptr, _ := b.InternType(lb.TypeSpec{Kind: lb.TypePointer})
	node, err := b.InternStruct(lb.StructSpec{Name: "magma.Node", Opaque: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := b.DefineStruct(node, []lb.TypeID{i32, ptr}, false); err != nil {
		t.Fatal(err)
	}
	zero, err := b.InternConstant(lb.ConstantSpec{Kind: lb.ConstantZero, Type: node})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.DeclareGlobal(lb.GlobalSpec{Symbol: "root", Type: node, Initializer: zero, Linkage: lb.LinkageInternal, Visibility: lb.VisibilityDefault, Definition: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.InternStruct(lb.StructSpec{Elements: []lb.TypeID{i32, ptr}, Packed: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.InternFunctionType(lb.FunctionTypeSpec{Result: i32, Parameters: []lb.TypeID{ptr}, Variadic: true}); err != nil {
		t.Fatal(err)
	}
	ir, _ := b.module.String()
	for _, want := range []string{
		`source_filename = "types.mg"`,
		`target datalayout = "e-p:64:64"`,
		`target triple = "x86_64-unknown-linux-gnu"`,
		`%magma.Node = type { i32, ptr }`,
	} {
		if !strings.Contains(ir, want) {
			t.Errorf("module IR missing %q:\n%s", want, ir)
		}
	}
}

func TestNeutralBackendBuildsCompositeConstants(t *testing.T) {
	b, _ := NewLoweringBackend("constants.mg")
	defer b.Close()
	f64, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeFloat, Bits: 64})
	i8, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 8})
	bytes, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeArray, Element: i8, Length: 4})
	pair, _ := b.InternStruct(lb.StructSpec{Elements: []lb.TypeID{f64, bytes}})
	one, err := b.InternConstant(lb.ConstantSpec{Kind: lb.ConstantFloat, Type: f64, Float: "1.0"})
	if err != nil {
		t.Fatal(err)
	}
	text, err := b.InternConstant(lb.ConstantSpec{Kind: lb.ConstantString, Type: bytes, Bytes: "abc", NullTerminated: true})
	if err != nil {
		t.Fatal(err)
	}
	value, err := b.InternConstant(lb.ConstantSpec{Kind: lb.ConstantAggregate, Type: pair, Elements: []lb.ConstantID{one, text}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.DeclareGlobal(lb.GlobalSpec{Symbol: "composite", Type: pair, Initializer: value, Linkage: lb.LinkageInternal, Visibility: lb.VisibilityDefault, Definition: true, Constant: true}); err != nil {
		t.Fatal(err)
	}
	undefined, err := b.InternConstant(lb.ConstantSpec{Kind: lb.ConstantUndef, Type: i8})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.DeclareGlobal(lb.GlobalSpec{Symbol: "undefined", Type: i8, Initializer: undefined, Linkage: lb.LinkageInternal, Visibility: lb.VisibilityDefault, Definition: true}); err != nil {
		t.Fatal(err)
	}
	ir, _ := b.module.String()
	for _, want := range []string{`{ double 1.000000e+00, [4 x i8] c"abc\00" }`, `@undefined = internal global i8 undef`} {
		if !strings.Contains(ir, want) {
			t.Errorf("constant IR missing %q:\n%s", want, ir)
		}
	}
}

func TestNeutralBackendBuildsVariadicAttributedDeclaration(t *testing.T) {
	b, _ := NewLoweringBackend("abi.mg")
	defer b.Close()
	i32, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 32})
	ptr, _ := b.InternType(lb.TypeSpec{Kind: lb.TypePointer})
	_, err := b.DeclareFunction(lb.FunctionSpec{
		Symbol: "native", Result: i32, Parameters: []lb.TypeID{ptr}, Variadic: true,
		Linkage: lb.LinkageExternal, CallingConvention: lb.CallingConventionC,
		Attributes: []lb.AttributeSpec{
			{Kind: lb.AttributeNoReturn, Placement: lb.AttributeFunction},
			{Kind: lb.AttributeNoUnwind, Placement: lb.AttributeFunction},
			{Kind: lb.AttributeSignExtend, Placement: lb.AttributeReturn},
			{Kind: lb.AttributeNonNull, Placement: lb.AttributeParameter, Parameter: 0},
			{Kind: lb.AttributeAlignment, Placement: lb.AttributeParameter, Parameter: 0, Value: 8},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.DeclareFunction(lb.FunctionSpec{Symbol: "background", Result: i32, Linkage: lb.LinkageExternal, CallingConvention: lb.CallingConventionCold})
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Verify(); err != nil {
		t.Fatal(err)
	}
	ir, _ := b.module.String()
	for _, want := range []string{"declare signext i32 @native", "ptr nonnull align 8", "...", "noreturn", "nounwind", "declare coldcc i32 @background"} {
		if !strings.Contains(ir, want) {
			t.Errorf("function ABI IR missing %q:\n%s", want, ir)
		}
	}
}

func TestNeutralBackendBuildsValidatedCFGAndPhi(t *testing.T) {
	b, _ := NewLoweringBackend("cfg.mg")
	defer b.Close()
	i1, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 1})
	i32, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 32})
	oneConstant, _ := b.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: i32, Integer: "1"})
	twoConstant, _ := b.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: i32, Integer: "2"})
	one, _ := b.ConstantValue(oneConstant)
	two, _ := b.ConstantValue(twoConstant)
	function, _ := b.DeclareFunction(lb.FunctionSpec{Symbol: "choose", Result: i32, Parameters: []lb.TypeID{i1}, Linkage: lb.LinkageInternal, Definition: true})
	entry, _ := b.AppendBlock(function, "entry")
	thenBlock, _ := b.AppendBlock(function, "then")
	elseBlock, _ := b.AppendBlock(function, "else")
	merge, _ := b.AppendBlock(function, "merge")
	condition, _ := b.Parameter(function, 0)
	if err := b.CondBranch(entry, condition, thenBlock, elseBlock); err != nil {
		t.Fatal(err)
	}
	if err := b.Branch(thenBlock, merge); err != nil {
		t.Fatal(err)
	}
	if err := b.Branch(elseBlock, merge); err != nil {
		t.Fatal(err)
	}
	phi, err := b.Phi(merge, i32)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.AddPhiIncoming(phi, []lb.ValueID{one, two}, []lb.BlockID{thenBlock, elseBlock}); err != nil {
		t.Fatal(err)
	}
	if err := b.AttachSource(phi, lb.SourceLocation{File: "cfg.mg", Line: 9, Column: 4}); err != nil {
		t.Fatal(err)
	}
	if err := b.Return(merge, phi); err != nil {
		t.Fatal(err)
	}
	if err := b.FinalizeFunction(function); err != nil {
		t.Fatal(err)
	}
	if err := b.Verify(); err != nil {
		t.Fatal(err)
	}
	ir, _ := b.module.String()
	for _, want := range []string{"br i1 %0", "phi i32 [ 1, %then ], [ 2, %else ]", `!magma.source !`} {
		if !strings.Contains(ir, want) {
			t.Errorf("CFG IR missing %q:\n%s", want, ir)
		}
	}
}

func TestNeutralBackendAttachesConditionalBranchWeights(t *testing.T) {
	b, _ := NewLoweringBackend("weighted-branch.mg")
	defer b.Close()
	i1, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 1})
	fn, _ := b.DeclareFunction(lb.FunctionSpec{Symbol: "weighted", Result: i1, Parameters: []lb.TypeID{i1}, Linkage: lb.LinkageInternal, Definition: true})
	entry, _ := b.AppendBlock(fn, "entry")
	unlikely, _ := b.AppendBlock(fn, "unlikely")
	likely, _ := b.AppendBlock(fn, "likely")
	condition, _ := b.Parameter(fn, 0)
	if err := b.CondBranchWeighted(entry, condition, unlikely, likely, lb.UnlikelyThen); err != nil {
		t.Fatal(err)
	}
	if err := b.Return(unlikely, condition); err != nil {
		t.Fatal(err)
	}
	if err := b.Return(likely, condition); err != nil {
		t.Fatal(err)
	}
	ir, _ := b.module.String()
	if !strings.Contains(ir, `!{!"branch_weights", i32 1, i32 2000}`) {
		t.Fatalf("weighted branch metadata missing:\n%s", ir)
	}
}

func TestNeutralBackendBuildsTypedMemoryOperations(t *testing.T) {
	b, _ := NewLoweringBackend("memory.mg")
	defer b.Close()
	i32, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 32})
	array, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeArray, Element: i32, Length: 2})
	zeroConstant, _ := b.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: i32, Integer: "0"})
	oneConstant, _ := b.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: i32, Integer: "1"})
	answerConstant, _ := b.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: i32, Integer: "42"})
	zero, _ := b.ConstantValue(zeroConstant)
	one, _ := b.ConstantValue(oneConstant)
	answer, _ := b.ConstantValue(answerConstant)
	function, _ := b.DeclareFunction(lb.FunctionSpec{Symbol: "memory", Result: i32, Linkage: lb.LinkageInternal, Definition: true})
	entry, _ := b.AppendBlock(function, "entry")
	storage, err := b.Alloca(entry, array, 8)
	if err != nil {
		t.Fatal(err)
	}
	element, err := b.GEP(entry, array, storage, []lb.ValueID{zero, one}, true)
	if err != nil {
		t.Fatal(err)
	}
	store, err := b.Store(entry, answer, element, 4, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.AttachSource(store, lb.SourceLocation{File: "memory.mg", Line: 4, Column: 2}); err != nil {
		t.Fatal(err)
	}
	loaded, err := b.Load(entry, i32, element, 4, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Return(entry, loaded); err != nil {
		t.Fatal(err)
	}
	if err := b.FinalizeFunction(function); err != nil {
		t.Fatal(err)
	}
	if err := b.Verify(); err != nil {
		t.Fatal(err)
	}
	ir, _ := b.module.String()
	for _, want := range []string{"alloca [2 x i32], align 8", "getelementptr inbounds [2 x i32]", "store volatile i32 42", "load volatile i32"} {
		if !strings.Contains(ir, want) {
			t.Errorf("memory IR missing %q:\n%s", want, ir)
		}
	}
}

func TestStaticAllocaIsHoistedWithoutMovingInitialization(t *testing.T) {
	b, _ := NewLoweringBackend("static-local.mg")
	defer b.Close()
	i32, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 32})
	function, _ := b.DeclareFunction(lb.FunctionSpec{Symbol: "static_local", Result: i32, Parameters: []lb.TypeID{i32}, Linkage: lb.LinkageInternal, Definition: true})
	entry, _ := b.AppendBlock(function, "entry")
	body, _ := b.AppendBlock(function, "body")
	parameter, _ := b.Parameter(function, 0)
	if err := b.Branch(entry, body); err != nil {
		t.Fatal(err)
	}
	storage, err := b.StaticAlloca(function, i32, 4)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Store(body, parameter, storage, 4, false); err != nil {
		t.Fatal(err)
	}
	loaded, err := b.Load(body, i32, storage, 4, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Return(body, loaded); err != nil {
		t.Fatal(err)
	}
	if err := b.FinalizeFunction(function); err != nil {
		t.Fatal(err)
	}
	if err := b.Verify(); err != nil {
		t.Fatal(err)
	}
	ir, _ := b.module.String()
	allocation := strings.Index(ir, "alloca i32, align 4")
	entryBranch := strings.Index(ir, "br label %body")
	bodyLabel := strings.Index(ir, "body:")
	initialization := strings.Index(ir, "store i32 %0")
	if allocation < 0 || entryBranch < 0 || bodyLabel < 0 || initialization < 0 || !(allocation < entryBranch && bodyLabel < initialization) {
		t.Fatalf("static storage or declaration-point initialization moved:\n%s", ir)
	}
}

func TestNeutralBackendRejectsInvalidMemoryOperations(t *testing.T) {
	b, _ := NewLoweringBackend("bad-memory.mg")
	defer b.Close()
	i32, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 32})
	f32, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeFloat, Bits: 32})
	zero, _ := b.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: i32, Integer: "0"})
	index, _ := b.ConstantValue(zero)
	floatZero, _ := b.InternConstant(lb.ConstantSpec{Kind: lb.ConstantZero, Type: f32})
	floatIndex, _ := b.ConstantValue(floatZero)
	function, _ := b.DeclareFunction(lb.FunctionSpec{Symbol: "bad", Result: i32, Linkage: lb.LinkageInternal, Definition: true})
	entry, _ := b.AppendBlock(function, "entry")
	if _, err := b.Alloca(entry, i32, 3); err == nil || !strings.Contains(err.Error(), "power of two") {
		t.Fatalf("invalid alignment error = %v", err)
	}
	storage, _ := b.Alloca(entry, i32, 4)
	if _, err := b.Load(entry, f32, storage, 4, false); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("mismatched load error = %v", err)
	}
	if _, err := b.Store(entry, floatIndex, storage, 4, false); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("mismatched store error = %v", err)
	}
	if _, err := b.Load(entry, i32, index, 4, false); err == nil || !strings.Contains(err.Error(), "pointer") {
		t.Fatalf("non-pointer load error = %v", err)
	}
	if _, err := b.GEP(entry, i32, storage, []lb.ValueID{floatIndex}, false); err == nil || !strings.Contains(err.Error(), "not an integer") {
		t.Fatalf("non-integer GEP error = %v", err)
	}
}

func TestNeutralBackendRejectsInvalidAtomicOrderings(t *testing.T) {
	b, _ := NewLoweringBackend("bad-atomics.mg")
	defer b.Close()
	i64, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 64})
	function, _ := b.DeclareFunction(lb.FunctionSpec{Symbol: "bad_atomics", Result: i64, Parameters: []lb.TypeID{i64}, Linkage: lb.LinkageInternal, Definition: true})
	entry, _ := b.AppendBlock(function, "entry")
	value, _ := b.Parameter(function, 0)
	storage, _ := b.Alloca(entry, i64, 8)
	if _, err := b.AtomicLoad(entry, i64, storage, lb.AtomicRelease, 8); err == nil || !strings.Contains(err.Error(), "load ordering") {
		t.Fatalf("invalid load ordering error = %v", err)
	}
	if _, err := b.AtomicStore(entry, value, storage, lb.AtomicAcquire, 8); err == nil || !strings.Contains(err.Error(), "store ordering") {
		t.Fatalf("invalid store ordering error = %v", err)
	}
	if _, err := b.CompareExchangeOld(entry, storage, value, value, lb.AtomicAcquire, lb.AtomicSequentiallyConsistent, 8); err == nil || !strings.Contains(err.Error(), "ordering pair") {
		t.Fatalf("invalid cmpxchg ordering error = %v", err)
	}
}

func TestNeutralBackendBuildsScalarExpressions(t *testing.T) {
	b, _ := NewLoweringBackend("scalar.mg")
	defer b.Close()
	i1, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 1})
	i32, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 32})
	i64, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 64})
	f64, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeFloat, Bits: 64})
	integerFunction, _ := b.DeclareFunction(lb.FunctionSpec{Symbol: "integer_scalar", Result: i32, Parameters: []lb.TypeID{i32, i32}, Linkage: lb.LinkageInternal, Definition: true})
	integerEntry, _ := b.AppendBlock(integerFunction, "entry")
	left, _ := b.Parameter(integerFunction, 0)
	right, _ := b.Parameter(integerFunction, 1)
	sum, err := b.Binary(integerEntry, lb.BinaryAdd, left, right)
	if err != nil {
		t.Fatal(err)
	}
	greater, err := b.Compare(integerEntry, lb.CompareSignedGreater, sum, right)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := b.Select(integerEntry, greater, sum, right)
	if err != nil {
		t.Fatal(err)
	}
	extended, err := b.Cast(integerEntry, lb.CastSignExtend, selected, i64)
	if err != nil {
		t.Fatal(err)
	}
	truncated, err := b.Cast(integerEntry, lb.CastTruncate, extended, i32)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Return(integerEntry, truncated); err != nil {
		t.Fatal(err)
	}
	if err := b.FinalizeFunction(integerFunction); err != nil {
		t.Fatal(err)
	}
	floatFunction, _ := b.DeclareFunction(lb.FunctionSpec{Symbol: "float_scalar", Result: i1, Parameters: []lb.TypeID{f64, f64}, Linkage: lb.LinkageInternal, Definition: true})
	floatEntry, _ := b.AppendBlock(floatFunction, "entry")
	first, _ := b.Parameter(floatFunction, 0)
	second, _ := b.Parameter(floatFunction, 1)
	added, _ := b.Binary(floatEntry, lb.BinaryAdd, first, second)
	divided, _ := b.Binary(floatEntry, lb.BinaryFloatDiv, added, second)
	less, err := b.Compare(floatEntry, lb.CompareFloatOrderedLess, divided, first)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Return(floatEntry, less); err != nil {
		t.Fatal(err)
	}
	if err := b.FinalizeFunction(floatFunction); err != nil {
		t.Fatal(err)
	}
	if err := b.Verify(); err != nil {
		t.Fatal(err)
	}
	ir, _ := b.module.String()
	for _, want := range []string{"add i32", "icmp sgt i32", "select i1", "sext i32", "trunc i64", "fadd double", "fdiv double", "fcmp olt double"} {
		if !strings.Contains(ir, want) {
			t.Errorf("scalar IR missing %q:\n%s", want, ir)
		}
	}
}

func TestNeutralBackendRejectsInvalidScalarExpressions(t *testing.T) {
	b, _ := NewLoweringBackend("bad-scalar.mg")
	defer b.Close()
	i32, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 32})
	f32, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeFloat, Bits: 32})
	ptr, _ := b.InternType(lb.TypeSpec{Kind: lb.TypePointer})
	function, _ := b.DeclareFunction(lb.FunctionSpec{Symbol: "bad_scalar", Result: i32, Parameters: []lb.TypeID{i32, f32, ptr, ptr}, Linkage: lb.LinkageInternal, Definition: true})
	entry, _ := b.AppendBlock(function, "entry")
	integer, _ := b.Parameter(function, 0)
	floating, _ := b.Parameter(function, 1)
	pointerA, _ := b.Parameter(function, 2)
	pointerB, _ := b.Parameter(function, 3)
	if _, err := b.Binary(entry, lb.BinaryAdd, integer, floating); err == nil || !strings.Contains(err.Error(), "different types") {
		t.Fatalf("mixed binary error = %v", err)
	}
	if _, err := b.Binary(entry, lb.BinarySignedDiv, floating, floating); err == nil || !strings.Contains(err.Error(), "integer") {
		t.Fatalf("float signed-div error = %v", err)
	}
	if _, err := b.Cast(entry, lb.CastZeroExtend, integer, i32); err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("same-width extension error = %v", err)
	}
	if _, err := b.Compare(entry, lb.CompareUnsignedLess, pointerA, pointerB); err == nil || !strings.Contains(err.Error(), "only supports equality") {
		t.Fatalf("pointer ordering error = %v", err)
	}
	if _, err := b.Select(entry, integer, integer, integer); err == nil || !strings.Contains(err.Error(), "must be i1") {
		t.Fatalf("select condition error = %v", err)
	}
}

func TestNeutralBackendBuildsDirectCallsWithABI(t *testing.T) {
	b, _ := NewLoweringBackend("calls.mg")
	defer b.Close()
	i32, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 32})
	callee, err := b.DeclareFunction(lb.FunctionSpec{Symbol: "fast_native", Result: i32, Parameters: []lb.TypeID{i32}, Linkage: lb.LinkageExternal, CallingConvention: lb.CallingConventionFast, Attributes: []lb.AttributeSpec{{Kind: lb.AttributeSignExtend, Placement: lb.AttributeReturn}, {Kind: lb.AttributeSignExtend, Placement: lb.AttributeParameter}}})
	if err != nil {
		t.Fatal(err)
	}
	caller, _ := b.DeclareFunction(lb.FunctionSpec{Symbol: "caller", Result: i32, Parameters: []lb.TypeID{i32}, Linkage: lb.LinkageInternal, Definition: true})
	entry, _ := b.AppendBlock(caller, "entry")
	argument, _ := b.Parameter(caller, 0)
	result, err := b.Call(entry, callee, []lb.ValueID{argument})
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Return(entry, result); err != nil {
		t.Fatal(err)
	}
	if err := b.FinalizeFunction(caller); err != nil {
		t.Fatal(err)
	}
	if err := b.Verify(); err != nil {
		t.Fatal(err)
	}
	ir, _ := b.module.String()
	for _, want := range []string{"declare fastcc signext i32 @fast_native(i32 signext)", "call fastcc signext i32 @fast_native(i32 signext"} {
		if !strings.Contains(ir, want) {
			t.Errorf("call IR missing %q:\n%s", want, ir)
		}
	}
}

func TestNeutralBackendRejectsInvalidDirectCalls(t *testing.T) {
	b, _ := NewLoweringBackend("bad-calls.mg")
	defer b.Close()
	i32, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 32})
	i64, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 64})
	callee, _ := b.DeclareFunction(lb.FunctionSpec{Symbol: "native", Result: i32, Parameters: []lb.TypeID{i32}, Linkage: lb.LinkageExternal})
	caller, _ := b.DeclareFunction(lb.FunctionSpec{Symbol: "caller", Result: i32, Parameters: []lb.TypeID{i64}, Linkage: lb.LinkageInternal, Definition: true})
	entry, _ := b.AppendBlock(caller, "entry")
	wrong, _ := b.Parameter(caller, 0)
	if _, err := b.Call(entry, callee, nil); err == nil || !strings.Contains(err.Error(), "expected 1") {
		t.Fatalf("missing argument error = %v", err)
	}
	if _, err := b.Call(entry, callee, []lb.ValueID{wrong}); err == nil || !strings.Contains(err.Error(), "wrong type") {
		t.Fatalf("wrong argument error = %v", err)
	}
}

func TestNeutralBackendBuildsIndirectCalls(t *testing.T) {
	b, _ := NewLoweringBackend("indirect-calls.mg")
	defer b.Close()
	i32, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 32})
	ptr, _ := b.InternType(lb.TypeSpec{Kind: lb.TypePointer})
	signature, _ := b.InternFunctionType(lb.FunctionTypeSpec{Result: i32, Parameters: []lb.TypeID{i32}})
	caller, _ := b.DeclareFunction(lb.FunctionSpec{Symbol: "dispatch", Result: i32, Parameters: []lb.TypeID{ptr, i32}, Linkage: lb.LinkageInternal, Definition: true})
	entry, _ := b.AppendBlock(caller, "entry")
	callee, _ := b.Parameter(caller, 0)
	argument, _ := b.Parameter(caller, 1)
	result, err := b.IndirectCall(entry, signature, callee, []lb.ValueID{argument}, lb.CallSpec{CallingConvention: lb.CallingConventionFast, Attributes: []lb.AttributeSpec{{Kind: lb.AttributeSignExtend, Placement: lb.AttributeReturn}, {Kind: lb.AttributeSignExtend, Placement: lb.AttributeParameter}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Return(entry, result); err != nil {
		t.Fatal(err)
	}
	if err := b.FinalizeFunction(caller); err != nil {
		t.Fatal(err)
	}
	if err := b.Verify(); err != nil {
		t.Fatal(err)
	}
	ir, _ := b.module.String()
	if !strings.Contains(ir, "call fastcc signext i32 %0(i32 signext %1)") {
		t.Fatalf("missing indirect ABI call:\n%s", ir)
	}
}

func TestFunctionAddressCarriesSignatureProvenance(t *testing.T) {
	b, _ := NewLoweringBackend("function-address.mg")
	defer b.Close()
	i32, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 32})
	i64, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 64})
	target, _ := b.DeclareFunction(lb.FunctionSpec{Symbol: "target", Result: i32, Parameters: []lb.TypeID{i32}, Linkage: lb.LinkageExternal})
	address, err := b.FunctionAddress(target)
	if err != nil {
		t.Fatal(err)
	}
	wrongSignature, _ := b.InternFunctionType(lb.FunctionTypeSpec{Result: i64, Parameters: []lb.TypeID{i32}})
	caller, _ := b.DeclareFunction(lb.FunctionSpec{Symbol: "caller_by_address", Result: i64, Parameters: []lb.TypeID{i32}, Linkage: lb.LinkageInternal, Definition: true})
	entry, _ := b.AppendBlock(caller, "entry")
	argument, _ := b.Parameter(caller, 0)
	if _, err := b.IndirectCall(entry, wrongSignature, address, []lb.ValueID{argument}, lb.CallSpec{}); err == nil || !strings.Contains(err.Error(), "signature does not match") {
		t.Fatalf("signature provenance error = %v", err)
	}
}

func TestNeutralBackendQueriesTargetLayout(t *testing.T) {
	b, _ := NewLoweringBackend("layout.mg")
	defer b.Close()
	if err := b.ConfigureModule(lb.ModuleSpec{SourceFile: "layout.mg", TargetTriple: "x86_64-unknown-linux-gnu", DataLayout: "e-p:64:64-i64:64"}); err != nil {
		t.Fatal(err)
	}
	i8, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 8})
	i64, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 64})
	pair, _ := b.InternStruct(lb.StructSpec{Name: "layout.Pair", Elements: []lb.TypeID{i8, i64}})
	layout, err := b.TypeLayout(pair)
	if err != nil {
		t.Fatal(err)
	}
	if layout.SizeBits != 128 || layout.StoreSize != 16 || layout.AllocationSize != 16 || layout.ABIAlignment != 8 {
		t.Fatalf("unexpected pair layout: %#v", layout)
	}
	offset, err := b.StructFieldOffset(pair, 1)
	if err != nil {
		t.Fatal(err)
	}
	if offset != 8 {
		t.Fatalf("field offset = %d, want 8", offset)
	}
	if _, err := b.StructFieldOffset(pair, 2); err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("field bounds error = %v", err)
	}
}

func TestNeutralBackendBuildsAndAddressesAggregates(t *testing.T) {
	b, _ := NewLoweringBackend("aggregates.mg")
	defer b.Close()
	i32, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 32})
	i64, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 64})
	array, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeArray, Element: i64, Length: 2})
	record, _ := b.InternStruct(lb.StructSpec{Name: "aggregate.Record", Elements: []lb.TypeID{i32, array}})
	function, _ := b.DeclareFunction(lb.FunctionSpec{Symbol: "aggregate", Result: i64, Parameters: []lb.TypeID{i32, i64, i64, i64}, Linkage: lb.LinkageInternal, Definition: true})
	entry, _ := b.AppendBlock(function, "entry")
	tag, _ := b.Parameter(function, 0)
	first, _ := b.Parameter(function, 1)
	second, _ := b.Parameter(function, 2)
	replacement, _ := b.Parameter(function, 3)
	values, err := b.BuildAggregate(entry, array, []lb.ValueID{first, second})
	if err != nil {
		t.Fatal(err)
	}
	aggregate, err := b.BuildAggregate(entry, record, []lb.ValueID{tag, values})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := b.InsertValue(entry, aggregate, replacement, []uint32{1, 0})
	if err != nil {
		t.Fatal(err)
	}
	extracted, err := b.ExtractValue(entry, updated, []uint32{1, 0})
	if err != nil {
		t.Fatal(err)
	}
	storage, _ := b.Alloca(entry, record, 8)
	if _, err := b.Store(entry, updated, storage, 8, false); err != nil {
		t.Fatal(err)
	}
	field, err := b.StructFieldAddress(entry, record, storage, 0)
	if err != nil {
		t.Fatal(err)
	}
	loadedTag, err := b.Load(entry, i32, field, 4, false)
	if err != nil {
		t.Fatal(err)
	}
	_ = loadedTag
	if err := b.Return(entry, extracted); err != nil {
		t.Fatal(err)
	}
	if err := b.FinalizeFunction(function); err != nil {
		t.Fatal(err)
	}
	if err := b.Verify(); err != nil {
		t.Fatal(err)
	}
	ir, _ := b.module.String()
	for _, want := range []string{"insertvalue [2 x i64]", "insertvalue %aggregate.Record", "extractvalue [2 x i64]", "getelementptr inbounds %aggregate.Record"} {
		if !strings.Contains(ir, want) {
			t.Errorf("aggregate IR missing %q:\n%s", want, ir)
		}
	}
}

func TestNeutralBackendRejectsInvalidAggregateOperations(t *testing.T) {
	b, _ := NewLoweringBackend("bad-aggregates.mg")
	defer b.Close()
	i32, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 32})
	i64, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 64})
	pair, _ := b.InternStruct(lb.StructSpec{Elements: []lb.TypeID{i32, i32}})
	constant, _ := b.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: i64, Integer: "1"})
	wrong, _ := b.ConstantValue(constant)
	function, _ := b.DeclareFunction(lb.FunctionSpec{Symbol: "bad_aggregate", Result: i32, Linkage: lb.LinkageInternal, Definition: true})
	entry, _ := b.AppendBlock(function, "entry")
	if _, err := b.BuildAggregate(entry, pair, []lb.ValueID{wrong}); err == nil || !strings.Contains(err.Error(), "requires 2") {
		t.Fatalf("aggregate arity error = %v", err)
	}
	if _, err := b.BuildAggregate(entry, pair, []lb.ValueID{wrong, wrong}); err == nil || !strings.Contains(err.Error(), "wrong type") {
		t.Fatalf("aggregate type error = %v", err)
	}
}

func TestInlineAssemblyRequiresMatchingConfiguredTarget(t *testing.T) {
	b, err := NewLoweringBackend("asm-target.mg")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	void, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeVoid})
	function, _ := b.DeclareFunction(lb.FunctionSpec{Symbol: "asm_target", Result: void, Linkage: lb.LinkageInternal, Definition: true})
	entry, _ := b.AppendBlock(function, "entry")
	if _, err := b.InlineAssemblySideEffect(entry, "pause"); err == nil || !strings.Contains(err.Error(), "configured target triple") {
		t.Fatalf("missing-target error = %v", err)
	}
	if err := b.ConfigureModule(lb.ModuleSpec{SourceFile: "asm-target.mg", TargetTriple: "aarch64-unknown-linux-gnu"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.InlineAssemblySideEffect(entry, "pause"); err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("wrong-target error = %v", err)
	}
	if _, err := b.InlineAssemblySideEffect(entry, "yield"); err != nil {
		t.Fatal(err)
	}
	if err := b.ReturnVoid(entry); err != nil {
		t.Fatal(err)
	}
	if err := b.FinalizeFunction(function); err != nil {
		t.Fatal(err)
	}
}

func TestGlobalAddressConstantsBuildImmutableAggregateGlobals(t *testing.T) {
	b, err := NewLoweringBackend("global-address.mg")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	i8, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 8})
	pointer, _ := b.InternType(lb.TypeSpec{Kind: lb.TypePointer})
	bytesType, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeArray, Element: i8, Length: 4})
	bytes, _ := b.InternConstant(lb.ConstantSpec{Kind: lb.ConstantString, Type: bytesType, Bytes: "site", NullTerminated: false})
	data, err := b.DeclareGlobal(lb.GlobalSpec{Symbol: ".trace.text", Type: bytesType, Initializer: bytes, Linkage: lb.LinkagePrivate, Visibility: lb.VisibilityDefault, Constant: true, Definition: true})
	if err != nil {
		t.Fatal(err)
	}
	address, err := b.InternConstant(lb.ConstantSpec{Kind: lb.ConstantGlobalAddress, Type: pointer, Global: data})
	if err != nil {
		t.Fatal(err)
	}
	void, _ := b.InternType(lb.TypeSpec{Kind: lb.TypeVoid})
	callback, err := b.DeclareFunction(lb.FunctionSpec{Symbol: "trace_callback", Result: void, Linkage: lb.LinkageExternal})
	if err != nil {
		t.Fatal(err)
	}
	callbackAddress, err := b.InternConstant(lb.ConstantSpec{Kind: lb.ConstantFunctionAddress, Type: pointer, Function: callback})
	if err != nil {
		t.Fatal(err)
	}
	siteType, _ := b.InternStruct(lb.StructSpec{Elements: []lb.TypeID{pointer, pointer}})
	siteValue, err := b.InternConstant(lb.ConstantSpec{Kind: lb.ConstantAggregate, Type: siteType, Elements: []lb.ConstantID{address, callbackAddress}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.DeclareGlobal(lb.GlobalSpec{Symbol: ".trace.site", Type: siteType, Initializer: siteValue, Linkage: lb.LinkagePrivate, Visibility: lb.VisibilityDefault, Constant: true, Definition: true}); err != nil {
		t.Fatal(err)
	}
	if err := b.Verify(); err != nil {
		t.Fatal(err)
	}
	ir, _ := b.module.String()
	if !strings.Contains(ir, "@.trace.site = private constant { ptr, ptr } { ptr @.trace.text, ptr @trace_callback }") {
		t.Fatalf("global address aggregate is missing:\n%s", ir)
	}
	if _, err := b.InternConstant(lb.ConstantSpec{Kind: lb.ConstantGlobalAddress, Type: pointer, Global: lb.GlobalID(999)}); err == nil || !strings.Contains(err.Error(), "unknown backend global") {
		t.Fatalf("unknown-global error = %v", err)
	}
	if _, err := b.InternConstant(lb.ConstantSpec{Kind: lb.ConstantFunctionAddress, Type: pointer, Function: lb.FunctionID(999)}); err == nil || !strings.Contains(err.Error(), "unknown backend function") {
		t.Fatalf("unknown-function error = %v", err)
	}
}
