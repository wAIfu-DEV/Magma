package shared

import (
	"Magma/src/pipeline"
	"Magma/src/target"
	"Magma/src/types"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

func MakeShared(cwd, stdRoot string) (*types.SharedState, error) {
	if stdRoot == "" {
		return nil, fmt.Errorf("standard library path is required")
	}
	absStdRoot, err := filepath.Abs(stdRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve standard library path %q: %w", stdRoot, err)
	}
	info, err := os.Stat(absStdRoot)
	if err != nil {
		return nil, fmt.Errorf("inspect standard library path %q: %w", absStdRoot, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("standard library path %q is not a directory", absStdRoot)
	}

	return &types.SharedState{
		Cwd:              cwd,
		StdRoot:          filepath.Clean(absStdRoot),
		MainPckgName:     "",
		ErrorTraceSlots:  1024,
		Target:           target.HostFallback(runtime.GOOS, runtime.GOARCH),
		ImportedFiles:    map[string]<-chan error{},
		ImportedFilesM:   sync.Mutex{},
		Files:            map[string]*types.FileCtx{},
		FilesM:           sync.Mutex{},
		SourceOverrides:  map[string][]byte{},
		SourceOverridesM: sync.RWMutex{},
		PipeChans:        []<-chan error{},
		PipeChansM:       sync.Mutex{},
		LlvmDecl:         map[string]bool{},
		LlvmDeclM:        sync.Mutex{},
		ExportedSymbols:  map[string]string{},
		ExportedSymbolsM: sync.Mutex{},

		// needed because Go sucks and can't figure out cyclical imports
		PipelineFunc: pipeline.DoAsync,
		WaitGroup:    sync.WaitGroup{},
	}, nil
}
