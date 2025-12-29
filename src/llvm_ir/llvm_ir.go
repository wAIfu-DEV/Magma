package llvmir

import (
	llvmfragments "Magma/src/llvm_fragments"
	magmatypes "Magma/src/magma_types"
	t "Magma/src/types"
	"bytes"
	"fmt"
	"maps"
	"strconv"
	"strings"
	"sync"
)

type ScopeBuilder struct {
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

func irSsaName(ctx *IrCtx) SsaName {
	name := strconv.Itoa(*ctx.nextSsa)
	(*ctx.nextSsa)++
	return ssaName("." + name)
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
	irWrite(ctx, "  ; stack var def\n")

	allocSsa := irNameSsa(ctx, vd.Name, false)

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
	irWrite(ctx, "  ; stack var def + assignment\n")

	assignSsa, e := irExpression(ctx, vda.AssignExpr)
	if e != nil {
		return ssaName(""), e
	}

	allocSsa, e := irVarDef(ctx, &vda.VarDef)
	if e != nil {
		return ssaName(""), e
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

func irExprFuncCall(ctx *IrCtx, fnCall *t.NodeExprCall) (SsaName, error) {
	ssa := irSsaName(ctx)

	argsSsa := make([]SsaName, len(fnCall.Args))
	for i, expr := range fnCall.Args {
		exprSsa, e := irExpression(ctx, expr)
		if e != nil {
			return ssaName(""), e
		}
		argsSsa[i] = exprSsa
	}

	isVoidRet := isVoidType(fnCall.InfType)

	if !isVoidRet {
		irWritef(ctx, "  %%%s = ", ssa.Repr)
	} else {
		irWrite(ctx, "  ")
	}

	irWritef(ctx, "call ")

	e := irType(ctx, fnCall.InfType)
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

	if isVoidRet {
		// TODO: Check and inforce that void ret calls HAVE to be statements
		// and cannot be in expressions
		return ssaName(""), nil
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

func irExprLit(ctx *IrCtx, lit *t.NodeExprLit) (SsaName, error) {
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

func irMemberAccess(ctx *IrCtx, fromType *t.NodeType, fromSsa SsaName, fieldNb int, fieldType *t.NodeType) (SsaName, error) {
	fieldSsa := irSsaName(ctx)

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

	e := irType(ctx, baseType)
	if e != nil {
		return SsaName{}, e
	}

	irWritef(ctx, ", ptr %%%s, i32 0, i32 %d\n", basePtr.Repr, fieldIndex)

	return fieldPtr, nil
}

func irExprName(ctx *IrCtx, nameExpr *t.NodeExprName) (SsaName, error) {
	if nameExpr.IsSsa {
		return irNameSsa(ctx, nameExpr.Name, false), nil
	}

	ptrSsa := irNameSsa(ctx, nameExpr.Name, false)
	ssa := irSsaName(ctx)

	var typeNd *t.NodeType = nil
	isMemberAccess := false

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
	default:
		isMemberAccess = false
	}

	irWritef(ctx, "  %%%s = load ", ssa.Repr)

	e := irType(ctx, typeNd)
	if e != nil {
		return ssaName(""), e
	}

	irWritef(ctx, ", ptr %%%s\n", ptrSsa.Repr)

	if isMemberAccess {
		lastSsa := ssa
		if len(nameExpr.MemberAccesses) == 0 {
			return SsaName{}, fmt.Errorf("member access but no member access history")
		}

		fromType := typeNd

		for _, m := range nameExpr.MemberAccesses {
			fieldSsa, e := irMemberAccess(ctx, fromType, lastSsa, m.FieldNb, m.Type)
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
	target, e := irExpression(ctx, subs.Target)
	if e != nil {
		return SsaName{}, e
	}

	loadedTarget := target

	if !subs.IsTargetSsa {
		irWritef(ctx, "  %%%s = load ", loadedTarget.Repr)

		e = irType(ctx, subs.BoxType)
		if e != nil {
			return SsaName{}, e
		}
		irWritef(ctx, ", ptr %%%s\n", target.Repr)
	}

	subsExpr, e := irExpression(ctx, subs.Expr)
	if e != nil {
		return SsaName{}, e
	}

	switch subs.BoxType.KindNode.(type) {
	case *t.NodeTypeSlice:
		// extract ptr from struct first
		extracted := irSsaName(ctx)
		irWritef(ctx, "  %%%s = extractvalue %%type.slice %%%s, 0\n", extracted.Repr, loadedTarget.Repr)
		return irExprSubscriptPtr(ctx, subs, extracted, subsExpr)
	case *t.NodeTypePointer:
		return irExprSubscriptPtr(ctx, subs, loadedTarget, subsExpr)
	}
	return SsaName{}, fmt.Errorf("invalid box type in subscript expression lowering")
}

func irExprSubscriptPtr(ctx *IrCtx, subs *t.NodeExprSubscript, targetSsa SsaName, subsSsa SsaName) (SsaName, error) {
	irWrite(ctx, "  ; subscript\n")

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

func irExprAssign(ctx *IrCtx, lhs t.NodeExpr, rhs t.NodeExpr) (SsaName, error) {
	irWrite(ctx, "  ; assignment\n")

	lhsPtr, e := irExpressionLvalue(ctx, lhs)
	if e != nil {
		return SsaName{}, e
	}

	rhsVal, e := irExpression(ctx, rhs)
	if e != nil {
		return SsaName{}, e
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

func irExpression(ctx *IrCtx, expr t.NodeExpr) (SsaName, error) {
	switch ne := expr.(type) {
	case *t.NodeExprVarDefAssign:
		return irVarDefAssign(ctx, ne)
	case *t.NodeExprVarDef:
		return irVarDef(ctx, ne)
	case *t.NodeExprAssign:
		return irExprAssign(ctx, ne.Left, ne.Right)
	case *t.NodeExprCall:
		return irExprFuncCall(ctx, ne)
	case *t.NodeExprSubscript:
		return irExprSubscript(ctx, ne)
	case *t.NodeExprLit:
		return irExprLit(ctx, ne)
	case *t.NodeExprName:
		return irExprName(ctx, ne)
	}
	return ssaName(""), nil
}

func irExpressionLvalue(ctx *IrCtx, expr t.NodeExpr) (SsaName, error) {
	switch ne := expr.(type) {
	case *t.NodeExprName:
		return irExprNameLvalue(ctx, ne)
	}
	return ssaName(""), nil
}

func irStmtReturn(ctx *IrCtx, stmtRet *t.NodeStmtRet) error {
	// TODO: lower expression
	switch stmtRet.Expression.(type) {
	case *t.NodeExprVoid:
		irWrite(ctx, "  ret void\n")
		return nil
	}

	ssa, e := irExpression(ctx, stmtRet.Expression)
	if e != nil {
		return e
	}
	irWritef(ctx, "  ret ")

	e = irType(ctx, stmtRet.OwnerFuncType)
	if e != nil {
		return e
	}

	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, ssa)

	irWrite(ctx, "\n")
	return nil
}

func irStmtThrow(ctx *IrCtx, stmtThrow *t.NodeStmtThrow, fnDef *t.NodeFuncDef) error {
	irWrite(ctx, "  ; throw error if code != 0\n")

	exprSsa, e := irExpression(ctx, stmtThrow.Expression)
	if e != nil {
		return e
	}

	fieldSsa := irSsaName(ctx)
	compSsa := irSsaName(ctx)

	eqLabel := irSsaName(ctx)
	neqLabel := irSsaName(ctx)

	// get error code field
	irWritef(ctx, "  %%%s = extractvalue %%type.error %%%s, 0\n", fieldSsa.Repr, exprSsa.Repr)

	// if errcode != 0
	irWritef(ctx, "  %%%s = icmp ne i32 %%%s, 0\n", compSsa.Repr, fieldSsa.Repr)
	irWritef(ctx, "  br i1 %%%s, label %%%s, label %%%s\n", compSsa.Repr, neqLabel.Repr, eqLabel.Repr)

	// throw = err; return 0
	irWritef(ctx, "%s:\n", neqLabel.Repr)
	irWritef(ctx, "  store %%type.error %%%s, ptr %%throw\n", exprSsa.Repr)
	irWrite(ctx, "  ret ")

	e = irType(ctx, fnDef.ReturnType)
	if e != nil {
		return e
	}

	if !isVoidType(fnDef.ReturnType) {
		irWritef(ctx, " zeroinitializer\n")
	} else {
		irWrite(ctx, "\n")
	}

	// else nothing
	irWritef(ctx, "%s:\n", eqLabel.Repr)

	return nil
}

func irStatement(ctx *IrCtx, stmtNode t.NodeStatement, fnDef *t.NodeFuncDef) error {
	var e error

	switch s := stmtNode.(type) {
	case *t.NodeStmtRet:
		e = irStmtReturn(ctx, s)
	case *t.NodeStmtExpr:
		_, e = irExpression(ctx, s.Expression)
	case *t.NodeStmtThrow:
		e = irStmtThrow(ctx, s, fnDef)
	case *t.NodeLlvm:
		irLlvm(ctx, s)
		return nil
	case *t.NodeStmtIf:
		e = irStmtIf(ctx, s, fnDef)
	}
	return e
}

func irStmtIf(ctx *IrCtx, ifStmt *t.NodeStmtIf, fnDef *t.NodeFuncDef) error {
	condSsa, e := irExpression(ctx, ifStmt.CondExpr)
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

func irBody(ctx *IrCtx, bodyNode *t.NodeBody, fnDef *t.NodeFuncDef) error {
	cpy := *ctx
	cpy.bld = ScopeBuilder{
		Global: ctx.bld.Global,
		Head:   &bytes.Buffer{},
		Tail:   &bytes.Buffer{},
		Body:   &bytes.Buffer{},
	}

	for _, stmt := range bodyNode.Statements {
		switch stmt.(type) {
		case *t.NodeStmtRet:
			return nil
		}

		e := irStatement(ctx, stmt, fnDef)
		if e != nil {
			return e
		}
	}

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

	foundRet := false

	for _, stmt := range bodyNode.Statements {
		switch stmt.(type) {
		case *t.NodeStmtRet:
			foundRet = true
		}

		e := irStatement(&cpy, stmt, fnDef)
		if e != nil {
			return e
		}
	}

	irWrite(ctx, cpy.bld.Head.String())
	irWrite(ctx, cpy.bld.Body.String())
	irWrite(ctx, cpy.bld.Tail.String())

	if !foundRet {
		irWrite(ctx, "  ret ")

		if !isVoidType(fnDef.ReturnType) {
			e := irType(ctx, fnDef.ReturnType)
			if e != nil {
				return e
			}

			irWrite(ctx, " zeroinitializer\n")
		} else {
			irWrite(ctx, "void\n")
		}
	}

	irWrite(ctx, "}\n\n")
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
	irWrite(ctx, "define i32 @main(i32 %argc, ptr %argv) {\n")
	irWrite(ctx, "entry:\n")

	if mainFnDef.ReturnType.Throws {
		irWrite(ctx, "  %e = alloca %type.error\n")

	}

	hasArgs := false

	if len(mainFnDef.Class.ArgsNode.Args) > 0 {
		first := mainFnDef.Class.ArgsNode.Args[0]

		// TODO check for slice type
		if first.Name == "args" {
			hasArgs = true
			irWrite(ctx, "  %a = call %type.slice @magma.argsToSlice(i32 %argc, ptr %argv)\n")
		}
	}

	if mainFnDef.ReturnType.Throws {
		irWrite(ctx, "  store %type.error zeroinitializer, ptr %e\n")

		if hasArgs {
			irWritef(ctx, "  call void @%s.main(ptr %%e, %%type.slice %%a)\n", ctx.fCtx.MainPckgName)
		} else {
			irWritef(ctx, "  call void @%s.main(ptr %%e)\n", ctx.fCtx.MainPckgName)
		}
		irWrite(ctx, "  %efld1 = getelementptr %type.error, ptr %e, i32 0, i32 0\n")
		irWrite(ctx, "  %ecd = load i32, ptr %efld1\n")
		irWrite(ctx, "  %isnz = icmp ne i32 %ecd, 0\n")
		irWrite(ctx, "  br i1 %isnz, label %enz, label %ez\n")
		irWrite(ctx, "enz:\n")
		irWrite(ctx, "  %efld2 = getelementptr %type.error, ptr %e, i32 0, i32 1\n")
		irWrite(ctx, "  %ems = load %type.str, ptr %efld2\n")
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
	e := irType(ctx, fnDefNode.ReturnType)
	if e != nil {
		return e
	}

	irWrite(ctx, " @")
	e = irName(ctx, fnDefNode.Class.NameNode, true)
	if e != nil {
		return e
	}

	e = irArgsList(ctx, &fnDefNode.Class.ArgsNode, isMemberFunc, fnDefNode.ReturnType.Throws)
	if e != nil {
		return e
	}

	irWrite(ctx, " ")
	e = irFuncBody(ctx, &fnDefNode.Body, fnDefNode)
	if e != nil {
		return e
	}
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

func irArgsList(ctx *IrCtx, argListNode *t.NodeArgList, thisArg bool, throwArg bool) error {
	irWrite(ctx, "(")
	bound := len(argListNode.Args)

	if thisArg {
		irWrite(ctx, "ptr %this")
		if bound > 0 || throwArg {
			irWrite(ctx, ", ")
		}
	}

	if throwArg {
		irWrite(ctx, "ptr %throw")
		if bound > 0 {
			irWrite(ctx, ", ")
		}
	}

	for i, a := range argListNode.Args {
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

		irWrite(ctx, "%struct.")
		e := irName(ctx, tn.NameNode, true)
		if e != nil {
			return e
		}
		return nil
	}
	irWrite(ctx, "<invalid type>")
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
	for _, d := range glNode.Declarations {
		switch s := d.(type) {
		case *t.NodeStructDef:
			e := irDefineStruct(ctx, s)
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

func irWriteModule(shared *t.SharedState, fCtx *t.FileCtx, builder *bytes.Buffer, glBld *bytes.Buffer) error {
	nextSsa := 0

	ctx := &IrCtx{
		Shared: shared,
		fCtx:   fCtx,
		bld: ScopeBuilder{
			Global: glBld,
			Head:   &bytes.Buffer{},
			Tail:   &bytes.Buffer{},
			Body:   &bytes.Buffer{},
		},
		parentBld: ScopeBuilder{
			Global: glBld,
			Head:   &bytes.Buffer{},
			Tail:   &bytes.Buffer{},
			Body:   &bytes.Buffer{},
		},
		nextSsa: &nextSsa,
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
		llvmfragments.Utils,
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

	i := fragLen
	for _, v := range filesMap {

		localI := i
		go func(idx int) {
			defer wg.Done()

			// module local builder
			moduleBld := &bytes.Buffer{}
			glBld := &bytes.Buffer{}
			e := irWriteModule(shared, v, moduleBld, glBld)
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
