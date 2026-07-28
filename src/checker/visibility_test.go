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
	state, err := shared.MakeShared(dir, filepath.Join("..", "..", "std"))
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

func TestGenericFunctionValue(t *testing.T) {
	main := `mod main
identity[T](value T) T:
    ret value
..
main() void:
    callback := identity[u64]
    value u64 = callback(7)
..
`
	if err := checkModules(t, visibilityLibrary, main); err != nil {
		t.Fatalf("generic function value rejected: %v", err)
	}
}

func TestImportedGenericFunctionValue(t *testing.T) {
	main := `mod main
use "library.mg" lib

main() void:
    callback := lib.publicGeneric[u64]
    value u64 = callback(7)
..
`
	if err := checkModules(t, visibilityLibrary, main); err != nil {
		t.Fatalf("imported generic function value rejected: %v", err)
	}
}

func TestTypeLikeSubscriptRemainsSubscript(t *testing.T) {
	main := `mod main
main() void:
    values := array u64[2](10, 20)
    u64 := 1
    value u64 = values[u64]
..
`
	if err := checkModules(t, visibilityLibrary, main); err != nil {
		t.Fatalf("type-like subscript was treated as a generic value: %v", err)
	}
}

func TestGenericArityErrorsAreStructured(t *testing.T) {
	tests := map[string]string{
		"function": `mod main
identity[T](value T) T:
    ret value
..
main() void:
    identity[u64, u32](1)
..
`,
		"struct": `mod main
Pair[T, U](left T, right U)
main() void:
    value Pair[u64]
..
`,
	}

	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			err := checkModules(t, visibilityLibrary, source)
			var diagnostic *comp_err.CompilationError
			if !errors.As(err, &diagnostic) {
				t.Fatalf("error = %T %v, want structured compilation error", err, err)
			}
			if !strings.Contains(diagnostic.ShortDesc, "expects") || diagnostic.Token.Pos.Line == 0 {
				t.Fatalf("diagnostic = %#v, want arity message at source position", diagnostic)
			}
		})
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

func TestPublicGlobalsAcrossModules(t *testing.T) {
	library := "mod library\npub const EXPORTED u8 = 7\npub value u8 = 9\nconst PRIVATE u8 = 1\n"
	main := "mod main\nuse \"library.mg\" lib\nconst COPY u8 = lib.EXPORTED\nmain() void:\n    current u8 = lib.value\n..\n"
	if err := checkModules(t, library, main); err != nil {
		t.Fatalf("public cross-module globals failed: %v", err)
	}
}

func TestPublicGlobalMemberChainsAcrossModules(t *testing.T) {
	library := `mod library
pub Inner(value u64)
pub Outer(inner Inner, ptr Inner*)
pub config Outer
`
	main := `mod main
use "library.mg" lib
main() void:
    current u64 = lib.config.inner.value
    lib.config.inner.value = current
    local lib.Inner
    lib.config.ptr = addrof local
    pointed u64 = lib.config.ptr.value
    fieldPointer u64* = addrof lib.config.inner.value
..
`
	if err := checkModules(t, library, main); err != nil {
		t.Fatalf("public cross-module global member access failed: %v", err)
	}
}

func TestUnknownMemberOnPublicGlobalRejectedAcrossModules(t *testing.T) {
	library := "mod library\npub Config(value u64)\npub config Config\n"
	main := "mod main\nuse \"library.mg\" lib\nmain() void:\n    value u64 = lib.config.missing\n..\n"
	err := checkModules(t, library, main)
	if err == nil || !strings.Contains(err.Error(), "type 'Config' has no member named 'missing'") {
		t.Fatalf("unknown imported global member error = %v", err)
	}
}

func TestImportedConstantMemberCannotBeAssigned(t *testing.T) {
	library := "mod library\npub Config(value u64)\npub const CONFIG Config = Config(value=1)\n"
	main := "mod main\nuse \"library.mg\" lib\nmain() void:\n    lib.CONFIG.value = 2\n..\n"
	err := checkModules(t, library, main)
	if err == nil || !strings.Contains(err.Error(), "cannot assign to constant 'lib.CONFIG.value'") {
		t.Fatalf("imported constant assignment error = %v", err)
	}
}

func TestPrivateGlobalRejectedAcrossModules(t *testing.T) {
	library := "mod library\nconst PRIVATE u8 = 1\n"
	main := "mod main\nuse \"library.mg\" lib\nconst COPY u8 = lib.PRIVATE\n"
	err := checkModules(t, library, main)
	if err == nil || !strings.Contains(err.Error(), "constant 'lib.PRIVATE' is private") {
		t.Fatalf("private global error = %v", err)
	}
}
