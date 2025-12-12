package types

import (
	"fmt"
	"strings"
	"unsafe"
)

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
	KwComma
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
	KwComma:   ",",
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
	",":   KwComma,
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

func PrintIndent(n int) {
	d := n * 2
	p := make([]byte, d)
	for i := range d {
		p[i] = ' '
	}
	s := *(*string)(unsafe.Pointer(&p))
	fmt.Print(s)
}

type Node interface {
	Print(int)
}

type NodeTypeKind interface {
	IsType()
	Print(int)
}

type NodeExpr interface {
	IsExpr()
	Print(int)
}

type NodeStatement interface {
	IsStatement()
	Print(int)
}

type NodeGlobalDecl interface {
	IsGlobalDecl()
	Print(int)
}

type NodeName interface {
	IsName()
	Print(int)
}

type NodeNameSingle struct {
	Name string
}

func (n *NodeNameSingle) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("NameSingle(%s)\n", n.Name)
}

type NodeNameComposite struct {
	Parts []string
}

func (n *NodeNameComposite) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("NameComposite(%s)\n", strings.Join(n.Parts, "."))
}

type NodeType struct {
	Throws   bool
	KindNode NodeTypeKind
}

func (n *NodeType) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("Type(throw=%t)\n", n.Throws)
	n.KindNode.Print(indent + 1)
}

type NodeTypeNamed struct {
	NameNode NodeName
}

func (n *NodeTypeNamed) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("TypeNamed\n")
	n.NameNode.Print(indent + 1)
}

type NodeExprVoid struct {
}

func (n *NodeExprVoid) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("ExprVoid\n")
}

type NodeStmtRet struct {
	Expression NodeExpr
}

func (n *NodeStmtRet) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("Return\n")
	n.Expression.Print(indent + 1)
}

type NodeArg struct {
	Name     string
	TypeNode NodeType
}

func (n *NodeArg) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("Arg(%s)\n", n.Name)
	n.TypeNode.Print(indent + 1)
}

type NodeArgList struct {
	Args []NodeArg
}

func (n *NodeArgList) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("ArgList\n")

	for _, x := range n.Args {
		x.Print(indent + 1)
	}
}

type NodeBody struct {
	Statements []NodeStatement
}

func (n *NodeBody) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("Body\n")

	for _, x := range n.Statements {
		x.Print(indent + 1)
	}
}

type NodeGenericClass struct {
	NameNode NodeName
	ArgsNode NodeArgList
}

func (n *NodeGenericClass) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("GenericClass\n")
	n.NameNode.Print(indent + 1)
	n.ArgsNode.Print(indent + 1)
}

type NodeFuncDef struct {
	Class      NodeGenericClass
	ReturnType NodeType
	Body       NodeBody
}

func (n *NodeFuncDef) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("FuncDef\n")
	n.Class.Print(indent + 1)
	n.ReturnType.Print(indent + 1)
	n.Body.Print(indent + 1)
}

type NodeStructDef struct {
	Class NodeGenericClass
}

func (n *NodeStructDef) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("StructDef\n")
	n.Class.Print(indent + 1)
}

type NodeGlobal struct {
	Declarations []NodeGlobalDecl
}

func (n *NodeGlobal) Print(indent int) {
	fmt.Printf("\nNode Tree:\n")

	PrintIndent(indent)
	fmt.Printf("Global\n")

	for _, x := range n.Declarations {
		x.Print(indent + 1)
	}
}

func (*NodeExprVoid) IsExpr()        {}
func (*NodeTypeNamed) IsType()       {}
func (*NodeNameSingle) IsName()      {}
func (*NodeNameComposite) IsName()   {}
func (*NodeStmtRet) IsStatement()    {}
func (*NodeFuncDef) IsGlobalDecl()   {}
func (*NodeStructDef) IsGlobalDecl() {}
