package main

import (
	"Magma/src/checker"
	llvmir "Magma/src/llvm_ir"
	"Magma/src/makeabs"
	"Magma/src/pipeline"
	"Magma/src/program"
	"Magma/src/shared"
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

	cwd, e := os.Getwd()
	if e != nil {
		return e
	}

	fmt.Printf("input file: %s\n", filePathArg)
	fmt.Printf("cwd: %s\n", cwd)

	// second arg of MakeAbs is expected to be file path
	absPath, e := makeabs.MakeAbs(filePathArg, cwd+"/a.b")
	if e != nil {
		return e
	}

	s := shared.MakeShared(cwd)

	// actual meat of the program, multithreaded per file
	// 1. lexing/tokenization
	// 2. parsing to AST
	// 3. scope info gathering
	if e = pipeline.DoMain(s, absPath); e != nil {
		fmt.Printf("fatal error in file '%s': %s\n", absPath, e.Error())
	}

	// wait for other compilation unit goroutines
	if e = program.JoinCompilationUnits(s, e); e != nil {
		os.Exit(1)
	}

	// check/resolve name->node
	if e = checker.CheckLinks(s); e != nil {
		return e
	}

	// check/resolve types
	if e = checker.TypeChecker(s); e != nil {
		return e
	}

	// write LLVM intermediate repr
	irStr, e := llvmir.IrWrite(s)
	if e != nil {
		return e
	}

	fmt.Println("Llvm IR:")
	fmt.Println(irStr)

	return os.WriteFile("out.ll", []byte(irStr), 0666)
}

func main() {
	err := wrappedMain()
	if err != nil {
		fmt.Printf("uncaught fatal error: %s\n", err.Error())
		os.Exit(1)
	}
}
