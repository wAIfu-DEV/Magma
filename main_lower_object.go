//go:build llvm_object

package main

import (
	compilerpipeline "Magma/src/compiler_pipeline"
	llvmobject "Magma/src/llvm_object"
	"fmt"
	"os"
)

func validateIncrementalOptions(opts options) error {
	if opts.incremental && opts.backend == "textual" {
		return fmt.Errorf("the deprecated textual backend does not support incremental compilation")
	}
	return nil
}

func defaultBackendOptions(opts *options) {
	opts.backend = "object"
	opts.incremental = true
}

func backendLoweringLabel(opts options) string {
	if opts.backend == "textual" {
		return "textual LLVM IR lowering"
	}
	return "LLVM object lowering"
}

func lowerBackend(program compilerpipeline.SafetyCheckedProgram, opts options) ([]byte, bool, error) {
	if opts.backend == "textual" {
		output, err := compilerpipeline.LowerReachable(program)
		return output, false, err
	}
	if opts.emit == "llvm" {
		output, err := compilerpipeline.LowerObjectIR(program)
		return output, false, err
	}
	level := llvmobject.OptimizationLevel(opts.opt)
	options := llvmobject.TargetOptions{
		Triple: opts.target, Optimization: level, PIC: opts.emit == "exe",
	}
	if opts.incremental {
		var explain func(string)
		if opts.incrementalExplain {
			explain = func(message string) { fmt.Fprintln(os.Stderr, "incremental:", message) }
		}
		safetyMode := "strict"
		if opts.safetyWarnings {
			safetyMode = "warnings"
		}
		output, err := compilerpipeline.LowerCachedIncrementalObjectBytes(program, options, opts.cacheDir, compilerVersion(), safetyMode, explain)
		return output, true, err
	}
	output, err := compilerpipeline.LowerObjectBytes(program, options)
	return output, true, err
}
