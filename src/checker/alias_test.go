package checker_test

import (
	"strings"
	"testing"
)

func TestLocalTypeAliases(t *testing.T) {
	main := `mod main

alias Word = u64
alias WordPtr = Word*

read(value WordPtr) Word:
    ret *value
..

main() void:
    value Word = 42
    pointer WordPtr = addrof value
    result Word = read(pointer)
..
`
	if err := checkModules(t, "mod library\n", main); err != nil {
		t.Fatalf("local aliases failed: %v", err)
	}
}

func TestPublicTypeAliasAcrossModules(t *testing.T) {
	library := `mod library

alias internal_size = u64
pub alias size_t = internal_size
`
	main := `mod main
use "library.mg" c

identity(value c.size_t) c.size_t:
    ret value
..

main() void:
    value c.size_t = identity(42)
..
`
	if err := checkModules(t, library, main); err != nil {
		t.Fatalf("public cross-module alias failed: %v", err)
	}
}

func TestPrivateTypeAliasRejectedAcrossModules(t *testing.T) {
	library := "mod library\n\nalias hidden = u64\n"
	main := "mod main\nuse \"library.mg\" c\n\nmain() void:\n    value c.hidden\n..\n"
	err := checkModules(t, library, main)
	if err == nil || !strings.Contains(err.Error(), "type alias 'c.hidden' is private") {
		t.Fatalf("private alias error = %v", err)
	}
}

func TestCyclicTypeAliasRejected(t *testing.T) {
	main := `mod main

alias First = Second
alias Second = First

main() void:
    value First
..
`
	err := checkModules(t, "mod library\n", main)
	if err == nil || !strings.Contains(err.Error(), "cyclic type alias") {
		t.Fatalf("cyclic alias error = %v", err)
	}
}

func TestCompilerKnownTypeAlias(t *testing.T) {
	main := `mod main

alias size_t = @compiler_known_type("c.size_t")

identity(value size_t) size_t:
    ret value
..

main() void:
    value size_t = identity(42)
..
`
	if err := checkModules(t, "mod library\n", main); err != nil {
		t.Fatalf("compiler-known alias failed: %v", err)
	}
}

func TestUnknownCompilerKnownTypeRejected(t *testing.T) {
	main := "mod main\n\nalias mystery = @compiler_known_type(\"missing.type\")\n\nmain() void:\n..\n"
	err := checkModules(t, "mod library\n", main)
	if err == nil || !strings.Contains(err.Error(), "compiler-known type 'missing.type' is unavailable") {
		t.Fatalf("unknown compiler-known type error = %v", err)
	}
}
