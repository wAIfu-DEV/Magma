package parser

import (
	"Magma/src/comp_err"
	t "Magma/src/types"
	"fmt"
	"strings"
)

func parseLlvm(ctx *ParseCtx, tk t.Token) (*t.NodeLlvm, error) {
	consume(ctx) // consume llvm kw

	next, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	if next.Type == t.TokLitStr {
		consume(ctx)
		return &t.NodeLlvm{Tk: tk, Text: next.Repr}, nil
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
			if string(ctx.Shared.Target.OS) == tok.Repr {
				found = true

				if found {
					//fmt.Printf("found platform: %s\n", ctx.Shared.Target.OS)
					break
				}
			}
		}

		ctx.PruneNext = !found
		return nil
	case "export_name":
		if len(dirArgs) < 1 || len(dirArgs) > 2 || dirArgs[0].Type != t.TokLitStr || (len(dirArgs) == 2 && dirArgs[1].Type != t.TokLitStr) {
			return comp_err.CompilationErrorToken(ctx.Fctx, &tk, "syntax error: directive 'export_name' takes a symbol name and optional ABI string", "expected: `@export_name(\"<symbol>\")` or `@export_name(\"<symbol>\", \"C\")`")
		}
		if ctx.NextExportName != "" {
			return comp_err.CompilationErrorToken(ctx.Fctx, &tk, "syntax error: duplicate 'export_name' directive", "only one export name can be applied to a function")
		}
		abi := "C"
		if len(dirArgs) == 2 {
			abi = dirArgs[1].Repr
		}
		if !strings.EqualFold(abi, "C") {
			return comp_err.CompilationErrorToken(ctx.Fctx, &dirArgs[1], fmt.Sprintf("syntax error: unsupported export ABI '%s'", abi), "currently supported ABI: \"C\"")
		}
		if !validExportName(dirArgs[0].Repr) {
			return comp_err.CompilationErrorToken(ctx.Fctx, &dirArgs[0], "syntax error: invalid external symbol name", "use a C identifier containing letters, digits, and underscores")
		}
		ctx.NextExportName = dirArgs[0].Repr
		ctx.NextExportABI = "C"
		return nil
	case "no_retain":
		if len(dirArgs) != 0 {
			return comp_err.CompilationErrorToken(ctx.Fctx, &tk, "syntax error: directive 'no_retain' takes no arguments", "expected: `@no_retain`")
		}
		if ctx.NextNoRetain {
			return comp_err.CompilationErrorToken(ctx.Fctx, &tk, "syntax error: duplicate 'no_retain' directive", "apply it once to a function definition")
		}
		ctx.NextNoRetain = true
		return nil
	default:
		return comp_err.CompilationErrorToken(
			ctx.Fctx,
			&next,
			"syntax error: invalid compiler directive name",
			"expected: `@platform(...)`, `@export_name(...)`, or `@no_retain`",
		)
	}
}
