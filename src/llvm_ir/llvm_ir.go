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

type SsaName struct {
	Repr      string
	IsLiteral bool
}

func ssaName(name string) SsaName {
	return SsaName{Repr: name}
}

type IrCtx struct {
	fCtx         *t.FileCtx
	glBuilder    *strings.Builder
	builder      *strings.Builder
	scopeHeadBld *strings.Builder
	nextSsa      *int
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
	ctx.builder.WriteString(text)
}

func irWritef(ctx *IrCtx, format string, a ...any) {
	fmt.Fprintf(ctx.builder, format, a...)
}

func irWriteHd(ctx *IrCtx, text string) {
	ctx.scopeHeadBld.WriteString(text)
}

func irWriteHdf(ctx *IrCtx, format string, a ...any) {
	fmt.Fprintf(ctx.scopeHeadBld, format, a...)
}

func irWriteGl(ctx *IrCtx, text string) {
	ctx.glBuilder.WriteString(text)
}

func irWriteGlf(ctx *IrCtx, format string, a ...any) {
	fmt.Fprintf(ctx.glBuilder, format, a...)
}

func irVarDef(ctx *IrCtx, vd *t.NodeExprVarDef) (SsaName, error) {
	allocSsa := irNameSsa(ctx, vd.Name, false)

	irWriteHdf(ctx, "  %%%s = alloca ", allocSsa.Repr)

	cpy := *ctx
	cpy.builder = ctx.scopeHeadBld

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

	sizeFieldSsa := irSsaName(ctx)
	strFieldSsa := irSsaName(ctx)

	constLen := len(litStr.Value) + 1

	cleanStr := strings.ReplaceAll(litStr.Value, "\n", "\\0A")

	irWriteGlf(ctx, "@%s = private constant [%d x i8] c\"%s\\00\"\n", constSsa.Repr, constLen, cleanStr)

	irWritef(ctx, "  %%%s = insertvalue %%type.str undef, i64 %d, 0\n", sizeFieldSsa.Repr, constLen-1)
	irWritef(ctx, "  %%%s = insertvalue %%type.str %%%s, ptr @%s, 1\n", strFieldSsa.Repr, sizeFieldSsa.Repr, constSsa.Repr)

	return strFieldSsa, nil
}

func irExprLitNum(ctx *IrCtx, litNum *t.NodeExprLit) (SsaName, error) {
	ssa := ssaName(litNum.Value)
	ssa.IsLiteral = true
	return ssa, nil
}

func irExprLit(ctx *IrCtx, lit *t.NodeExprLit) (SsaName, error) {
	switch lit.LitType {
	case t.TokLitStr:
		return irExprLitStr(ctx, lit)
	case t.TokLitNum:
		return irExprLitNum(ctx, lit)
	}
	return ssaName(""), nil
}

func irExprName(ctx *IrCtx, nameExpr *t.NodeExprName) (SsaName, error) {
	if nameExpr.IsSsa {
		return irNameSsa(ctx, nameExpr.Name, false), nil
	}

	ptrSsa := irNameSsa(ctx, nameExpr.Name, false)
	ssa := irSsaName(ctx)

	var typeNd *t.NodeType = nil

	switch n := nameExpr.AssociatedNode.(type) {
	case *t.NodeExprVarDef:
		typeNd = n.Type
	case *t.NodeExprVarDefAssign:
		typeNd = n.VarDef.Type
	}

	irWritef(ctx, "  %%%s = load ", ssa.Repr)

	e := irType(ctx, typeNd)
	if e != nil {
		return ssaName(""), e
	}

	irWritef(ctx, ", ptr %%%s\n", ptrSsa.Repr)
	return ssa, nil
}

func irExpression(ctx *IrCtx, expr t.NodeExpr) (SsaName, error) {
	switch ne := expr.(type) {
	case *t.NodeExprVarDefAssign:
		return irVarDefAssign(ctx, ne)
	case *t.NodeExprVarDef:
		return irVarDef(ctx, ne)
	case *t.NodeExprCall:
		return irExprFuncCall(ctx, ne)
	case *t.NodeExprLit:
		return irExprLit(ctx, ne)
	case *t.NodeExprName:
		return irExprName(ctx, ne)
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
	// TODO: implement throw lowering
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
	}
	return e
}

func irFuncBody(ctx *IrCtx, bodyNode *t.NodeBody, fnDef *t.NodeFuncDef) error {
	irWrite(ctx, "{\n")

	bdyHeadBld := &strings.Builder{}
	bdyBld := &strings.Builder{}

	// making du ctx to redirect writes
	cpy := *ctx
	cpy.scopeHeadBld = bdyHeadBld
	cpy.builder = bdyBld

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

	irWrite(ctx, bdyHeadBld.String())
	irWrite(ctx, "\n")
	irWrite(ctx, bdyBld.String())

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
	irWrite(ctx, "; Entry point\n")
	irWrite(ctx, "define i32 @main(i32 %argc, ptr %argv) {\n")
	irWrite(ctx, "entry:\n")

	// TODO: create args slice and pass to main

	if mainFnDef.ReturnType.Throws {
		irWrite(ctx, "  %e = alloca %type.error\n")
		irWrite(ctx, "  store %type.error zeroinitializer, ptr %e\n")
		irWritef(ctx, "  call void @%s.main(ptr %%e)\n", ctx.fCtx.MainPckgName)
		irWrite(ctx, "  %efld1 = getelementptr %type.error, ptr %e, i32 0, i32 0\n")
		irWrite(ctx, "  %ecd = load i32, ptr %efld1\n")
		irWrite(ctx, "  %isnz = icmp ne i32 %ecd, 0\n")
		irWrite(ctx, "  br i1 %isnz, label %enz, label %ez\n")
		irWrite(ctx, "enz:\n")
		irWrite(ctx, "  ret i32 %ecd\n")
		irWrite(ctx, "ez:\n")
	} else {
		irWrite(ctx, "  call void @main.main()\n")
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
			if withPackage {
				first := nameNode.Parts[0]

				// if not imported package, prepend with <thispackage>.
				alias, ok := ctx.fCtx.GlNode.ImportAlias[first]
				if !ok {
					irWrite(ctx, ctx.fCtx.PackageName)
				} else {
					irWrite(ctx, alias)
				}
			} else {
				irWrite(ctx, n)
			}
		} else {
			irWrite(ctx, n)
		}

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

func irType(ctx *IrCtx, typeNode *t.NodeType) error {
	if typeNode == nil {
		irWrite(ctx, "<null type node>")
		return nil
	}

	switch tn := typeNode.KindNode.(type) {
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

func irDefineStruct(ctx *IrCtx, structNode *t.NodeStructDef) error {
	irWriteGl(ctx, "%struct.")

	// making dud ctx to redirect name IR to global writer
	cpy := *ctx
	cpy.builder = cpy.glBuilder

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

func irWriteModule(fCtx *t.FileCtx, builder *strings.Builder, glBld *strings.Builder) error {
	nextSsa := 0

	ctx := &IrCtx{
		fCtx:      fCtx,
		builder:   builder,
		glBuilder: glBld,
		nextSsa:   &nextSsa,
	}
	ctx.builder.Grow(512)

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
			glBld := &strings.Builder{}
			e := irWriteModule(v, moduleBld, glBld)
			if e != nil {
				results[idx] = resStr{E: e}
				return
			}
			results[idx] = resStr{S: glBld.String() + moduleBld.String()}
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
