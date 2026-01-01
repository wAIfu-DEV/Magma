package checker

import (
	t "Magma/src/types"
	"fmt"
	"strings"
)

func makeNamedType(name string) *t.NodeType {
	return &t.NodeType{
		Throws: false,
		KindNode: &t.NodeTypeNamed{
			NameNode: &t.NodeNameSingle{Name: name},
		},
	}
}

func makePtrType(from *t.NodeType) *t.NodeType {
	cpy := *from

	var kind t.NodeTypeKind

	switch n := cpy.KindNode.(type) {
	case *t.NodeTypeNamed:
		kind = &t.NodeTypePointer{
			Kind: n,
		}
	}

	return &t.NodeType{
		Throws:   cpy.Throws,
		KindNode: kind,
	}
}

func isVoidType(node *t.NodeType) bool {
	if node == nil {
		return false
	}

	switch n := node.KindNode.(type) {
	case *t.NodeTypeNamed:
		switch nn := n.NameNode.(type) {
		case *t.NodeNameSingle:
			return nn.Name == "void"
		}
	}
	return false
}

func isErrType(node *t.NodeType) bool {
	if node == nil {
		return false
	}

	switch n := node.KindNode.(type) {
	case *t.NodeTypeNamed:
		switch nn := n.NameNode.(type) {
		case *t.NodeNameSingle:
			return nn.Name == "error"
		}
	}
	return false
}

func isBoolType(node *t.NodeType) bool {
	if node == nil {
		return false
	}

	switch n := node.KindNode.(type) {
	case *t.NodeTypeNamed:
		switch nn := n.NameNode.(type) {
		case *t.NodeNameSingle:
			return nn.Name == "bool"
		}
	}
	return false
}

func isArrayType(node *t.NodeType) bool {
	if node == nil {
		return false
	}

	switch node.KindNode.(type) {
	case *t.NodeTypeNamed:
		return false
	case *t.NodeTypeRfc:
		return false
	case *t.NodeTypePointer:
		return true
	case *t.NodeTypeSlice:
		return true
	}
	return false
}

func getBoxedType(node *t.NodeType) *t.NodeType {
	if node == nil {
		return nil
	}

	switch n := node.KindNode.(type) {
	case *t.NodeTypeNamed:
		return nil
	case *t.NodeTypeRfc:
		return &t.NodeType{
			Throws:   node.Throws,
			KindNode: n.Kind,
		}
	case *t.NodeTypePointer:
		return &t.NodeType{
			Throws:   node.Throws,
			KindNode: n.Kind,
		}
	case *t.NodeTypeSlice:
		return &t.NodeType{
			Throws:   node.Throws,
			KindNode: n.ElemKind,
		}
	}
	return nil
}

func flattenTypeKind(nodeKind t.NodeTypeKind) string {
	switch n := nodeKind.(type) {
	case *t.NodeTypeNamed:
		switch nn := n.NameNode.(type) {
		case *t.NodeNameSingle:
			return nn.Name
		case *t.NodeNameComposite:
			return strings.Join(nn.Parts, ".")
		}
	case *t.NodeTypePointer:
		return "ptr<" + flattenTypeKind(n.Kind) + ">"
	}
	return "undef"
}

func flattenType(node *t.NodeType) string {
	if node == nil {
		return "nil"
	}

	return flattenTypeKind(node.KindNode)
}

func ctExpr(c *ctx, expr t.NodeExpr) error {
	switch n := expr.(type) {
	case *t.NodeExprVoid:
		n.VoidType = makeNamedType("void")
		return nil
	case *t.NodeExprCall:
		for _, a := range n.Args {
			e := ctExpr(c, a)
			if e != nil {
				return e
			}
		}

		e := ctExpr(c, n.Callee)
		if e != nil {
			return e
		}

		if n.IsFuncPointer {
			n.InfType = n.FuncPtrType.KindNode.(*t.NodeTypeFunc).RetType
		} else {
			n.InfType = n.AssociatedFnDef.ReturnType
		}
		return nil
	case *t.NodeExprSubscript:
		e := ctExpr(c, n.Expr)
		if e != nil {
			return e
		}
		e = ctExpr(c, n.Target)
		if e != nil {
			return e
		}

		switch n2 := n.AssociatedNode.(type) {
		case *t.NodeExprVarDef:
			n.BoxType = n2.GetInferredType()
		case *t.NodeExprVarDefAssign:
			n.BoxType = n2.GetInferredType()
		default:
			return fmt.Errorf("cannot subscript on type other than pointer or slice")
		}

		var elemType *t.NodeType = getBoxedType(n.BoxType)
		if elemType == nil {
			return fmt.Errorf("failed to infer array element type")
		}
		n.ElemType = elemType
		return nil
	case *t.NodeExprLit:
		switch n.LitType {
		case t.TokLitNum:
			n.InfType = makeNamedType("i64")
			return nil
		case t.TokLitStr:
			n.InfType = makeNamedType("str")
			return nil
		case t.TokLitBool:
			n.InfType = makeNamedType("bool")
			return nil
		}
	case *t.NodeExprName:
		if n.AssociatedNode == nil {
			fmt.Printf("name: %s\n", flattenName(n.Name))
			return fmt.Errorf("name node pointing to no valid node")
		}

		// TODO: following is not correct for member accesses
		if len(n.MemberAccesses) > 0 {
			last := n.MemberAccesses[len(n.MemberAccesses)-1]
			n.InfType = last.Type
			return nil
		}

		switch n2 := n.AssociatedNode.(type) {
		case *t.NodeExprVarDef:
			n.InfType = n2.GetInferredType()
		case *t.NodeExprVarDefAssign:
			n.InfType = n2.GetInferredType()
		case *t.NodeFuncDef:
			// TODO: build function type
			n.InfType = n2.ReturnType
		default:
			return fmt.Errorf("name node pointing to invalid node type, failed to infer type")
		}
		return nil
	case *t.NodeExprBinary:
		e := ctExpr(c, n.Left)
		if e != nil {
			return e
		}

		e = ctExpr(c, n.Right)
		if e != nil {
			return e
		}

		switch n.Operator {
		case t.KwCmpEq, t.KwCmpNeq:
			n.InfType = makeNamedType("bool")
		default:
			// TODO: implicit casting rules
			n.InfType = n.Left.GetInferredType()
		}
		return nil
	case *t.NodeExprUnary:
		switch n.Operator {
		case t.KwAsterisk:
			e := ctExpr(c, n.Operand)
			if e != nil {
				return e
			}
			n.InfType = makePtrType(n.Operand.GetInferredType())
			return nil
		}
		return fmt.Errorf("unexpected unary expression type")
	case *t.NodeExprVarDef:
		return nil
	case *t.NodeExprVarDefAssign:
		e := ctExpr(c, n.AssignExpr)
		if e != nil {
			return e
		}
		return nil
	case *t.NodeExprAssign:
		e := ctExpr(c, n.Left)
		if e != nil {
			return e
		}
		e = ctExpr(c, n.Right)
		if e != nil {
			return e
		}
		n.InfType = n.Left.GetInferredType()
		return nil
	case *t.NodeExprTry:
		e := ctExpr(c, n.Call)
		if e != nil {
			return e
		}
		return nil
	}
	return fmt.Errorf("unexpected expression type")
}

func ctIfStmt(c *ctx, ifStmt *t.NodeStmtIf) error {
	e := ctExpr(c, ifStmt.CondExpr)
	if e != nil {
		return e
	}

	infType := ifStmt.CondExpr.GetInferredType()
	isBool := isBoolType(infType)

	if !isBool {
		fmt.Printf("inferred type: %s\n", flattenType(infType))
		return fmt.Errorf("type of expression in if statement must be of type 'bool'")
	}

	e = ctBody(c, &ifStmt.Body)
	if e != nil {
		return e
	}

	if ifStmt.NextCondStmt != nil {
		switch n := ifStmt.NextCondStmt.(type) {
		case *t.NodeStmtIf:
			e = ctIfStmt(c, n)
		case *t.NodeStmtElse:
			e = ctBody(c, &n.Body)
		}
		if e != nil {
			return e
		}
	}

	return nil
}

func ctWhileStmt(c *ctx, whileStmt *t.NodeStmtWhile) error {
	e := ctExpr(c, whileStmt.CondExpr)
	if e != nil {
		return e
	}

	infType := whileStmt.CondExpr.GetInferredType()
	isBool := isBoolType(infType)

	if !isBool {
		fmt.Printf("inferred type: %s\n", flattenType(infType))
		return fmt.Errorf("type of expression in if statement must be of type 'bool'")
	}

	e = ctBody(c, &whileStmt.Body)
	if e != nil {
		return e
	}
	return nil
}
func ctThrow(c *ctx, throw *t.NodeStmtThrow) error {
	e := ctExpr(c, throw.Expression)
	if e != nil {
		return e
	}

	infType := throw.Expression.GetInferredType()
	isErr := isErrType(infType)

	if !isErr {
		fmt.Printf("inferred type: %s\n", flattenType(infType))
		return fmt.Errorf("type of expression in throw statement must be of type 'error'")
	}

	return nil
}

func ctReturn(c *ctx, ret *t.NodeStmtRet) error {
	if c.LastFuncDef == nil {
		// TODO: compiler error
		return fmt.Errorf("return statement outside function")
	}

	if c.LastFuncDef.ReturnType == nil {
		// TODO: compiler error
		return fmt.Errorf("function return type is null when trying to infer ret type")
	}

	ret.OwnerFuncType = c.LastFuncDef.ReturnType

	e := ctExpr(c, ret.Expression)
	if e != nil {
		return e
	}

	retIsVoid := isVoidType(ret.OwnerFuncType)
	exprIsVoid := isVoidType(ret.Expression.GetInferredType())

	if retIsVoid != exprIsVoid {
		if retIsVoid {
			return fmt.Errorf("unexpected expression after 'ret' statment within func with return type 'void'")
		}
		return fmt.Errorf("missing expression after 'ret' statement in function with non-null return type")
	}

	// TODO: check if is same type
	return nil
}

func ctBody(c *ctx, bdy *t.NodeBody) error {
	for _, stmt := range bdy.Statements {
		var e error
		switch n := stmt.(type) {
		case *t.NodeStmtRet:
			e = ctReturn(c, n)
		case *t.NodeStmtExpr:
			e = ctExpr(c, n.Expression)
		case *t.NodeStmtThrow:
			e = ctThrow(c, n)
		case *t.NodeStmtIf:
			e = ctIfStmt(c, n)
		case *t.NodeStmtWhile:
			e = ctWhileStmt(c, n)
		}
		if e != nil {
			return e
		}
	}
	return nil
}

func ctFuncDef(c *ctx, fnDef *t.NodeFuncDef) error {
	c.LastFuncDef = fnDef

	e := ctBody(c, &fnDef.Body)
	if e != nil {
		return e
	}
	return nil
}

func ctGlDecl(c *ctx, glDecl t.NodeGlobalDecl) error {
	switch n := glDecl.(type) {
	case *t.NodeFuncDef:
		return ctFuncDef(c, n)
	case *t.NodeStructDef:
		return nil // TODO: check type names of arguments
	}
	return nil
}

func ctGlobal(c *ctx, gl *t.NodeGlobal) error {
	for _, dcl := range gl.Declarations {
		e := ctGlDecl(c, dcl)
		if e != nil {
			return e
		}
	}
	return nil
}

func TypeChecker(s *t.SharedState) error {
	ctx := &ctx{
		Shared: s,
	}

	for _, fCtx := range s.Files {
		n := fCtx.GlNode
		ctx.GlobalNode = n
		e := ctGlobal(ctx, n)
		if e != nil {
			return e
		}
	}

	return nil
}
