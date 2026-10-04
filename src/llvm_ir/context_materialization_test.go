package llvmir_test

import (
	"strings"
	"testing"
)

func TestReadOnlyImplicitContextIsForwardedWithoutMaterialization(t *testing.T) {
	ir, err := compileSource(t, `mod main

callee() void:
..

main() void:
    callee()
..
`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ir, ".callee(ptr %.ctx.in)") {
		t.Fatalf("context pointer was not forwarded directly:\n%s", ir)
	}
	mainStart := strings.Index(ir, ".main(ptr %.ctx.in) {")
	if mainStart < 0 {
		t.Fatalf("main function missing:\n%s", ir)
	}
	mainIR := ir[mainStart:]
	if end := strings.Index(mainIR, "\n}\n"); end >= 0 {
		mainIR = mainIR[:end]
	}
	if strings.Contains(mainIR, "%.ctx.addr = alloca") {
		t.Fatalf("read-only implicit context was materialized:\n%s", mainIR)
	}
}

func TestAssignedImplicitContextIsMaterializedOnFirstWrite(t *testing.T) {
	ir, err := compileSource(t, `mod main

callee() void:
..

main() void:
    ctx = ctx
    callee()
..
`)
	if err != nil {
		t.Fatal(err)
	}
	mainStart := strings.Index(ir, ".main(ptr %.ctx.in) {")
	if mainStart < 0 {
		t.Fatalf("main function missing:\n%s", ir)
	}
	mainIR := ir[mainStart:]
	for _, want := range []string{
		"%.ctx.addr = alloca",
		"%.ctx.value = load",
		"store %struct.context_",
		".callee(ptr %.ctx.addr)",
	} {
		if !strings.Contains(mainIR, want) {
			t.Fatalf("assigned context is missing %q:\n%s", want, mainIR)
		}
	}
}
