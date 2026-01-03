package parser

import (
	"Magma/src/comp_err"
	"Magma/src/makeabs"
	t "Magma/src/types"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

var errOutOfBounds error = errors.New("oob")

type ModifierType string

const (
	MdPublic ModifierType = "pub"
)

type ParseCtx struct {
	Shared          *t.SharedState
	GlobalNode      *t.NodeGlobal
	Fctx            *t.FileCtx
	Toks            []t.Token
	TokIdx          int
	NextModifiers   []ModifierType
	CurrentFunction *t.NodeFuncDef
}

func peek(ctx *ParseCtx) (t.Token, error) {
	if ctx.TokIdx >= len(ctx.Toks) {
		return t.Token{}, errOutOfBounds
	}
	return ctx.Toks[ctx.TokIdx], nil
}

func peekNth(ctx *ParseCtx, n int) (t.Token, error) {
	nthIdx := ctx.TokIdx + n
	if nthIdx >= len(ctx.Toks) || nthIdx < 0 {
		return t.Token{}, errOutOfBounds
	}
	return ctx.Toks[nthIdx], nil
}

func consume(ctx *ParseCtx) {
	ctx.TokIdx += 1
}

func ensureNoModifiers(ctx *ParseCtx, tk t.Token) error {
	if len(ctx.NextModifiers) > 0 {
		list := []string{}

		for _, x := range ctx.NextModifiers {
			list = append(list, string(x))
		}

		return comp_err.CompilationErrorToken(
			ctx.Fctx,
			&tk,
			fmt.Sprintf("syntax error: unexpected modifier(s) [%s] applied to '%s'", strings.Join(list, ", "), tk.Repr),
			"",
		)
	}
	return nil
}

func parseApplyModifier(ctx *ParseCtx, tk t.Token, md ModifierType) error {
	// TODO: apply modifiers to decl

	if slices.Contains(ctx.NextModifiers, md) {
		return comp_err.CompilationErrorToken(
			ctx.Fctx,
			&tk,
			fmt.Sprintf("syntax error: duplicate modifier '%s'", tk.Repr),
			"only one modifier each can be applied to a declaration",
		)
	}

	ctx.NextModifiers = append(ctx.NextModifiers, md)
	consume(ctx)
	return nil
}

func parseModuleDecl(ctx *ParseCtx, _ t.Token) error {
	// WARNING: Now handled in pipeline
	consume(ctx) // mod
	consume(ctx) // name
	consume(ctx) // newln

	/*
		e := ensureNoModifiers(ctx, tk)
		if e != nil {
			return e
		}

		name, e := peekNth(ctx, 1)
		if e != nil || name.Type != t.TokName {
			return comp_err.CompilationErrorToken(
				ctx.Fctx,
				&tk,
				"syntax error: expected module name after 'mod'",
				"expected: `mod <name>`",
			)
		}

		newln, e := peekNth(ctx, 2)
		if (e != nil && !errors.Is(e, errOutOfBounds)) || newln.KeywType != t.KwNewline {
			return comp_err.CompilationErrorToken(
				ctx.Fctx,
				&tk,
				fmt.Sprintf("syntax error: expected end of line after module name but got '%s'", newln.Repr),
				"expected: `mod <name>(\\n)`",
			)
		}

		if ctx.Fctx.PackageName != "" {
			return comp_err.CompilationErrorToken(
				ctx.Fctx,
				&tk,
				fmt.Sprintf("syntax error: previously declared module as '%s'", ctx.Fctx.PackageName),
				"only a single 'mod' declaration is allowed within the same file",
			)
		}

		ctx.Fctx.PackageName = name.Repr
		consume(ctx)
		consume(ctx)
		consume(ctx)
	*/
	return nil
}

func parseUseDecl(ctx *ParseCtx, tk t.Token) error {
	e := ensureNoModifiers(ctx, tk)
	if e != nil {
		return e
	}

	path, e := peekNth(ctx, 1)
	if e != nil || path.Type != t.TokLitStr {
		return comp_err.CompilationErrorToken(
			ctx.Fctx,
			&tk,
			"syntax error: expected file path after 'use'",
			"expected: `use \"<filepath>\" <alias>`",
		)
	}

	alias, e := peekNth(ctx, 2)
	if e != nil || alias.Type != t.TokName {
		return comp_err.CompilationErrorToken(
			ctx.Fctx,
			&tk,
			"syntax error: expected alias after file path in 'use' statement",
			"expected: `use \"<filepath>\" <alias>`",
		)
	}

	newln, e := peekNth(ctx, 3)
	if e != nil && !errors.Is(e, errOutOfBounds) {
		return comp_err.CompilationErrorToken(
			ctx.Fctx,
			&tk,
			fmt.Sprintf("syntax error: expected end of line after file path but got '%s'", newln.Repr),
			"expected: `use \"<filepath>\" <alias>(\\n)`",
		)
	}

	_, ok := ctx.Fctx.ImportAlias[alias.Repr]
	if ok {
		return comp_err.CompilationErrorToken(
			ctx.Fctx,
			&alias,
			fmt.Sprintf("syntax error: already using a module with alias of '%s'", alias.Repr),
			"cannot reuse module aliases within the same file",
		)
	}

	absPath, err := makeabs.MakeAbs(path.Repr, ctx.Fctx.FilePath)
	if err != nil {
		return comp_err.CompilationErrorToken(
			ctx.Fctx,
			&path,
			fmt.Sprintf("syntax error: failed to get full path from '%s' (%s)", path.Repr, err.Error()),
			"",
		)
	}

	if slices.Contains(ctx.Fctx.Imports, absPath) {
		return comp_err.CompilationErrorToken(
			ctx.Fctx,
			&path,
			"syntax error: already using module from another 'use' declaration within this file",
			"cannot use the same module multiple times within the same file",
		)
	}

	consume(ctx) // use
	consume(ctx) // path
	consume(ctx) // alias
	consume(ctx) // newln

	ctx.Fctx.Imports = append(ctx.Fctx.Imports, absPath)
	ctx.Fctx.ImportAlias[alias.Repr] = absPath

	// start pipeline for imported file
	fmt.Printf("running compilation pipeline for file: %s\n", absPath)
	c := ctx.Shared.PipelineFunc(ctx.Shared, absPath, alias.Repr, ctx.Fctx.FilePath, ctx.GlobalNode)

	ctx.Shared.PipeChansM.Lock()
	ctx.Shared.PipeChans = append(ctx.Shared.PipeChans, c)
	ctx.Shared.PipeChansM.Unlock()
	return nil
}

func parseSimplePrimaryExpr(ctx *ParseCtx, tk t.Token) (t.NodeExpr, error) {
	if tk.KeywType == t.KwParenOp {
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
		expr, e := parseExpression(ctx, next, 0)
		if e != nil {
			return nil, e
		}
		n := &t.NodeExprTry{
			Call: expr,
		}
		return n, nil
	}

	if tk.KeywType == t.KwTrue || tk.KeywType == t.KwFalse {
		consume(ctx)
		boolVal := "0"
		if tk.KeywType == t.KwTrue {
			boolVal = "1"
		}
		return &t.NodeExprLit{Value: boolVal, LitType: t.TokLitBool}, nil
	}

	if tk.Type == t.TokLitNum || tk.Type == t.TokLitStr {
		consume(ctx)
		return &t.NodeExprLit{Value: tk.Repr, LitType: tk.Type}, nil
	}

	if tk.Type == t.TokName {
		n, e := parseName(ctx, tk, true)
		if e != nil {
			return nil, e
		}
		return &t.NodeExprName{Name: n}, nil
	}

	return nil, comp_err.CompilationErrorToken(
		ctx.Fctx, &tk,
		fmt.Sprintf("syntax error: unexpected '%s' in expression", tk.Repr),
		"",
	)
}

func parsePostfixCallExpr(ctx *ParseCtx, tk t.Token, calleeExpr t.NodeExpr) (*t.NodeExprCall, error) {
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
				ctx.Fctx, &tk,
				fmt.Sprintf("syntax error: unexpected '%s' in call argument expression list", tk.Repr),
				"",
			)
		}
	}

	return &t.NodeExprCall{
		Callee: calleeExpr,
		Args:   argExprs,
	}, nil
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
		Target: targetExpr,
		Expr:   parsedExpr,
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
			expr, e = parsePostfixCallExpr(ctx, tk, expr)
			if e != nil {
				return nil, e
			}
			continue
		}

		if next.KeywType == t.KwBrackOp {
			expr, e = parsePostfixSubscriptExpr(ctx, tk, expr)
			if e != nil {
				return nil, e
			}
			continue
		}

		if expr == baseExpr {
			switch n := baseExpr.(type) {
			case *t.NodeExprName:
				// Only treat `name <type>` as a variable definition when the next token
				// can actually start a type. Otherwise this would incorrectly swallow
				// valid expressions like `x = y` by trying to parse `=` as a type.
				if next.Type != t.TokName {
					break
				}
				typeNd, e := parseType(ctx, next, false)
				if e != nil {
					return nil, e
				}

				/*
					maybeComma, e := peek(ctx)
					if e != nil {
						return nil, e
					}

					if maybeComma.KeywType == t.KwComma {
						expr2, e := parseSimplePrimaryExpr(ctx, tk)
						if e != nil {
							return nil, e
						}

						switch expr2.(type) {
						case *t.NodeExprName:
							typeNd2, e := parseType(ctx, next, false)
							if e != nil {
								return nil, e
							}
						}
					}*/

				return &t.NodeExprVarDef{
					Name: n.Name,
					Type: typeNd,
				}, nil
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
	if tk.Type != t.TokKeyword {
		switch tk.KeywType {
		case t.KwExclam, t.KwMinus, t.KwAsterisk, t.KwAmpersand:
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
				Operator: tk.KeywType,
				Operand:  exp,
			}
			return n, nil
		}
	}

	return parsePrimaryExpr(ctx, tk)
}

func tokenEndsExpr(tk t.Token) bool {
	if tk.KeywType == t.KwNewline {
		return true
	}

	switch tk.KeywType {
	case t.KwComma, t.KwParenCl, t.KwColon, t.KwDots, t.KwBrackCl:
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
	case t.KwAsterisk:
		// TODO: implement
		//case KW_TYPE_SLASH:
		//case KW_TYPE_PERCENT:
		return 50
	case t.KwPlus, t.KwMinus:
		return 40

	// TODO: implement
	//case KW_TYPE_CMPEQUAL:
	//case KW_TYPE_CMPNEQUAL:
	//	return 35
	case t.KwCmpEq, t.KwCmpNeq:
		return 32

	case t.KwEqual:
		return 20
	default:
		return 0
	}
}

func parseDestructureAssignAfterComma(ctx *ParseCtx, commaTk t.Token, left t.NodeExpr) (t.NodeExpr, bool, error) {
	if commaTk.KeywType != t.KwComma {
		return nil, false, nil
	}

	// Only valid for `<name> <type>, <name> <type> = <call>`
	vd, ok := left.(*t.NodeExprVarDef)
	if !ok {
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

	typeTk2, e := peek(ctx)
	if e != nil {
		return nil, true, e
	}
	type2, e := parseType(ctx, typeTk2, false)
	if e != nil {
		return nil, true, e
	}

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

	rhsTk, e := peek(ctx)
	if e != nil {
		return nil, true, e
	}

	rhsExpr, e := parseExpression(ctx, rhsTk, 0)
	if e != nil {
		return nil, true, e
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
		ValueDef: *vd,
		ErrDef:   t.NodeExprVarDef{Name: name2, Type: type2},
		Call:     callExpr,
	}, true, nil
}

func parseDefer(ctx *ParseCtx, tk t.Token) (*t.NodeStmtDefer, error) {
	consume(ctx) // consume defer

	n := &t.NodeStmtDefer{}

	next, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	if next.KeywType == t.KwColon {
		n.IsBody = true

		body, e := parseBody(ctx, next)
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
					VarDef:     *vd,
					AssignExpr: right,
				}
				left = varDefAssign
				continue
			case *t.NodeExprName:
				left = &t.NodeExprAssign{
					Left:  left,
					Right: right,
				}
				continue
			default:
				return nil, comp_err.CompilationErrorToken(
					ctx.Fctx,
					&opTk,
					"syntax error: invalid assignment target",
					"left side of '=' must be an assignable expression (e.g. a name)",
				)
			}
		}

		binaryNd := &t.NodeExprBinary{
			Operator: opTk.KeywType,
			Left:     left,
			Right:    right,
		}
		left = binaryNd
	}

	return left, nil
}

func parseName(ctx *ParseCtx, tk t.Token, allowComposite bool) (t.NodeName, error) {
	parts := []string{}

	for {
		namePart, e := peek(ctx)
		if e != nil {
			return nil, e
		}

		if namePart.Type != t.TokName {
			return nil, comp_err.CompilationErrorToken(
				ctx.Fctx,
				&tk,
				"syntax error: expected name after dot",
				"",
			)
		}

		parts = append(parts, namePart.Repr)
		consume(ctx)

		maybeDot, e := peek(ctx)
		if e != nil {
			if errors.Is(e, errOutOfBounds) {
				break
			}
			return nil, e
		}

		if maybeDot.KeywType != t.KwDot {
			break
		}

		if !allowComposite {
			return nil, comp_err.CompilationErrorToken(
				ctx.Fctx,
				&tk,
				"syntax error: context does not allow for name to be a composite name",
				"a name chain joined by '.' is a composite name, some contexts do not allow them.",
			)
		}
		consume(ctx)
	}

	switch len(parts) {
	case 0:
		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&tk,
			"syntax error: name parsing failure, unexpected state",
			"",
		)
	case 1:
		return &t.NodeNameSingle{
			Name: parts[0],
		}, nil
	default:
		return &t.NodeNameComposite{
			Parts: parts,
		}, nil
	}
}

func parseArgument(ctx *ParseCtx) (t.NodeArg, error) {
	name, e := peek(ctx)
	if e != nil {
		return t.NodeArg{}, e
	}

	if name.Type != t.TokName {
		return t.NodeArg{}, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&name,
			fmt.Sprintf("syntax error: expected argument name but got '%s'", name.Repr),
			"expected: `(name type, ...)`",
		)
	}

	consume(ctx)

	typeTk, e := peek(ctx)
	if e != nil {
		return t.NodeArg{}, e
	}

	ndType, e := parseType(ctx, typeTk, false)
	if e != nil {
		return t.NodeArg{}, e
	}

	return t.NodeArg{
		Name:     name.Repr,
		TypeNode: ndType,
	}, nil
}

func parseArgsList(ctx *ParseCtx) (t.NodeArgList, error) {
	n := t.NodeArgList{
		Args: make([]t.NodeArg, 0),
	}

	openPar, e := peek(ctx)
	if e != nil {
		return t.NodeArgList{}, e
	}
	if openPar.KeywType != t.KwParenOp {
		return t.NodeArgList{}, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&openPar,
			fmt.Sprintf("syntax error: expected '(' but got '%s'", openPar.Repr),
			"",
		)
	}
	consume(ctx)

	for {
		tk, e := peek(ctx)
		if e != nil {
			return t.NodeArgList{}, e
		}

		// TODO: func drainNewLines()
		if tk.KeywType == t.KwNewline {
			consume(ctx)
			continue
		}

		if tk.KeywType == t.KwParenCl {
			consume(ctx)
			return n, nil
		}

		argNode, e := parseArgument(ctx)
		if e != nil {
			return t.NodeArgList{}, e
		}
		n.Args = append(n.Args, argNode)

		tk, e = peek(ctx)
		if e != nil {
			return t.NodeArgList{}, e
		}

		if tk.KeywType != t.KwParenCl && tk.KeywType != t.KwComma && tk.KeywType != t.KwNewline {
			return t.NodeArgList{}, comp_err.CompilationErrorToken(
				ctx.Fctx,
				&tk,
				fmt.Sprintf("syntax error: unexpected '%s' when expected ',' or ')'", tk.Repr),
				"expected args list format: `()`, `(name type)`, `(name type, ...)`",
			)
		}

		if tk.KeywType == t.KwComma {
			consume(ctx)
		}
	}
}

func parseGenericClass(ctx *ParseCtx, nameNode t.NodeName) (t.NodeGenericClass, error) {
	n := t.NodeGenericClass{
		NameNode: nameNode,
	}
	al, e := parseArgsList(ctx)
	if e != nil {
		return t.NodeGenericClass{}, e
	}
	n.ArgsNode = al
	return n, nil
}

func parseFuncType(ctx *ParseCtx) (*t.NodeType, error) {
	outT := &t.NodeType{}

	fnT := &t.NodeTypeFunc{
		Args: []*t.NodeType{},
	}

	outT.KindNode = fnT

	tk, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	if tk.KeywType != t.KwParenOp {
		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&tk,
			"expected function type but type does not start with '('",
			"",
		)
	}
	consume(ctx)

	expectComma := false

	for {
		tk, e = peek(ctx)
		if e != nil {
			return nil, e
		}

		if tk.KeywType == t.KwParenCl {
			consume(ctx)
			break
		}

		if expectComma && tk.KeywType != t.KwComma {
			return nil, comp_err.CompilationErrorToken(
				ctx.Fctx,
				&tk,
				fmt.Sprintf("expected ',' after argument type in function type definition. instead got '%s'", tk.Repr),
				"",
			)
		}

		if tk.KeywType == t.KwComma {
			expectComma = false
			consume(ctx)
			tk, e = peek(ctx)
			if e != nil {
				return nil, e
			}
		}

		n, e := parseType(ctx, tk, false)
		if e != nil {
			return nil, e
		}

		fnT.Args = append(fnT.Args, n)
		expectComma = true
	}

	tk, e = peek(ctx)
	if e != nil {
		return nil, e
	}

	n, e := parseType(ctx, tk, true)
	if e != nil {
		return nil, e
	}

	fnT.RetType = n

	return outT, nil
}

func parseTypePostfix(ctx *ParseCtx, inType *t.NodeType) (*t.NodeType, error) {

	outT := inType

	for {
		after, e := peek(ctx)
		if e != nil {
			return nil, e
		}

		// parse array types / (future) templated types
		if after.KeywType == t.KwBrackOp {
			consume(ctx)

			maybeCl, e := peek(ctx)
			if e != nil {
				return nil, e
			}

			size := int64(-1)

			if maybeCl.Type == t.TokLitNum {
				consume(ctx)

				size, e = strconv.ParseInt(maybeCl.Repr, 10, 64)
				if e != nil {
					return nil, e
				}

				maybeCl, e = peek(ctx)
				if e != nil {
					return nil, e
				}
			}

			if maybeCl.KeywType == t.KwBrackCl {
				consume(ctx)

				sliceKind := &t.NodeTypeSlice{
					ElemKind: outT.KindNode,
				}

				sliceT := &t.NodeType{
					Throws:   outT.Throws,
					KindNode: sliceKind,
				}

				if size > -1 {
					sliceKind.HasSize = true
					sliceKind.Size = int(size)
				}

				outT = sliceT
				continue
			}
		}

		if after.KeywType == t.KwAsterisk {
			consume(ctx)

			sliceKind := &t.NodeTypePointer{
				Kind: outT.KindNode,
			}

			ptrT := &t.NodeType{
				Throws:   outT.Throws,
				KindNode: sliceKind,
			}

			outT = ptrT
			continue
		}

		if after.KeywType == t.KwDollar {
			consume(ctx)

			sliceKind := &t.NodeTypeRfc{
				Kind: outT.KindNode,
			}

			ptrT := &t.NodeType{
				Throws:   outT.Throws,
				KindNode: sliceKind,
			}

			outT = ptrT
			continue
		}

		break
	}

	return outT, nil
}

func parseType(ctx *ParseCtx, tk t.Token, allowThrow bool) (*t.NodeType, error) {
	isThrowing := false
	if tk.KeywType == t.KwExclam {
		if !allowThrow {
			return nil, comp_err.CompilationErrorToken(
				ctx.Fctx,
				&tk,
				"syntax error: context does not allow for type to be a throwing type",
				"a type prefixed by '!' is a throwing type, some contexts do not allow them.",
			)
		}
		isThrowing = true
		consume(ctx)
	}

	tk, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	if tk.KeywType == t.KwParenOp {
		n, e := parseFuncType(ctx)
		if e != nil {
			return nil, e
		}
		return n, nil
	}

	named, e := parseName(ctx, tk, true)
	if e != nil {
		return nil, e
	}

	outT := &t.NodeType{
		Throws: isThrowing,
		KindNode: &t.NodeTypeNamed{
			NameNode: named,
		},
	}

	tk, e = peek(ctx)
	if e != nil {
		return nil, e
	}

	outTpost, e := parseTypePostfix(ctx, outT)
	if e != nil {
		return nil, e
	}

	return outTpost, nil
}

func parseStmtReturn(ctx *ParseCtx) (t.NodeStatement, error) {
	consume(ctx) // consume ret kw

	next, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	if next.KeywType == t.KwNewline {
		return &t.NodeStmtRet{Expression: &t.NodeExprVoid{}}, nil
	}

	expr, e := parseExpression(ctx, next, 0)
	if e != nil {
		return nil, e
	}

	return &t.NodeStmtRet{Expression: expr}, nil
}

func parseStmtThrow(ctx *ParseCtx) (t.NodeStatement, error) {
	consume(ctx) // consume ret kw

	next, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	expr, e := parseExpression(ctx, next, 0)
	if e != nil {
		return nil, e
	}

	return &t.NodeStmtThrow{Expression: expr}, nil
}

func parseStatement(ctx *ParseCtx, tk t.Token) (t.NodeStatement, error) {
	switch tk.KeywType {
	case t.KwReturn:
		return parseStmtReturn(ctx)
	case t.KwThrow:
		return parseStmtThrow(ctx)
	case t.KwLlvm:
		return parseLlvm(ctx, tk)
	case t.KwIf:
		return parseStmtIf(ctx, tk)
	case t.KwWhile:
		return parseStmtWhile(ctx, tk)
	case t.KwDefer:
		n, e := parseDefer(ctx, tk)
		if e != nil {
			return nil, e
		}
		ctx.CurrentFunction.HasDefer = true
		ctx.CurrentFunction.Deferred = append(ctx.CurrentFunction.Deferred, n)
		return n, nil
	}

	expr, e := parseExpression(ctx, tk, 0)
	if e != nil {
		comp_err.CompilationErrorToken(
			ctx.Fctx,
			&tk,
			fmt.Sprintf("syntax error: '%s' is not a valid start of statement", tk.Repr),
			"valid statements include: `name: type = expr`, `name()`, `ret expr`, etc.",
		)
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

		if tk.KeywType == t.KwDefer {
			return n, comp_err.CompilationErrorToken(
				ctx.Fctx,
				&tk,
				"syntax error: cannot nest defer statements",
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

func ensureSimpleName(ctx *ParseCtx, tk t.Token, name t.NodeName) error {
	switch n := name.(type) {
	case *t.NodeNameComposite:
		// TODO: associate nodes with tokens for better error reporting
		return comp_err.CompilationErrorToken(
			ctx.Fctx,
			&tk,
			fmt.Sprintf("syntax error: complex name: '%s' not allowed in this context, expected simple name", strings.Join(n.Parts, ".")),
			"cannot define a struct with a name containing a '.'",
		)
	}
	return nil
}

func parseStructDef(ctx *ParseCtx, tk t.Token, gncls t.NodeGenericClass) (*t.NodeStructDef, error) {

	// check if struct name is valid (complex name not allowed)
	e := ensureSimpleName(ctx, tk, gncls.NameNode)
	if e != nil {
		return nil, e
	}

	simpleName := gncls.NameNode.(*t.NodeNameSingle)

	// create struct def in global node for easir type checking later
	structMap := &t.StructDef{
		Module:  ctx.Fctx.PackageName,
		Name:    simpleName.Name,
		Fields:  map[string]*t.NodeType{},
		Funcs:   map[string]*t.NodeFuncDef{},
		FieldNb: map[string]int{},
	}

	for i, arg := range gncls.ArgsNode.Args {
		structMap.Fields[arg.Name] = arg.TypeNode
		structMap.FieldNb[arg.Name] = i
	}

	ctx.GlobalNode.StructDefs[simpleName.Name] = structMap

	return &t.NodeStructDef{
		Class: gncls,
	}, nil
}

func parseFuncDef(ctx *ParseCtx, nameTk t.Token, after t.Token, gncls t.NodeGenericClass) (*t.NodeFuncDef, error) {
	isMemberFunc := false
	fnNameSimple := ""

	switch n := gncls.NameNode.(type) {
	case *t.NodeNameComposite:
		isMemberFunc = true
		fnNameSimple = strings.Join(n.Parts, ".")
	case *t.NodeNameSingle:
		fnNameSimple = n.Name
	}

	fnDef := &t.NodeFuncDef{
		Class: gncls,
	}

	ctx.CurrentFunction = fnDef
	defer func() {
		ctx.CurrentFunction = nil
	}()

	typeNode, e := parseType(ctx, after, true)
	if e != nil {
		return nil, e
	}

	bodyStart, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	bodyNode, e := parseBody(ctx, bodyStart)
	if e != nil {
		return nil, e
	}

	fnDef.Body = bodyNode
	fnDef.ReturnType = typeNode

	if isMemberFunc {
		complexName := gncls.NameNode.(*t.NodeNameComposite)
		if len(complexName.Parts) > 2 {
			return nil, comp_err.CompilationErrorToken(
				ctx.Fctx,
				&nameTk,
				fmt.Sprintf("syntax error: too many parts in complex name: '%s' a function definition should have 1 or 2 parts, no more", strings.Join(complexName.Parts, ".")),
				"expected: `<name> (<args>) <type>:` or `<structname>.<name> (<args>) <type>:` ",
			)
		}

		ownerName := complexName.Parts[0]
		memberName := complexName.Parts[1]

		// check if type exists in file (at least before member func)
		// if not, this is sign of garbage code, so I don't feel bad about
		// making this a compiler error
		_, ok := ctx.GlobalNode.StructDefs[ownerName]
		if !ok {
			return nil, comp_err.CompilationErrorToken(
				ctx.Fctx,
				&nameTk,
				fmt.Sprintf("syntax error: defined member function for '%s', but the struct was not defined in this file", ownerName),
				"member functions need to be defined after the owner struct",
			)
		}

		fmt.Printf("added implicit this to: %s.%s()\n", ownerName, memberName)

		fnDef.Class.ArgsNode.Args = slices.Insert(fnDef.Class.ArgsNode.Args, 0, t.NodeArg{
			Name: "this",
			TypeNode: &t.NodeType{
				KindNode: &t.NodeTypePointer{
					Kind: &t.NodeTypeNamed{
						NameNode: &t.NodeNameSingle{Name: ownerName},
					},
				},
			},
		})

		ctx.GlobalNode.StructDefs[ownerName].Funcs[memberName] = fnDef
	}

	ctx.GlobalNode.FuncDefs[fnNameSimple] = fnDef
	return fnDef, nil
}

func parseGlobalDeclFromName(ctx *ParseCtx, tk t.Token) (t.NodeGlobalDecl, error) {
	n, e := parseName(ctx, tk, true)
	if e != nil {
		return nil, e
	}

	next, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	if next.Type != t.TokKeyword {
		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&next,
			fmt.Sprintf("syntax error: unexpected '%s' after name in global declaration", next.Repr),
			"expected in global scope: `<name> :`, `<name> (",
		)
	}

	switch next.KeywType {
	case t.KwParenOp:
		// TODO: apply modifiers
		ctx.NextModifiers = []ModifierType{}

		gncls, e := parseGenericClass(ctx, n)
		if e != nil {
			return nil, e
		}

		after, e := peek(ctx)
		if e != nil && !errors.Is(e, errOutOfBounds) {
			return nil, e
		}

		if errors.Is(e, errOutOfBounds) || after.KeywType == t.KwNewline {
			return parseStructDef(ctx, tk, gncls)
		}

		return parseFuncDef(ctx, tk, after, gncls)
	default:
		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&next,
			fmt.Sprintf("syntax error: unexpected '%s' after name in global declaration", next.Repr),
			"expected in global scope: `<name> :`, `<name> (",
		)
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
		CondExpr: condExpr,
	}

	body, e := parseBody(ctx, next2)
	if e != nil {
		return nil, e
	}

	whileStmt.Body = body
	return whileStmt, nil
}

func parseLlvm(ctx *ParseCtx, tk t.Token) (*t.NodeLlvm, error) {
	consume(ctx) // consume llvm kw

	next, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	if next.Type == t.TokLitStr {
		consume(ctx)
		return &t.NodeLlvm{Text: next.Repr}, nil
	}

	return nil, comp_err.CompilationErrorToken(
		ctx.Fctx,
		&next,
		fmt.Sprintf("syntax error: unexpected '%s' after 'llvm' keyword", next.Repr),
		"expected: `llvm \"<llvm text>\"`",
	)
}

func parseGlobalDecl(ctx *ParseCtx, tk t.Token) (t.NodeGlobalDecl, error) {
	var n t.NodeGlobalDecl = nil
	var e error = nil

outer:
	switch tk.Type {
	case t.TokName:
		n, e := parseGlobalDeclFromName(ctx, tk)
		if e != nil {
			return nil, e
		}
		return n, nil

	case t.TokKeyword:
		switch tk.KeywType {
		case t.KwNewline:
			consume(ctx)
			return nil, nil
		case t.KwPublic:
			e = parseApplyModifier(ctx, tk, MdPublic)
		case t.KwModule:
			e = parseModuleDecl(ctx, tk)
		case t.KwUse:
			e = parseUseDecl(ctx, tk)
		case t.KwLlvm:
			return parseLlvm(ctx, tk)

		default:
			break outer
		}
		if e != nil {
			return nil, e
		}
		return n, e

	default:
		break

	}
	return nil, comp_err.CompilationErrorToken(
		ctx.Fctx,
		&tk,
		fmt.Sprintf("syntax error: unexpected '%s' in global scope", tk.Repr),
		"expected in global scope: `name: type = expr`, `name ( args, ... ) type`, etc.",
	)
}

func parseGlobal(ctx *ParseCtx) (*t.NodeGlobal, error) {
	n := &t.NodeGlobal{
		StructDefs: map[string]*t.StructDef{},
		FuncDefs:   map[string]*t.NodeFuncDef{},

		Declarations: []t.NodeGlobalDecl{},
		ImportAlias:  map[string]string{},
	}
	ctx.GlobalNode = n

	for {
		tk, e := peek(ctx)
		if e != nil {
			if errors.Is(e, errOutOfBounds) {
				return n, nil
			}
			return nil, e
		}

		if tk.KeywType == t.KwNewline {
			consume(ctx)
			continue
		}

		glDecl, e := parseGlobalDecl(ctx, tk)
		if e != nil {
			return nil, e
		}

		// this is sketch af
		// we do this since some valid declarations won't return a node
		if glDecl != nil {
			n.Declarations = append(n.Declarations, glDecl)
		}
	}
}

func Parse(shared *t.SharedState, fCtx *t.FileCtx) (*t.NodeGlobal, error) {
	ctx := &ParseCtx{
		Shared: shared,
		Fctx:   fCtx,
		Toks:   fCtx.Tokens,
	}

	glNd, e := parseGlobal(ctx)
	if e != nil {
		if errors.Is(e, errOutOfBounds) {
			var last t.Token
			if len(ctx.Toks) > 0 {
				last, _ = peekNth(ctx, -1)
			}

			return glNd, comp_err.CompilationErrorToken(
				ctx.Fctx, &last,
				"syntax error: reached end of file prematurely",
				"",
			)
		}
		return glNd, e
	}
	return glNd, nil
}
