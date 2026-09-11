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
	opts.backend = "textual"
	opts.incremental = false
}

func backendLoweringLabel(_ options) string { return "textual LLVM IR lowering" }

func lowerBackend(program compilerpipeline.SafetyCheckedProgram, opts options) ([]byte, bool, error) {
	if opts.backend == "object" {
		return nil, false, fmt.Errorf("object lowering requires an llvm_object compiler build")
	}
	output, err := compilerpipeline.LowerReachable(program)
	return output, false, err
}
