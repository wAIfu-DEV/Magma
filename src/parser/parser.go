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
	if (ctx.TokIdx + n) >= len(ctx.Toks) {
		return t.Token{}, errOutOfBounds
	}
	return ctx.Toks[ctx.TokIdx+n], nil
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
	if e != nil {
		return comp_err.CompilationErrorToken(
			ctx.Fctx,
			&tk,
			"syntax error: expected module name after 'mod'",
			"expected: `mod <name>`",
		)
	}

	newln, e := peekNth(ctx, 2)
	if e != nil && !errors.Is(e, errOutOfBounds) {
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

func parseName(ctx *ParseCtx, tk t.Token) (t.NodeName, error) {
	i := 0
	parts := []string{}

	for {
		namePart, e := peekNth(ctx, i)
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

		maybeDot, e := peekNth(ctx, i+1)
		if errors.Is(e, errOutOfBounds) {
			break
		} else if e != nil {
			return nil, e
		}

		if maybeDot.Type != t.TokKeyword || maybeDot.KeywType != t.KwDot {
			break
		}
		consume(ctx)
	}

	partsLen := len(parts)

	if partsLen == 0 {
		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&tk,
			"syntax error: name parsing failure, unexpected state",
			"",
		)
	} else if partsLen == 1 {
		return &t.NodeNameSingle{
			Name: parts[0],
		}, nil
	} else {
		return &t.NodeNameComposite{
			Parts: parts,
		}, nil
	}
}

func parseArgsList(ctx *ParseCtx) (t.NodeArgList, error) {
	openPar, e := peek(ctx)
	if e != nil {
		return t.NodeArgList{}, e
	}
	if openPar.KeywType != t.KwParenOp {
		return t.NodeArgList{}, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&openPar,
			"syntax error: expected '(' but got '%s'",
			"",
		)
	}
	consume(ctx)

	closePar, e := peek(ctx)
	if e != nil {
		return t.NodeArgList{}, e
	}
	if closePar.KeywType != t.KwParenCl {
		return t.NodeArgList{}, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&closePar,
			"syntax error: expected ')' but got '%s'",
			"",
		)
	}
	consume(ctx)

	return t.NodeArgList{
		Args: []t.NodeArg{},
	}, nil
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

func parseGlobalDecl(ctx *ParseCtx, tk t.Token) (t.NodeGlobalDecl, error) {
	var n t.NodeGlobalDecl = nil
	var e error = nil

outer:
	switch tk.Type {
	case t.TokName:
		n, e := parseName(ctx, tk)
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
			fnDef := &t.NodeFuncDef{
				Class: gncls,
				ReturnType: t.NodeType{
					Throws:   true,
					KindNode: &t.NodeTypeNamed{NameNode: &t.NodeNameSingle{Name: "void"}},
				},
				Body: t.NodeBody{Statements: []t.NodeStatement{}},
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
			return n, nil
		}

		glDecl, e := parseGlobalDecl(ctx, tk)
		if e != nil {
			return t.NodeGlobal{}, e
		}

		// this is sketch af
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
		return glNd, e
	}
	return glNd, nil
}
