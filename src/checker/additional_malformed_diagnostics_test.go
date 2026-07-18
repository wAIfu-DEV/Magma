package checker_test

import (
	"Magma/src/comp_err"
	"errors"
	"strings"
	"testing"
)

// TestAdditionalMalformedProgramsDiagnostics is an exploratory corpus for
// diagnostics not covered by TestMalformedProgramsDiagnostics. A malformed
// program being accepted is logged as a semantic gap so the entire corpus can
// be inspected in one verbose run before expectations are tightened.
func TestAdditionalMalformedProgramsDiagnostics(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name: "duplicate struct field declaration",
			source: `mod test

Thing(
    count u64
    count bool
)
`,
		},
		{
			name: "unknown constructor field",
			source: `mod test

Thing(
    count u64
)

test() void:
    value := Thing(unknown=1)
..
`,
		},
		{
			name: "missing constructor field",
			source: `mod test

Thing(
    count u64
    enabled bool
)

test() void:
    value := Thing(count=1)
..
`,
		},
		{
			name: "duplicate constructor field",
			source: `mod test

Thing(
    count u64
)

test() void:
    value := Thing(count=1, count=2)
..
`,
		},
		{
			name: "wrong constructor field type",
			source: `mod test

Thing(
    count u64
)

test() void:
    value := Thing(count=true)
..
`,
		},
		{
			name: "constructor on intrinsic type",
			source: `mod test

test() void:
    value := u64(item=1)
..
`,
		},
		{
			name: "numeric if condition",
			source: `mod test

test() void:
    if 1:
    ..
..
`,
		},
		{
			name: "string while condition",
			source: `mod test

test() void:
    while "yes":
    ..
..
`,
		},
		{
			name: "throw non-error value",
			source: `mod test

test() !void:
    throw 1
..
`,
		},
		{
			name: "return value from void function",
			source: `mod test

test() void:
    ret 1
..
`,
		},
		{
			name: "bare return from value function",
			source: `mod test

test() u64:
    ret
..
`,
		},
		{
			name: "dereference non-pointer",
			source: `mod test

test() void:
    value := *true
..
`,
		},
		{
			name: "bitwise not string",
			source: `mod test

test() void:
    value := ~"hello"
..
`,
		},
		{
			name: "logical operator with integer",
			source: `mod test

test() void:
    value := true && 1
..
`,
		},
		{
			name: "arithmetic on booleans",
			source: `mod test

test() void:
    value := true + false
..
`,
		},
		{
			name: "comparison of unrelated types",
			source: `mod test

test() void:
    value := "hello" == 1
..
`,
		},
		{
			name: "wrong call argument type",
			source: `mod test

consume(value u64) void:
..

test() void:
    consume(true)
..
`,
		},
		{
			name: "destructure non-throwing call",
			source: `mod test

value() u64:
    ret 1
..

test() void:
    result, err := value()
..
`,
		},
		{
			name: "destructure throwing void call",
			source: `mod test

work() !void:
    ret
..

test() void:
    result, err := work()
..
`,
		},
		{
			name: "destructure into non-error binding",
			source: `mod test

value() !u64:
    ret 1
..

test() void:
    result u64, err u64 = value()
..
`,
		},
	}

	if len(tests) != 20 {
		t.Fatalf("additional malformed corpus has %d cases, want 20", len(tests))
	}
	type expectation struct {
		stage   string
		token   string
		message string
	}
	expected := map[string]expectation{
		"duplicate struct field declaration": {"parse", "count", "duplicate field 'count' in struct 'Thing'"},
		"unknown constructor field":          {"link", "unknown", "type 'Thing' has no field named 'unknown'"},
		"missing constructor field":          {"link", "Thing", "missing field 'enabled'"},
		"duplicate constructor field":        {"link", "count", "duplicate field 'count'"},
		"wrong constructor field type":       {"link", "count", "expects type 'u64'"},
		"constructor on intrinsic type":      {"link", "u64", "non-struct type 'u64'"},
		"numeric if condition":               {"type", "if", "if condition must have type 'bool'"},
		"string while condition":             {"type", "while", "while condition must have type 'bool'"},
		"throw non-error value":              {"type", "throw", "cannot throw value of type 'i64'"},
		"return value from void function":    {"type", "ret", "cannot return a value"},
		"bare return from value function":    {"type", "ret", "missing return value"},
		"dereference non-pointer":            {"link", "*", "cannot dereference value of non-pointer type 'bool'"},
		"bitwise not string":                 {"link", "~", "bitwise not requires"},
		"logical operator with integer":      {"link", "&&", "requires 'bool' operands"},
		"arithmetic on booleans":             {"link", "+", "requires numeric operands"},
		"comparison of unrelated types":      {"link", "==", "unrelated types 'str' and 'i64'"},
		"wrong call argument type":           {"type", "true", "argument 1 to 'consume' expects type 'u64'"},
		"destructure non-throwing call":      {"link", "value", "cannot destructure non-throwing call"},
		"destructure throwing void call":     {"link", "work", "cannot bind a result value"},
		"destructure into non-error binding": {"link", "value", "error binding must have type 'error'"},
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
