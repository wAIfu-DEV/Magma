package main

import (
	"Magma/src/checker"
	llvmir "Magma/src/llvm_ir"
	"Magma/src/makeabs"
	"Magma/src/pipeline"
	"Magma/src/program"
	"Magma/src/types"
	"fmt"
	"os"
	"sync"
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

	shared := &types.SharedState{
		Cwd:            cwd,
		ImportedFiles:  map[string]<-chan error{},
		ImportedFilesM: sync.Mutex{},
		Files:          map[string]*types.FileCtx{},
		FilesM:         sync.Mutex{},
		PipeChans:      []<-chan error{},
		PipeChansM:     sync.Mutex{},

		// needed because Go sucks and can't figure out cyclical imports
		PipelineFunc: pipeline.DoAsync,
		WaitGroup:    sync.WaitGroup{},
	}

	// actual meat of the program
	if e = pipeline.Do(shared, absPath, "", absPath, nil); e != nil {
		fmt.Printf("fatal error in file '%s': %s\n", absPath, e.Error())
	}

	// wait for other compilation unit goroutines
	if e = program.JoinCompilationUnits(shared, e); e != nil {
		os.Exit(1)
	}

	if e = checker.CheckLinks(shared); e != nil {
		return e
	}

	if e = checker.TypeChecker(shared); e != nil {
		return e
	}

	// write LLVM intermediate repr
	irStr, e := llvmir.IrWrite(shared)
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
