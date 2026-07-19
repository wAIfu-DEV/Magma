package types

import (
	"fmt"
	"strings"
	"unsafe"
)

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
	GetInferredType() *NodeType
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
	Tk   Token
	Name string
}

func (n *NodeNameSingle) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("NameSingle(name=%s)\n", n.Name)
}

type NodeNameComposite struct {
	Tokens []Token
	Parts  []string
}

func (n *NodeNameComposite) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("NameComposite\n")
	for _, x := range n.Parts {
		PrintIndent(indent + 1)
		fmt.Printf("%s\n", x)
	}
}

type NodeType struct {
	KindNode   NodeTypeKind
	Destructor *NodeFuncDef
	Throws     bool
	// Owned marks an ownership-transfer position. It does not affect layout.
	Owned bool
}

func (n *NodeType) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("Type(throw=%t)\n", n.Throws)
	n.KindNode.Print(indent + 1)
}

type NodeTypeNamed struct {
	NameNode    NodeName
	GenericArgs []*NodeType
}

func (n *NodeTypeNamed) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("TypeNamed\n")
	n.NameNode.Print(indent + 1)
	if len(n.GenericArgs) > 0 {
		PrintIndent(indent + 1)
		fmt.Printf("GenericArgs\n")
		for _, g := range n.GenericArgs {
			g.Print(indent + 2)
		}
	}
}

type NodeTypeAbsolute struct {
	AbsoluteName string
}

func (n *NodeTypeAbsolute) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("TypeAbsolute(name='%s')\n", n.AbsoluteName)
}

type NodeTypePointer struct {
	Kind NodeTypeKind
}

func (n *NodeTypePointer) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("TypePointer\n")
	n.Kind.Print(indent + 1)
}

type NodeTypeRfc struct {
	Kind NodeTypeKind
}

func (n *NodeTypeRfc) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("TypeRfc\n")
	n.Kind.Print(indent + 1)
}

type NodeTypeSlice struct {
	HasSize  bool
	Size     int
	ElemKind NodeTypeKind
}

func (n *NodeTypeSlice) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("TypeSlice\n")
	n.ElemKind.Print(indent + 1)
}

type NodeTypeFunc struct {
	Args    []*NodeType
	RetType *NodeType
}

func (n *NodeTypeFunc) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("TypeFunc\n")

	PrintIndent(indent + 1)
	fmt.Printf("ArgsType\n")

	for _, n2 := range n.Args {
		n2.Print(indent + 2)
	}

	PrintIndent(indent + 1)
	fmt.Printf("RetType\n")

	n.RetType.Print(indent + 2)
}

type NodeExprVoid struct {
	VoidType *NodeType
}

func (n *NodeExprVoid) GetInferredType() *NodeType {
	//fmt.Println("ExprVoid")
	return n.VoidType
}

func (n *NodeExprVoid) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("ExprVoid\n")
}

type NodeExprUnary struct {
	Tk       Token
	Operator KwType
	Operand  NodeExpr

	InfType *NodeType
}

func (n *NodeExprUnary) GetInferredType() *NodeType {
	//fmt.Println("ExprUnary")
	return n.InfType
}

func (n *NodeExprUnary) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("ExprUnary(op=%s)\n", KwTypeToRepr[n.Operator])
	n.Operand.Print(indent + 1)
}

type NodeExprLit struct {
	Tk      Token
	Value   string
	LitType TokType

	InfType *NodeType
}

func (n *NodeExprLit) GetInferredType() *NodeType {
	//fmt.Println("ExprLit")
	return n.InfType
}

func (n *NodeExprLit) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("ExprLit(type=%s, '%s')\n", TokTypeToRepr[n.LitType], strings.ReplaceAll(n.Value, "\n", "\\n"))
}

type NodeLlvm struct {
	Text string
}

func (n *NodeLlvm) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("Llvm('%s')\n", strings.ReplaceAll(n.Text, "\n", "\\n"))
}

type NodeExprName struct {
	Tk   Token
	Name NodeName
	// GenericArgs specializes a generic function when its name is used as a
	// value (as opposed to being called immediately).
	GenericArgs []*NodeType

	InfType        *NodeType
	MemberAccesses []*MemberAccess

	AssociatedNode Node
	IsSsa          bool
}

func (n *NodeExprName) GetInferredType() *NodeType {
	//fmt.Println("ExprName")
	return n.InfType
}

func (n *NodeExprName) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("ExprName\n")
	n.Name.Print(indent + 1)
	for _, g := range n.GenericArgs {
		g.Print(indent + 1)
	}
}

type NodeExprCall struct {
	Tk          Token
	Callee      NodeExpr
	Args        []NodeExpr
	GenericArgs []*NodeType

	AssociatedFnDef *NodeFuncDef
	InfType         *NodeType

	IsMemberFunc      bool
	MemberOwnerType   *NodeType
	MemberOwnerName   *NodeExprName
	MemberOwnerExpr   NodeExpr
	MemberOwnerIsPtr  bool
	MemberOwnerModule string

	IsFuncPointer bool
	FuncPtrType   *NodeType
	FuncPtrOwner  *NodeExprName
}

type NodeStructFieldInit struct {
	Tk         Token
	Name       string
	Expression NodeExpr
	FieldIndex int
	FieldType  *NodeType
}

// NodeExprStructInit is syntactically distinguished from a call by its
// name=value argument list.
type NodeExprStructInit struct {
	Type   *NodeType
	Fields []NodeStructFieldInit
	Tk     Token
}

func (n *NodeExprStructInit) GetInferredType() *NodeType { return n.Type }
func (n *NodeExprStructInit) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("ExprStructInit\n")
	n.Type.Print(indent + 1)
	for _, field := range n.Fields {
		PrintIndent(indent + 1)
		fmt.Printf("Field(%s)\n", field.Name)
		field.Expression.Print(indent + 2)
	}
}

func (n *NodeExprCall) GetInferredType() *NodeType {
	//fmt.Println("ExprCall")
	return n.InfType
}

func (n *NodeExprCall) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("ExprCall\n")

	PrintIndent(indent + 1)
	fmt.Printf("Callee\n")
	n.Callee.Print(indent + 2)

	PrintIndent(indent + 1)
	fmt.Printf("ArgExprs\n")
	for _, expr := range n.Args {
		expr.Print(indent + 2)
	}
	if len(n.GenericArgs) > 0 {
		PrintIndent(indent + 1)
		fmt.Printf("GenericArgs\n")
		for _, g := range n.GenericArgs {
			g.Print(indent + 2)
		}
	}
}

type NodeExprSubscript struct {
	Tk     Token
	Target NodeExpr
	Expr   NodeExpr

	AssociatedNode Node
	IsTargetSsa    bool

	BoxType  *NodeType
	ElemType *NodeType
}

type NodeExprMemberAccess struct {
	Tk     Token
	Target NodeExpr
	Member string

	Access  *MemberAccess
	InfType *NodeType
}

func (n *NodeExprSubscript) GetInferredType() *NodeType {
	//fmt.Println("ExprSubscript")
	return n.ElemType
}

func (n *NodeExprMemberAccess) GetInferredType() *NodeType {
	//fmt.Println("ExprMemberAccess")
	return n.InfType
}

func (n *NodeExprSubscript) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("ExprSubscript\n")

	PrintIndent(indent + 1)
	fmt.Printf("Target\n")
	n.Target.Print(indent + 2)

	PrintIndent(indent + 1)
	fmt.Printf("Expr\n")
	n.Expr.Print(indent + 2)
}

func (n *NodeExprMemberAccess) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("ExprMemberAccess(member=%s)\n", n.Member)

	PrintIndent(indent + 1)
	fmt.Printf("Target\n")
	n.Target.Print(indent + 2)
}

type NodeExprBinary struct {
	Tk       Token
	Operator KwType
	Left     NodeExpr
	Right    NodeExpr

	InfType *NodeType
}

func (n *NodeExprBinary) GetInferredType() *NodeType {
	//fmt.Println("ExprBinary")
	return n.InfType
}

func (n *NodeExprBinary) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("ExprBinary(op=%s)\n", KwTypeToRepr[n.Operator])

	PrintIndent(indent + 1)
	fmt.Printf("Left\n")
	n.Left.Print(indent + 2)

	PrintIndent(indent + 1)
	fmt.Printf("Right\n")
	n.Right.Print(indent + 2)
}

type NameTypePair struct {
	Name NodeName
	Type *NodeType
}

type NodeExprVarDef struct {
	Name        NodeName
	Type        *NodeType
	Initializer NodeExpr
	IsConst     bool
	AbsName     string
	RetFlagId   string
	IsSsa       bool
	IsReturned  bool
	IsGlobal    bool
	IrName      string
}

func (n *NodeExprVarDef) GetInferredType() *NodeType {
	//fmt.Println("ExprVarDef")
	return n.Type
}

func (n *NodeExprVarDef) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("ExprVarDef\n")
	n.Name.Print(indent + 1)

	if n.Type != nil {
		n.Type.Print(indent + 1)
	} else {
		PrintIndent(indent + 1)
		fmt.Printf("Type: <TO INFER>\n")
	}
}

type NodeExprVarDefAssign struct {
	Tk Token

	VarDef     *NodeExprVarDef
	AssignExpr NodeExpr
}

type NodeConstDef struct {
	Tk          Token
	VarDef      *NodeExprVarDef
	Initializer NodeExpr
}

func (n *NodeConstDef) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("ConstDef\n")
	n.VarDef.Print(indent + 1)
	n.Initializer.Print(indent + 1)
}

func (n *NodeExprVarDefAssign) GetInferredType() *NodeType {
	//fmt.Println("ExprVarDefAssign")
	return n.VarDef.Type
}

func (n *NodeExprVarDefAssign) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("ExprVarDefAssign\n")
	n.VarDef.Print(indent + 1)
	PrintIndent(indent + 1)
	fmt.Printf("AssignExpr\n")
	n.AssignExpr.Print(indent + 2)
}

type NodeExprAssign struct {
	Tk Token

	Left  NodeExpr
	Right NodeExpr

	InfType *NodeType
}

func (n *NodeExprAssign) GetInferredType() *NodeType {
	//fmt.Println("ExprAssign")
	return n.InfType
}

func (n *NodeExprAssign) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("ExprAssign\n")
	PrintIndent(indent + 1)
	fmt.Printf("Left\n")
	n.Left.Print(indent + 2)
	PrintIndent(indent + 1)
	fmt.Printf("Right\n")
	n.Right.Print(indent + 2)
}

type NodeExprTry struct {
	Call    NodeExpr
	Tk      Token
	Pos     FilePos
	InfType *NodeType
}

func (n *NodeExprTry) GetInferredType() *NodeType {
	return n.InfType
}

func (n *NodeExprTry) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("ExprTry\n")
	n.Call.Print(indent + 1)
}

type NodeExprSizeof struct {
	Tk      Token
	Type    *NodeType
	InfType *NodeType
}

func (n *NodeExprSizeof) GetInferredType() *NodeType {
	//fmt.Println("ExprSizeof")
	return n.InfType
}

func (n *NodeExprSizeof) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("ExprSizeof\n")
	if n.Type == nil {
		PrintIndent(indent + 1)
		fmt.Println("<null type>")
		return
	}
	n.Type.Print(indent + 1)
}

type NodeExprAddrof struct {
	Tk      Token
	Expr    NodeExpr
	InfType *NodeType
}

func (n *NodeExprAddrof) GetInferredType() *NodeType {
	//fmt.Println("ExprAddrof")
	return n.InfType
}

func (n *NodeExprAddrof) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("ExprAddrof\n")
	n.Expr.Print(indent + 1)
}

type NodeExprDestructor struct {
	VarDef     *NodeExprVarDef
	Destructor *NodeFuncDef
}

func (n *NodeExprDestructor) GetInferredType() *NodeType {
	//fmt.Println("ExprTry")
	return nil
}

func (n *NodeExprDestructor) Print(int) {}

type NodeStmtRet struct {
	Tk         Token
	Expression NodeExpr

	OwnerFuncType *NodeType
}

func (n *NodeStmtRet) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("StmtRet\n")
	n.Expression.Print(indent + 1)
}

type NodeStmtContinue struct{ Tk Token }

func (n *NodeStmtContinue) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("StmtContinue\n")
}

type NodeStmtBreak struct{ Tk Token }

func (n *NodeStmtBreak) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("StmtBreak\n")
}

type NodeStmtIf struct {
	Tk       Token
	CondExpr NodeExpr
	Body     NodeBody

	NextCondStmt NodeStatement
}

func (n *NodeStmtIf) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("StmtIf\n")

	PrintIndent(indent + 1)
	fmt.Printf("CondExpr\n")
	n.CondExpr.Print(indent + 2)

	n.Body.Print(indent + 1)

	PrintIndent(indent + 1)
	fmt.Printf("Next\n")

	if n.NextCondStmt == nil {
		PrintIndent(indent + 2)
		fmt.Println("<null>")
	} else {
		n.NextCondStmt.Print(indent + 2)
	}
}

type NodeStmtElse struct {
	Body NodeBody
}

func (n *NodeStmtElse) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("StmtElse\n")

	n.Body.Print(indent + 1)
}

type NodeStmtWhile struct {
	Tk       Token
	CondExpr NodeExpr
	Body     NodeBody
}

func (n *NodeStmtWhile) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("StmtWhile\n")

	PrintIndent(indent + 1)
	fmt.Printf("CondExpr\n")
	n.CondExpr.Print(indent + 2)

	n.Body.Print(indent + 1)
}

type NodeStmtExpr struct {
	Expression NodeExpr
}

func (n *NodeStmtExpr) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("StmtExpr\n")
	n.Expression.Print(indent + 1)
}

type NodeStmtThrow struct {
	Tk         Token
	Expression NodeExpr
	Pos        FilePos
}

func (n *NodeStmtThrow) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("StmtThrow\n")
	n.Expression.Print(indent + 1)
}

type NodeStmtDefer struct {
	Expression NodeExpr
	Body       NodeBody
	IsBody     bool
}

func (n *NodeStmtDefer) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("StmtDefer\n")
	if n.IsBody {
		n.Body.Print(indent + 1)
	} else {
		n.Expression.Print(indent + 1)
	}
}

type NodeExprDestructureAssign struct {
	ValueDef NodeExprVarDef
	ErrDef   NodeExprVarDef
	Call     *NodeExprCall
}

func (n *NodeExprDestructureAssign) GetInferredType() *NodeType {
	//fmt.Println("ExprDestructureAssign")
	return n.ValueDef.Type
}

func (n *NodeExprDestructureAssign) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("ExprDestructureAssign\n")

	PrintIndent(indent + 1)
	fmt.Printf("ValueDef\n")
	n.ValueDef.Print(indent + 2)

	PrintIndent(indent + 1)
	fmt.Printf("ErrDef\n")
	n.ErrDef.Print(indent + 2)

	PrintIndent(indent + 1)
	fmt.Printf("Call\n")
	n.Call.Print(indent + 2)
}

type NodeArg struct {
	Tk       Token
	Name     string
	TypeNode *NodeType
}

func (n *NodeArg) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("Arg(name=%s)\n", n.Name)
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
	Scope      *Scope
}

func (n *NodeBody) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("Body\n")

	for _, x := range n.Statements {
		x.Print(indent + 1)
	}
}

type NodeGenericClass struct {
	NameNode        NodeName
	ArgsNode        NodeArgList
	TypeParams      []string
	OwnerTypeParams []string
}

func (n *NodeGenericClass) Print(indent int) {
	PrintIndent(indent)
	fmt.Printf("GenericClass\n")
	n.NameNode.Print(indent + 1)
	if len(n.TypeParams) > 0 {
		PrintIndent(indent + 1)
		fmt.Printf("TypeParams\n")
		for _, p := range n.TypeParams {
			PrintIndent(indent + 2)
			fmt.Printf("%s\n", p)
		}
	}
	if len(n.OwnerTypeParams) > 0 {
		PrintIndent(indent + 1)
		fmt.Printf("OwnerTypeParams\n")
		for _, p := range n.OwnerTypeParams {
			PrintIndent(indent + 2)
			fmt.Printf("%s\n", p)
		}
	}
	n.ArgsNode.Print(indent + 1)
}

type NodeFuncDef struct {
	Class       NodeGenericClass
	ReturnType  *NodeType
	Body        NodeBody
	AbsName     string
	NoAliasName string
	DisplayName string

	Deferred []*NodeStmtDefer
	DeferCnt int
	HasDefer bool

	IsDestructor   bool
	IsExternal     bool
	ErrorPredicate ErrorPredicateKind
}

type ErrorPredicateKind uint8

const (
	ErrorPredicateNone ErrorPredicateKind = iota
	ErrorPredicateOk
	ErrorPredicateNok
)

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

	ImportAlias map[string]string

	StructDefs           map[string]*StructDef
	FuncDefs             map[string]*NodeFuncDef
	PrimitiveMethods     map[string]map[string]*NodeFuncDef
	PrimitiveDestructors map[string][]*NodeFuncDef
}

func (n *NodeGlobal) Print(indent int) {
	fmt.Printf("\nNode Tree:\n")

	PrintIndent(indent)
	fmt.Printf("Global\n")

	for _, x := range n.Declarations {
		x.Print(indent + 1)
	}

	fmt.Printf("\n")
}

type ModuleBundle struct {
	Main    *NodeGlobal
	Modules map[string]*NodeGlobal
}

func (*NodeExprVoid) IsExpr()              {}
func (*NodeExprUnary) IsExpr()             {}
func (*NodeExprLit) IsExpr()               {}
func (*NodeExprName) IsExpr()              {}
func (*NodeExprCall) IsExpr()              {}
func (*NodeExprStructInit) IsExpr()        {}
func (*NodeExprSubscript) IsExpr()         {}
func (*NodeExprMemberAccess) IsExpr()      {}
func (*NodeExprBinary) IsExpr()            {}
func (*NodeExprVarDef) IsExpr()            {}
func (*NodeExprVarDefAssign) IsExpr()      {}
func (*NodeExprAssign) IsExpr()            {}
func (*NodeExprTry) IsExpr()               {}
func (*NodeExprDestructureAssign) IsExpr() {}
func (*NodeExprDestructor) IsExpr()        {}
func (*NodeExprSizeof) IsExpr()            {}
func (*NodeExprAddrof) IsExpr()            {}
func (*NodeTypeNamed) IsType()             {}
func (*NodeTypePointer) IsType()           {}
func (*NodeTypeRfc) IsType()               {}
func (*NodeTypeSlice) IsType()             {}
func (*NodeTypeFunc) IsType()              {}
func (*NodeTypeAbsolute) IsType()          {}
func (*NodeNameSingle) IsName()            {}
func (*NodeNameComposite) IsName()         {}
func (*NodeStmtRet) IsStatement()          {}
func (*NodeStmtContinue) IsStatement()     {}
func (*NodeStmtBreak) IsStatement()        {}
func (*NodeStmtExpr) IsStatement()         {}
func (*NodeStmtThrow) IsStatement()        {}
func (*NodeStmtIf) IsStatement()           {}
func (*NodeStmtElse) IsStatement()         {}
func (*NodeStmtWhile) IsStatement()        {}
func (*NodeLlvm) IsStatement()             {}
func (*NodeStmtDefer) IsStatement()        {}
func (*NodeExprVarDef) IsGlobalDecl()      {}
func (*NodeFuncDef) IsGlobalDecl()         {}
func (*NodeStructDef) IsGlobalDecl()       {}
func (*NodeLlvm) IsGlobalDecl()            {}
func (*NodeConstDef) IsGlobalDecl()        {}
