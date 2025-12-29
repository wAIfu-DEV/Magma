package shared

import (
	"Magma/src/pipeline"
	"Magma/src/types"
	"os"
	"sync"
)

func MakeShared(cwd string) (*types.SharedState, error) {
	ex, e := os.Executable()
	if e != nil {
		return nil, e
	}

	return &types.SharedState{
		Cwd:            cwd,
		ExecPath:       ex,
		MainPckgName:   "",
		ImportedFiles:  map[string]<-chan error{},
		ImportedFilesM: sync.Mutex{},
		Files:          map[string]*types.FileCtx{},
		FilesM:         sync.Mutex{},
		PipeChans:      []<-chan error{},
		PipeChansM:     sync.Mutex{},
		LlvmDecl:       map[string]bool{},
		LlvmDeclM:      sync.Mutex{},

		// needed because Go sucks and can't figure out cyclical imports
		PipelineFunc: pipeline.DoAsync,
		WaitGroup:    sync.WaitGroup{},
	}, nil
}
