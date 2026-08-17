package llvmir_test

import (
	"regexp"
	"strings"
	"testing"
)

func TestForLoopLowersBoundOnceAndUsesDeferredIncrement(t *testing.T) {
	ir, err := compileSource(t, `mod main

bound() u64:
    ret 3
..

main() void:
    sum u64 = 0
    for i u64 = 0 to bound():
        if i == 1:
            continue
        ..
        sum = sum + i
    ..
..
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(regexp.MustCompile(`call i64 @[^\s]+\.bound\(ptr %\.ctx\.addr\)`).FindAllString(ir, -1)); got != 1 {
		t.Fatalf("bound call count = %d, want 1\n%s", got, ir)
	}
	for _, want := range []string{"icmp ult i64", "add i64"} {
		if !strings.Contains(ir, want) {
			t.Fatalf("for-loop IR is missing %q:\n%s", want, ir)
		}
	}
}

func TestForLoopSupportsInferredSignedIndex(t *testing.T) {
	_, err := compileSource(t, `mod main

main() void:
    for i := -2 to 2:
        if i == 0:
            break
        ..
    ..
..
`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestForLoopInfersLiteralIndexFromBound(t *testing.T) {
	ir, err := compileSource(t, `mod main

bound() u64:
    ret 3
..

main() void:
    for i := 0 to bound():
        if i == 1:
            break
        ..
    ..
..
`)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"icmp ult i64", "add i64"} {
		if !strings.Contains(ir, want) {
			t.Fatalf("inferred u64 for-loop IR is missing %q:\n%s", want, ir)
		}
	}
}
