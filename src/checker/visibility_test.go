package checker_test

import (
	"Magma/src/checker"
	"Magma/src/join"
	"Magma/src/monomorph"
	"Magma/src/pipeline"
	"Magma/src/shared"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func checkModules(t *testing.T, library, main string) error {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "library.mg"), []byte(library), 0600); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(dir, "main.mg")
	if err := os.WriteFile(mainPath, []byte(main), 0600); err != nil {
		t.Fatal(err)
	}
	state, err := shared.MakeShared(dir)
	if err == nil {
		err = pipeline.DoMain(state, mainPath)
	}
	if err == nil {
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
	return err
}

const visibilityLibrary = `mod library

pub Public(value u64)
Private(value u64)
pub PublicBox[T](value T)
PrivateBox[T](value T)

Public.get() u64:
    ret this.value
..

privateHelper() u64:
    ret 42
..

pub publicFunction() u64:
    ret privateHelper()
..

pub publicGeneric[T](value T) T:
    ret value
..

privateGeneric[T](value T) T:
    ret value
..
`

func TestPublicFunctionsStructsAndMethodsCrossModules(t *testing.T) {
	main := `mod main
use "library.mg" lib

main() void:
    value lib.Public
    value.value = lib.publicFunction()
    value.get()
    box := lib.PublicBox[u64](value=lib.publicGeneric[u64](1))
    box.value = 2
..
`
	if err := checkModules(t, visibilityLibrary, main); err != nil {
		t.Fatalf("public cross-module use failed: %v", err)
	}
}

func TestPrivateFunctionRejectedAcrossModules(t *testing.T) {
	main := `mod main
use "library.mg" lib

main() void:
    lib.privateHelper()
..
`
	err := checkModules(t, visibilityLibrary, main)
	if err == nil || !strings.Contains(err.Error(), "function 'lib.privateHelper' is private") {
		t.Fatalf("private function error = %v", err)
	}
}

func TestPrivateStructRejectedAcrossModules(t *testing.T) {
	main := `mod main
use "library.mg" lib

main() void:
    value lib.Private
..
`
	err := checkModules(t, visibilityLibrary, main)
	if err == nil || !strings.Contains(err.Error(), "struct 'lib.Private' is private") {
		t.Fatalf("private struct error = %v", err)
	}
}

func TestPrivateGenericDeclarationsRejectedAcrossModules(t *testing.T) {
	tests := map[string]struct {
		body string
		want string
	}{
		"function": {body: "lib.privateGeneric[u64](1)", want: "function 'lib.privateGeneric' is private"},
		"struct":   {body: "value lib.PrivateBox[u64]", want: "struct 'lib.PrivateBox' is private"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			main := "mod main\nuse \"library.mg\" lib\n\nmain() void:\n    " + test.body + "\n..\n"
			err := checkModules(t, visibilityLibrary, main)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("private generic error = %v", err)
			}
		})
	}
}
