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
	// OwnerType is the type of the expression immediately before this field
	// access. Type is the field's actual result type. Keeping both prevents
	// later stages from having to reconstruct pointer transitions in a chain.
	OwnerType *NodeType
	Type      *NodeType
	// OwnerDef is the resolved declaration which owns the field.  FieldNb is
	// only unique within this declaration, so place analysis uses the pair as
	// the canonical field identity rather than source spelling.
	OwnerDef *StructDef
	FieldNb  int

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
	StdRoot      string
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

	// Warnings are non-fatal semantic diagnostics collected after parsing.
	Warnings []Diagnostic
}

type DiagnosticSeverity uint8

const (
	SeverityError DiagnosticSeverity = iota
	SeverityWarning
)

// Diagnostic is the compiler's transport-neutral source diagnostic. Rendering
// for the command line and editor protocols belongs to their respective
// adapters, not to compiler passes.
type Diagnostic struct {
	Severity DiagnosticSeverity
	// Code is a stable machine-readable identifier shared by CLI and editor
	// consumers. Human-readable wording may improve without invalidating tools.
	Code     string
	Stage    string
	Ctx      *FileCtx
	FilePath string
	Token    Token
	Message  string
	// ShortDesc is the legacy name for Message. Constructors keep both set.
	ShortDesc  string
	Additional string
	Cause      error
	Related    []DiagnosticRelated
}

// DiagnosticRelated points at an earlier operation which caused a later
// diagnostic, such as the move or destruction preceding a use-after-move.
type DiagnosticRelated struct {
	FilePath string
	Token    Token
	Message  string
}

func (d *Diagnostic) Error() string {
	if d.Message != "" {
		return d.Message
	}
	return d.ShortDesc
}
func (d *Diagnostic) Unwrap() error { return d.Cause }

// Warning remains an alias while older consumers migrate to Diagnostic.
type Warning = Diagnostic

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
