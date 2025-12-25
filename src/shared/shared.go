package shared

import (
	"Magma/src/pipeline"
	"Magma/src/types"
	"sync"
)

func MakeShared(cwd string) *types.SharedState {
	return &types.SharedState{
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
}
