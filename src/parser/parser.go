package parser

import (
	"Magma/src/comp_err"
	t "Magma/src/types"
	"errors"
)

func parseGlobal(ctx *ParseCtx) (*t.NodeGlobal, error) {
	n := &t.NodeGlobal{
		StructDefs:           map[string]*t.StructDef{},
		TypeAliases:          map[string]*t.TypeAlias{},
		FuncDefs:             map[string]*t.NodeFuncDef{},
		PrimitiveMethods:     map[string]map[string]*t.NodeFuncDef{},
		PrimitiveDestructors: map[string][]*t.NodeFuncDef{},

		Declarations:      []t.NodeGlobalDecl{},
		ImportAlias:       map[string]string{},
		PublicImportAlias: map[string]bool{},
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
