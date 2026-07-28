package checker

import (
	"Magma/src/comp_err"
	t "Magma/src/types"
	"fmt"
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
		return comp_err.CompilationErrorToken(c.FileCtx, &whileStmt.Tk, fmt.Sprintf("while condition must have type 'bool', but got '%s'", flattenType(infType)), "")
	}

	c.LoopDepth++
	e = ctBody(c, &whileStmt.Body)
	c.LoopDepth--
	if e != nil {
		return e
	}
	return nil
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
		case *t.NodeStmtBreak:
			if c.LoopDepth == 0 {
				e = comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, "cannot use 'break' outside a loop", "place 'break' inside a while loop")
			}
		case *t.NodeStmtContinue:
			if c.LoopDepth == 0 {
				e = comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, "cannot use 'continue' outside a loop", "place 'continue' inside a while loop")
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
