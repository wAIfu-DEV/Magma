package shared

import (
	"Magma/src/pipeline"
	"Magma/src/target"
	"Magma/src/types"
	"os"
	"runtime"
	"sync"
)

func MakeShared(cwd string) (*types.SharedState, error) {
	ex, e := os.Executable()
	if e != nil {
		return nil, e
	}

	return &types.SharedState{
		Cwd:              cwd,
		ExecPath:         ex,
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
