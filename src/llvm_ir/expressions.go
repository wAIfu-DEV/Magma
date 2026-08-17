package llvmir

import (
	magmatypes "Magma/src/magma_types"
	t "Magma/src/types"
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

func irCStringGlobal(ctx *IrCtx, value string) SsaName {
	constSsa := irSsaGlobal(ctx)
	constLen := len(value) + 1

	irWriteGlf(ctx, "%s = private constant [%d x i8] c\"%s\\00\"\n", constSsa.Repr, constLen, escapeCString(value))
	return constSsa
}

func irExprLitStr(ctx *IrCtx, litStr *t.NodeExprLit) (SsaName, error) {
	constSsa := irCStringGlobal(ctx, litStr.Value)
	constLen := len(litStr.Value) + 1

	//irWritef(ctx, "  %%%s = insertvalue %%type.str undef, ptr @%s, 0\n", strFieldSsa.Repr, constSsa.Repr)
	//irWritef(ctx, "  %%%s = insertvalue %%type.str %%%s, i64 %d, 1\n", sizeFieldSsa.Repr, strFieldSsa.Repr, constLen-1)

	litSsa := SsaName{
		Repr:      fmt.Sprintf("{ ptr %s, i64 %d }", constSsa.Repr, constLen-1),
		IsLiteral: true,
	}

	return litSsa, nil
}

func irExprLitNum(ctx *IrCtx, litNum *t.NodeExprLit) (SsaName, error) {
	ssa := ssaName(litNum.Value)
	ssa.IsLiteral = true
	return ssa, nil
}

func irExprLitBool(ctx *IrCtx, litBool *t.NodeExprLit) (SsaName, error) {
	ssa := ssaName(litBool.Value)
	ssa.IsLiteral = true
	return ssa, nil
}

func irExprLitNone(ctx *IrCtx, litNone *t.NodeExprLit) (SsaName, error) {
	return SsaName{Repr: "null", IsLiteral: true}, nil
}

func irExprLit(ctx *IrCtx, lit *t.NodeExprLit, expectedType *t.NodeType) (SsaName, error) {
	switch lit.LitType {
	case t.TokLitStr:
		return irExprLitStr(ctx, lit)
	case t.TokLitNum:
		return irExprLitNum(ctx, lit)
	case t.TokLitBool:
		return irExprLitBool(ctx, lit)
	case t.TokLitNone:
		return irExprLitNone(ctx, lit)
	}
	return ssaName(""), nil
}

func irExprSizeof(ctx *IrCtx, sz *t.NodeExprSizeof) (SsaName, error) {
	if sz.Type == nil {
		return SsaName{}, fmt.Errorf("sizeof: missing type")
	}

	if isVoidType(sz.Type) {
		irWrite(ctx, "; sizeof void type\n")
		return SsaName{Repr: "0", IsLiteral: true}, nil
	}

	switch n := sz.Type.KindNode.(type) {
	case *t.NodeTypeNamed:
		if nn, ok := n.NameNode.(*t.NodeNameSingle); ok {
			if nn.Name == "bool" {
				return SsaName{Repr: "1", IsLiteral: true}, nil
			}
			if desc, ok := magmatypes.NumberTypes[nn.Name]; ok {
				return SsaName{Repr: strconv.Itoa(desc.ByteSize / 8), IsLiteral: true}, nil
			}
		}
	}

	typeBld := &bytes.Buffer{}
	cpy := *ctx
	cpy.bld = ScopeBuilder{
		Global: typeBld,
		Head:   typeBld,
		Tail:   typeBld,
		Body:   typeBld,
	}

	if err := irType(&cpy, sz.Type); err != nil {
		return SsaName{}, err
	}

	typeStr := strings.TrimSpace(typeBld.String())
	if typeStr == "" || strings.Contains(typeStr, "<") {
		return SsaName{}, fmt.Errorf("sizeof: unsupported type for sizing")
	}

	gepSsa := irSsaLocal(ctx)
	irWritef(ctx, "  %s = getelementptr %s, ptr null, i64 1\n", gepSsa.Repr, typeStr)

	sizeSsa := irSsaLocal(ctx)
	irWritef(ctx, "  %s = ptrtoint ptr %s to i64\n", sizeSsa.Repr, gepSsa.Repr)
	return sizeSsa, nil
}

func irExprAddrof(ctx *IrCtx, ao *t.NodeExprAddrof) (SsaName, error) {
	forceMaterialize := false
	addressedType := ao.Expr.GetInferredType()
	if name, ok := ao.Expr.(*t.NodeExprName); ok {
		forceMaterialize = name.Storage.IsSSA() && len(name.MemberAccesses) == 0
		switch definition := name.AssociatedNode.(type) {
		case *t.NodeExprVarDef:
			addressedType = definition.Type
		case *t.NodeExprVarDefAssign:
			addressedType = definition.VarDef.Type
		}
	}
	var exprSsa SsaName
	var e error
	if forceMaterialize {
		e = fmt.Errorf("expr not lvalue")
	} else {
		exprSsa, e = irExpressionLvalue(ctx, ao.Expr) // handles alloca'd values and GEP
	}
	if e != nil && e.Error() != "expr not lvalue" {
		return SsaName{}, e
	} else if e != nil {
		exprSsa, e = irExpression(ctx, nil, ao.Expr, false)
		if e != nil {
			return SsaName{}, e
		}
		cpy := *ctx
		cpy.bld.Body = cpy.bld.Head // redirect writing to head of scope

		allocSsa := irSsaLocal(ctx)
		irWritef(&cpy, "  %s = alloca ", allocSsa.Repr)
		e := irType(&cpy, addressedType)
		if e != nil {
			return SsaName{}, e
		}
		irWrite(&cpy, "\n")
		irWrite(ctx, "  store ")
		e = irType(ctx, addressedType)
		if e != nil {
			return SsaName{}, e
		}
		irWrite(ctx, " ")
		irPossibleLitSsa(ctx, exprSsa)
		irWritef(ctx, ", ptr %s\n", allocSsa.Repr)
		exprSsa = allocSsa
	}
	return exprSsa, nil
}

func irMemberAccess(ctx *IrCtx, fromType *t.NodeType, fromSsa SsaName, fieldNb int, fieldType *t.NodeType, isPtrDeref bool) (SsaName, error) {
	fieldSsa := irSsaLocal(ctx)

	if isPtrDeref {
		loadSsa := irSsaLocal(ctx)

		switch n := fromType.KindNode.(type) {
		case *t.NodeTypePointer:
			fromType = &t.NodeType{
				KindNode: n.Kind,
			}
		}

		irWritef(ctx, "  %s = load ", loadSsa.Repr)
		e := irType(ctx, fromType)
		if e != nil {
			return SsaName{}, e
		}
		irWritef(ctx, ", ptr %s\n", fromSsa.Repr)
		fromSsa = loadSsa
	}

	// TODO: Possible lit ssa? maybe not
	irWritef(ctx, "  %s = extractvalue ", fieldSsa.Repr)
	e := irType(ctx, fromType)
	if e != nil {
		return SsaName{}, e
	}

	irWritef(ctx, " %s, %d\n", fromSsa.Repr, fieldNb)
	return fieldSsa, nil
}

func irMemberAddress(ctx *IrCtx, basePtr SsaName, baseType *t.NodeType, fieldIndex int) (SsaName, error) {
	fieldPtr := irSsaLocal(ctx)

	irWritef(ctx, "  %s = getelementptr ", fieldPtr.Repr)

	if isPointerType(baseType) {
		pointerType := baseType.KindNode.(*t.NodeTypePointer)
		e := irTypeKind(ctx, pointerType.Kind)
		if e != nil {
			return SsaName{}, e
		}
	} else {
		e := irType(ctx, baseType)
		if e != nil {
			return SsaName{}, e
		}
	}

	irWritef(ctx, ", ptr %s, i32 0, i32 %d\n", basePtr.Repr, fieldIndex)
	return fieldPtr, nil
}

func irNameVariableStorage(ctx *IrCtx, nameExpr *t.NodeExprName) (SsaName, *t.NodeType, error) {
	if nameExpr == nil || nameExpr.Name == nil {
		return SsaName{}, nil, fmt.Errorf("cannot lower unresolved variable name")
	}

	var variable *t.NodeExprVarDef
	switch definition := nameExpr.AssociatedNode.(type) {
	case *t.NodeExprVarDef:
		variable = definition
	case *t.NodeExprVarDefAssign:
		if definition.VarDef == nil {
			return SsaName{}, nil, fmt.Errorf("cannot lower name with incomplete variable definition")
		}
		variable = definition.VarDef
	default:
		return SsaName{}, nil, fmt.Errorf("cannot lower name without a resolved variable")
	}

	if variable.Type == nil || variable.Type.KindNode == nil {
		return SsaName{}, nil, fmt.Errorf("cannot lower variable name without a resolved type")
	}
	if variable.IsImplicitContext {
		return SsaName{Repr: "%.ctx.addr"}, variable.Type, nil
	}
	switch variable.Storage {
	case t.VariableStorageGlobal:
		if variable.AbsName == "" {
			return SsaName{}, nil, fmt.Errorf("cannot lower global variable without a resolved symbol")
		}
		return SsaName{Repr: "@" + variable.AbsName}, variable.Type, nil
	case t.VariableStorageArgument:
		name, ok := variable.Name.(*t.NodeNameSingle)
		if !ok || name.Name == "" {
			return SsaName{}, nil, fmt.Errorf("cannot lower argument without a concrete name")
		}
		return SsaName{Repr: "%" + name.Name + ".addr"}, variable.Type, nil
	case t.VariableStorageLocal:
		slot, ok := ctx.localSlots[variable]
		if !ok {
			return SsaName{}, nil, fmt.Errorf("cannot lower local variable without an assigned backend slot")
		}
		return slot, variable.Type, nil
	case t.VariableStorageSSA:
		return irNameSsa(ctx, variable.Name, false), variable.Type, nil
	default:
		return SsaName{}, nil, fmt.Errorf("cannot lower variable with unresolved storage")
	}
}

func irExprName(ctx *IrCtx, nameExpr *t.NodeExprName) (SsaName, error) {
	if nameExpr == nil || nameExpr.Name == nil {
		return SsaName{}, fmt.Errorf("cannot lower unresolved name expression")
	}
	ptrSsa := irNameSsa(ctx, nameExpr.Name, false)
	ssa := irSsaLocal(ctx)

	var typeNd *t.NodeType = nil
	isMemberAccess := len(nameExpr.MemberAccesses) > 0

	isFuncName := false
	var fnDef *t.NodeFuncDef = nil

	switch n := nameExpr.AssociatedNode.(type) {
	case *t.NodeExprVarDef:
		var e error
		ptrSsa, typeNd, e = irNameVariableStorage(ctx, nameExpr)
		if e != nil {
			return SsaName{}, e
		}
	case *t.NodeExprVarDefAssign:
		var e error
		ptrSsa, typeNd, e = irNameVariableStorage(ctx, nameExpr)
		if e != nil {
			return SsaName{}, e
		}
	case *t.NodeFuncDef:
		fnDef = n
		isFuncName = true
		// Function values are fully typed by the checker. Lowering deliberately
		// consumes that resolved signature instead of rebuilding it from the
		// declaration and risking disagreement with specialization metadata.
		typeNd = nameExpr.InfType
	default:
		return SsaName{}, fmt.Errorf("cannot lower name without a resolved declaration")
	}
	if typeNd == nil || typeNd.KindNode == nil {
		return SsaName{}, fmt.Errorf("cannot lower name without a resolved type")
	}

	if nameExpr.Storage.IsSSA() && !isMemberAccess {
		return irNameSsa(ctx, nameExpr.Name, false), nil
	} else if nameExpr.Storage.IsSSA() {
		ssa = ptrSsa
	} else if isFuncName {
		irWritef(ctx, "  %s = bitcast ptr @", ssa.Repr)

		if nameExpr.NativeContextThunk {
			irWrite(ctx, nativeContextThunkSymbol(fnDef))
		} else if nameExpr.ContextAdapter {
			irWrite(ctx, contextAdapterSymbol(fnDef))
		} else if fnDef.NoAliasName != "" {
			// NoAliasName has precedence, used for extern func aliasing
			irWrite(ctx, fnDef.NoAliasName)
		} else {
			irWrite(ctx, fnDef.AbsName)
		}
		/*
			e := irName(ctx, fnDef.Class.NameNode, true)
			if e != nil {
				return ssaName(""), e
			}*/

		irWrite(ctx, " to ")

		functionType, ok := typeNd.KindNode.(*t.NodeTypeFunc)
		if !ok {
			return SsaName{}, fmt.Errorf("cannot lower function value with non-function type")
		}
		e := irFuncPtrType(ctx, functionType)
		if e != nil {
			return ssaName(""), e
		}
		irWrite(ctx, "\n")
	} else {
		irWritef(ctx, "  %s = load ", ssa.Repr)

		e := irType(ctx, typeNd)
		if e != nil {
			return ssaName(""), e
		}

		irWritef(ctx, ", ptr %s\n", ptrSsa.Repr)
	}

	if isMemberAccess {
		lastSsa := ssa
		for _, m := range nameExpr.MemberAccesses {
			fieldSsa, e := irMemberAccess(ctx, m.OwnerType, lastSsa, m.FieldNb, m.Type, m.PtrDeref)
			if e != nil {
				return SsaName{}, e
			}
			lastSsa = fieldSsa
		}
		return lastSsa, nil
	}
	return ssa, nil
}

func irExprMemberAccess(ctx *IrCtx, member *t.NodeExprMemberAccess) (SsaName, error) {
	if member == nil || member.Target == nil {
		return SsaName{}, fmt.Errorf("member access has no target")
	}
	if member.Access == nil || member.Access.Type == nil {
		return SsaName{}, fmt.Errorf("member access '%s' has no resolved access info", member.Member)
	}
	if member.Target.GetInferredType() == nil {
		return SsaName{}, fmt.Errorf("member access '%s' has no resolved target type", member.Member)
	}

	targetSsa, e := irExpression(ctx, member.Target.GetInferredType(), member.Target, false)
	if e != nil {
		return SsaName{}, e
	}

	return irMemberAccess(
		ctx,
		member.Access.OwnerType,
		targetSsa,
		member.Access.FieldNb,
		member.Access.Type,
		member.Access.PtrDeref,
	)
}

func irExprMemberAccessLvalue(ctx *IrCtx, member *t.NodeExprMemberAccess) (SsaName, error) {
	if member == nil || member.Target == nil {
		return SsaName{}, fmt.Errorf("member access has no target")
	}
	if member.Access == nil || member.Access.Type == nil {
		return SsaName{}, fmt.Errorf("member access '%s' has no resolved access info", member.Member)
	}
	if member.Target.GetInferredType() == nil {
		return SsaName{}, fmt.Errorf("member access '%s' has no resolved target type", member.Member)
	}

	var basePtr SsaName
	var e error
	if name, ok := member.Target.(*t.NodeExprName); ok && name.Storage.IsSSA() {
		value, valueErr := irExpression(ctx, member.Target.GetInferredType(), member.Target, false)
		if valueErr != nil {
			return SsaName{}, valueErr
		}
		basePtr = irSsaLocal(ctx)
		cpy := *ctx
		cpy.bld.Body = cpy.bld.Head
		irWritef(&cpy, "  %s = alloca ", basePtr.Repr)
		if e = irType(&cpy, member.Target.GetInferredType()); e != nil {
			return SsaName{}, e
		}
		irWrite(&cpy, "\n")
		irWrite(ctx, "  store ")
		if e = irType(ctx, member.Target.GetInferredType()); e != nil {
			return SsaName{}, e
		}
		irWritef(ctx, " %s, ptr %s\n", value.Repr, basePtr.Repr)
	} else {
		basePtr, e = irExpressionLvalue(ctx, member.Target)
		if e != nil {
			return SsaName{}, e
		}
	}

	// For an access such as wrapper.pointerField.value, basePtr initially
	// addresses the slot containing pointerField. Load that pointer before
	// computing the address of value in the pointee.
	if isPointerType(member.Target.GetInferredType()) {
		loadedPtr := irSsaLocal(ctx)
		irWritef(ctx, "  %s = load ptr, ptr %s\n", loadedPtr.Repr, basePtr.Repr)
		basePtr = loadedPtr
	}

	return irMemberAddress(ctx, basePtr, member.Access.OwnerType, member.Access.FieldNb)
}

func irExprNameLvalue(ctx *IrCtx, nameExpr *t.NodeExprName) (SsaName, error) {
	if nameExpr == nil || nameExpr.Name == nil {
		return SsaName{}, fmt.Errorf("cannot lower unresolved name lvalue")
	}
	basePtr, rootType, err := irNameVariableStorage(ctx, nameExpr)
	if err != nil {
		return SsaName{}, err
	}

	if nameExpr.Storage.IsSSA() && len(nameExpr.MemberAccesses) > 0 {
		if !isPointerType(rootType) {
			rootValue := basePtr
			basePtr = irSsaLocal(ctx)
			cpy := *ctx
			cpy.bld.Body = cpy.bld.Head
			irWritef(&cpy, "  %s = alloca ", basePtr.Repr)
			if e := irType(&cpy, rootType); e != nil {
				return SsaName{}, e
			}
			irWrite(&cpy, "\n")
			irWrite(ctx, "  store ")
			if e := irType(ctx, rootType); e != nil {
				return SsaName{}, e
			}
			irWritef(ctx, " %s, ptr %s\n", rootValue.Repr, basePtr.Repr)
		}
	}

	if len(nameExpr.MemberAccesses) == 0 {
		return basePtr, nil
	}

	curPtr := basePtr
	curType := rootType
	curPtrIsPointerValue := false
	curPtrIsPointerValue = isPointerType(curType) && nameExpr.Storage.IsSSA()

	// A non-SSA pointer local is an alloca containing the pointer value. Load
	// that value before taking the address of one of the pointee's fields.
	if isPointerType(curType) && !nameExpr.Storage.IsSSA() {
		loaded := irSsaLocal(ctx)
		irWritef(ctx, "  %s = load ptr, ptr %s\n", loaded.Repr, basePtr.Repr)
		basePtr = loaded
		curPtr = loaded
		curPtrIsPointerValue = true
	}

	for _, m := range nameExpr.MemberAccesses {
		// Access metadata records whether this particular hop crosses a pointer.
		if m.PtrDeref && !curPtrIsPointerValue {
			loaded := irSsaLocal(ctx)
			irWritef(ctx, "  %s = load ptr, ptr %s\n", loaded.Repr, curPtr.Repr)
			curPtr = loaded
		}
		nextPtr, err := irMemberAddress(ctx, curPtr, curType, m.FieldNb)
		if err != nil {
			return ssaName(""), err
		}

		curPtr = nextPtr
		curType = m.Type
		curPtrIsPointerValue = false
	}

	return curPtr, nil
}

func irExprSubscript(ctx *IrCtx, subs *t.NodeExprSubscript) (SsaName, error) {
	if subs == nil || subs.Target == nil || subs.Expr == nil {
		return SsaName{}, fmt.Errorf("cannot lower incomplete subscript expression")
	}
	if subs.BoxType == nil || subs.BoxType.KindNode == nil || subs.ElemType == nil || subs.ElemType.KindNode == nil {
		return SsaName{}, fmt.Errorf("cannot lower subscript without resolved container and element types")
	}
	if _, pointer := subs.BoxType.KindNode.(*t.NodeTypePointer); !pointer && subs.RangeProof == nil {
		return SsaName{}, fmt.Errorf("cannot lower safe subscript at line %d, column %d without a validated range proof", subs.Tk.Pos.Line, subs.Tk.Pos.Col)
	}
	subsExpr, e := irExpression(ctx, subs.IndexType, subs.Expr, false)
	if e != nil {
		return SsaName{}, e
	}
	subsExpr, e = irPromoteSingleToNum(ctx, subs.IndexType, subsExpr, subs.Expr.GetInferredType())
	if e != nil {
		return SsaName{}, e
	}

	switch subs.BoxType.KindNode.(type) {
	case *t.NodeTypeSlice:
		var loadedTarget SsaName
		if subs.IsTargetSsa {
			loadedTarget, e = irExpression(ctx, subs.BoxType, subs.Target, false)
			if e != nil {
				return SsaName{}, e
			}
		} else {
			targetPtr, e := irExpressionLvalue(ctx, subs.Target)
			if e != nil {
				return SsaName{}, e
			}

			loadedTarget = irSsaLocal(ctx)
			irWritef(ctx, "  %s = load ", loadedTarget.Repr)
			e = irType(ctx, subs.BoxType)
			if e != nil {
				return SsaName{}, e
			}
			irWritef(ctx, ", ptr %s\n", targetPtr.Repr)
		}
		// extract ptr from struct first
		extracted := irSsaLocal(ctx)
		irWritef(ctx, "  %s = extractvalue %%type.slice %s, 0\n", extracted.Repr, loadedTarget.Repr)
		return irExprSubscriptPtr(ctx, subs, extracted, subsExpr)
	case *t.NodeTypePointer:
		var loadedTarget SsaName
		if subs.IsTargetSsa {
			loadedTarget, e = irExpression(ctx, nil, subs.Target, false)
			if e != nil {
				return SsaName{}, e
			}
		} else {
			targetPtr, e := irExpressionLvalue(ctx, subs.Target)
			if e != nil {
				return SsaName{}, e
			}

			loadedTarget = irSsaLocal(ctx)
			irWritef(ctx, "  %s = load ", loadedTarget.Repr)
			e = irType(ctx, subs.BoxType)
			if e != nil {
				return SsaName{}, e
			}
			irWritef(ctx, ", ptr %s\n", targetPtr.Repr)
		}
		return irExprSubscriptPtr(ctx, subs, loadedTarget, subsExpr)
	case *t.NodeTypeRfc:
		var loadedTarget SsaName
		if subs.IsTargetSsa {
			loadedTarget, e = irExpression(ctx, nil, subs.Target, false)
			if e != nil {
				return SsaName{}, e
			}
		} else {
			targetPtr, e := irExpressionLvalue(ctx, subs.Target)
			if e != nil {
				return SsaName{}, e
			}

			loadedTarget = irSsaLocal(ctx)
			irWritef(ctx, "  %s = load ", loadedTarget.Repr)
			e = irType(ctx, subs.BoxType)
			if e != nil {
				return SsaName{}, e
			}
			irWritef(ctx, ", ptr %s\n", targetPtr.Repr)
		}
		return irExprSubscriptPtr(ctx, subs, loadedTarget, subsExpr)
	}
	return SsaName{}, fmt.Errorf("invalid box type in subscript expression lowering")
}

func irExprSubscriptPtr(ctx *IrCtx, subs *t.NodeExprSubscript, targetSsa SsaName, subsSsa SsaName) (SsaName, error) {
	elemPtr := irSsaLocal(ctx)
	loadedElem := irSsaLocal(ctx)

	irWritef(ctx, "  %s = getelementptr ", elemPtr.Repr)

	e := irType(ctx, subs.ElemType)
	if e != nil {
		return SsaName{}, e
	}

	irWritef(ctx, ", ptr %s, i64 ", targetSsa.Repr)

	irPossibleLitSsa(ctx, subsSsa)
	irWrite(ctx, "\n")

	irWritef(ctx, "  %s = load ", loadedElem.Repr)

	e = irType(ctx, subs.ElemType)
	if e != nil {
		return SsaName{}, e
	}

	irWritef(ctx, ", ptr %s\n", elemPtr.Repr)
	return loadedElem, nil
}

func irExprSubscriptLvalue(ctx *IrCtx, subs *t.NodeExprSubscript) (SsaName, error) {
	if subs == nil || subs.Target == nil || subs.Expr == nil {
		return SsaName{}, fmt.Errorf("cannot lower incomplete subscript lvalue")
	}
	if subs.BoxType == nil || subs.BoxType.KindNode == nil || subs.ElemType == nil || subs.ElemType.KindNode == nil {
		return SsaName{}, fmt.Errorf("cannot lower subscript lvalue without resolved container and element types")
	}
	if _, pointer := subs.BoxType.KindNode.(*t.NodeTypePointer); !pointer && subs.RangeProof == nil {
		return SsaName{}, fmt.Errorf("cannot lower safe subscript lvalue at line %d, column %d without a validated range proof", subs.Tk.Pos.Line, subs.Tk.Pos.Col)
	}
	subsExpr, e := irExpression(ctx, subs.IndexType, subs.Expr, false)
	if e != nil {
		return SsaName{}, e
	}
	subsExpr, e = irPromoteSingleToNum(ctx, subs.IndexType, subsExpr, subs.Expr.GetInferredType())
	if e != nil {
		return SsaName{}, e
	}

	var targetPtrSsa SsaName

	switch subs.BoxType.KindNode.(type) {
	case *t.NodeTypeSlice:
		var loadedTarget SsaName
		if subs.IsTargetSsa {
			loadedTarget, e = irExpression(ctx, subs.BoxType, subs.Target, false)
			if e != nil {
				return SsaName{}, e
			}
		} else {
			targetPtr, e := irExpressionLvalue(ctx, subs.Target)
			if e != nil {
				return SsaName{}, e
			}

			loadedTarget = irSsaLocal(ctx)
			irWritef(ctx, "  %s = load ", loadedTarget.Repr)
			e = irType(ctx, subs.BoxType)
			if e != nil {
				return SsaName{}, e
			}
			irWritef(ctx, ", ptr %s\n", targetPtr.Repr)
		}

		// extract ptr from struct first
		extracted := irSsaLocal(ctx)
		irWritef(ctx, "  %s = extractvalue %%type.slice %s, 0\n", extracted.Repr, loadedTarget.Repr)
		targetPtrSsa = extracted
	case *t.NodeTypePointer, *t.NodeTypeRfc:
		if subs.IsTargetSsa {
			targetPtrSsa, e = irExpression(ctx, nil, subs.Target, false)
			if e != nil {
				return SsaName{}, e
			}
		} else {
			targetPtr, e := irExpressionLvalue(ctx, subs.Target)
			if e != nil {
				return SsaName{}, e
			}

			targetPtrSsa = irSsaLocal(ctx)
			irWritef(ctx, "  %s = load ", targetPtrSsa.Repr)
			e = irType(ctx, subs.BoxType)
			if e != nil {
				return SsaName{}, e
			}
			irWritef(ctx, ", ptr %s\n", targetPtr.Repr)
		}
	default:
		return SsaName{}, fmt.Errorf("invalid box type in subscript lvalue lowering")
	}

	elemPtr := irSsaLocal(ctx)
	irWritef(ctx, "  %s = getelementptr ", elemPtr.Repr)

	e = irType(ctx, subs.ElemType)
	if e != nil {
		return SsaName{}, e
	}

	irWritef(ctx, ", ptr %s, i64 ", targetPtrSsa.Repr)
	irPossibleLitSsa(ctx, subsExpr)
	irWrite(ctx, "\n")

	return elemPtr, nil
}

func irExprAssign(ctx *IrCtx, ass *t.NodeExprAssign, lhs t.NodeExpr, rhs t.NodeExpr) (SsaName, error) {
	lhsPtr, e := irExpressionLvalue(ctx, lhs)
	if e != nil {
		return SsaName{}, e
	}

	rhsVal, e := irExpression(ctx, lhs.GetInferredType(), rhs, false)
	if e != nil {
		return SsaName{}, e
	}
	rhsVal, e = irCoerceNumeric(ctx, lhs.GetInferredType(), rhs, rhsVal)
	if e != nil {
		return SsaName{}, e
	}

	// TODO: this assumes we correctly infer the expression type during type checking,
	// but we don't, we need to make sure the inference rules mirror number promotion
	/*
		if isNumberType(lhs.GetInferredType()) {
			if !isSameNumType(lhs.GetInferredType(), rhs.GetInferredType()) {
				if !rhsVal.IsLiteral {
					return SsaName{}, comp_err.CompilationErrorToken(
						ctx.fCtx,
						&ass.Tk,
						"implicit number cast is forbidden on assignment",
						fmt.Sprintf("left side type is: %s, right side type is: %s", t.DisplayType(lhs.GetInferredType()), t.DisplayType(rhs.GetInferredType())),
					)
				}
			}
		}*/

	irWrite(ctx, "  store ")

	e = irType(ctx, lhs.GetInferredType())
	if e != nil {
		return SsaName{}, e
	}

	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, rhsVal)
	irWritef(ctx, ", ptr %s\n", lhsPtr.Repr)

	ssa := irSsaLocal(ctx)
	irWritef(ctx, "  %s = load ", ssa.Repr)

	e = irType(ctx, lhs.GetInferredType())
	if e != nil {
		return SsaName{}, e
	}

	irWritef(ctx, ", ptr %s\n", lhsPtr.Repr)
	return lhsPtr, nil
}

func irTryCall(ctx *IrCtx, callRetSsa SsaName, fnCall *t.NodeExprCall, pos t.FilePos) (SsaName, error) {
	errSsa := irSsaLocal(ctx)

	irWritef(ctx, "  %s = extractvalue ", errSsa.Repr)

	returnType := callReturnType(fnCall)
	e := irThrowingType(ctx, returnType)
	if e != nil {
		return SsaName{}, e
	}

	irWritef(ctx, " %s, 0\n", callRetSsa.Repr)

	e = irThrowSsa(ctx, errSsa, ctx.CurrFunc, pos)
	if e != nil {
		return SsaName{}, e
	}

	if !isVoidType(returnType) {
		valSsa := irSsaLocal(ctx)

		irWritef(ctx, "  %s = extractvalue ", valSsa.Repr)

		e = irThrowingType(ctx, returnType)
		if e != nil {
			return SsaName{}, e
		}

		irWritef(ctx, " %s, 1\n", callRetSsa.Repr)
		return valSsa, nil
	}

	return SsaName{Repr: "<void ret>"}, nil
}

func irCaptureCall(ctx *IrCtx, callRetSsa SsaName, fnCall *t.NodeExprCall) (SsaName, error) {
	returnType := callReturnType(fnCall)
	errSsa := irSsaLocal(ctx)
	irWritef(ctx, "  %s = extractvalue ", errSsa.Repr)
	if err := irThrowingType(ctx, returnType); err != nil {
		return SsaName{}, err
	}
	irWritef(ctx, " %s, 0\n", callRetSsa.Repr)
	codeSsa, failedSsa := irSsaLocal(ctx), irSsaLocal(ctx)
	successLabel := irSsaName(ctx)
	irWritef(ctx, "  %s = extractvalue %%type.error %s, 1\n", codeSsa.Repr, errSsa.Repr)
	irWritef(ctx, "  %s = icmp ne i32 %s, 0\n", failedSsa.Repr, codeSsa.Repr)
	failureStore := irSsaName(ctx)
	irWritef(ctx, "  br i1 %s, label %%%s, label %%%s, !prof !9000\n", failedSsa.Repr, failureStore.Repr, successLabel.Repr)
	irWritef(ctx, "%s:\n", failureStore.Repr)
	irWritef(ctx, "  store %%type.error %s, ptr %s\n", errSsa.Repr, ctx.CapturedErrorSlot.Repr)
	irWritef(ctx, "  br label %%%s\n", ctx.ErrorFailureLabel.Repr)
	irWritef(ctx, "%s:\n", successLabel.Repr)
	if isVoidType(returnType) {
		return SsaName{Repr: "<void ret>"}, nil
	}
	valueSsa := irSsaLocal(ctx)
	irWritef(ctx, "  %s = extractvalue ", valueSsa.Repr)
	if err := irThrowingType(ctx, returnType); err != nil {
		return SsaName{}, err
	}
	irWritef(ctx, " %s, 1\n", callRetSsa.Repr)
	return valueSsa, nil
}

func irExpression(ctx *IrCtx, expectedType *t.NodeType, expr t.NodeExpr, topLevel bool) (SsaName, error) {
	switch ne := expr.(type) {
	case *t.NodeExprArray:
		return irExprArray(ctx, ne)
	case *t.NodeExprVarDefAssign:
		return irVarDefAssign(ctx, ne)
	case *t.NodeExprVarDef:
		return irVarDef(ctx, ne)
	case *t.NodeExprAssign:
		return irExprAssign(ctx, ne, ne.Left, ne.Right)
	case *t.NodeExprCall:
		return irExprFuncCall(ctx, ne, false, topLevel)
	case *t.NodeExprStructInit:
		return irExprStructInit(ctx, ne)
	case *t.NodeExprProtoView:
		return irExprProtoView(ctx, ne)
	// DEPRECATED
	/*case *t.NodeExprDestructor:
	return irExprDestructor(ctx, ne)*/
	case *t.NodeExprUnary:
		return irExprUnary(ctx, expectedType, ne)
	case *t.NodeExprTry:
		previousMode := ctx.ErrorMode
		ctx.ErrorMode = 1
		value, err := irExpression(ctx, expectedType, ne.Call, false)
		ctx.ErrorMode = previousMode
		return value, err
	case *t.NodeExprDestructureAssign:
		return irExprDestructureAssign(ctx, ne)
	case *t.NodeExprSubscript:
		return irExprSubscript(ctx, ne)
	case *t.NodeExprLit:
		return irExprLit(ctx, ne, expectedType)
	case *t.NodeExprSizeof:
		return irExprSizeof(ctx, ne)
	case *t.NodeExprAddrof:
		return irExprAddrof(ctx, ne)
	case *t.NodeExprMove:
		return irExpression(ctx, expectedType, ne.Expr, topLevel)
	case *t.NodeExprName:
		return irExprName(ctx, ne)
	case *t.NodeExprMemberAccess:
		return irExprMemberAccess(ctx, ne)
	case *t.NodeExprBinary:
		return irExprBinary(ctx, expectedType, ne)
	}
	return ssaName(""), fmt.Errorf("unsupported expression")
}

func irExprArray(ctx *IrCtx, expr *t.NodeExprArray) (SsaName, error) {
	length, e := irExpression(ctx, expr.LengthType, expr.Length, false)
	if e != nil {
		return SsaName{}, e
	}
	length, e = irPromoteSingleToNum(ctx, expr.LengthType, length, expr.Length.GetInferredType())
	if e != nil {
		return SsaName{}, e
	}
	data := irSsaLocal(ctx)
	irWritef(ctx, "  %s = alloca ", data.Repr)
	if e := irType(ctx, expr.ElemType); e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, ", i64 ")
	irPossibleLitSsa(ctx, length)
	irWrite(ctx, "\n")

	elemSize, e := irExprSizeof(ctx, &t.NodeExprSizeof{Type: expr.ElemType})
	if e != nil {
		return SsaName{}, e
	}
	totalSize := irSsaLocal(ctx)
	irWritef(ctx, "  %s = mul i64 ", totalSize.Repr)
	irPossibleLitSsa(ctx, elemSize)
	irWrite(ctx, ", ")
	irPossibleLitSsa(ctx, length)
	irWrite(ctx, "\n")
	irWritef(ctx, "  call void @llvm.memset.p0i8.i64(ptr %s, i8 0, i64 %s, i32 1, i1 0)\n", data.Repr, totalSize.Repr)
	for _, entry := range expr.Entries {
		value, err := irExpression(ctx, expr.ElemType, entry.Value, false)
		if err != nil {
			return SsaName{}, err
		}
		value, err = irCoerceNumeric(ctx, expr.ElemType, entry.Value, value)
		if err != nil {
			return SsaName{}, err
		}
		elem := irSsaLocal(ctx)
		irWritef(ctx, "  %s = getelementptr ", elem.Repr)
		if err := irType(ctx, expr.ElemType); err != nil {
			return SsaName{}, err
		}
		irWritef(ctx, ", ptr %s, i64 %d\n", data.Repr, entry.ResolvedIndex)
		irWrite(ctx, "  store ")
		if err := irType(ctx, expr.ElemType); err != nil {
			return SsaName{}, err
		}
		irWrite(ctx, " ")
		irPossibleLitSsa(ctx, value)
		irWritef(ctx, ", ptr %s\n", elem.Repr)
	}

	withPtr := irSsaLocal(ctx)
	result := irSsaLocal(ctx)
	irWritef(ctx, "  %s = insertvalue %%type.slice zeroinitializer, ptr %s, 0\n", withPtr.Repr, data.Repr)
	irWritef(ctx, "  %s = insertvalue %%type.slice %s, i64 ", result.Repr, withPtr.Repr)
	irPossibleLitSsa(ctx, length)
	irWrite(ctx, ", 1\n")
	return result, nil
}

func irExpressionLvalue(ctx *IrCtx, expr t.NodeExpr) (SsaName, error) {
	switch ne := expr.(type) {
	case *t.NodeExprName:
		return irExprNameLvalue(ctx, ne)
	case *t.NodeExprSubscript:
		return irExprSubscriptLvalue(ctx, ne)
	case *t.NodeExprMemberAccess:
		return irExprMemberAccessLvalue(ctx, ne)
	case *t.NodeExprUnary:
		if ne.Operator == t.KwAsterisk {
			if !ne.ProvenanceChecked {
				return SsaName{}, fmt.Errorf("refusing to lower pointer dereference at line %d, column %d without provenance analysis", ne.Tk.Pos.Line, ne.Tk.Pos.Col)
			}
			return irExpression(ctx, ne.Operand.GetInferredType(), ne.Operand, false)
		}
	}
	return ssaName(""), fmt.Errorf("expr not lvalue")
}
