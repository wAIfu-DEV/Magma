package parser

import (
	"Magma/src/comp_err"
	t "Magma/src/types"
	"fmt"
)

func parseStmtReturn(ctx *ParseCtx) (t.NodeStatement, error) {
	retTk, e := peek(ctx)
	if e != nil {
		return nil, e
	}
	consume(ctx) // consume ret kw

	next, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	if next.KeywType == t.KwNewline {
		return &t.NodeStmtRet{Tk: retTk, Expression: &t.NodeExprVoid{}}, nil
	}

	expr, e := parseExpression(ctx, next, 0)
	if e != nil {
		return nil, e
	}

	return &t.NodeStmtRet{Tk: retTk, Expression: expr}, nil
}

func parseStmtContinue(ctx *ParseCtx) (t.NodeStatement, error) {
	keyword, _ := peek(ctx)
	consume(ctx)
	next, e := peek(ctx)
	if e != nil {
		return nil, e
	}
	if next.KeywType != t.KwNewline {
		return nil, comp_err.CompilationErrorToken(ctx.Fctx, &keyword, "syntax error: 'continue' does not accept an operand", "expected a newline after 'continue'")
	}
	return &t.NodeStmtContinue{Tk: keyword}, nil
}

func parseStmtBreak(ctx *ParseCtx) (t.NodeStatement, error) {
	keyword, _ := peek(ctx)
	consume(ctx)
	next, e := peek(ctx)
	if e != nil {
		return nil, e
	}
	if next.KeywType != t.KwNewline {
		return nil, comp_err.CompilationErrorToken(ctx.Fctx, &keyword, "syntax error: 'break' does not accept an operand", "expected a newline after 'break'")
	}
	return &t.NodeStmtBreak{Tk: keyword}, nil
}

func parseStmtThrow(ctx *ParseCtx) (t.NodeStatement, error) {
	keyword, e := peek(ctx)
	if e != nil {
		return nil, e
	}
	consume(ctx) // consume ret kw

	next, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	expr, e := parseExpression(ctx, next, 0)
	if e != nil {
		return nil, e
	}

	return &t.NodeStmtThrow{Tk: keyword, Expression: expr, Pos: keyword.Pos}, nil
}

func parseStatement(ctx *ParseCtx, tk t.Token) (t.NodeStatement, error) {
	switch tk.KeywType {
	case t.KwReturn:
		return parseStmtReturn(ctx)
	case t.KwBreak:
		return parseStmtBreak(ctx)
	case t.KwContinue:
		return parseStmtContinue(ctx)
	case t.KwThrow:
		return parseStmtThrow(ctx)
	case t.KwLlvm:
		return parseLlvm(ctx, tk)
	case t.KwIf:
		return parseStmtIf(ctx, tk)
	case t.KwWhile:
		return parseStmtWhile(ctx, tk)
	case t.KwFor:
		return parseStmtFor(ctx, tk)
	case t.KwDefer, t.KwOnError:
		n, e := parseDefer(ctx, tk)
		if e != nil {
			return nil, e
		}
		return n, nil
	}

	expr, e := parseExpression(ctx, tk, 0)
	if e != nil {
		return nil, e
	}

	return &t.NodeStmtExpr{Expression: expr}, nil
}

func parseBody(ctx *ParseCtx, tk t.Token) (t.NodeBody, error) {
	n := t.NodeBody{}

	if tk.KeywType != t.KwColon {
		return t.NodeBody{}, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&tk,
			fmt.Sprintf("syntax error: expected body opening ':' but got '%s' instead", tk.Repr),
			"bodies/scopes are opened with ':' and ended with '..'",
		)
	}
	consume(ctx)

	for {
		tk, e := peek(ctx)
		if e != nil {
			return t.NodeBody{}, e
		}

		if tk.KeywType == t.KwNewline {
			consume(ctx)
			continue
		}

		if tk.KeywType == t.KwDots {
			consume(ctx)
			return n, nil
		}

		stmtNode, e := parseStatement(ctx, tk)
		if e != nil {
			return t.NodeBody{}, e
		}
		n.Statements = append(n.Statements, stmtNode)
	}
}

func parseDeferBody(ctx *ParseCtx, tk t.Token) (t.NodeBody, error) {
	n := t.NodeBody{}

	if tk.KeywType != t.KwColon {
		return t.NodeBody{}, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&tk,
			fmt.Sprintf("syntax error: expected body opening ':' but got '%s' instead", tk.Repr),
			"bodies/scopes are opened with ':' and ended with '..'",
		)
	}
	consume(ctx)

	for {
		tk, e := peek(ctx)
		if e != nil {
			return t.NodeBody{}, e
		}

		if tk.KeywType == t.KwNewline {
			consume(ctx)
			continue
		}

		if tk.KeywType == t.KwDots {
			consume(ctx)
			return n, nil
		}

		if tk.KeywType == t.KwDefer || tk.KeywType == t.KwOnError {
			return n, comp_err.CompilationErrorToken(
				ctx.Fctx,
				&tk,
				"syntax error: cannot nest deferred statements",
				"",
			)
		}

		stmtNode, e := parseStatement(ctx, tk)
		if e != nil {
			return t.NodeBody{}, e
		}
		n.Statements = append(n.Statements, stmtNode)
	}
}

func parseIfBody(ctx *ParseCtx, tk t.Token, ifStmt *t.NodeStmtIf) (t.NodeBody, error) {
	n := t.NodeBody{}

	if tk.KeywType != t.KwColon {
		return t.NodeBody{}, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&tk,
			fmt.Sprintf("syntax error: expected body opening ':' but got '%s' instead", tk.Repr),
			"bodies/scopes are opened with ':' and ended with '..'",
		)
	}
	consume(ctx)

	for {
		tk, e := peek(ctx)
		if e != nil {
			return t.NodeBody{}, e
		}

		if tk.KeywType == t.KwNewline {
			consume(ctx)
			continue
		}

		if tk.KeywType == t.KwDots {
			consume(ctx)
			return n, nil
		}

		if tk.KeywType == t.KwElif {
			elifStmt, e := parseStmtIf(ctx, tk)
			if e != nil {
				return t.NodeBody{}, e
			}
			ifStmt.NextCondStmt = elifStmt
			return n, nil
		}

		if tk.KeywType == t.KwElse {
			elseStmt, e := parseStmtElse(ctx, tk)
			if e != nil {
				return t.NodeBody{}, e
			}
			ifStmt.NextCondStmt = elseStmt
			return n, nil
		}

		stmtNode, e := parseStatement(ctx, tk)
		if e != nil {
			return t.NodeBody{}, e
		}
		n.Statements = append(n.Statements, stmtNode)
	}
}

func parseStmtElse(ctx *ParseCtx, tk t.Token) (*t.NodeStmtElse, error) {
	consume(ctx)

	next, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	body, e := parseBody(ctx, next)
	if e != nil {
		return nil, e
	}

	return &t.NodeStmtElse{
		Body: body,
	}, nil
}

func parseStmtIf(ctx *ParseCtx, tk t.Token) (*t.NodeStmtIf, error) {
	consume(ctx)

	next, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	condExpr, e := parseExpression(ctx, next, 0)
	if e != nil {
		return nil, e
	}

	next2, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	ifStmt := &t.NodeStmtIf{
		Tk:       tk,
		CondExpr: condExpr,
	}

	body, e := parseIfBody(ctx, next2, ifStmt)
	if e != nil {
		return nil, e
	}

	ifStmt.Body = body
	return ifStmt, nil
}

func parseStmtWhile(ctx *ParseCtx, tk t.Token) (*t.NodeStmtWhile, error) {
	consume(ctx)

	next, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	condExpr, e := parseExpression(ctx, next, 0)
	if e != nil {
		return nil, e
	}

	next2, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	whileStmt := &t.NodeStmtWhile{
		Tk:       tk,
		CondExpr: condExpr,
	}

	body, e := parseBody(ctx, next2)
	if e != nil {
		return nil, e
	}

	whileStmt.Body = body
	return whileStmt, nil
}

func parseStmtFor(ctx *ParseCtx, tk t.Token) (*t.NodeStmtFor, error) {
	consume(ctx)

	toIndex := -1
	depth := 0
	for i := ctx.TokIdx; i < len(ctx.Toks); i++ {
		candidate := ctx.Toks[i]
		switch candidate.KeywType {
		case t.KwParenOp, t.KwBrackOp:
			depth++
		case t.KwParenCl, t.KwBrackCl:
			if depth > 0 {
				depth--
			}
		case t.KwColon, t.KwNewline:
			if depth == 0 {
				i = len(ctx.Toks)
			}
		}
		if depth == 0 && candidate.Repr == "to" {
			toIndex = i
			break
		}
	}
	if toIndex < 0 {
		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&tk,
			"syntax error: expected 'to' keyword after index declaration in 'for' statement",
			"example: 'for i u64 = 0 to 10:' or 'for i := 0 to expr:'",
		)
	}
	ctx.Toks[toIndex].Type = t.TokKeyword
	ctx.Toks[toIndex].KeywType = t.KwTo

	next, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	declExpr, e := parseExpression(ctx, next, 0)
	if e != nil {
		return nil, e
	}

	switch declExpr.(type) {
	case *t.NodeExprVarDefAssign:
		break
	default:
		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&tk,
			"syntax error: expected index variable declaration with initial value after 'for' keyword",
			"example: 'for i u64 = 0 to expr:' or 'for i := 0 to expr:'",
		)
	}

	next2, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	if next2.KeywType != t.KwTo {
		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&tk,
			"syntax error: expected 'to' keyword after index declaration in 'for' statement",
			"example: 'for i u64 = 0 to 10:' or 'for i := 0 to expr:'\nexpr is evaluated only once and is exclusive",
		)
	}

	consume(ctx)
	next3, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	boundExpr, e := parseExpression(ctx, next3, 0)
	if e != nil {
		return nil, e
	}

	forStmt := &t.NodeStmtFor{
		Tk:        tk,
		DeclExpr:  declExpr,
		BoundExpr: boundExpr,
	}

	bodyStart, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	body, e := parseBody(ctx, bodyStart)
	if e != nil {
		return nil, e
	}

	forStmt.Body = body
	return forStmt, nil
}
