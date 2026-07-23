package types

import (
	"Magma/src/target"
	"sync"
)

type StructDef struct {
	Module     string
	Name       string
	IsPublic   bool
	TypeParams []string

	FieldNb    map[string]int
	Fields     map[string]*NodeType
	FieldOrder []string
	Funcs      map[string]*NodeFuncDef

	Destructor  *NodeFuncDef
	Destructors []*NodeFuncDef
}

func (*StructDef) Print(int) {
	// This is a filthy hack
}

type MemberAccess struct {
	Type    *NodeType
	FieldNb int

	PtrDeref    bool
	ResultIsPtr bool
}

type FileCtx struct {
	FilePath        string
	ModuleName      string
	PackageName     string
	MainPckgName    string
	Imports         []string
	NativeLibraries []string
	Bundles         []string
	ImportAlias     map[string]string
	Content         []byte
	LineIdx         []int
	Tokens          []Token
	GlNode          *NodeGlobal
	ScopeTree       Scope
}

type SharedState struct {
	Cwd          string
	ExecPath     string
	MainPckgName string
	// ErrorTraceSlots is the number of reusable trace nodes in each runtime
	// shard. It is a power of two so generated code can mask instead of divide.
	ErrorTraceSlots uint64
	Target          target.Target

	ImportedFiles  map[string]<-chan error
	ImportedFilesM sync.Mutex

	Files  map[string]*FileCtx
	FilesM sync.Mutex

	// SourceOverrides lets editor tooling analyze unsaved buffers while imports
	// continue to be loaded from disk.
	SourceOverrides  map[string][]byte
	SourceOverridesM sync.RWMutex

	PipeChans  []<-chan error
	PipeChansM sync.Mutex

	LlvmDecl  map[string]bool
	LlvmDeclM sync.Mutex

	// ExportedSymbols tracks native symbol names across every module in one
	// compilation. Parsing modules may happen concurrently, so registration is
	// protected separately from the LLVM declaration set.
	ExportedSymbols  map[string]string
	ExportedSymbolsM sync.Mutex

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
