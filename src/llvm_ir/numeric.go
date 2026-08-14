package llvmir

import (
	t "Magma/src/types"
	"fmt"
)

func irExtendFlt(ctx *IrCtx, valSsa SsaName, prevType *t.NodeType, newType *t.NodeType) (SsaName, error) {
	outSsa := irSsaLocal(ctx)
	irWritef(ctx, "  %s = fpext ", outSsa.Repr)

	e := irType(ctx, prevType)
	if e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, valSsa)
	irWrite(ctx, " to ")

	e = irType(ctx, newType)
	if e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, "\n")
	return outSsa, nil
}

func irExtendInt(ctx *IrCtx, valSsa SsaName, signed bool, prevType *t.NodeType, newType *t.NodeType) (SsaName, error) {
	outSsa := irSsaLocal(ctx)

	if signed {
		irWritef(ctx, "  %s = sext ", outSsa.Repr)
	} else {
		irWritef(ctx, "  %s = zext ", outSsa.Repr)
	}

	e := irType(ctx, prevType)
	if e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, valSsa)
	irWrite(ctx, " to ")

	e = irType(ctx, newType)
	if e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, "\n")
	return outSsa, nil
}

func irTruncInt(ctx *IrCtx, valSsa SsaName, prevType *t.NodeType, newType *t.NodeType) (SsaName, error) {
	outSsa := irSsaLocal(ctx)

	irWritef(ctx, "  %s = trunc ", outSsa.Repr)

	e := irType(ctx, prevType)
	if e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, valSsa)
	irWrite(ctx, " to ")

	e = irType(ctx, newType)
	if e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, "\n")
	return outSsa, nil
}

func irTruncFlt(ctx *IrCtx, valSsa SsaName, prevType *t.NodeType, newType *t.NodeType) (SsaName, error) {
	outSsa := irSsaLocal(ctx)

	irWritef(ctx, "  %s = fptrunc ", outSsa.Repr)

	e := irType(ctx, prevType)
	if e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, valSsa)
	irWrite(ctx, " to ")

	e = irType(ctx, newType)
	if e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, "\n")
	return outSsa, nil
}

func irPtrToInt(ctx *IrCtx, valSsa SsaName) SsaName {
	ssa := irSsaLocal(ctx)
	irWritef(ctx, "  %s = ptrtoint ptr %s to i64", ssa.Repr, valSsa.Repr)
	return ssa
}

func irIntToPtr(ctx *IrCtx, valSsa SsaName) SsaName {
	ssa := irSsaLocal(ctx)
	irWritef(ctx, "  %s = inttoptr i64 %s to ptr", ssa.Repr, valSsa.Repr)
	return ssa
}

func irIntToFloat(ctx *IrCtx, valSsa SsaName, numType *t.NodeType, toType *t.NodeType) (SsaName, error) {
	numDesc := getNumDesc(numType)

	// here target is guaranteed to be integer type
	outSsa := irSsaLocal(ctx)
	irWritef(ctx, "  %s = ", outSsa.Repr)

	if numDesc.IsSigned {
		irWrite(ctx, "sitofp ")
	} else {
		irWrite(ctx, "uitofp ")
	}

	e := irType(ctx, numType)
	if e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, valSsa)
	irWrite(ctx, " to ")

	e = irType(ctx, toType)
	if e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, "\n")
	return outSsa, nil
}

func irFloatToInt(ctx *IrCtx, valSsa SsaName, numType *t.NodeType, toType *t.NodeType) (SsaName, error) {
	numDesc := getNumDesc(numType)

	// here target is guaranteed to be integer type
	outSsa := irSsaLocal(ctx)
	irWritef(ctx, "  %s = ", outSsa.Repr)

	if numDesc.IsSigned {
		irWrite(ctx, "fptosi ")
	} else {
		irWrite(ctx, "fptoui ")
	}

	e := irType(ctx, numType)
	if e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, valSsa)
	irWrite(ctx, " to ")

	e = irType(ctx, toType)
	if e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, "\n")
	return outSsa, nil
}

func irPromoteSingleToNum(ctx *IrCtx, expectedType *t.NodeType, ssa SsaName, fromType *t.NodeType) (SsaName, error) {
	if ssa.IsLiteral {
		fromType = expectedType
	}

	fromNum := getNumDesc(fromType)
	expectedNum := getNumDesc(expectedType)

	// with floating point ops always return type of largest (byte size) float
	if fromNum.IsFloat && expectedNum.IsFloat {
		if fromNum.ByteSize == expectedNum.ByteSize {
			// no need for promotion
			return ssa, nil
		} else if fromNum.ByteSize > expectedNum.ByteSize {
			return irTruncFlt(ctx, ssa, fromType, expectedType)
		} else {
			outSsa, e := irExtendFlt(ctx, ssa, fromType, expectedType)
			return outSsa, e
		}
	}

	if fromNum.IsFloat != expectedNum.IsFloat {
		if fromNum.IsFloat {
			return irFloatToInt(ctx, ssa, fromType, expectedType)
		}
		return irIntToFloat(ctx, ssa, fromType, expectedType)
	}

	// integers
	if fromNum.ByteSize > expectedNum.ByteSize {
		return irTruncInt(ctx, ssa, fromType, expectedType)
	} else if fromNum.ByteSize < expectedNum.ByteSize {
		return irExtendInt(ctx, ssa, fromNum.IsSigned, fromType, expectedType)
	} else {
		return ssa, nil
	}

	//return SsaName{}, SsaName{}, nil, fmt.Errorf("unhandled type in numerical promotion")
}

func irCoerceNumeric(ctx *IrCtx, target *t.NodeType, expr t.NodeExpr, value SsaName) (SsaName, error) {
	if target == nil || expr == nil || value.IsLiteral || !isNumberType(target) || !isNumberType(expr.GetInferredType()) || isSameNumType(target, expr.GetInferredType()) {
		return value, nil
	}
	return irPromoteSingleToNum(ctx, target, value, expr.GetInferredType())
}

func irPromoteToNum(ctx *IrCtx, expectedType *t.NodeType, leftSsa SsaName, leftType *t.NodeType, rightSsa SsaName, rightType *t.NodeType) (SsaName, SsaName, *t.NodeType, error) {
	if expectedType == nil {
		return SsaName{}, SsaName{}, nil, fmt.Errorf("numeric operation has no resolved operand type")
	}
	left, err := irPromoteSingleToNum(ctx, expectedType, leftSsa, leftType)
	if err != nil {
		return SsaName{}, SsaName{}, nil, err
	}
	right, err := irPromoteSingleToNum(ctx, expectedType, rightSsa, rightType)
	if err != nil {
		return SsaName{}, SsaName{}, nil, err
	}
	return left, right, expectedType, nil
}

func irExprUnary(ctx *IrCtx, expectedType *t.NodeType, unaryExpr *t.NodeExprUnary) (SsaName, error) {
	if unaryExpr.Operator == t.KwAsterisk && !unaryExpr.ProvenanceChecked {
		return SsaName{}, fmt.Errorf("refusing to lower pointer dereference at line %d, column %d without provenance analysis", unaryExpr.Tk.Pos.Line, unaryExpr.Tk.Pos.Col)
	}
	operandType := unaryExpr.Operand.GetInferredType()
	operandSsa, e := irExpression(ctx, operandType, unaryExpr.Operand, false)
	if e != nil {
		return SsaName{}, e
	}

	switch unaryExpr.Operator {
	case t.KwAsterisk:
		resSsa := irSsaLocal(ctx)
		irWritef(ctx, "  %s = load ", resSsa.Repr)
		e = irType(ctx, unaryExpr.InfType)
		if e != nil {
			return SsaName{}, e
		}
		irWritef(ctx, ", ptr %s\n", operandSsa.Repr)
		return resSsa, nil
	case t.KwTilde:
		if isPointerType(operandType) {
			return SsaName{}, fmt.Errorf("bitwise not (~) is not supported for pointer types")
		}
		if isFloatType(operandType) {
			return SsaName{}, fmt.Errorf("bitwise not (~) is not supported for floating-point types")
		}
		if !isBoolType(operandType) && !isNumberType(operandType) {
			return SsaName{}, fmt.Errorf("bitwise not (~) requires an integer or bool operand")
		}

		resSsa := irSsaLocal(ctx)
		irWritef(ctx, "  %s = xor ", resSsa.Repr)
		e = irType(ctx, operandType)
		if e != nil {
			return SsaName{}, e
		}
		irWrite(ctx, " ")
		irPossibleLitSsa(ctx, operandSsa)
		irWrite(ctx, ", -1\n")
		return resSsa, nil
	default:
		return SsaName{}, fmt.Errorf("unsupported unary expression")
	}
}

func irExprBinBitwise(ctx *IrCtx, expectedType *t.NodeType, binaryExpr *t.NodeExprBinary) (SsaName, error) {
	leftType := binaryExpr.Left.GetInferredType()
	rightType := binaryExpr.Right.GetInferredType()

	leftSsa, e := irExpression(ctx, leftType, binaryExpr.Left, false)
	if e != nil {
		return SsaName{}, e
	}

	rightSsa, e := irExpression(ctx, rightType, binaryExpr.Right, false)
	if e != nil {
		return SsaName{}, e
	}

	if isPointerType(leftType) || isPointerType(rightType) {
		return SsaName{}, fmt.Errorf("bitwise operators are not supported for pointer types")
	}

	resultType := leftType

	if isBoolType(leftType) || isBoolType(rightType) {
		if !isBoolType(leftType) || !isBoolType(rightType) {
			return SsaName{}, fmt.Errorf("bitwise operators on bool require both operands to be bool")
		}
	} else {
		if !isNumberType(leftType) || !isNumberType(rightType) {
			return SsaName{}, fmt.Errorf("bitwise operators require integer operands")
		}
		if isFloatType(leftType) || isFloatType(rightType) {
			return SsaName{}, fmt.Errorf("bitwise operators are not supported for floating-point types")
		}

		if expectedType == nil {
			expectedType = binaryExpr.InfType
		}
		if expectedType == nil {
			expectedType = leftType
		}

		leftSsa, rightSsa, resultType, e = irPromoteToNum(ctx, binaryExpr.OperandType, leftSsa, leftType, rightSsa, rightType)
		if e != nil {
			return SsaName{}, e
		}
	}

	resSsa := irSsaLocal(ctx)
	irWritef(ctx, "  %s = ", resSsa.Repr)

	switch binaryExpr.Operator {
	case t.KwAmpersand:
		irWrite(ctx, "and ")
	case t.KwPipe:
		irWrite(ctx, "or ")
	case t.KwCaret:
		irWrite(ctx, "xor ")
	default:
		return SsaName{}, fmt.Errorf("unexpected bitwise operator")
	}

	e = irType(ctx, resultType)
	if e != nil {
		return SsaName{}, e
	}

	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, leftSsa)
	irWrite(ctx, ", ")
	irPossibleLitSsa(ctx, rightSsa)
	irWrite(ctx, "\n")
	return resSsa, nil
}

func irExprBinShift(ctx *IrCtx, expectedType *t.NodeType, binaryExpr *t.NodeExprBinary) (SsaName, error) {
	leftType := binaryExpr.Left.GetInferredType()
	rightType := binaryExpr.Right.GetInferredType()

	leftSsa, e := irExpression(ctx, leftType, binaryExpr.Left, false)
	if e != nil {
		return SsaName{}, e
	}

	rightSsa, e := irExpression(ctx, rightType, binaryExpr.Right, false)
	if e != nil {
		return SsaName{}, e
	}

	if isPointerType(leftType) || isPointerType(rightType) {
		return SsaName{}, fmt.Errorf("shift operators are not supported for pointer types")
	}
	if !isNumberType(leftType) || !isNumberType(rightType) {
		return SsaName{}, fmt.Errorf("shift operators require integer operands")
	}
	if isFloatType(leftType) || isFloatType(rightType) {
		return SsaName{}, fmt.Errorf("shift operators are not supported for floating-point types")
	}

	if expectedType == nil {
		expectedType = binaryExpr.InfType
	}
	if expectedType == nil {
		expectedType = leftType
	}

	leftSsa, rightSsa, leftType, e = irPromoteToNum(ctx, binaryExpr.OperandType, leftSsa, leftType, rightSsa, rightType)
	if e != nil {
		return SsaName{}, e
	}

	resSsa := irSsaLocal(ctx)
	irWritef(ctx, "  %s = ", resSsa.Repr)

	switch binaryExpr.Operator {
	case t.KwShiftLeft:
		irWrite(ctx, "shl ")
	case t.KwShiftRight:
		if getNumDesc(leftType).IsSigned {
			irWrite(ctx, "ashr ")
		} else {
			irWrite(ctx, "lshr ")
		}
	default:
		return SsaName{}, fmt.Errorf("unexpected shift operator")
	}

	e = irType(ctx, leftType)
	if e != nil {
		return SsaName{}, e
	}

	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, leftSsa)
	irWrite(ctx, ", ")
	irPossibleLitSsa(ctx, rightSsa)
	irWrite(ctx, "\n")
	return resSsa, nil
}

func irExprBinLogical(ctx *IrCtx, binaryExpr *t.NodeExprBinary) (SsaName, error) {
	leftSsa, e := irExpression(ctx, binaryExpr.OperandType, binaryExpr.Left, false)
	if e != nil {
		return SsaName{}, e
	}

	// Allocate the result in the function entry (head) so we can avoid `phi`.
	resultPtr := irSsaLocal(ctx)
	cpy := *ctx
	cpy.bld.Body = ctx.bld.Head
	irWritef(&cpy, " %s = alloca i1\n", resultPtr.Repr)

	rhsLabel := irSsaName(ctx)
	shortCircuitLabel := irSsaName(ctx)
	endLabel := irSsaName(ctx)

	irWrite(ctx, "  br i1 ")
	irPossibleLitSsa(ctx, leftSsa)

	switch binaryExpr.Operator {
	case t.KwAndAnd:
		irWritef(ctx, ", label %%%s, label %%%s\n", rhsLabel.Repr, shortCircuitLabel.Repr)
	case t.KwOrOr:
		irWritef(ctx, ", label %%%s, label %%%s\n", shortCircuitLabel.Repr, rhsLabel.Repr)
	default:
		return SsaName{}, fmt.Errorf("unexpected logical operator")
	}

	irWritef(ctx, "%s:\n", rhsLabel.Repr)

	rightSsa, e := irExpression(ctx, binaryExpr.OperandType, binaryExpr.Right, false)
	if e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, "  store i1 ")
	irPossibleLitSsa(ctx, rightSsa)
	irWritef(ctx, ", ptr %s\n", resultPtr.Repr)
	irWritef(ctx, "  br label %%%s\n", endLabel.Repr)

	irWritef(ctx, "%s:\n", shortCircuitLabel.Repr)
	irWrite(ctx, "  store i1 ")
	switch binaryExpr.Operator {
	case t.KwAndAnd:
		irWrite(ctx, "0")
	case t.KwOrOr:
		irWrite(ctx, "1")
	default:
		return SsaName{}, fmt.Errorf("unexpected logical operator")
	}
	irWritef(ctx, ", ptr %s\n", resultPtr.Repr)
	irWritef(ctx, "  br label %%%s\n", endLabel.Repr)

	irWritef(ctx, "%s:\n", endLabel.Repr)

	resSsa := irSsaLocal(ctx)
	irWritef(ctx, "  %s = load i1, ptr %s\n", resSsa.Repr, resultPtr.Repr)
	return resSsa, nil
}

/*
func irImplicitCastNum(ctx *IrCtx, target SsaName, fromType *t.NodeType, toType *t.NodeType) (SsaName, error) {
	if !isNumberType(fromType) || !isNumberType(toType) {
		return SsaName{}, fmt.Errorf("failure to implicit cast number as both types are not numerical")
	}

	fromDesc := getNumDesc(fromType)
	toDesc := getNumDesc(toType)

	if fromDesc.IsFloat == toDesc.IsFloat {
		if fromDesc.ByteSize == toDesc.ByteSize {
			return target, nil
		}
	}

	if (!fromDesc.IsFloat) && (!toDesc.IsFloat) {
		if fromDesc.ByteSize == toDesc.ByteSize {
			return target, nil
		}
	}

	if fromDesc.IsFloat != toDesc.IsFloat {

	}
}*/

func irExprBinAddition(ctx *IrCtx, expectedType *t.NodeType, binaryExpr *t.NodeExprBinary) (SsaName, error) {
	leftType := binaryExpr.Left.GetInferredType()
	rightType := binaryExpr.Right.GetInferredType()

	lhsSsa, e := irExpression(ctx, leftType, binaryExpr.Left, false)
	if e != nil {
		return SsaName{}, e
	}

	rhsSsa, e := irExpression(ctx, rightType, binaryExpr.Right, false)
	if e != nil {
		return SsaName{}, e
	}

	lhs, rhs, newType, e := irPromoteToNum(
		ctx,
		binaryExpr.OperandType,
		lhsSsa,
		leftType,
		rhsSsa,
		rightType,
	)

	if e != nil {
		return SsaName{}, e
	}

	resSsa := irSsaLocal(ctx)
	irWritef(ctx, "  %s = ", resSsa.Repr)

	if isFloatType(newType) {
		irWrite(ctx, "fadd ")
	} else {
		irWrite(ctx, "add ")
	}

	e = irType(ctx, newType)
	if e != nil {
		return SsaName{}, e
	}

	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, lhs)
	irWrite(ctx, ", ")
	irPossibleLitSsa(ctx, rhs)
	irWrite(ctx, "\n")

	return resSsa, nil
}

func irExprBinSubstraction(ctx *IrCtx, expectedType *t.NodeType, binaryExpr *t.NodeExprBinary) (SsaName, error) {
	lhsSsa, e := irExpression(ctx, binaryExpr.Left.GetInferredType(), binaryExpr.Left, false)
	if e != nil {
		return SsaName{}, e
	}

	rhsSsa, e := irExpression(ctx, binaryExpr.Right.GetInferredType(), binaryExpr.Right, false)
	if e != nil {
		return SsaName{}, e
	}

	lhs, rhs, newType, e := irPromoteToNum(
		ctx,
		binaryExpr.OperandType,
		lhsSsa,
		binaryExpr.Left.GetInferredType(),
		rhsSsa,
		binaryExpr.Right.GetInferredType(),
	)

	if e != nil {
		return SsaName{}, e
	}

	resSsa := irSsaLocal(ctx)
	irWritef(ctx, "  %s = ", resSsa.Repr)

	if isFloatType(newType) {
		irWrite(ctx, "fsub ")
	} else {
		irWrite(ctx, "sub ")
	}

	e = irType(ctx, newType)
	if e != nil {
		return SsaName{}, e
	}

	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, lhs)
	irWrite(ctx, ", ")
	irPossibleLitSsa(ctx, rhs)
	irWrite(ctx, "\n")
	return resSsa, nil
}

func irExprBinMultiplication(ctx *IrCtx, expectedType *t.NodeType, binaryExpr *t.NodeExprBinary) (SsaName, error) {
	// Generate IR for the left-hand side expression
	lhsSsa, e := irExpression(ctx, binaryExpr.Left.GetInferredType(), binaryExpr.Left, false)
	if e != nil {
		return SsaName{}, e
	}

	// Generate IR for the right-hand side expression
	rhsSsa, e := irExpression(ctx, binaryExpr.Right.GetInferredType(), binaryExpr.Right, false)
	if e != nil {
		return SsaName{}, e
	}

	// Promote both sides to a compatible numeric type
	lhs, rhs, newType, e := irPromoteToNum(
		ctx,
		binaryExpr.OperandType,
		lhsSsa,
		binaryExpr.Left.GetInferredType(),
		rhsSsa,
		binaryExpr.Right.GetInferredType(),
	)
	if e != nil {
		return SsaName{}, e
	}

	// Allocate a new SSA name for the result
	resSsa := irSsaLocal(ctx)

	// Start writing the IR instruction
	irWritef(ctx, "  %s = ", resSsa.Repr)

	// Write the appropriate multiplication opcode
	if isFloatType(newType) {
		irWrite(ctx, "fmul ")
	} else {
		irWrite(ctx, "mul ")
	}

	// Write the type
	e = irType(ctx, newType)
	if e != nil {
		return SsaName{}, e
	}

	// Write the operands
	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, lhs)
	irWrite(ctx, ", ")
	irPossibleLitSsa(ctx, rhs)
	irWrite(ctx, "\n")

	// Return the resulting SSA name
	return resSsa, nil
}

func irExprBinDivision(ctx *IrCtx, expectedType *t.NodeType, binaryExpr *t.NodeExprBinary) (SsaName, error) {
	// Generate IR for the left-hand side expression
	lhsSsa, e := irExpression(ctx, binaryExpr.Left.GetInferredType(), binaryExpr.Left, false)
	if e != nil {
		return SsaName{}, e
	}

	// Generate IR for the right-hand side expression
	rhsSsa, e := irExpression(ctx, binaryExpr.Right.GetInferredType(), binaryExpr.Right, false)
	if e != nil {
		return SsaName{}, e
	}

	// Promote both sides to a compatible numeric type
	lhs, rhs, newType, e := irPromoteToNum(
		ctx,
		binaryExpr.OperandType,
		lhsSsa,
		binaryExpr.Left.GetInferredType(),
		rhsSsa,
		binaryExpr.Right.GetInferredType(),
	)
	if e != nil {
		return SsaName{}, e
	}

	// Allocate a new SSA name for the result
	resSsa := irSsaLocal(ctx)

	// Start writing the IR instruction
	irWritef(ctx, "  %s = ", resSsa.Repr)

	// Write the appropriate multiplication opcode
	if isFloatType(newType) {
		irWrite(ctx, "fdiv ")
	} else {
		numDes := getNumDesc(newType)
		if numDes.IsSigned {
			irWrite(ctx, "sdiv ")
		} else {
			irWrite(ctx, "udiv ")
		}
	}

	// Write the type
	e = irType(ctx, newType)
	if e != nil {
		return SsaName{}, e
	}

	// Write the operands
	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, lhs)
	irWrite(ctx, ", ")
	irPossibleLitSsa(ctx, rhs)
	irWrite(ctx, "\n")

	// Return the resulting SSA name
	return resSsa, nil
}

func irExprBinModulo(ctx *IrCtx, expectedType *t.NodeType, binaryExpr *t.NodeExprBinary) (SsaName, error) {
	// Generate IR for the left-hand side expression
	lhsSsa, e := irExpression(ctx, binaryExpr.Left.GetInferredType(), binaryExpr.Left, false)
	if e != nil {
		return SsaName{}, e
	}

	// Generate IR for the right-hand side expression
	rhsSsa, e := irExpression(ctx, binaryExpr.Right.GetInferredType(), binaryExpr.Right, false)
	if e != nil {
		return SsaName{}, e
	}

	// Promote both sides to a compatible numeric type
	lhs, rhs, newType, e := irPromoteToNum(
		ctx,
		binaryExpr.OperandType,
		lhsSsa,
		binaryExpr.Left.GetInferredType(),
		rhsSsa,
		binaryExpr.Right.GetInferredType(),
	)
	if e != nil {
		return SsaName{}, e
	}

	// Allocate a new SSA name for the result
	resSsa := irSsaLocal(ctx)

	// Start writing the IR instruction
	irWritef(ctx, "  %s = ", resSsa.Repr)

	// Write the appropriate multiplication opcode
	if isFloatType(newType) {
		irWrite(ctx, "frem ")
	} else {
		numDes := getNumDesc(newType)
		if numDes.IsSigned {
			irWrite(ctx, "srem ")
		} else {
			irWrite(ctx, "urem ")
		}
	}

	// Write the type
	e = irType(ctx, newType)
	if e != nil {
		return SsaName{}, e
	}

	// Write the operands
	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, lhs)
	irWrite(ctx, ", ")
	irPossibleLitSsa(ctx, rhs)
	irWrite(ctx, "\n")

	// Return the resulting SSA name
	return resSsa, nil
}

func irExprBinCmp(ctx *IrCtx, binaryExpr *t.NodeExprBinary) (SsaName, error) {
	lhsSsa, e := irExpression(ctx, binaryExpr.Left.GetInferredType(), binaryExpr.Left, false)
	if e != nil {
		return SsaName{}, e
	}

	rhsSsa, e := irExpression(ctx, binaryExpr.Right.GetInferredType(), binaryExpr.Right, false)
	if e != nil {
		return SsaName{}, e
	}

	cmpType := binaryExpr.Left.GetInferredType()

	if isNumberType(binaryExpr.Left.GetInferredType()) && isNumberType(binaryExpr.Right.GetInferredType()) {
		lhsSsa, rhsSsa, cmpType, e = irPromoteToNum(
			ctx,
			binaryExpr.OperandType,
			lhsSsa,
			binaryExpr.Left.GetInferredType(),
			rhsSsa,
			binaryExpr.Right.GetInferredType(),
		)

		if e != nil {
			return SsaName{}, e
		}
	}

	isSigned := false
	isFloat := false
	if isNumberType(cmpType) {
		nd := getNumDesc(cmpType)
		isSigned = nd.IsSigned
		isFloat = nd.IsFloat
	}

	resSsa := irSsaLocal(ctx)
	cmpPref := "i"
	if isFloat {
		cmpPref = "f"
	}

	irWritef(ctx, "  %s = %scmp ", resSsa.Repr, cmpPref)

	switch binaryExpr.Operator {
	case t.KwCmpEq:
		if isFloat {
			irWrite(ctx, "oeq ")
		} else {
			irWrite(ctx, "eq ")
		}
	case t.KwCmpNeq:
		if isFloat {
			irWrite(ctx, "une ")
		} else {
			irWrite(ctx, "ne ")
		}
	case t.KwCmpGt:
		if isSigned {
			irWrite(ctx, "sgt ")
		} else if isFloat {
			irWrite(ctx, "ogt ")
		} else {
			irWrite(ctx, "ugt ")
		}
	case t.KwCmpLt:
		if isSigned {
			irWrite(ctx, "slt ")
		} else if isFloat {
			irWrite(ctx, "olt ")
		} else {
			irWrite(ctx, "ult ")
		}
	case t.KwCmpGtEq:
		if isSigned {
			irWrite(ctx, "sge ")
		} else if isFloat {
			irWrite(ctx, "oge ")
		} else {
			irWrite(ctx, "uge ")
		}
	case t.KwCmpLtEq:
		if isSigned {
			irWrite(ctx, "sle ")
		} else if isFloat {
			irWrite(ctx, "ole ")
		} else {
			irWrite(ctx, "ule ")
		}
	}

	e = irType(ctx, cmpType)
	if e != nil {
		return SsaName{}, e
	}

	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, lhsSsa)
	irWrite(ctx, ", ")
	irPossibleLitSsa(ctx, rhsSsa)
	irWrite(ctx, "\n")
	return resSsa, nil
}

func irExprBinary(ctx *IrCtx, expectedType *t.NodeType, binaryExpr *t.NodeExprBinary) (SsaName, error) {
	switch binaryExpr.Operator {
	case t.KwPlus:
		return irExprBinAddition(ctx, expectedType, binaryExpr)
	case t.KwMinus:
		return irExprBinSubstraction(ctx, expectedType, binaryExpr)
	case t.KwAsterisk:
		return irExprBinMultiplication(ctx, expectedType, binaryExpr)
	case t.KwSlash:
		return irExprBinDivision(ctx, expectedType, binaryExpr)
	case t.KwPercent:
		return irExprBinModulo(ctx, expectedType, binaryExpr)
	case t.KwAndAnd, t.KwOrOr:
		return irExprBinLogical(ctx, binaryExpr)
	case t.KwAmpersand, t.KwPipe, t.KwCaret:
		return irExprBinBitwise(ctx, expectedType, binaryExpr)
	case t.KwShiftLeft, t.KwShiftRight:
		return irExprBinShift(ctx, expectedType, binaryExpr)
	case t.KwCmpEq, t.KwCmpNeq, t.KwCmpGt, t.KwCmpLt, t.KwCmpGtEq, t.KwCmpLtEq:
		return irExprBinCmp(ctx, binaryExpr)
	}
	return SsaName{}, fmt.Errorf("unsupported binary expression")
}
