package llvmfragments

import (
	"strings"
	"testing"
)

func TestRenderUtilsLeavesTraceToMagmaCore(t *testing.T) {
	ir, err := RenderUtils(1024)
	if err != nil {
		t.Fatal(err)
	}
	text := string(ir)
	if strings.Contains(text, "magma.error.trace") || strings.Contains(text, "magma.error.push") {
		t.Fatal("legacy LLVM trace implementation remains in utils fragment")
	}
	if strings.Contains(text, "{{TRACE_") {
		t.Fatal("rendered runtime still contains a template token")
	}
}

func TestRuntimeDefinitionsHaveInternalLinkage(t *testing.T) {
	ir, err := RenderUtils(1024)
	if err != nil {
		t.Fatal(err)
	}
	text := string(ir)
	for _, name := range []string{
		"magma.argsToSlice",
	} {
		internal := false
		for _, line := range strings.Split(text, "\n") {
			if strings.Contains(line, "@"+name+"(") {
				internal = strings.HasPrefix(line, "define internal ")
				break
			}
		}
		if !internal {
			t.Fatalf("runtime helper %q is missing or does not have internal linkage", name)
		}
	}
	if strings.Contains(text, "\ndefine i64 @magma.") ||
		strings.Contains(text, "\ndefine i32 @magma.") ||
		strings.Contains(text, "\ndefine void @magma.") ||
		strings.Contains(text, "\ndefine %type.") {
		t.Fatal("runtime fragment contains an externally visible definition")
	}
}

func TestRenderUtilsRejectsInvalidTraceSlots(t *testing.T) {
	for _, slots := range []uint64{0, 3, 2048} {
		if _, err := RenderUtils(slots); err == nil {
			t.Errorf("RenderUtils(%d) succeeded", slots)
		}
	}
}
