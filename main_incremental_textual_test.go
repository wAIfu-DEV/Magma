//go:build !llvm_object

package main

import (
	"strings"
	"testing"
)

func TestIncrementalOptionRequiresObjectBuild(t *testing.T) {
	opts, err := parseArgs([]string{"--incremental", "input.mg"})
	if err != nil {
		t.Fatal(err)
	}
	err = validateIncrementalOptions(opts)
	if err == nil || !strings.Contains(err.Error(), "requires an llvm_object compiler build") {
		t.Fatalf("error = %v, want object-build requirement", err)
	}
}
