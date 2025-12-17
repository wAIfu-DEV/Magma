package pipeline

import (
	"Magma/src/comp_err"
	lineidx "Magma/src/line_idx"
	"Magma/src/makeabs"
	pipelineasync "Magma/src/pipeline_async"
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

func pipelineSyncPrelude(shared *types.SharedState, c chan error, filePath string, alias string, fromAbs string) (*types.FileCtx, error) {

	absPath, err := makeabs.MakeAbs(filePath, fromAbs)
	if err != nil {
		shared.ImportM.Lock()
		shared.ImportedFiles[filePath] = c
		shared.ImportM.Unlock()
		return nil, err
	}

	shared.ImportM.Lock()
	shared.ImportedFiles[absPath] = c
	shared.ImportM.Unlock()

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

	fmt.Printf("read first line\n")
	scanner := bufio.NewScanner(bytes.NewReader(fileBytes))

	if !scanner.Scan() {
		return nil, fmt.Errorf("failed to read any data from file")
	}

	firstLine := scanner.Text()

	fmt.Printf("extract module name\n")
	moduleName, err := extractModuleName(fCtx, firstLine)
	if err != nil {
		return nil, err
	}

	fCtx.PackageName = moduleName
	return fCtx, nil
}

func Pipeline(shared *types.SharedState, filePath string, alias string, fromAbs string) error {
	c := make(chan error, 1)

	shared.WaitGroup.Add(1)

	fCtx, err := pipelineSyncPrelude(shared, c, filePath, alias, fromAbs)
	if err != nil {
		return err
	}

	// running as Sync
	pipelineasync.PipelineAsync(shared, c, fCtx, filePath, alias)
	return <-c
}

func PipelineAsync(shared *types.SharedState, filePath string, alias string, fromAbs string) <-chan error {
	c := make(chan error, 1)

	shared.WaitGroup.Add(1)

	fCtx, err := pipelineSyncPrelude(shared, c, filePath, alias, fromAbs)
	if err != nil {
		c <- err
		close(c)
		shared.WaitGroup.Done()
		return c
	}

	go pipelineasync.PipelineAsync(shared, c, fCtx, filePath, alias)
	return c
}
