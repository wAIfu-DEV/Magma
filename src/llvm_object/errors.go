//go:build llvm_object

package llvmobject

import (
	"errors"
	"fmt"

	t "Magma/src/types"
)

// ErrorContext identifies the Magma lowering operation responsible for a
// backend failure. Empty fields are omitted when an error happens before a
// source function or target has been selected.
type ErrorContext struct {
	Token     *t.Token
	Function  string
	Block     string
	Operation string
	Target    string
}

// BackendError keeps LLVM diagnostics attached to source-level lowering
// context so callers never need to interpret an unstructured verifier dump.
type BackendError struct {
	ErrorContext
	LLVMDiagnostic string
	Err            error
	CompilerBug    bool
}

func (e *BackendError) Error() string {
	if e == nil {
		return "LLVM backend error"
	}
	where := ""
	if e.Function != "" {
		where = " in " + e.Function
		if e.Block != "" {
			where += ":" + e.Block
		}
	}
	operation := e.Operation
	if operation == "" {
		operation = "LLVM backend"
	}
	if e.Target != "" {
		where += " for " + e.Target
	}
	if e.Token != nil {
		where += fmt.Sprintf(" at %d:%d", e.Token.Pos.Line, e.Token.Pos.Col)
	}
	return fmt.Sprintf("%s%s: %s", operation, where, e.LLVMDiagnostic)
}

func (e *BackendError) Unwrap() error { return e.Err }

func backendError(context ErrorContext, err error) error {
	if err == nil {
		return nil
	}
	var existing *BackendError
	if errors.As(err, &existing) {
		return err
	}
	return &BackendError{ErrorContext: context, LLVMDiagnostic: err.Error(), Err: err}
}

func verificationError(context ErrorContext, err error) error {
	if err == nil {
		return nil
	}
	wrapped := backendError(context, err)
	if value, ok := wrapped.(*BackendError); ok {
		value.CompilerBug = true
	}
	return wrapped
}

func (b *Builder) errorContext(operation string, token *t.Token) ErrorContext {
	context := ErrorContext{Operation: operation, Token: token}
	if b != nil {
		context.Function = b.function
		context.Block = b.block
	}
	return context
}
