package magmatypes

import (
	"strings"
	"testing"
)

func TestErrorTraceTypesComeFromMagmaCore(t *testing.T) {
	var ir strings.Builder
	WriteIrBasicTypes(&ir)
	text := ir.String()
	for _, legacy := range []string{
		"%type.error.site", "%type.error.trace.node",
		"%type.error.trace.shard", "%type.error.trace.snapshot",
	} {
		if strings.Contains(text, legacy) {
			t.Errorf("basic types still contain legacy trace type %q", legacy)
		}
	}
}

func TestSourceDefinedRepresentationsAreNotDuplicatedByBasicTypes(t *testing.T) {
	var ir strings.Builder
	WriteIrBasicTypes(&ir)
	for _, name := range []string{"%type.str = type", "%type.error = type", "%type.slice = type"} {
		if strings.Contains(ir.String(), name) {
			t.Fatalf("basic types unexpectedly contain source-defined representation %q", name)
		}
	}
}

func TestBasicTypesDeclareBothErrorPredicateWeightPolarities(t *testing.T) {
	var ir strings.Builder
	WriteIrBasicTypes(&ir)
	text := ir.String()
	for _, weights := range []string{
		`!9000 = !{!"branch_weights", i32 1, i32 2000}`,
		`!9001 = !{!"branch_weights", i32 2000, i32 1}`,
	} {
		if !strings.Contains(text, weights) {
			t.Fatalf("basic IR metadata missing %q:\n%s", weights, text)
		}
	}
}
