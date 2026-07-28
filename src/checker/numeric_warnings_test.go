package checker_test

import (
	"Magma/src/checker"
	"Magma/src/join"
	"Magma/src/monomorph"
	"Magma/src/pipeline"
	"Magma/src/shared"
	"Magma/src/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNarrowingConversionsProduceWarnings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.mg")
	source := `mod main

consume(value i16) void:
    ret
..

narrow(value i64) i16:
    local i8 = value
    consume(value)
    ret value
..

mask(value u32) u32:
    ret value | 0x80000000
..
`
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	state, err := shared.MakeShared(dir, filepath.Join("..", "..", "std"))
	if err == nil {
		err = pipeline.DoMain(state, path)
	}
	if err = join.JoinCompilationUnits(state, err); err == nil {
		err = monomorph.Run(state)
	}
	if err == nil {
		err = checker.CheckLinks(state)
	}
	if err == nil {
		err = checker.TypeChecker(state)
	}
	if err != nil {
		t.Fatalf("type check: %v", err)
	}
	warnings := []types.Warning{}
	for _, warning := range state.Warnings {
		if warning.FilePath == path {
			warnings = append(warnings, warning)
		}
	}
	if len(warnings) != 3 {
		t.Fatalf("warnings = %+v, want variable, argument, and return warnings", warnings)
	}
	for _, context := range []string{"variable initialization", "argument 1", "return value"} {
		found := false
		for _, warning := range warnings {
			found = found || strings.Contains(warning.Message, context)
		}
		if !found {
			t.Errorf("missing %s warning: %+v", context, warnings)
		}
	}
}
