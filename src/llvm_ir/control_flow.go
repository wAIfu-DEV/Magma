package llvmir

import (
	t "Magma/src/types"
	"bytes"
	"fmt"
	"path/filepath"
)

func irJmpToDefer(ctx *IrCtx) {
	if ctx.CurrDeferIdx == 0 {
		irWritef(ctx, "  br label %%.defer.%d.base\n", ctx.CurrNestedScopeIdx)
	} else {
		irWritef(ctx, "  br label %%.defer.%d.%d\n", ctx.CurrNestedScopeIdx, ctx.CurrDeferIdx-1)
	}
}

func irJmpToParentDeferOnControl(ctx *IrCtx, parentCtx *IrCtx) {
	retSsa := irSsaLocal(ctx)
	brkSsa := irSsaLocal(ctx)
	contSsa := irSsaLocal(ctx)
	retOrBrkSsa := irSsaLocal(ctx)
	pendingSsa := irSsaLocal(ctx)
	after := irSsaName(ctx)

	irWritef(ctx, "  %s = load i1, ptr %%.defer.ret\n", retSsa.Repr)
	irWritef(ctx, "  %s = load i1, ptr %%.defer.brk\n", brkSsa.Repr)
	irWritef(ctx, "  %s = load i1, ptr %%.defer.cont\n", contSsa.Repr)
	irWritef(ctx, "  %s = or i1 %s, %s\n", retOrBrkSsa.Repr, retSsa.Repr, brkSsa.Repr)
	irWritef(ctx, "  %s = or i1 %s, %s\n", pendingSsa.Repr, retOrBrkSsa.Repr, contSsa.Repr)

	if parentCtx.CurrDeferIdx == 0 {
		irWritef(ctx, "  br i1 %s, label %%.defer.%d.base, label %%%s\n", pendingSsa.Repr, parentCtx.CurrNestedScopeIdx, after.Repr)
	} else {
		irWritef(ctx, "  br i1 %s, label %%.defer.%d.%d, label %%%s\n", pendingSsa.Repr, parentCtx.CurrNestedScopeIdx, parentCtx.CurrDeferIdx-1, after.Repr)
	}

	irWritef(ctx, "%s:\n", after.Repr)
}

func irStmtReturnDeferred(ctx *IrCtx, stmtRet *t.NodeStmtRet) error {

	/* DEPRECATED
	switch ne := stmtRet.Expression.(type) {
	case *t.NodeExprName:
		switch ne2 := ne.AssociatedNode.(type) {
		case *t.NodeExprVarDef:
			if !ne2.IsReturned && ne2.Type.Destructor != nil {
				ne2.IsReturned = true
				irWriteHdf(ctx, "  %%.destr%s = alloca i1\n", ne2.RetFlagId)
				irWriteHdf(ctx, "  store i1 0, ptr %%.destr%s\n", ne2.RetFlagId)

				// on branch that returns a destructible value, prevent destructor
				irWritef(ctx, "  store i1 1, ptr %%.destr%s\n", ne2.RetFlagId)
			}
		}
	}*/

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

	ssa, e := irExpression(ctx, stmtRet.OwnerFuncType, stmtRet.Expression, false)
	if e != nil {
		return e
	}
	ssa, e = irCoerceNumeric(ctx, stmtRet.OwnerFuncType, stmtRet.Expression, ssa)
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
}

func irStmtBreak(ctx *IrCtx, stmtBreak *t.NodeStmtBreak) error {
	if *ctx.NestedLoopCnt <= 0 {
		return fmt.Errorf("used break statement outside loop body")
	}

	// set flag for break after deferred statements
	irWrite(ctx, "  store i1 1, ptr %.defer.brk\n")
	irJmpToDefer(ctx)
	return nil
}

func irStmtContinue(ctx *IrCtx, stmtBreak *t.NodeStmtContinue) error {
	if *ctx.NestedLoopCnt <= 0 {
		return fmt.Errorf("used continue statement outside loop body")
	}
	irWrite(ctx, "  store i1 1, ptr %.defer.cont\n")
	irJmpToDefer(ctx)
	return nil
}

func irMakeThrowingRetVal(ctx *IrCtx, retType *t.NodeType, errSsa SsaName, valSsa SsaName) (SsaName, error) {
	r1Ssa := irSsaLocal(ctx)
	r2Ssa := irSsaLocal(ctx)

	irWritef(ctx, "  %s = insertvalue ", r1Ssa.Repr)
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
		irWritef(ctx, "  %s = insertvalue ", r2Ssa.Repr)
		e = irThrowingType(ctx, retType)
		if e != nil {
			return SsaName{}, e
		}
		irWritef(ctx, " %s, ", r1Ssa.Repr)

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

func irErrorSite(ctx *IrCtx, pos t.FilePos) SsaName {
	functionName := "<global>"
	if ctx.CurrFunc != nil {
		functionName = ctx.CurrFunc.DisplayName
		if functionName == "" {
			functionName = traceDisplayName(ctx.CurrFunc.Class.NameNode)
		}
	}
	functionStr := ctx.traceStrings.intern(functionName)
	// Runtime diagnostics should identify the source without embedding the
	// build machine's absolute directory in the executable.
	fileStr := ctx.traceStrings.intern(filepath.Base(ctx.fCtx.FilePath))
	site := irSsaGlobal(ctx)
	irWriteGlf(ctx, "%s = private constant %%type.error.site { ptr %s, ptr %s, i32 %d, i32 %d }\n",
		site.Repr, functionStr.Repr, fileStr.Repr, pos.Line, pos.Col)
	return site
}

func irThrowSsa(ctx *IrCtx, errSsa SsaName, fnDef *t.NodeFuncDef, pos t.FilePos) error {
	fieldSsa := irSsaLocal(ctx)
	compSsa := irSsaLocal(ctx)

	eqLabel := irSsaName(ctx)
	neqLabel := irSsaName(ctx)

	// get error code field
	irWritef(ctx, "  %s = extractvalue %%type.error %s, 1\n", fieldSsa.Repr, errSsa.Repr)

	// if errcode != 0
	irWritef(ctx, "  %s = icmp ne i32 %s, 0\n", compSsa.Repr, fieldSsa.Repr)
	irWritef(ctx, "  br i1 %s, label %%%s, label %%%s, !prof !9000\n", compSsa.Repr, neqLabel.Repr, eqLabel.Repr)

	// throw = err; return 0
	irWritef(ctx, "%s:\n", neqLabel.Repr)

	// Add source metadata only on the failing edge. The runtime uses bounded
	// static storage and retains recent propagation sites without allocating.
	site := irErrorSite(ctx, pos)
	tracedErrSsa := irSsaLocal(ctx)
	irWritef(ctx, "  %s = call %%type.error @magma.error.push(%%type.error %s, ptr %s)\n",
		tracedErrSsa.Repr, errSsa.Repr, site.Repr)
	errSsa = tracedErrSsa

	retValSsa := errSsa
	if fnDef.ReturnType.Throws {
		// generate throwing ret val
		var e error
		retValSsa, e = irMakeThrowingRetVal(ctx, fnDef.ReturnType, errSsa, SsaName{})
		if e != nil {
			return e
		}
	}

	irWrite(ctx, "  store i1 1, ptr %.defer.ret\n")
	irWrite(ctx, "  store i1 1, ptr %.defer.err\n")

	irWrite(ctx, "  store ")
	e := irThrowingType(ctx, fnDef.ReturnType)
	if e != nil {
		return e
	}
	irWrite(ctx, " ")
	irWritef(ctx, "%s", retValSsa.Repr)
	irWrite(ctx, ", ptr %.defer.rv\n")

	irJmpToDefer(ctx)
	irWritef(ctx, "%s:\n", eqLabel.Repr)
	return nil
}

func irStmtThrow(ctx *IrCtx, stmtThrow *t.NodeStmtThrow, fnDef *t.NodeFuncDef) error {
	if flattenType(stmtThrow.Expression.GetInferredType()) == "str" {
		strSsa, e := irExpression(ctx, stmtThrow.Expression.GetInferredType(), stmtThrow.Expression, false)
		if e != nil {
			return e
		}
		message := irSsaLocal(ctx)
		length64 := irSsaLocal(ctx)
		lengthBounded := irSsaLocal(ctx)
		lengthTooLong := irSsaLocal(ctx)
		length16 := irSsaLocal(ctx)
		errorMessage := irSsaLocal(ctx)
		errorCode := irSsaLocal(ctx)
		errorValue := irSsaLocal(ctx)
		irWritef(ctx, "  %s = extractvalue %%type.str %s, 0\n", message.Repr, strSsa.Repr)
		irWritef(ctx, "  %s = extractvalue %%type.str %s, 1\n", length64.Repr, strSsa.Repr)
		irWritef(ctx, "  %s = icmp ugt i64 %s, 65535\n", lengthTooLong.Repr, length64.Repr)
		irWritef(ctx, "  %s = select i1 %s, i64 65535, i64 %s\n", lengthBounded.Repr, lengthTooLong.Repr, length64.Repr)
		irWritef(ctx, "  %s = trunc i64 %s to i16\n", length16.Repr, lengthBounded.Repr)
		irWritef(ctx, "  %s = insertvalue %%type.error zeroinitializer, ptr %s, 0\n", errorMessage.Repr, message.Repr)
		irWritef(ctx, "  %s = insertvalue %%type.error %s, i32 1, 1\n", errorCode.Repr, errorMessage.Repr)
		irWritef(ctx, "  %s = insertvalue %%type.error %s, i16 %s, 3\n", errorValue.Repr, errorCode.Repr, length16.Repr)
		return irThrowSsa(ctx, errorValue, fnDef, stmtThrow.Pos)
	}
	exprSsa, e := irExpression(ctx, stmtThrow.Expression.GetInferredType(), stmtThrow.Expression, false)
	if e != nil {
		return e
	}

	return irThrowSsa(ctx, exprSsa, fnDef, stmtThrow.Pos)
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

	capturedError := irSsaLocal(ctx)
	irWritef(ctx, "  %s = alloca %%type.error\n", capturedError.Repr)
	failureLabel, endLabel := irSsaName(ctx), irSsaName(ctx)
	previousMode, previousSlot, previousFailure := ctx.ErrorMode, ctx.CapturedErrorSlot, ctx.ErrorFailureLabel
	ctx.ErrorMode, ctx.CapturedErrorSlot, ctx.ErrorFailureLabel = 2, capturedError, failureLabel
	valueSsa, e := irExpression(ctx, expr.ValueDef.Type, expr.Call, false)
	ctx.ErrorMode, ctx.CapturedErrorSlot, ctx.ErrorFailureLabel = previousMode, previousSlot, previousFailure
	if e != nil {
		return SsaName{}, e
	}

	// Success initializes the error binding to its zero value.
	irWrite(ctx, "  store ")
	e = irType(ctx, expr.ErrDef.Type)
	if e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, " ")
	irWrite(ctx, "zeroinitializer")
	irWritef(ctx, ", ptr %s\n", errPtr.Repr)

	if !isVoidType(expr.Call.InfType) {
		irWrite(ctx, "  store ")
		e = irType(ctx, expr.ValueDef.Type)
		if e != nil {
			return SsaName{}, e
		}
		irWrite(ctx, " ")
		irPossibleLitSsa(ctx, valueSsa)
		irWritef(ctx, ", ptr %s\n", valPtr.Repr)
	}
	irWritef(ctx, "  br label %%%s\n", endLabel.Repr)

	// Every throwing subcall in the RHS converges here.
	irWritef(ctx, "%s:\n", failureLabel.Repr)
	errVal := irSsaLocal(ctx)
	irWritef(ctx, "  %s = load %%type.error, ptr %s\n", errVal.Repr, capturedError.Repr)
	irWritef(ctx, "  store %%type.error %s, ptr %s\n", errVal.Repr, errPtr.Repr)
	irWritef(ctx, "  br label %%%s\n", endLabel.Repr)
	irWritef(ctx, "%s:\n", endLabel.Repr)

	return valPtr, nil
}

func irStatement(ctx *IrCtx, stmtNode t.NodeStatement, fnDef *t.NodeFuncDef) error {
	var e error

	ctx.IsTopLevel = true

	switch s := stmtNode.(type) {
	case *t.NodeStmtRet:
		e = irStmtReturn(ctx, s)
	case *t.NodeStmtExpr:
		_, e = irExpression(ctx, nil, s.Expression, true)
	case *t.NodeStmtThrow:
		e = irStmtThrow(ctx, s, fnDef)
	case *t.NodeLlvm:
		irLlvm(ctx, s)
		return nil
	case *t.NodeStmtIf:
		e = irStmtIf(ctx, s, fnDef)
	case *t.NodeStmtWhile:
		e = irStmtWhile(ctx, s, fnDef)
	case *t.NodeStmtFor:
		e = irStmtFor(ctx, s, fnDef)
	case *t.NodeStmtBounded:
		e = irStmtBounded(ctx, s, fnDef)
	case *t.NodeStmtUnsafe:
		e = irBody(ctx, &s.Body, fnDef, false)
	case *t.NodeStmtContinue:
		e = irStmtContinue(ctx, s)
	case *t.NodeStmtBreak:
		e = irStmtBreak(ctx, s)
	}
	return e
}

func irStmtBounded(ctx *IrCtx, stmt *t.NodeStmtBounded, fnDef *t.NodeFuncDef) error {
	if len(stmt.Predicates) == 0 || len(stmt.Proofs) == 0 {
		return fmt.Errorf("cannot lower bounded statement without validated range facts")
	}
	var condition SsaName
	for i, predicate := range stmt.Predicates {
		value, err := irExpression(ctx, predicate.GetInferredType(), predicate, false)
		if err != nil {
			return err
		}
		if i == 0 {
			condition = value
			continue
		}
		combined := irSsaLocal(ctx)
		irWritef(ctx, "  %s = and i1 ", combined.Repr)
		irPossibleLitSsa(ctx, condition)
		irWrite(ctx, ", ")
		irPossibleLitSsa(ctx, value)
		irWrite(ctx, "\n")
		condition = combined
	}
	bodyLabel, exitLabel := irSsaName(ctx), irSsaName(ctx)
	irWrite(ctx, "  br i1 ")
	irPossibleLitSsa(ctx, condition)
	irWritef(ctx, ", label %%%s, label %%%s\n%s:\n", bodyLabel.Repr, exitLabel.Repr, bodyLabel.Repr)
	if err := irBody(ctx, &stmt.Body, fnDef, false); err != nil {
		return err
	}
	irWritef(ctx, "  br label %%%s\n%s:\n", exitLabel.Repr, exitLabel.Repr)
	return nil
}

func irStmtIf(ctx *IrCtx, ifStmt *t.NodeStmtIf, fnDef *t.NodeFuncDef) error {
	condSsa, e := irExpression(ctx, ifStmt.CondExpr.GetInferredType(), ifStmt.CondExpr, false)
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

	e = irBody(ctx, &ifStmt.Body, fnDef, false)
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
			e = irBody(ctx, &n.Body, fnDef, false)
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

	ctx.LoopCondLbl = condLbl
	ctx.LoopExitLbl = exitLbl

	irWritef(ctx, "  br label %%%s\n", condLbl.Repr)
	irWritef(ctx, "%s:\n", condLbl.Repr)

	condSsa, e := irExpression(ctx, ifStmt.CondExpr.GetInferredType(), ifStmt.CondExpr, false)
	if e != nil {
		return e
	}

	eqLbl := irSsaName(ctx)

	irWrite(ctx, "  br i1 ")
	irPossibleLitSsa(ctx, condSsa)

	irWritef(ctx, ", label %%%s, label %%%s\n", eqLbl.Repr, exitLbl.Repr)

	irWritef(ctx, "%s:\n", eqLbl.Repr)

	*ctx.NestedLoopCnt = *ctx.NestedLoopCnt + 1

	e = irBody(ctx, &ifStmt.Body, fnDef, true)
	if e != nil {
		return e
	}

	*ctx.NestedLoopCnt = *ctx.NestedLoopCnt - 1

	irWritef(ctx, "  br label %%%s\n", condLbl.Repr)
	irWritef(ctx, "%s:\n", exitLbl.Repr)

	ctx.LoopCondLbl = SsaName{}
	ctx.LoopExitLbl = SsaName{}
	return nil
}

func forIndexName(stmt *t.NodeStmtFor, variable *t.NodeExprVarDef) *t.NodeExprName {
	return &t.NodeExprName{
		Tk:             stmt.Tk,
		Name:           variable.Name,
		InfType:        variable.Type,
		AssociatedNode: variable,
		Storage:        variable.Storage,
	}
}

func forIncrement(stmt *t.NodeStmtFor, variable *t.NodeExprVarDef) *t.NodeStmtDefer {
	left := forIndexName(stmt, variable)
	rightIndex := forIndexName(stmt, variable)
	one := &t.NodeExprLit{Tk: stmt.Tk, Value: "1", LitType: t.TokLitNum, InfType: variable.Type}
	add := &t.NodeExprBinary{
		Tk:          stmt.Tk,
		Operator:    t.KwPlus,
		Left:        rightIndex,
		Right:       one,
		InfType:     variable.Type,
		OperandType: variable.Type,
	}
	assignment := &t.NodeExprAssign{Tk: stmt.Tk, Left: left, Right: add, InfType: variable.Type}
	return &t.NodeStmtDefer{Expression: assignment}
}

func irStmtFor(ctx *IrCtx, stmt *t.NodeStmtFor, fnDef *t.NodeFuncDef) error {
	decl, ok := stmt.DeclExpr.(*t.NodeExprVarDefAssign)
	if !ok || decl.VarDef == nil {
		return fmt.Errorf("for loop has no initialized index declaration")
	}
	indexType := decl.VarDef.Type
	if _, e := irExpression(ctx, indexType, stmt.DeclExpr, true); e != nil {
		return e
	}
	bound, e := irExpression(ctx, indexType, stmt.BoundExpr, false)
	if e != nil {
		return e
	}
	bound, e = irCoerceNumeric(ctx, indexType, stmt.BoundExpr, bound)
	if e != nil {
		return e
	}

	condLabel := irSsaName(ctx)
	bodyLabel := irSsaName(ctx)
	exitLabel := irSsaName(ctx)
	previousCond := ctx.LoopCondLbl
	previousExit := ctx.LoopExitLbl
	ctx.LoopCondLbl = condLabel
	ctx.LoopExitLbl = exitLabel
	defer func() {
		ctx.LoopCondLbl = previousCond
		ctx.LoopExitLbl = previousExit
	}()

	irWritef(ctx, "  br label %%%s\n", condLabel.Repr)
	irWritef(ctx, "%s:\n", condLabel.Repr)
	index, e := irExpression(ctx, indexType, forIndexName(stmt, decl.VarDef), false)
	if e != nil {
		return e
	}
	comparison := irSsaLocal(ctx)
	predicate := "ult"
	if getNumDesc(indexType).IsSigned {
		predicate = "slt"
	}
	irWritef(ctx, "  %s = icmp %s ", comparison.Repr, predicate)
	if e := irType(ctx, indexType); e != nil {
		return e
	}
	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, index)
	irWrite(ctx, ", ")
	irPossibleLitSsa(ctx, bound)
	irWritef(ctx, "\n  br i1 %s, label %%%s, label %%%s\n", comparison.Repr, bodyLabel.Repr, exitLabel.Repr)
	irWritef(ctx, "%s:\n", bodyLabel.Repr)

	body := stmt.Body
	body.Statements = append([]t.NodeStatement{forIncrement(stmt, decl.VarDef)}, body.Statements...)
	*ctx.NestedLoopCnt = *ctx.NestedLoopCnt + 1
	e = irBody(ctx, &body, fnDef, true)
	*ctx.NestedLoopCnt = *ctx.NestedLoopCnt - 1
	if e != nil {
		return e
	}
	irWritef(ctx, "  br label %%%s\n", condLabel.Repr)
	irWritef(ctx, "%s:\n", exitLabel.Repr)
	return nil
}

func irBody(ctx *IrCtx, bodyNode *t.NodeBody, fnDef *t.NodeFuncDef, isLoopBody bool) error {
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
		var onErrorAfter SsaName
		if def.OnError {
			pending := irSsaLocal(&cpy)
			run := irSsaName(&cpy)
			onErrorAfter = irSsaName(&cpy)
			irWritef(&cpy, "  %s = load i1, ptr %%.defer.err\n", pending.Repr)
			irWritef(&cpy, "  br i1 %s, label %%%s, label %%%s\n", pending.Repr, run.Repr, onErrorAfter.Repr)
			irWritef(&cpy, "%s:\n", run.Repr)
		}
		if !def.IsBody {
			_, e := irExpression(&cpy, nil, def.Expression, false)
			if e != nil {
				return e
			}
		} else {
			for _, stmt := range def.Body.Statements {
				e := irStatement(&cpy, stmt, fnDef)
				if e != nil {
					return e
				}
			}
		}
		if def.OnError {
			irWritef(&cpy, "  br label %%%s\n", onErrorAfter.Repr)
			irWritef(&cpy, "%s:\n", onErrorAfter.Repr)
		}
	}

	//if defLen == 0 {
	irWritef(&cpy, "  br label %%.defer.%d.base\n", cpy.CurrNestedScopeIdx)
	irWritef(&cpy, ".defer.%d.base:\n", cpy.CurrNestedScopeIdx)
	//}

	/*
		shouldRetSsa := irSsaName(ctx)
		afterSsa := irSsaName(ctx)
		irWritef(&cpy, "  %%%s = load i1, ptr %%.defer.ret\n", shouldRetSsa.Repr)
		irWritef(&cpy, "  br i1 %%%s, label %%.defer.%d.%d, label %%%s\n", shouldRetSsa.Repr, ctx.CurrNestedScopeIdx, ctx.CurrDeferIdx, afterSsa.Repr)
		irWritef(&cpy, "%s:\n", afterSsa.Repr)*/

	if isLoopBody {
		shouldBrkSsa := irSsaLocal(ctx)
		brkLbl := irSsaName(ctx)
		checkContLbl := irSsaName(ctx)
		irWritef(&cpy, "  %s = load i1, ptr %%.defer.brk\n", shouldBrkSsa.Repr)
		irWritef(&cpy, "  br i1 %s, label %%%s, label %%%s\n", shouldBrkSsa.Repr, brkLbl.Repr, checkContLbl.Repr)
		irWritef(&cpy, "%s:\n", brkLbl.Repr)
		irWrite(&cpy, "  store i1 0, ptr %.defer.brk\n")
		irWritef(&cpy, "  br label %%%s\n", ctx.LoopExitLbl.Repr)
		irWritef(&cpy, "%s:\n", checkContLbl.Repr)

		shouldContSsa := irSsaLocal(ctx)
		contLbl := irSsaName(ctx)
		afterLbl := irSsaName(ctx)
		irWritef(&cpy, "  %s = load i1, ptr %%.defer.cont\n", shouldContSsa.Repr)
		irWritef(&cpy, "  br i1 %s, label %%%s, label %%%s\n", shouldContSsa.Repr, contLbl.Repr, afterLbl.Repr)
		irWritef(&cpy, "%s:\n", contLbl.Repr)
		irWrite(&cpy, "  store i1 0, ptr %.defer.cont\n")
		irWritef(&cpy, "  br label %%%s\n", ctx.LoopCondLbl.Repr)
		irWritef(&cpy, "%s:\n", afterLbl.Repr)
	}

	irJmpToParentDeferOnControl(&cpy, ctx)

	irWrite(ctx, cpy.bld.Head.String())
	irWrite(ctx, cpy.bld.Body.String())
	irWrite(ctx, cpy.bld.Tail.String())
	return nil
}
