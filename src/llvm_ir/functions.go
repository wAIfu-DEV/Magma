package llvmir

import (
	t "Magma/src/types"
	"bytes"
)

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

	for i, arg := range fnDef.Class.ArgsNode.Args {
		if fnDef.IsMember && i == 0 {
			continue
		}
		irWritef(&cpy, "  %%%s.addr = alloca ", arg.Name)
		if e := irType(&cpy, arg.TypeNode); e != nil {
			return e
		}
		irWrite(&cpy, "\n  store ")
		if e := irType(&cpy, arg.TypeNode); e != nil {
			return e
		}
		irWritef(&cpy, " %%%s, ptr %%%s.addr\n", arg.Name, arg.Name)
	}

	if !(isVoidType(fnDef.ReturnType) && !fnDef.ReturnType.Throws) {
		irWrite(&cpy, "  %.defer.rv = alloca ")
		e := irThrowingType(&cpy, fnDef.ReturnType)
		if e != nil {
			return e
		}
		irWrite(&cpy, "\n")
	}

	irWrite(&cpy, "  %.defer.ret = alloca i1\n")
	irWrite(&cpy, "  %.defer.err = alloca i1\n")
	irWrite(&cpy, "  %.defer.brk = alloca i1\n")
	irWrite(&cpy, "  %.defer.cont = alloca i1\n")

	irWrite(&cpy, "  store i1 0, ptr %.defer.ret\n")
	irWrite(&cpy, "  store i1 0, ptr %.defer.err\n")
	irWrite(&cpy, "  store i1 0, ptr %.defer.brk\n")
	irWrite(&cpy, "  store i1 0, ptr %.defer.cont\n")

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
	var deferred []*t.NodeStmtDefer

	for _, stmt := range bodyNode.Statements {
		switch n := stmt.(type) {
		case *t.NodeStmtRet:
			foundRet = true
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

		irWritef(&cpy, "  br label %%.defer.%d.%d\n", ctx.CurrNestedScopeIdx, revIdx)
		irWritef(&cpy, ".defer.%d.%d:\n", ctx.CurrNestedScopeIdx, revIdx)

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
				switch stmt.(type) {
				case *t.NodeStmtRet:
					foundRet = true
				}

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
	irWrite(&cpy, "  br label %.defer.0.base\n")
	irWrite(&cpy, ".defer.0.base:\n")
	//}

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
	irWrite(ctx, "; Entry point\n")
	if ctx.Shared.Target.OS == "windows" {
		irWrite(ctx, "declare dllimport ptr @GetProcessHeap()\n")
		irWrite(ctx, "declare dllimport ptr @HeapAlloc(ptr, i32, i64)\n")
		irWrite(ctx, "declare dllimport i32 @HeapFree(ptr, i32, ptr)\n")
		irWrite(ctx, "declare dllimport i32 @SetConsoleOutputCP(i32)\n")
		irWrite(ctx, "declare dllimport i32 @WideCharToMultiByte(i32, i32, ptr, i32, ptr, i32, ptr, ptr)\n\n")
		irWrite(ctx, `define internal i1 @magma.argsFromUtf16(i32 %argc, ptr %argv, ptr %buf) {
entry:
  %heap = call ptr @GetProcessHeap()
  %argc64 = sext i32 %argc to i64
  br label %loop

loop:
  %i = phi i64 [ 0, %entry ], [ %next, %store ]
  %done = icmp eq i64 %i, %argc64
  br i1 %done, label %success, label %convert

convert:
  %arg.slot = getelementptr ptr, ptr %argv, i64 %i
  %wide = load ptr, ptr %arg.slot
  %size = call i32 @WideCharToMultiByte(i32 65001, i32 128, ptr %wide, i32 -1, ptr null, i32 0, ptr null, ptr null)
  %size.ok = icmp sgt i32 %size, 0
  br i1 %size.ok, label %allocate, label %fail

allocate:
  %size64 = zext i32 %size to i64
  %bytes = call ptr @HeapAlloc(ptr %heap, i32 0, i64 %size64)
  %allocated = icmp ne ptr %bytes, null
  br i1 %allocated, label %encode, label %fail

encode:
  %written = call i32 @WideCharToMultiByte(i32 65001, i32 128, ptr %wide, i32 -1, ptr %bytes, i32 %size, ptr null, ptr null)
  %encoded = icmp eq i32 %written, %size
  br i1 %encoded, label %store, label %free.current

free.current:
  call i32 @HeapFree(ptr %heap, i32 0, ptr %bytes)
  br label %fail

store:
  %length32 = sub i32 %size, 1
  %length = zext i32 %length32 to i64
  %elem = getelementptr %type.str, ptr %buf, i64 %i
  %str0 = insertvalue %type.str zeroinitializer, ptr %bytes, 0
  %str1 = insertvalue %type.str %str0, i64 %length, 1
  store %type.str %str1, ptr %elem
  %next = add i64 %i, 1
  br label %loop

fail:
  br label %cleanup

cleanup:
  %j = phi i64 [ 0, %fail ], [ %j.next, %cleanup.body ]
  %cleaned = icmp eq i64 %j, %i
  br i1 %cleaned, label %failure, label %cleanup.body

cleanup.body:
  %old.elem = getelementptr %type.str, ptr %buf, i64 %j
  %old = load %type.str, ptr %old.elem
  %old.bytes = extractvalue %type.str %old, 0
  call i32 @HeapFree(ptr %heap, i32 0, ptr %old.bytes)
  %j.next = add i64 %j, 1
  br label %cleanup

failure:
  ret i1 false

success:
  ret i1 true
}

define internal void @magma.freeUtf8Args(i32 %argc, ptr %buf) {
entry:
  %heap = call ptr @GetProcessHeap()
  %argc64 = sext i32 %argc to i64
  br label %loop

loop:
  %i = phi i64 [ 0, %entry ], [ %next, %body ]
  %done = icmp eq i64 %i, %argc64
  br i1 %done, label %finish, label %body

body:
  %elem = getelementptr %type.str, ptr %buf, i64 %i
  %arg = load %type.str, ptr %elem
  %bytes = extractvalue %type.str %arg, 0
  call i32 @HeapFree(ptr %heap, i32 0, ptr %bytes)
  %next = add i64 %i, 1
  br label %loop

finish:
  ret void
}

`)
		irWrite(ctx, "define i32 @wmain(i32 %argc, ptr %argv) {\n")
	} else {
		irWrite(ctx, "define i32 @main(i32 %argc, ptr %argv) {\n")
	}
	irWrite(ctx, "entry:\n")
	if ctx.Shared.Target.OS == "windows" {
		irWrite(ctx, "  %console.utf8 = call i32 @SetConsoleOutputCP(i32 65001)\n")
	}

	hasArgs := false

	if len(mainFnDef.Class.ArgsNode.Args) > 0 {
		first := mainFnDef.Class.ArgsNode.Args[0]

		// TODO check for slice type
		if first.Name == "args" {
			hasArgs = true

			irWrite(ctx, "  %arr = alloca %type.str, i32 %argc\n")
			if ctx.Shared.Target.OS == "windows" {
				irWrite(ctx, "  %args.ok = call i1 @magma.argsFromUtf16(i32 %argc, ptr %argv, ptr %arr)\n")
				irWrite(ctx, "  br i1 %args.ok, label %args.ready, label %args.failed\n")
				irWrite(ctx, "args.failed:\n")
				irWrite(ctx, "  ret i32 1\n")
				irWrite(ctx, "args.ready:\n")
				irWrite(ctx, "  %argc64 = sext i32 %argc to i64\n")
				irWrite(ctx, "  %a0 = insertvalue %type.slice zeroinitializer, ptr %arr, 0\n")
				irWrite(ctx, "  %a = insertvalue %type.slice %a0, i64 %argc64, 1\n")
			} else {
				irWrite(ctx, "  %a = call %type.slice @magma.argsToSlice(i32 %argc, ptr %argv, ptr %arr)\n")
			}
		}
	}

	if mainFnDef.ReturnType.Throws {
		if hasArgs {
			irWritef(ctx, "  %%r = call { %%type.error } @%s.main(%%type.slice %%a)\n", ctx.fCtx.MainPckgName)
		} else {
			irWritef(ctx, "  %%r = call { %%type.error } @%s.main()\n", ctx.fCtx.MainPckgName)
		}
		irWrite(ctx, "  %e = extractvalue { %type.error } %r, 0\n")
		irWrite(ctx, "  %ecd = extractvalue %type.error %e, 2\n")
		irWrite(ctx, "  %isnz = icmp ne i32 %ecd, 0\n")
		irWrite(ctx, "  br i1 %isnz, label %enz, label %ez, !prof !9000\n")
		irWrite(ctx, "enz:\n")
		irWrite(ctx, "  call void @magma.error.print(%type.error %e)\n")
		if hasArgs && ctx.Shared.Target.OS == "windows" {
			irWrite(ctx, "  call void @magma.freeUtf8Args(i32 %argc, ptr %arr)\n")
		}
		irWrite(ctx, "  ret i32 %ecd\n")
		irWrite(ctx, "ez:\n")
	} else {
		if hasArgs {
			irWritef(ctx, "  call void @%s.main(%%type.slice %%a)\n", ctx.fCtx.MainPckgName)
		} else {
			irWritef(ctx, "  call void @%s.main()\n", ctx.fCtx.MainPckgName)
		}
	}
	if hasArgs && ctx.Shared.Target.OS == "windows" {
		irWrite(ctx, "  call void @magma.freeUtf8Args(i32 %argc, ptr %arr)\n")
	}
	irWrite(ctx, "  ret i32 0\n")
	irWrite(ctx, "}\n\n")
	return nil
}

func irFunDefAliased(ctx *IrCtx, fnDefNode *t.NodeFuncDef) error {
	return irCABIExternalDeclaration(ctx, fnDefNode)
}

func irFuncDef(ctx *IrCtx, fnDefNode *t.NodeFuncDef) error {
	if fnDefNode.NoAliasName != "" {
		// func declared elsewhere, just emit declaration
		return irFunDefAliased(ctx, fnDefNode)
	}

	if ctx.fCtx.PackageName == ctx.fCtx.MainPckgName && fnDefNode.IsEntryPoint {
		e := irMainWrapper(ctx, fnDefNode)
		if e != nil {
			return e
		}
	}

	// Magma implementations are private to the generated LLVM module. Native
	// entry points and explicit export wrappers are emitted separately with
	// external linkage. Keeping implementations internal allows LLVM to inline
	// and eliminate functions which are unreachable from those external roots.
	irWrite(ctx, "define internal ")
	e := irThrowingType(ctx, fnDefNode.ReturnType)
	if e != nil {
		return e
	}

	irWrite(ctx, " @")
	irWrite(ctx, fnDefNode.AbsName)

	e = irArgsList(ctx, &fnDefNode.Class.ArgsNode, fnDefNode.IsMember)
	if e != nil {
		return e
	}

	ctx.CurrFunc = fnDefNode
	ctx.localSlots = map[*t.NodeExprVarDef]SsaName{}
	assignLocalIrNames(ctx, &fnDefNode.Body)

	if len(fnDefNode.Body.Statements) > 5 {
		irWrite(ctx, " inlinehint ")
	} else {
		irWrite(ctx, " alwaysinline ")
	}

	e = irFuncBody(ctx, &fnDefNode.Body, fnDefNode)
	if e != nil {
		return e
	}
	if fnDefNode.ExportName != "" {
		if e := irExportWrapper(ctx, fnDefNode); e != nil {
			return e
		}
	}
	ctx.CurrFunc = nil
	return nil
}

func irExportWrapper(ctx *IrCtx, fn *t.NodeFuncDef) error {
	return irCABIExportWrapper(ctx, fn)
}

func assignExprIrNames(ctx *IrCtx, expr t.NodeExpr) {
	switch n := expr.(type) {
	case *t.NodeExprVarDef:
		assignLocalIrName(ctx, n)
	case *t.NodeExprVarDefAssign:
		assignExprIrNames(ctx, n.VarDef)
	case *t.NodeExprDestructureAssign:
		assignExprIrNames(ctx, &n.ValueDef)
		assignExprIrNames(ctx, &n.ErrDef)
	}
}

func assignLocalIrName(ctx *IrCtx, variable *t.NodeExprVarDef) {
	if variable == nil || variable.Storage != t.VariableStorageLocal {
		return
	}
	if ctx.localSlots == nil {
		ctx.localSlots = map[*t.NodeExprVarDef]SsaName{}
	}
	if _, exists := ctx.localSlots[variable]; !exists {
		ctx.localSlots[variable] = irSsaLocal(ctx)
	}
}

func assignLocalIrNames(ctx *IrCtx, body *t.NodeBody) {
	for _, statement := range body.Statements {
		switch n := statement.(type) {
		case *t.NodeStmtExpr:
			assignExprIrNames(ctx, n.Expression)
		case *t.NodeStmtIf:
			assignLocalIrNames(ctx, &n.Body)
			for next := n.NextCondStmt; next != nil; {
				switch branch := next.(type) {
				case *t.NodeStmtIf:
					assignLocalIrNames(ctx, &branch.Body)
					next = branch.NextCondStmt
				case *t.NodeStmtElse:
					assignLocalIrNames(ctx, &branch.Body)
					next = nil
				default:
					next = nil
				}
			}
		case *t.NodeStmtElse:
			assignLocalIrNames(ctx, &n.Body)
		case *t.NodeStmtWhile:
			assignLocalIrNames(ctx, &n.Body)
		case *t.NodeStmtDefer:
			if n.IsBody {
				assignLocalIrNames(ctx, &n.Body)
			}
		}
	}
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
	case *t.NodeExprVarDef:
		return irGlVarDef(ctx, g)
	case *t.NodeConstDef:
		return irConstDef(ctx, g)
	}
	return nil
}
