//go:build !llvm_object

package main

import (
	compilerpipeline "Magma/src/compiler_pipeline"
	"fmt"
)

func validateIncrementalOptions(opts options) error {
	if opts.incremental {
		return fmt.Errorf("incremental compilation requires an llvm_object compiler build")
	}
	return nil
}

func defaultBackendOptions(opts *options) {
	opts.strategy = "textual"
	opts.incremental = false
}

func backendLoweringLabel(_ options) string { return "textual LLVM IR lowering" }

func lowerBackend(program compilerpipeline.SafetyCheckedProgram, opts options) ([]byte, [][]byte, error) {
	if opts.strategy != "textual" {
		return nil, nil, fmt.Errorf("object lowering requires an llvm_object compiler build")
	}
	output, err := compilerpipeline.LowerReachable(program)
	return output, nil, err
}
