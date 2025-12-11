package types

type TokType uint8

const (
	TokNone TokType = iota
	TokName
	TokLitStr
	TokLitNum
	TokKeyword
)

var TokTypeToRepr []string = []string{
	TokNone:    "None",
	TokName:    "Name",
	TokLitStr:  "LitStr",
	TokLitNum:  "LitNum",
	TokKeyword: "Keyword",
}

type KwType uint8

const (
	KwNone KwType = iota
	KwEqual
	KwParenOp
	KwParenCl
	KwColon
	KwDot
	KwDots
	KwExclam
	KwModule
	KwUse
	KwPublic
	KwReturn
	KwNewline
)

var KwTypeToRepr []string = []string{
	KwNone:    "",
	KwEqual:   "=",
	KwParenOp: "(",
	KwParenCl: ")",
	KwColon:   ":",
	KwDot:     ".",
	KwDots:    "..",
	KwExclam:  "!",
	KwModule:  "mod",
	KwUse:     "use",
	KwPublic:  "pub",
	KwReturn:  "ret",
	KwNewline: "\n",
}

var KwReprToType map[string]KwType = map[string]KwType{
	"":    KwNone,
	"=":   KwEqual,
	"(":   KwParenOp,
	")":   KwParenCl,
	":":   KwColon,
	".":   KwDot,
	"..":  KwDots,
	"!":   KwExclam,
	"mod": KwModule,
	"use": KwUse,
	"pub": KwPublic,
	"ret": KwReturn,
	"\n":  KwNewline,
}

type Token struct {
	Repr     string
	Pos      FilePos
	Type     TokType
	KeywType KwType
}

type FilePos struct {
	Line uint32
	Col  uint32
}

type FileCtx struct {
	FilePath    string
	PackageName string
	Imports     []string
	Content     []byte
	LineIdx     []int
	Tokens      []Token
	GlNode      NodeGlobal
}

type NodeT uint8

const (
	NdNone NodeT = iota
	NdType
	NdTypeNamed
	NdGlobal
	NdGlobalDecl
	NdFuncDef
	NdBody
	NdStatement
	NdStmtRet
)

type NodeTypeKind interface {
	IsType()
}

type NodeExpr interface {
	IsExpr()
}

type NodeStatement interface {
	IsStatement()
}

type NodeGlobalDecl interface {
	IsGlobalDecl()
}

type NodeName interface {
	IsName()
}

type NodeNameSingle struct {
	Name string
}

type NodeNameComposite struct {
	Parts []string
}

type NodeType struct {
	Throws   bool
	KindNode NodeTypeKind
}

type NodeTypeNamed struct {
	NameNode NodeName
}

type NodeExprVoid struct {
}

type NodeStmtRet struct {
	Expression NodeExpr
}

type NodeArg struct {
	Name     string
	TypeNode NodeType
}

type NodeArgList struct {
	Args []NodeArg
}

type NodeBody struct {
	Statements []NodeStatement
}

type NodeGenericClass struct {
	NameNode NodeName
	ArgsNode NodeArgList
}

type NodeFuncDef struct {
	Class      NodeGenericClass
	ReturnType NodeType
	Body       NodeBody
}

type NodeStructDef struct {
	Class NodeGenericClass
}

type NodeGlobal struct {
	Declarations []NodeGlobalDecl
}

func (*NodeExprVoid) IsExpr()      {}
func (*NodeTypeNamed) IsType()     {}
func (*NodeNameSingle) IsName()    {}
func (*NodeNameComposite) IsName() {}
func (*NodeStmtRet) IsStatement()  {}
func (*NodeFuncDef) IsGlobalDecl() {}
