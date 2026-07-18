package comp_err

import (
	"Magma/src/types"
	"bytes"
	"errors"
	"fmt"
	"strings"
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
	lines := bytes.Split(ctx.Content, []byte{'\n'})
	if line < 1 || line > len(lines) {
		return
	}
	lineText := strings.TrimSuffix(string(lines[line-1]), "\r")
	fmt.Printf("%d| %s\n", line, lineText)
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
	description := strings.ReplaceAll(compilationErr.ShortDesc, "'\r\n'", "newline")
	description = strings.ReplaceAll(description, "'\n'", "newline")
	description = strings.ReplaceAll(description, "'\r'", "newline")
	fmt.Printf("%s:l%d:c%d %s\n", ctx.FilePath, tk.Pos.Line, tk.Pos.Col, description)

	pos := tk.Pos

	// previous line
	if pos.Line > 1 {
		printLine(ctx, int(pos.Line)-1)
	}

	if true {
		printLine(ctx, int(pos.Line))
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
	if int(pos.Line) < len(bytes.Split(ctx.Content, []byte{'\n'})) {
		printLine(ctx, int(pos.Line)+1)
	}

	fmt.Printf("%s\n\n", compilationErr.Additional)
	return true
}
