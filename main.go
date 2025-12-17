package main

import (
	llvmir "Magma/src/llvm_ir"
	"Magma/src/makeabs"
	"Magma/src/pipeline"
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
	absPath, e := makeabs.MakeAbs(filePathArg, os.Args[0])
	if e != nil {
		return e
	}

	shared := &types.SharedState{
		ImportM:       sync.Mutex{},
		ImportedFiles: map[string]<-chan error{},

		Files:  map[string]*types.FileCtx{},
		FilesM: sync.Mutex{},

		PipeChans:  []<-chan error{},
		PipeChansM: sync.Mutex{},

		PipelineFunc: pipeline.PipelineAsync,
		WaitGroup:    sync.WaitGroup{},
	}

	e = pipeline.Pipeline(shared, absPath, "", absPath)
	if e != nil {
		return e
	}

	shared.WaitGroup.Wait() // wait for other compilation units

	for k, v := range shared.ImportedFiles {
		err := <-v
		if err != nil {
			fmt.Printf("%s: %s\n", k, err.Error())
		}
	}

	irStr, e := llvmir.IrWrite(shared)
	if e != nil {
		return e
	}

	fmt.Println("Llvm IR:")
	fmt.Println(irStr)

	e = os.WriteFile("out.ll", []byte(irStr), 0666)
	if e != nil {
		return e
	}
	return nil
}

func main() {
	err := wrappedMain()
	if err != nil {
		fmt.Printf("fatal error: %s\n", err.Error())
		os.Exit(1)
	}
}
