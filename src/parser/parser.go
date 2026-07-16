package parser

import (
	"Magma/src/comp_err"
	"Magma/src/makeabs"
	t "Magma/src/types"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
)

var errOutOfBounds error = errors.New("oob")

type ModifierType string

const (
	MdPublic     ModifierType = "pub"
	MdDestructor ModifierType = "destr"
)

type ParseCtx struct {
	Shared          *t.SharedState
	GlobalNode      *t.NodeGlobal
	Fctx            *t.FileCtx
	Toks            []t.Token
	TokIdx          int
	NextModifiers   []ModifierType
	CurrentFunction *t.NodeFuncDef

	PruneNext bool
}

type parsedName struct {
	First    string
	Parts    []string
	HasParts bool
}

func parseNameNode(name t.NodeName) parsedName {
	switch n := name.(type) {
	case *t.NodeNameSingle:
		return parsedName{
			First:    n.Name,
			HasParts: false,
		}
	case *t.NodeNameComposite:
		return parsedName{
			First:    n.Parts[0],
			HasParts: true,
			Parts:    n.Parts[1:],
		}
	}
	return parsedName{}
}

func flattenName(name t.NodeName) string {
	s := ""

	parsed := parseNameNode(name)

	s += parsed.First
	if parsed.HasParts {
		for _, x := range parsed.Parts {
			s += "." + x
		}
	}
	return s
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

func parseUseDecl(ctx *ParseCtx, tk t.Token, prune bool) error {
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
	if ok && !prune { // alias shadowing is valid state if we will prune the use afterwards
		return comp_err.CompilationErrorToken(
			ctx.Fctx,
			&alias,
			fmt.Sprintf("syntax error: already using a module with alias of '%s' in file '%s'", alias.Repr, ctx.Fctx.PackageName),
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

	if slices.Contains(ctx.Fctx.Imports, absPath) && !prune { // file import shadowing is valid state if we will prune the use afterwards
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

	if prune {
		//fmt.Printf("pruning use decl for: \"%s\" %s\n", path.Repr, alias.Repr)
		return nil
	}

	ctx.Fctx.Imports = append(ctx.Fctx.Imports, absPath)
	ctx.Fctx.ImportAlias[alias.Repr] = absPath

	// start pipeline for imported file
	//("running compilation pipeline for file: %s\n", absPath)
	c := ctx.Shared.PipelineFunc(ctx.Shared, absPath, alias.Repr, ctx.Fctx.FilePath, ctx.GlobalNode)

	ctx.Shared.PipeChansM.Lock()
	ctx.Shared.PipeChans = append(ctx.Shared.PipeChans, c)
	ctx.Shared.PipeChansM.Unlock()
	return nil
}

func parseLinkDecl(ctx *ParseCtx, tk t.Token, prune bool) error {
	if err := ensureNoModifiers(ctx, tk); err != nil {
		return err
	}
	library, err := peekNth(ctx, 1)
	if err != nil || library.Type != t.TokLitStr || library.Repr == "" {
		return comp_err.CompilationErrorToken(ctx.Fctx, &tk,
			"syntax error: expected native library name after 'link'",
			"expected: `link \"<library>\"`")
	}
	newline, err := peekNth(ctx, 2)
	if err == nil && newline.KeywType != t.KwNewline {
		return comp_err.CompilationErrorToken(ctx.Fctx, &newline,
			fmt.Sprintf("syntax error: expected end of line after library name but got '%s'", newline.Repr),
			"expected: `link \"<library>\"(\\n)`")
	}
	consume(ctx)
	consume(ctx)
	consume(ctx)
	if prune {
		return nil
	}
	requirement := library.Repr
	// Values that look like files are module-relative inputs passed directly to
	// Clang. Bare logical names retain the portable -l<name> behavior.
	if filepath.IsAbs(requirement) || strings.ContainsAny(requirement, `/\`) || filepath.Ext(requirement) != "" {
		if !filepath.IsAbs(requirement) {
			requirement = filepath.Join(filepath.Dir(ctx.Fctx.FilePath), requirement)
		}
		requirement = filepath.Clean(requirement)
	}
	if !slices.Contains(ctx.Fctx.NativeLibraries, requirement) {
		ctx.Fctx.NativeLibraries = append(ctx.Fctx.NativeLibraries, requirement)
	}
	return nil
}

func parseSimplePrimaryExpr(ctx *ParseCtx, tk t.Token) (t.NodeExpr, error) {
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

	if tk.KeywType == t.KwNoneLit {
		consume(ctx)
		return &t.NodeExprLit{Value: "null", LitType: t.TokLitNone}, nil
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
				ctx.Fctx, &tk,
				fmt.Sprintf("syntax error: unexpected '%s' in call argument expression list", tk.Repr),
				"",
			)
		}
	}

	return &t.NodeExprCall{
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
		fields = append(fields, t.NodeStructFieldInit{Name: fieldTk.Repr, Expression: value, FieldIndex: -1})

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
			if isStructInitList(ctx) {
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
					if isStructInitList(ctx) {
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
			}

			expr, e = parsePostfixSubscriptExpr(ctx, tk, expr)
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
				// Only treat `name <type>` as a variable definition when the next token
				// can actually start a type. Otherwise this would incorrectly swallow
				// valid expressions like `x = y` by trying to parse `=` as a type.
				if next.Type != t.TokName && next.KeywType != t.KwInfer {
					break
				}

				if next.Type == t.TokName {
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

			return &t.NodeExprSizeof{Type: typeNd}, nil

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

			return &t.NodeExprAddrof{Expr: exprNd}, nil

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
					Tk:         tk,
					VarDef:     vd,
					AssignExpr: right,
				}
				left = varDefAssign
				continue
			case *t.NodeExprName, *t.NodeExprSubscript, *t.NodeExprMemberAccess:
				left = &t.NodeExprAssign{
					Tk:    tk,
					Left:  left,
					Right: right,
				}
				continue
			case *t.NodeExprUnary:
				if vd.Operator != t.KwAsterisk {
					return nil, comp_err.CompilationErrorToken(
						ctx.Fctx,
						&opTk,
						"syntax error: invalid assignment target",
						"only pointer dereference (*) is assignable among unary expressions",
					)
				}
				left = &t.NodeExprAssign{
					Tk:    tk,
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
					Tk: tk,
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
	afterDot := false

	for {
		namePart, e := peek(ctx)
		if e != nil {
			return nil, e
		}

		if namePart.Type != t.TokName {
			description := "syntax error: expected name"
			if afterDot {
				description = "syntax error: expected name after dot"
			}
			return nil, comp_err.CompilationErrorToken(
				ctx.Fctx,
				&namePart,
				description,
				"",
			)
		}

		parts = append(parts, namePart.Repr)
		consume(ctx)
		afterDot = false

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
		afterDot = true
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

type parsedDeclName struct {
	NameNode        t.NodeName
	TypeParams      []string
	OwnerTypeParams []string
}

func parseDeclNameWithGenerics(ctx *ParseCtx) (*parsedDeclName, error) {
	firstTk, e := peek(ctx)
	if e != nil {
		return nil, e
	}
	if firstTk.Type != t.TokName {
		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&firstTk,
			fmt.Sprintf("syntax error: expected declaration name but got '%s'", firstTk.Repr),
			"",
		)
	}

	firstName := firstTk.Repr
	consume(ctx)

	firstParams := []string{}
	maybeOpen, e := peek(ctx)
	if e != nil {
		return nil, e
	}
	if maybeOpen.KeywType == t.KwBrackOp {
		firstParams, e = parseTypeParamList(ctx)
		if e != nil {
			return nil, e
		}
	}

	maybeDot, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	if maybeDot.KeywType != t.KwDot {
		return &parsedDeclName{
			NameNode:   &t.NodeNameSingle{Name: firstName},
			TypeParams: firstParams,
		}, nil
	}

	consume(ctx) // dot

	secondTk, e := peek(ctx)
	if e != nil {
		return nil, e
	}
	if secondTk.Type != t.TokName {
		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&secondTk,
			fmt.Sprintf("syntax error: expected member name after '.' but got '%s'", secondTk.Repr),
			"",
		)
	}

	secondName := secondTk.Repr
	consume(ctx)

	secondParams := []string{}
	maybeOpen2, e := peek(ctx)
	if e != nil {
		return nil, e
	}
	if maybeOpen2.KeywType == t.KwBrackOp {
		secondParams, e = parseTypeParamList(ctx)
		if e != nil {
			return nil, e
		}
	}

	return &parsedDeclName{
		NameNode: &t.NodeNameComposite{
			Parts: []string{firstName, secondName},
		},
		TypeParams:      secondParams,
		OwnerTypeParams: firstParams,
	}, nil
}

func parseNameTemplated(ctx *ParseCtx, tk t.Token, allowComposite bool) (t.NodeName, error) {
	parts := []string{}
	afterDot := false

	for {
		namePart, e := peek(ctx)
		if e != nil {
			return nil, e
		}

		if namePart.Type != t.TokName {
			description := "syntax error: expected name"
			if afterDot {
				description = "syntax error: expected name after dot"
			}
			return nil, comp_err.CompilationErrorToken(
				ctx.Fctx,
				&namePart,
				description,
				"",
			)
		}

		consume(ctx)
		afterDot = false

		maybeDot, e := peek(ctx)
		if e != nil {
			if errors.Is(e, errOutOfBounds) {
				break
			}
			return nil, e
		}

		if maybeDot.KeywType == t.KwBrackOp {
			consume(ctx) // [

			// TODO: implement parsing of generics
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
		afterDot = true
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

func parseTypeParamList(ctx *ParseCtx) ([]string, error) {
	params := []string{}

	open, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	if open.KeywType != t.KwBrackOp {
		return params, nil
	}

	consume(ctx)

	for {
		tk, e := peek(ctx)
		if e != nil {
			return nil, e
		}

		if tk.KeywType == t.KwBrackCl {
			consume(ctx)
			if len(params) == 0 {
				return nil, comp_err.CompilationErrorToken(
					ctx.Fctx,
					&tk,
					"syntax error: empty generic parameter list",
					"expected at least one type parameter name inside '[' and ']'",
				)
			}
			return params, nil
		}

		if tk.Type != t.TokName {
			return nil, comp_err.CompilationErrorToken(
				ctx.Fctx,
				&tk,
				fmt.Sprintf("syntax error: expected generic type parameter name but got '%s'", tk.Repr),
				"expected: `[T]`, `[T, U]`, ...",
			)
		}

		params = append(params, tk.Repr)
		consume(ctx)

		sep, e := peek(ctx)
		if e != nil {
			return nil, e
		}

		if sep.KeywType == t.KwComma {
			consume(ctx)
			continue
		}
		if sep.KeywType == t.KwBrackCl {
			continue
		}

		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&sep,
			fmt.Sprintf("syntax error: expected ',' or ']' in generic parameter list but got '%s'", sep.Repr),
			"",
		)
	}
}

func parseTypeArgList(ctx *ParseCtx) ([]*t.NodeType, error) {
	out := []*t.NodeType{}

	open, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	if open.KeywType != t.KwBrackOp {
		return out, nil
	}

	consume(ctx)

	for {
		tk, e := peek(ctx)
		if e != nil {
			return nil, e
		}

		if tk.KeywType == t.KwBrackCl {
			consume(ctx)
			if len(out) == 0 {
				return nil, comp_err.CompilationErrorToken(
					ctx.Fctx,
					&tk,
					"syntax error: empty generic argument list",
					"expected at least one type argument inside '[' and ']'",
				)
			}
			return out, nil
		}

		typeNd, e := parseType(ctx, tk, false)
		if e != nil {
			return nil, e
		}
		out = append(out, typeNd)

		sep, e := peek(ctx)
		if e != nil {
			return nil, e
		}

		if sep.KeywType == t.KwComma {
			consume(ctx)
			continue
		}
		if sep.KeywType == t.KwBrackCl {
			continue
		}

		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&sep,
			fmt.Sprintf("syntax error: expected ',' or ']' in generic argument list but got '%s'", sep.Repr),
			"",
		)
	}
}

func parseGenericClass(ctx *ParseCtx, nameNode t.NodeName, typeParams []string, ownerTypeParams []string) (t.NodeGenericClass, error) {
	n := t.NodeGenericClass{
		NameNode:        nameNode,
		TypeParams:      typeParams,
		OwnerTypeParams: ownerTypeParams,
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
					Owned:    outT.Owned,
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
				Owned:    outT.Owned,
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
				Owned:    outT.Owned,
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

	// owned marker
	isOwned := false
	if tk.KeywType == t.KwDollar {
		isOwned = true
		consume(ctx)
		tk, e = peek(ctx)
		if e != nil {
			return nil, e
		}
	}

	if tk.KeywType == t.KwParenOp {
		n, e := parseFuncType(ctx)
		if e != nil {
			return nil, e
		}
		n.Owned = isOwned
		return n, nil
	}

	named, e := parseName(ctx, tk, true)
	if e != nil {
		return nil, e
	}

	outT := &t.NodeType{
		Throws: isThrowing,
		Owned:  isOwned,
		KindNode: &t.NodeTypeNamed{
			NameNode: named,
		},
	}

	tk, e = peek(ctx)
	if e != nil {
		return nil, e
	}

	if tk.KeywType == t.KwBrackOp {
		maybeInner, e := peekNth(ctx, 1)
		if e != nil {
			return nil, e
		}

		// In type context, [] / [num] are slice/array suffixes.
		// Any other bracket content is parsed as generic type arguments.
		if maybeInner.KeywType != t.KwBrackCl && maybeInner.Type != t.TokLitNum {
			typeArgs, e := parseTypeArgList(ctx)
			if e != nil {
				return nil, e
			}

			outT.KindNode.(*t.NodeTypeNamed).GenericArgs = typeArgs
		}
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

func parseStmtContinue(ctx *ParseCtx) (t.NodeStatement, error) {
	consume(ctx) // consume ret kw
	return &t.NodeStmtContinue{}, nil
}

func parseStmtBreak(ctx *ParseCtx) (t.NodeStatement, error) {
	consume(ctx) // consume ret kw
	return &t.NodeStmtBreak{}, nil
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
	case t.KwDefer:
		n, e := parseDefer(ctx, tk)
		if e != nil {
			return nil, e
		}
		ctx.CurrentFunction.HasDefer = true
		ctx.CurrentFunction.DeferCnt += 2
		ctx.CurrentFunction.Deferred = append(ctx.CurrentFunction.Deferred, n)
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
	if ctx.PruneNext {
		return nil, nil
	}

	// check if struct name is valid (complex name not allowed)
	e := ensureSimpleName(ctx, tk, gncls.NameNode)
	if e != nil {
		return nil, e
	}

	simpleName := gncls.NameNode.(*t.NodeNameSingle)

	// create struct def in global node for easir type checking later
	structMap := &t.StructDef{
		Module:     ctx.Fctx.PackageName,
		Name:       simpleName.Name,
		TypeParams: gncls.TypeParams,
		Fields:     map[string]*t.NodeType{},
		Funcs:      map[string]*t.NodeFuncDef{},
		FieldNb:    map[string]int{},
		FieldOrder: []string{},
	}

	for i, arg := range gncls.ArgsNode.Args {
		structMap.Fields[arg.Name] = arg.TypeNode
		structMap.FieldNb[arg.Name] = i
		structMap.FieldOrder = append(structMap.FieldOrder, arg.Name)
	}

	ctx.GlobalNode.StructDefs[simpleName.Name] = structMap

	return &t.NodeStructDef{
		Class: gncls,
	}, nil
}

func parseFuncDef(ctx *ParseCtx, nameTk t.Token, after t.Token, gncls t.NodeGenericClass, alias string) (*t.NodeFuncDef, error) {
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
		Class:      gncls,
		IsExternal: alias != "",
	}

	if alias != "" {
		fnDef.NoAliasName = flattenName(gncls.NameNode)
		aliasedNameNode := &t.NodeNameSingle{Name: alias}
		fnDef.Class.NameNode = aliasedNameNode
		fnNameSimple = aliasedNameNode.Name
	}

	ctx.CurrentFunction = fnDef
	defer func() {
		ctx.CurrentFunction = nil
	}()

	typeNode, e := parseType(ctx, after, true)
	if e != nil {
		return nil, e
	}

	if alias == "" {
		bodyStart, e := peek(ctx)
		if e != nil {
			return nil, e
		}

		bodyNode, e := parseBody(ctx, bodyStart)
		if e != nil {
			return nil, e
		}
		fnDef.Body = bodyNode
	}

	fnDef.ReturnType = typeNode
	fnDef.AbsName = ctx.Fctx.PackageName + "." + flattenName(gncls.NameNode)

	// =========================================================================
	// pruning should result in NO SIDE EFFECT
	// section beyond this point is basically all side effects and nothing else
	if ctx.PruneNext {
		return nil, nil
	}
	// =========================================================================

	if isMemberFunc && alias == "" { // alias == "" since aliased functions cannot be also member funcs
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

		//fmt.Printf("added implicit this to: %s.%s()\n", ownerName, memberName)

		thisOwnerNamed := &t.NodeTypeNamed{
			NameNode: &t.NodeNameSingle{Name: ownerName},
		}
		if len(fnDef.Class.OwnerTypeParams) > 0 {
			typeArgs := make([]*t.NodeType, 0, len(fnDef.Class.OwnerTypeParams))
			for _, p := range fnDef.Class.OwnerTypeParams {
				typeArgs = append(typeArgs, &t.NodeType{
					KindNode: &t.NodeTypeNamed{
						NameNode: &t.NodeNameSingle{Name: p},
					},
				})
			}
			thisOwnerNamed.GenericArgs = typeArgs
		}

		fnDef.Class.ArgsNode.Args = slices.Insert(fnDef.Class.ArgsNode.Args, 0, t.NodeArg{
			Name: "this",
			TypeNode: &t.NodeType{
				KindNode: &t.NodeTypePointer{
					Kind: thisOwnerNamed,
				},
			},
		})

		ctx.GlobalNode.StructDefs[ownerName].Funcs[memberName] = fnDef

		/* DEPRECATED: Destructors will not be implemented in language.
		if memberName == "destructor" {

			if len(fnDef.Class.ArgsNode.Args) > 1 {
				return nil, comp_err.CompilationErrorToken(
					ctx.Fctx,
					&nameTk,
					fmt.Sprintf("syntax error: destructor function for '%s' cannot have any defined arguments", ownerName),
					fmt.Sprintf("signature of destructor should be: `%s.destructor() void`", ownerName),
				)
			}
			// TODO: enforce 0 args and non-throwing void type
			ctx.GlobalNode.StructDefs[ownerName].Destructor = fnDef
			// Destructor discovery is intentionally silent; callers can inspect the AST in debug mode.
		}*/
	}

	ctx.GlobalNode.FuncDefs[fnNameSimple] = fnDef
	return fnDef, nil
}

func parseExternalFunc(ctx *ParseCtx, tk t.Token) (t.NodeGlobalDecl, error) {
	consume(ctx) // consume "extern"

	nAlias, e := parseName(ctx, tk, false)
	if e != nil {
		return nil, e
	}

	next, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	if next.Type != t.TokName {
		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&next,
			fmt.Sprintf("syntax error: expected external function name after alias but got '%s'", next.Repr),
			"expected: `extern <alias> <actual name> (<args>) <return type>`",
		)
	}

	n, e := parseName(ctx, next, false)
	if e != nil {
		return nil, e
	}

	next, e = peek(ctx)
	if e != nil {
		return nil, e
	}

	if next.Type != t.TokKeyword {
		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&next,
			fmt.Sprintf("syntax error: unexpected '%s' after name in extern function declaration", next.Repr),
			"expected: `extern <name> (`",
		)
	}

	gncls, e := parseGenericClass(ctx, n, nil, nil)
	if e != nil {
		return nil, e
	}

	after, e := peek(ctx)
	if e != nil && !errors.Is(e, errOutOfBounds) {
		return nil, e
	}

	if errors.Is(e, errOutOfBounds) || after.KeywType == t.KwNewline {
		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&next,
			fmt.Sprintf("syntax error: unexpected '%s' after argument list in external function declaration", next.Repr),
			"expected: `extern <name> (<args>) <return type>",
		)
	}

	alias := flattenName(nAlias)
	return parseFuncDef(ctx, tk, after, gncls, alias)
}

func parseGlobalDeclFromName(ctx *ParseCtx, tk t.Token) (t.NodeGlobalDecl, error) {
	modifiers := slices.Clone(ctx.NextModifiers)
	declName, e := parseDeclNameWithGenerics(ctx)
	if e != nil {
		return nil, e
	}

	next, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	switch next.KeywType {
	case t.KwParenOp:
		ctx.NextModifiers = []ModifierType{}

		gncls, e := parseGenericClass(ctx, declName.NameNode, declName.TypeParams, declName.OwnerTypeParams)
		if e != nil {
			return nil, e
		}

		after, e := peek(ctx)
		if e != nil && !errors.Is(e, errOutOfBounds) {
			return nil, e
		}

		if errors.Is(e, errOutOfBounds) || after.KeywType == t.KwNewline {
			if slices.Contains(modifiers, MdDestructor) {
				return nil, comp_err.CompilationErrorToken(ctx.Fctx, &tk, "syntax error: destructor modifier requires a member function", "expected: `destructor Type.method() void:`")
			}
			return parseStructDef(ctx, tk, gncls)
		}

		fn, e := parseFuncDef(ctx, tk, after, gncls, "")
		if e != nil {
			return nil, e
		}
		if slices.Contains(modifiers, MdDestructor) {
			name, ok := fn.Class.NameNode.(*t.NodeNameComposite)
			if !ok || len(name.Parts) != 2 {
				return nil, comp_err.CompilationErrorToken(ctx.Fctx, &tk, "syntax error: destructor must be a struct member function", "expected: `destructor Type.method() void:`")
			}
			fn.IsDestructor = true
			owner := ctx.GlobalNode.StructDefs[name.Parts[0]]
			owner.Destructors = append(owner.Destructors, fn)
			if owner.Destructor == nil {
				owner.Destructor = fn
			}
		}
		return fn, nil
	default:
		if len(declName.TypeParams) > 0 || len(declName.OwnerTypeParams) > 0 {
			return nil, comp_err.CompilationErrorToken(
				ctx.Fctx,
				&next,
				"syntax error: generic parameters are only valid on struct/function declarations",
				"",
			)
		}

		tNode, e := parseType(ctx, next, false)
		if e == nil {
			return &t.NodeExprVarDef{
				Name:       declName.NameNode,
				AbsName:    ctx.Fctx.PackageName + "." + flattenName(declName.NameNode),
				Type:       tNode,
				IsGlobal:   true,
				IsSsa:      false,
				IsReturned: false,
			}, nil
		}

		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&next,
			fmt.Sprintf("syntax error: unexpected '%s' after name in global declaration", next.Repr),
			"expected in global scope: `<name> <type>`, `<name> (",
		)
	}
}

func parseConstDecl(ctx *ParseCtx, constTk t.Token) (t.NodeGlobalDecl, error) {
	consume(ctx)
	nameTk, e := peek(ctx)
	if e != nil || nameTk.Type != t.TokName {
		return nil, comp_err.CompilationErrorToken(ctx.Fctx, &constTk, "expected a name after 'const'", "expected: `const name Type = expression` or `const name := expression`")
	}
	consume(ctx)

	next, e := peek(ctx)
	if e != nil {
		return nil, e
	}
	var typeNode *t.NodeType
	if next.KeywType == t.KwInfer {
		consume(ctx)
	} else {
		typeNode, e = parseType(ctx, next, false)
		if e != nil {
			return nil, e
		}
		eq, e := peek(ctx)
		if e != nil || eq.KeywType != t.KwEqual {
			return nil, comp_err.CompilationErrorToken(ctx.Fctx, &nameTk, "constant declaration is missing '='", "expected: `const name Type = expression`")
		}
		consume(ctx)
	}

	first, e := peek(ctx)
	if e != nil {
		return nil, e
	}
	initializer, e := parseExpression(ctx, first, 0)
	if e != nil {
		return nil, e
	}
	vd := &t.NodeExprVarDef{
		Name:     &t.NodeNameSingle{Name: nameTk.Repr},
		Type:     typeNode,
		AbsName:  ctx.Fctx.PackageName + "." + nameTk.Repr,
		IsGlobal: true,
	}
	return &t.NodeConstDef{Tk: constTk, VarDef: vd, Initializer: initializer}, nil
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

func parseCompilerDirective(ctx *ParseCtx, tk t.Token) error {
	consume(ctx) // @

	tk, e := peek(ctx)
	if e != nil {
		return e
	}

	if tk.Type != t.TokName {
		return comp_err.CompilationErrorToken(
			ctx.Fctx,
			&tk,
			fmt.Sprintf("syntax error: expected directive name after '@', but got '%s'", tk.Repr),
			"expected: `@<name>`, ex: `@platform(\"windows\")`",
		)
	}
	consume(ctx)

	dirName := tk.Repr
	dirArgs := []t.Token{}

	next, e := peek(ctx)
	if e != nil {
		return e
	}

	if next.KeywType == t.KwParenOp {
		consume(ctx)

		next, e = peek(ctx)
		if e != nil {
			return e
		}

		for next.KeywType != t.KwParenCl {
			switch next.Type {
			case t.TokLitBool, t.TokLitNum, t.TokLitStr:
				dirArgs = append(dirArgs, next)
				consume(ctx)
			case t.TokKeyword:
				if next.KeywType == t.KwComma {
					consume(ctx)
					goto switch_end
				}
				consume(ctx)
				fallthrough
			default:
				return comp_err.CompilationErrorToken(
					ctx.Fctx,
					&next,
					"syntax error: argument in compiler directive needs to be a constant literal",
					"expected: `@<name>(<literal>, ...)`, ex: `@platform(\"windows\")`",
				)
			}
		switch_end:

			next, e = peek(ctx)
			if e != nil {
				return e
			}
		}
		consume(ctx)
	}

	switch dirName {
	case "platform":
		if len(dirArgs) < 1 {
			return comp_err.CompilationErrorToken(
				ctx.Fctx,
				&tk,
				"syntax error: directive 'platform' takes in 1 or many arguments",
				"expected: `@platform(\"<platform/os>, ...\")`",
			)
		}

		found := false

		for _, tok := range dirArgs {
			if runtime.GOOS == tok.Repr {
				found = true

				if found {
					//fmt.Printf("found platform: %s\n", runtime.GOOS)
					break
				}
			}
		}

		ctx.PruneNext = !found
		return nil
	default:
		return comp_err.CompilationErrorToken(
			ctx.Fctx,
			&next,
			"syntax error: invalid compiler directive name",
			"expected: `@platform(...)`",
		)
	}
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
		if ctx.PruneNext {
			ctx.PruneNext = false
			return nil, nil
		}
		return n, nil

	case t.TokKeyword:
		switch tk.KeywType {
		case t.KwNewline:
			consume(ctx)
			return nil, nil
		case t.KwPublic:
			e = parseApplyModifier(ctx, tk, MdPublic)
		case t.KwDestructor:
			e = parseApplyModifier(ctx, tk, MdDestructor)
		case t.KwConst:
			n, e = parseConstDecl(ctx, tk)
			return n, e
		case t.KwAt:
			e = parseCompilerDirective(ctx, tk)
		case t.KwModule:
			e = parseModuleDecl(ctx, tk)
		case t.KwUse:
			e = parseUseDecl(ctx, tk, ctx.PruneNext)
			if ctx.PruneNext {
				ctx.PruneNext = false
				return nil, nil
			}
		case t.KwLink:
			e = parseLinkDecl(ctx, tk, ctx.PruneNext)
			if ctx.PruneNext {
				ctx.PruneNext = false
				return nil, nil
			}
		case t.KwLlvm:
			n, e = parseLlvm(ctx, tk)
			if e != nil {
				return nil, e
			}
			if ctx.PruneNext {
				ctx.PruneNext = false
				return nil, nil
			}
			return n, nil
		case t.KwExtern:
			n, e = parseExternalFunc(ctx, tk)
			if ctx.PruneNext {
				ctx.PruneNext = false
				return nil, nil
			}
			return n, nil

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
