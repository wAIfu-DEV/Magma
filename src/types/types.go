package types

import (
	"sync"
)

type StructDef struct {
	Name   string
	Fields map[string]*NodeType
	Funcs  map[string]*NodeFuncDef
}

type FileCtx struct {
	FilePath    string
	PackageName string
	Imports     []string
	ImportAlias map[string]string
	Content     []byte
	LineIdx     []int
	Tokens      []Token
	GlNode      *NodeGlobal
	ScopeTree   Scope
}

type SharedState struct {
	Cwd string

	ImportedFiles  map[string]<-chan error
	ImportedFilesM sync.Mutex

	Files  map[string]*FileCtx
	FilesM sync.Mutex

	PipeChans  []<-chan error
	PipeChansM sync.Mutex

	PipelineFunc func(shared *SharedState, filePath string, alias string, fromAbs string, fromGl *NodeGlobal) <-chan error
	WaitGroup    sync.WaitGroup
}

type Scope struct {
	Name       NodeName
	Parent     *Scope
	Associated Node
	ReturnType *NodeType

	DeclVars    map[string]*NodeExprVarDef
	DeclFuncs   map[string]FnScope
	DeclStructs map[string]*NodeStructDef
}

type FnScope struct {
	Func  *NodeFuncDef
	Scope *Scope
}
