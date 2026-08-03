package compilerpipeline

import (
	"Magma/src/shared"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testProgram(t *testing.T, source string) (*ParsedProgram, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "main.mg")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	stdRoot, err := filepath.Abs(filepath.Join("..", "..", "std"))
	if err != nil {
		t.Fatal(err)
	}
	state, err := shared.MakeShared(dir, stdRoot)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(state, path)
	if err != nil {
		t.Fatalf("parse pipeline: %v", err)
	}
	return &parsed, path
}

func TestStagesPreserveProgramIdentityAndLower(t *testing.T) {
	parsed, path := testProgram(t, "mod main\nmain() void:\n..\n")
	if err := RequireMainModule(*parsed, path); err != nil {
		t.Fatal(err)
	}
	specialized, err := Specialize(*parsed)
	if err != nil {
		t.Fatal(err)
	}
	linked, err := Link(specialized)
	if err != nil {
		t.Fatal(err)
	}
	typed, err := CheckTypes(linked)
	if err != nil {
		t.Fatal(err)
	}
	validated, err := ValidateLowering(typed)
	if err != nil {
		t.Fatal(err)
	}
	ready := CheckOwnership(validated)
	if ready.State() != parsed.State() {
		t.Fatal("compiler stage replaced the shared program state")
	}
	ir, err := Lower(ready)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ir), ".main()") {
		t.Fatal("lowered IR does not contain main definition")
	}
}

func TestPublicUseReexportLowers(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"heap.mg":    "mod heap\npub allocator() u64:\n    ret 7\n..\n",
		"library.mg": "mod library\npub use \"heap.mg\" heap\n",
		"main.mg":    "mod main\nuse \"library.mg\" lib\nmain() void:\n    value u64 = lib.heap.allocator()\n..\n",
	}
	for name, source := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	stdRoot, err := filepath.Abs(filepath.Join("..", "..", "std"))
	if err != nil {
		t.Fatal(err)
	}
	state, err := shared.MakeShared(dir, stdRoot)
	if err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(dir, "main.mg")
	parsed, err := Parse(state, mainPath)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	specialized, err := Specialize(parsed)
	if err != nil {
		t.Fatalf("specialize: %v", err)
	}
	linked, err := Link(specialized)
	if err != nil {
		t.Fatalf("link: %v", err)
	}
	typed, err := CheckTypes(linked)
	if err != nil {
		t.Fatalf("types: %v", err)
	}
	validated, err := ValidateLowering(typed)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if _, err := Lower(CheckOwnership(validated)); err != nil {
		t.Fatalf("lower: %v", err)
	}
}

func TestParseReturnsPartialProgramOnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.mg")
	if err := os.WriteFile(path, []byte("mod main\nmain() void:\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdRoot, err := filepath.Abs(filepath.Join("..", "..", "std"))
	if err != nil {
		t.Fatal(err)
	}
	state, err := shared.MakeShared(dir, stdRoot)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(state, path)
	if err == nil {
		t.Fatal("expected malformed source to fail")
	}
	if parsed.State() != state || state.Files[path] == nil {
		t.Fatal("parse error discarded the partial program")
	}
}

func TestRequireMainModuleIsAnExplicitStage(t *testing.T) {
	parsed, path := testProgram(t, "mod library\n")
	if err := RequireMainModule(*parsed, path); err == nil {
		t.Fatal("expected non-main root module to be rejected")
	}
}
