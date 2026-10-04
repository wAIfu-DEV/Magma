//go:build llvm_object

package main

import (
	compilerpipeline "Magma/src/compiler_pipeline"
	llvmobject "Magma/src/llvm_object"
	"fmt"
	"os"
)

func validateIncrementalOptions(opts options) error {
	if opts.incremental && opts.strategy == "textual" {
		return fmt.Errorf("the textual strategy does not support incremental compilation")
	}
	return nil
}

func defaultBackendOptions(opts *options) {
	opts.strategy = "thinlto"
	opts.incremental = true
}

func backendLoweringLabel(opts options) string {
	if opts.strategy == "textual" {
		return "textual LLVM IR lowering"
	}
	if opts.strategy == "thinlto" {
		return "cached ThinLTO lowering"
	}
	return "LLVM object lowering"
}

func lowerBackend(program compilerpipeline.SafetyCheckedProgram, opts options) ([]byte, [][]byte, error) {
	if opts.strategy == "textual" {
		output, err := compilerpipeline.LowerReachable(program)
		return output, nil, err
	}
	if opts.emit == "llvm" {
		output, err := compilerpipeline.LowerObjectIR(program)
		return output, nil, err
	}
	level := llvmobject.OptimizationLevel(opts.opt)
	options := llvmobject.TargetOptions{
		Triple: opts.target, Optimization: level, PIC: opts.emit == "exe",
	}
	if opts.jobs > 0 && opts.strategy == "whole" {
		llvmobject.ConfigureThreads(opts.jobs)
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
		if opts.strategy == "thinlto" {
			units, err := compilerpipeline.LowerCachedThinLTOBitcode(program, options, opts.cacheDir, compilerVersion(), safetyMode, explain)
			return nil, units, err
		}
		output, err := compilerpipeline.LowerCachedIncrementalObjectBytes(program, options, opts.cacheDir, compilerVersion(), safetyMode, explain)
		return nil, [][]byte{output}, err
	}
	output, err := compilerpipeline.LowerObjectBytes(program, options)
	return nil, [][]byte{output}, err
}
