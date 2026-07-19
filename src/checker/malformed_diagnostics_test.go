package checker_test

import (
	"Magma/src/checker"
	"Magma/src/comp_err"
	"Magma/src/join"
	llvmir "Magma/src/llvm_ir"
	"Magma/src/monomorph"
	"Magma/src/pipeline"
	"Magma/src/shared"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// compileMalformed runs the same front-end stages as the compiler and returns
// the stage which first rejected the input. These tests intentionally avoid
// asserting diagnostic prose: their purpose is to make the current errors easy
// to inspect with `go test ./src/checker -run TestMalformedProgramsDiagnostics -v`.
func compileMalformed(t *testing.T, source string) (string, error) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "malformed.mg")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}

	state, err := shared.MakeShared(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = pipeline.DoMain(state, path); err != nil {
		return "parse", err
	}
	if err = join.JoinCompilationUnits(state, nil); err != nil {
		return "imports", err
	}
	if err = monomorph.Run(state); err != nil {
		return "monomorph", err
	}
	if err = checker.CheckLinks(state); err != nil {
		return "link", err
	}
	if err = checker.TypeChecker(state); err != nil {
		return "type", err
	}
	if _, err = llvmir.IrWrite(state); err != nil {
		return "IR", err
	}
	return "accepted", nil
}

func observedDiagnostic(err error) string {
	var diagnostic *comp_err.CompilationError
	if errors.As(err, &diagnostic) {
		additional := ""
		if diagnostic.Additional != "" {
			additional = " | " + diagnostic.Additional
		}
		return fmt.Sprintf(
			"structured l%d:c%d token=%q: %s%s",
			diagnostic.Token.Pos.Line,
			diagnostic.Token.Pos.Col,
			diagnostic.Token.Repr,
			diagnostic.ShortDesc,
			additional,
		)
	}
	return fmt.Sprintf("raw %T: %v", err, err)
}

func TestMalformedProgramsDiagnostics(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name: "missing module declaration",
			source: `Thing()
`,
		},
		{
			name: "missing call argument separator",
			source: `mod test

consume(value u64) void:
..

test() void:
    consume(1 2)
..
`,
		},
		{
			name: "unknown variable",
			source: `mod test

test() void:
    missing = 1
..
`,
		},
		{
			name: "unknown function",
			source: `mod test

test() void:
    missing()
..
`,
		},
		{
			name: "unknown declared type",
			source: `mod test

test() void:
    value Missing
..
`,
		},
		{
			name: "unknown struct member",
			source: `mod test

Thing()

test() void:
    value Thing
    value.missing()
..
`,
		},
		{
			name: "call non-function field",
			source: `mod test

Thing(
    count u64
)

test() void:
    value Thing
    value.count()
..
`,
		},
		{
			name: "wrong argument count",
			source: `mod test

consume(value u64) void:
..

test() void:
    consume()
..
`,
		},
		{
			name: "return wrong type",
			source: `mod test

value() u64:
    ret true
..
`,
		},
		{
			name: "assign wrong type",
			source: `mod test

test() void:
    value u64
    value = true
..
`,
		},
		{
			name: "subscript scalar",
			source: `mod test

test() void:
    value u64
    value[0] = 1
..
`,
		},
		{
			name: "member access on scalar",
			source: `mod test

test() void:
    value u64
    value.missing()
..
`,
		},
		{
			name: "try non-throwing function",
			source: `mod test

noop() void:
..

test() void:
    try noop()
..
`,
		},
	}
	type expectation struct {
		stage   string
		token   string
		message string
	}
	expected := map[string]expectation{
		"missing module declaration":      {"parse", "", "expected module name declaration"},
		"missing call argument separator": {"parse", "2", "unexpected '2'"},
		"unknown variable":                {"link", "missing", "unknown variable 'missing'"},
		"unknown function":                {"link", "missing", "unknown function 'missing'"},
		"unknown declared type":           {"link", "Missing", "unknown type 'Missing'"},
		"unknown struct member":           {"link", "missing", "has no member named 'missing'"},
		"call non-function field":         {"type", "count", "non-function type 'u64'"},
		"wrong argument count":            {"type", "consume", "expects 1 argument(s), but got 0"},
		"return wrong type":               {"type", "ret", "cannot return value of type 'bool'"},
		"assign wrong type":               {"type", "=", "cannot assign value of type 'bool'"},
		"subscript scalar":                {"type", "[", "cannot index value of type 'u64'"},
		"member access on scalar":         {"link", "missing", "type 'u64' has no member function 'missing'"},
		"try non-throwing function":       {"type", "try", "cannot use 'try' with non-throwing call"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stage, err := compileMalformed(t, test.source)
			if err == nil {
				t.Fatalf("malformed program was unexpectedly accepted through IR generation")
			}
			want := expected[test.name]
			if stage != want.stage {
				t.Fatalf("diagnostic stage = %q, want %q: %s", stage, want.stage, observedDiagnostic(err))
			}
			var diagnostic *comp_err.CompilationError
			if !errors.As(err, &diagnostic) {
				t.Fatalf("expected structured diagnostic, got %s", observedDiagnostic(err))
			}
			if diagnostic.Token.Repr != want.token {
				t.Errorf("diagnostic token = %q, want %q", diagnostic.Token.Repr, want.token)
			}
			if !strings.Contains(diagnostic.ShortDesc, want.message) {
				t.Errorf("diagnostic = %q, want it to contain %q", diagnostic.ShortDesc, want.message)
			}
			t.Logf("[%s] %s", stage, observedDiagnostic(err))
		})
	}
}
