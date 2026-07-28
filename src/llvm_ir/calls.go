package llvmir

import (
	t "Magma/src/types"
	"fmt"
	"slices"
)

func irFuncPtrType(ctx *IrCtx, fnType *t.NodeTypeFunc) error {
	e := irThrowingType(ctx, fnType.RetType)
	if e != nil {
		return e
	}

	irWrite(ctx, " (")

	for i, n := range fnType.Args {
		e := irType(ctx, n)
		if e != nil {
			return e
		}

		if i != len(fnType.Args)-1 {
			irWrite(ctx, ", ")
		}
	}

	irWrite(ctx, ")*")
	return nil
}

func irExprCallFuncPtr(ctx *IrCtx, fnCall *t.NodeExprCall, topLevel bool) (SsaName, error) {
	irWrite(ctx, "  ; call fnptr\n")

	if fnCall.FuncPtrType == nil {
		return SsaName{}, fmt.Errorf("cannot lower unresolved function-pointer call")
	}
	fnType, ok := fnCall.FuncPtrType.KindNode.(*t.NodeTypeFunc)
	if !ok {
		return SsaName{}, fmt.Errorf("cannot lower call through non-function type")
	}
	if len(fnCall.Args) != len(fnType.Args) {
		return SsaName{}, fmt.Errorf("cannot lower function-pointer call: expected %d arguments, got %d", len(fnType.Args), len(fnCall.Args))
	}

	argsSsa := make([]SsaName, len(fnCall.Args))
	for i, expr := range fnCall.Args {
		exprSsa, e := irExpression(ctx, fnType.Args[i], expr, false)
		if e != nil {
			return ssaName(""), e
		}
		argsSsa[i] = exprSsa
	}

	fnPtrSsa, e := irExpression(ctx, fnCall.FuncPtrType, fnCall.Callee, false)
	if e != nil {
		return ssaName(""), e
	}
	bitCastPtr := irSsaLocal(ctx)

	irWritef(ctx, "  %s = bitcast ptr %s to ", bitCastPtr.Repr, fnPtrSsa.Repr)

	e = irFuncPtrType(ctx, fnType)
	if e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, "\n")

	ssa := irSsaLocal(ctx)

	isVoidRet := isVoidType(fnCall.InfType)

	if !topLevel && (!isVoidRet || fnCall.InfType.Throws) {
		irWritef(ctx, "  %s = ", ssa.Repr)
	} else {
		irWrite(ctx, "  ")
	}

	irWritef(ctx, "call ")

	e = irThrowingType(ctx, fnCall.InfType)
	if e != nil {
		return ssaName(""), e
	}

	irWritef(ctx, " %s(", bitCastPtr.Repr)

	bound := len(argsSsa)
	for i, ssa := range argsSsa {
		e = irType(ctx, fnType.Args[i])
		if e != nil {
			return ssaName(""), e
		}
		irWrite(ctx, " ")
		irPossibleLitSsa(ctx, ssa)

		if i < bound-1 {
			irWrite(ctx, ", ")
		}
	}

	irWrite(ctx, ")\n")

	if topLevel || (isVoidRet && !fnCall.InfType.Throws) {
		// TODO: Check and inforce that void ret calls HAVE to be statements
		// and cannot be in expressions
		return ssaName(""), nil
	}

	return ssa, nil
}

func irExprCallFuncNonPtr(ctx *IrCtx, fnCall *t.NodeExprCall, topLevel bool) (SsaName, error) {
	if fnCall.AssociatedFnDef == nil {
		return SsaName{}, fmt.Errorf("cannot lower unresolved function call")
	}
	if len(fnCall.Args) != len(fnCall.AssociatedFnDef.Class.ArgsNode.Args) {
		return SsaName{}, fmt.Errorf("cannot lower call to %q: expected %d arguments, got %d", fnCall.AssociatedFnDef.AbsName, len(fnCall.AssociatedFnDef.Class.ArgsNode.Args), len(fnCall.Args))
	}
	irWritef(ctx, "  ; call %s\n", fnCall.AssociatedFnDef.AbsName)

	argsSsa := make([]SsaName, len(fnCall.Args))
	for i, expr := range fnCall.Args {
		argT := fnCall.AssociatedFnDef.Class.ArgsNode.Args[i].TypeNode

		exprSsa, e := irExpression(ctx, argT, expr, false)
		if e != nil {
			return ssaName(""), e
		}

		if isNumberType(argT) && !exprSsa.IsLiteral {
			if !isSameNumType(expr.GetInferredType(), argT) {
				outSsa, e := irPromoteSingleToNum(ctx, argT, exprSsa, expr.GetInferredType())
				if e != nil {
					return SsaName{}, e
				}
				exprSsa = outSsa
			}
		}

		argsSsa[i] = exprSsa
	}

	if fnCall.AssociatedFnDef.IsExternal {
		return irCABIExternalCall(ctx, fnCall, argsSsa, topLevel)
	}

	ssa := irSsaLocal(ctx)
	isVoidRet := isVoidType(fnCall.InfType)

	if !topLevel && (!isVoidRet || fnCall.InfType.Throws) {
		irWritef(ctx, "  %s = ", ssa.Repr)
	} else {
		irWrite(ctx, "  ")
	}

	irWritef(ctx, "call ")

	e := irThrowingType(ctx, fnCall.InfType)
	if e != nil {
		return ssaName(""), e
	}

	irWrite(ctx, " @")

	if fnCall.AssociatedFnDef.NoAliasName != "" {
		// NoAliasName has precedence, used for extern func aliasing
		irWrite(ctx, fnCall.AssociatedFnDef.NoAliasName)
	} else {
		irWrite(ctx, fnCall.AssociatedFnDef.AbsName)
	}

	/*
		switch expr := fnCall.Callee.(type) {
		case *t.NodeExprName:
			e := irName(ctx, expr.Name, true)
			if e != nil {
				return ssaName(""), e
			}
		default:
			irWrite(ctx, "<name>")
		}*/

	irWrite(ctx, "(")

	bound := len(argsSsa)
	for i, ssa := range argsSsa {
		e = irType(ctx, fnCall.AssociatedFnDef.Class.ArgsNode.Args[i].TypeNode)
		if e != nil {
			return ssaName(""), e
		}
		irWrite(ctx, " ")
		irPossibleLitSsa(ctx, ssa)

		if i < bound-1 {
			irWrite(ctx, ", ")
		}
	}

	irWrite(ctx, ")\n")

	if topLevel || (isVoidRet && !fnCall.InfType.Throws) {
		// TODO: Check and inforce that void ret calls HAVE to be statements
		// and cannot be in expressions
		return ssaName(""), nil
	}

	return ssa, nil
}

func irExprCallFuncMember(ctx *IrCtx, fnCall *t.NodeExprCall, topLevel bool) (SsaName, error) {
	if fnCall.AssociatedFnDef == nil {
		return SsaName{}, fmt.Errorf("cannot lower unresolved member-function call")
	}
	expectedArgs := len(fnCall.AssociatedFnDef.Class.ArgsNode.Args) - 1
	if expectedArgs < 0 || len(fnCall.Args) != expectedArgs {
		return SsaName{}, fmt.Errorf("cannot lower member call to %q: expected %d explicit arguments, got %d", fnCall.AssociatedFnDef.AbsName, expectedArgs, len(fnCall.Args))
	}
	irWritef(ctx, "  ; call member %s\n", fnCall.AssociatedFnDef.AbsName)

	argsSsa := make([]SsaName, len(fnCall.Args))
	for i, expr := range fnCall.Args {
		argT := fnCall.AssociatedFnDef.Class.ArgsNode.Args[i+1].TypeNode

		exprSsa, e := irExpression(ctx, argT, expr, false)
		if e != nil {
			return ssaName(""), e
		}

		if isNumberType(argT) && !exprSsa.IsLiteral {
			if !isSameNumType(expr.GetInferredType(), argT) {
				outSsa, e := irPromoteSingleToNum(ctx, argT, exprSsa, expr.GetInferredType())
				if e != nil {
					return SsaName{}, e
				}
				exprSsa = outSsa
			}
		}

		argsSsa[i] = exprSsa
	}

	// implicit this
	var ownerSsa SsaName
	var e error

	if fnCall.MemberOwnerExpr != nil {
		ownerSsa, e = irExpression(ctx, fnCall.MemberOwnerType, fnCall.MemberOwnerExpr, false)
		if e != nil {
			return SsaName{}, e
		}

		if !fnCall.MemberOwnerIsPtr {
			allocSsa := irSsaLocal(ctx)
			irWritef(ctx, "  %s = alloca ", allocSsa.Repr)
			e := irType(ctx, fnCall.MemberOwnerType)
			if e != nil {
				return SsaName{}, e
			}
			irWrite(ctx, "\n")

			irWrite(ctx, "  store ")
			e = irType(ctx, fnCall.MemberOwnerType)
			if e != nil {
				return SsaName{}, e
			}
			irWritef(ctx, " %s, ptr %s\n", ownerSsa.Repr, allocSsa.Repr)

			ownerSsa = allocSsa
		}
	} else {
		var isSsaOwner = false
		switch n := fnCall.Callee.(type) {
		case *t.NodeExprName:
			isSsaOwner = n.Storage.IsSSA()
		}

		ownerSsa, e = irExprNameLvalue(ctx, fnCall.MemberOwnerName)
		if e != nil {
			return SsaName{}, e
		}

		ownerHasMemberPath := fnCall.MemberOwnerName != nil && len(fnCall.MemberOwnerName.MemberAccesses) > 0
		pointerFieldOwner := false
		if ownerHasMemberPath {
			lastAccess := fnCall.MemberOwnerName.MemberAccesses[len(fnCall.MemberOwnerName.MemberAccesses)-1]
			pointerFieldOwner = lastAccess.ResultIsPtr
		}
		pointerLocalOwner := !isSsaOwner && isPointerType(fnCall.MemberOwnerType)
		if pointerFieldOwner || pointerLocalOwner {
			// Non-SSA pointer locals and pointer-valued fields are represented by
			// the address of their pointer slot here. A member call needs the
			// stored pointer value as `this`, not T**.
			loadedOwner := irSsaLocal(ctx)
			irWritef(ctx, "  %s = load ptr, ptr %s\n", loadedOwner.Repr, ownerSsa.Repr)
			ownerSsa = loadedOwner
		} else if isSsaOwner && !fnCall.MemberOwnerIsPtr {
			allocSsa := irSsaLocal(ctx)
			irWritef(ctx, "  %s = alloca ", allocSsa.Repr)
			e := irType(ctx, fnCall.MemberOwnerType)
			if e != nil {
				return SsaName{}, e
			}
			irWrite(ctx, "\n")

			irWrite(ctx, "  store ")
			e = irType(ctx, fnCall.MemberOwnerType)
			if e != nil {
				return SsaName{}, e
			}
			irWritef(ctx, " %s, ptr %s\n", ownerSsa.Repr, allocSsa.Repr)

			ownerSsa = allocSsa
		}
	}
	argsSsa = slices.Insert(argsSsa, 0, ownerSsa)

	ssa := irSsaLocal(ctx)
	isVoidRet := isVoidType(fnCall.InfType)

	if !topLevel && (!isVoidRet || fnCall.InfType.Throws) {
		irWritef(ctx, "  %s = ", ssa.Repr)
	} else {
		irWrite(ctx, "  ")
	}

	irWrite(ctx, "call ")

	e = irThrowingType(ctx, fnCall.InfType)
	if e != nil {
		return ssaName(""), e
	}

	irWrite(ctx, " @")

	//irWritef(ctx, "%s.", fnCall.MemberOwnerModule)
	if fnCall.AssociatedFnDef.NoAliasName != "" {
		return SsaName{}, fmt.Errorf("cannot call aliased or external function as member function, something went terribly wrong.")
	} else {
		irWrite(ctx, fnCall.AssociatedFnDef.AbsName)
	}

	/*
		e = irName(ctx, fnCall.AssociatedFnDef.Class.NameNode, false)
		if e != nil {
			return ssaName(""), e
		}*/

	irWrite(ctx, "(")

	bound := len(argsSsa)
	for i, ssa := range argsSsa {
		e = irType(ctx, fnCall.AssociatedFnDef.Class.ArgsNode.Args[i].TypeNode)
		if e != nil {
			return ssaName(""), e
		}
		irWrite(ctx, " ")
		irPossibleLitSsa(ctx, ssa)

		if i < bound-1 {
			irWrite(ctx, ", ")
		}
	}

	irWrite(ctx, ")\n")

	if topLevel || (isVoidRet && !fnCall.InfType.Throws) {
		// TODO: Check and enforce that void ret calls HAVE to be statements
		// and cannot be in expressions
		return ssaName(""), nil
	}

	return ssa, nil
}

func irExprFuncCall(ctx *IrCtx, fnCall *t.NodeExprCall, keepError bool, topLevel bool) (SsaName, error) {
	var ssa = SsaName{}
	var e error

	if fnCall.IsFuncPointer {
		ssa, e = irExprCallFuncPtr(ctx, fnCall, topLevel)
	} else if fnCall.IsMemberFunc {
		ssa, e = irExprCallFuncMember(ctx, fnCall, topLevel)
	} else {
		ssa, e = irExprCallFuncNonPtr(ctx, fnCall, topLevel)
	}

	if e != nil {
		return SsaName{}, e
	}

	if ssa.Repr == "" {
		return SsaName{}, nil
	}

	if topLevel {
		// discard return
		return SsaName{}, nil
	}

	if fnCall.InfType.Throws && !keepError && !isVoidType(fnCall.InfType) {
		extractSsa := irSsaLocal(ctx)
		irWritef(ctx, "  %s = extractvalue ", extractSsa.Repr)

		e = irThrowingType(ctx, fnCall.InfType)
		if e != nil {
			return SsaName{}, e
		}

		irWritef(ctx, " %s, 1\n", ssa.Repr)
		ssa = extractSsa
	}
	return ssa, nil
}
