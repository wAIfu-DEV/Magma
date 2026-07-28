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

	state, err := shared.MakeShared(dir, filepath.Join("..", "..", "std"))
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

func TestTypeDiagnosticUsesSourceName(t *testing.T) {
	_, message := checkSource(t, `mod test

Box[T](value T)

fail(value Box[u64]*) !void:
    throw value
..
`)

	if want := "cannot throw value of type 'Box[u64]*'"; !strings.Contains(message, want) {
		t.Fatalf("diagnostic = %q, want it to contain %q", message, want)
	}
	if strings.Contains(message, "__g__") || strings.Contains(message, "test_") {
		t.Fatalf("diagnostic exposes an internal type name: %q", message)
	}
}

func TestFunctionDeclarationCannotBeAssigned(t *testing.T) {
	diagnostic, message := checkSource(t, `mod test

identity(value u64) u64:
    ret value
..

main() void:
    identity = identity
..
`)
	if !strings.Contains(message, "cannot assign to function 'identity'") {
		t.Fatalf("diagnostic = %q, want immutable function declaration error", message)
	}
	if diagnostic.Additional == "" {
		t.Fatal("function assignment diagnostic should explain declaration immutability")
	}
}

func TestVoidCannotBeUsedAsVariableType(t *testing.T) {
	diagnostic, message := checkSource(t, `mod test

main() void:
    value void
..
`)
	if !strings.Contains(message, "void cannot be used as a variable type") {
		t.Fatalf("diagnostic = %q, want void value-type rejection", message)
	}
	if diagnostic.Token.Repr != "void" || diagnostic.Additional == "" {
		t.Fatalf("diagnostic should identify and explain void, got token %q and guidance %q", diagnostic.Token.Repr, diagnostic.Additional)
	}
}

func TestVoidPointerIsRejectedInFavorOfPtr(t *testing.T) {
	diagnostic, message := checkSource(t, `mod test

main() void:
    value void*
..
`)
	if !strings.Contains(message, "void cannot be used as a pointer element type") {
		t.Fatalf("diagnostic = %q, want void pointer rejection", message)
	}
	if !strings.Contains(diagnostic.Additional, "use 'ptr'") {
		t.Fatalf("diagnostic guidance = %q, want canonical ptr spelling", diagnostic.Additional)
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

func TestFunctionArgumentMismatchShowsCompleteSignatures(t *testing.T) {
	diagnostic, message := checkSource(t, `mod test

consume(callback (ptr, u64*) !u8) void:
..

next(impl ptr, index u64*) u8:
    ret 0
..

test() void:
    consume(next)
..
`)

	want := "expects type '(ptr, u64*) !u8', but got '(ptr, u64*) u8'"
	if !strings.Contains(message, want) {
		t.Fatalf("diagnostic = %q, want it to contain %q", message, want)
	}
	if !strings.Contains(diagnostic.Additional, "throwing and non-throwing function pointers are not interchangeable") {
		t.Fatalf("additional diagnostic = %q, want function pointer return guidance", diagnostic.Additional)
	}
	if diagnostic.Token.Repr != "next" {
		t.Fatalf("diagnostic token = %q, want %q", diagnostic.Token.Repr, "next")
	}
}

func TestGenericCallDiagnosticUsesDisplayName(t *testing.T) {
	_, message := checkSource(t, `mod test

consume[T](value T) void:
..

test() void:
    consume[u8]()
..
`)

	if !strings.Contains(message, "function 'consume[u8]' expects 1 argument(s), but got 0") {
		t.Fatalf("diagnostic does not use the source-level generic name: %q", message)
	}
	if strings.Contains(message, "__g__") {
		t.Fatalf("diagnostic exposes a mangled function name: %q", message)
	}
}

func TestNamedFunctionArgumentsAreRejected(t *testing.T) {
	diagnostic, message := checkSource(t, `mod test

consume(value u64) void:
..

test() void:
    consume(value=1)
..
`)

	if !strings.Contains(message, "named arguments are not supported in function calls") {
		t.Fatalf("diagnostic = %q, want named-argument rejection", message)
	}
	if diagnostic.Additional == "" {
		t.Fatal("named-argument diagnostic should explain positional syntax")
	}
}

func TestVoidReturningCallCannotBeUsedAsValue(t *testing.T) {
	_, message := checkSource(t, `mod test

work() void:
..

test() void:
    value := work()
..
`)

	if !strings.Contains(message, "cannot use void-returning call 'work' as a value") {
		t.Fatalf("diagnostic = %q, want void-value rejection", message)
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
