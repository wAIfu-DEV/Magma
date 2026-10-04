//go:build llvm_object

package main

import "testing"

func TestThinLTOIsDefault(t *testing.T) {
	opts, err := parseArgs([]string{"input.mg"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.strategy != "thinlto" || opts.opt != 3 || !opts.incremental {
		t.Fatalf("default pipeline is not optimized ThinLTO: %#v", opts)
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
	plainTextual, err := parseArgs([]string{"--strategy", "textual", "--incremental", "input.mg"})
	if err != nil {
		t.Fatal(err)
	}
	if err := validateIncrementalOptions(plainTextual); err == nil {
		t.Fatal("textual strategy accepted incremental mode")
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
	if opts.strategy != "whole" {
		t.Fatalf("LLVM text output selected strategy %q", opts.strategy)
	}
	if _, err := parseArgs([]string{"--emit", "llvm", "--incremental", "input.mg"}); err == nil {
		t.Fatal("explicit incremental LLVM text output was accepted")
	}
}

func TestWholeProgramJobsOption(t *testing.T) {
	opts, err := parseArgs([]string{"--jobs", "12", "input.mg"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.jobs != 12 {
		t.Fatalf("whole-program --jobs options = %#v", opts)
	}
}

func TestStrategyOption(t *testing.T) {
	opts, err := parseArgs([]string{"--strategy", "thinlto", "--jobs", "8", "input.mg"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.strategy != "thinlto" || !opts.incremental || opts.jobs != 8 {
		t.Fatalf("ThinLTO options = %#v", opts)
	}
	for _, args := range [][]string{
		{"--strategy", "thinlto", "--incremental=false", "input.mg"},
		{"--strategy", "unknown", "input.mg"},
	} {
		if _, err := parseArgs(args); err == nil {
			t.Fatalf("invalid ThinLTO options accepted: %v", args)
		}
	}
}

func TestPresetsOnlyReplaceUnchangedFlags(t *testing.T) {
	fast, err := parseArgs([]string{"--preset", "fast-comp", "input.mg"})
	if err != nil {
		t.Fatal(err)
	}
	if fast.strategy != "textual" || fast.opt != 0 || fast.jobs != 0 || fast.incremental {
		t.Fatalf("fast-comp preset = %#v", fast)
	}
	overridden, err := parseArgs([]string{"-O2", "--jobs", "3", "--strategy", "whole", "--preset", "fast-comp", "input.mg"})
	if err != nil {
		t.Fatal(err)
	}
	if overridden.strategy != "whole" || overridden.opt != 2 || overridden.jobs != 3 {
		t.Fatalf("explicit flags were overwritten: %#v", overridden)
	}
	runtime, err := parseArgs([]string{"--preset", "fast-runtime", "input.mg"})
	if err != nil {
		t.Fatal(err)
	}
	if runtime.strategy != "whole" || runtime.opt != 3 {
		t.Fatalf("fast-runtime preset = %#v", runtime)
	}
}

func TestFastCompPresetControlsLLVMTextStrategy(t *testing.T) {
	for _, emitFlag := range []string{"--emit", "-e"} {
		opts, err := parseArgs([]string{"--preset", "fast-comp", emitFlag, "llvm", "input.mg"})
		if err != nil {
			t.Fatalf("%s: %v", emitFlag, err)
		}
		if opts.strategy != "textual" || opts.opt != 0 || opts.incremental {
			t.Fatalf("%s: fast-comp LLVM options = %#v", emitFlag, opts)
		}
	}
}
