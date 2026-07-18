package checker

import (
	"Magma/src/comp_err"
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

func makeFuncPtrTypeFromDef(fnDef *t.NodeFuncDef) *t.NodeType {
	k := &t.NodeTypeFunc{
		Args:    []*t.NodeType{},
		RetType: fnDef.ReturnType,
	}

	for _, v := range fnDef.Class.ArgsNode.Args {
		k.Args = append(k.Args, v.TypeNode)
	}

	return &t.NodeType{
		Throws:   false,
		KindNode: k,
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

func isStrType(node *t.NodeType) bool {
	if node == nil {
		return false
	}
	named, ok := node.KindNode.(*t.NodeTypeNamed)
	if !ok {
		return false
	}
	single, ok := named.NameNode.(*t.NodeNameSingle)
	return ok && single.Name == "str"
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

func isUntypedSlice(node *t.NodeType) bool {
	if node == nil {
		return false
	}
	named, ok := node.KindNode.(*t.NodeTypeNamed)
	if !ok {
		return false
	}
	single, ok := named.NameNode.(*t.NodeNameSingle)
	return ok && single.Name == "slice"
}

func isTypedSlice(node *t.NodeType) bool {
	if node == nil {
		return false
	}
	_, ok := node.KindNode.(*t.NodeTypeSlice)
	return ok
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
		suffix := ""
		if len(n.GenericArgs) > 0 {
			parts := make([]string, 0, len(n.GenericArgs))
			for _, a := range n.GenericArgs {
				parts = append(parts, flattenType(a))
			}
			suffix = "[" + strings.Join(parts, ",") + "]"
		}
		switch nn := n.NameNode.(type) {
		case *t.NodeNameSingle:
			return nn.Name + suffix
		case *t.NodeNameComposite:
			return strings.Join(nn.Parts, ".") + suffix
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

func flattenCallee(expr t.NodeExpr) string {
	switch n := expr.(type) {
	case *t.NodeExprName:
		return flattenName(n.Name)
	case *t.NodeExprMemberAccess:
		return flattenCallee(n.Target) + "." + n.Member
	case *t.NodeExprCall:
		return flattenCallee(n.Callee) + "()"
	default:
		return "<expr>"
	}
}

func expressionSourceToken(expr t.NodeExpr) *t.Token {
	switch n := expr.(type) {
	case *t.NodeExprName:
		return &n.Tk
	case *t.NodeExprLit:
		return &n.Tk
	case *t.NodeExprUnary:
		return &n.Tk
	case *t.NodeExprBinary:
		return &n.Tk
	case *t.NodeExprCall:
		return &n.Tk
	case *t.NodeExprMemberAccess:
		return &n.Tk
	case *t.NodeExprSubscript:
		return &n.Tk
	case *t.NodeExprTry:
		return &n.Tk
	case *t.NodeExprSizeof:
		return &n.Tk
	case *t.NodeExprAddrof:
		return &n.Tk
	case *t.NodeExprAssign:
		return &n.Tk
	case *t.NodeExprVarDefAssign:
		return &n.Tk
	}
	return &t.Token{}
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

func compatibleInitializer(expected *t.NodeType, expr t.NodeExpr) bool {
	actual := expr.GetInferredType()
	if compatibleTypes(expected, actual) {
		return true
	}
	if lit, ok := expr.(*t.NodeExprLit); ok && lit.LitType == t.TokLitNum && isNumberType(expected) {
		return true
	}
	return false
}

func compatibleTypes(expected *t.NodeType, actual *t.NodeType) bool {
	if expected == nil || actual == nil || expected.Throws != actual.Throws {
		return false
	}
	if sameType(expected, actual) {
		return true
	}
	expectedFunc, expectedIsFunc := expected.KindNode.(*t.NodeTypeFunc)
	actualFunc, actualIsFunc := actual.KindNode.(*t.NodeTypeFunc)
	if expectedIsFunc || actualIsFunc {
		if (expectedIsFunc && isPointerType(actual)) || (actualIsFunc && isPointerType(expected)) {
			return true
		}
		if !expectedIsFunc || !actualIsFunc || len(expectedFunc.Args) != len(actualFunc.Args) {
			return false
		}
		for i := range expectedFunc.Args {
			if !compatibleTypes(expectedFunc.Args[i], actualFunc.Args[i]) {
				return false
			}
		}
		return compatibleTypes(expectedFunc.RetType, actualFunc.RetType)
	}
	if isPointerType(expected) && isPointerType(actual) {
		return true
	}
	if isNumberType(expected) && isNumberType(actual) {
		return true
	}
	expectedSlice, expectedIsSlice := expected.KindNode.(*t.NodeTypeSlice)
	actualSlice, actualIsSlice := actual.KindNode.(*t.NodeTypeSlice)
	if expectedIsSlice && actualIsSlice {
		if expectedSlice.HasSize && actualSlice.HasSize && expectedSlice.Size != actualSlice.Size {
			return false
		}
		return compatibleTypes(&t.NodeType{KindNode: expectedSlice.ElemKind}, &t.NodeType{KindNode: actualSlice.ElemKind})
	}
	if (isUntypedSlice(expected) && isTypedSlice(actual)) || (isTypedSlice(expected) && isUntypedSlice(actual)) {
		return true
	}
	return false
}

func sameTypeKind(a t.NodeTypeKind, b t.NodeTypeKind) bool {
	switch ta := a.(type) {
	case *t.NodeTypeNamed:
		tb, ok := b.(*t.NodeTypeNamed)
		if !ok {
			return false
		}
		if len(ta.GenericArgs) != len(tb.GenericArgs) {
			return false
		}
		for i := range ta.GenericArgs {
			if !sameType(ta.GenericArgs[i], tb.GenericArgs[i]) {
				return false
			}
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
	case *t.NodeTypeAbsolute:
		tb, ok := b.(*t.NodeTypeAbsolute)
		if !ok {
			return false
		}
		return ta.AbsoluteName == tb.AbsoluteName
	default:
		return false
	}
}

func ctExprLvalue(c *ctx, expr t.NodeExpr) error {
	if name, variable := assignedConstant(expr); variable != nil {
		return comp_err.CompilationErrorToken(c.FileCtx, lastNameToken(name.Name), fmt.Sprintf("cannot assign to constant '%s'", flattenName(name.Name)), "constants are immutable")
	}
	switch n := expr.(type) {
	case *t.NodeExprUnary:
		if n.Operator != t.KwAsterisk {
			return fmt.Errorf("unary expression is not assignable")
		}
		return ctExpr(c, n)
	case *t.NodeExprMemberAccess:
		return ctExpr(c, n)
	case *t.NodeExprSubscript:
		e := ctExpr(c, n.Expr)
		if e != nil {
			return e
		}
		e = ctExpr(c, n.Target)
		if e != nil {
			return e
		}

		n.BoxType = n.Target.GetInferredType()

		var elemType *t.NodeType = getBoxedType(n.BoxType)
		if elemType == nil {
			return comp_err.CompilationErrorToken(
				c.FileCtx,
				&n.Tk,
				fmt.Sprintf("cannot index value of type '%s'", flattenType(n.BoxType)),
				"only arrays, slices, and pointers can be indexed",
			)
		}

		n.ElemType = elemType

		//fmt.Printf("subscript:\n")
		//fmt.Printf(" type: %s\n", flattenType(n.BoxType))
		//fmt.Printf(" elemtype: %s\n", flattenType(n.ElemType))
		return nil
	case *t.NodeExprName:
		if n.AssociatedNode == nil {
			//fmt.Printf("name: %s\n", flattenName(n.Name))
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

		//fmt.Printf("name: %s\n", flattenName(n.Name))
		//fmt.Printf(" type: %s\n", flattenType(n.InfType))
		return nil
	}
	return fmt.Errorf("unexpected expression type")
}

func assignedConstant(expr t.NodeExpr) (*t.NodeExprName, *t.NodeExprVarDef) {
	switch n := expr.(type) {
	case *t.NodeExprName:
		if variable, ok := n.AssociatedNode.(*t.NodeExprVarDef); ok && variable.IsConst {
			return n, variable
		}
	case *t.NodeExprMemberAccess:
		return assignedConstant(n.Target)
	case *t.NodeExprSubscript:
		return assignedConstant(n.Target)
	}
	return nil, nil
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
		//fmt.Printf("call: %s\n", flattenCallee(n.Callee))

		// TODO: compare arg types

		callArgCount := len(n.Args)
		defArgCount := 0
		expectedArgs := []*t.NodeType{}

		if n.IsFuncPointer {
			if n.FuncPtrType == nil {
				return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, fmt.Sprintf("cannot call '%s': its type could not be resolved", flattenCallee(n.Callee)), "")
			}
			funcType, ok := n.FuncPtrType.KindNode.(*t.NodeTypeFunc)
			if !ok {
				return comp_err.CompilationErrorToken(
					c.FileCtx,
					&n.Tk,
					fmt.Sprintf("cannot call '%s': value has non-function type '%s'", flattenCallee(n.Callee), flattenType(n.FuncPtrType)),
					"only functions and function-pointer values can be called",
				)
			}
			defArgCount = len(funcType.Args)
			expectedArgs = funcType.Args
		} else {
			definedArgs := n.AssociatedFnDef.Class.ArgsNode.Args
			defArgCount = len(definedArgs)

			if defArgCount > 0 {
				firstArg := definedArgs[0]
				if firstArg.Name == "this" {
					definedArgs = definedArgs[1:]
					defArgCount -= 1
				}
			}
			for _, arg := range definedArgs {
				expectedArgs = append(expectedArgs, arg.TypeNode)
			}
		}

		if callArgCount != defArgCount {
			return comp_err.CompilationErrorToken(
				c.FileCtx,
				&n.Tk,
				fmt.Sprintf("function '%s' expects %d argument(s), but got %d", flattenCallee(n.Callee), defArgCount, callArgCount),
				"",
			)
		}

		for i, a := range n.Args {
			e := ctExpr(c, a)
			if e != nil {
				return e
			}
			if !compatibleInitializer(expectedArgs[i], a) {
				return comp_err.CompilationErrorToken(
					c.FileCtx,
					expressionSourceToken(a),
					fmt.Sprintf("argument %d to '%s' expects type '%s', but got '%s'", i+1, flattenCallee(n.Callee), flattenType(expectedArgs[i]), flattenType(a.GetInferredType())),
					"",
				)
			}
		}

		if !n.IsMemberFunc {
			e := ctExpr(c, n.Callee)
			if e != nil {
				return e
			}
		}

		//fmt.Printf("is ptr to func: %t\n", n.IsFuncPointer)
		if n.IsFuncPointer {
			//fmt.Printf("func type: %s\n", flattenType(n.FuncPtrType))
		}

		if n.IsFuncPointer {
			n.InfType = n.FuncPtrType.KindNode.(*t.NodeTypeFunc).RetType
		} else {
			n.InfType = n.AssociatedFnDef.ReturnType
		}
		return nil
	case *t.NodeExprStructInit:
		for i := range n.Fields {
			field := &n.Fields[i]
			if field.FieldType == nil {
				return fmt.Errorf("constructor field '%s' was not resolved", field.Name)
			}
			if e := ctExpr(c, field.Expression); e != nil {
				return e
			}
			if !compatibleInitializer(field.FieldType, field.Expression) {
				return comp_err.CompilationErrorToken(c.FileCtx, &field.Tk, fmt.Sprintf("field '%s' expects type '%s', but initializer has type '%s'", field.Name, flattenType(field.FieldType), flattenType(field.Expression.GetInferredType())), "")
			}
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

		n.BoxType = n.Target.GetInferredType()

		var elemType *t.NodeType = getBoxedType(n.BoxType)
		if elemType == nil {
			return comp_err.CompilationErrorToken(
				c.FileCtx,
				&n.Tk,
				fmt.Sprintf("cannot index value of type '%s'", flattenType(n.BoxType)),
				"only arrays, slices, and pointers can be indexed",
			)
		}

		n.ElemType = elemType

		//fmt.Printf("subscript:\n")
		//fmt.Printf(" type: %s\n", flattenType(n.BoxType))
		//fmt.Printf(" elemtype: %s\n", flattenType(n.ElemType))
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
		case t.TokLitNone:
			n.InfType = makeNamedType("ptr")
			return nil
		}
	case *t.NodeExprName:
		if n.AssociatedNode == nil {
			//fmt.Printf("name: %s\n", flattenName(n.Name))
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
			n.InfType = makeFuncPtrTypeFromDef(n2)
		default:
			return fmt.Errorf("name node pointing to invalid node type, failed to infer type")
		}

		//fmt.Printf("name: %s\n", flattenName(n.Name))
		//fmt.Printf(" type: %s\n", flattenType(n.InfType))
		return nil
	case *t.NodeExprMemberAccess:
		e := ctExpr(c, n.Target)
		if e != nil {
			return e
		}
		if n.InfType == nil && n.Access != nil {
			n.InfType = n.Access.Type
		}
		if n.InfType == nil {
			return fmt.Errorf("member access '%s' has no inferred type", n.Member)
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
			leftT := n.Left.GetInferredType()
			rightT := n.Right.GetInferredType()
			if !compatibleTypes(leftT, rightT) {
				return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, fmt.Sprintf("cannot compare values of unrelated types '%s' and '%s'", flattenType(leftT), flattenType(rightT)), "")
			}
			n.InfType = makeNamedType("bool")
		case t.KwCmpLt, t.KwCmpGt, t.KwCmpLtEq, t.KwCmpGtEq:
			leftT := n.Left.GetInferredType()
			rightT := n.Right.GetInferredType()
			if !isNumberType(leftT) || !isNumberType(rightT) {
				return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, fmt.Sprintf("ordering comparison requires numeric operands, but got '%s' and '%s'", flattenType(leftT), flattenType(rightT)), "")
			}
			n.InfType = makeNamedType("bool")
		case t.KwAndAnd, t.KwOrOr:
			leftT := n.Left.GetInferredType()
			rightT := n.Right.GetInferredType()

			if !isBoolType(leftT) || !isBoolType(rightT) {
				return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, fmt.Sprintf("logical operator '%s' requires 'bool' operands, but got '%s' and '%s'", t.KwTypeToRepr[n.Operator], flattenType(leftT), flattenType(rightT)), "")
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
				// TODO: compilation error
				//("lType: %s\n", flattenType(leftT))
				//fmt.Printf("rType: %s\n", flattenType(rightT))
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
		case t.KwPlus, t.KwMinus, t.KwAsterisk, t.KwSlash, t.KwPercent:
			leftT := n.Left.GetInferredType()
			rightT := n.Right.GetInferredType()
			if !isNumberType(leftT) || !isNumberType(rightT) {
				return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, fmt.Sprintf("arithmetic operator '%s' requires numeric operands, but got '%s' and '%s'", t.KwTypeToRepr[n.Operator], flattenType(leftT), flattenType(rightT)), "")
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
			operandType := n.Operand.GetInferredType()
			pointerType, ok := operandType.KindNode.(*t.NodeTypePointer)
			if !ok {
				return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, fmt.Sprintf("cannot dereference value of non-pointer type '%s'", flattenType(operandType)), "")
			}
			n.InfType = &t.NodeType{
				Throws:   false,
				KindNode: pointerType.Kind,
			}
			return nil
		case t.KwTilde:
			operandT := n.Operand.GetInferredType()
			if isBoolType(operandT) {
				n.InfType = makeNamedType("bool")
				return nil
			}
			if !isIntegerType(operandT) {
				return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, fmt.Sprintf("bitwise not requires an integer or 'bool' operand, but got '%s'", flattenType(operandT)), "")
			}
			n.InfType = operandT
			return nil
		default:
			operator := t.KwTypeToRepr[n.Operator]
			additional := "this unary operator is not supported"
			if n.Operator == t.KwAmpersand {
				additional = "use `addrof expression` to take an address"
			}
			return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, fmt.Sprintf("unary operator '%s' is not supported", operator), additional)
		}
	case *t.NodeExprVarDef:
		if n.Type == nil {
			return fmt.Errorf("unassigned var def expr cannot have nil type")
		}
		return nil
	case *t.NodeExprVarDefAssign:
		e := ctExpr(c, n.AssignExpr)
		if e != nil {
			return e
		}

		if n.VarDef.Type == nil {
			n.VarDef.Type = n.AssignExpr.GetInferredType()
		} else if !compatibleInitializer(n.VarDef.Type, n.AssignExpr) {
			return comp_err.CompilationErrorToken(
				c.FileCtx,
				&n.Tk,
				fmt.Sprintf("cannot initialize value of type '%s' with expression of type '%s'", flattenType(n.VarDef.Type), flattenType(n.AssignExpr.GetInferredType())),
				"",
			)
		}

		return nil
	case *t.NodeExprAssign:
		e := ctExprLvalue(c, n.Left)
		if e != nil {
			return e
		}
		e = ctExpr(c, n.Right)
		if e != nil {
			return e
		}
		n.InfType = n.Left.GetInferredType()
		if !compatibleInitializer(n.InfType, n.Right) {
			return comp_err.CompilationErrorToken(
				c.FileCtx,
				&n.Tk,
				fmt.Sprintf("cannot assign value of type '%s' to value of type '%s'", flattenType(n.Right.GetInferredType()), flattenType(n.InfType)),
				"",
			)
		}
		return nil
	case *t.NodeExprTry:
		e := ctExpr(c, n.Call)
		if e != nil {
			return e
		}
		callType := n.Call.GetInferredType()
		if callType == nil || !callType.Throws {
			return comp_err.CompilationErrorToken(
				c.FileCtx,
				&n.Tk,
				fmt.Sprintf("cannot use 'try' with non-throwing call '%s'", flattenCallee(n.Call)),
				"remove 'try' or call a function whose return type is marked with '!'",
			)
		}
		unwrapped := *callType
		unwrapped.Throws = false
		n.InfType = &unwrapped
		return nil
	case *t.NodeExprDestructureAssign:
		e := ctExpr(c, n.Call)
		if e != nil {
			return e
		}

		if n.Call.InfType == nil {
			return comp_err.CompilationErrorToken(c.FileCtx, &n.Call.Tk, "cannot determine the return type for destructuring assignment", "")
		}

		if !n.Call.InfType.Throws {
			return comp_err.CompilationErrorToken(c.FileCtx, &n.Call.Tk, fmt.Sprintf("cannot destructure non-throwing call '%s'", flattenCallee(n.Call.Callee)), "destructuring requires a call whose return type is marked with '!'")
		}

		if isVoidType(n.Call.InfType) {
			return comp_err.CompilationErrorToken(c.FileCtx, &n.Call.Tk, fmt.Sprintf("cannot bind a result value from throwing void call '%s'", flattenCallee(n.Call.Callee)), "a '!void' call only produces an error result")
		}

		if n.ErrDef.Type == nil {
			n.ErrDef.Type = makeNamedType("error")
		}
		if !isErrType(n.ErrDef.Type) || n.ErrDef.Type.Throws {
			return comp_err.CompilationErrorToken(c.FileCtx, &n.Call.Tk, fmt.Sprintf("destructuring error binding must have type 'error', but got '%s'", flattenType(n.ErrDef.Type)), "")
		}

		unwrappedValue := *n.Call.InfType
		unwrappedValue.Throws = false
		unwrapped := &unwrappedValue

		if n.ValueDef.Type == nil {
			n.ValueDef.Type = unwrapped
		}

		if !sameType(unwrapped, n.ValueDef.Type) {
			return comp_err.CompilationErrorToken(c.FileCtx, &n.Call.Tk, fmt.Sprintf("destructuring value binding expects type '%s', but call returns '%s'", flattenType(n.ValueDef.Type), flattenType(unwrapped)), "")
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
		return comp_err.CompilationErrorToken(c.FileCtx, &ifStmt.Tk, fmt.Sprintf("if condition must have type 'bool', but got '%s'", flattenType(infType)), "")
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
		return comp_err.CompilationErrorToken(c.FileCtx, &whileStmt.Tk, fmt.Sprintf("while condition must have type 'bool', but got '%s'", flattenType(infType)), "")
	}

	c.LoopDepth++
	e = ctBody(c, &whileStmt.Body)
	c.LoopDepth--
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

	if !isErr && !isStrType(infType) {
		return comp_err.CompilationErrorToken(c.FileCtx, &throw.Tk, fmt.Sprintf("cannot throw value of type '%s'; expected 'error' or 'str'", flattenType(infType)), "")
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
			return comp_err.CompilationErrorToken(c.FileCtx, &ret.Tk, "cannot return a value from a function returning 'void'", "use a bare 'ret' statement")
		}
		return comp_err.CompilationErrorToken(c.FileCtx, &ret.Tk, fmt.Sprintf("missing return value in function returning '%s'", flattenType(ret.OwnerFuncType)), "provide a value after 'ret'")
	}

	expectedValue := *ret.OwnerFuncType
	expectedValue.Throws = false
	if !compatibleInitializer(&expectedValue, ret.Expression) {
		return comp_err.CompilationErrorToken(
			c.FileCtx,
			&ret.Tk,
			fmt.Sprintf("cannot return value of type '%s' from function returning '%s'", flattenType(ret.Expression.GetInferredType()), flattenType(&expectedValue)),
			"",
		)
	}
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
		case *t.NodeStmtBreak:
			if c.LoopDepth == 0 {
				e = comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, "cannot use 'break' outside a loop", "place 'break' inside a while loop")
			}
		case *t.NodeStmtContinue:
			if c.LoopDepth == 0 {
				e = comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, "cannot use 'continue' outside a loop", "place 'continue' inside a while loop")
			}
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

func isSimpleConstInitializer(expr t.NodeExpr) bool {
	switch n := expr.(type) {
	case *t.NodeExprLit:
		return true
	case *t.NodeExprName:
		switch associated := n.AssociatedNode.(type) {
		case *t.NodeFuncDef:
			return true
		case *t.NodeExprVarDef:
			return associated.IsConst && associated.Initializer != nil
		}
		return false
	case *t.NodeExprStructInit:
		for _, field := range n.Fields {
			if !isSimpleConstInitializer(field.Expression) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func ctGlDecl(c *ctx, glDecl t.NodeGlobalDecl) error {
	switch n := glDecl.(type) {
	case *t.NodeFuncDef:
		return ctFuncDef(c, n)
	case *t.NodeExprVarDef:
		if n.Initializer != nil {
			if e := ctExpr(c, n.Initializer); e != nil {
				return e
			}
			if n.Type == nil {
				n.Type = n.Initializer.GetInferredType()
			}
			if !compatibleInitializer(n.Type, n.Initializer) {
				return comp_err.CompilationErrorToken(c.FileCtx, &t.Token{}, fmt.Sprintf("cannot initialize global '%s' of type '%s' with expression of type '%s'", flattenName(n.Name), flattenType(n.Type), flattenType(n.Initializer.GetInferredType())), "")
			}
		}
		return ctExpr(c, n)
	case *t.NodeStructDef:
		return nil // TODO: check type names of arguments
	case *t.NodeConstDef:
		if !isSimpleConstInitializer(n.Initializer) {
			return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, "constant initializer must be a literal, constant value, function value, or struct constructor", "general constant expressions are not supported")
		}
		if e := ctExpr(c, n.Initializer); e != nil {
			return e
		}
		if n.VarDef.Type == nil {
			n.VarDef.Type = n.Initializer.GetInferredType()
			return nil
		}
		if !compatibleInitializer(n.VarDef.Type, n.Initializer) {
			return fmt.Errorf("constant %s expects %s but initializer has type %s", flattenName(n.VarDef.Name), flattenType(n.VarDef.Type), flattenType(n.Initializer.GetInferredType()))
		}
		return nil
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
		// fmt.Printf("check types of: %s\n", fCtx.PackageName)

		n := fCtx.GlNode
		ctx.GlobalNode = n
		ctx.FileCtx = fCtx
		e := ctGlobal(ctx, n)
		if e != nil {
			return e
		}
	}

	return nil
}
