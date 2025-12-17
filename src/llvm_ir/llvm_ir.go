package llvmir

import (
	magmatypes "Magma/src/magma_types"
	t "Magma/src/types"
	"fmt"
	"maps"
	"strconv"
	"strings"
	"sync"
)

type SsaName string

type IrCtx struct {
	fCtx    *t.FileCtx
	builder *strings.Builder
	nextSsa int
}

func irSsaName(ctx *IrCtx) SsaName {
	name := strconv.Itoa(ctx.nextSsa)
	ctx.nextSsa++
	return SsaName(name)
}

func irWritef(ctx *IrCtx, format string, a ...any) {
	ctx.builder.WriteString(fmt.Sprintf(format, a...))
}

func irVarDefAssign(ctx *IrCtx, vda *t.NodeExprVarDefAssign) (SsaName, error) {
	assignSsa, e := irExpression(ctx, vda.AssignExpr)
	if e != nil {
		return SsaName(""), e
	}

	allocSsa := irSsaName(ctx)
	ctx.builder.WriteString(fmt.Sprintf("  %%%s = alloca ", allocSsa))
	e = irType(ctx, &vda.VarDef.Type)
	if e != nil {
		return SsaName(""), e
	}
	ctx.builder.WriteString("\n")

	ctx.builder.WriteString("  store ")
	e = irType(ctx, &vda.VarDef.Type)
	if e != nil {
		return SsaName(""), e
	}
	ctx.builder.WriteString(fmt.Sprintf(" %%%s, ", assignSsa))
	e = irType(ctx, &vda.VarDef.Type)
	if e != nil {
		return SsaName(""), e
	}
	ctx.builder.WriteString(fmt.Sprintf(" %%%s\n", allocSsa))
	return allocSsa, nil
}

func irExprFuncCall(ctx *IrCtx, fnCall *t.NodeExprCall) (SsaName, error) {
	ssaName := irSsaName(ctx)

	argsSsa := make([]SsaName, len(fnCall.Args))
	for i, expr := range fnCall.Args {
		exprSsa, e := irExpression(ctx, expr)
		if e != nil {
			return SsaName(""), e
		}
		argsSsa[i] = exprSsa
	}

	ctx.builder.WriteString(fmt.Sprintf("  %%%s = call ", ssaName))

	// TODO: print type
	ctx.builder.WriteString("<type> ")

	switch expr := fnCall.Callee.(type) {
	case *t.NodeExprName:
		e := irName(ctx, expr.Name, true)
		if e != nil {
			return SsaName(""), e
		}
	default:
		ctx.builder.WriteString("<name>")
	}

	ctx.builder.WriteString("(")

	bound := len(argsSsa)
	for i, ssa := range argsSsa {
		ctx.builder.WriteString(fmt.Sprintf("%%%s", ssa))
		if bound < i {
			ctx.builder.WriteString(", ")
		}
	}

	ctx.builder.WriteString(")\n")
	return ssaName, nil
}

func irExpression(ctx *IrCtx, expr t.NodeExpr) (SsaName, error) {
	switch ne := expr.(type) {
	case *t.NodeExprVarDefAssign:
		return irVarDefAssign(ctx, ne)
	case *t.NodeExprCall:
		return irExprFuncCall(ctx, ne)
	}
	return SsaName(""), nil
}

func irStmtReturn(ctx *IrCtx, stmtRet *t.NodeStmtRet) error {
	// TODO: lower expression
	switch stmtRet.Expression.(type) {
	case *t.NodeExprVoid:
		ctx.builder.WriteString("  ret <type>\n")
	default:
		ssa, e := irExpression(ctx, stmtRet.Expression)
		if e != nil {
			return e
		}
		ctx.builder.WriteString(fmt.Sprintf("  ret <type> %%%s\n", ssa))
	}
	return nil
}

func irStatement(ctx *IrCtx, stmtNode t.NodeStatement) error {
	var e error

	switch s := stmtNode.(type) {
	case *t.NodeStmtRet:
		e = irStmtReturn(ctx, s)
	case *t.NodeStmtExpr:
		_, e = irExpression(ctx, s.Expression)
	}

	if e != nil {
		return e
	}
	return nil
}

func irBody(ctx *IrCtx, bodyNode *t.NodeBody) error {
	ctx.builder.WriteString("{\n")

	for _, stmt := range bodyNode.Statements {
		e := irStatement(ctx, stmt)
		if e != nil {
			return e
		}
	}
	ctx.builder.WriteString("}\n\n")
	return nil
}

func irMainWrapper(ctx *IrCtx, mainFnDef *t.NodeFuncDef) error {
	ctx.builder.WriteString("; Entry point\n")
	ctx.builder.WriteString("define i32 @main(i32 %argc, ptr %argv) {\n")
	ctx.builder.WriteString("entry:\n")

	// TODO: create args slice and pass to main

	if mainFnDef.ReturnType.Throws {
		// TODO: alloca error struct and pass it as first arg
	} else {
		ctx.builder.WriteString("  call void main.main()\n")
	}
	ctx.builder.WriteString("  ret i32 0\n")
	ctx.builder.WriteString("}\n\n")
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

	if ctx.fCtx.PackageName == "main" && singleName == "main" {
		e := irMainWrapper(ctx, fnDefNode)
		if e != nil {
			return e
		}
	}

	ctx.builder.WriteString("define ")
	e := irType(ctx, &fnDefNode.ReturnType)
	if e != nil {
		return e
	}

	ctx.builder.WriteString(" @")
	e = irName(ctx, fnDefNode.Class.NameNode, true)
	if e != nil {
		return e
	}

	e = irArgsList(ctx, &fnDefNode.Class.ArgsNode, isMemberFunc, fnDefNode.ReturnType.Throws)
	if e != nil {
		return e
	}

	ctx.builder.WriteString(" ")
	e = irBody(ctx, &fnDefNode.Body)
	if e != nil {
		return e
	}
	return nil
}

func irArg(ctx *IrCtx, argNode *t.NodeArg) error {
	e := irType(ctx, &argNode.TypeNode)
	if e != nil {
		return e
	}

	ctx.builder.WriteString(" %")
	ctx.builder.WriteString(argNode.Name)
	return nil
}

func irArgsList(ctx *IrCtx, argListNode *t.NodeArgList, thisArg bool, throwArg bool) error {
	ctx.builder.WriteString("(")
	bound := len(argListNode.Args)

	if thisArg {
		ctx.builder.WriteString("ptr %this")
		if bound > 0 || throwArg {
			ctx.builder.WriteString(", ")
		}
	}

	if throwArg {
		ctx.builder.WriteString("ptr %throw")
		if bound > 0 {
			ctx.builder.WriteString(", ")
		}
	}

	for i, a := range argListNode.Args {
		e := irArg(ctx, &a)
		if e != nil {
			return e
		}

		if i < bound-1 {
			ctx.builder.WriteString(", ")
		}
	}

	ctx.builder.WriteString(")")
	return nil
}

func irGlobalDecl(ctx *IrCtx, glDeclNode t.NodeGlobalDecl) error {
	switch g := glDeclNode.(type) {
	case *t.NodeFuncDef:
		e := irFuncDef(ctx, g)
		if e != nil {
			return e
		}
	}
	return nil
}

func irNameSingle(ctx *IrCtx, nameNode *t.NodeNameSingle, withPackage bool) error {
	if withPackage {
		ctx.builder.WriteString(ctx.fCtx.PackageName)
		ctx.builder.WriteString(".")
	}
	ctx.builder.WriteString(nameNode.Name)
	return nil
}

func irNameComposite(ctx *IrCtx, nameNode *t.NodeNameComposite, withPackage bool) error {
	if withPackage {
		first := nameNode.Parts[0]

		// if not imported package, prepend with <thispackage>.
		_, ok := ctx.fCtx.ImportAlias[first]
		if !ok {
			ctx.builder.WriteString(ctx.fCtx.PackageName)
			ctx.builder.WriteString(".")
		}
	}

	bound := len(nameNode.Parts)
	for i, n := range nameNode.Parts {
		ctx.builder.WriteString(n)
		if i < bound-1 {
			ctx.builder.WriteString(".")
		}
	}

	return nil
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

func irType(ctx *IrCtx, typeNode *t.NodeType) error {
	switch tn := typeNode.KindNode.(type) {
	case *t.NodeTypeNamed:
		switch n := tn.NameNode.(type) {
		case *t.NodeNameSingle:
			// TODO: check if intrinsic type
			_, ok := magmatypes.BasicTypes[n.Name]
			if ok {
				ctx.builder.WriteString(magmatypes.BasicTypes[n.Name])
				return nil
			}
		}

		ctx.builder.WriteString("%struct.")
		e := irName(ctx, tn.NameNode, true)
		if e != nil {
			return e
		}
		return nil
	}
	ctx.builder.WriteString("<invalid type>")
	return nil
}

func irDefineStruct(ctx *IrCtx, structNode *t.NodeStructDef) error {
	ctx.builder.WriteString("%struct.")

	e := irName(ctx, structNode.Class.NameNode, true)
	if e != nil {
		return e
	}
	ctx.builder.WriteString(" = type { ")

	bound := len(structNode.Class.ArgsNode.Args)
	for i, field := range structNode.Class.ArgsNode.Args {
		e = irType(ctx, &field.TypeNode)
		if e != nil {
			return e
		}

		if i < bound-1 {
			ctx.builder.WriteString(", ")
		}
	}

	ctx.builder.WriteString(" }\n")
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

func irWriteModule(fCtx *t.FileCtx, builder *strings.Builder) error {
	ctx := &IrCtx{
		fCtx:    fCtx,
		builder: builder,
	}
	ctx.builder.Grow(512)

	irWritef(ctx, "; File=\"%s\"\n", ctx.fCtx.FilePath)
	irWritef(ctx, "; Module=\"%s\"\n\n", ctx.fCtx.PackageName)

	ctx.builder.WriteString("; Defined Types\n")
	e := irGlobalStructDefs(ctx, &fCtx.GlNode)
	if e != nil {
		return e
	}

	ctx.builder.WriteString("\n; Code\n")
	e = irGlobal(ctx, &fCtx.GlNode)
	if e != nil {
		return e
	}
	return nil
}

func IrWrite(shared *t.SharedState) (string, error) {
	// creates a shallow copy of shared.Files, will prevent any race condition
	// if it were ever to be modified, which it shouldn't.
	shared.FilesM.Lock()
	filesMap := maps.Clone(shared.Files)
	shared.FilesM.Unlock()

	// write header
	headBld := &strings.Builder{}
	headBld.WriteString("; Magma\n\n")
	headBld.WriteString("; Basic Types\n")
	magmatypes.WriteIrBasicTypes(headBld)
	header := headBld.String()

	// result receiver
	type resStr struct {
		S string
		E error
	}
	results := make([]resStr, len(filesMap)+1)
	results[0] = resStr{S: header}

	// multithreaded writing per-module

	wg := sync.WaitGroup{}
	wg.Add(len(filesMap))

	i := 1
	for _, v := range filesMap {

		localI := i
		go func(idx int) {
			defer wg.Done()

			// module local builder
			moduleBld := &strings.Builder{}
			e := irWriteModule(v, moduleBld)
			if e != nil {
				results[idx] = resStr{E: e}
				return
			}
			results[idx] = resStr{S: moduleBld.String()}
		}(localI)

		i++
	}

	// join threads
	wg.Wait()

	// process results
	irStrings := []string{}
	for _, r := range results {
		if r.E != nil {
			return "", r.E
		}
		irStrings = append(irStrings, r.S)
	}
	return strings.Join(irStrings, "\n"), nil
}
