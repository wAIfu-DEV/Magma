package parser

import (
	"Magma/src/comp_err"
	"Magma/src/makeabs"
	t "Magma/src/types"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

var errOutOfBounds error = errors.New("oob")

type ModifierType string

const (
	MdPublic     ModifierType = "pub"
	MdDestructor ModifierType = "destr"
	MdNoCtx      ModifierType = "noctx"
)

type ParseCtx struct {
	Shared          *t.SharedState
	GlobalNode      *t.NodeGlobal
	Fctx            *t.FileCtx
	Toks            []t.Token
	TokIdx          int
	NextModifiers   []ModifierType
	CurrentFunction *t.NodeFuncDef
	NextExportName  string
	NextExportABI   string
	NextNoRetain    bool

	PruneNext  bool
	ModuleSeen bool
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

func parseModuleDecl(ctx *ParseCtx, tk t.Token) error {
	if ctx.ModuleSeen {
		return comp_err.CompilationErrorToken(ctx.Fctx, &tk, "syntax error: duplicate module declaration", "only one 'mod' declaration is allowed per file")
	}
	ctx.ModuleSeen = true
	// The module name itself is validated and recorded by the pipeline prelude.
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
	isPublic := false
	for _, modifier := range ctx.NextModifiers {
		if modifier != MdPublic {
			return comp_err.CompilationErrorToken(ctx.Fctx, &tk, fmt.Sprintf("syntax error: modifier '%s' cannot be applied to 'use'", modifier), "only 'pub' may modify a use declaration")
		}
		isPublic = true
	}
	ctx.NextModifiers = nil

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

	absPath, err := makeabs.ResolveImport(path.Repr, ctx.Fctx.FilePath, ctx.Shared.StdRoot)
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
	if isPublic {
		ctx.GlobalNode.PublicImportAlias[alias.Repr] = true
	}

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
	// A leading colon is the Clang/GNU ld exact-library-name form. Preserve it
	// so it reaches emission as `-l:filename` rather than a relative file path.
	if !strings.HasPrefix(requirement, ":") && (filepath.IsAbs(requirement) || strings.ContainsAny(requirement, `/\`) || filepath.Ext(requirement) != "") {
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

func parseBundleDecl(ctx *ParseCtx, tk t.Token, prune bool) error {
	if err := ensureNoModifiers(ctx, tk); err != nil {
		return err
	}
	file, err := peekNth(ctx, 1)
	if err != nil || file.Type != t.TokLitStr || file.Repr == "" {
		return comp_err.CompilationErrorToken(ctx.Fctx, &tk,
			"syntax error: expected file path after 'bundle'",
			"expected: `bundle \"<file>\"`")
	}
	newline, err := peekNth(ctx, 2)
	if err == nil && newline.KeywType != t.KwNewline {
		return comp_err.CompilationErrorToken(ctx.Fctx, &newline,
			fmt.Sprintf("syntax error: expected end of line after bundle path but got '%s'", newline.Repr),
			"expected: `bundle \"<file>\"(\\n)`")
	}
	consume(ctx)
	consume(ctx)
	consume(ctx)
	if prune {
		return nil
	}
	path := file.Repr
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(ctx.Fctx.FilePath), path)
	}
	path = filepath.Clean(path)
	if !slices.Contains(ctx.Fctx.Bundles, path) {
		ctx.Fctx.Bundles = append(ctx.Fctx.Bundles, path)
	}
	return nil
}

// parseArrayExpr parses `array T[length]`. Generic element types retain their
// own brackets: `array Pair[A, B][length]`.
