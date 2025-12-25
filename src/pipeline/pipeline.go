package pipeline

import (
	"Magma/src/comp_err"
	lineidx "Magma/src/line_idx"
	"Magma/src/makeabs"
	pipelineasync "Magma/src/pipeline_async"
	randid "Magma/src/rand_id"
	"Magma/src/types"
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

func extractModuleName(fCtx *types.FileCtx, firstLine string) (string, error) {
	firstLine = strings.TrimSpace(firstLine)

	if !strings.HasPrefix(firstLine, "mod ") {
		return "", comp_err.CompilationErrorToken(
			fCtx,
			&types.Token{},
			"syntax error: expected module name declaration as very first line of file",
			"magma files should start with: `mod <modulename>`",
		)
	}

	moduleName := strings.TrimPrefix(firstLine, "mod ")
	moduleName = strings.TrimSpace(moduleName)

	if moduleName == "" {
		return "", comp_err.CompilationErrorToken(
			fCtx,
			&types.Token{},
			"syntax error: expected module name after 'mod' keyword",
			"magma files should start with: `mod <modulename>`",
		)
	}

	r, size := utf8.DecodeRuneInString(moduleName)
	if size <= 0 {
		return "", fmt.Errorf("failed to decode first rune in module name")
	}

	if !(unicode.IsLetter(r)) {
		return "", comp_err.CompilationErrorToken(
			fCtx,
			&types.Token{},
			"syntax error: module name should start with a character in the range [a-zA-Z]",
			"",
		)
	}

	for _, r := range moduleName {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_') {
			return "", comp_err.CompilationErrorToken(
				fCtx,
				&types.Token{},
				"syntax error: characters in module name should all be within the range [a-zA-Z_]",
				"",
			)
		}
	}
	return moduleName, nil
}

func pipelineSyncPrelude(shared *types.SharedState, c chan error, filePath string, alias string, fromAbs string, fromGl *types.NodeGlobal) (*types.FileCtx, error) {

	fmt.Printf("sync pipeline prelude for: %s\n", filePath)

	absPath, err := makeabs.MakeAbs(filePath, fromAbs)
	if err != nil {
		shared.ImportedFilesM.Lock()
		shared.ImportedFiles[filePath] = c
		shared.ImportedFilesM.Unlock()
		return nil, err
	}

	fmt.Printf("resolved to abs path: %s\n", absPath)

	shared.ImportedFilesM.Lock()
	shared.ImportedFiles[absPath] = c
	shared.ImportedFilesM.Unlock()

	fileBytes, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file")
	}

	fCtx := &types.FileCtx{
		FilePath:    absPath,
		Content:     fileBytes,
		ImportAlias: map[string]string{},
		Imports:     []string{},
		LineIdx:     lineidx.GetLineIdx(fileBytes),
	}

	shared.FilesM.Lock()
	shared.Files[absPath] = fCtx
	shared.FilesM.Unlock()

	scanner := bufio.NewScanner(bytes.NewReader(fileBytes))
	if !scanner.Scan() {
		return nil, fmt.Errorf("failed to read any data from file")
	}

	firstLine := scanner.Text()
	moduleName, err := extractModuleName(fCtx, firstLine)
	if err != nil {
		return nil, err
	}

	moduleId := randid.RandId(5)
	moduleNameId := moduleName + "_" + moduleId

	fCtx.PackageName = moduleNameId

	if fromGl != nil {
		fromGl.ImportAlias[alias] = moduleNameId
	} else {
		shared.MainPckgName = moduleNameId
	}

	fCtx.MainPckgName = shared.MainPckgName
	return fCtx, nil
}

func DoMain(shared *types.SharedState, filePath string) error {
	return Do(shared, filePath, "", filePath, nil)
}

func Do(shared *types.SharedState, filePath string, alias string, fromAbs string, fromGl *types.NodeGlobal) error {
	fmt.Printf("running pipeline for: %s with alias: %s from file: %s\n", filePath, alias, fromAbs)

	c := make(chan error, 1)

	shared.WaitGroup.Add(1)

	fCtx, err := pipelineSyncPrelude(shared, c, filePath, alias, fromAbs, fromGl)
	if err != nil {
		c <- err
		close(c)
		shared.WaitGroup.Done()
		return err
	}

	// running as Sync
	pipelineasync.PipelineAsync(shared, c, fCtx, filePath, alias)
	return <-c
}

func DoAsync(shared *types.SharedState, filePath string, alias string, fromAbs string, fromGl *types.NodeGlobal) <-chan error {
	fmt.Printf("running pipelineasync for: %s with alias: %s from file: %s\n", filePath, alias, fromAbs)

	c := make(chan error, 1)

	shared.WaitGroup.Add(1)

	fCtx, err := pipelineSyncPrelude(shared, c, filePath, alias, fromAbs, fromGl)
	if err != nil {
		c <- err
		close(c)
		shared.WaitGroup.Done()
		return c
	}

	go pipelineasync.PipelineAsync(shared, c, fCtx, filePath, alias)
	return c
}
