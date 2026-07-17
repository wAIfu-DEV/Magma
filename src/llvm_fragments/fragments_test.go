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

func TestRenderUtilsRejectsInvalidTraceSlots(t *testing.T) {
	for _, slots := range []uint64{0, 3, 65537} {
		if _, err := RenderUtils(slots); err == nil {
			t.Errorf("RenderUtils(%d) succeeded", slots)
		}
	}
}
