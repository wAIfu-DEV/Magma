//go:build llvm_object

package loweringruntime_test

import (
	"bytes"
	"strings"
	"testing"

	llvmfragments "Magma/src/llvm_fragments"
	llvmobject "Magma/src/llvm_object"
	lb "Magma/src/lowering_backend"
	loweringruntime "Magma/src/lowering_runtime"
)

func TestBuildUtilsProducesCanonicalObjectIR(t *testing.T) {
	ir, err := llvmobject.RuntimeUtilsIR()
	if err != nil {
		t.Fatal(err)
	}
	text := string(ir)
	for _, fragment := range []string{"define internal %type.slice @magma.argsToSlice", "phi i64", "call i64 @strlen", "sext i32", "ret %type.slice"} {
		if !strings.Contains(text, fragment) {
			t.Errorf("object runtime IR lacks %q:\n%s", fragment, text)
		}
	}
}

func TestBuildUtilsRejectsDuplicateDefinition(t *testing.T) {
	_, err := llvmobject.ExperimentalIR("runtime-utils-duplicates.mg", func(backend lb.Backend) error {
		if _, err := loweringruntime.BuildUtils(backend); err != nil {
			return err
		}
		_, err := loweringruntime.BuildUtils(backend)
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate definition") {
		t.Fatalf("expected structural duplicate-definition error, got %v", err)
	}
}

func TestTextualCompatibilityFragmentIsGeneratedFromObjects(t *testing.T) {
	generated, err := llvmobject.RuntimeUtilsFragmentIR()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.TrimSpace(generated), bytes.TrimSpace(llvmfragments.Utils)) {
		t.Fatalf("src/llvm_fragments/utils.ll is stale; generated fragment:\n%s", generated)
	}
}
