//go:build llvm_object

package main

import "testing"

func TestIncrementalBitcodeIsDefault(t *testing.T) {
	opts, err := parseArgs([]string{"input.mg"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.backend != "object" || !opts.incremental {
		t.Fatalf("default pipeline is not incremental bitcode: %#v", opts)
	}
}

func TestIncrementalOptionIsAcceptedByObjectBuild(t *testing.T) {
	opts, err := parseArgs([]string{"--incremental", "input.mg"})
	if err != nil {
		t.Fatal(err)
	}
	if err := validateIncrementalOptions(opts); err != nil {
		t.Fatal(err)
	}
}

func TestExplicitTextualIncrementalModeIsRejected(t *testing.T) {
	plainTextual, err := parseArgs([]string{"--backend", "textual", "--incremental", "input.mg"})
	if err != nil {
		t.Fatal(err)
	}
	if err := validateIncrementalOptions(plainTextual); err == nil {
		t.Fatal("deprecated textual backend accepted incremental mode")
	}
}

func TestLLVMTextOutputDisablesImplicitIncrementalMode(t *testing.T) {
	opts, err := parseArgs([]string{"--emit", "llvm", "input.mg"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.incremental {
		t.Fatal("LLVM text output retained the implicit incremental mode")
	}
	if opts.backend != "object" {
		t.Fatalf("LLVM text output selected deprecated backend %q", opts.backend)
	}
	if _, err := parseArgs([]string{"--emit", "llvm", "--incremental", "input.mg"}); err == nil {
		t.Fatal("explicit incremental LLVM text output was accepted")
	}
}
