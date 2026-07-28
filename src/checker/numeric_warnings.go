package checker

import (
	"fmt"

	t "Magma/src/types"
)

func warnNumericConversion(c *ctx, target *t.NodeType, expr t.NodeExpr, context string) {
	if c == nil || c.Shared == nil || c.FileCtx == nil || target == nil || expr == nil {
		return
	}
	// Numeric literals are context-typed during lowering. A separate range
	// diagnostic is preferable to treating every small literal as i64 narrowing.
	if isNumericLiteralExpression(expr) {
		return
	}
	actual := expr.GetInferredType()
	to, toOK := numericDescriptor(target)
	from, fromOK := numericDescriptor(actual)
	if !toOK || !fromOK || sameType(target, actual) {
		return
	}

	risky := false
	reason := ""
	switch {
	case from.IsFloat && !to.IsFloat:
		risky, reason = true, "floating-point to integer conversion discards the fractional part and may exceed the integer range"
	case !from.IsFloat && to.IsFloat:
		// IEEE formats have fewer precision bits than their storage widths. This
		// deliberately warns whenever the integer representation is at least as
		// wide as the float representation.
		if from.ByteSize >= to.ByteSize {
			risky, reason = true, "integer to floating-point conversion may lose precision"
		}
	case from.IsFloat && to.IsFloat && from.ByteSize > to.ByteSize:
		risky, reason = true, "conversion to a narrower floating-point representation may lose precision"
	case !from.IsFloat && !to.IsFloat:
		if from.ByteSize > to.ByteSize {
			risky, reason = true, "conversion to a narrower integer representation may lose data"
		} else if from.ByteSize == to.ByteSize && from.IsSigned != to.IsSigned {
			risky, reason = true, "conversion between signed and unsigned representations may change the value"
		}
	}
	if !risky {
		return
	}
	c.Shared.Warnings = append(c.Shared.Warnings, t.Diagnostic{
		Severity: t.SeverityWarning,
		Stage:    "type checking",
		Ctx:      c.FileCtx,
		FilePath: c.FileCtx.FilePath,
		Token:    *expressionSourceToken(expr),
		Message:  fmt.Sprintf("%s converts '%s' to '%s': %s", context, flattenType(actual), flattenType(target), reason),
	})
}

func isNumericLiteralExpression(expr t.NodeExpr) bool {
	switch node := expr.(type) {
	case *t.NodeExprLit:
		return node.LitType == t.TokLitNum
	case *t.NodeExprUnary:
		return isNumericLiteralExpression(node.Operand)
	case *t.NodeExprBinary:
		return isNumericLiteralExpression(node.Left) && isNumericLiteralExpression(node.Right)
	default:
		return false
	}
}
