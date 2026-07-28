package llvmir_test

import (
	"regexp"
	"strings"
	"testing"
)

func TestGlobalStructFieldReadsAndWritesUseResolvedMemberPath(t *testing.T) {
	ir, err := compileSource(t, `mod main

Inner(value u64)
Outer(inner Inner*, direct Inner)

globalOuter Outer

main() void:
    globalOuter.direct.value = 7
    directValue u64 = globalOuter.direct.value

    inner Inner
    globalOuter.inner = addrof inner
    globalOuter.inner.value = 42
    pointerValue u64 = globalOuter.inner.value
    fieldPointer u64* = addrof globalOuter.direct.value
..
`)
	if err != nil {
		t.Fatalf("compile global struct field accesses: %v", err)
	}

	globalOuter := regexp.MustCompile(`@[^\s,]+\.globalOuter`)
	if references := globalOuter.FindAllString(ir, -1); len(references) < 6 {
		t.Fatalf("global root has %d references, want reads and writes through the resolved symbol", len(references))
	}
	if !regexp.MustCompile(`getelementptr %struct\.[^,]+\.Outer, ptr @[^\s,]+\.globalOuter, i32 0, i32 [01]`).MatchString(ir) {
		t.Fatal("global field write does not compute its address from the global symbol")
	}
	if !regexp.MustCompile(`load %struct\.[^,]+\.Outer, ptr @[^\s,]+\.globalOuter`).MatchString(ir) {
		t.Fatal("global field read does not load the global root value")
	}
	if !strings.Contains(ir, "extractvalue") {
		t.Fatal("global field read returns the whole struct instead of traversing member metadata")
	}
	if !strings.Contains(ir, "store i64 7") || !strings.Contains(ir, "store i64 42") {
		t.Fatal("global direct or pointer-field write is missing")
	}
}
