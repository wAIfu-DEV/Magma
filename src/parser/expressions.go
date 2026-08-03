package parser

import (
	"Magma/src/comp_err"
	magmatypes "Magma/src/magma_types"
	t "Magma/src/types"
	"fmt"
)

func parseArrayExpr(ctx *ParseCtx, arrayTk t.Token) (t.NodeExpr, error) {
	typeTk, e := peek(ctx)
	if e != nil {
		return nil, e
	}
	openIdx := -1
	for i := ctx.TokIdx; i < len(ctx.Toks); i++ {
		if ctx.Toks[i].KeywType == t.KwBrackOp {
			openIdx = i
			break
		}
		if ctx.Toks[i].KeywType == t.KwNewline {
			break
		}
	}
	if openIdx < 0 {
		return nil, comp_err.CompilationErrorToken(ctx.Fctx, &typeTk, "array expression is missing its length", "expected: `array Type[length]`")
	}
	depth, closeIdx := 0, -1
	for i := openIdx; i < len(ctx.Toks); i++ {
		switch ctx.Toks[i].KeywType {
		case t.KwBrackOp:
			depth++
		case t.KwBrackCl:
			depth--
			if depth == 0 {
				closeIdx = i
			}
		}
		if closeIdx >= 0 {
			break
		}
	}
	if closeIdx >= 0 && closeIdx+1 < len(ctx.Toks) && ctx.Toks[closeIdx+1].KeywType == t.KwBrackOp {
		openIdx = closeIdx + 1
	}

	originalTokens := ctx.Toks
	typeTokens := make([]t.Token, openIdx+1)
	copy(typeTokens, originalTokens[:openIdx])
	typeTokens[openIdx] = t.Token{Repr: "<array-length>", Type: t.TokKeyword, KeywType: t.KwNewline, Pos: originalTokens[openIdx].Pos}
	ctx.Toks = typeTokens
	elemType, typeErr := parseType(ctx, typeTk, false)
	ctx.Toks = originalTokens
	if typeErr != nil {
		return nil, typeErr
	}
	if ctx.TokIdx != openIdx {
		unexpected, _ := peek(ctx)
		return nil, comp_err.CompilationErrorToken(ctx.Fctx, &unexpected, "invalid array element type", "expected: `array Type[length]`")
	}
	consume(ctx)
	lengthTk, e := peek(ctx)
	if e != nil {
		return nil, e
	}
	if lengthTk.KeywType == t.KwBrackCl {
		return nil, comp_err.CompilationErrorToken(ctx.Fctx, &lengthTk, "array length cannot be empty", "provide an integer expression")
	}
	length, e := parseExpression(ctx, lengthTk, 0)
	if e != nil {
		return nil, e
	}
	closeTk, e := peek(ctx)
	if e != nil {
		return nil, e
	}
	if closeTk.KeywType != t.KwBrackCl {
		return nil, comp_err.CompilationErrorToken(ctx.Fctx, &closeTk, "array expression is missing closing ']'", "expected: `array Type[length]`")
	}
	consume(ctx)
	array := &t.NodeExprArray{Tk: arrayTk, ElemType: elemType, Length: length}

	next, nextErr := peek(ctx)
	if nextErr != nil || next.KeywType != t.KwParenOp {
		return array, nil
	}
	consume(ctx)
	consumeNewlines(ctx)
	for {
		entryTk, err := peek(ctx)
		if err != nil {
			return nil, err
		}
		if entryTk.KeywType == t.KwParenCl {
			consume(ctx)
			break
		}

		first, err := parseExpression(ctx, entryTk, getBinaryPrecedence(t.Token{Type: t.TokKeyword, KeywType: t.KwEqual})+1)
		if err != nil {
			return nil, err
		}
		entry := t.NodeArrayInitEntry{Tk: entryTk, Value: first}
		after, err := peek(ctx)
		if err != nil {
			return nil, err
		}
		if after.KeywType == t.KwEqual {
			entry.Index = first
			consume(ctx)
			valueTk, err := peek(ctx)
			if err != nil {
				return nil, err
			}
			entry.Value, err = parseExpression(ctx, valueTk, 0)
			if err != nil {
				return nil, err
			}
			after, err = peek(ctx)
			if err != nil {
				return nil, err
			}
		}
		array.Entries = append(array.Entries, entry)

		if after.KeywType == t.KwComma {
			consume(ctx)
			consumeNewlines(ctx)
			continue
		}
		if after.KeywType == t.KwNewline {
			consumeNewlines(ctx)
			continue
		}
		if after.KeywType != t.KwParenCl {
			return nil, comp_err.CompilationErrorToken(ctx.Fctx, &after, "unexpected token in array initializer", "expected ',', newline, or ')'")
		}
	}
	return array, nil
}

func parseSimplePrimaryExpr(ctx *ParseCtx, tk t.Token) (t.NodeExpr, error) {
	// `array` is contextual so existing variables, imports, and modules may
	// continue to use that name. It starts an array expression only when it is
	// followed by the beginning of an element type.
	if tk.Type == t.TokName && tk.Repr == "array" {
		next, nextErr := peekNth(ctx, 1)
		hasBracket := false
		for i := ctx.TokIdx + 1; i < len(ctx.Toks); i++ {
			if ctx.Toks[i].KeywType == t.KwEqual {
				hasBracket = false
				break
			}
			if ctx.Toks[i].KeywType == t.KwNewline {
				break
			}
			if ctx.Toks[i].KeywType == t.KwBrackOp {
				hasBracket = true
				break
			}
		}
		if hasBracket && nextErr == nil && (next.Type == t.TokName || next.KeywType == t.KwParenOp || next.KeywType == t.KwDollar) {
			consume(ctx)
			return parseArrayExpr(ctx, tk)
		}
	}
	if tk.KeywType == t.KwParenOp {
		consume(ctx)

		next, e := peek(ctx)
		if e != nil {
			return nil, e
		}

		if next.KeywType == t.KwParenCl {
			consume(ctx)
			return &t.NodeExprVoid{}, nil
		}

		n, e := parseExpression(ctx, next, 0)
		if e != nil {
			return nil, e
		}

		maybeClose, e := peek(ctx)
		if e != nil {
			return nil, e
		}

		if maybeClose.KeywType != t.KwParenCl {
			return nil, comp_err.CompilationErrorToken(
				ctx.Fctx, &maybeClose,
				"syntax error: missing closing ')' in grouped expression",
				"",
			)
		}

		consume(ctx)
		return n, nil
	}

	if tk.KeywType == t.KwTry {
		consume(ctx)
		next, e := peek(ctx)
		if e != nil {
			return nil, e
		}
		// Bind try more tightly than binary operators: `try call() == value`
		// means `(try call()) == value`, not `try (call() == value)`.
		expr, e := parseExpression(ctx, next, 60)
		if e != nil {
			return nil, e
		}
		n := &t.NodeExprTry{
			Call: expr,
			Tk:   tk,
			Pos:  tk.Pos,
		}
		return n, nil
	}

	if tk.KeywType == t.KwTrue || tk.KeywType == t.KwFalse {
		consume(ctx)
		boolVal := "0"
		if tk.KeywType == t.KwTrue {
			boolVal = "1"
		}
		return &t.NodeExprLit{Tk: tk, Value: boolVal, LitType: t.TokLitBool}, nil
	}

	if tk.KeywType == t.KwNoneLit {
		consume(ctx)
		return &t.NodeExprLit{Tk: tk, Value: "null", LitType: t.TokLitNone}, nil
	}

	if tk.Type == t.TokLitNum || tk.Type == t.TokLitStr {
		consume(ctx)
		return &t.NodeExprLit{Tk: tk, Value: tk.Repr, LitType: tk.Type}, nil
	}

	if tk.Type == t.TokName {
		n, e := parseName(ctx, tk, true)
		if e != nil {
			return nil, e
		}
		return &t.NodeExprName{Tk: tk, Name: n}, nil
	}

	return nil, comp_err.CompilationErrorToken(
		ctx.Fctx, &tk,
		fmt.Sprintf("syntax error: unexpected '%s' in expression", tk.Repr),
		"",
	)
}

func expressionToken(expr t.NodeExpr, fallback t.Token) t.Token {
	switch n := expr.(type) {
	case *t.NodeExprName:
		switch name := n.Name.(type) {
		case *t.NodeNameSingle:
			return name.Tk
		case *t.NodeNameComposite:
			if len(name.Tokens) > 0 {
				return name.Tokens[len(name.Tokens)-1]
			}
		}
		return n.Tk
	case *t.NodeExprMemberAccess:
		return n.Tk
	}
	return fallback
}

func parsePostfixCallExpr(ctx *ParseCtx, tk t.Token, calleeExpr t.NodeExpr, genericArgs []*t.NodeType) (*t.NodeExprCall, error) {
	consume(ctx)
	argExprs := []t.NodeExpr{}

	maybeCl, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	if maybeCl.KeywType == t.KwParenCl {
		consume(ctx)
	} else {
		for {
			nextExpr, e := peek(ctx)
			if e != nil {
				return nil, e
			}
			parsedExpr, e := parseExpression(ctx, nextExpr, 0)
			if e != nil {
				return nil, e
			}
			argExprs = append(argExprs, parsedExpr)

			afterExpr, e := peek(ctx)
			if e != nil {
				return nil, e
			}
			if afterExpr.KeywType == t.KwComma {
				consume(ctx)
				afterComma, e := peek(ctx)
				if e != nil {
					return nil, e
				}
				if afterComma.KeywType == t.KwParenCl {
					consume(ctx)
					break
				}
				continue
			}
			if afterExpr.KeywType == t.KwParenCl {
				consume(ctx)
				break
			}

			return nil, comp_err.CompilationErrorToken(
				ctx.Fctx, &afterExpr,
				fmt.Sprintf("syntax error: unexpected '%s' in call argument expression list", afterExpr.Repr),
				"",
			)
		}
	}

	return &t.NodeExprCall{
		Tk:          expressionToken(calleeExpr, tk),
		Callee:      calleeExpr,
		Args:        argExprs,
		GenericArgs: genericArgs,
	}, nil
}

func isStructInitList(ctx *ParseCtx) bool {
	offset := 1
	for {
		tk, e := peekNth(ctx, offset)
		if e != nil || tk.KeywType != t.KwNewline {
			break
		}
		offset++
	}
	name, e1 := peekNth(ctx, offset)
	eq, e2 := peekNth(ctx, offset+1)
	return e1 == nil && e2 == nil && name.Type == t.TokName && eq.KeywType == t.KwEqual
}

func consumeNewlines(ctx *ParseCtx) {
	for {
		tk, e := peek(ctx)
		if e != nil || tk.KeywType != t.KwNewline {
			return
		}
		consume(ctx)
	}
}

func parsePostfixStructInit(ctx *ParseCtx, tk t.Token, calleeExpr t.NodeExpr, genericArgs []*t.NodeType) (*t.NodeExprStructInit, error) {
	nameExpr, ok := calleeExpr.(*t.NodeExprName)
	if !ok {
		return nil, comp_err.CompilationErrorToken(ctx.Fctx, &tk, "struct constructor requires a type name", "")
	}
	consume(ctx) // '('
	consumeNewlines(ctx)
	fields := []t.NodeStructFieldInit{}
	for {
		fieldTk, e := peek(ctx)
		if e != nil {
			return nil, e
		}
		if fieldTk.KeywType == t.KwParenCl {
			consume(ctx)
			break
		}
		if fieldTk.Type != t.TokName {
			return nil, comp_err.CompilationErrorToken(ctx.Fctx, &fieldTk, "struct constructor fields must be named", "expected: `field=expression`")
		}
		consume(ctx)
		eq, e := peek(ctx)
		if e != nil || eq.KeywType != t.KwEqual {
			return nil, comp_err.CompilationErrorToken(ctx.Fctx, &fieldTk, "struct constructor field is missing '='", "expected: `field=expression`")
		}
		consume(ctx)
		first, e := peek(ctx)
		if e != nil {
			return nil, e
		}
		value, e := parseExpression(ctx, first, 0)
		if e != nil {
			return nil, e
		}
		fields = append(fields, t.NodeStructFieldInit{Tk: fieldTk, Name: fieldTk.Repr, Expression: value, FieldIndex: -1})

		after, e := peek(ctx)
		if e != nil {
			return nil, e
		}
		if after.KeywType == t.KwParenCl {
			consume(ctx)
			break
		}
		if after.KeywType == t.KwNewline {
			consumeNewlines(ctx)
			continue
		}
		if after.KeywType != t.KwComma {
			return nil, comp_err.CompilationErrorToken(ctx.Fctx, &after, "unexpected token in struct constructor", "expected ',', newline, or ')'")
		}
		consume(ctx)
		consumeNewlines(ctx)
		after, e = peek(ctx)
		if e != nil {
			return nil, e
		}
		if after.KeywType == t.KwParenCl {
			consume(ctx)
			break
		}
	}

	return &t.NodeExprStructInit{
		Tk:     tk,
		Type:   &t.NodeType{KindNode: &t.NodeTypeNamed{NameNode: nameExpr.Name, GenericArgs: genericArgs}},
		Fields: fields,
	}, nil
}

func tryParseGenericCallTypeArgs(ctx *ParseCtx) ([]*t.NodeType, bool) {
	startIdx := ctx.TokIdx

	open, e := peek(ctx)
	if e != nil || open.KeywType != t.KwBrackOp {
		return nil, false
	}

	typeArgs, e := parseTypeArgList(ctx)
	if e != nil {
		ctx.TokIdx = startIdx
		return nil, false
	}

	next, e := peek(ctx)
	if e != nil || next.KeywType != t.KwParenOp {
		ctx.TokIdx = startIdx
		return nil, false
	}

	return typeArgs, true
}

func isKnownGenericFunction(ctx *ParseCtx, expr *t.NodeExprName) bool {
	name, ok := expr.Name.(*t.NodeNameSingle)
	if !ok {
		return false
	}
	fn, ok := ctx.GlobalNode.FuncDefs[name.Name]
	return ok && len(fn.Class.TypeParams) > 0
}

func isKnownFunction(ctx *ParseCtx, expr t.NodeExpr) bool {
	nameExpr, ok := expr.(*t.NodeExprName)
	if !ok {
		return false
	}
	name, ok := nameExpr.Name.(*t.NodeNameSingle)
	if !ok {
		return false
	}
	_, ok = ctx.GlobalNode.FuncDefs[name.Name]
	return ok
}

func parsePostfixNamedCall(ctx *ParseCtx, tk t.Token, calleeExpr t.NodeExpr, genericArgs []*t.NodeType) (*t.NodeExprCall, error) {
	init, err := parsePostfixStructInit(ctx, tk, calleeExpr, genericArgs)
	if err != nil {
		return nil, err
	}
	args := make([]t.NodeExpr, len(init.Fields))
	for index, field := range init.Fields {
		args[index] = field.Expression
	}
	if len(init.Fields) > 0 {
		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&init.Fields[0].Tk,
			"named arguments are not supported in function calls",
			"use positional arguments instead",
		)
	}
	return &t.NodeExprCall{Tk: expressionToken(calleeExpr, tk), Callee: calleeExpr, Args: args, GenericArgs: genericArgs}, nil
}

func parsePostfixSubscriptExpr(ctx *ParseCtx, tk t.Token, targetExpr t.NodeExpr) (*t.NodeExprSubscript, error) {
	consume(ctx)

	nextExpr, e := peek(ctx)
	if e != nil {
		return nil, e
	}
	parsedExpr, e := parseExpression(ctx, nextExpr, 0)
	if e != nil {
		return nil, e
	}

	afterExpr, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	if afterExpr.KeywType != t.KwBrackCl {
		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx, &tk,
			fmt.Sprintf("syntax error: unexpected '%s' in array indexing expression, expected closing ']'", tk.Repr),
			"expected: `<arrayname>[<expr>]`, `my_array[0]`",
		)
	}

	consume(ctx)

	return &t.NodeExprSubscript{
		Tk:     tk,
		Target: targetExpr,
		Expr:   parsedExpr,
	}, nil
}

func parsePostfixMemberExpr(ctx *ParseCtx, tk t.Token, targetExpr t.NodeExpr) (*t.NodeExprMemberAccess, error) {
	consume(ctx)

	memberTk, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	if memberTk.Type != t.TokName {
		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&memberTk,
			fmt.Sprintf("syntax error: expected member name after '.' but got '%s'", memberTk.Repr),
			"expected: `<expr>.<member>`",
		)
	}

	consume(ctx)

	return &t.NodeExprMemberAccess{
		Tk:     memberTk,
		Target: targetExpr,
		Member: memberTk.Repr,
	}, nil
}

func parsePostfixExpr(ctx *ParseCtx, tk t.Token, baseExpr t.NodeExpr) (t.NodeExpr, error) {
	expr := baseExpr

	for {
		next, e := peek(ctx)
		if e != nil {
			return nil, e
		}

		if next.KeywType == t.KwNewline {
			break
		}

		if next.KeywType == t.KwColon {
			break
		}

		if next.KeywType == t.KwParenCl {
			break
		}

		if next.KeywType == t.KwParenOp {
			// A parenthesized token sequence after a name is normally a call, but
			// function types use the same opening syntax. Speculatively parse a
			// function type and only keep it when the following token proves this
			// is a variable declaration. Otherwise, restore the token position and
			// continue with ordinary call parsing.
			if expr == baseExpr {
				if nameExpr, ok := baseExpr.(*t.NodeExprName); ok {
					startIdx := ctx.TokIdx
					typeNd, typeErr := parseType(ctx, next, false)
					if typeErr == nil {
						afterType, afterErr := peek(ctx)
						if afterErr == nil && (afterType.KeywType == t.KwEqual ||
							afterType.KeywType == t.KwNewline ||
							afterType.KeywType == t.KwComma) {
							return &t.NodeExprVarDef{
								Name: nameExpr.Name,
								Type: typeNd,
							}, nil
						}
					}
					ctx.TokIdx = startIdx
				}
			}

			if isStructInitList(ctx) && isKnownFunction(ctx, expr) {
				expr, e = parsePostfixNamedCall(ctx, tk, expr, nil)
			} else if isStructInitList(ctx) {
				expr, e = parsePostfixStructInit(ctx, tk, expr, nil)
			} else {
				expr, e = parsePostfixCallExpr(ctx, tk, expr, nil)
			}
			if e != nil {
				return nil, e
			}
			continue
		}

		if next.KeywType == t.KwBrackOp {
			if nameExpr, ok := expr.(*t.NodeExprName); ok {
				typeArgs, isGenericCall := tryParseGenericCallTypeArgs(ctx)
				if isGenericCall {
					if isStructInitList(ctx) && isKnownFunction(ctx, nameExpr) {
						expr, e = parsePostfixNamedCall(ctx, tk, expr, typeArgs)
					} else if isStructInitList(ctx) {
						expr, e = parsePostfixStructInit(ctx, tk, expr, typeArgs)
					} else {
						expr, e = parsePostfixCallExpr(ctx, tk, expr, typeArgs)
					}
					if e != nil {
						return nil, e
					}
					continue
				}

				// Unlike a generic call, a specialized function value has no
				// following `(` to distinguish it from an array subscript. Only
				// select this form when the name is already known to be a generic
				// function, preserving ordinary `array[index]` expressions.
				if isKnownGenericFunction(ctx, nameExpr) {
					startIdx := ctx.TokIdx
					typeArgs, parseErr := parseTypeArgList(ctx)
					if parseErr == nil {
						nameExpr.GenericArgs = typeArgs
						continue
					}
					ctx.TokIdx = startIdx
				}

				// A qualified imported function cannot be known while modules are
				// still parsing concurrently. Preserve a syntactically valid type-
				// argument interpretation alongside the ordinary subscript parse;
				// monomorphization resolves it once every symbol table is available.
				startIdx := ctx.TokIdx
				candidateArgs, candidateErr := parseTypeArgList(ctx)
				ctx.TokIdx = startIdx
				if candidateErr == nil {
					subscript, subscriptErr := parsePostfixSubscriptExpr(ctx, next, expr)
					if subscriptErr != nil {
						return nil, subscriptErr
					}
					subscript.GenericCandidate = candidateArgs
					expr = subscript
					continue
				}
			}

			expr, e = parsePostfixSubscriptExpr(ctx, next, expr)
			if e != nil {
				return nil, e
			}
			continue
		}

		if next.KeywType == t.KwDot {
			expr, e = parsePostfixMemberExpr(ctx, tk, expr)
			if e != nil {
				return nil, e
			}
			continue
		}

		if expr == baseExpr {
			switch n := baseExpr.(type) {
			case *t.NodeExprName:
				if next.KeywType == t.KwAsterisk {
					maybeType, typeErr := peekNth(ctx, 1)
					afterType, afterErr := peekNth(ctx, 2)
					if typeErr == nil && afterErr == nil && maybeType.Type == t.TokName &&
						afterType.KeywType == t.KwNewline {
						if _, basic := magmatypes.BasicTypes[maybeType.Repr]; basic {
							return nil, comp_err.CompilationErrorToken(
								ctx.Fctx,
								&next,
								"syntax error: pointer marker '*' must follow the type, not the variable name",
								fmt.Sprintf("write `%s %s*` instead of `%s *%s`", flattenName(n.Name), maybeType.Repr, flattenName(n.Name), maybeType.Repr),
							)
						}
					}
				}
				// Only treat `name <type>` as a variable definition when the next token
				// can actually start a type. Otherwise this would incorrectly swallow
				// valid expressions like `x = y` by trying to parse `=` as a type.
				if next.Type != t.TokName && next.KeywType != t.KwInfer && next.KeywType != t.KwDollar {
					break
				}

				if next.Type == t.TokName || next.KeywType == t.KwDollar {
					typeNd, e := parseType(ctx, next, false)
					if e != nil {
						return nil, e
					}

					return &t.NodeExprVarDef{
						Name: n.Name,
						Type: typeNd,
					}, nil
				} else if next.KeywType == t.KwInfer {
					return &t.NodeExprVarDef{
						Name: n.Name,
						Type: nil,
					}, nil
				}
			}
		}
		break
	}

	return expr, nil
}

func parsePrimaryExpr(ctx *ParseCtx, tk t.Token) (t.NodeExpr, error) {
	n, e := parseSimplePrimaryExpr(ctx, tk)
	if e != nil {
		return nil, e
	}
	return parsePostfixExpr(ctx, tk, n)
}

func parseUnaryExpr(ctx *ParseCtx, tk t.Token) (t.NodeExpr, error) {
	if tk.Type == t.TokKeyword {
		switch tk.KeywType {
		case t.KwSizeof:
			consume(ctx)
			next, e := peek(ctx)
			if e != nil {
				return nil, e
			}

			if next.KeywType == t.KwNewline {
				return nil, comp_err.CompilationErrorToken(
					ctx.Fctx,
					&next,
					"syntax error: expected type after 'sizeof'",
					"expected: `sizeof <type>`",
				)
			}

			typeNd, e := parseType(ctx, next, false)
			if e != nil {
				return nil, e
			}

			return &t.NodeExprSizeof{Tk: tk, Type: typeNd}, nil

		case t.KwAddrof:
			consume(ctx)
			next, e := peek(ctx)
			if e != nil {
				return nil, e
			}

			if next.KeywType == t.KwNewline {
				return nil, comp_err.CompilationErrorToken(
					ctx.Fctx,
					&next,
					"syntax error: expected expression after 'addrof'",
					"expected: `addrof <expr>`",
				)
			}

			exprNd, e := parseExpression(ctx, next, 0)
			if e != nil {
				return nil, e
			}

			return &t.NodeExprAddrof{Tk: tk, Expr: exprNd}, nil

		case t.KwExclam, t.KwMinus, t.KwAsterisk, t.KwAmpersand, t.KwTilde:
			consume(ctx)
			next, e := peek(ctx)
			if e != nil {
				return nil, e
			}
			exp, e := parseUnaryExpr(ctx, next)
			if e != nil {
				return nil, e
			}

			n := &t.NodeExprUnary{
				Tk:       tk,
				Operator: tk.KeywType,
				Operand:  exp,
			}
			return n, nil
		}
	}

	return parsePrimaryExpr(ctx, tk)
}

func tokenEndsExpr(tk t.Token) bool {
	switch tk.KeywType {
	case t.KwNewline, t.KwComma, t.KwParenCl, t.KwColon, t.KwDots, t.KwBrackCl:
		return true
	default:
		return false
	}
}

func getBinaryPrecedence(tk t.Token) int {
	if tk.Type != t.TokKeyword {
		return 0
	}

	switch tk.KeywType {
	case t.KwAsterisk, t.KwPercent, t.KwSlash:
		return 50

	case t.KwPlus, t.KwMinus:
		return 40

	case t.KwShiftLeft, t.KwShiftRight:
		return 35

	case t.KwCmpEq, t.KwCmpNeq, t.KwCmpLt, t.KwCmpGt, t.KwCmpLtEq, t.KwCmpGtEq:
		return 32

	case t.KwAmpersand:
		return 31

	case t.KwCaret:
		return 30

	case t.KwPipe:
		return 29

	case t.KwAndAnd:
		return 28

	case t.KwOrOr:
		return 27

	case t.KwEqual:
		return 20

	case t.KwInfer:
		return 19
	default:
		return 0
	}
}

func parseDestructureAssignAfterComma(ctx *ParseCtx, commaTk t.Token, left t.NodeExpr) (t.NodeExpr, bool, error) {
	if commaTk.KeywType != t.KwComma {
		return nil, false, nil
	}

	// Valid forms are `<name> <type>, <name> <type> = <call>` and
	// `<name>, <name> := <call>`.
	var valueDef *t.NodeExprVarDef
	valueIsInferred := false
	switch n := left.(type) {
	case *t.NodeExprVarDef:
		valueDef = n
	case *t.NodeExprName:
		if _, ok := n.Name.(*t.NodeNameSingle); !ok {
			return nil, false, nil
		}
		secondName, nameErr := peekNth(ctx, 1)
		inferOp, inferErr := peekNth(ctx, 2)
		if nameErr != nil || inferErr != nil || secondName.Type != t.TokName || inferOp.KeywType != t.KwInfer {
			// A plain `name, name` is commonly an argument list. Only claim it as
			// destructuring when the distinctive `:=` follows the second name.
			return nil, false, nil
		}
		valueDef = &t.NodeExprVarDef{Name: n.Name}
		valueIsInferred = true
	default:
		return nil, false, nil
	}

	consume(ctx) // consume comma

	nameTk2, e := peek(ctx)
	if e != nil {
		return nil, true, e
	}
	if nameTk2.Type != t.TokName {
		return nil, true, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&nameTk2,
			fmt.Sprintf("syntax error: expected name after ',' but got '%s'", nameTk2.Repr),
			"expected: `value T, err error = call()`",
		)
	}

	name2, e := parseName(ctx, nameTk2, false)
	if e != nil {
		return nil, true, e
	}
	if _, ok := name2.(*t.NodeNameSingle); !ok {
		return nil, true, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&nameTk2,
			"syntax error: destructuring bindings must be simple names",
			"expected: `value, err := call()`",
		)
	}

	afterName, e := peek(ctx)
	if e != nil {
		return nil, true, e
	}

	var type2 *t.NodeType
	var assignKeyword t.KwType
	if afterName.KeywType == t.KwInfer {
		if !valueIsInferred {
			return nil, true, comp_err.CompilationErrorToken(
				ctx.Fctx,
				&afterName,
				"syntax error: cannot mix typed and inferred destructuring bindings",
				"use either `value T, err error = call()` or `value, err := call()`",
			)
		}
		assignKeyword = t.KwInfer
		consume(ctx)
	} else {
		if valueIsInferred {
			return nil, true, comp_err.CompilationErrorToken(
				ctx.Fctx,
				&afterName,
				fmt.Sprintf("syntax error: inferred destructuring requires ':=' but got '%s'", afterName.Repr),
				"expected: `value, err := call()`",
			)
		}
		type2, e = parseType(ctx, afterName, false)
		if e != nil {
			return nil, true, e
		}
		assignKeyword = t.KwEqual
	}

	if assignKeyword == t.KwEqual {
		eqTk, e := peek(ctx)
		if e != nil {
			return nil, true, e
		}
		if eqTk.KeywType != t.KwEqual {
			return nil, true, comp_err.CompilationErrorToken(
				ctx.Fctx,
				&eqTk,
				fmt.Sprintf("syntax error: expected '=' in destructuring assignment but got '%s'", eqTk.Repr),
				"expected: `value T, err error = call()`",
			)
		}
		consume(ctx) // '='
	}

	rhsTk, e := peek(ctx)
	if e != nil {
		return nil, true, e
	}

	rhsExpr, e := parseExpression(ctx, rhsTk, 0)
	if e != nil {
		return nil, true, e
	}

	// Destructuring captures the error while try propagates it. Combining the
	// two would leave the error binding undefined on the propagated path.
	if tryExpr, ok := rhsExpr.(*t.NodeExprTry); ok {
		return nil, true, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&tryExpr.Tk,
			"cannot use 'try' in a destructuring assignment",
			"remove `try`; destructuring already handles the error",
		)
	}

	callExpr, ok := rhsExpr.(*t.NodeExprCall)
	if !ok {
		return nil, true, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&rhsTk,
			"syntax error: destructuring assignment only supports function calls on the right-hand side",
			"expected: `value T, err error = someFunc(...)`",
		)
	}

	return &t.NodeExprDestructureAssign{
		ValueDef: *valueDef,
		ErrDef:   t.NodeExprVarDef{Name: name2, Type: type2},
		Call:     callExpr,
	}, true, nil
}

func parseDefer(ctx *ParseCtx, tk t.Token) (*t.NodeStmtDefer, error) {
	consume(ctx) // consume defer/onerror

	n := &t.NodeStmtDefer{OnError: tk.KeywType == t.KwOnError}

	next, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	if next.KeywType == t.KwColon {
		n.IsBody = true

		body, e := parseDeferBody(ctx, next)
		if e != nil {
			return nil, e
		}

		n.Body = body
		return n, nil
	}

	expr, e := parseExpression(ctx, next, 0)
	if e != nil {
		return nil, e
	}

	n.Expression = expr
	return n, nil
}

func parseExpression(ctx *ParseCtx, tk t.Token, minPrecedence int) (t.NodeExpr, error) {
	left, e := parseUnaryExpr(ctx, tk)
	if e != nil {
		return nil, e
	}

	for {
		opTk, e := peek(ctx)
		if e != nil {
			return nil, e
		}

		if opTk.KeywType == t.KwComma {
			expr, matched, e := parseDestructureAssignAfterComma(ctx, opTk, left)
			if e != nil {
				return nil, e
			}
			if matched {
				left = expr
				continue
			}
		}

		if tokenEndsExpr(opTk) {
			break
		}

		precedence := getBinaryPrecedence(opTk)
		if precedence == 0 || precedence < minPrecedence {
			break
		}

		consume(ctx)

		rTk, e := peek(ctx)
		if e != nil {
			return nil, e
		}
		// `=` should be right-associative (e.g. `x = y = z` -> `x = (y = z)`).
		nextMinPrecedence := precedence + 1
		if opTk.KeywType == t.KwEqual {
			nextMinPrecedence = precedence
		}
		right, e := parseExpression(ctx, rTk, nextMinPrecedence)
		if e != nil {
			return nil, e
		}

		if opTk.KeywType == t.KwEqual {
			switch vd := left.(type) {
			case *t.NodeExprVarDef:
				varDefAssign := &t.NodeExprVarDefAssign{
					Tk:         opTk,
					VarDef:     vd,
					AssignExpr: right,
				}
				left = varDefAssign
				continue
			case *t.NodeExprName, *t.NodeExprSubscript, *t.NodeExprMemberAccess:
				left = &t.NodeExprAssign{
					Tk:    opTk,
					Left:  left,
					Right: right,
				}
				continue
			case *t.NodeExprUnary:
				if vd.Operator != t.KwAsterisk {
					return nil, comp_err.CompilationErrorToken(
						ctx.Fctx,
						&opTk,
						"syntax error: unary expression is not assignable",
						"only pointer dereference (*) is assignable among unary expressions",
					)
				}
				left = &t.NodeExprAssign{
					Tk:    opTk,
					Left:  left,
					Right: right,
				}
				continue
			default:
				return nil, comp_err.CompilationErrorToken(
					ctx.Fctx,
					&opTk,
					fmt.Sprintf("syntax error: cannot assign to %s", assignmentTargetKind(left)),
					"left side of '=' must be a name, member, subscript, or pointer dereference",
				)
			}
		} else if opTk.KeywType == t.KwInfer {
			switch vd := left.(type) {
			case *t.NodeExprVarDef:
				switch vd.Name.(type) {
				case *t.NodeNameSingle:
					break
				default:
					return nil, comp_err.CompilationErrorToken(
						ctx.Fctx,
						&opTk,
						"syntax error: invalid infered assignment target",
						"left side of ':=' must be a simple name",
					)
				}

				if vd.Type != nil {
					return nil, comp_err.CompilationErrorToken(
						ctx.Fctx,
						&opTk,
						"syntax error: typed infered assignment",
						"left side of ':=' must be a name with no type",
					)
				}

				varDefAssign := &t.NodeExprVarDefAssign{
					Tk: opTk,
					VarDef: &t.NodeExprVarDef{
						Name: vd.Name,
						Type: nil,
					},
					AssignExpr: right,
				}
				left = varDefAssign
				continue
			default:
				return nil, comp_err.CompilationErrorToken(
					ctx.Fctx,
					&opTk,
					"syntax error: invalid infered assignment target",
					"left side of ':=' must be an assignable expression (e.g. a name)",
				)
			}
		}

		binaryNd := &t.NodeExprBinary{
			Tk:       opTk,
			Operator: opTk.KeywType,
			Left:     left,
			Right:    right,
		}
		left = binaryNd
	}

	return left, nil
}

func assignmentTargetKind(expr t.NodeExpr) string {
	switch expr.(type) {
	case *t.NodeExprLit:
		return "a literal"
	case *t.NodeExprCall:
		return "a function call result"
	case *t.NodeExprBinary:
		return "a binary expression"
	default:
		return "this expression"
	}
}
