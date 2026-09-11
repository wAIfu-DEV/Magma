//go:build llvm_object

package llvmobject

import (
	"fmt"
	"strings"
	"testing"

	lb "Magma/src/lowering_backend"
	mt "Magma/src/types"
)

func TestIndependentModulesBuildConcurrentlyThenMerge(t *testing.T) {
	type result struct {
		module *Module
		err    error
	}
	results := make(chan result, 8)
	for i := 0; i < 8; i++ {
		go func(index int) {
			module, err := NewModule(fmt.Sprintf("unit-%02d.mg", index))
			if err == nil {
				void, typeErr := module.VoidType()
				err = typeErr
				if err == nil {
					function, functionErr := module.NewFunction(fmt.Sprintf("unit_%02d", index), void)
					err = functionErr
					if err == nil {
						block, _ := module.NewBlock(function, "entry")
						builder, _ := module.BuilderAt(block)
						err = builder.RetVoid()
					}
				}
			}
			results <- result{module: module, err: err}
		}(i)
	}
	units := make([]*Module, 0, 8)
	for range 8 {
		result := <-results
		if result.err != nil {
			t.Fatal(result.err)
		}
		units = append(units, result.module)
		defer result.module.Close()
	}
	merged, err := MergeModules("concurrent-program", units)
	if err != nil {
		t.Fatal(err)
	}
	defer merged.Close()
	if err := merged.Verify(); err != nil {
		t.Fatal(err)
	}
}

func unitWithVoidFunction(t *testing.T, moduleName, functionName string) *Module {
	t.Helper()
	module, err := NewModule(moduleName)
	if err != nil {
		t.Fatal(err)
	}
	void, _ := module.VoidType()
	function, _ := module.NewFunction(functionName, void)
	block, _ := module.NewBlock(function, "entry")
	builder, _ := module.BuilderAt(block)
	if err := builder.RetVoid(); err != nil {
		module.Close()
		t.Fatal(err)
	}
	return module
}

func TestMergeModulesIsDeterministicAcrossInputOrder(t *testing.T) {
	a := unitWithVoidFunction(t, "a.mg", "alpha")
	defer a.Close()
	b := unitWithVoidFunction(t, "b.mg", "beta")
	defer b.Close()
	first, err := MergeModules("program", []*Module{b, a})
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := MergeModules("program", []*Module{a, b})
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	firstIR, _ := first.String()
	secondIR, _ := second.String()
	if firstIR != secondIR {
		t.Fatalf("merge depends on input order:\n%s\n---\n%s", firstIR, secondIR)
	}
	if strings.Index(firstIR, "@alpha") > strings.Index(firstIR, "@beta") {
		t.Fatalf("units were not linked in canonical order:\n%s", firstIR)
	}
}

func TestBitcodeRoundTripAndDeterministicMerge(t *testing.T) {
	a := unitWithVoidFunction(t, "a.mg", "alpha")
	b := unitWithVoidFunction(t, "b.mg", "beta")
	a.configure("x86_64-unknown-linux-gnu", "")
	b.configure("x86_64-unknown-linux-gnu", "")
	aBytes, err := a.Bitcode()
	if err != nil {
		t.Fatal(err)
	}
	bBytes, err := b.Bitcode()
	if err != nil {
		t.Fatal(err)
	}
	a.Close()
	b.Close()

	first, err := MergeBitcode("program", []BitcodeUnit{{Name: "b", Data: bBytes}, {Name: "a", Data: aBytes}})
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := MergeBitcode("program", []BitcodeUnit{{Name: "a", Data: aBytes}, {Name: "b", Data: bBytes}})
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	firstIR, _ := first.String()
	secondIR, _ := second.String()
	if firstIR != secondIR {
		t.Fatalf("cached bitcode merge depends on input order:\n%s\n---\n%s", firstIR, secondIR)
	}
}

func TestMergeBitcodeRejectsInvalidCacheEntries(t *testing.T) {
	if _, err := MergeBitcode("program", []BitcodeUnit{{Name: "unit", Data: []byte("not bitcode")}}); err == nil {
		t.Fatal("corrupt bitcode was accepted")
	}
	unit := unitWithVoidFunction(t, "unit.mg", "unit")
	data, err := unit.Bitcode()
	unit.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := MergeBitcode("program", []BitcodeUnit{{Name: "same", Data: data}, {Name: "same", Data: data}}); err == nil {
		t.Fatal("duplicate stable unit names were accepted")
	}
}

func TestExperimentalIRProvidesDirectNeutralEntryPoint(t *testing.T) {
	ir, err := ExperimentalIR("embed.mg", func(backend lb.Backend) error {
		void, err := backend.InternType(lb.TypeSpec{Kind: lb.TypeVoid})
		if err != nil {
			return err
		}
		function, err := backend.DeclareFunction(lb.FunctionSpec{Symbol: "embedded", Result: void, Linkage: lb.LinkageInternal, Definition: true})
		if err != nil {
			return err
		}
		block, err := backend.AppendBlock(function, "entry")
		if err != nil {
			return err
		}
		if err := backend.ReturnVoid(block); err != nil {
			return err
		}
		return backend.FinalizeFunction(function)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ir), "define internal void @embedded") {
		t.Fatalf("missing embedded function:\n%s", ir)
	}
}

func TestVerificationFailureIsCompilerBugWithSource(t *testing.T) {
	module, _ := NewModule("broken.mg")
	defer module.Close()
	void, _ := module.VoidType()
	function, _ := module.NewFunction("broken", void)
	_, _ = module.NewBlock(function, "entry")
	token := &mt.Token{Pos: mt.FilePos{Line: 7, Col: 11}}
	err := module.VerifyAt(ErrorContext{Operation: "finalize function", Function: "broken", Token: token})
	backend, ok := err.(*BackendError)
	if !ok || !backend.CompilerBug {
		t.Fatalf("verification error was not classified as compiler bug: %#v", err)
	}
	if !strings.Contains(err.Error(), "at 7:11") {
		t.Fatalf("verification error lacks source position: %v", err)
	}
}
