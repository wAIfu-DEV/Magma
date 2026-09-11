//go:build llvm_object

package loweringtypes_test

import (
	"strings"
	"testing"

	llvmobject "Magma/src/llvm_object"
	lb "Magma/src/lowering_backend"
	loweringtypes "Magma/src/lowering_types"
	mt "Magma/src/types"
)

func primitive(name string) *mt.NodeType {
	return &mt.NodeType{KindNode: &mt.NodeTypeNamed{NameNode: &mt.NodeNameSingle{Name: name}}}
}

func semanticState() (*mt.SharedState, *mt.StructDef) {
	errorDef := &mt.StructDef{Module: "std", Name: "Error", CoreRole: mt.CoreTypeError, FieldOrder: []string{"code"}, FieldNb: map[string]int{"code": 0}, Fields: map[string]*mt.NodeType{"code": primitive("u64")}}
	sliceDef := &mt.StructDef{Module: "std", Name: "Slice", CoreRole: mt.CoreTypeSlice, FieldOrder: []string{"__data", "__count"}, FieldNb: map[string]int{"__data": 0, "__count": 1}, Fields: map[string]*mt.NodeType{"__data": primitive("ptr"), "__count": primitive("u64")}}
	stringDef := &mt.StructDef{Module: "std", Name: "String", CoreRole: mt.CoreTypeString, FieldOrder: []string{"__data", "__byteCount"}, FieldNb: map[string]int{"__data": 0, "__byteCount": 1}, Fields: map[string]*mt.NodeType{"__data": primitive("ptr"), "__byteCount": primitive("u64")}}
	record := &mt.StructDef{Module: "app", Name: "Record", FieldOrder: []string{"tag", "value", "next"}, Fields: map[string]*mt.NodeType{
		"tag": primitive("u8"), "value": primitive("u64"),
		"next": {KindNode: &mt.NodeTypePointer{Kind: &mt.NodeTypeAbsolute{AbsoluteName: "app.Record"}}},
	}}
	global := &mt.NodeGlobal{StructDefs: map[string]*mt.StructDef{"Record": record}}
	return &mt.SharedState{Files: map[string]*mt.FileCtx{"record.mg": {GlNode: global}}, CoreTypes: map[mt.CoreTypeRole]*mt.StructDef{mt.CoreTypeError: errorDef, mt.CoreTypeSlice: sliceDef, mt.CoreTypeString: stringDef}}, record
}

func TestSemanticTypeLowererCachesStructsAndUsesTargetLayout(t *testing.T) {
	backend, err := llvmobject.NewLoweringBackend("types.mg")
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	if err := backend.ConfigureModule(lb.ModuleSpec{SourceFile: "types.mg", TargetTriple: "x86_64-unknown-linux-gnu", DataLayout: "e-p:64:64-i64:64"}); err != nil {
		t.Fatal(err)
	}
	state, _ := semanticState()
	lowerer, err := loweringtypes.New(backend, state)
	if err != nil {
		t.Fatal(err)
	}
	node := &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "app.Record"}}
	first, err := lowerer.Lower(node)
	if err != nil {
		t.Fatal(err)
	}
	second, err := lowerer.Lower(node)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("cached struct IDs differ: %d and %d", first, second)
	}
	layout, err := backend.TypeLayout(first)
	if err != nil {
		t.Fatal(err)
	}
	if layout.AllocationSize != 24 || layout.ABIAlignment != 8 {
		t.Fatalf("record layout = %#v", layout)
	}
	offset, _ := backend.StructFieldOffset(first, 2)
	if offset != 16 {
		t.Fatalf("next field offset = %d", offset)
	}
}

func TestSemanticTypeLowererBuildsThrowingAndContextfulSignatures(t *testing.T) {
	backend, _ := llvmobject.NewLoweringBackend("signatures.mg")
	defer backend.Close()
	state, _ := semanticState()
	lowerer, _ := loweringtypes.New(backend, state)
	throwing := primitive("u64")
	throwing.Throws = true
	result, err := lowerer.Lower(throwing)
	if err != nil {
		t.Fatal(err)
	}
	plain := &mt.NodeTypeFunc{Args: []*mt.NodeType{primitive("u64")}, RetType: throwing, ContextABI: mt.ContextABIContextless}
	contextful := &mt.NodeTypeFunc{Args: []*mt.NodeType{primitive("u64")}, RetType: throwing, ContextABI: mt.ContextABIContextful}
	plainID, err := lowerer.Signature(plain)
	if err != nil {
		t.Fatal(err)
	}
	contextID, err := lowerer.Signature(contextful)
	if err != nil {
		t.Fatal(err)
	}
	if plainID == contextID {
		t.Fatal("contextful signature did not include its implicit context pointer")
	}
	if _, err := backend.TypeLayout(result); err == nil || !strings.Contains(err.Error(), "data layout") {
		t.Fatalf("unconfigured layout error = %v", err)
	}
}

func TestSemanticTypeLowererRejectsUnresolvedTypes(t *testing.T) {
	backend, _ := llvmobject.NewLoweringBackend("invalid-types.mg")
	defer backend.Close()
	state, _ := semanticState()
	lowerer, _ := loweringtypes.New(backend, state)
	_, err := lowerer.Lower(primitive("Missing"))
	if err == nil || !strings.Contains(err.Error(), "unresolved") {
		t.Fatalf("unresolved type error = %v", err)
	}
}

func TestSemanticTypeLowererKeepsStructFailuresSticky(t *testing.T) {
	backend, _ := llvmobject.NewLoweringBackend("broken-struct.mg")
	defer backend.Close()
	broken := &mt.StructDef{Module: "app", Name: "Broken", FieldOrder: []string{"missing"}, Fields: map[string]*mt.NodeType{}}
	state := &mt.SharedState{Files: map[string]*mt.FileCtx{"broken.mg": {GlNode: &mt.NodeGlobal{StructDefs: map[string]*mt.StructDef{"Broken": broken}}}}, CoreTypes: map[mt.CoreTypeRole]*mt.StructDef{}}
	lowerer, _ := loweringtypes.New(backend, state)
	node := &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "app.Broken"}}
	_, first := lowerer.Lower(node)
	_, second := lowerer.Lower(node)
	if first == nil || second == nil || first.Error() != second.Error() {
		t.Fatalf("struct failure was not sticky: first=%v second=%v", first, second)
	}
}

func TestProvenSliceIndexEmitsNoAdditionalRuntimeGuard(t *testing.T) {
	backend, _ := llvmobject.NewLoweringBackend("slice-index.mg")
	defer backend.Close()
	state, _ := semanticState()
	lowerer, _ := loweringtypes.New(backend, state)
	sliceType, err := lowerer.Lower(&mt.NodeType{KindNode: &mt.NodeTypeSlice{ElemKind: &mt.NodeTypeNamed{NameNode: &mt.NodeNameSingle{Name: "u8"}}}})
	if err != nil {
		t.Fatal(err)
	}
	i8, _ := lowerer.Lower(primitive("u8"))
	u64, _ := lowerer.Lower(primitive("u64"))
	function, _ := backend.DeclareFunction(lb.FunctionSpec{Symbol: "slice_at", Result: i8, Parameters: []lb.TypeID{sliceType, u64}, Linkage: lb.LinkageInternal, Definition: true})
	entry, _ := backend.AppendBlock(function, "entry")
	slice, _ := backend.Parameter(function, 0)
	index, _ := backend.Parameter(function, 1)
	if _, err := lowerer.ProvenSliceElementAddress(entry, slice, index, i8, nil); err == nil || !strings.Contains(err.Error(), "range proof") {
		t.Fatalf("missing proof error = %v", err)
	}
	address, err := lowerer.ProvenSliceElementAddress(entry, slice, index, i8, &mt.RangeProof{ID: 1, Guarded: true})
	if err != nil {
		t.Fatal(err)
	}
	value, err := backend.Load(entry, i8, address, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.Return(entry, value); err != nil {
		t.Fatal(err)
	}
	if err := backend.FinalizeFunction(function); err != nil {
		t.Fatal(err)
	}
	if err := backend.Verify(); err != nil {
		t.Fatal(err)
	}
}

func TestCoreValueRecipesRejectSchemaDrift(t *testing.T) {
	backend, _ := llvmobject.NewLoweringBackend("schema.mg")
	defer backend.Close()
	state, _ := semanticState()
	state.CoreTypes[mt.CoreTypeSlice].FieldNb["__count"] = 0
	lowerer, _ := loweringtypes.New(backend, state)
	_, err := lowerer.BuildSlice(1, 1, 2)
	if err == nil || !strings.Contains(err.Error(), "inconsistent") {
		t.Fatalf("schema drift error = %v", err)
	}
}

func TestNumericLoweringMatchesTextualOpcodesWithoutRuntimeGuards(t *testing.T) {
	ir, err := llvmobject.ExperimentalIR("numeric.mg", func(backend lb.Backend) error {
		state, _ := semanticState()
		lowerer, err := loweringtypes.New(backend, state)
		if err != nil {
			return err
		}
		i32, _ := lowerer.Lower(primitive("i32"))
		i64, _ := lowerer.Lower(primitive("i64"))
		function, err := backend.DeclareFunction(lb.FunctionSpec{Symbol: "numeric", Result: i64, Parameters: []lb.TypeID{i32, i32}, Linkage: lb.LinkageInternal, Definition: true})
		if err != nil {
			return err
		}
		entry, _ := backend.AppendBlock(function, "entry")
		left, _ := backend.Parameter(function, 0)
		right, _ := backend.Parameter(function, 1)
		quotient, err := lowerer.NumericBinary(entry, mt.KwSlash, left, right, primitive("i32"))
		if err != nil {
			return err
		}
		widened, err := lowerer.CoerceNumeric(entry, quotient, primitive("i32"), primitive("i64"))
		if err != nil {
			return err
		}
		if err := backend.Return(entry, widened); err != nil {
			return err
		}
		return backend.FinalizeFunction(function)
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(ir)
	if !strings.Contains(text, "sdiv i32") || !strings.Contains(text, "sext i32") {
		t.Fatalf("numeric lowering did not preserve textual opcodes:\n%s", text)
	}
	if strings.Contains(text, "icmp") || strings.Contains(text, "br i1") {
		t.Fatalf("numeric lowering introduced a runtime guard:\n%s", text)
	}
}

func TestNumericComparisonAndUnaryPreserveTextualSemantics(t *testing.T) {
	ir, err := llvmobject.ExperimentalIR("predicates.mg", func(backend lb.Backend) error {
		state, _ := semanticState()
		lowerer, _ := loweringtypes.New(backend, state)
		f64, _ := lowerer.Lower(primitive("f64"))
		boolean, _ := lowerer.Lower(primitive("bool"))
		function, _ := backend.DeclareFunction(lb.FunctionSpec{Symbol: "float_ne", Result: boolean, Parameters: []lb.TypeID{f64, f64}, Linkage: lb.LinkageInternal, Definition: true})
		entry, _ := backend.AppendBlock(function, "entry")
		left, _ := backend.Parameter(function, 0)
		right, _ := backend.Parameter(function, 1)
		result, err := lowerer.Compare(entry, mt.KwCmpNeq, left, right, primitive("f64"))
		if err != nil {
			return err
		}
		if err := backend.Return(entry, result); err != nil {
			return err
		}
		if err := backend.FinalizeFunction(function); err != nil {
			return err
		}
		i8, _ := lowerer.Lower(primitive("i8"))
		invert, _ := backend.DeclareFunction(lb.FunctionSpec{Symbol: "invert", Result: i8, Parameters: []lb.TypeID{i8}, Linkage: lb.LinkageInternal, Definition: true})
		invertEntry, _ := backend.AppendBlock(invert, "entry")
		input, _ := backend.Parameter(invert, 0)
		inverted, err := lowerer.UnaryNot(invertEntry, input, primitive("i8"))
		if err != nil {
			return err
		}
		if err := backend.Return(invertEntry, inverted); err != nil {
			return err
		}
		return backend.FinalizeFunction(invert)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ir), "fcmp une double") {
		t.Fatalf("float != must retain textual unordered semantics:\n%s", ir)
	}
	if !strings.Contains(string(ir), "xor i8") {
		t.Fatalf("integer not must retain textual xor lowering:\n%s", ir)
	}
}

func TestSemanticLocalStorageMatchesTextualPlacement(t *testing.T) {
	ir, err := llvmobject.ExperimentalIR("locals.mg", func(backend lb.Backend) error {
		state, _ := semanticState()
		lowerer, _ := loweringtypes.New(backend, state)
		i64, _ := lowerer.Lower(primitive("i64"))
		function, _ := backend.DeclareFunction(lb.FunctionSpec{Symbol: "locals", Result: i64, Parameters: []lb.TypeID{i64}, Linkage: lb.LinkageInternal, Definition: true})
		entry, _ := backend.AppendBlock(function, "entry")
		body, _ := backend.AppendBlock(function, "body")
		argument, err := lowerer.MaterializeArgument(function, entry, 0, primitive("i64"))
		if err != nil {
			return err
		}
		if err := backend.Branch(entry, body); err != nil {
			return err
		}
		local, err := lowerer.CreateLocal(function, body, primitive("i64"))
		if err != nil {
			return err
		}
		value, err := backend.Load(body, i64, argument, 0, false)
		if err != nil {
			return err
		}
		if _, err := backend.Store(body, value, local, 0, false); err != nil {
			return err
		}
		result, err := backend.Load(body, i64, local, 0, false)
		if err != nil {
			return err
		}
		if err := backend.Return(body, result); err != nil {
			return err
		}
		return backend.FinalizeFunction(function)
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(ir)
	entryEnd := strings.Index(text, "br label %body")
	bodyStart := strings.Index(text, "body:")
	if entryEnd < 0 || bodyStart < 0 || strings.Count(text[:entryEnd], "alloca i64") != 2 {
		t.Fatalf("argument and local slots were not hoisted to entry:\n%s", text)
	}
	if !strings.Contains(text[bodyStart:], "store i64 0") {
		t.Fatalf("local zero-initialization did not remain at declaration point:\n%s", text)
	}
	if strings.Contains(text, "llvm.lifetime") {
		t.Fatalf("object lowering added lifetime behavior absent from textual IR:\n%s", text)
	}
}
