package parser

import (
	"Magma/src/comp_err"
	t "Magma/src/types"
	"errors"
	"fmt"
	"slices"
	"strings"
)

var errOutOfBounds error = errors.New("oob")

type ModifierType string

const (
	MdPublic ModifierType = "pub"
)

type ParseCtx struct {
	Fctx          *t.FileCtx
	Toks          []t.Token
	TokIdx        int
	NextModifiers []ModifierType
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

func parseModuleDecl(ctx *ParseCtx, tk t.Token) error {
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
	return nil
}

func parseUseDecl(ctx *ParseCtx, tk t.Token) error {
	e := ensureNoModifiers(ctx, tk)
	if e != nil {
		return e
	}

	name, e := peekNth(ctx, 1)
	if e != nil {
		return comp_err.CompilationErrorToken(
			ctx.Fctx,
			&tk,
			"syntax error: expected module name after 'use'",
			"expected: `use <name>`",
		)
	}

	newln, e := peekNth(ctx, 2)
	if e != nil && !errors.Is(e, errOutOfBounds) {
		return comp_err.CompilationErrorToken(
			ctx.Fctx,
			&tk,
			fmt.Sprintf("syntax error: expected end of line after module name but got '%s'", newln.Repr),
			"expected: `use <name>(\\n)`",
		)
	}

	if slices.Contains(ctx.Fctx.Imports, name.Repr) {
		return comp_err.CompilationErrorToken(
			ctx.Fctx,
			&tk,
			fmt.Sprintf("syntax error: already using module '%s'", name.Repr),
			"only a single 'use' declaration is allowed per module within the same file",
		)
	}

	ctx.Fctx.Imports = append(ctx.Fctx.Imports, name.Repr)
	consume(ctx)
	consume(ctx)
	consume(ctx)
	return nil
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

		if tk.KeywType != t.KwParenCl && tk.KeywType != t.KwComma {
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

func parseType(ctx *ParseCtx, tk t.Token, allowThrow bool) (t.NodeType, error) {
	isThrowing := false
	if tk.KeywType == t.KwExclam {
		if !allowThrow {
			return t.NodeType{}, comp_err.CompilationErrorToken(
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
		return t.NodeType{}, e
	}

	if tk.Type == t.TokName {
		n, e := parseName(ctx, tk, true)
		if e != nil {
			return t.NodeType{}, e
		}

		return t.NodeType{
			Throws: isThrowing,
			KindNode: &t.NodeTypeNamed{
				NameNode: n,
			},
		}, nil
	}

	// TODO: implement complex types
	return t.NodeType{}, comp_err.CompilationErrorToken(
		ctx.Fctx,
		&tk,
		fmt.Sprintf("syntax error: unexpected '%s' when expected name of type", tk.Repr),
		"",
	)
}

func parseStatement(ctx *ParseCtx, tk t.Token) (t.NodeStatement, error) {
	// TODO: expand

	if tk.KeywType == t.KwReturn {
		// TODO: parse following expr
		consume(ctx)
		return &t.NodeStmtRet{Expression: &t.NodeExprVoid{}}, nil
	}

	return nil, comp_err.CompilationErrorToken(
		ctx.Fctx,
		&tk,
		fmt.Sprintf("syntax error: '%s' is not a valid start of statement", tk.Repr),
		"valid statements include: `name: type = expr`, `name()`, `ret expr`, etc.",
	)
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
			&tk,
			fmt.Sprintf("syntax error: unexpected '%s' after name in global declaration", next.Repr),
			"expected in global scope: `<name> :`, `<name> (",
		)
	}

	switch next.KeywType {
	case t.KwParenOp:
		gncls, e := parseGenericClass(ctx, n)
		if e != nil {
			return nil, e
		}

		after, e := peek(ctx)
		if e != nil && !errors.Is(e, errOutOfBounds) {
			return nil, e
		}

		if errors.Is(e, errOutOfBounds) || after.KeywType == t.KwNewline {
			return &t.NodeStructDef{
				Class: gncls,
			}, nil
		}

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

		fnDef := &t.NodeFuncDef{
			Class:      gncls,
			ReturnType: typeNode,
			Body:       bodyNode,
		}
		return fnDef, nil
	default:
		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&tk,
			fmt.Sprintf("syntax error: unexpected '%s' after name in global declaration", next.Repr),
			"expected in global scope: `<name> :`, `<name> (",
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

func parseGlobal(ctx *ParseCtx) (t.NodeGlobal, error) {
	n := t.NodeGlobal{}

	for {
		tk, e := peek(ctx)
		if e != nil {
			if errors.Is(e, errOutOfBounds) {
				return n, nil
			}
			return t.NodeGlobal{}, e
		}

		if tk.KeywType == t.KwNewline {
			consume(ctx)
			continue
		}

		glDecl, e := parseGlobalDecl(ctx, tk)
		if e != nil {
			return t.NodeGlobal{}, e
		}

		// this is sketch af
		// we do this since some valid declarations won't return a node
		if glDecl != nil {
			n.Declarations = append(n.Declarations, glDecl)
		}
	}
}

func Parse(fCtx *t.FileCtx) (t.NodeGlobal, error) {
	ctx := &ParseCtx{
		Fctx: fCtx,
		Toks: fCtx.Tokens,
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
