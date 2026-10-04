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
	case t.KwMatch:
		return parseStmtMatch(ctx, tk)
	case t.KwWhile:
		return parseStmtWhile(ctx, tk)
	case t.KwFor:
		return parseStmtFor(ctx, tk)
	case t.KwBounded:
		return parseStmtBounded(ctx, tk)
	case t.KwUnsafe:
		consume(ctx)
		next, e := peek(ctx)
		if e != nil {
			return nil, e
		}
		if next.KeywType == t.KwColon {
			body, e := parseBody(ctx, next)
			if e != nil {
				return nil, e
			}
			return &t.NodeStmtUnsafe{Tk: tk, Body: body}, nil
		}
		expr, e := parseExpression(ctx, next, 0)
		if e != nil {
			return nil, e
		}
		return &t.NodeStmtUnsafe{
			Tk: tk,
			Body: t.NodeBody{Statements: []t.NodeStatement{
				&t.NodeStmtExpr{Expression: expr},
			}},
		}, nil
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

func parseMatchArmBody(ctx *ParseCtx, open t.Token) (t.NodeBody, error) {
	if open.KeywType != t.KwColon {
		return t.NodeBody{}, comp_err.CompilationErrorToken(ctx.Fctx, &open, "match arm requires ':'", "expected: `case Union.Variant:`")
	}
	consume(ctx)
	body := t.NodeBody{}
	for {
		tk, err := peek(ctx)
		if err != nil {
			return t.NodeBody{}, err
		}
		if tk.KeywType == t.KwNewline {
			consume(ctx)
			continue
		}
		if tk.KeywType == t.KwCase || tk.KeywType == t.KwElse || tk.KeywType == t.KwDots {
			return body, nil
		}
		stmt, err := parseStatement(ctx, tk)
		if err != nil {
			return t.NodeBody{}, err
		}
		body.Statements = append(body.Statements, stmt)
	}
}

func parseStmtMatch(ctx *ParseCtx, matchTk t.Token) (*t.NodeStmtMatch, error) {
	consume(ctx)
	first, err := peek(ctx)
	if err != nil {
		return nil, err
	}
	expression, err := parseExpression(ctx, first, 0)
	if err != nil {
		return nil, err
	}
	asTk, err := peek(ctx)
	if err != nil || asTk.KeywType != t.KwAs {
		return nil, comp_err.CompilationErrorToken(ctx.Fctx, &matchTk, "match requires a narrowed binding", "expected: `match value as binding:`")
	}
	consume(ctx)
	binding, err := peek(ctx)
	if err != nil || binding.Type != t.TokName {
		return nil, comp_err.CompilationErrorToken(ctx.Fctx, &asTk, "expected a binding name after 'as'", "expected: `match value as binding:`")
	}
	consume(ctx)
	colon, err := peek(ctx)
	if err != nil || colon.KeywType != t.KwColon {
		return nil, comp_err.CompilationErrorToken(ctx.Fctx, &binding, "match header requires ':'", "expected: `match value as binding:`")
	}
	consume(ctx)
	stmt := &t.NodeStmtMatch{Tk: matchTk, Expression: expression, BindingTk: binding}
	for {
		next, nextErr := peek(ctx)
		if nextErr != nil {
			return nil, nextErr
		}
		if next.KeywType == t.KwNewline {
			consume(ctx)
			continue
		}
		if next.KeywType == t.KwDots {
			consume(ctx)
			return stmt, nil
		}
		if next.KeywType == t.KwElse {
			consume(ctx)
			open, openErr := peek(ctx)
			if openErr != nil {
				return nil, openErr
			}
			body, bodyErr := parseMatchArmBody(ctx, open)
			if bodyErr != nil {
				return nil, bodyErr
			}
			stmt.ElseBody = &body
			continue
		}
		if next.KeywType != t.KwCase {
			return nil, comp_err.CompilationErrorToken(ctx.Fctx, &next, "expected 'case', 'else', or '..' in match", "")
		}
		caseTk := next
		consume(ctx)
		nameTk, nameErr := peek(ctx)
		if nameErr != nil || nameTk.Type != t.TokName {
			return nil, comp_err.CompilationErrorToken(ctx.Fctx, &caseTk, "case requires an explicit union variant", "expected: `case Union.Variant:`")
		}
		name, nameErr := parseName(ctx, nameTk, true)
		if nameErr != nil {
			return nil, nameErr
		}
		if composite, ok := name.(*t.NodeNameComposite); !ok || len(composite.Parts) < 2 {
			return nil, comp_err.CompilationErrorToken(ctx.Fctx, &nameTk, "case variant must include its union name", "expected: `case Union.Variant:`")
		}
		open, openErr := peek(ctx)
		if openErr != nil {
			return nil, openErr
		}
		body, bodyErr := parseMatchArmBody(ctx, open)
		if bodyErr != nil {
			return nil, bodyErr
		}
		bindingDef := &t.NodeExprVarDef{Name: &t.NodeNameSingle{Tk: binding, Name: binding.Repr}}
		stmt.Cases = append(stmt.Cases, &t.NodeMatchCase{Tk: caseTk, VariantName: name, Binding: bindingDef, Body: body})
	}
}

func parseStmtBounded(ctx *ParseCtx, tk t.Token) (*t.NodeStmtBounded, error) {
	consume(ctx)
	stmt := &t.NodeStmtBounded{Tk: tk}
	// Multiple leading `pointer by extent` clauses are nested at parse time.
	// This keeps each assertion scoped and requires no runtime guard.
	var pointers []t.NodeExpr
	var extents []t.NodeExpr
	for {
		first, e := peek(ctx)
		if e != nil {
			return nil, e
		}
		second, err := peekNth(ctx, 1)
		if err != nil || first.Type != t.TokName || second.Type != t.TokName || second.Repr != "by" {
			break
		}
		consume(ctx)
		pointer := &t.NodeExprName{Tk: first, Name: &t.NodeNameSingle{Tk: first, Name: first.Repr}}
		by, err := peek(ctx)
		if err != nil {
			return nil, err
		}
		if by.Type != t.TokName || by.Repr != "by" {
			return nil, comp_err.CompilationErrorToken(ctx.Fctx, &by, "expected 'by' after bounded pointer", "use: bounded ptr by count, i < count:")
		}
		consume(ctx)
		next, err := peek(ctx)
		if err != nil {
			return nil, err
		}
		extent, err := parseExpression(ctx, next, 0)
		if err != nil {
			return nil, err
		}
		pointers = append(pointers, pointer)
		extents = append(extents, extent)
		next, err = peek(ctx)
		if err != nil {
			return nil, err
		}
		if next.KeywType == t.KwComma {
			consume(ctx)
			continue
		}
		break
	}
	for {
		next, e := peek(ctx)
		if e != nil {
			return nil, e
		}
		if next.KeywType == t.KwColon {
			break
		}
		predicate, e := parseExpression(ctx, next, 0)
		if e != nil {
			return nil, e
		}
		stmt.Predicates = append(stmt.Predicates, predicate)
		next, e = peek(ctx)
		if e != nil {
			return nil, e
		}
		if next.KeywType != t.KwComma {
			break
		}
		consume(ctx)
	}
	if len(stmt.Predicates) == 0 && len(pointers) == 0 {
		return nil, comp_err.CompilationErrorToken(ctx.Fctx, &tk, "bounded requires at least one range comparison", "use: bounded i < values.count():")
	}
	next, e := peek(ctx)
	if e != nil {
		return nil, e
	}
	body, e := parseBody(ctx, next)
	if e != nil {
		return nil, e
	}
	stmt.Body = body
	if len(pointers) > 0 {
		stmt.Pointer = pointers[len(pointers)-1]
		stmt.Extent = extents[len(extents)-1]
		for i := len(pointers) - 2; i >= 0; i-- {
			stmt = &t.NodeStmtBounded{
				Tk: tk, Pointer: pointers[i], Extent: extents[i],
				Body: t.NodeBody{Statements: []t.NodeStatement{stmt}},
			}
		}
	}
	return stmt, nil
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
			n.EndTk = tk
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
			n.EndTk = tk
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
			n.EndTk = tk
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
