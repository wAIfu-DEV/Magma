// Package compilerpipeline defines the ordered, whole-program compiler stages.
//
// The compiler still uses SharedState internally, but distinct result types make
// stage ordering explicit to callers and provide one home for orchestration.
package compilerpipeline

import (
	"Magma/src/checker"
	"Magma/src/comp_err"
	destroychecker "Magma/src/destroy_checker"
	ircleaner "Magma/src/ir_cleaner"
	"Magma/src/join"
	llvmir "Magma/src/llvm_ir"
	loweringvalidate "Magma/src/lowering_validate"
	"Magma/src/monomorph"
	"Magma/src/pipeline"
	"Magma/src/types"
	"fmt"
)

type ParsedProgram struct{ state *types.SharedState }
type SpecializedProgram struct{ state *types.SharedState }
type LinkedProgram struct{ state *types.SharedState }
type TypedProgram struct{ state *types.SharedState }
type ValidatedProgram struct{ state *types.SharedState }
type SafetyCheckedProgram struct{ state *types.SharedState }

// State exposes the program state for read-only consumers such as the LSP and
// output metadata collection. Compiler passes should use the stage functions.
func (p ParsedProgram) State() *types.SharedState        { return p.state }
func (p SpecializedProgram) State() *types.SharedState   { return p.state }
func (p LinkedProgram) State() *types.SharedState        { return p.state }
func (p TypedProgram) State() *types.SharedState         { return p.state }
func (p ValidatedProgram) State() *types.SharedState     { return p.state }
func (p SafetyCheckedProgram) State() *types.SharedState { return p.state }

// Parse loads, tokenizes, parses, and builds scopes for the root and all of its
// imports. It returns a partial program alongside an error so editor tooling can
// retain successfully parsed declarations.
func Parse(state *types.SharedState, rootPath string) (ParsedProgram, error) {
	program := ParsedProgram{state: state}
	err := pipeline.DoMain(state, rootPath)
	return program, comp_err.AtStage("parsing", join.JoinCompilationUnits(state, err))
}

// RequireMainModule checks the command-line compiler's root-module invariant.
// Editor analysis intentionally does not require this stage.
func RequireMainModule(program ParsedProgram, rootPath string) error {
	mainFile := program.state.Files[rootPath]
	if mainFile == nil {
		return comp_err.AtStage("root validation", fmt.Errorf("main compilation unit %q was not registered", rootPath))
	}
	if mainFile.ModuleName == "main" {
		return nil
	}
	return comp_err.AtStage("root validation", comp_err.CompilationErrorToken(
		mainFile,
		&types.Token{Pos: types.FilePos{Line: 1, Col: 5}, Repr: mainFile.ModuleName},
		fmt.Sprintf("main file must declare module 'main', not '%s'", mainFile.ModuleName),
		"the root compilation unit must start with: `mod main`",
	))
}

func Specialize(program ParsedProgram) (SpecializedProgram, error) {
	if err := monomorph.Run(program.state); err != nil {
		return SpecializedProgram{}, comp_err.AtStage("specialization", err)
	}
	return SpecializedProgram{state: program.state}, nil
}

func Link(program SpecializedProgram) (LinkedProgram, error) {
	if err := checker.CheckLinks(program.state); err != nil {
		return LinkedProgram{}, comp_err.AtStage("linking", err)
	}
	return LinkedProgram{state: program.state}, nil
}

func CheckTypes(program LinkedProgram) (TypedProgram, error) {
	if err := checker.TypeChecker(program.state); err != nil {
		return TypedProgram{}, comp_err.AtStage("type checking", err)
	}
	return TypedProgram{state: program.state}, nil
}

func ValidateLowering(program TypedProgram) (ValidatedProgram, error) {
	if err := loweringvalidate.Validate(program.state); err != nil {
		return ValidatedProgram{}, comp_err.AtStage("lowering validation", err)
	}
	return ValidatedProgram{state: program.state}, nil
}

// CheckSafety is the mandatory ownership-safety gate before lowering. Definite
// safety violations fail the stage unless warningMode was explicitly selected;
// resource-leak findings remain warnings in either mode.
func CheckSafety(program ValidatedProgram, warningMode bool) (SafetyCheckedProgram, error) {
	if err := destroychecker.Run(program.state, warningMode); err != nil {
		return SafetyCheckedProgram{}, comp_err.AtStage("ownership checking", err)
	}
	return SafetyCheckedProgram{state: program.state}, nil
}

// Lower emits and cleans LLVM IR. IrWrite retains its own defensive contract
// validation for direct users outside this pipeline.
func Lower(program SafetyCheckedProgram) ([]byte, error) {
	ir, err := llvmir.IrWrite(program.state)
	if err != nil {
		return nil, comp_err.AtStage("LLVM lowering", err)
	}
	cleaned, err := ircleaner.CleanIr(ir)
	return cleaned, comp_err.AtStage("IR cleanup", err)
}
