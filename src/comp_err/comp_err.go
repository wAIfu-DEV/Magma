package comp_err

import (
	"Magma/src/types"
	"errors"
	"fmt"
)

func CompilationErrorToken(ctx *types.FileCtx, tk *types.Token, shortDesc string, additional string) error {
	fmt.Printf("%s:l%d:c%d %s\n", ctx.FilePath, tk.Pos.Line, tk.Pos.Col, shortDesc)

	pos := tk.Pos

	// previous line
	if pos.Line > 1 {
		prevLine := pos.Line - 2
		lnStart := ctx.LineIdx[prevLine] + 1
		lnEnd := ctx.LineIdx[prevLine+1] - 1

		line := ctx.Content[lnStart:lnEnd]
		fmt.Printf("%d| %s\n", pos.Line-1, line)
	}

	if true {
		// current line
		currLine := pos.Line - 1
		lnStart := ctx.LineIdx[currLine] + 1
		lnEnd := ctx.LineIdx[currLine+1] - 1
		line := ctx.Content[lnStart:lnEnd]

		left := line[:pos.Col-1]
		middle := line[pos.Col-1 : int(pos.Col-1)+len(tk.Repr)]
		right := line[int(pos.Col-1)+len(tk.Repr):]

		fmt.Printf("%d| %s\x1b[31m\x1b[4:3m%s\x1b[0m%s\n", pos.Line, left, middle, right)
	}

	// next line
	if int(pos.Line)+1 < len(ctx.LineIdx)-1 {
		nextLine := pos.Line
		lnStart := ctx.LineIdx[nextLine] + 1
		lnEnd := ctx.LineIdx[nextLine+1] - 1

		line := ctx.Content[lnStart:lnEnd]
		fmt.Printf("%d| %s\n", pos.Line+1, line)
	}

	fmt.Printf("%s\n\n", additional)
	return errors.New(shortDesc)
}
