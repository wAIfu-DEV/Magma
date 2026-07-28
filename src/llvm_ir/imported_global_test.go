package llvmir_test

import (
	"Magma/src/checker"
	"Magma/src/join"
	llvmir "Magma/src/llvm_ir"
	"Magma/src/monomorph"
	"Magma/src/pipeline"
	"Magma/src/shared"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestImportedGlobalNestedMembersLowerFromResolvedRoot(t *testing.T) {
	dir := t.TempDir()
	libraryPath := filepath.Join(dir, "library.mg")
	mainPath := filepath.Join(dir, "main.mg")
	if err := os.WriteFile(libraryPath, []byte(`mod library
pub Inner(value u64)
pub Outer(inner Inner, ptr Inner*)
pub config Outer
`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mainPath, []byte(`mod main
use "library.mg" lib
main() void:
    current u64 = lib.config.inner.value
    lib.config.inner.value = current
    local lib.Inner
    lib.config.ptr = addrof local
    pointed u64 = lib.config.ptr.value
    fieldPointer u64* = addrof lib.config.inner.value
..
`), 0600); err != nil {
		t.Fatal(err)
	}

	state, err := shared.MakeShared(dir, filepath.Join("..", "..", "std"))
	if err == nil {
		err = pipeline.DoMain(state, mainPath)
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
		t.Fatalf("check imported global member chain: %v", err)
	}
	ir, err := llvmir.IrWrite(state)
	if err != nil {
		t.Fatalf("lower imported global member chain: %v", err)
	}
	text := string(ir)
	if !regexp.MustCompile(`@[^\s,]+\.config`).MatchString(text) {
		t.Fatal("IR does not reference the resolved imported global symbol")
	}
	if strings.Count(text, "getelementptr") < 2 {
		t.Fatal("IR does not lower the nested imported member path")
	}
}
