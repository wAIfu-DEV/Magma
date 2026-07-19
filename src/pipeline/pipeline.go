package pipeline

import (
	"Magma/src/comp_err"
	"Magma/src/debug"
	lineidx "Magma/src/line_idx"
	"Magma/src/makeabs"
	pipelineasync "Magma/src/pipeline_async"
	randid "Magma/src/rand_id"
	"Magma/src/types"
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"
	"unicode/utf8"
)

var errAlreadyScheduled = errors.New("compilation unit already scheduled")

func registerImportResult(shared *types.SharedState, path string, c <-chan error) {
	shared.ImportedFilesM.Lock()
	if _, found := shared.ImportedFiles[path]; !found {
		shared.ImportedFiles[path] = c
	}
	shared.ImportedFilesM.Unlock()
}

func extractModuleName(fCtx *types.FileCtx, firstLine string) (string, error) {
	if beforeComment, _, found := strings.Cut(firstLine, "#"); found {
		firstLine = beforeComment
	}
	firstLine = strings.TrimSpace(firstLine)

	if !strings.HasPrefix(firstLine, "mod ") {
		start := types.Token{Pos: types.FilePos{Line: 1, Col: 1}}
		return "", comp_err.CompilationErrorToken(
			fCtx,
			&start,
			"syntax error: expected module name declaration as very first line of file",
			"magma files should start with: `mod <modulename>`",
		)
	}

	moduleName := strings.TrimPrefix(firstLine, "mod ")
	moduleName = strings.TrimSpace(moduleName)

	if moduleName == "" {
		nameToken := types.Token{Pos: types.FilePos{Line: 1, Col: 5}}
		return "", comp_err.CompilationErrorToken(
			fCtx,
			&nameToken,
			"syntax error: expected module name after 'mod' keyword",
			"magma files should start with: `mod <modulename>`",
		)
	}

	r, size := utf8.DecodeRuneInString(moduleName)
	if size <= 0 {
		return "", fmt.Errorf("failed to decode first rune in module name")
	}

	if !(unicode.IsLetter(r)) {
		nameToken := types.Token{Repr: string(r), Pos: types.FilePos{Line: 1, Col: 5}}
		return "", comp_err.CompilationErrorToken(
			fCtx,
			&nameToken,
			"syntax error: module name should start with a character in the range [a-zA-Z]",
			"",
		)
	}

	for i, r := range moduleName {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_') {
			nameToken := types.Token{Repr: string(r), Pos: types.FilePos{Line: 1, Col: uint32(i + 5)}}
			return "", comp_err.CompilationErrorToken(
				fCtx,
				&nameToken,
				"syntax error: characters in module name should all be within the range [a-zA-Z_]",
				"",
			)
		}
	}
	return moduleName, nil
}

func pipelineSyncPrelude(shared *types.SharedState, c chan error, filePath string, alias string, fromAbs string, fromGl *types.NodeGlobal) (*types.FileCtx, error) {

	debug.Printf("sync pipeline prelude for: %s\n", filePath)

	absPath, err := makeabs.MakeAbs(filePath, fromAbs)
	if err != nil {
		registerImportResult(shared, filePath, c)
		return nil, err
	}

	debug.Printf("resolved to abs path: %s\n", absPath)

	fileBytes, err := os.ReadFile(absPath)
	if err != nil {
		registerImportResult(shared, absPath, c)
		return nil, fmt.Errorf("failed to open file")
	}

	fCtx := &types.FileCtx{
		FilePath:        absPath,
		Content:         fileBytes,
		ImportAlias:     map[string]string{},
		Imports:         []string{},
		NativeLibraries: []string{},
		LineIdx:         lineidx.GetLineIdx(fileBytes),
	}

	scanner := bufio.NewScanner(bytes.NewReader(fileBytes))
	if !scanner.Scan() {
		registerImportResult(shared, absPath, c)
		return nil, fmt.Errorf("failed to read any data from file")
	}

	firstLine := scanner.Text()
	moduleName, err := extractModuleName(fCtx, firstLine)
	if err != nil {
		registerImportResult(shared, absPath, c)
		return nil, err
	}
	moduleId := randid.RandId(10)
	moduleNameId := moduleName + "_" + moduleId

	fCtx.ModuleName = moduleName
	fCtx.PackageName = moduleNameId

	if fromGl == nil {
		shared.MainPckgName = moduleNameId
	}
	fCtx.MainPckgName = shared.MainPckgName

	// Publish a compilation unit only after its immutable identity has been
	// initialized. The lookup and insertion must be one atomic operation: two
	// import goroutines can discover the same transitive dependency at once.
	shared.FilesM.Lock()
	foundFctx, found := shared.Files[absPath]
	if !found {
		shared.Files[absPath] = fCtx
	}
	shared.FilesM.Unlock()

	if found {
		if fromGl != nil {
			fromGl.ImportAlias[alias] = foundFctx.PackageName
		}
		return foundFctx, errAlreadyScheduled
	}

	if fromGl != nil {
		fromGl.ImportAlias[alias] = fCtx.PackageName
	}

	// Only the goroutine which inserted the unit owns and publishes its result
	// channel. Duplicate importers must not overwrite that channel.
	registerImportResult(shared, absPath, c)

	return fCtx, nil
}

func DoMain(shared *types.SharedState, filePath string) error {
	if err := Do(shared, filePath, "", filePath, nil); err != nil {
		return err
	}
	mainFile := shared.Files[filePath]
	if mainFile == nil {
		absPath, err := makeabs.MakeAbs(filePath, filePath)
		if err == nil {
			mainFile = shared.Files[absPath]
		}
	}
	if mainFile == nil || mainFile.GlNode == nil {
		return fmt.Errorf("main compilation unit was not registered")
	}
	corePath, err := findCorePath(shared)
	if err != nil {
		return err
	}
	mainFile.Imports = append(mainFile.Imports, corePath)
	return Do(shared, corePath, "__core", mainFile.FilePath, mainFile.GlNode)
}

func findCorePath(shared *types.SharedState) (string, error) {
	candidates := []string{
		filepath.Join(filepath.Dir(shared.ExecPath), "std", "core.mg"),
		filepath.Join(shared.Cwd, "std", "core.mg"),
	}
	if _, source, _, ok := runtime.Caller(0); ok {
		candidates = append(candidates, filepath.Join(filepath.Dir(source), "..", "..", "std", "core.mg"))
	}
	for _, candidate := range candidates {
		absolute, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if info, err := os.Stat(absolute); err == nil && !info.IsDir() {
			return absolute, nil
		}
	}
	return "", fmt.Errorf("failed to locate implicitly imported std/core.mg")
}

func Do(shared *types.SharedState, filePath string, alias string, fromAbs string, fromGl *types.NodeGlobal) error {
	debug.Printf("running pipeline for: %s with alias: %s from file: %s\n", filePath, alias, fromAbs)

	c := make(chan error, 1)

	shared.WaitGroup.Add(1)

	fCtx, err := pipelineSyncPrelude(shared, c, filePath, alias, fromAbs, fromGl)

	if errors.Is(err, errAlreadyScheduled) {
		c <- nil
		close(c)
		shared.WaitGroup.Done()
		return nil
	}

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
	debug.Printf("running async pipeline for: %s with alias: %s from file: %s\n", filePath, alias, fromAbs)

	c := make(chan error, 1)

	shared.WaitGroup.Add(1)

	fCtx, err := pipelineSyncPrelude(shared, c, filePath, alias, fromAbs, fromGl)

	if errors.Is(err, errAlreadyScheduled) {
		c <- nil
		close(c)
		shared.WaitGroup.Done()
		return c
	}

	if err != nil {
		c <- err
		close(c)
		shared.WaitGroup.Done()
		return c
	}

	go pipelineasync.PipelineAsync(shared, c, fCtx, filePath, alias)
	return c
}
