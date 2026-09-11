//go:build llvm_object

package loweringcabi_test

import (
	"testing"

	llvmobject "Magma/src/llvm_object"
	lb "Magma/src/lowering_backend"
	loweringcabi "Magma/src/lowering_cabi"
	loweringtypes "Magma/src/lowering_types"
	magmatarget "Magma/src/target"
	mt "Magma/src/types"
)

func TestSysVClassifiesFloatPairAsVector(t *testing.T) {
	backend, err := llvmobject.NewLoweringBackend("cabi.classify")
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	if err := backend.ConfigureModule(lb.ModuleSpec{TargetTriple: "x86_64-pc-linux-gnu", DataLayout: "e-m:e-p:64:64-i64:64-n8:16:32:64-S128"}); err != nil {
		t.Fatal(err)
	}
	f32 := named("f32")
	definition := &mt.StructDef{Module: "probe", Name: "Vector2", FieldOrder: []string{"x", "y"}, Fields: map[string]*mt.NodeType{"x": f32, "y": f32}}
	typeNode := &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "probe.Vector2"}}
	state := &mt.SharedState{
		Files:  map[string]*mt.FileCtx{"probe.mg": {GlNode: &mt.NodeGlobal{StructDefs: map[string]*mt.StructDef{"Vector2": definition}}}},
		Target: magmatarget.Target{Arch: "x86_64", OS: "linux", PointerBits: 64},
	}
	types, err := loweringtypes.New(backend, state)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := loweringcabi.Classify(backend, types, state, typeNode, false)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Class != loweringcabi.Coerce || len(plan.Parts) != 1 {
		t.Fatalf("Vector2 plan = %#v", plan)
	}
	physical, err := loweringcabi.PhysicalResult(backend, plan)
	if err != nil {
		t.Fatal(err)
	}
	layout, err := backend.TypeLayout(physical)
	if err != nil {
		t.Fatal(err)
	}
	if layout.StoreSize != 8 {
		t.Fatalf("physical Vector2 size = %d", layout.StoreSize)
	}
}

func TestAggregateClassificationAcrossSupportedABIs(t *testing.T) {
	tests := []struct {
		name      string
		target    magmatarget.Target
		wantClass loweringcabi.Class
		wantByVal bool
		wantSRet  bool
		wantError bool
	}{
		{name: "sysv argument", target: magmatarget.Target{Arch: "x86_64", OS: "linux", PointerBits: 64}, wantClass: loweringcabi.Indirect, wantByVal: true},
		{name: "windows return", target: magmatarget.Target{Arch: "x86_64", OS: "windows", PointerBits: 64}, wantClass: loweringcabi.Indirect, wantSRet: true},
		{name: "aarch64 return", target: magmatarget.Target{Arch: "aarch64", OS: "darwin", PointerBits: 64}, wantClass: loweringcabi.Indirect, wantSRet: true},
		{name: "unsupported target stub", target: magmatarget.Target{Arch: "riscv64", OS: "linux", PointerBits: 64}, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			backend, err := llvmobject.NewLoweringBackend("cabi.large")
			if err != nil {
				t.Fatal(err)
			}
			defer backend.Close()
			if err := backend.ConfigureModule(lb.ModuleSpec{DataLayout: "e-p:64:64-i64:64"}); err != nil {
				t.Fatal(err)
			}
			u64 := named("u64")
			definition := &mt.StructDef{Module: "probe", Name: "Large", FieldOrder: []string{"a", "b", "c"}, Fields: map[string]*mt.NodeType{"a": u64, "b": u64, "c": u64}}
			node := &mt.NodeType{KindNode: &mt.NodeTypeAbsolute{AbsoluteName: "probe.Large"}}
			state := &mt.SharedState{Files: map[string]*mt.FileCtx{"probe.mg": {GlNode: &mt.NodeGlobal{StructDefs: map[string]*mt.StructDef{"Large": definition}}}}, Target: test.target}
			types, err := loweringtypes.New(backend, state)
			if err != nil {
				t.Fatal(err)
			}
			plan, err := loweringcabi.Classify(backend, types, state, node, test.wantSRet)
			if test.wantError {
				if err == nil {
					t.Fatalf("unsupported target classification unexpectedly succeeded: %#v", plan)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if plan.Class != test.wantClass || plan.ByValue != test.wantByVal || plan.StructRet != test.wantSRet {
				t.Fatalf("classification = %#v", plan)
			}
		})
	}
}

func named(name string) *mt.NodeType {
	return &mt.NodeType{KindNode: &mt.NodeTypeNamed{NameNode: &mt.NodeNameSingle{Name: name}}}
}
