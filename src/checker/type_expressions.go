package checker

import (
	"Magma/src/comp_err"
	t "Magma/src/types"
	"fmt"
)

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
		return ctExprSubscript(c, n)
	case *t.NodeExprName:
		if n.AssociatedNode == nil {
			//fmt.Printf("name: %s\n", flattenName(n.Name))
			return fmt.Errorf("name node pointing to no valid node")
		}

		if len(n.MemberAccesses) > 0 {
			result, err := memberPathResult(n)
			if err != nil {
				return err
			}
			n.InfType = result
			return nil
		}

		switch n2 := n.AssociatedNode.(type) {
		case *t.NodeExprVarDef:
			n.InfType = n2.GetInferredType()
		case *t.NodeExprVarDefAssign:
			n.InfType = n2.GetInferredType()
		case *t.NodeFuncDef:
			return comp_err.CompilationErrorToken(
				c.FileCtx,
				&n.Tk,
				fmt.Sprintf("cannot assign to function '%s'", flattenName(n.Name)),
				"functions are values, but function declarations are immutable",
			)
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

func memberPathResult(name *t.NodeExprName) (*t.NodeType, error) {
	var current *t.NodeType
	switch definition := name.AssociatedNode.(type) {
	case *t.NodeExprVarDef:
		current = definition.Type
	case *t.NodeExprVarDefAssign:
		if definition.VarDef != nil {
			current = definition.VarDef.Type
		}
	}
	if current == nil {
		return nil, fmt.Errorf("member path has no resolved root type")
	}
	for _, access := range name.MemberAccesses {
		if access == nil || access.OwnerType == nil || access.Type == nil {
			return nil, fmt.Errorf("member path contains incomplete type metadata")
		}
		if !sameType(current, access.OwnerType) {
			return nil, fmt.Errorf("member path owner type does not match the preceding expression")
		}
		_, pointerOwner := current.KindNode.(*t.NodeTypePointer)
		if access.PtrDeref != pointerOwner {
			return nil, fmt.Errorf("member path contains inconsistent pointer-dereference metadata")
		}
		current = access.Type
	}
	return current, nil
}

func ctExpr(c *ctx, expr t.NodeExpr) error {
	return ctExprWithUsage(c, expr, true)
}

// ctExprWithUsage type-checks an expression and controls whether the value of
// a top-level call is consumed by its parent. A throwing call has no defined
// result on its error path, so only an ignored standalone call, try, or a
// destructuring assignment may inspect it without first unwrapping it.
func ctExprWithUsage(c *ctx, expr t.NodeExpr, valueUsed bool) error {
	switch n := expr.(type) {
	case *t.NodeExprVoid:
		n.VoidType = makeNamedType("void")
		return nil
	case *t.NodeExprSizeof:
		n.InfType = makeNamedType("u64")
		return nil
	case *t.NodeExprArray:
		if e := ctExpr(c, n.Length); e != nil {
			return e
		}
		if !isIntegerType(n.Length.GetInferredType()) {
			return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, "array length must be an integer", "expected: `array Type[integer-expression]`")
		}
		n.LengthType = makeNamedType("u64")
		if len(n.Entries) != 0 {
			length, ok := constArrayIndex(n.Length)
			if !ok {
				return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, "initialized array length must be a compile-time integer constant", "")
			}
			used := map[uint64]bool{}
			cursor := uint64(0)
			for i := range n.Entries {
				entry := &n.Entries[i]
				index := cursor
				if entry.Index != nil {
					if e := ctExpr(c, entry.Index); e != nil {
						return e
					}
					var valid bool
					index, valid = constArrayIndex(entry.Index)
					if !valid {
						return comp_err.CompilationErrorToken(c.FileCtx, &entry.Tk, "array initializer index must be a compile-time non-negative integer constant", "")
					}
				}
				if index >= length {
					return comp_err.CompilationErrorToken(c.FileCtx, &entry.Tk, fmt.Sprintf("array initializer index %d is out of bounds for length %d", index, length), "")
				}
				if used[index] {
					return comp_err.CompilationErrorToken(c.FileCtx, &entry.Tk, fmt.Sprintf("array initializer index %d is initialized more than once", index), "index overlap is not allowed")
				}
				if e := ctExpr(c, entry.Value); e != nil {
					return e
				}
				if !compatibleInitializer(c, n.ElemType, entry.Value) {
					return comp_err.CompilationErrorToken(c.FileCtx, &entry.Tk, fmt.Sprintf("array element expects type '%s', but initializer has type '%s'", flattenType(n.ElemType), flattenType(entry.Value.GetInferredType())), "")
				}
				warnNumericConversion(c, n.ElemType, entry.Value, "array element")
				entry.ResolvedIndex = index
				used[index] = true
				cursor = index + 1
			}
		}
		n.InfType = &t.NodeType{KindNode: &t.NodeTypeSlice{ElemKind: n.ElemType.KindNode}}
		return nil
	case *t.NodeExprAddrof:
		if err := ctExpr(c, n.Expr); err != nil {
			return err
		}
		valueType := n.Expr.GetInferredType()
		if valueType == nil || valueType.KindNode == nil {
			return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, "cannot take the address of an expression with no value type", "")
		}
		n.InfType = &t.NodeType{KindNode: &t.NodeTypePointer{Kind: valueType.KindNode}}
		return nil
	case *t.NodeExprMove:
		if err := ctExpr(c, n.Expr); err != nil {
			return err
		}
		n.InfType = n.Expr.GetInferredType()
		return nil
	case *t.NodeExprLlvm:
		arities := map[string]int{
			"reinterpret": 1, "offset": 2, "ptrtoint": 1, "inttoptr": 1,
			"load_volatile": 2, "store_volatile": 3,
			"atomic_load": 3, "atomic_store": 4, "atomic_rmw": 5,
			"cmpxchg_old": 6, "asm_sideeffect": 1, "sideeffect": 0,
			"bitcast": 1, "sext": 1, "zext": 1, "trunc": 1,
			"sitofp": 1, "uitofp": 1, "fptosi": 1, "fptoui": 1,
			"bswap": 1, "bitreverse": 1, "ctpop": 1, "ctlz": 1,
			"cttz": 1, "expect": 2, "assume": 1, "fence": 1, "trap": 0,
		}
		arity, known := arities[n.Operation]
		if !known {
			return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, fmt.Sprintf("unknown @llvm operation '%s'", n.Operation), "use a compiler-registered LLVM operation")
		}
		if len(n.Args) != arity {
			return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, fmt.Sprintf("@llvm operation '%s' expects %d argument(s), but got %d", n.Operation, arity, len(n.Args)), "")
		}
		for _, arg := range n.Args {
			if err := ctExpr(c, arg); err != nil {
				return err
			}
		}
		switch n.Operation {
		case "ptrtoint":
			if !isPointerType(n.Args[0].GetInferredType()) || !isIntegerType(n.ResultType) {
				return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, "@llvm ptrtoint requires a pointer operand and integer result", "example: `@llvm(\"ptrtoint\", value, u64)`")
			}
		case "inttoptr":
			if !isIntegerType(n.Args[0].GetInferredType()) || !isPointerType(n.ResultType) {
				return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, "@llvm inttoptr requires an integer operand and pointer result", "example: `@llvm(\"inttoptr\", value, ptr)`")
			}
		case "store_volatile", "atomic_store", "asm_sideeffect", "sideeffect", "assume", "fence", "trap":
			if !isVoidType(n.ResultType) {
				return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, fmt.Sprintf("@llvm operation '%s' must have result type void", n.Operation), "")
			}
		}
		n.InfType = n.ResultType
		return nil
	case *t.NodeExprCall:
		if n.UnionVariant != nil {
			return nil
		}
		//fmt.Printf("call: %s\n", flattenCallee(n.Callee))

		callArgCount := len(n.Args)
		defArgCount := 0
		expectedArgs := []*t.NodeType{}

		if n.IsFuncPointer {
			if n.FuncPtrType == nil {
				return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, fmt.Sprintf("cannot call '%s': its type could not be resolved", callDisplayName(n)), "")
			}
			funcType, ok := n.FuncPtrType.KindNode.(*t.NodeTypeFunc)
			if !ok {
				return comp_err.CompilationErrorToken(
					c.FileCtx,
					&n.Tk,
					fmt.Sprintf("cannot call '%s': value has non-function type '%s'", callDisplayName(n), flattenType(n.FuncPtrType)),
					"only functions and function-pointer values can be called",
				)
			}
			defArgCount = len(funcType.Args)
			expectedArgs = funcType.Args
		} else {
			definedArgs := n.AssociatedFnDef.Class.ArgsNode.Args
			defArgCount = len(definedArgs)

			if n.IsMemberFunc && defArgCount > 0 {
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
				fmt.Sprintf("function '%s' expects %d argument(s), but got %d", callDisplayName(n), defArgCount, callArgCount),
				"",
			)
		}

		for i, a := range n.Args {
			e := ctExpr(c, a)
			if e != nil {
				return e
			}
			compatible := compatibleInitializer(c, expectedArgs[i], a)
			if !compatible && n.AssociatedFnDef != nil && n.AssociatedFnDef.IsExternal {
				compatible = compatibleNativeCallback(expectedArgs[i], a.GetInferredType(), a)
			}
			if !compatible {
				return comp_err.CompilationErrorToken(
					c.FileCtx,
					expressionSourceToken(a),
					fmt.Sprintf("argument %d to '%s' expects type '%s', but got '%s'", i+1, callDisplayName(n), flattenType(expectedArgs[i]), flattenType(a.GetInferredType())),
					functionPointerMismatchHint(expectedArgs[i], a.GetInferredType(), a),
				)
			}
			warnNumericConversion(c, expectedArgs[i], a, fmt.Sprintf("argument %d", i+1))
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
		if c.ErrorBoundary > 0 && n.InfType != nil && n.InfType.Throws {
			n.ThrowingType = n.InfType
			n.ErrorMode = uint8(c.ErrorBoundary)
			unwrapped := *n.InfType
			unwrapped.Throws = false
			n.InfType = &unwrapped
		}
		if valueUsed && n.InfType != nil && n.InfType.Throws {
			return comp_err.CompilationErrorToken(
				c.FileCtx,
				&n.Tk,
				fmt.Sprintf("cannot use the return value of throwing call '%s' without handling its error", callDisplayName(n)),
				"use `try` to propagate the error or destructure the call into value and error bindings",
			)
		}
		if valueUsed && isVoidType(n.InfType) {
			return comp_err.CompilationErrorToken(
				c.FileCtx,
				&n.Tk,
				fmt.Sprintf("cannot use void-returning call '%s' as a value", callDisplayName(n)),
				"use the call as a standalone statement",
			)
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
			if !compatibleInitializer(c, field.FieldType, field.Expression) {
				return comp_err.CompilationErrorToken(c.FileCtx, &field.Tk, fmt.Sprintf("field '%s' expects type '%s', but initializer has type '%s'", field.Name, flattenType(field.FieldType), flattenType(field.Expression.GetInferredType())), "")
			}
			warnNumericConversion(c, field.FieldType, field.Expression, fmt.Sprintf("field '%s'", field.Name))
		}
		return nil
	case *t.NodeExprProtoView:
		if e := ctExpr(c, n.Target); e != nil {
			return e
		}
		if n.Implementation == nil || n.InfType == nil {
			return fmt.Errorf("prototype view was not resolved during linking")
		}
		return nil
	case *t.NodeExprSubscript:
		return ctExprSubscript(c, n)
	case *t.NodeExprLit:
		switch n.LitType {
		case t.TokLitNum:
			n.InfType = numericLiteralDefaultType(n.Value)
			return nil
		case t.TokLitStr:
			n.InfType = makeNamedType(t.CoreTypeString.Name())
			return nil
		case t.TokLitBool:
			n.InfType = makeNamedType("bool")
			return nil
		case t.TokLitNone:
			n.InfType = makeNamedType("ptr")
			return nil
		}
	case *t.NodeExprEmbed:
		n.InfType = &t.NodeType{KindNode: &t.NodeTypeSlice{ElemKind: makeNamedType("u8").KindNode}}
		return nil
	case *t.NodeExprName:
		if n.AssociatedNode == nil {
			//fmt.Printf("name: %s\n", flattenName(n.Name))
			return fmt.Errorf("name node pointing to no valid node")
		}

		if len(n.MemberAccesses) > 0 {
			result, err := memberPathResult(n)
			if err != nil {
				return err
			}
			n.InfType = result
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
			nullPointerComparison := (isNoneLiteral(n.Left) && (isPointerType(rightT) || isRawPointerType(rightT))) || (isNoneLiteral(n.Right) && (isPointerType(leftT) || isRawPointerType(leftT)))
			if !compatibleTypes(leftT, rightT) && !compatibleTypes(rightT, leftT) && !nullPointerComparison {
				return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, fmt.Sprintf("cannot compare values of unrelated types '%s' and '%s'", flattenType(leftT), flattenType(rightT)), "")
			}
			n.InfType = makeNamedType("bool")
			if isNumberType(leftT) && isNumberType(rightT) {
				n.OperandType = numericPromotionForExpressions(n.Left, n.Right)
			}
		case t.KwCmpLt, t.KwCmpGt, t.KwCmpLtEq, t.KwCmpGtEq:
			leftT := n.Left.GetInferredType()
			rightT := n.Right.GetInferredType()
			if !isNumberType(leftT) || !isNumberType(rightT) {
				return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, fmt.Sprintf("ordering comparison requires numeric operands, but got '%s' and '%s'", flattenType(leftT), flattenType(rightT)), "")
			}
			n.InfType = makeNamedType("bool")
			n.OperandType = numericPromotionForExpressions(n.Left, n.Right)
		case t.KwAndAnd, t.KwOrOr:
			leftT := n.Left.GetInferredType()
			rightT := n.Right.GetInferredType()

			if !isBoolType(leftT) || !isBoolType(rightT) {
				return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, fmt.Sprintf("logical operator '%s' requires 'bool' operands, but got '%s' and '%s'", t.KwTypeToRepr[n.Operator], flattenType(leftT), flattenType(rightT)), "")
			}

			n.InfType = makeNamedType("bool")
			n.OperandType = n.InfType
			return nil
		case t.KwAmpersand, t.KwPipe, t.KwCaret:
			leftT := n.Left.GetInferredType()
			rightT := n.Right.GetInferredType()

			if isBoolType(leftT) || isBoolType(rightT) {
				if !isBoolType(leftT) || !isBoolType(rightT) {
					nonBoolType := leftT
					if isBoolType(leftT) {
						nonBoolType = rightT
					}
					return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, fmt.Sprintf("bitwise operator '%s' cannot mix 'bool' with '%s'", t.KwTypeToRepr[n.Operator], flattenType(nonBoolType)), "both operands must be 'bool', or both must be integers")
				}
				n.InfType = makeNamedType("bool")
				n.OperandType = n.InfType
				return nil
			}

			if !isIntegerType(leftT) || !isIntegerType(rightT) {
				return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, fmt.Sprintf("bitwise operator '%s' requires integer operands, but got '%s' and '%s'", t.KwTypeToRepr[n.Operator], flattenType(leftT), flattenType(rightT)), "")
			}
			n.OperandType = numericPromotionForExpressions(n.Left, n.Right)
			n.InfType = n.OperandType
			return nil
		case t.KwShiftLeft, t.KwShiftRight:
			leftT := n.Left.GetInferredType()
			rightT := n.Right.GetInferredType()
			if !isIntegerType(leftT) || !isIntegerType(rightT) {
				return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, fmt.Sprintf("shift operator '%s' requires integer operands, but got '%s' and '%s'", t.KwTypeToRepr[n.Operator], flattenType(leftT), flattenType(rightT)), "")
			}
			n.OperandType = numericPromotionForExpressions(n.Left, n.Right)
			n.InfType = n.OperandType
			return nil
		case t.KwPlus, t.KwMinus, t.KwAsterisk, t.KwSlash, t.KwPercent:
			leftT := n.Left.GetInferredType()
			rightT := n.Right.GetInferredType()
			if !isNumberType(leftT) || !isNumberType(rightT) {
				return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, fmt.Sprintf("arithmetic operator '%s' requires numeric operands, but got '%s' and '%s'", t.KwTypeToRepr[n.Operator], flattenType(leftT), flattenType(rightT)), "")
			}
			n.OperandType = numericPromotionForExpressions(n.Left, n.Right)
			n.InfType = n.OperandType
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
			if !isIntegerType(operandT) {
				return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, fmt.Sprintf("bitwise not requires an integer operand, but got '%s'", flattenType(operandT)), "")
			}
			n.InfType = operandT
			return nil
		case t.KwNot:
			operandT := n.Operand.GetInferredType()
			if !isBoolType(operandT) {
				return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, fmt.Sprintf("boolean not requires a 'bool' operand, but got '%s'", flattenType(operandT)), "")
			}
			n.InfType = makeNamedType("bool")
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
		} else if !compatibleInitializer(c, n.VarDef.Type, n.AssignExpr) {
			return comp_err.CompilationErrorToken(
				c.FileCtx,
				&n.Tk,
				fmt.Sprintf("cannot initialize value of type '%s' with expression of type '%s'", flattenType(n.VarDef.Type), flattenType(n.AssignExpr.GetInferredType())),
				"",
			)
		}
		warnNumericConversion(c, n.VarDef.Type, n.AssignExpr, "variable initialization")

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
		if !compatibleInitializer(c, n.InfType, n.Right) {
			return comp_err.CompilationErrorToken(
				c.FileCtx,
				&n.Tk,
				fmt.Sprintf("cannot assign value of type '%s' to value of type '%s'", flattenType(n.Right.GetInferredType()), flattenType(n.InfType)),
				"",
			)
		}
		warnNumericConversion(c, n.InfType, n.Right, "assignment")
		return nil
	case *t.NodeExprTry:
		previousBoundary := c.ErrorBoundary
		c.ErrorBoundary = 1
		e := ctExprWithUsage(c, n.Call, false)
		c.ErrorBoundary = previousBoundary
		if e != nil {
			return e
		}
		call, ok := n.Call.(*t.NodeExprCall)
		if !ok || call.ThrowingType == nil || !call.ThrowingType.Throws {
			return comp_err.CompilationErrorToken(
				c.FileCtx,
				&n.Tk,
				fmt.Sprintf("cannot use 'try' with non-throwing call '%s'", expressionDisplayName(n.Call)),
				"remove 'try' or call a function whose return type is marked with '!'",
			)
		}
		if c.CurrentTypeFunc != nil && (c.CurrentTypeFunc.ReturnType == nil || !c.CurrentTypeFunc.ReturnType.Throws) {
			return comp_err.CompilationErrorToken(
				c.FileCtx,
				&n.Tk,
				"cannot use 'try' inside a non-throwing function",
				"mark the enclosing function's return type with '!' or handle the error explicitly",
			)
		}
		n.InfType = call.InfType
		return nil
	case *t.NodeExprDestructureAssign:
		previousBoundary := c.ErrorBoundary
		c.ErrorBoundary = 2
		e := ctExprWithUsage(c, n.Call, false)
		c.ErrorBoundary = previousBoundary
		if e != nil {
			return e
		}

		if n.Call.InfType == nil {
			return comp_err.CompilationErrorToken(c.FileCtx, &n.Call.Tk, "cannot determine the return type for destructuring assignment", "")
		}

		if n.Call.ThrowingType == nil || !n.Call.ThrowingType.Throws {
			return comp_err.CompilationErrorToken(c.FileCtx, &n.Call.Tk, fmt.Sprintf("cannot destructure non-throwing call '%s'", callDisplayName(n.Call)), "destructuring requires a call whose return type is marked with '!'")
		}

		if isVoidType(n.Call.ThrowingType) {
			return comp_err.CompilationErrorToken(c.FileCtx, &n.Call.Tk, fmt.Sprintf("cannot bind a result value from throwing void call '%s'", callDisplayName(n.Call)), "a '!void' call only produces an error result")
		}

		if n.ErrDef.Type == nil {
			n.ErrDef.Type = makeNamedType(t.CoreTypeError.Name())
		}
		if !isErrType(n.ErrDef.Type) || n.ErrDef.Type.Throws {
			return comp_err.CompilationErrorToken(c.FileCtx, &n.Call.Tk, fmt.Sprintf("destructuring error binding must have type 'error', but got '%s'", flattenType(n.ErrDef.Type)), "")
		}

		unwrapped := n.Call.InfType

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

func isNoneLiteral(expr t.NodeExpr) bool {
	literal, ok := expr.(*t.NodeExprLit)
	return ok && literal.LitType == t.TokLitNone
}

func ctExprSubscript(c *ctx, n *t.NodeExprSubscript) error {
	if e := ctExpr(c, n.Expr); e != nil {
		return e
	}
	if !isIntegerType(n.Expr.GetInferredType()) {
		return comp_err.CompilationErrorToken(c.FileCtx, expressionSourceToken(n.Expr), fmt.Sprintf("subscript index must be an integer, but got '%s'", flattenType(n.Expr.GetInferredType())), "use an integer expression to index an array, slice, or pointer")
	}
	if e := ctExpr(c, n.Target); e != nil {
		return e
	}
	n.BoxType = n.Target.GetInferredType()
	n.ElemType = getBoxedType(n.BoxType)
	if n.ElemType == nil {
		return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, fmt.Sprintf("cannot index value of type '%s'", flattenType(n.BoxType)), "only arrays, slices, and pointers can be indexed")
	}
	// Magma indexes are canonically unsigned. LLVM spells both Magma i64 and u64
	// as i64, but signedness remains part of the checker/lowering contract.
	n.IndexType = makeNamedType("u64")
	return nil
}
