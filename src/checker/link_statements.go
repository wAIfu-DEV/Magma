package checker

import (
	t "Magma/src/types"
)

func clDefer(c *ctx, def *t.NodeStmtDefer) error {
	if def.IsBody {
		return clBody(c, &def.Body)
	} else {
		return clExpr(c, def.Expression, false)
	}
}

func clReturn(c *ctx, ret *t.NodeStmtRet) error {
	e := clExpr(c, ret.Expression, false)
	if e != nil {
		return e
	}
	return nil
}

func clThrow(c *ctx, throw *t.NodeStmtThrow) error {
	e := clExpr(c, throw.Expression, false)
	if e != nil {
		return e
	}
	return nil
}

func clIf(c *ctx, ifStmt *t.NodeStmtIf) error {
	e := clExpr(c, ifStmt.CondExpr, false)
	if e != nil {
		return e
	}

	e = clBody(c, &ifStmt.Body)
	if e != nil {
		return e
	}

	if ifStmt.NextCondStmt != nil {
		switch n := ifStmt.NextCondStmt.(type) {
		case *t.NodeStmtIf:
			e = clIf(c, n)
		case *t.NodeStmtElse:
			e = clBody(c, &n.Body)
		}
		if e != nil {
			return e
		}
	}

	return nil
}

func clWhile(c *ctx, whileStmt *t.NodeStmtWhile) error {
	e := clExpr(c, whileStmt.CondExpr, false)
	if e != nil {
		return e
	}

	e = clBody(c, &whileStmt.Body)
	if e != nil {
		return e
	}
	return nil
}

func clBody(c *ctx, bdy *t.NodeBody) error {
	if bdy.Scope != nil {
		enterScope(c, bdy.Scope)
		defer leaveScope(c)
	}
	for _, stmt := range bdy.Statements {
		switch n := stmt.(type) {
		case *t.NodeStmtRet:
			e := clReturn(c, n)
			if e != nil {
				return e
			}
		case *t.NodeStmtExpr:
			e := clExpr(c, n.Expression, false)
			if e != nil {
				return e
			}
		case *t.NodeStmtThrow:
			e := clThrow(c, n)
			if e != nil {
				return e
			}
		case *t.NodeStmtIf:
			e := clIf(c, n)
			if e != nil {
				return e
			}
		case *t.NodeStmtWhile:
			e := clWhile(c, n)
			if e != nil {
				return e
			}
		case *t.NodeStmtDefer:
			e := clDefer(c, n)
			if e != nil {
				return e
			}
		}
	}
	return nil
}
