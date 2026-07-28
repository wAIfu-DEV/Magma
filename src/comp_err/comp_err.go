// Package comp_err constructs, classifies, aggregates, and renders compiler
// diagnostics. Compiler passes should return errors; presentation belongs here.
package comp_err

import (
	"Magma/src/types"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
)

// CompilationError is retained as a source-compatible name for the unified
// diagnostic type.
type CompilationError = types.Diagnostic

// ErrorList preserves multiple independent failures and their deterministic
// order. It participates in errors.Is/errors.As through multi-error unwrapping.
type ErrorList struct{ Errs []error }

func (e *ErrorList) Error() string {
	parts := make([]string, 0, len(e.Errs))
	for _, err := range e.Errs {
		if err != nil {
			parts = append(parts, err.Error())
		}
	}
	return strings.Join(parts, "\n")
}
func (e *ErrorList) Unwrap() []error { return e.Errs }

// StageError records the compiler boundary that returned an error without
// discarding its concrete type or source location.
type StageError struct {
	Stage string
	Err   error
}

func (e *StageError) Error() string { return e.Stage + ": " + e.Err.Error() }
func (e *StageError) Unwrap() error { return e.Err }

func AtStage(stage string, err error) error {
	if err == nil {
		return nil
	}
	return &StageError{Stage: stage, Err: err}
}

func Join(errs ...error) error {
	filtered := make([]error, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	switch len(filtered) {
	case 0:
		return nil
	case 1:
		return filtered[0]
	default:
		return &ErrorList{Errs: filtered}
	}
}

func CompilationErrorToken(ctx *types.FileCtx, tk *types.Token, shortDesc, additional string) error {
	return &types.Diagnostic{
		Severity: types.SeverityError,
		Ctx:      ctx, FilePath: ctx.FilePath, Token: *tk,
		Message: shortDesc, ShortDesc: shortDesc, Additional: additional,
	}
}

// Diagnostics returns all source diagnostics in stable traversal order. Stage
// wrappers supply a stage only when the originating pass did not set one.
func Diagnostics(err error) []*types.Diagnostic {
	var result []*types.Diagnostic
	var walk func(error, string)
	walk = func(current error, stage string) {
		if current == nil {
			return
		}
		if staged, ok := current.(*StageError); ok {
			walk(staged.Err, staged.Stage)
			return
		}
		if diagnostic, ok := current.(*types.Diagnostic); ok {
			copy := *diagnostic
			if copy.Stage == "" {
				copy.Stage = stage
			}
			if copy.FilePath == "" && copy.Ctx != nil {
				copy.FilePath = copy.Ctx.FilePath
			}
			result = append(result, &copy)
			return
		}
		if many, ok := current.(interface{ Unwrap() []error }); ok {
			for _, child := range many.Unwrap() {
				walk(child, stage)
			}
			return
		}
		if one, ok := current.(interface{ Unwrap() error }); ok {
			walk(one.Unwrap(), stage)
		}
	}
	walk(err, "")
	return result
}

func printLine(out io.Writer, ctx *types.FileCtx, line int) {
	lines := bytes.Split(ctx.Content, []byte{'\n'})
	if line < 1 || line > len(lines) {
		return
	}
	lineText := strings.TrimSuffix(string(lines[line-1]), "\r")
	fmt.Fprintf(out, "%d| %s\n", line, lineText)
}

func Fprint(out io.Writer, err error) bool {
	if err == nil {
		return false
	}
	var render func(error, string)
	render = func(current error, stage string) {
		if current == nil {
			return
		}
		if staged, ok := current.(*StageError); ok {
			render(staged.Err, staged.Stage)
			return
		}
		if diagnostic, ok := current.(*types.Diagnostic); ok {
			copy := *diagnostic
			if copy.Stage == "" {
				copy.Stage = stage
			}
			if copy.FilePath == "" && copy.Ctx != nil {
				copy.FilePath = copy.Ctx.FilePath
			}
			FprintDiagnostic(out, &copy)
			return
		}
		if many, ok := current.(interface{ Unwrap() []error }); ok {
			for _, child := range many.Unwrap() {
				render(child, stage)
			}
			return
		}
		if one, ok := current.(interface{ Unwrap() error }); ok && len(Diagnostics(one.Unwrap())) > 0 {
			render(one.Unwrap(), stage)
			return
		}
		if stage != "" {
			fmt.Fprintf(out, "fatal error [%s]: %s\n", stage, current)
		} else {
			fmt.Fprintf(out, "fatal error: %s\n", current)
		}
	}
	render(err, "")
	return true
}

func FprintDiagnostic(out io.Writer, diagnostic *types.Diagnostic) {
	description := strings.NewReplacer("'\r\n'", "newline", "'\n'", "newline", "'\r'", "newline").Replace(diagnostic.Error())
	severity := "error"
	if diagnostic.Severity == types.SeverityWarning {
		severity = "warning"
	}
	stage := ""
	if diagnostic.Stage != "" {
		stage = " [" + diagnostic.Stage + "]"
	}
	fmt.Fprintf(out, "%s:l%d:c%d: %s%s: %s\n", diagnostic.FilePath, diagnostic.Token.Pos.Line, diagnostic.Token.Pos.Col, severity, stage, description)
	if diagnostic.Ctx != nil {
		line := int(diagnostic.Token.Pos.Line)
		printLine(out, diagnostic.Ctx, line-1)
		printLine(out, diagnostic.Ctx, line)
		printLine(out, diagnostic.Ctx, line+1)
	}
	if diagnostic.Additional != "" {
		fmt.Fprintln(out, diagnostic.Additional)
	}
	if diagnostic.Cause != nil {
		fmt.Fprintf(out, "caused by: %v\n", diagnostic.Cause)
	}
	fmt.Fprintln(out)
}

// Print retains the historical API used by the compiler entry point.
func Print(err error) bool { return Fprint(os.Stderr, err) }
