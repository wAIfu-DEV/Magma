package llvmfragments

import (
	"strings"
	"testing"
)

func TestRenderUtilsConfiguresTraceRing(t *testing.T) {
	ir, err := RenderUtils(2048)
	if err != nil {
		t.Fatal(err)
	}
	text := string(ir)
	for _, want := range []string{
		"%type.error.trace.arena = type { [2048 x %type.error.trace.node], [0 x i8] }",
		"and i64 %ticket, 2047",
		"--error-trace-slots=2048",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("rendered runtime does not contain %q", want)
		}
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
		"magma.error.trace.capacity",
		"magma.error.push",
		"magma.error.trace",
		"magma.error.printTrace",
		"magma.error.print",
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
	for _, slots := range []uint64{0, 3, 65537} {
		if _, err := RenderUtils(slots); err == nil {
			t.Errorf("RenderUtils(%d) succeeded", slots)
		}
	}
}
