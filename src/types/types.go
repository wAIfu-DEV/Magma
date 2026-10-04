package types

import (
	"Magma/src/target"
	"sync"
)

type StructDef struct {
	Module string
	Name   string
	// CoreRole identifies a language-level core type whose canonical storage
	// layout is supplied by this source declaration.
	CoreRole   CoreTypeRole
	IsPublic   bool
	TypeParams []string

	FieldNb    map[string]int
	Fields     map[string]*NodeType
	FieldOrder []string
	Funcs      map[string]*NodeFuncDef
	// Implements records the prototype types named by `impl` on this struct.
	// Resolution fills Proto after imports and named types are available.
	Implements  []*ProtoImpl
	IsProto     bool
	Proto       *ProtoDef
	LayoutSize  int
	LayoutAlign int

	Destructor  *NodeFuncDef
	Destructors []*NodeFuncDef
}

type UnionDef struct {
	Module   string
	Name     string
	IsPublic bool
	Variants []*UnionVariant
}

type UnionVariant struct {
	Name   string
	Tk     Token
	Tag    int
	Fields []NodeArg
	Owner  *UnionDef
}

type ProtoDef struct {
	Module     string
	Name       string
	IsPublic   bool
	TypeParams []string
	Methods    []*ProtoMethod
	MethodMap  map[string]*ProtoMethod
	VtableName string
}

type ProtoMethod struct {
	Name       string
	Args       []NodeArg
	Ret        *NodeType
	ContextABI ContextABI
	Slot       int
	Tk         Token
	FnDef      *NodeFuncDef
	Proto      *ProtoDef
}

type ProtoImpl struct {
	Type  *NodeType
	Proto *ProtoDef
	Owner *StructDef
	Tk    Token
}

func ProtoVtableSymbol(implementation *StructDef, proto *ProtoDef) string {
	return implementation.Module + "." + implementation.Name + ".__proto." + proto.Module + "." + proto.Name
}

func ProtoBorrowVtableSymbol(implementation *StructDef, proto *ProtoDef) string {
	return ProtoVtableSymbol(implementation, proto) + ".borrow"
}

func ProtoBorrowThunkSymbol(implementation *StructDef, proto *ProtoDef, method *ProtoMethod) string {
	return ProtoBorrowVtableSymbol(implementation, proto) + "." + method.Name
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
	ModuleID        ModuleID
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
	// InterfaceOnly marks a declaration-only module materialized from .mgi.
	// It has no implementation source or bodies to check or lower.
	InterfaceOnly bool
	// InterfaceSnapshot is canonical pre-specialization semantic interface data.
	// It is not an AST cache and remains stable as concrete generic requests vary.
	InterfaceSnapshot []byte
}

type SharedState struct {
	Cwd          string
	StdRoot      string
	MainPckgName string
	// ErrorTraceSlots is the number of reusable trace nodes in each runtime
	// shard. It is a power of two so generated code can mask instead of divide.
	ErrorTraceSlots uint64
	NullContext     bool
	Target          target.Target
	CoreTypes       map[CoreTypeRole]*StructDef
	CoreMethods     map[string]*NodeFuncDef
	CompilerArgs    map[string]string

	ImportedFiles  map[string]<-chan error
	ImportedFilesM sync.Mutex

	Files  map[string]*FileCtx
	FilesM sync.Mutex
	// ModuleNames detects the unlikely case where two full module identities
	// produce the same shortened package name used by LLVM symbols.
	ModuleNames  map[string]ModuleID
	ModuleNamesM sync.Mutex
	// InterfaceFiles contains declaration-only modules keyed by the source path
	// used in local import statements during interface-backed compilation.
	InterfaceFiles map[string]*FileCtx
	// GenericProviderLoader reparses a declaration-only provider on demand when
	// specialization needs its template body. Ordinary interface-backed calls
	// never invoke it.
	GenericProviderLoader func(packageName string) (*FileCtx, error)
	// GenericSpecializationHit reports whether provider-owned specialization
	// bitcode is already available, allowing monomorphization to materialize a
	// signature-only local stub without reparsing the provider body.
	GenericSpecializationHit func(packageName, symbol string) bool

	// SourceOverrides lets editor tooling analyze unsaved buffers while imports
	// continue to be loaded from disk.
	SourceOverrides  map[string][]byte
	SourceOverridesM sync.RWMutex

	PipeChans  []<-chan error
	PipeChansM sync.Mutex

	LlvmDecl            map[string]bool
	LlvmDeclM           sync.Mutex
	NativeDeclarations  map[string]string
	NativeDeclarationsM sync.Mutex

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
