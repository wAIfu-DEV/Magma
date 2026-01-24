package checker

import (
	magmatypes "Magma/src/magma_types"
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

func isPointerType(node *t.NodeType) bool {
	if node == nil {
		return false
	}

	switch n := node.KindNode.(type) {
	case *t.NodeTypePointer:
		return true
	case *t.NodeTypeRfc:
		return true
	case *t.NodeTypeNamed:
		switch nn := n.NameNode.(type) {
		case *t.NodeNameSingle:
			return nn.Name == "ptr"
		}
	}
	return false
}

func isNumberType(node *t.NodeType) bool {
	if node == nil {
		return false
	}

	switch n := node.KindNode.(type) {
	case *t.NodeTypeNamed:
		switch nn := n.NameNode.(type) {
		case *t.NodeNameSingle:
			_, ok := magmatypes.NumberTypes[nn.Name]
			return ok
		}
	}
	return false
}

func isFloatType(node *t.NodeType) bool {
	if node == nil {
		return false
	}

	switch n := node.KindNode.(type) {
	case *t.NodeTypeNamed:
		switch nn := n.NameNode.(type) {
		case *t.NodeNameSingle:
			desc, ok := magmatypes.NumberTypes[nn.Name]
			return ok && desc.IsFloat
		}
	}
	return false
}

func isIntegerType(node *t.NodeType) bool {
	if node == nil || !isNumberType(node) {
		return false
	}
	if isFloatType(node) {
		return false
	}
	if isPointerType(node) {
		return false
	}
	return true
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
	case *t.NodeTypeSlice:
		return flattenTypeKind(n.ElemKind) + "[]"
	case *t.NodeTypeFunc:
		return flattenTypeKind(n.RetType.KindNode) + "()"
	case *t.NodeTypeAbsolute:
		return n.AbsoluteName
	}
	return "undef"
}

func flattenType(node *t.NodeType) string {
	if node == nil {
		return "nil"
	}

	return flattenTypeKind(node.KindNode)
}

func sameType(a *t.NodeType, b *t.NodeType) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Throws != b.Throws {
		return false
	}
	return sameTypeKind(a.KindNode, b.KindNode)
}

func sameTypeKind(a t.NodeTypeKind, b t.NodeTypeKind) bool {
	switch ta := a.(type) {
	case *t.NodeTypeNamed:
		tb, ok := b.(*t.NodeTypeNamed)
		if !ok {
			return false
		}
		return flattenName(ta.NameNode) == flattenName(tb.NameNode)
	case *t.NodeTypePointer:
		tb, ok := b.(*t.NodeTypePointer)
		if !ok {
			return false
		}
		return sameTypeKind(ta.Kind, tb.Kind)
	case *t.NodeTypeRfc:
		tb, ok := b.(*t.NodeTypeRfc)
		if !ok {
			return false
		}
		return sameTypeKind(ta.Kind, tb.Kind)
	case *t.NodeTypeSlice:
		tb, ok := b.(*t.NodeTypeSlice)
		if !ok {
			return false
		}
		if ta.HasSize != tb.HasSize || ta.Size != tb.Size {
			return false
		}
		return sameTypeKind(ta.ElemKind, tb.ElemKind)
	case *t.NodeTypeFunc:
		tb, ok := b.(*t.NodeTypeFunc)
		if !ok {
			return false
		}
		if len(ta.Args) != len(tb.Args) {
			return false
		}
		for i := range ta.Args {
			if !sameType(ta.Args[i], tb.Args[i]) {
				return false
			}
		}
		return sameType(ta.RetType, tb.RetType)
	default:
		return false
	}
}

func ctExpr(c *ctx, expr t.NodeExpr) error {
	switch n := expr.(type) {
	case *t.NodeExprVoid:
		n.VoidType = makeNamedType("void")
		return nil
	case *t.NodeExprSizeof:
		n.InfType = makeNamedType("u64")
		return nil
	case *t.NodeExprAddrof:
		n.InfType = makeNamedType("ptr")
		return nil
	case *t.NodeExprCall:
		fmt.Printf("call: %s\n", flattenName(n.Callee.(*t.NodeExprName).Name))

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

		fmt.Printf("is ptr to func: %t\n", n.IsFuncPointer)
		fmt.Printf("func type: %s\n", flattenType(n.FuncPtrType))

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

		fmt.Printf("subscript:\n")
		fmt.Printf(" type: %s\n", flattenType(n.BoxType))
		fmt.Printf(" elemtype: %s\n", flattenType(n.ElemType))
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

		fmt.Printf("name: %s\n", flattenName(n.Name))
		fmt.Printf(" type: %s\n", flattenType(n.InfType))
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
		case t.KwCmpEq, t.KwCmpNeq, t.KwCmpLt, t.KwCmpGt, t.KwCmpLtEq, t.KwCmpGtEq:
			n.InfType = makeNamedType("bool")
		case t.KwAndAnd, t.KwOrOr:
			leftT := n.Left.GetInferredType()
			rightT := n.Right.GetInferredType()

			if !isBoolType(leftT) || !isBoolType(rightT) {
				return fmt.Errorf("logical operators require both operands to be bool")
			}

			n.InfType = makeNamedType("bool")
			return nil
		case t.KwAmpersand, t.KwPipe, t.KwCaret:
			leftT := n.Left.GetInferredType()
			rightT := n.Right.GetInferredType()

			if isBoolType(leftT) || isBoolType(rightT) {
				if !isBoolType(leftT) || !isBoolType(rightT) {
					return fmt.Errorf("bitwise operators on bool require both operands to be bool")
				}
				n.InfType = makeNamedType("bool")
				return nil
			}

			if !isIntegerType(leftT) || !isIntegerType(rightT) {
				fmt.Printf("lType: %s\n", flattenType(leftT))
				fmt.Printf("rType: %s\n", flattenType(rightT))
				return fmt.Errorf("bitwise operators require integer operands. operator: %s", t.KwTypeToRepr[n.Operator])
			}
			n.InfType = leftT
			return nil
		case t.KwShiftLeft, t.KwShiftRight:
			leftT := n.Left.GetInferredType()
			rightT := n.Right.GetInferredType()
			if !isIntegerType(leftT) || !isIntegerType(rightT) {
				return fmt.Errorf("shift operators require integer operands")
			}
			n.InfType = leftT
			return nil
		default:
			// TODO: implicit casting rules
			n.InfType = n.Left.GetInferredType()
		}
		return nil
	case *t.NodeExprUnary:
		e := ctExpr(c, n.Operand)
		if e != nil {
			return e
		}

		switch n.Operator {
		case t.KwAsterisk:
			n.InfType = makePtrType(n.Operand.GetInferredType())
			return nil
		case t.KwTilde:
			operandT := n.Operand.GetInferredType()
			if isBoolType(operandT) {
				n.InfType = makeNamedType("bool")
				return nil
			}
			if !isIntegerType(operandT) {
				return fmt.Errorf("bitwise not (~) requires an integer or bool operand")
			}
			n.InfType = operandT
			return nil
		default:
			return fmt.Errorf("unexpected unary expression type")
		}
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
	case *t.NodeExprDestructureAssign:
		e := ctExpr(c, n.Call)
		if e != nil {
			return e
		}

		if n.Call.InfType == nil {
			return fmt.Errorf("destructuring assignment: call has null inferred type")
		}

		if !n.Call.InfType.Throws {
			return fmt.Errorf("destructuring assignment requires a throwing call (return type must be !T)")
		}

		if isVoidType(n.Call.InfType) {
			return fmt.Errorf("destructuring assignment does not support !void calls (no value to bind)")
		}

		if n.ErrDef.Type == nil || !isErrType(n.ErrDef.Type) || n.ErrDef.Type.Throws {
			return fmt.Errorf("destructuring assignment: error binding must be of type 'error'")
		}

		unwrapped := &t.NodeType{
			Throws:   false,
			KindNode: n.Call.InfType.KindNode,
		}

		if !sameType(unwrapped, n.ValueDef.Type) {
			return fmt.Errorf(
				"destructuring assignment: value type mismatch (expected %s but call returns %s)",
				flattenType(n.ValueDef.Type),
				flattenType(unwrapped),
			)
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

func ctDefer(c *ctx, def *t.NodeStmtDefer) error {
	if def.IsBody {
		return ctBody(c, &def.Body)
	} else {
		return ctExpr(c, def.Expression)
	}
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
		case *t.NodeStmtDefer:
			e = ctDefer(c, n)
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
	case *t.NodeExprVarDef:
		return ctExpr(c, n)
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
		fmt.Printf("check types of: %s\n", fCtx.PackageName)

		n := fCtx.GlNode
		ctx.GlobalNode = n
		e := ctGlobal(ctx, n)
		if e != nil {
			return e
		}
	}

	return nil
}
