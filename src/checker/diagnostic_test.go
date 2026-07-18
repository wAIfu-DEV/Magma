package checker_test

import (
	"Magma/src/checker"
	"Magma/src/comp_err"
	"Magma/src/join"
	"Magma/src/monomorph"
	"Magma/src/pipeline"
	"Magma/src/shared"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func checkSource(t *testing.T, source string) (*comp_err.CompilationError, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.mg")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}

	state, err := shared.MakeShared(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = pipeline.DoMain(state, path); err == nil {
		err = join.JoinCompilationUnits(state, nil)
	}
	if err == nil {
		err = monomorph.Run(state)
	}
	if err == nil {
		err = checker.CheckLinks(state)
	}
	if err == nil {
		err = checker.TypeChecker(state)
	}
	if err == nil {
		t.Fatal("expected compilation to fail")
	}

	var diagnostic *comp_err.CompilationError
	if !errors.As(err, &diagnostic) {
		t.Fatalf("expected a structured compilation error, got %T: %v", err, err)
	}
	return diagnostic, err.Error()
}

func TestMissingStructMemberDiagnostic(t *testing.T) {
	diagnostic, message := checkSource(t, `mod test

Thing()

test() void:
    value Thing
    value.fake()
..
`)

	if want := "type 'Thing' has no member named 'fake'"; !strings.Contains(message, want) {
		t.Fatalf("diagnostic = %q, want it to contain %q", message, want)
	}
	if diagnostic.Token.Repr != "fake" {
		t.Fatalf("diagnostic token = %q, want %q", diagnostic.Token.Repr, "fake")
	}
	if diagnostic.Token.Pos.Line != 7 {
		t.Fatalf("diagnostic line = %d, want 7", diagnostic.Token.Pos.Line)
	}
}

func TestCallingNonFunctionFieldUsesSourceDiagnostic(t *testing.T) {
	diagnostic, message := checkSource(t, `mod test

Thing(
    count u64
)

test() void:
    value Thing
    value.count()
..
`)

	if !strings.Contains(message, "non-function type 'u64'") {
		t.Fatalf("diagnostic = %q, want a user-facing non-function type", message)
	}
	if strings.Contains(message, "types.NodeType") {
		t.Fatalf("diagnostic leaks an internal Go type: %q", message)
	}
	if diagnostic.Token.Repr != "count" {
		t.Fatalf("diagnostic token = %q, want %q", diagnostic.Token.Repr, "count")
	}
	if diagnostic.Token.Pos.Line != 9 {
		t.Fatalf("diagnostic line = %d, want 9", diagnostic.Token.Pos.Line)
	}
}

func TestFormerCrashPathsReturnSourceDiagnostics(t *testing.T) {
	tests := map[string]string{
		"constant member assignment": `mod test
Point(x u64)
const ORIGIN := Point(x=0)
test() void:
    ORIGIN.x = 1
..
`,
		"break outside loop": `mod test
test() void:
    break
..
`,
		"unsupported unary address": `mod test
test() void:
    value u64 = 1
    pointer u64* = &value
..
`,
		"invalid hexadecimal literal": `mod test
test() void:
    value u64 = 0xGG
..
`,
		"invalid array length": `mod test
test() void:
    value u64[abc]
..
`,
		"duplicate function": `mod test
work() void:
..
work() void:
..
`,
		"duplicate parameter": `mod test
consume(value u64, value u64) void:
..
`,
	}

	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			diagnostic, message := checkSource(t, source)
			if diagnostic.Token.Pos.Line == 0 || diagnostic.Token.Pos.Col == 0 {
				t.Fatalf("diagnostic has no source position: %#v", diagnostic.Token.Pos)
			}
			for _, internal := range []string{"panic:", "uncaught fatal error", "Clang failed", "unknown generic struct template"} {
				if strings.Contains(message, internal) {
					t.Fatalf("diagnostic leaks internal failure %q: %s", internal, message)
				}
			}
		})
	}
}
