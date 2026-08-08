package checker

import (
	"Magma/src/comp_err"
	t "Magma/src/types"
	"fmt"
	"strings"
)

func ctIfStmt(c *ctx, ifStmt *t.NodeStmtIf) error {
	e := ctExpr(c, ifStmt.CondExpr)
	if e != nil {
		return e
	}

	infType := ifStmt.CondExpr.GetInferredType()
	isBool := isBoolType(infType)

	if !isBool {
		return comp_err.CompilationErrorToken(c.FileCtx, &ifStmt.Tk, fmt.Sprintf("if condition must have type 'bool', but got '%s'", flattenType(infType)), "")
	}

	e = ctBody(c, &ifStmt.Body)
	if e != nil {
		return e
	}

	if ifStmt.NextCondStmt != nil {
		switch n := ifStmt.NextCondStmt.(type) {
		case *t.NodeStmtIf:
			e = ctIfStmt(c, n)
		case *t.NodeStmtElse:
			e = ctBody(c, &n.Body)
		}
		if e != nil {
			return e
		}
	}

	return nil
}

func ctWhileStmt(c *ctx, whileStmt *t.NodeStmtWhile) error {
	e := ctExpr(c, whileStmt.CondExpr)
	if e != nil {
		return e
	}

	infType := whileStmt.CondExpr.GetInferredType()
	isBool := isBoolType(infType)

	if !isBool {
		return comp_err.CompilationErrorToken(c.FileCtx, &whileStmt.Tk, fmt.Sprintf("loop condition must have type 'bool', but got '%s'", flattenType(infType)), "")
	}

	c.LoopDepth++
	e = ctBody(c, &whileStmt.Body)
	c.LoopDepth--
	if e != nil {
		return e
	}
	return nil
}

func ctForStmt(c *ctx, forStmt *t.NodeStmtFor) error {
	decl, ok := forStmt.DeclExpr.(*t.NodeExprVarDefAssign)
	if !ok || decl.VarDef == nil {
		return comp_err.CompilationErrorToken(c.FileCtx, &forStmt.Tk, "for loop index must be an initialized variable declaration", "")
	}
	if e := ctExpr(c, forStmt.DeclExpr); e != nil {
		return e
	}
	if literal, ok := decl.AssignExpr.(*t.NodeExprLit); ok && strings.Contains(literal.Value, ".") {
		return comp_err.CompilationErrorToken(c.FileCtx, &forStmt.Tk, "for loop index must have an integer type, but got a floating-point initializer", "floating-point loop indexes are not supported")
	}
	indexType := decl.VarDef.Type
	if !isIntegerType(indexType) {
		return comp_err.CompilationErrorToken(c.FileCtx, &forStmt.Tk, fmt.Sprintf("for loop index must have an integer type, but got '%s'", flattenType(indexType)), "floating-point loop indexes are not supported")
	}
	if e := ctExprWithUsage(c, forStmt.BoundExpr, true); e != nil {
		return e
	}
	if literal, ok := forStmt.BoundExpr.(*t.NodeExprLit); ok && strings.Contains(literal.Value, ".") {
		return comp_err.CompilationErrorToken(c.FileCtx, &forStmt.Tk, "for loop bound must have an integer type, but got a floating-point literal", "")
	}
	boundType := forStmt.BoundExpr.GetInferredType()
	if !isIntegerType(boundType) {
		return comp_err.CompilationErrorToken(c.FileCtx, &forStmt.Tk, fmt.Sprintf("for loop bound must have an integer type, but got '%s'", flattenType(boundType)), "")
	}
	_, literalBound := forStmt.BoundExpr.(*t.NodeExprLit)
	if !sameType(indexType, boundType) && !literalBound {
		return comp_err.CompilationErrorToken(c.FileCtx, &forStmt.Tk, fmt.Sprintf("for loop bound must have type '%s', but got '%s'", flattenType(indexType), flattenType(boundType)), "the index and bound must use the same integer type")
	}

	c.LoopDepth++
	e := ctBody(c, &forStmt.Body)
	c.LoopDepth--
	return e
}
func ctThrow(c *ctx, throw *t.NodeStmtThrow) error {
	if c.CurrentTypeFunc != nil && (c.CurrentTypeFunc.ReturnType == nil || !c.CurrentTypeFunc.ReturnType.Throws) {
		return comp_err.CompilationErrorToken(
			c.FileCtx,
			&throw.Tk,
			"cannot use 'throw' inside a non-throwing function",
			"mark the enclosing function's return type with '!' or return normally",
		)
	}

	e := ctExpr(c, throw.Expression)
	if e != nil {
		return e
	}

	infType := throw.Expression.GetInferredType()
	isErr := isErrType(infType)

	if !isErr && !isStrType(infType) {
		return comp_err.CompilationErrorToken(c.FileCtx, &throw.Tk, fmt.Sprintf("cannot throw value of type '%s'; expected 'error' or 'str'", flattenType(infType)), "")
	}

	return nil
}

func ctDefer(c *ctx, def *t.NodeStmtDefer) error {
	if def.IsBody {
		return ctBody(c, &def.Body)
	} else {
		_, ignoredCall := def.Expression.(*t.NodeExprCall)
		return ctExprWithUsage(c, def.Expression, !ignoredCall)
	}
}

func ctReturn(c *ctx, ret *t.NodeStmtRet) error {
	if c.CurrentTypeFunc == nil {
		return comp_err.CompilationErrorToken(c.FileCtx, &ret.Tk, "cannot use 'ret' outside a function", "place the return statement inside a function body")
	}

	if c.CurrentTypeFunc.ReturnType == nil {
		// A linked function always has a return type; retain a defensive error for
		// callers that bypass the normal checking pipeline.
		return fmt.Errorf("function return type is null when trying to infer ret type")
	}

	ret.OwnerFuncType = c.CurrentTypeFunc.ReturnType

	e := ctExpr(c, ret.Expression)
	if e != nil {
		return e
	}

	retIsVoid := isVoidType(ret.OwnerFuncType)
	exprIsVoid := isVoidType(ret.Expression.GetInferredType())

	if retIsVoid != exprIsVoid {
		if retIsVoid {
			return comp_err.CompilationErrorToken(c.FileCtx, &ret.Tk, "cannot return a value from a function returning 'void'", "use a bare 'ret' statement")
		}
		return comp_err.CompilationErrorToken(c.FileCtx, &ret.Tk, fmt.Sprintf("missing return value in function returning '%s'", flattenType(ret.OwnerFuncType)), "provide a value after 'ret'")
	}

	expectedValue := *ret.OwnerFuncType
	expectedValue.Throws = false
	if !compatibleInitializer(&expectedValue, ret.Expression) {
		return comp_err.CompilationErrorToken(
			c.FileCtx,
			&ret.Tk,
			fmt.Sprintf("cannot return value of type '%s' from function returning '%s'", flattenType(ret.Expression.GetInferredType()), flattenType(&expectedValue)),
			"",
		)
	}
	warnNumericConversion(c, &expectedValue, ret.Expression, "return value")
	return nil
}

func ctBody(c *ctx, bdy *t.NodeBody) error {
	for _, stmt := range bdy.Statements {
		var e error
		switch n := stmt.(type) {
		case *t.NodeStmtRet:
			e = ctReturn(c, n)
		case *t.NodeStmtExpr:
			_, ignoredCall := n.Expression.(*t.NodeExprCall)
			e = ctExprWithUsage(c, n.Expression, !ignoredCall)
		case *t.NodeStmtThrow:
			e = ctThrow(c, n)
		case *t.NodeStmtIf:
			e = ctIfStmt(c, n)
		case *t.NodeStmtWhile:
			e = ctWhileStmt(c, n)
		case *t.NodeStmtFor:
			e = ctForStmt(c, n)
		case *t.NodeStmtBreak:
			if c.LoopDepth == 0 {
				e = comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, "cannot use 'break' outside a loop", "place 'break' inside a loop")
			}
		case *t.NodeStmtContinue:
			if c.LoopDepth == 0 {
				e = comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, "cannot use 'continue' outside a loop", "place 'continue' inside a loop")
			}
		case *t.NodeStmtDefer:
			e = ctDefer(c, n)
		}
		if e != nil {
			return e
		}
	}
	return nil
}
