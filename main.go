package main

import (
	"Magma/src/tokenizer"
	"Magma/src/types"
	"fmt"
	"os"
)

func wrappedMain() error {
	args := os.Args[1:]

	if len(args) > 1 {
		return fmt.Errorf("Too many arguments")
	} else if len(args) == 0 {
		return fmt.Errorf("Not enough arguments")
	}

	filePathArg := args[0]

	fileBytes, err := os.ReadFile(filePathArg)
	if err != nil {
		return fmt.Errorf("Failed to open file")
	}

	fCtx := &types.FileCtx{
		FilePath: filePathArg,
		Content: fileBytes,
		LineIdx: make([]int, 0, 8),
	}
	fCtx.LineIdx = append(fCtx.LineIdx, -1)

	for i, b := range fileBytes {
		if b == '\n' {
			fCtx.LineIdx = append(fCtx.LineIdx, i)
		}
	}
	fCtx.LineIdx = append(fCtx.LineIdx, len(fileBytes))

	tokens, err := tokenizer.Tokenize(fCtx, fileBytes)

	if err != nil {
		return err
	}
	tokenizer.PrintTokens(tokens)
	return nil
}

func main() {
	err := wrappedMain()
	if err != nil {
		fmt.Printf("fatal error: %s\n", err.Error())
		os.Exit(1)
	}
}
