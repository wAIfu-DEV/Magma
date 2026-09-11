package loweringbackend

import (
	"strings"
	"testing"
)

func TestRegistryInternsStructuralTypes(t *testing.T) {
	r := NewRegistry()
	i64a, err := r.InternType(TypeSpec{Kind: TypeInteger, Bits: 64})
	if err != nil {
		t.Fatal(err)
	}
	i64b, _ := r.InternType(TypeSpec{Kind: TypeInteger, Bits: 64})
	if i64a != i64b {
		t.Fatalf("equivalent integer types got %d and %d", i64a, i64b)
	}
	arrayA, _ := r.InternType(TypeSpec{Kind: TypeArray, Element: i64a, Length: 4})
	arrayB, _ := r.InternType(TypeSpec{Kind: TypeArray, Element: i64a, Length: 4})
	if arrayA != arrayB {
		t.Fatalf("equivalent arrays got %d and %d", arrayA, arrayB)
	}
	vectorA, _ := r.InternType(TypeSpec{Kind: TypeVector, Element: i64a, Length: 2})
	vectorB, _ := r.InternType(TypeSpec{Kind: TypeVector, Element: i64a, Length: 2})
	if vectorA != vectorB {
		t.Fatalf("equivalent vectors got %d and %d", vectorA, vectorB)
	}
}

func TestRegistryRejectsInvalidAndForeignTypeIDs(t *testing.T) {
	r := NewRegistry()
	for _, spec := range []TypeSpec{{Kind: TypeInteger}, {Kind: TypeFloat, Bits: 80}, {Kind: TypeArray, Element: 42, Length: 1}, {Kind: TypeVector, Element: 42, Length: 2}, {Kind: TypeVector, Element: 1}} {
		if _, err := r.InternType(spec); err == nil {
			t.Errorf("accepted invalid type %#v", spec)
		}
	}
}

func TestFunctionDeclarationsAreCanonical(t *testing.T) {
	r := NewRegistry()
	void, _ := r.InternType(TypeSpec{Kind: TypeVoid})
	i32, _ := r.InternType(TypeSpec{Kind: TypeInteger, Bits: 32})
	spec := FunctionSpec{Symbol: "work", Result: void, Parameters: []TypeID{i32}, Linkage: LinkageInternal}
	first, err := r.DeclareFunction(spec)
	if err != nil {
		t.Fatal(err)
	}
	definition := spec
	definition.Definition = true
	second, err := r.DeclareFunction(definition)
	if err != nil || first != second {
		t.Fatalf("compatible redeclaration = %d, %v", second, err)
	}
	if _, err := r.DeclareFunction(definition); err == nil || !strings.Contains(err.Error(), "duplicate definition") {
		t.Fatalf("duplicate definition error = %v", err)
	}
	bad := spec
	bad.Result = i32
	if _, err := r.DeclareFunction(bad); err == nil || !strings.Contains(err.Error(), "incompatible") {
		t.Fatalf("incompatible declaration error = %v", err)
	}
}

func TestCFGRequiresExactlyOneTerminator(t *testing.T) {
	cfg := NewCFG()
	block, _ := cfg.AppendBlock(1, "entry")
	if err := cfg.Finalize(1); err == nil || !strings.Contains(err.Error(), "not terminated") {
		t.Fatalf("unterminated finalize error = %v", err)
	}
	if err := cfg.Terminate(block); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Terminate(block); err == nil || !strings.Contains(err.Error(), "already has a terminator") {
		t.Fatalf("second terminator error = %v", err)
	}
	if err := cfg.Finalize(1); err != nil {
		t.Fatal(err)
	}
}

func TestCFGTracksValidatedPredecessors(t *testing.T) {
	cfg := NewCFG()
	entry, _ := cfg.AppendBlock(1, "entry")
	merge, _ := cfg.AppendBlock(1, "merge")
	foreign, _ := cfg.AppendBlock(2, "foreign")
	if err := cfg.ValidateTargets(entry, foreign); err == nil || !strings.Contains(err.Error(), "different function") {
		t.Fatalf("foreign target error = %v", err)
	}
	if err := cfg.TerminateWithTargets(entry, merge); err != nil {
		t.Fatal(err)
	}
	if !cfg.IsPredecessor(merge, entry) || cfg.PredecessorCount(merge) != 1 {
		t.Fatal("CFG did not record predecessor")
	}
}

func TestConstantsAreCanonicalAndArbitraryWidth(t *testing.T) {
	r := NewRegistry()
	i128, _ := r.InternType(TypeSpec{Kind: TypeInteger, Bits: 128})
	first, err := r.InternConstant(ConstantSpec{Kind: ConstantInteger, Type: i128, Integer: "000340282366920938463463374607431768211455"})
	if err != nil {
		t.Fatal(err)
	}
	second, _ := r.InternConstant(ConstantSpec{Kind: ConstantInteger, Type: i128, Integer: "340282366920938463463374607431768211455"})
	if first != second {
		t.Fatalf("equivalent constants got %d and %d", first, second)
	}
	if _, err := r.InternConstant(ConstantSpec{Kind: ConstantInteger, Type: i128, Integer: "340282366920938463463374607431768211456"}); err == nil {
		t.Fatal("overflowing i128 constant was accepted")
	}
}

func TestGlobalDeclarationsAreCanonical(t *testing.T) {
	r := NewRegistry()
	i32, _ := r.InternType(TypeSpec{Kind: TypeInteger, Bits: 32})
	zero, _ := r.InternConstant(ConstantSpec{Kind: ConstantZero, Type: i32})
	declaration := GlobalSpec{Symbol: "counter", Type: i32, Linkage: LinkageInternal, Visibility: VisibilityHidden, Alignment: 4}
	first, err := r.DeclareGlobal(declaration)
	if err != nil {
		t.Fatal(err)
	}
	definition := declaration
	definition.Definition = true
	definition.Initializer = zero
	second, err := r.DeclareGlobal(definition)
	if err != nil || first != second {
		t.Fatalf("compatible global definition = %d, %v", second, err)
	}
	third, err := r.DeclareGlobal(definition)
	if err != nil || third != first {
		t.Fatalf("identical global definition was not interned: id=%d err=%v", third, err)
	}
	bad := declaration
	bad.Alignment = 8
	if _, err := r.DeclareGlobal(bad); err == nil || !strings.Contains(err.Error(), "incompatible") {
		t.Fatalf("incompatible global error = %v", err)
	}
}

func TestCompoundTypesAreCanonical(t *testing.T) {
	r := NewRegistry()
	void, _ := r.InternType(TypeSpec{Kind: TypeVoid})
	i32, _ := r.InternType(TypeSpec{Kind: TypeInteger, Bits: 32})
	literal, err := r.InternStruct(StructSpec{Elements: []TypeID{i32, i32}})
	if err != nil {
		t.Fatal(err)
	}
	again, _ := r.InternStruct(StructSpec{Elements: []TypeID{i32, i32}})
	if literal != again {
		t.Fatalf("equivalent literal structs got %d and %d", literal, again)
	}
	function, err := r.InternFunctionType(FunctionTypeSpec{Result: void, Parameters: []TypeID{literal}, Variadic: true})
	if err != nil {
		t.Fatal(err)
	}
	functionAgain, _ := r.InternFunctionType(FunctionTypeSpec{Result: void, Parameters: []TypeID{literal}, Variadic: true})
	if function != functionAgain {
		t.Fatalf("equivalent function types got %d and %d", function, functionAgain)
	}
}

func TestNamedStructUsesTwoPhaseDefinition(t *testing.T) {
	r := NewRegistry()
	i8, _ := r.InternType(TypeSpec{Kind: TypeInteger, Bits: 8})
	node, err := r.InternStruct(StructSpec{Name: "magma.Node", Opaque: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.DefineStruct(node, []TypeID{i8}, true); err != nil {
		t.Fatal(err)
	}
	if err := r.DefineStruct(node, []TypeID{i8}, true); err == nil || !strings.Contains(err.Error(), "already has a body") {
		t.Fatalf("second body error = %v", err)
	}
	if _, err := r.InternStruct(StructSpec{Name: "magma.Node", Elements: []TypeID{i8}}); err == nil || !strings.Contains(err.Error(), "incompatible") {
		t.Fatalf("incompatible named struct error = %v", err)
	}
}

func TestFloatStringAndAggregateConstants(t *testing.T) {
	r := NewRegistry()
	f64, _ := r.InternType(TypeSpec{Kind: TypeFloat, Bits: 64})
	i8, _ := r.InternType(TypeSpec{Kind: TypeInteger, Bits: 8})
	bytes, _ := r.InternType(TypeSpec{Kind: TypeArray, Element: i8, Length: 4})
	pair, _ := r.InternStruct(StructSpec{Elements: []TypeID{f64, bytes}})
	one, err := r.InternConstant(ConstantSpec{Kind: ConstantFloat, Type: f64, Float: "1.000"})
	if err != nil {
		t.Fatal(err)
	}
	oneAgain, _ := r.InternConstant(ConstantSpec{Kind: ConstantFloat, Type: f64, Float: "1"})
	if one != oneAgain {
		t.Fatalf("equivalent floats got %d and %d", one, oneAgain)
	}
	text, err := r.InternConstant(ConstantSpec{Kind: ConstantString, Type: bytes, Bytes: "abc", NullTerminated: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.InternConstant(ConstantSpec{Kind: ConstantAggregate, Type: pair, Elements: []ConstantID{one, text}}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.InternConstant(ConstantSpec{Kind: ConstantString, Type: bytes, Bytes: "too long", NullTerminated: true}); err == nil {
		t.Fatal("accepted string with mismatched array length")
	}
}

func TestFunctionABIIsCanonicalAndValidated(t *testing.T) {
	r := NewRegistry()
	i32, _ := r.InternType(TypeSpec{Kind: TypeInteger, Bits: 32})
	ptr, _ := r.InternType(TypeSpec{Kind: TypePointer})
	spec := FunctionSpec{Symbol: "native", Result: i32, Parameters: []TypeID{ptr}, Variadic: true, Linkage: LinkageExternal, CallingConvention: CallingConventionC, Attributes: []AttributeSpec{
		{Kind: AttributeNonNull, Placement: AttributeParameter},
		{Kind: AttributeNoUnwind, Placement: AttributeFunction},
	}}
	first, err := r.DeclareFunction(spec)
	if err != nil {
		t.Fatal(err)
	}
	reordered := spec
	reordered.Attributes = []AttributeSpec{spec.Attributes[1], spec.Attributes[0]}
	second, err := r.DeclareFunction(reordered)
	if err != nil || first != second {
		t.Fatalf("equivalent ABI declaration = %d, %v", second, err)
	}
	changed := spec
	changed.Variadic = false
	changed.CallingConvention = CallingConventionCold
	if _, err := r.DeclareFunction(changed); err == nil || !strings.Contains(err.Error(), "incompatible") {
		t.Fatalf("calling convention conflict = %v", err)
	}
	invalid := FunctionSpec{Symbol: "bad", Result: i32, Linkage: LinkageExternal, Attributes: []AttributeSpec{{Kind: AttributeNoReturn, Placement: AttributeReturn}}}
	if _, err := r.DeclareFunction(invalid); err == nil || !strings.Contains(err.Error(), "invalid at placement") {
		t.Fatalf("attribute placement error = %v", err)
	}
}

func TestRegistryValidatesNestedAggregatePaths(t *testing.T) {
	r := NewRegistry()
	i8, _ := r.InternType(TypeSpec{Kind: TypeInteger, Bits: 8})
	i64, _ := r.InternType(TypeSpec{Kind: TypeInteger, Bits: 64})
	array, _ := r.InternType(TypeSpec{Kind: TypeArray, Element: i64, Length: 2})
	structure, _ := r.InternStruct(StructSpec{Elements: []TypeID{i8, array}})
	result, err := r.AggregateElement(structure, []uint32{1, 0})
	if err != nil || result != i64 {
		t.Fatalf("nested aggregate result = %d, %v", result, err)
	}
	if _, err := r.AggregateElement(structure, []uint32{1, 2}); err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("aggregate bounds error = %v", err)
	}
	if _, err := r.AggregateElement(structure, []uint32{0, 0}); err == nil || !strings.Contains(err.Error(), "non-aggregate") {
		t.Fatalf("aggregate descent error = %v", err)
	}
}
