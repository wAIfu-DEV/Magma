package llvmir

import (
	llvmfragments "Magma/src/llvm_fragments"
	magmatypes "Magma/src/magma_types"
	t "Magma/src/types"
	"bytes"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"
)

type ScopeBuilder struct {
	Struct  *bytes.Buffer
	StructM *sync.Mutex

	Global *bytes.Buffer
	Head   *bytes.Buffer
	Body   *bytes.Buffer
	Tail   *bytes.Buffer
}

type SsaName struct {
	Repr      string
	IsLiteral bool
}

func ssaName(name string) SsaName {
	return SsaName{Repr: name}
}

type IrCtx struct {
	Shared    *t.SharedState
	fCtx      *t.FileCtx
	bld       ScopeBuilder
	parentBld ScopeBuilder
	nextSsa   *int
	moduleIdx int
	CurrFunc  *t.NodeFuncDef

	CurrNestedScopeIdx int
	SeenNestedScopes   *int
	CurrDeferIdx       int
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

func makeNamedType(name string) *t.NodeType {
	return &t.NodeType{
		Throws: false,
		KindNode: &t.NodeTypeNamed{
			NameNode: &t.NodeNameSingle{Name: name},
		},
	}
}

func makeTypeFromKind(kind t.NodeTypeKind) *t.NodeType {
	return &t.NodeType{
		Throws:   false,
		KindNode: kind,
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

func extractBoxedPtrType(from *t.NodeType) *t.NodeType {
	switch n := from.KindNode.(type) {
	case *t.NodeTypePointer:
		return &t.NodeType{
			KindNode:   n.Kind,
			Throws:     from.Throws,
			Destructor: from.Destructor,
		}
	}
	return nil
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

	switch node.KindNode.(type) {
	case *t.NodeTypePointer:
		return true
	case *t.NodeTypeRfc:
		return true
	case *t.NodeTypeNamed:
		switch nn := node.KindNode.(*t.NodeTypeNamed).NameNode.(type) {
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
			_, ok := magmatypes.FloatTypes[nn.Name]
			return ok
		}
	}
	return false
}

func isSliceType(node *t.NodeType) bool {
	if node == nil {
		return false
	}

	switch node.KindNode.(type) {
	case *t.NodeTypeSlice:
		return true
	}
	return false
}

func getNumDesc(node *t.NodeType) magmatypes.NumberType {
	if node == nil {
		return magmatypes.NumberType{}
	}

	switch n := node.KindNode.(type) {
	case *t.NodeTypePointer:
		numType, ok := magmatypes.NumberTypes["ptr"]
		if !ok {
			return magmatypes.NumberType{}
		}
		return numType
	case *t.NodeTypeNamed:
		switch nn := n.NameNode.(type) {
		case *t.NodeNameSingle:
			numType, ok := magmatypes.NumberTypes[nn.Name]
			if !ok {
				return magmatypes.NumberType{}
			}
			return numType
		}
	}
	return magmatypes.NumberType{}
}

func isSameNumType(a *t.NodeType, b *t.NodeType) bool {
	return getNumDesc(a) == getNumDesc(b)
}

func irSsaName(ctx *IrCtx) SsaName {
	mdIdx := strconv.Itoa(ctx.moduleIdx)
	name := strconv.Itoa(*ctx.nextSsa)
	(*ctx.nextSsa)++
	return ssaName("." + mdIdx + name)
}

func irWrite(ctx *IrCtx, text string) {
	ctx.bld.Body.WriteString(text)
}

func irWritef(ctx *IrCtx, format string, a ...any) {
	fmt.Fprintf(ctx.bld.Body, format, a...)
}

func irWriteHd(ctx *IrCtx, text string) {
	ctx.bld.Head.WriteString(text)
}

func irWriteHdf(ctx *IrCtx, format string, a ...any) {
	fmt.Fprintf(ctx.bld.Head, format, a...)
}

func irWriteTl(ctx *IrCtx, text string) {
	ctx.bld.Tail.WriteString(text)
}

func irWriteTlf(ctx *IrCtx, format string, a ...any) {
	fmt.Fprintf(ctx.bld.Tail, format, a...)
}

func irWriteGl(ctx *IrCtx, text string) {
	ctx.bld.Global.WriteString(text)
}

func irWriteGlf(ctx *IrCtx, format string, a ...any) {
	fmt.Fprintf(ctx.bld.Global, format, a...)
}

func irVarDef(ctx *IrCtx, vd *t.NodeExprVarDef) (SsaName, error) {
	allocSsa := irNameSsa(ctx, vd.Name, false)

	if vd.Type.Destructor != nil {
		irWrite(ctx, "  ; has destructor\n")
	}

	irWriteHdf(ctx, "  %%%s = alloca ", allocSsa.Repr)

	cpy := *ctx
	cpy.bld.Body = ctx.bld.Head

	e := irType(&cpy, vd.Type)
	if e != nil {
		return ssaName(""), e
	}
	irWriteHd(ctx, "\n")

	irWrite(ctx, "  store ")
	e = irType(ctx, vd.Type)
	if e != nil {
		return ssaName(""), e
	}
	irWritef(ctx, " zeroinitializer, ptr %%%s\n", allocSsa.Repr)

	if isSliceType(vd.Type) {
		sliceT := vd.Type.KindNode.(*t.NodeTypeSlice)
		elemType := makeTypeFromKind(sliceT.ElemKind)

		if sliceT.HasSize {
			arrSsa := irSsaName(ctx)

			// making dud ctx to redirect name IR to head
			cpy := *ctx
			cpy.bld = ScopeBuilder{
				Global: ctx.bld.Global,
				Head:   ctx.bld.Head,
				Tail:   ctx.bld.Head,
				Body:   ctx.bld.Head,
			}

			irWritef(&cpy, "  %%%s = alloca [ %d x ", arrSsa.Repr, sliceT.Size)
			e = irType(&cpy, elemType)
			if e != nil {
				return SsaName{}, e
			}
			irWrite(&cpy, " ]\n")

			sizeofSsa, e := irExprSizeof(ctx, &t.NodeExprSizeof{
				Type: elemType,
			})

			if e != nil {
				return SsaName{}, e
			}

			totalSizeSsa := irSsaName(ctx)
			irWritef(ctx, "  %%%s = mul i64 ", totalSizeSsa.Repr)
			irPossibleLitSsa(ctx, sizeofSsa)
			irWritef(ctx, ", %d\n", sliceT.Size)

			irWritef(ctx, "  call void @llvm.memset.p0i8.i64(ptr %%%s, i8 0, i64 %%%s, i32 1, i1 0)\n", arrSsa.Repr, totalSizeSsa.Repr)

			fldPtrSsa := irSsaName(ctx)
			fldSizSsa := irSsaName(ctx)

			irWritef(ctx, "  %%%s = insertvalue %%type.slice zeroinitializer, ptr %%%s, 0\n", fldPtrSsa.Repr, arrSsa.Repr)
			irWritef(ctx, "  %%%s = insertvalue %%type.slice %%%s, i64 %d, 1\n", fldSizSsa.Repr, fldPtrSsa.Repr, sliceT.Size)
			irWritef(ctx, "  store %%type.slice %%%s, ptr %%%s\n", fldSizSsa.Repr, allocSsa.Repr)
		}
	}

	return allocSsa, nil
}

func irPossibleLitSsa(ctx *IrCtx, ssa SsaName) {
	if ssa.IsLiteral {
		irWrite(ctx, ssa.Repr)
	} else {
		irWritef(ctx, "%%%s", ssa.Repr)
	}
}

func irVarDefAssign(ctx *IrCtx, vda *t.NodeExprVarDefAssign) (SsaName, error) {
	assignSsa, e := irExpression(ctx, vda.VarDef.Type, vda.AssignExpr)
	if e != nil {
		return ssaName(""), e
	}

	allocSsa, e := irVarDef(ctx, vda.VarDef)
	if e != nil {
		return ssaName(""), e
	}

	if isNumberType(vda.GetInferredType()) {
		if !isSameNumType(vda.GetInferredType(), vda.AssignExpr.GetInferredType()) {
			if !assignSsa.IsLiteral {
				return SsaName{}, fmt.Errorf("implicit number cast is forbidden on assignment")
			}
		}
	}

	irWrite(ctx, "  store ")
	e = irType(ctx, vda.VarDef.Type)
	if e != nil {
		return ssaName(""), e
	}

	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, assignSsa)

	irWritef(ctx, ", ptr %%%s\n", allocSsa.Repr)
	return allocSsa, nil
}

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

func irExprCallFuncPtr(ctx *IrCtx, fnCall *t.NodeExprCall) (SsaName, error) {
	fnType := fnCall.FuncPtrType.KindNode.(*t.NodeTypeFunc)

	argsSsa := make([]SsaName, len(fnCall.Args))
	for i, expr := range fnCall.Args {
		exprSsa, e := irExpression(ctx, fnType.Args[i], expr)
		if e != nil {
			return ssaName(""), e
		}
		argsSsa[i] = exprSsa
	}

	fnPtrSsa, e := irExprName(ctx, fnCall.Callee.(*t.NodeExprName))
	bitCastPtr := irSsaName(ctx)

	irWritef(ctx, "  %%%s = bitcast ptr %%%s to ", bitCastPtr.Repr, fnPtrSsa.Repr)

	e = irFuncPtrType(ctx, fnCall.FuncPtrType.KindNode.(*t.NodeTypeFunc))
	if e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, "\n")

	ssa := irSsaName(ctx)

	isVoidRet := isVoidType(fnCall.InfType)

	if !isVoidRet || fnCall.InfType.Throws {
		irWritef(ctx, "  %%%s = ", ssa.Repr)
	} else {
		irWrite(ctx, "  ")
	}

	irWritef(ctx, "call ")

	e = irThrowingType(ctx, fnCall.InfType)
	if e != nil {
		return ssaName(""), e
	}

	irWritef(ctx, " %%%s(", bitCastPtr.Repr)

	bound := len(argsSsa)
	for i, ssa := range argsSsa {
		e = irType(ctx, fnCall.Args[i].GetInferredType())
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

	if isVoidRet && !fnCall.InfType.Throws {
		// TODO: Check and inforce that void ret calls HAVE to be statements
		// and cannot be in expressions
		return ssaName(""), nil
	}

	return ssa, nil
}

func irExprCallFuncNonPtr(ctx *IrCtx, fnCall *t.NodeExprCall) (SsaName, error) {
	argsSsa := make([]SsaName, len(fnCall.Args))
	for i, expr := range fnCall.Args {
		argT := fnCall.AssociatedFnDef.Class.ArgsNode.Args[i].TypeNode

		exprSsa, e := irExpression(ctx, argT, expr)
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

	ssa := irSsaName(ctx)
	isVoidRet := isVoidType(fnCall.InfType)

	if !isVoidRet || fnCall.InfType.Throws {
		irWritef(ctx, "  %%%s = ", ssa.Repr)
	} else {
		irWrite(ctx, "  ")
	}

	irWritef(ctx, "call ")

	e := irThrowingType(ctx, fnCall.InfType)
	if e != nil {
		return ssaName(""), e
	}

	irWrite(ctx, " @")

	switch expr := fnCall.Callee.(type) {
	case *t.NodeExprName:
		e := irName(ctx, expr.Name, true)
		if e != nil {
			return ssaName(""), e
		}
	default:
		irWrite(ctx, "<name>")
	}

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

	if isVoidRet && !fnCall.InfType.Throws {
		// TODO: Check and inforce that void ret calls HAVE to be statements
		// and cannot be in expressions
		return ssaName(""), nil
	}

	return ssa, nil
}

func irExprCallFuncMember(ctx *IrCtx, fnCall *t.NodeExprCall) (SsaName, error) {
	argsSsa := make([]SsaName, len(fnCall.Args))
	for i, expr := range fnCall.Args {
		argT := fnCall.AssociatedFnDef.Class.ArgsNode.Args[i+1].TypeNode

		exprSsa, e := irExpression(ctx, argT, expr)
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
	ownerSsa, e := irExprNameLvalue(ctx, fnCall.MemberOwnerName)
	argsSsa = slices.Insert(argsSsa, 0, ownerSsa)

	ssa := irSsaName(ctx)
	isVoidRet := isVoidType(fnCall.InfType)

	if !isVoidRet || fnCall.InfType.Throws {
		irWritef(ctx, "  %%%s = ", ssa.Repr)
	} else {
		irWrite(ctx, "  ")
	}

	irWritef(ctx, "call ")

	e = irThrowingType(ctx, fnCall.InfType)
	if e != nil {
		return ssaName(""), e
	}

	irWrite(ctx, " @")

	irWritef(ctx, "%s.", fnCall.MemberOwnerModule)

	e = irName(ctx, fnCall.AssociatedFnDef.Class.NameNode, false)
	if e != nil {
		return ssaName(""), e
	}

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

	if isVoidRet && !fnCall.InfType.Throws {
		// TODO: Check and enforce that void ret calls HAVE to be statements
		// and cannot be in expressions
		return ssaName(""), nil
	}

	return ssa, nil
}

func irExprDestructor(ctx *IrCtx, destructor *t.NodeExprDestructor) (SsaName, error) {
	irWrite(ctx, "  call void @")

	e := irName(ctx, destructor.Destructor.Class.NameNode, true)
	if e != nil {
		return ssaName(""), e
	}
	irWritef(ctx, "(ptr %%%s)\n", destructor.VarDef.Name.(*t.NodeNameSingle).Name)

	return SsaName{}, nil
}

func irExprFuncCall(ctx *IrCtx, fnCall *t.NodeExprCall, keepError bool) (SsaName, error) {
	var ssa = SsaName{}
	var e error

	if fnCall.IsFuncPointer {
		ssa, e = irExprCallFuncPtr(ctx, fnCall)
	} else if fnCall.IsMemberFunc {
		ssa, e = irExprCallFuncMember(ctx, fnCall)
	} else {
		ssa, e = irExprCallFuncNonPtr(ctx, fnCall)
	}

	if e != nil {
		return SsaName{}, e
	}

	if ssa.Repr == "" {
		return SsaName{}, nil
	}

	if fnCall.InfType.Throws && !keepError && !isVoidType(fnCall.InfType) {
		extractSsa := irSsaName(ctx)
		irWritef(ctx, "  %%%s = extractvalue ", extractSsa.Repr)

		e = irThrowingType(ctx, fnCall.InfType)
		if e != nil {
			return SsaName{}, e
		}

		irWritef(ctx, " %%%s, 1\n", ssa.Repr)
		ssa = extractSsa
	}
	return ssa, nil
}

func irExprLitStr(ctx *IrCtx, litStr *t.NodeExprLit) (SsaName, error) {
	constSsa := irSsaName(ctx)

	//strFieldSsa := irSsaName(ctx)
	//sizeFieldSsa := irSsaName(ctx)

	constLen := len(litStr.Value) + 1

	cleanStr := strings.ReplaceAll(litStr.Value, "\n", "\\0A")

	irWriteGlf(ctx, "@%s = private constant [%d x i8] c\"%s\\00\"\n", constSsa.Repr, constLen, cleanStr)

	//irWritef(ctx, "  %%%s = insertvalue %%type.str undef, ptr @%s, 0\n", strFieldSsa.Repr, constSsa.Repr)
	//irWritef(ctx, "  %%%s = insertvalue %%type.str %%%s, i64 %d, 1\n", sizeFieldSsa.Repr, strFieldSsa.Repr, constLen-1)

	litSsa := SsaName{
		Repr:      fmt.Sprintf("{ ptr @%s, i64 %d }", constSsa.Repr, constLen-1),
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

func irExprLit(ctx *IrCtx, lit *t.NodeExprLit, expectedType *t.NodeType) (SsaName, error) {
	switch lit.LitType {
	case t.TokLitStr:
		return irExprLitStr(ctx, lit)
	case t.TokLitNum:
		return irExprLitNum(ctx, lit)
	case t.TokLitBool:
		return irExprLitBool(ctx, lit)
	}
	return ssaName(""), nil
}

func irExprSizeof(ctx *IrCtx, sz *t.NodeExprSizeof) (SsaName, error) {
	if sz.Type == nil {
		return SsaName{}, fmt.Errorf("sizeof: missing type")
	}

	if isVoidType(sz.Type) {
		return SsaName{Repr: "0", IsLiteral: true}, nil
	}

	switch n := sz.Type.KindNode.(type) {
	case *t.NodeTypePointer, *t.NodeTypeRfc, *t.NodeTypeFunc:
		ptrBytes := magmatypes.NumberTypes["ptr"].ByteSize / 8
		return SsaName{Repr: strconv.Itoa(ptrBytes), IsLiteral: true}, nil
	case *t.NodeTypeNamed:
		if nn, ok := n.NameNode.(*t.NodeNameSingle); ok {
			if nn.Name == "ptr" {
				ptrBytes := magmatypes.NumberTypes["ptr"].ByteSize / 8
				return SsaName{Repr: strconv.Itoa(ptrBytes), IsLiteral: true}, nil
			}
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

	gepSsa := irSsaName(ctx)
	irWritef(ctx, "  %%%s = getelementptr %s, ptr null, i64 1\n", gepSsa.Repr, typeStr)

	sizeSsa := irSsaName(ctx)
	irWritef(ctx, "  %%%s = ptrtoint ptr %%%s to i64\n", sizeSsa.Repr, gepSsa.Repr)
	return sizeSsa, nil
}

func irMemberAccess(ctx *IrCtx, fromType *t.NodeType, fromSsa SsaName, fieldNb int, fieldType *t.NodeType, isPtrDeref bool) (SsaName, error) {
	fieldSsa := irSsaName(ctx)

	if isPtrDeref {
		loadSsa := irSsaName(ctx)

		switch n := fromType.KindNode.(type) {
		case *t.NodeTypePointer:
			fromType = &t.NodeType{
				KindNode: n.Kind,
			}
		}

		irWritef(ctx, "  %%%s = load ", loadSsa.Repr)
		e := irType(ctx, fromType)
		if e != nil {
			return SsaName{}, e
		}
		irWritef(ctx, ", ptr %%%s\n", fromSsa.Repr)
		fromSsa = loadSsa
	}

	// TODO: Possible lit ssa? maybe not
	irWritef(ctx, "  %%%s = extractvalue ", fieldSsa.Repr)
	e := irType(ctx, fromType)
	if e != nil {
		return SsaName{}, e
	}

	irWritef(ctx, " %%%s, %d\n", fromSsa.Repr, fieldNb)
	return fieldSsa, nil
}

func irMemberAddress(ctx *IrCtx, basePtr SsaName, baseType *t.NodeType, fieldIndex int) (SsaName, error) {
	fieldPtr := irSsaName(ctx)

	irWritef(ctx, "  %%%s = getelementptr ", fieldPtr.Repr)

	if isPointerType(baseType) {
		e := irType(ctx, extractBoxedPtrType(baseType))
		if e != nil {
			return SsaName{}, e
		}
	} else {
		e := irType(ctx, baseType)
		if e != nil {
			return SsaName{}, e
		}
	}

	irWritef(ctx, ", ptr %%%s, i32 0, i32 %d\n", basePtr.Repr, fieldIndex)
	return fieldPtr, nil
}

func irExprName(ctx *IrCtx, nameExpr *t.NodeExprName) (SsaName, error) {
	ptrSsa := irNameSsa(ctx, nameExpr.Name, false)
	ssa := irSsaName(ctx)

	var typeNd *t.NodeType = nil
	isMemberAccess := false

	isFuncName := false
	var fnDef *t.NodeFuncDef = nil

	switch n := nameExpr.Name.(type) {
	case *t.NodeNameComposite:
		isMemberAccess = true
		ptrSsa = irNameSsa(ctx, &t.NodeNameSingle{
			Name: n.Parts[0],
		}, false)
	}

	switch n := nameExpr.AssociatedNode.(type) {
	case *t.NodeExprVarDef:
		typeNd = n.Type
	case *t.NodeExprVarDefAssign:
		typeNd = n.VarDef.Type
	case *t.NodeFuncDef:
		fnDef = n
		isFuncName = true
		typeNd = makeFuncPtrTypeFromDef(n)
	default:
		isMemberAccess = false
	}

	if nameExpr.IsSsa && !isMemberAccess {
		return irNameSsa(ctx, nameExpr.Name, false), nil
	} else if nameExpr.IsSsa {
		ssa = ptrSsa
	} else if isFuncName {
		irWritef(ctx, "  %%%s = bitcast ptr @", ssa.Repr)

		e := irName(ctx, fnDef.Class.NameNode, true)
		if e != nil {
			return ssaName(""), e
		}

		irWrite(ctx, " to ")

		e = irFuncPtrType(ctx, typeNd.KindNode.(*t.NodeTypeFunc))
		if e != nil {
			return ssaName(""), e
		}
		irWrite(ctx, "\n")
	} else {
		irWritef(ctx, "  %%%s = load ", ssa.Repr)

		e := irType(ctx, typeNd)
		if e != nil {
			return ssaName(""), e
		}

		irWritef(ctx, ", ptr %%%s\n", ptrSsa.Repr)
	}

	if isMemberAccess {
		lastSsa := ssa
		if len(nameExpr.MemberAccesses) == 0 {
			return SsaName{}, fmt.Errorf("member access but no member access history")
		}

		fromType := typeNd

		for _, m := range nameExpr.MemberAccesses {
			fieldSsa, e := irMemberAccess(ctx, fromType, lastSsa, m.FieldNb, m.Type, m.PtrDeref)
			if e != nil {
				return SsaName{}, e
			}
			lastSsa = fieldSsa
			fromType = m.Type
		}
		return lastSsa, nil
	}
	return ssa, nil
}

func irExprNameLvalue(ctx *IrCtx, nameExpr *t.NodeExprName) (SsaName, error) {
	basePtr := irNameSsa(ctx, nameExpr.Name, false)

	switch n := nameExpr.Name.(type) {
	case *t.NodeNameComposite:
		basePtr = irNameSsa(ctx, &t.NodeNameSingle{
			Name: n.Parts[0],
		}, false)
	}

	if len(nameExpr.MemberAccesses) == 0 {
		return basePtr, nil
	}

	curPtr := basePtr
	var curType *t.NodeType = nil

	switch n := nameExpr.AssociatedNode.(type) {
	case *t.NodeExprVarDef:
		curType = n.Type
	case *t.NodeExprVarDefAssign:
		curType = n.VarDef.Type
	}

	for _, m := range nameExpr.MemberAccesses {
		nextPtr, err := irMemberAddress(ctx, curPtr, curType, m.FieldNb)
		if err != nil {
			return ssaName(""), err
		}

		curPtr = nextPtr
		curType = m.Type
	}

	return curPtr, nil
}

func irExprSubscript(ctx *IrCtx, subs *t.NodeExprSubscript) (SsaName, error) {
	subsExpr, e := irExpression(ctx, makeNamedType("i64"), subs.Expr)
	if e != nil {
		return SsaName{}, e
	}

	switch subs.BoxType.KindNode.(type) {
	case *t.NodeTypeSlice:
		var loadedTarget SsaName
		if subs.IsTargetSsa {
			loadedTarget, e = irExpression(ctx, makeNamedType("slice"), subs.Target)
			if e != nil {
				return SsaName{}, e
			}
		} else {
			targetPtr, e := irExpressionLvalue(ctx, subs.Target)
			if e != nil {
				return SsaName{}, e
			}

			loadedTarget = irSsaName(ctx)
			irWritef(ctx, "  %%%s = load ", loadedTarget.Repr)
			e = irType(ctx, subs.BoxType)
			if e != nil {
				return SsaName{}, e
			}
			irWritef(ctx, ", ptr %%%s\n", targetPtr.Repr)
		}
		// extract ptr from struct first
		extracted := irSsaName(ctx)
		irWritef(ctx, "  %%%s = extractvalue %%type.slice %%%s, 0\n", extracted.Repr, loadedTarget.Repr)
		return irExprSubscriptPtr(ctx, subs, extracted, subsExpr)
	case *t.NodeTypePointer:
		var loadedTarget SsaName
		if subs.IsTargetSsa {
			loadedTarget, e = irExpression(ctx, nil, subs.Target)
			if e != nil {
				return SsaName{}, e
			}
		} else {
			targetPtr, e := irExpressionLvalue(ctx, subs.Target)
			if e != nil {
				return SsaName{}, e
			}

			loadedTarget = irSsaName(ctx)
			irWritef(ctx, "  %%%s = load ", loadedTarget.Repr)
			e = irType(ctx, subs.BoxType)
			if e != nil {
				return SsaName{}, e
			}
			irWritef(ctx, ", ptr %%%s\n", targetPtr.Repr)
		}
		return irExprSubscriptPtr(ctx, subs, loadedTarget, subsExpr)
	case *t.NodeTypeRfc:
		var loadedTarget SsaName
		if subs.IsTargetSsa {
			loadedTarget, e = irExpression(ctx, nil, subs.Target)
			if e != nil {
				return SsaName{}, e
			}
		} else {
			targetPtr, e := irExpressionLvalue(ctx, subs.Target)
			if e != nil {
				return SsaName{}, e
			}

			loadedTarget = irSsaName(ctx)
			irWritef(ctx, "  %%%s = load ", loadedTarget.Repr)
			e = irType(ctx, subs.BoxType)
			if e != nil {
				return SsaName{}, e
			}
			irWritef(ctx, ", ptr %%%s\n", targetPtr.Repr)
		}
		return irExprSubscriptPtr(ctx, subs, loadedTarget, subsExpr)
	}
	return SsaName{}, fmt.Errorf("invalid box type in subscript expression lowering")
}

func irExprSubscriptPtr(ctx *IrCtx, subs *t.NodeExprSubscript, targetSsa SsaName, subsSsa SsaName) (SsaName, error) {
	elemPtr := irSsaName(ctx)
	loadedElem := irSsaName(ctx)

	irWritef(ctx, "  %%%s = getelementptr ", elemPtr.Repr)

	e := irType(ctx, subs.ElemType)
	if e != nil {
		return SsaName{}, e
	}

	irWritef(ctx, ", ptr %%%s, i64 ", targetSsa.Repr)

	irPossibleLitSsa(ctx, subsSsa)
	irWrite(ctx, "\n")

	irWritef(ctx, "  %%%s = load ", loadedElem.Repr)

	e = irType(ctx, subs.ElemType)
	if e != nil {
		return SsaName{}, e
	}

	irWritef(ctx, ", ptr %%%s\n", elemPtr.Repr)
	return loadedElem, nil
}

func irExprSubscriptLvalue(ctx *IrCtx, subs *t.NodeExprSubscript) (SsaName, error) {
	subsExpr, e := irExpression(ctx, makeNamedType("i64"), subs.Expr)
	if e != nil {
		return SsaName{}, e
	}

	var targetPtrSsa SsaName

	switch subs.BoxType.KindNode.(type) {
	case *t.NodeTypeSlice:
		var loadedTarget SsaName
		if subs.IsTargetSsa {
			loadedTarget, e = irExpression(ctx, makeNamedType("slice"), subs.Target)
			if e != nil {
				return SsaName{}, e
			}
		} else {
			targetPtr, e := irExpressionLvalue(ctx, subs.Target)
			if e != nil {
				return SsaName{}, e
			}

			loadedTarget = irSsaName(ctx)
			irWritef(ctx, "  %%%s = load ", loadedTarget.Repr)
			e = irType(ctx, subs.BoxType)
			if e != nil {
				return SsaName{}, e
			}
			irWritef(ctx, ", ptr %%%s\n", targetPtr.Repr)
		}

		// extract ptr from struct first
		extracted := irSsaName(ctx)
		irWritef(ctx, "  %%%s = extractvalue %%type.slice %%%s, 0\n", extracted.Repr, loadedTarget.Repr)
		targetPtrSsa = extracted
	case *t.NodeTypePointer, *t.NodeTypeRfc:
		if subs.IsTargetSsa {
			targetPtrSsa, e = irExpression(ctx, nil, subs.Target)
			if e != nil {
				return SsaName{}, e
			}
		} else {
			targetPtr, e := irExpressionLvalue(ctx, subs.Target)
			if e != nil {
				return SsaName{}, e
			}

			targetPtrSsa = irSsaName(ctx)
			irWritef(ctx, "  %%%s = load ", targetPtrSsa.Repr)
			e = irType(ctx, subs.BoxType)
			if e != nil {
				return SsaName{}, e
			}
			irWritef(ctx, ", ptr %%%s\n", targetPtr.Repr)
		}
	default:
		return SsaName{}, fmt.Errorf("invalid box type in subscript lvalue lowering")
	}

	elemPtr := irSsaName(ctx)
	irWritef(ctx, "  %%%s = getelementptr ", elemPtr.Repr)

	e = irType(ctx, subs.ElemType)
	if e != nil {
		return SsaName{}, e
	}

	irWritef(ctx, ", ptr %%%s, i64 ", targetPtrSsa.Repr)
	irPossibleLitSsa(ctx, subsExpr)
	irWrite(ctx, "\n")

	return elemPtr, nil
}

func irExprAssign(ctx *IrCtx, lhs t.NodeExpr, rhs t.NodeExpr) (SsaName, error) {
	lhsPtr, e := irExpressionLvalue(ctx, lhs)
	if e != nil {
		return SsaName{}, e
	}

	rhsVal, e := irExpression(ctx, lhs.GetInferredType(), rhs)
	if e != nil {
		return SsaName{}, e
	}

	if isNumberType(lhs.GetInferredType()) {
		if !isSameNumType(lhs.GetInferredType(), rhs.GetInferredType()) {
			if !rhsVal.IsLiteral {
				return SsaName{}, fmt.Errorf("implicit number cast is forbidden on assignment")
			}
		}
	}

	irWrite(ctx, "  store ")

	e = irType(ctx, lhs.GetInferredType())
	if e != nil {
		return SsaName{}, e
	}

	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, rhsVal)
	irWritef(ctx, ", ptr %%%s\n", lhsPtr.Repr)

	ssa := irSsaName(ctx)
	irWritef(ctx, "  %%%s = load ", ssa.Repr)

	e = irType(ctx, lhs.GetInferredType())
	if e != nil {
		return SsaName{}, e
	}

	irWritef(ctx, ", ptr %%%s\n", lhsPtr.Repr)
	return lhsPtr, nil
}

func irExtendFlt(ctx *IrCtx, valSsa SsaName, prevType *t.NodeType, newType *t.NodeType) (SsaName, error) {
	outSsa := irSsaName(ctx)
	irWritef(ctx, "  %%%s = fpext ", outSsa.Repr)

	e := irType(ctx, prevType)
	if e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, valSsa)
	irWrite(ctx, " to ")

	e = irType(ctx, newType)
	if e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, "\n")
	return outSsa, nil
}

func irExtendInt(ctx *IrCtx, valSsa SsaName, signed bool, prevType *t.NodeType, newType *t.NodeType) (SsaName, error) {
	outSsa := irSsaName(ctx)

	if signed {
		irWritef(ctx, "  %%%s = sext ", outSsa.Repr)
	} else {
		irWritef(ctx, "  %%%s = zext ", outSsa.Repr)
	}

	e := irType(ctx, prevType)
	if e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, valSsa)
	irWrite(ctx, " to ")

	e = irType(ctx, newType)
	if e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, "\n")
	return outSsa, nil
}

func irTruncInt(ctx *IrCtx, valSsa SsaName, prevType *t.NodeType, newType *t.NodeType) (SsaName, error) {
	outSsa := irSsaName(ctx)

	irWritef(ctx, "  %%%s = trunc ", outSsa.Repr)

	e := irType(ctx, prevType)
	if e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, valSsa)
	irWrite(ctx, " to ")

	e = irType(ctx, newType)
	if e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, "\n")
	return outSsa, nil
}

func irTruncFlt(ctx *IrCtx, valSsa SsaName, prevType *t.NodeType, newType *t.NodeType) (SsaName, error) {
	outSsa := irSsaName(ctx)

	irWritef(ctx, "  %%%s = fptrunc ", outSsa.Repr)

	e := irType(ctx, prevType)
	if e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, valSsa)
	irWrite(ctx, " to ")

	e = irType(ctx, newType)
	if e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, "\n")
	return outSsa, nil
}

func irPtrToInt(ctx *IrCtx, valSsa SsaName) SsaName {
	ssa := irSsaName(ctx)
	irWritef(ctx, "  %%%s = ptrtoint ptr %%%s to i64", ssa.Repr, valSsa.Repr)
	return ssa
}

func irIntToPtr(ctx *IrCtx, valSsa SsaName) SsaName {
	ssa := irSsaName(ctx)
	irWritef(ctx, "  %%%s = inttoptr i64 %%%s to ptr", ssa.Repr, valSsa.Repr)
	return ssa
}

func irIntToFloat(ctx *IrCtx, valSsa SsaName, numType *t.NodeType, toType *t.NodeType) (SsaName, error) {
	numDesc := getNumDesc(numType)

	// here target is guaranteed to be integer type
	outSsa := irSsaName(ctx)
	irWritef(ctx, "  %%%s = ", outSsa.Repr)

	if numDesc.IsSigned {
		irWrite(ctx, "sitofp ")
	} else {
		irWrite(ctx, "uitofp ")
	}

	e := irType(ctx, numType)
	if e != nil {
		return SsaName{}, e
	}

	irPossibleLitSsa(ctx, valSsa)
	irWrite(ctx, " to ")

	e = irType(ctx, toType)
	if e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, "\n")
	return outSsa, nil
}

func irFloatToInt(ctx *IrCtx, valSsa SsaName, numType *t.NodeType, toType *t.NodeType) (SsaName, error) {
	numDesc := getNumDesc(numType)

	// here target is guaranteed to be integer type
	outSsa := irSsaName(ctx)
	irWritef(ctx, "  %%%s = ", outSsa.Repr)

	if numDesc.IsSigned {
		irWrite(ctx, "fptosi ")
	} else {
		irWrite(ctx, "fptoui ")
	}

	e := irType(ctx, numType)
	if e != nil {
		return SsaName{}, e
	}

	irPossibleLitSsa(ctx, valSsa)
	irWrite(ctx, " to ")

	e = irType(ctx, toType)
	if e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, "\n")
	return outSsa, nil
}

func irPromoteSingleToNum(ctx *IrCtx, expectedType *t.NodeType, ssa SsaName, fromType *t.NodeType) (SsaName, error) {
	if ssa.IsLiteral {
		fromType = expectedType
	}

	fromNum := getNumDesc(fromType)
	expectedNum := getNumDesc(expectedType)

	// with floating point ops always return type of largest (byte size) float
	if fromNum.IsFloat && expectedNum.IsFloat {
		if fromNum.ByteSize == expectedNum.ByteSize {
			// no need for promotion
			return ssa, nil
		} else if fromNum.ByteSize > expectedNum.ByteSize {
			return irTruncFlt(ctx, ssa, fromType, expectedType)
		} else {
			outSsa, e := irExtendFlt(ctx, ssa, fromType, expectedType)
			return outSsa, e
		}
	}

	if fromNum.IsFloat {
		return irIntToFloat(
			ctx, ssa,
			fromType,
			expectedType,
		)
	}

	// integers
	if fromNum.ByteSize > expectedNum.ByteSize {
		return irTruncInt(ctx, ssa, fromType, expectedType)
	} else if fromNum.ByteSize < expectedNum.ByteSize {
		return irExtendInt(ctx, ssa, fromNum.IsSigned, fromType, expectedType)
	} else {
		return ssa, nil
	}

	//return SsaName{}, SsaName{}, nil, fmt.Errorf("unhandled type in numerical promotion")
}

func irPromoteToNum(ctx *IrCtx, expectedType *t.NodeType, leftSsa SsaName, leftType *t.NodeType, rightSsa SsaName, rightType *t.NodeType) (SsaName, SsaName, *t.NodeType, error) {
	if leftSsa.IsLiteral {
		leftType = expectedType
	}

	if rightSsa.IsLiteral {
		rightType = expectedType
	}

	leftNum := getNumDesc(leftType)
	rightNum := getNumDesc(rightType)

	// with floating point ops always return type of largest (byte size) float
	if leftNum.IsFloat && rightNum.IsFloat {
		if leftNum.ByteSize == rightNum.ByteSize {
			// no need for promotion
			return leftSsa, rightSsa, leftType, nil
		} else if leftNum.ByteSize > rightNum.ByteSize {
			// extend rhs float
			outSsa, e := irExtendFlt(ctx, rightSsa, rightType, leftType)
			return leftSsa, outSsa, leftType, e
		} else {
			outSsa, e := irExtendFlt(ctx, leftSsa, leftType, rightType)
			return outSsa, rightSsa, rightType, e
		}
	}

	// between int / flt result is alway flt
	if leftNum.IsFloat || rightNum.IsFloat {
		var resType *t.NodeType = nil
		var target SsaName = SsaName{}
		var targetType *t.NodeType = nil

		if leftNum.IsFloat {
			resType = leftType
			target = rightSsa
			targetType = rightType
		} else {
			resType = rightType
			target = leftSsa
			targetType = leftType
		}

		outSsa, e := irIntToFloat(
			ctx, target,
			targetType,
			resType,
		)

		if e != nil {
			return SsaName{}, SsaName{}, nil, e
		}

		if leftNum.IsFloat {
			return leftSsa, outSsa, leftType, nil
		} else {
			return outSsa, rightSsa, rightType, nil
		}
	}

	// integers
	if leftNum.ByteSize > rightNum.ByteSize {
		// extend rhs int
		outSsa, e := irExtendInt(ctx, rightSsa, rightNum.IsSigned, rightType, leftType)
		return leftSsa, outSsa, leftType, e
	} else if leftNum.ByteSize < rightNum.ByteSize {
		outSsa, e := irExtendInt(ctx, leftSsa, leftNum.IsSigned, leftType, rightType)
		return outSsa, rightSsa, rightType, e
	} else {
		return leftSsa, rightSsa, leftType, nil
	}

	//return SsaName{}, SsaName{}, nil, fmt.Errorf("unhandled type in numerical promotion")
}

func irExprUnary(ctx *IrCtx, expectedType *t.NodeType, unaryExpr *t.NodeExprUnary) (SsaName, error) {
	operandType := unaryExpr.Operand.GetInferredType()
	operandSsa, e := irExpression(ctx, operandType, unaryExpr.Operand)
	if e != nil {
		return SsaName{}, e
	}

	switch unaryExpr.Operator {
	case t.KwTilde:
		if isPointerType(operandType) {
			return SsaName{}, fmt.Errorf("bitwise not (~) is not supported for pointer types")
		}
		if isFloatType(operandType) {
			return SsaName{}, fmt.Errorf("bitwise not (~) is not supported for floating-point types")
		}
		if !isBoolType(operandType) && !isNumberType(operandType) {
			return SsaName{}, fmt.Errorf("bitwise not (~) requires an integer or bool operand")
		}

		resSsa := irSsaName(ctx)
		irWritef(ctx, "  %%%s = xor ", resSsa.Repr)
		e = irType(ctx, operandType)
		if e != nil {
			return SsaName{}, e
		}
		irWrite(ctx, " ")
		irPossibleLitSsa(ctx, operandSsa)
		irWrite(ctx, ", -1\n")
		return resSsa, nil
	default:
		return SsaName{}, fmt.Errorf("unsupported unary expression")
	}
}

func irExprBinBitwise(ctx *IrCtx, expectedType *t.NodeType, binaryExpr *t.NodeExprBinary) (SsaName, error) {
	leftType := binaryExpr.Left.GetInferredType()
	rightType := binaryExpr.Right.GetInferredType()

	leftSsa, e := irExpression(ctx, leftType, binaryExpr.Left)
	if e != nil {
		return SsaName{}, e
	}

	rightSsa, e := irExpression(ctx, rightType, binaryExpr.Right)
	if e != nil {
		return SsaName{}, e
	}

	if isPointerType(leftType) || isPointerType(rightType) {
		return SsaName{}, fmt.Errorf("bitwise operators are not supported for pointer types")
	}

	resultType := leftType

	if isBoolType(leftType) || isBoolType(rightType) {
		if !isBoolType(leftType) || !isBoolType(rightType) {
			return SsaName{}, fmt.Errorf("bitwise operators on bool require both operands to be bool")
		}
	} else {
		if !isNumberType(leftType) || !isNumberType(rightType) {
			return SsaName{}, fmt.Errorf("bitwise operators require integer operands")
		}
		if isFloatType(leftType) || isFloatType(rightType) {
			return SsaName{}, fmt.Errorf("bitwise operators are not supported for floating-point types")
		}

		if expectedType == nil {
			expectedType = binaryExpr.InfType
		}
		if expectedType == nil {
			expectedType = leftType
		}

		leftSsa, rightSsa, resultType, e = irPromoteToNum(ctx, expectedType, leftSsa, leftType, rightSsa, rightType)
		if e != nil {
			return SsaName{}, e
		}
	}

	resSsa := irSsaName(ctx)
	irWritef(ctx, "  %%%s = ", resSsa.Repr)

	switch binaryExpr.Operator {
	case t.KwAmpersand:
		irWrite(ctx, "and ")
	case t.KwPipe:
		irWrite(ctx, "or ")
	case t.KwCaret:
		irWrite(ctx, "xor ")
	default:
		return SsaName{}, fmt.Errorf("unexpected bitwise operator")
	}

	e = irType(ctx, resultType)
	if e != nil {
		return SsaName{}, e
	}

	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, leftSsa)
	irWrite(ctx, ", ")
	irPossibleLitSsa(ctx, rightSsa)
	irWrite(ctx, "\n")
	return resSsa, nil
}

func irExprBinShift(ctx *IrCtx, expectedType *t.NodeType, binaryExpr *t.NodeExprBinary) (SsaName, error) {
	leftType := binaryExpr.Left.GetInferredType()
	rightType := binaryExpr.Right.GetInferredType()

	leftSsa, e := irExpression(ctx, leftType, binaryExpr.Left)
	if e != nil {
		return SsaName{}, e
	}

	rightSsa, e := irExpression(ctx, rightType, binaryExpr.Right)
	if e != nil {
		return SsaName{}, e
	}

	if isPointerType(leftType) || isPointerType(rightType) {
		return SsaName{}, fmt.Errorf("shift operators are not supported for pointer types")
	}
	if !isNumberType(leftType) || !isNumberType(rightType) {
		return SsaName{}, fmt.Errorf("shift operators require integer operands")
	}
	if isFloatType(leftType) || isFloatType(rightType) {
		return SsaName{}, fmt.Errorf("shift operators are not supported for floating-point types")
	}

	if expectedType == nil {
		expectedType = binaryExpr.InfType
	}
	if expectedType == nil {
		expectedType = leftType
	}

	leftSsa, rightSsa, leftType, e = irPromoteToNum(ctx, expectedType, leftSsa, leftType, rightSsa, rightType)
	if e != nil {
		return SsaName{}, e
	}

	resSsa := irSsaName(ctx)
	irWritef(ctx, "  %%%s = ", resSsa.Repr)

	switch binaryExpr.Operator {
	case t.KwShiftLeft:
		irWrite(ctx, "shl ")
	case t.KwShiftRight:
		if getNumDesc(leftType).IsSigned {
			irWrite(ctx, "ashr ")
		} else {
			irWrite(ctx, "lshr ")
		}
	default:
		return SsaName{}, fmt.Errorf("unexpected shift operator")
	}

	e = irType(ctx, leftType)
	if e != nil {
		return SsaName{}, e
	}

	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, leftSsa)
	irWrite(ctx, ", ")
	irPossibleLitSsa(ctx, rightSsa)
	irWrite(ctx, "\n")
	return resSsa, nil
}

func irExprBinLogical(ctx *IrCtx, binaryExpr *t.NodeExprBinary) (SsaName, error) {
	boolType := makeNamedType("bool")

	leftSsa, e := irExpression(ctx, boolType, binaryExpr.Left)
	if e != nil {
		return SsaName{}, e
	}

	// Allocate the result in the function entry (head) so we can avoid `phi`.
	resultPtr := irSsaName(ctx)
	cpy := *ctx
	cpy.bld.Body = ctx.bld.Head
	irWritef(&cpy, "  %%%s = alloca i1\n", resultPtr.Repr)

	rhsLabel := irSsaName(ctx)
	shortCircuitLabel := irSsaName(ctx)
	endLabel := irSsaName(ctx)

	irWrite(ctx, "  br i1 ")
	irPossibleLitSsa(ctx, leftSsa)

	switch binaryExpr.Operator {
	case t.KwAndAnd:
		irWritef(ctx, ", label %%%s, label %%%s\n", rhsLabel.Repr, shortCircuitLabel.Repr)
	case t.KwOrOr:
		irWritef(ctx, ", label %%%s, label %%%s\n", shortCircuitLabel.Repr, rhsLabel.Repr)
	default:
		return SsaName{}, fmt.Errorf("unexpected logical operator")
	}

	irWritef(ctx, "%s:\n", rhsLabel.Repr)

	rightSsa, e := irExpression(ctx, boolType, binaryExpr.Right)
	if e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, "  store i1 ")
	irPossibleLitSsa(ctx, rightSsa)
	irWritef(ctx, ", ptr %%%s\n", resultPtr.Repr)
	irWritef(ctx, "  br label %%%s\n", endLabel.Repr)

	irWritef(ctx, "%s:\n", shortCircuitLabel.Repr)
	irWrite(ctx, "  store i1 ")
	switch binaryExpr.Operator {
	case t.KwAndAnd:
		irWrite(ctx, "0")
	case t.KwOrOr:
		irWrite(ctx, "1")
	default:
		return SsaName{}, fmt.Errorf("unexpected logical operator")
	}
	irWritef(ctx, ", ptr %%%s\n", resultPtr.Repr)
	irWritef(ctx, "  br label %%%s\n", endLabel.Repr)

	irWritef(ctx, "%s:\n", endLabel.Repr)

	resSsa := irSsaName(ctx)
	irWritef(ctx, "  %%%s = load i1, ptr %%%s\n", resSsa.Repr, resultPtr.Repr)
	return resSsa, nil
}

/*
func irImplicitCastNum(ctx *IrCtx, target SsaName, fromType *t.NodeType, toType *t.NodeType) (SsaName, error) {
	if !isNumberType(fromType) || !isNumberType(toType) {
		return SsaName{}, fmt.Errorf("failure to implicit cast number as both types are not numerical")
	}

	fromDesc := getNumDesc(fromType)
	toDesc := getNumDesc(toType)

	if fromDesc.IsFloat == toDesc.IsFloat {
		if fromDesc.ByteSize == toDesc.ByteSize {
			return target, nil
		}
	}

	if (!fromDesc.IsFloat) && (!toDesc.IsFloat) {
		if fromDesc.ByteSize == toDesc.ByteSize {
			return target, nil
		}
	}

	if fromDesc.IsFloat != toDesc.IsFloat {

	}
}*/

func irExprBinAddition(ctx *IrCtx, expectedType *t.NodeType, binaryExpr *t.NodeExprBinary) (SsaName, error) {
	leftType := binaryExpr.Left.GetInferredType()
	rightType := binaryExpr.Right.GetInferredType()

	lhsSsa, e := irExpression(ctx, leftType, binaryExpr.Left)
	if e != nil {
		return SsaName{}, e
	}

	rhsSsa, e := irExpression(ctx, rightType, binaryExpr.Right)
	if e != nil {
		return SsaName{}, e
	}

	/*
		if isPointerType(leftType) {
			lhsSsa = irPtrToInt(ctx, lhsSsa)
			leftType = makeNamedType("u64")
		}

		if isPointerType(rightType) {
			rhsSsa = irPtrToInt(ctx, rhsSsa)
			rightType = makeNamedType("u64")
		}*/

	lhs, rhs, newType, e := irPromoteToNum(
		ctx,
		expectedType,
		lhsSsa,
		leftType,
		rhsSsa,
		rightType,
	)

	if e != nil {
		return SsaName{}, e
	}

	resSsa := irSsaName(ctx)
	irWritef(ctx, "  %%%s = ", resSsa.Repr)

	if isFloatType(newType) {
		irWrite(ctx, "fadd ")
	} else {
		irWrite(ctx, "add ")
	}

	e = irType(ctx, newType)
	if e != nil {
		return SsaName{}, e
	}

	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, lhs)
	irWrite(ctx, ", ")
	irPossibleLitSsa(ctx, rhs)
	irWrite(ctx, "\n")

	/*
		if isPointerType(expectedType) {
			tpDesc := getNumDesc(newType)
			if tpDesc.ByteSize > 64 {

			}

			resSsa = irIntToPtr(ctx, resSsa)
			rightType = makeNamedType("u64")
		}*/

	return resSsa, nil
}

func irExprBinSubstraction(ctx *IrCtx, expectedType *t.NodeType, binaryExpr *t.NodeExprBinary) (SsaName, error) {
	lhsSsa, e := irExpression(ctx, binaryExpr.Left.GetInferredType(), binaryExpr.Left)
	if e != nil {
		return SsaName{}, e
	}

	rhsSsa, e := irExpression(ctx, binaryExpr.Right.GetInferredType(), binaryExpr.Right)
	if e != nil {
		return SsaName{}, e
	}

	lhs, rhs, newType, e := irPromoteToNum(
		ctx,
		expectedType,
		lhsSsa,
		binaryExpr.Left.GetInferredType(),
		rhsSsa,
		binaryExpr.Right.GetInferredType(),
	)

	if e != nil {
		return SsaName{}, e
	}

	resSsa := irSsaName(ctx)
	irWritef(ctx, "  %%%s = ", resSsa.Repr)

	if isFloatType(newType) {
		irWrite(ctx, "fsub ")
	} else {
		irWrite(ctx, "sub ")
	}

	e = irType(ctx, newType)
	if e != nil {
		return SsaName{}, e
	}

	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, lhs)
	irWrite(ctx, ", ")
	irPossibleLitSsa(ctx, rhs)
	irWrite(ctx, "\n")
	return resSsa, nil
}

func irExprBinMultiplication(ctx *IrCtx, expectedType *t.NodeType, binaryExpr *t.NodeExprBinary) (SsaName, error) {
	// Generate IR for the left-hand side expression
	lhsSsa, e := irExpression(ctx, binaryExpr.Left.GetInferredType(), binaryExpr.Left)
	if e != nil {
		return SsaName{}, e
	}

	// Generate IR for the right-hand side expression
	rhsSsa, e := irExpression(ctx, binaryExpr.Right.GetInferredType(), binaryExpr.Right)
	if e != nil {
		return SsaName{}, e
	}

	// Promote both sides to a compatible numeric type
	lhs, rhs, newType, e := irPromoteToNum(
		ctx,
		expectedType,
		lhsSsa,
		binaryExpr.Left.GetInferredType(),
		rhsSsa,
		binaryExpr.Right.GetInferredType(),
	)
	if e != nil {
		return SsaName{}, e
	}

	// Allocate a new SSA name for the result
	resSsa := irSsaName(ctx)

	// Start writing the IR instruction
	irWritef(ctx, "  %%%s = ", resSsa.Repr)

	// Write the appropriate multiplication opcode
	if isFloatType(newType) {
		irWrite(ctx, "fmul ")
	} else {
		irWrite(ctx, "mul ")
	}

	// Write the type
	e = irType(ctx, newType)
	if e != nil {
		return SsaName{}, e
	}

	// Write the operands
	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, lhs)
	irWrite(ctx, ", ")
	irPossibleLitSsa(ctx, rhs)
	irWrite(ctx, "\n")

	// Return the resulting SSA name
	return resSsa, nil
}

func irExprBinCmp(ctx *IrCtx, binaryExpr *t.NodeExprBinary) (SsaName, error) {
	lhsSsa, e := irExpression(ctx, binaryExpr.Left.GetInferredType(), binaryExpr.Left)
	if e != nil {
		return SsaName{}, e
	}

	rhsSsa, e := irExpression(ctx, binaryExpr.Right.GetInferredType(), binaryExpr.Right)
	if e != nil {
		return SsaName{}, e
	}

	cmpType := binaryExpr.Left.GetInferredType()

	if isNumberType(binaryExpr.Left.GetInferredType()) && isNumberType(binaryExpr.Right.GetInferredType()) {
		lhsSsa, rhsSsa, cmpType, e = irPromoteToNum(
			ctx,
			binaryExpr.Left.GetInferredType(),
			lhsSsa,
			binaryExpr.Left.GetInferredType(),
			rhsSsa,
			binaryExpr.Right.GetInferredType(),
		)

		if e != nil {
			return SsaName{}, e
		}
	}

	isSigned := false
	isFloat := false
	if isNumberType(cmpType) {
		nd := getNumDesc(cmpType)
		isSigned = nd.IsSigned
		isFloat = nd.IsFloat
	}

	resSsa := irSsaName(ctx)
	irWritef(ctx, "  %%%s = icmp ", resSsa.Repr)

	switch binaryExpr.Operator {
	case t.KwCmpEq:
		irWrite(ctx, "eq ")
	case t.KwCmpNeq:
		irWrite(ctx, "ne ")
	case t.KwCmpGt:
		if isSigned {
			irWrite(ctx, "sgt ")
		} else if isFloat {
			irWrite(ctx, "ogt ")
		} else {
			irWrite(ctx, "ugt ")
		}
	case t.KwCmpLt:
		if isSigned {
			irWrite(ctx, "slt ")
		} else if isFloat {
			irWrite(ctx, "olt ")
		} else {
			irWrite(ctx, "ult ")
		}
	case t.KwCmpGtEq:
		if isSigned {
			irWrite(ctx, "sge ")
		} else if isFloat {
			irWrite(ctx, "oge ")
		} else {
			irWrite(ctx, "uge ")
		}
	case t.KwCmpLtEq:
		if isSigned {
			irWrite(ctx, "sle ")
		} else if isFloat {
			irWrite(ctx, "ole ")
		} else {
			irWrite(ctx, "ule ")
		}
	}

	e = irType(ctx, cmpType)
	if e != nil {
		return SsaName{}, e
	}

	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, lhsSsa)
	irWrite(ctx, ", ")
	irPossibleLitSsa(ctx, rhsSsa)
	irWrite(ctx, "\n")
	return resSsa, nil
}

func irExprBinary(ctx *IrCtx, expectedType *t.NodeType, binaryExpr *t.NodeExprBinary) (SsaName, error) {
	switch binaryExpr.Operator {
	case t.KwPlus:
		return irExprBinAddition(ctx, expectedType, binaryExpr)
	case t.KwMinus:
		return irExprBinSubstraction(ctx, expectedType, binaryExpr)
	case t.KwAsterisk:
		return irExprBinMultiplication(ctx, expectedType, binaryExpr)
	case t.KwAndAnd, t.KwOrOr:
		return irExprBinLogical(ctx, binaryExpr)
	case t.KwAmpersand, t.KwPipe, t.KwCaret:
		return irExprBinBitwise(ctx, expectedType, binaryExpr)
	case t.KwShiftLeft, t.KwShiftRight:
		return irExprBinShift(ctx, expectedType, binaryExpr)
	case t.KwCmpEq, t.KwCmpNeq, t.KwCmpGt, t.KwCmpLt, t.KwCmpGtEq, t.KwCmpLtEq:
		return irExprBinCmp(ctx, binaryExpr)
	}
	return SsaName{}, fmt.Errorf("unsupported binary expression")
}

func irTryCall(ctx *IrCtx, callRetSsa SsaName, fnCall *t.NodeExprCall) (SsaName, error) {
	errSsa := irSsaName(ctx)

	irWritef(ctx, "  %%%s = extractvalue ", errSsa.Repr)

	e := irThrowingType(ctx, fnCall.InfType)
	if e != nil {
		return SsaName{}, e
	}

	irWritef(ctx, " %%%s, 0\n", callRetSsa.Repr)

	e = irThrowSsa(ctx, errSsa, ctx.CurrFunc)
	if e != nil {
		return SsaName{}, e
	}

	if !isVoidType(fnCall.InfType) {
		valSsa := irSsaName(ctx)

		irWritef(ctx, "  %%%s = extractvalue ", valSsa.Repr)

		e = irThrowingType(ctx, fnCall.InfType)
		if e != nil {
			return SsaName{}, e
		}

		irWritef(ctx, " %%%s, 1\n", callRetSsa.Repr)
		return valSsa, nil
	}

	return SsaName{Repr: "<void ret>"}, nil
}

func irExpression(ctx *IrCtx, expectedType *t.NodeType, expr t.NodeExpr) (SsaName, error) {
	switch ne := expr.(type) {
	case *t.NodeExprVarDefAssign:
		return irVarDefAssign(ctx, ne)
	case *t.NodeExprVarDef:
		return irVarDef(ctx, ne)
	case *t.NodeExprAssign:
		return irExprAssign(ctx, ne.Left, ne.Right)
	case *t.NodeExprCall:
		return irExprFuncCall(ctx, ne, false)
	case *t.NodeExprDestructor:
		return irExprDestructor(ctx, ne)
	case *t.NodeExprUnary:
		return irExprUnary(ctx, expectedType, ne)
	case *t.NodeExprTry:
		callSsa, e := irExprFuncCall(ctx, ne.Call.(*t.NodeExprCall), true)
		if e != nil {
			return SsaName{}, e
		}
		return irTryCall(ctx, callSsa, ne.Call.(*t.NodeExprCall))
	case *t.NodeExprDestructureAssign:
		return irExprDestructureAssign(ctx, ne)
	case *t.NodeExprSubscript:
		return irExprSubscript(ctx, ne)
	case *t.NodeExprLit:
		return irExprLit(ctx, ne, expectedType)
	case *t.NodeExprSizeof:
		return irExprSizeof(ctx, ne)
	case *t.NodeExprName:
		return irExprName(ctx, ne)
	case *t.NodeExprBinary:
		return irExprBinary(ctx, expectedType, ne)
	}
	return ssaName(""), fmt.Errorf("unsupported expression")
}

func irExpressionLvalue(ctx *IrCtx, expr t.NodeExpr) (SsaName, error) {
	switch ne := expr.(type) {
	case *t.NodeExprName:
		return irExprNameLvalue(ctx, ne)
	case *t.NodeExprSubscript:
		return irExprSubscriptLvalue(ctx, ne)
	}
	return ssaName(""), fmt.Errorf("expression is not assignable")
}

func irJmpToDefer(ctx *IrCtx) {
	if ctx.CurrDeferIdx == 0 {
		irWritef(ctx, "  br label %%.defer.%d.base\n", ctx.CurrNestedScopeIdx)
	} else {
		irWritef(ctx, "  br label %%.defer.%d.%d\n", ctx.CurrNestedScopeIdx, ctx.CurrDeferIdx-1)
	}
}

func irJmpToParentDeferOnRet(ctx *IrCtx, parentCtx *IrCtx) {
	ssa := irSsaName(ctx)
	after := irSsaName(ctx)

	irWritef(ctx, "  %%%s = load i1, ptr %%.defer.ret\n", ssa.Repr)

	if parentCtx.CurrDeferIdx == 0 {
		irWritef(ctx, "  br i1 %%%s, label %%.defer.%d.base, label %%%s\n", ssa.Repr, parentCtx.CurrNestedScopeIdx, after.Repr)
	} else {
		irWritef(ctx, "  br i1 %%%s, label %%.defer.%d.%d, label %%%s\n", ssa.Repr, parentCtx.CurrNestedScopeIdx, parentCtx.CurrDeferIdx-1, after.Repr)
	}

	irWritef(ctx, "%s:\n", after.Repr)
}

func irStmtReturnDeferred(ctx *IrCtx, stmtRet *t.NodeStmtRet) error {
	// set flag for return after deferred statements
	irWrite(ctx, "  store i1 1, ptr %.defer.ret\n")

	if isVoidType(ctx.CurrFunc.ReturnType) && !ctx.CurrFunc.ReturnType.Throws {
		irJmpToDefer(ctx)
		return nil
	}

	switch stmtRet.Expression.(type) {
	case *t.NodeExprVoid:
		if stmtRet.OwnerFuncType.Throws {
			irWrite(ctx, "  store { %type.error } { %type.error zeroinitializer }, ptr %.defer.rv\n")
		}
		irJmpToDefer(ctx)
		return nil
	}

	ssa, e := irExpression(ctx, stmtRet.OwnerFuncType, stmtRet.Expression)
	if e != nil {
		return e
	}

	if stmtRet.OwnerFuncType.Throws {
		ssa, e = irMakeThrowingRetVal(ctx, stmtRet.OwnerFuncType, SsaName{}, ssa)
		if e != nil {
			return e
		}
	}

	irWrite(ctx, "  store ")
	e = irThrowingType(ctx, stmtRet.OwnerFuncType)
	if e != nil {
		return e
	}
	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, ssa)
	irWrite(ctx, ", ptr %.defer.rv\n")

	irJmpToDefer(ctx)
	return nil
}

func irStmtReturn(ctx *IrCtx, stmtRet *t.NodeStmtRet) error {
	return irStmtReturnDeferred(ctx, stmtRet)

	// LEGACY
	/*
		// TODO: lower expression
		switch stmtRet.Expression.(type) {
		case *t.NodeExprVoid:
			if stmtRet.OwnerFuncType.Throws {
				irWrite(ctx, "  ret { %type.error } { %type.error zeroinitializer }\n")
			} else {
				irWrite(ctx, "  ret void\n")
			}
			return nil
		}

		ssa, e := irExpression(ctx, stmtRet.OwnerFuncType, stmtRet.Expression)
		if e != nil {
			return e
		}

		if stmtRet.OwnerFuncType.Throws {
			ssa, e = irMakeThrowingRetVal(ctx, stmtRet.OwnerFuncType, SsaName{}, ssa)
			if e != nil {
				return e
			}
		}

		irWritef(ctx, "  ret ")

		e = irThrowingType(ctx, stmtRet.OwnerFuncType)
		if e != nil {
			return e
		}

		irWrite(ctx, " ")
		irPossibleLitSsa(ctx, ssa)

		irWrite(ctx, "\n")
		return nil*/
}

func irMakeThrowingRetVal(ctx *IrCtx, retType *t.NodeType, errSsa SsaName, valSsa SsaName) (SsaName, error) {
	r1Ssa := irSsaName(ctx)
	r2Ssa := irSsaName(ctx)

	irWritef(ctx, "  %%%s = insertvalue ", r1Ssa.Repr)
	e := irThrowingType(ctx, retType)
	if e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, " zeroinitializer, %type.error")

	if errSsa.Repr == "" {
		irWrite(ctx, " zeroinitializer")
	} else {
		irWrite(ctx, " ")
		irPossibleLitSsa(ctx, errSsa)
	}

	irWrite(ctx, ", 0\n")

	if isVoidType(retType) {
		return r1Ssa, nil
	} else {
		irWritef(ctx, "  %%%s = insertvalue ", r2Ssa.Repr)
		e = irThrowingType(ctx, retType)
		if e != nil {
			return SsaName{}, e
		}
		irWritef(ctx, " %%%s, ", r1Ssa.Repr)

		e = irType(ctx, retType)
		if e != nil {
			return SsaName{}, e
		}

		if valSsa.Repr == "" {
			irWrite(ctx, " zeroinitializer")
		} else {
			irWrite(ctx, " ")
			irPossibleLitSsa(ctx, valSsa)
		}

		irWrite(ctx, ", 1\n")
		return r2Ssa, nil
	}
}

func irThrowSsa(ctx *IrCtx, errSsa SsaName, fnDef *t.NodeFuncDef) error {
	fieldSsa := irSsaName(ctx)
	compSsa := irSsaName(ctx)

	eqLabel := irSsaName(ctx)
	neqLabel := irSsaName(ctx)

	// get error code field
	irWritef(ctx, "  %%%s = extractvalue %%type.error %%%s, 0\n", fieldSsa.Repr, errSsa.Repr)

	// if errcode != 0
	irWritef(ctx, "  %%%s = icmp ne i32 %%%s, 0\n", compSsa.Repr, fieldSsa.Repr)
	irWritef(ctx, "  br i1 %%%s, label %%%s, label %%%s\n", compSsa.Repr, neqLabel.Repr, eqLabel.Repr)

	// throw = err; return 0
	irWritef(ctx, "%s:\n", neqLabel.Repr)

	retValSsa := errSsa
	if fnDef.ReturnType.Throws {
		// generate throwing ret val
		var e error
		retValSsa, e = irMakeThrowingRetVal(ctx, fnDef.ReturnType, errSsa, SsaName{})
		if e != nil {
			return e
		}
	}

	//if ctx.CurrFunc.HasDefer {

	irWrite(ctx, "  store i1 1, ptr %.defer.ret\n")

	irWrite(ctx, "  store ")
	e := irThrowingType(ctx, fnDef.ReturnType)
	if e != nil {
		return e
	}
	irWrite(ctx, " ")
	irWritef(ctx, "%%%s", retValSsa.Repr)
	irWrite(ctx, ", ptr %.defer.rv\n")

	irJmpToDefer(ctx)

	/*
		} else {
			irWrite(ctx, "  ret ")
			e := irThrowingType(ctx, fnDef.ReturnType)
			if e != nil {
				return e
			}
			irWritef(ctx, " %%%s\n", retValSsa.Repr)
		}*/

	// else nothing
	irWritef(ctx, "%s:\n", eqLabel.Repr)
	return nil
}

func irStmtThrow(ctx *IrCtx, stmtThrow *t.NodeStmtThrow, fnDef *t.NodeFuncDef) error {
	exprSsa, e := irExpression(ctx, makeNamedType("error"), stmtThrow.Expression)
	if e != nil {
		return e
	}

	return irThrowSsa(ctx, exprSsa, fnDef)
}

func irExprDestructureAssign(ctx *IrCtx, expr *t.NodeExprDestructureAssign) (SsaName, error) {
	// Allocate both locals first so they exist regardless of call outcome.
	valPtr, e := irVarDef(ctx, &expr.ValueDef)
	if e != nil {
		return SsaName{}, e
	}
	errPtr, e := irVarDef(ctx, &expr.ErrDef)
	if e != nil {
		return SsaName{}, e
	}

	callSsa, e := irExprFuncCall(ctx, expr.Call, true)
	if e != nil {
		return SsaName{}, e
	}

	// Extract error
	errVal := irSsaName(ctx)
	irWritef(ctx, "  %%%s = extractvalue ", errVal.Repr)
	e = irThrowingType(ctx, expr.Call.InfType)
	if e != nil {
		return SsaName{}, e
	}
	irWritef(ctx, " %%%s, 0\n", callSsa.Repr)

	irWrite(ctx, "  store ")
	e = irType(ctx, expr.ErrDef.Type)
	if e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, errVal)
	irWritef(ctx, ", ptr %%%s\n", errPtr.Repr)

	// Extract value (if any)
	if !isVoidType(expr.Call.InfType) {
		valVal := irSsaName(ctx)
		irWritef(ctx, "  %%%s = extractvalue ", valVal.Repr)
		e = irThrowingType(ctx, expr.Call.InfType)
		if e != nil {
			return SsaName{}, e
		}
		irWritef(ctx, " %%%s, 1\n", callSsa.Repr)

		irWrite(ctx, "  store ")
		e = irType(ctx, expr.ValueDef.Type)
		if e != nil {
			return SsaName{}, e
		}
		irWrite(ctx, " ")
		irPossibleLitSsa(ctx, valVal)
		irWritef(ctx, ", ptr %%%s\n", valPtr.Repr)
	}

	return valPtr, nil
}

func irStatement(ctx *IrCtx, stmtNode t.NodeStatement, fnDef *t.NodeFuncDef) error {
	var e error

	switch s := stmtNode.(type) {
	case *t.NodeStmtRet:
		e = irStmtReturn(ctx, s)
	case *t.NodeStmtExpr:
		_, e = irExpression(ctx, nil, s.Expression)
	case *t.NodeStmtThrow:
		e = irStmtThrow(ctx, s, fnDef)
	case *t.NodeLlvm:
		irLlvm(ctx, s)
		return nil
	case *t.NodeStmtIf:
		e = irStmtIf(ctx, s, fnDef)
	case *t.NodeStmtWhile:
		e = irStmtWhile(ctx, s, fnDef)
	}
	return e
}

func irStmtIf(ctx *IrCtx, ifStmt *t.NodeStmtIf, fnDef *t.NodeFuncDef) error {
	condSsa, e := irExpression(ctx, makeNamedType("bool"), ifStmt.CondExpr)
	if e != nil {
		return e
	}

	eqLabel := irSsaName(ctx)
	neqLabel := irSsaName(ctx)
	endLabel := irSsaName(ctx)

	irWrite(ctx, "  br i1 ")
	irPossibleLitSsa(ctx, condSsa)

	irWritef(ctx, ", label %%%s, label %%%s\n", eqLabel.Repr, neqLabel.Repr)

	irWritef(ctx, "%s:\n", eqLabel.Repr)

	e = irBody(ctx, &ifStmt.Body, fnDef)
	if e != nil {
		return e
	}

	irWritef(ctx, "  br label %%%s\n", endLabel.Repr)
	irWritef(ctx, "%s:\n", neqLabel.Repr)

	if ifStmt.NextCondStmt != nil {
		switch n := ifStmt.NextCondStmt.(type) {
		case *t.NodeStmtIf:
			e = irStmtIf(ctx, n, fnDef)
			if e != nil {
				return e
			}
		case *t.NodeStmtElse:
			e = irBody(ctx, &n.Body, fnDef)
			if e != nil {
				return e
			}
		}
		irWritef(ctx, "  br label %%%s\n", endLabel.Repr)
	} else {
		irWritef(ctx, "  br label %%%s\n", endLabel.Repr)
	}

	irWritef(ctx, "%s:\n", endLabel.Repr)
	return nil
}

func irStmtWhile(ctx *IrCtx, ifStmt *t.NodeStmtWhile, fnDef *t.NodeFuncDef) error {
	condLbl := irSsaName(ctx)
	exitLbl := irSsaName(ctx)

	irWritef(ctx, "  br label %%%s\n", condLbl.Repr)
	irWritef(ctx, "%s:\n", condLbl.Repr)

	condSsa, e := irExpression(ctx, makeNamedType("bool"), ifStmt.CondExpr)
	if e != nil {
		return e
	}

	eqLbl := irSsaName(ctx)

	irWrite(ctx, "  br i1 ")
	irPossibleLitSsa(ctx, condSsa)

	irWritef(ctx, ", label %%%s, label %%%s\n", eqLbl.Repr, exitLbl.Repr)

	irWritef(ctx, "%s:\n", eqLbl.Repr)

	e = irBody(ctx, &ifStmt.Body, fnDef)
	if e != nil {
		return e
	}

	irWritef(ctx, "  br label %%%s\n", condLbl.Repr)
	irWritef(ctx, "%s:\n", exitLbl.Repr)
	return nil
}

func irBody(ctx *IrCtx, bodyNode *t.NodeBody, fnDef *t.NodeFuncDef) error {
	*ctx.SeenNestedScopes = (*ctx.SeenNestedScopes) + 1

	cpy := *ctx
	cpy.bld = ScopeBuilder{
		Global: ctx.bld.Global,
		Head:   &bytes.Buffer{},
		Tail:   &bytes.Buffer{},
		Body:   &bytes.Buffer{},
	}

	cpy.CurrNestedScopeIdx = *ctx.SeenNestedScopes
	cpy.CurrDeferIdx = 0
	var deferred []*t.NodeStmtDefer = nil

	for _, stmt := range bodyNode.Statements {
		switch n := stmt.(type) {
		case *t.NodeStmtDefer:
			cpy.CurrDeferIdx++
			deferred = append(deferred, n)
		case *t.NodeStmtExpr:
			switch n2 := n.Expression.(type) {
			case *t.NodeExprVarDef:
				if n2.Type.Destructor != nil {
					cpy.CurrDeferIdx++
					deferred = append(deferred, &t.NodeStmtDefer{
						Expression: &t.NodeExprDestructor{
							VarDef:     n2,
							Destructor: n2.Type.Destructor,
						},
					})
				}
			case *t.NodeExprVarDefAssign:
				if n2.VarDef.Type.Destructor != nil {
					cpy.CurrDeferIdx++
					deferred = append(deferred, &t.NodeStmtDefer{
						Expression: &t.NodeExprDestructor{
							VarDef:     n2.VarDef,
							Destructor: n2.VarDef.Type.Destructor,
						},
					})
				}
			}
		}

		e := irStatement(&cpy, stmt, fnDef)
		if e != nil {
			return e
		}
	}

	defLen := len(deferred)

	for i := range defLen {
		revIdx := defLen - 1 - i

		irWritef(&cpy, "  br label %%.defer.%d.%d\n", cpy.CurrNestedScopeIdx, revIdx)
		irWritef(&cpy, ".defer.%d.%d:\n", cpy.CurrNestedScopeIdx, revIdx)

		def := deferred[revIdx]
		if !def.IsBody {
			_, e := irExpression(&cpy, nil, def.Expression)
			if e != nil {
				return e
			}
			continue
		} else {
			for _, stmt := range def.Body.Statements {
				e := irStatement(&cpy, stmt, fnDef)
				if e != nil {
					return e
				}
			}
			continue
		}
	}

	if defLen == 0 {
		irWritef(&cpy, "  br label %%.defer.%d.base\n", cpy.CurrNestedScopeIdx)
		irWritef(&cpy, ".defer.%d.base:\n", cpy.CurrNestedScopeIdx)
	}

	/*
		shouldRetSsa := irSsaName(ctx)
		afterSsa := irSsaName(ctx)
		irWritef(&cpy, "  %%%s = load i1, ptr %%.defer.ret\n", shouldRetSsa.Repr)
		irWritef(&cpy, "  br i1 %%%s, label %%.defer.%d.%d, label %%%s\n", shouldRetSsa.Repr, ctx.CurrNestedScopeIdx, ctx.CurrDeferIdx, afterSsa.Repr)
		irWritef(&cpy, "%s:\n", afterSsa.Repr)*/

	irJmpToParentDeferOnRet(&cpy, ctx)

	irWrite(ctx, cpy.bld.Head.String())
	irWrite(ctx, cpy.bld.Body.String())
	irWrite(ctx, cpy.bld.Tail.String())
	return nil
}

func irFuncBody(ctx *IrCtx, bodyNode *t.NodeBody, fnDef *t.NodeFuncDef) error {
	irWrite(ctx, "{\n")

	// making du ctx to redirect writes
	cpy := *ctx
	cpy.bld = ScopeBuilder{
		Global: ctx.bld.Global,
		Head:   &bytes.Buffer{},
		Tail:   &bytes.Buffer{},
		Body:   &bytes.Buffer{},
	}
	cpy.parentBld = cpy.bld

	if !(isVoidType(fnDef.ReturnType) && !fnDef.ReturnType.Throws) {
		irWrite(&cpy, "  %.defer.rv = alloca ")
		e := irThrowingType(&cpy, fnDef.ReturnType)
		if e != nil {
			return e
		}
		irWrite(&cpy, "\n")
	}

	irWrite(&cpy, "  %.defer.ret = alloca i1\n")
	irWrite(&cpy, "  store i1 0, ptr %.defer.ret\n")

	if !(isVoidType(fnDef.ReturnType) && !fnDef.ReturnType.Throws) {
		irWrite(&cpy, "  store ")
		e := irThrowingType(&cpy, fnDef.ReturnType)
		if e != nil {
			return e
		}
		irWrite(&cpy, " zeroinitializer, ptr %.defer.rv\n")
	}

	foundRet := false
	cpy.CurrDeferIdx = 0

	// TODO: fix this, this is a stinking hack
	// used to insert destructors as deferred statements
	fnDef.Deferred = nil

	for _, stmt := range bodyNode.Statements {
		switch n := stmt.(type) {
		case *t.NodeStmtRet:
			foundRet = true
		case *t.NodeStmtDefer:
			cpy.CurrDeferIdx++
			fnDef.Deferred = append(fnDef.Deferred, n)
		case *t.NodeStmtExpr:
			switch n2 := n.Expression.(type) {
			case *t.NodeExprVarDef:
				if n2.Type.Destructor != nil {
					cpy.CurrDeferIdx++
					fnDef.Deferred = append(fnDef.Deferred, &t.NodeStmtDefer{
						Expression: &t.NodeExprDestructor{
							VarDef:     n2,
							Destructor: n2.Type.Destructor,
						},
					})
				}
			case *t.NodeExprVarDefAssign:
				if n2.VarDef.Type.Destructor != nil {
					cpy.CurrDeferIdx++
					fnDef.Deferred = append(fnDef.Deferred, &t.NodeStmtDefer{
						Expression: &t.NodeExprDestructor{
							VarDef:     n2.VarDef,
							Destructor: n2.VarDef.Type.Destructor,
						},
					})
				}
			}
		}

		e := irStatement(&cpy, stmt, fnDef)
		if e != nil {
			return e
		}
	}

	defLen := len(fnDef.Deferred)

	for i := range defLen {
		revIdx := defLen - 1 - i

		irWritef(&cpy, "  br label %%.defer.%d.%d\n", ctx.CurrNestedScopeIdx, revIdx)
		irWritef(&cpy, ".defer.%d.%d:\n", ctx.CurrNestedScopeIdx, revIdx)

		def := fnDef.Deferred[revIdx]
		if !def.IsBody {
			_, e := irExpression(&cpy, nil, def.Expression)
			if e != nil {
				return e
			}
			continue
		} else {
			for _, stmt := range def.Body.Statements {
				switch stmt.(type) {
				case *t.NodeStmtRet:
					foundRet = true
				}

				e := irStatement(&cpy, stmt, fnDef)
				if e != nil {
					return e
				}
			}
			continue
		}
	}

	if defLen == 0 {
		irWrite(&cpy, "  br label %.defer.0.base\n")
		irWrite(&cpy, ".defer.0.base:\n")
	}

	if !(isVoidType(fnDef.ReturnType) && !fnDef.ReturnType.Throws) {
		irWrite(&cpy, "  %rv = load ")
		e := irThrowingType(&cpy, fnDef.ReturnType)
		if e != nil {
			return e
		}
		irWrite(&cpy, ", ptr %.defer.rv\n")
	}
	irWrite(&cpy, "  ret ")

	if isVoidType(fnDef.ReturnType) && !fnDef.ReturnType.Throws {
		irWrite(&cpy, "void\n")
	} else {
		e := irThrowingType(&cpy, fnDef.ReturnType)
		if e != nil {
			return e
		}
		irWrite(&cpy, " %rv\n")
	}

	irWrite(ctx, cpy.bld.Head.String())
	irWrite(ctx, cpy.bld.Body.String())
	irWrite(ctx, cpy.bld.Tail.String())

	if !foundRet {
		irWrite(ctx, "  ret ")

		e := irThrowingType(ctx, fnDef.ReturnType)
		if e != nil {
			return e
		}

		if fnDef.ReturnType.Throws {
			if !isVoidType(fnDef.ReturnType) {
				irWrite(ctx, " { %type.error zeroinitializer, ")
				e := irType(ctx, fnDef.ReturnType)
				if e != nil {
					return e
				}
				irWrite(ctx, " zeroinitializer }\n")
			} else {
				irWrite(ctx, " { %type.error zeroinitializer }\n")
			}
		} else {
			if !isVoidType(fnDef.ReturnType) {
				irWrite(ctx, " zeroinitializer\n")
			} else {
				irWrite(ctx, "\n")
			}
		}
	}

	irWrite(ctx, "}\n\n\n")
	return nil
}

func irMainWrapper(ctx *IrCtx, mainFnDef *t.NodeFuncDef) error {

	if mainFnDef.ReturnType.Throws {
		errFmt := "Uncaught Error: %d '%s'\\0A"
		irWritef(ctx, "@.main.fmt.err = private constant [%d x i8] c\"%s\\00\"\n\n", len(errFmt)-1, errFmt)

		// check if printf was already declared in another module
		printfDecl := "declare i32 @printf(ptr, ...)\n"

		ctx.Shared.LlvmDeclM.Lock()
		_, ok := ctx.Shared.LlvmDecl[printfDecl]
		ctx.Shared.LlvmDeclM.Unlock()

		if !ok {
			irWrite(ctx, printfDecl)
		}
	}

	irWrite(ctx, "; Entry point\n")
	irWrite(ctx, "define i32 @main(i32 %argc, ptr %argv) alwaysinline {\n")
	irWrite(ctx, "entry:\n")

	hasArgs := false

	if len(mainFnDef.Class.ArgsNode.Args) > 0 {
		first := mainFnDef.Class.ArgsNode.Args[0]

		// TODO check for slice type
		if first.Name == "args" {
			hasArgs = true

			irWrite(ctx, "  %arr = alloca %type.str, i32 %argc\n")
			irWrite(ctx, "  %a = call %type.slice @magma.argsToSlice(i32 %argc, ptr %argv, ptr %arr)\n")
		}
	}

	if mainFnDef.ReturnType.Throws {
		if hasArgs {
			irWritef(ctx, "  %%r = call { %%type.error } @%s.main(%%type.slice %%a)\n", ctx.fCtx.MainPckgName)
		} else {
			irWritef(ctx, "  %%r = call { %%type.error } @%s.main()\n", ctx.fCtx.MainPckgName)
		}
		irWrite(ctx, "  %e = extractvalue { %type.error } %r, 0\n")
		irWrite(ctx, "  %ecd = extractvalue %type.error %e, 0\n")
		irWrite(ctx, "  %isnz = icmp ne i32 %ecd, 0\n")
		irWrite(ctx, "  br i1 %isnz, label %enz, label %ez\n")
		irWrite(ctx, "enz:\n")
		irWrite(ctx, "  %ems = extractvalue %type.error %e, 1\n")
		irWrite(ctx, "  %emss = extractvalue %type.str %ems, 0\n")
		irWrite(ctx, "  call i32 (ptr, ...) @printf(ptr @.main.fmt.err, i32 %ecd, ptr %emss)\n")
		irWrite(ctx, "  ret i32 %ecd\n")
		irWrite(ctx, "ez:\n")
	} else {
		if hasArgs {
			irWritef(ctx, "  call void @%s.main(%%type.slice %%a)\n", ctx.fCtx.MainPckgName)
		} else {
			irWritef(ctx, "  call void @%s.main()\n", ctx.fCtx.MainPckgName)
		}
	}
	irWrite(ctx, "  ret i32 0\n")
	irWrite(ctx, "}\n\n")
	return nil
}

func irFuncDef(ctx *IrCtx, fnDefNode *t.NodeFuncDef) error {
	isMemberFunc := false
	singleName := ""

	switch n := fnDefNode.Class.NameNode.(type) {
	case *t.NodeNameComposite:
		isMemberFunc = true
	case *t.NodeNameSingle:
		singleName = n.Name
	}

	if ctx.fCtx.PackageName == ctx.fCtx.MainPckgName && singleName == "main" {
		e := irMainWrapper(ctx, fnDefNode)
		if e != nil {
			return e
		}
	}

	irWrite(ctx, "define ")
	e := irThrowingType(ctx, fnDefNode.ReturnType)
	if e != nil {
		return e
	}

	irWrite(ctx, " @")
	e = irName(ctx, fnDefNode.Class.NameNode, true)
	if e != nil {
		return e
	}

	e = irArgsList(ctx, &fnDefNode.Class.ArgsNode, isMemberFunc)
	if e != nil {
		return e
	}

	irWrite(ctx, " alwaysinline ")

	ctx.CurrFunc = fnDefNode
	e = irFuncBody(ctx, &fnDefNode.Body, fnDefNode)
	if e != nil {
		return e
	}
	ctx.CurrFunc = nil
	return nil
}

func irArg(ctx *IrCtx, argNode *t.NodeArg) error {
	e := irType(ctx, argNode.TypeNode)
	if e != nil {
		return e
	}

	irWrite(ctx, " %")
	irWrite(ctx, argNode.Name)
	return nil
}

func irArgsList(ctx *IrCtx, argListNode *t.NodeArgList, thisArg bool) error {
	irWrite(ctx, "(")
	bound := len(argListNode.Args)

	if thisArg {
		irWrite(ctx, "ptr %this")
		if bound > 1 {
			irWrite(ctx, ", ")
		}
	}

	for i, a := range argListNode.Args {
		if thisArg && i == 0 {
			continue
		}

		e := irArg(ctx, &a)
		if e != nil {
			return e
		}

		if i < bound-1 {
			irWrite(ctx, ", ")
		}
	}

	irWrite(ctx, ")")
	return nil
}

func irGlobalDecl(ctx *IrCtx, glDeclNode t.NodeGlobalDecl) error {
	switch g := glDeclNode.(type) {
	case *t.NodeFuncDef:
		e := irFuncDef(ctx, g)
		if e != nil {
			return e
		}
	case *t.NodeLlvm:
		irLlvm(ctx, g)
		return nil
	}
	return nil
}

func irNameSingle(ctx *IrCtx, nameNode *t.NodeNameSingle, withPackage bool) error {
	if withPackage {
		irWrite(ctx, ctx.fCtx.PackageName)
		irWrite(ctx, ".")
	}
	irWrite(ctx, nameNode.Name)
	return nil
}

func irNameSingleSsa(ctx *IrCtx, nameNode *t.NodeNameSingle, withPackage bool) SsaName {
	ssa := ""

	if withPackage {
		ssa += ctx.fCtx.PackageName + "."
	}
	ssa += nameNode.Name
	return ssaName(ssa)
}

func irNameComposite(ctx *IrCtx, nameNode *t.NodeNameComposite, withPackage bool) error {
	bound := len(nameNode.Parts)
	for i, n := range nameNode.Parts {

		if i == 0 {
			first := nameNode.Parts[0]

			// if not imported package, prepend with <thispackage>.
			alias, ok := ctx.fCtx.GlNode.ImportAlias[first]
			if !ok {
				if withPackage {
					irWrite(ctx, ctx.fCtx.PackageName)
					irWrite(ctx, ".")
				}
				irWrite(ctx, first)
			} else {
				irWrite(ctx, alias)
			}
			goto next
		}

		irWrite(ctx, n)

	next:
		if i < bound-1 {
			irWrite(ctx, ".")
		}
	}

	return nil
}

func irNameCompositeSsa(ctx *IrCtx, nameNode *t.NodeNameComposite, withPackage bool) SsaName {
	ssa := ""

	if withPackage {
		first := nameNode.Parts[0]

		// if not imported package, prepend with <thispackage>.
		_, ok := ctx.fCtx.ImportAlias[first]
		if !ok {
			ssa += ctx.fCtx.PackageName + "."
		}
	}

	bound := len(nameNode.Parts)
	for i, n := range nameNode.Parts {
		ssa += n
		if i < bound-1 {
			ssa += "."
		}
	}

	return ssaName(ssa)
}

func irName(ctx *IrCtx, nameNode t.NodeName, withPackage bool) error {
	switch n := nameNode.(type) {
	case *t.NodeNameComposite:
		e := irNameComposite(ctx, n, withPackage)
		if e != nil {
			return e
		}
	case *t.NodeNameSingle:
		e := irNameSingle(ctx, n, withPackage)
		if e != nil {
			return e
		}
	}
	return nil
}

func irNameSsa(ctx *IrCtx, nameNode t.NodeName, withPackage bool) SsaName {
	switch n := nameNode.(type) {
	case *t.NodeNameComposite:
		return irNameCompositeSsa(ctx, n, withPackage)
	case *t.NodeNameSingle:
		return irNameSingleSsa(ctx, n, withPackage)
	}
	return ssaName("")
}

func irTypeKind(ctx *IrCtx, typeKind t.NodeTypeKind) error {
	switch tn := typeKind.(type) {
	case *t.NodeTypeSlice:
		irWrite(ctx, "%type.slice")
		return nil
	case *t.NodeTypePointer:
		irWrite(ctx, "ptr")
		return nil
	case *t.NodeTypeRfc:
		irWrite(ctx, "ptr")
		return nil
	case *t.NodeTypeFunc:
		e := irFuncPtrType(ctx, tn)
		if e != nil {
			return e
		}
		return nil
	case *t.NodeTypeNamed:
		switch n := tn.NameNode.(type) {
		case *t.NodeNameSingle:
			// TODO: check if intrinsic type
			_, ok := magmatypes.BasicTypes[n.Name]
			if ok {
				irWrite(ctx, magmatypes.BasicTypes[n.Name])
				return nil
			}
		}

		irWrite(ctx, "<unchecked type>")
		/*
			irWrite(ctx, "%struct.")
			e := irName(ctx, tn.NameNode, true)
			if e != nil {
				return e
			}*/
		return nil
	case *t.NodeTypeAbsolute:
		irWritef(ctx, "%%struct.%s", tn.AbsoluteName)
		return nil
	}
	irWrite(ctx, "<invalid type>")
	return nil
}

func irThrowingType(ctx *IrCtx, typeNode *t.NodeType) error {
	if typeNode == nil {
		irWrite(ctx, "<null type node>")
		return nil
	}

	if isVoidType(typeNode) && typeNode.Throws {
		irWrite(ctx, "{ %type.error }")
		return nil
	}

	if typeNode.Throws {
		irWrite(ctx, "{ %type.error, ")
	}

	e := irTypeKind(ctx, typeNode.KindNode)
	if e != nil {
		return e
	}

	if typeNode.Throws {
		irWrite(ctx, " }")
	}
	return nil
}

func irType(ctx *IrCtx, typeNode *t.NodeType) error {
	if typeNode == nil {
		irWrite(ctx, "<null type node>")
		return nil
	}

	return irTypeKind(ctx, typeNode.KindNode)
}

func irDefineStruct(ctx *IrCtx, structNode *t.NodeStructDef) error {
	irWriteGl(ctx, "%struct.")

	// making dud ctx to redirect name IR to global writer
	cpy := *ctx
	cpy.bld = ScopeBuilder{
		Global: ctx.bld.Global,
		Head:   ctx.bld.Global,
		Tail:   ctx.bld.Global,
		Body:   ctx.bld.Global,
	}

	e := irName(&cpy, structNode.Class.NameNode, true)
	if e != nil {
		return e
	}
	irWriteGl(ctx, " = type { ")

	bound := len(structNode.Class.ArgsNode.Args)
	for i, field := range structNode.Class.ArgsNode.Args {
		e = irType(&cpy, field.TypeNode)
		if e != nil {
			return e
		}

		if i < bound-1 {
			irWriteGl(ctx, ", ")
		}
	}

	irWriteGl(ctx, " }\n")
	return nil
}

func irGlobalStructDefs(ctx *IrCtx, glNode *t.NodeGlobal) error {
	ctx.bld.StructM.Lock()
	defer ctx.bld.StructM.Unlock()

	for _, d := range glNode.Declarations {
		switch s := d.(type) {
		case *t.NodeStructDef:
			cpy := *ctx
			cpy.bld.Body = ctx.bld.Struct
			cpy.bld.Head = ctx.bld.Struct
			cpy.bld.Global = ctx.bld.Struct
			cpy.bld.Tail = ctx.bld.Struct

			e := irDefineStruct(&cpy, s)
			if e != nil {
				return e
			}
		default:
			continue
		}
	}
	return nil
}

func irGlobal(ctx *IrCtx, glNode *t.NodeGlobal) error {
	for _, d := range glNode.Declarations {
		e := irGlobalDecl(ctx, d)
		if e != nil {
			return e
		}
	}
	return nil
}

func irLlvm(ctx *IrCtx, llvmNode *t.NodeLlvm) {
	irWrite(ctx, llvmNode.Text)
}

func irWriteModule(
	shared *t.SharedState,
	fCtx *t.FileCtx,
	builder *bytes.Buffer,
	glBld *bytes.Buffer,
	structBld *bytes.Buffer,
	strctBldM *sync.Mutex,
	i int,
) error {
	nextSsa := 0
	seenScopes := 0

	ctx := &IrCtx{
		Shared: shared,
		fCtx:   fCtx,
		bld: ScopeBuilder{
			Struct:  structBld,
			StructM: strctBldM,
			Global:  glBld,
			Head:    &bytes.Buffer{},
			Tail:    &bytes.Buffer{},
			Body:    &bytes.Buffer{},
		},
		parentBld: ScopeBuilder{
			Struct:  structBld,
			StructM: strctBldM,
			Global:  glBld,
			Head:    &bytes.Buffer{},
			Tail:    &bytes.Buffer{},
			Body:    &bytes.Buffer{},
		},
		nextSsa:          &nextSsa,
		moduleIdx:        i,
		SeenNestedScopes: &seenScopes,
	}
	builder.Grow(512)

	irWriteGlf(ctx, "; File=\"%s\"\n", ctx.fCtx.FilePath)
	irWriteGlf(ctx, "; Module=\"%s\"\n\n", ctx.fCtx.PackageName)

	irWriteGl(ctx, "; Defined Types\n")
	e := irGlobalStructDefs(ctx, fCtx.GlNode)
	if e != nil {
		return e
	}

	irWriteGl(ctx, "\n; Global Defs\n")

	irWrite(ctx, "\n; Code\n")
	e = irGlobal(ctx, fCtx.GlNode)
	if e != nil {
		return e
	}

	builder.WriteString(ctx.bld.Head.String())
	builder.WriteString(ctx.bld.Body.String())
	builder.WriteString(ctx.bld.Tail.String())
	return nil
}

func IrWrite(shared *t.SharedState) ([]byte, error) {
	// creates a shallow copy of shared.Files, will prevent any race condition
	// if it were ever to be modified, which it shouldn't.
	shared.FilesM.Lock()
	filesMap := maps.Clone(shared.Files)
	shared.FilesM.Unlock()

	// write header
	headBld := &bytes.Buffer{}
	headBld.WriteString("; Magma\n\n")
	headBld.WriteString("; Basic Types\n")
	magmatypes.WriteIrBasicTypes(headBld)

	headBld.WriteString("\n; Declarations\n")
	shared.LlvmDeclM.Lock()
	for llvm := range shared.LlvmDecl {
		headBld.WriteString(llvm)
	}
	shared.LlvmDeclM.Unlock()

	header := headBld.Bytes()

	llvmFragments := [][]byte{
		header,
		{},
		llvmfragments.Utils,
		llvmfragments.Utf8,
	}
	fragLen := len(llvmFragments)

	// result receiver
	type resStr struct {
		S []byte
		E error
	}
	results := make([]resStr, len(filesMap)+fragLen)

	// insert llvm fragments
	for i := range fragLen {
		results[i] = resStr{S: llvmFragments[i]}
	}

	// multithreaded writing per-module

	wg := sync.WaitGroup{}
	wg.Add(len(filesMap))

	structDefBld := &bytes.Buffer{}
	structDefBldM := sync.Mutex{}

	i := fragLen
	for _, v := range filesMap {

		localI := i
		go func(idx int) {
			defer wg.Done()

			// module local builder
			moduleBld := &bytes.Buffer{}
			glBld := &bytes.Buffer{}
			e := irWriteModule(shared, v, moduleBld, glBld, structDefBld, &structDefBldM, idx)
			if e != nil {
				results[idx] = resStr{E: e}
				return
			}
			glBld.Write(moduleBld.Bytes())

			results[idx] = resStr{S: glBld.Bytes()}
		}(localI)

		i++
	}

	// join threads
	wg.Wait()

	results[1].S = structDefBld.Bytes()

	// process results
	irStrings := [][]byte{}
	for _, r := range results {
		if r.E != nil {
			return []byte{}, r.E
		}
		irStrings = append(irStrings, r.S)
	}
	return bytes.Join(irStrings, []byte("\n")), nil
}
