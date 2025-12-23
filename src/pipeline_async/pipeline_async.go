package pipelineasync

import (
	"Magma/src/parser"
	scopeinfo "Magma/src/scope_info"
	"Magma/src/tokenizer"
	"Magma/src/types"
	"fmt"
)

// Do not call outside context of pipeline.Do* functions
func PipelineAsync(shared *types.SharedState, c chan error, fCtx *types.FileCtx, filePath string, alias string) {
	defer shared.WaitGroup.Done()

	var err error = nil

	fmt.Printf("started async pipeline for file: %s\n", filePath)
	defer fmt.Printf("exited async pipeline for: %s\n", filePath)

	fCtx.Tokens, err = tokenizer.Tokenize(fCtx, fCtx.Content)
	if err != nil {
		c <- err
		close(c)
		return
	}
	tokenizer.PrintTokens(fCtx.Tokens)

	fCtx.GlNode, err = parser.Parse(shared, fCtx)
	if err != nil {
		c <- err
		close(c)
		return
	}

	fCtx.GlNode.Print(0)

	fCtx.ScopeTree, err = scopeinfo.BuildScopeTree(fCtx.GlNode)
	if err != nil {
		c <- err
		close(c)
		return
	}

	scopeinfo.PrintScopeTree(&fCtx.ScopeTree, 0)

	c <- nil
	close(c)
}
