package makeabs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("mod test\n"), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestRelativeImportOptionalExtension(t *testing.T) {
	dir := t.TempDir()
	module := filepath.Join(dir, "library.mg")
	writeTestFile(t, module)
	got, err := ResolveImport("library", filepath.Join(dir, "main.mg"), filepath.Join(dir, "Magma.exe"))
	if err != nil || got != module {
		t.Fatalf("ResolveImport = %q, %v; want %q", got, err, module)
	}
}

func TestStandardImportBesideExecutable(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "Magma.exe")
	module := filepath.Join(dir, "std", "collections", "array.mg")
	writeTestFile(t, module)
	for _, specifier := range []string{"std:collections/array", "std:collections/array.mg"} {
		got, err := ResolveImport(specifier, filepath.Join(dir, "project", "main.mg"), executable)
		if err != nil || got != module {
			t.Fatalf("%s resolved to %q, %v; want %q", specifier, got, err, module)
		}
	}
}

func TestStandardImportRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	_, err := ResolveImport("std:../secret", filepath.Join(dir, "main.mg"), filepath.Join(dir, "Magma.exe"))
	if err == nil || !strings.Contains(err.Error(), "escapes the std directory") {
		t.Fatalf("traversal error = %v", err)
	}
}
