package comp_err

import (
	"Magma/src/types"
	"errors"
	"fmt"
)

type CompilationError struct {
	Ctx        *types.FileCtx
	Token      types.Token
	ShortDesc  string
	Additional string
}

func (e *CompilationError) Error() string {
	return e.ShortDesc
}

func printLine(ctx *types.FileCtx, line int) {
	lnStart := ctx.LineIdx[line] + 1
	lnEnd := ctx.LineIdx[line+1] + 1

	lineStr := ctx.Content[lnStart:lnEnd]
	fmt.Printf("%d| %s", line, lineStr)
}

func printLineTok(ctx *types.FileCtx, line int) {
	lnStart := ctx.LineIdx[line]
	lnEnd := ctx.LineIdx[line+1]

	lineStr := ctx.Content[lnStart:lnEnd]
	fmt.Printf("%d| %s", line, lineStr)
}

func CompilationErrorToken(ctx *types.FileCtx, tk *types.Token, shortDesc string, additional string) error {
	return &CompilationError{
		Ctx:        ctx,
		Token:      *tk,
		ShortDesc:  shortDesc,
		Additional: additional,
	}
}

func Print(err error) bool {
	var compilationErr *CompilationError
	if !errors.As(err, &compilationErr) {
		return false
	}

	ctx := compilationErr.Ctx
	tk := compilationErr.Token
	fmt.Printf("%s:l%d:c%d %s\n", ctx.FilePath, tk.Pos.Line, tk.Pos.Col, compilationErr.ShortDesc)

	pos := tk.Pos

	// previous line
	if pos.Line > 1 {
		printLine(ctx, int(pos.Line)-2)
	}

	if true {
		printLine(ctx, int(pos.Line)-1)
		/*
			// current line
			currLine := pos.Line - 1
			lnStart := ctx.LineIdx[currLine] + 1
			lnEnd := ctx.LineIdx[currLine+1] - 1
			line := ctx.Content[lnStart:lnEnd]

			left := line[:pos.Col-1]
			middle := line[pos.Col-1 : int(pos.Col-1)+len(tk.Repr)]
			right := line[int(pos.Col-1)+len(tk.Repr):]

			fmt.Printf("%d| %s\x1b[31m\x1b[4:3m%s\x1b[0m%s\n", pos.Line, left, middle, right)*/
	}

	// next line
	if int(pos.Line)+1 < len(ctx.LineIdx)-1 {
		printLine(ctx, int(pos.Line))
	}

	fmt.Printf("%s\n\n", compilationErr.Additional)
	return true
}
