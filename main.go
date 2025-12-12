package main

import (
	lineidx "Magma/src/line_idx"
	llvmir "Magma/src/llvm_ir"
	"Magma/src/parser"
	"Magma/src/tokenizer"
	"Magma/src/types"
	"fmt"
	"os"
)

func wrappedMain() error {
	args := os.Args[1:]

	if len(args) > 1 {
		return fmt.Errorf("too many arguments")
	} else if len(args) == 0 {
		return fmt.Errorf("not enough arguments")
	}

	filePathArg := args[0]

	fileBytes, err := os.ReadFile(filePathArg)
	if err != nil {
		return fmt.Errorf("failed to open file")
	}

	fCtx := &types.FileCtx{
		FilePath: filePathArg,
		Content:  fileBytes,
		LineIdx:  lineidx.GetLineIdx(fileBytes),
	}

	fCtx.Tokens, err = tokenizer.Tokenize(fCtx, fileBytes)

	if err != nil {
		return err
	}
	tokenizer.PrintTokens(fCtx.Tokens)

	fCtx.GlNode, err = parser.Parse(fCtx)
	if err != nil {
		return err
	}

	fCtx.GlNode.Print(0)

	irStr, err := llvmir.IrWrite(fCtx, &fCtx.GlNode)
	if err != nil {
		return err
	}

	fmt.Println("Llvm IR:")
	fmt.Println(irStr)
	return nil
}

func main() {
	err := wrappedMain()
	if err != nil {
		fmt.Printf("fatal error: %s\n", err.Error())
		os.Exit(1)
	}
}
