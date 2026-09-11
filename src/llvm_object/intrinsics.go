//go:build llvm_object

package llvmobject

import (
	t "Magma/src/types"
	"fmt"
)

// LowerLLVMExpr is the object-backend registry for the shared typed @llvm AST
// node. The operand slice contains lowered runtime operands; literal
// configuration arguments stay on expr for operation-specific builders.
func (b *Builder) LowerLLVMExpr(expr *t.NodeExprLlvm, operands []Value, result Type) (value Value, err error) {
	var token *t.Token
	operation := "lower @llvm directive"
	if expr != nil {
		token = &expr.Tk
		operation = "lower @llvm(" + expr.Operation + ")"
	}
	defer func() { err = backendError(b.errorContext(operation, token), err) }()
	if err := b.valid(); err != nil {
		return Value{}, err
	}
	if expr == nil || expr.ResultType == nil {
		return Value{}, fmt.Errorf("incomplete @llvm expression")
	}
	requireOne := func() error {
		if len(operands) != 1 {
			return fmt.Errorf("object @llvm %s expects one runtime operand, got %d", expr.Operation, len(operands))
		}
		if err := b.module.ownsValue(operands[0], expr.Operation+" operand"); err != nil {
			return err
		}
		if err := b.module.ownsType(result, expr.Operation+" result"); err != nil {
			return err
		}
		return nil
	}
	switch expr.Operation {
	case "reinterpret":
		if err := requireOne(); err != nil {
			return Value{}, err
		}
		return operands[0], nil
	case "ptrtoint":
		if err := requireOne(); err != nil {
			return Value{}, err
		}
		return b.PtrToInt(operands[0], result, "")
	case "inttoptr":
		if err := requireOne(); err != nil {
			return Value{}, err
		}
		return b.IntToPtr(operands[0], result, "")
	case "bitcast":
		if err := requireOne(); err != nil {
			return Value{}, err
		}
		return Value{raw: b.module.builder.CreateBitCast(operands[0].raw, result.raw, ""), owner: b.module}, nil
	case "sext":
		if err := requireOne(); err != nil {
			return Value{}, err
		}
		return Value{raw: b.module.builder.CreateSExt(operands[0].raw, result.raw, ""), owner: b.module}, nil
	case "zext":
		if err := requireOne(); err != nil {
			return Value{}, err
		}
		return Value{raw: b.module.builder.CreateZExt(operands[0].raw, result.raw, ""), owner: b.module}, nil
	case "trunc":
		if err := requireOne(); err != nil {
			return Value{}, err
		}
		return Value{raw: b.module.builder.CreateTrunc(operands[0].raw, result.raw, ""), owner: b.module}, nil
	case "sitofp":
		if err := requireOne(); err != nil {
			return Value{}, err
		}
		return Value{raw: b.module.builder.CreateSIToFP(operands[0].raw, result.raw, ""), owner: b.module}, nil
	case "uitofp":
		if err := requireOne(); err != nil {
			return Value{}, err
		}
		return Value{raw: b.module.builder.CreateUIToFP(operands[0].raw, result.raw, ""), owner: b.module}, nil
	case "fptosi":
		if err := requireOne(); err != nil {
			return Value{}, err
		}
		return Value{raw: b.module.builder.CreateFPToSI(operands[0].raw, result.raw, ""), owner: b.module}, nil
	case "fptoui":
		if err := requireOne(); err != nil {
			return Value{}, err
		}
		return Value{raw: b.module.builder.CreateFPToUI(operands[0].raw, result.raw, ""), owner: b.module}, nil
	default:
		return Value{}, fmt.Errorf("unsupported object @llvm operation %q", expr.Operation)
	}
}
