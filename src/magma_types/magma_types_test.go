package magmatypes

import (
	"strings"
	"testing"
)

func TestErrorTypesUseCompactFields(t *testing.T) {
	var ir strings.Builder
	WriteIrBasicTypes(&ir)
	text := ir.String()
	for _, want := range []string{
		"%type.error = type { ptr, i32, i16, i16 }",
		"%type.error.trace.node = type { i32, i16, ptr }",
		"%type.error.trace.snapshot = type { ptr, i16, i1 }",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("basic types do not contain %q", want)
		}
	}
}
