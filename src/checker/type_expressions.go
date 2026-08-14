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
		n.IndexType = makeNamedType("i64")

		//fmt.Printf("subscript:\n")
		//fmt.Printf(" type: %s\n", flattenType(n.BoxType))
		//fmt.Printf(" elemtype: %s\n", flattenType(n.ElemType))
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
				if !compatibleInitializer(n.ElemType, entry.Value) {
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
		n.InfType = makeNamedType("ptr")
		return nil
	case *t.NodeExprMove:
		if err := ctExpr(c, n.Expr); err != nil {
			return err
		}
		n.InfType = n.Expr.GetInferredType()
		return nil
	case *t.NodeExprCall:
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
				fmt.Sprintf("function '%s' expects %d argument(s), but got %d", callDisplayName(n), defArgCount, callArgCount),
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
			if !compatibleInitializer(field.FieldType, field.Expression) {
				return comp_err.CompilationErrorToken(c.FileCtx, &field.Tk, fmt.Sprintf("field '%s' expects type '%s', but initializer has type '%s'", field.Name, flattenType(field.FieldType), flattenType(field.Expression.GetInferredType())), "")
			}
			warnNumericConversion(c, field.FieldType, field.Expression, fmt.Sprintf("field '%s'", field.Name))
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
		n.IndexType = makeNamedType("i64")

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
			if !compatibleTypes(leftT, rightT) {
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
		if !compatibleInitializer(n.InfType, n.Right) {
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
		e := ctExprWithUsage(c, n.Call, false)
		if e != nil {
			return e
		}
		callType := n.Call.GetInferredType()
		if callType == nil || !callType.Throws {
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
		unwrapped := *callType
		unwrapped.Throws = false
		n.InfType = &unwrapped
		return nil
	case *t.NodeExprDestructureAssign:
		e := ctExprWithUsage(c, n.Call, false)
		if e != nil {
			return e
		}

		if n.Call.InfType == nil {
			return comp_err.CompilationErrorToken(c.FileCtx, &n.Call.Tk, "cannot determine the return type for destructuring assignment", "")
		}

		if !n.Call.InfType.Throws {
			return comp_err.CompilationErrorToken(c.FileCtx, &n.Call.Tk, fmt.Sprintf("cannot destructure non-throwing call '%s'", callDisplayName(n.Call)), "destructuring requires a call whose return type is marked with '!'")
		}

		if isVoidType(n.Call.InfType) {
			return comp_err.CompilationErrorToken(c.FileCtx, &n.Call.Tk, fmt.Sprintf("cannot bind a result value from throwing void call '%s'", callDisplayName(n.Call)), "a '!void' call only produces an error result")
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
