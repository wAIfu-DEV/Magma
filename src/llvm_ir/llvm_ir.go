package llvmir

import (
	llvmfragments "Magma/src/llvm_fragments"
	magmatypes "Magma/src/magma_types"
	t "Magma/src/types"
	"bytes"
	"encoding/hex"
	"fmt"
	"maps"
	"path/filepath"
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
	Shared       *t.SharedState
	fCtx         *t.FileCtx
	bld          ScopeBuilder
	parentBld    ScopeBuilder
	nextSsa      *int
	moduleIdx    int
	CurrFunc     *t.NodeFuncDef
	traceStrings *traceStringPool

	CurrNestedScopeIdx int
	SeenNestedScopes   *int
	CurrDeferIdx       int
	NestedLoopCnt      *int

	LoopCondLbl SsaName
	LoopExitLbl SsaName

	IsTopLevel bool
}

// traceStringPool gives trace metadata one constant per distinct byte string.
// Names are derived from the complete contents, so parallel module generation
// cannot affect either references or final IR ordering.
type traceStringPool struct {
	mu     sync.Mutex
	values map[string]struct{}
}

func newTraceStringPool() *traceStringPool {
	return &traceStringPool{values: make(map[string]struct{})}
}

func traceStringName(value string) string {
	return "@.magma.trace.str." + hex.EncodeToString([]byte(value))
}

func (p *traceStringPool) intern(value string) SsaName {
	p.mu.Lock()
	p.values[value] = struct{}{}
	p.mu.Unlock()
	return ssaName(traceStringName(value))
}

func escapeCString(value string) string {
	var escaped strings.Builder
	for i := 0; i < len(value); i++ {
		b := value[i]
		if b == '"' || b == '\\' || b < 0x20 || b > 0x7e {
			fmt.Fprintf(&escaped, "\\%02X", b)
		} else {
			escaped.WriteByte(b)
		}
	}
	return escaped.String()
}

func (p *traceStringPool) writeTo(b *bytes.Buffer) {
	p.mu.Lock()
	values := make([]string, 0, len(p.values))
	for value := range p.values {
		values = append(values, value)
	}
	p.mu.Unlock()
	slices.Sort(values)
	for _, value := range values {
		fmt.Fprintf(b, "%s = private unnamed_addr constant [%d x i8] c\"%s\\00\"\n",
			traceStringName(value), len(value)+1, escapeCString(value))
	}
}

func flattenName(name t.NodeName) string {
	s := ""

	parsed := parseName(name)

	s += parsed.First
	if parsed.HasParts {
		for _, x := range parsed.Parts {
			s += "." + x
		}
	}
	return s
}

// traceDisplayName returns the source-level spelling of a function name.
// Generic specializations must retain their mangled names as LLVM symbols,
// but exposing the encoded type arguments in diagnostics makes traces hard to
// read. Each qualified name component is mangled independently, so remove the
// specialization suffix from every component.
func traceDisplayName(name t.NodeName) string {
	parts := strings.Split(flattenName(name), ".")
	for i, part := range parts {
		if generic := strings.Index(part, "__g__"); generic >= 0 {
			parts[i] = part[:generic]
		}
	}
	return strings.Join(parts, ".")
}

type parsedName struct {
	First    string
	Parts    []string
	HasParts bool
}

func parseName(name t.NodeName) parsedName {
	switch n := name.(type) {
	case *t.NodeNameSingle:
		return parsedName{
			First:    n.Name,
			HasParts: false,
		}
	case *t.NodeNameComposite:
		return parsedName{
			First:    n.Parts[0],
			HasParts: true,
			Parts:    n.Parts[1:],
		}
	}
	return parsedName{}
}

func flattenTypeKind(nodeKind t.NodeTypeKind) string {
	switch n := nodeKind.(type) {
	case *t.NodeTypeNamed:
		switch nn := n.NameNode.(type) {
		case *t.NodeNameSingle:
			return nn.Name
		case *t.NodeNameComposite:
			return strings.Join(nn.Parts, ".")
		}
	case *t.NodeTypePointer:
		return flattenTypeKind(n.Kind) + "*"
	case *t.NodeTypeSlice:
		return flattenTypeKind(n.ElemKind) + "[]"
	case *t.NodeTypeFunc:
		return flattenTypeKind(n.RetType.KindNode) + "()"
	case *t.NodeTypeAbsolute:
		return n.AbsoluteName
	}
	return "undef"
}

func flattenType(node *t.NodeType) string {
	if node == nil {
		return "nil"
	}

	return flattenTypeKind(node.KindNode)
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
			KindNode: n.Kind,
			Throws:   from.Throws,
			// Destructor: from.Destructor,
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

func irSsaLocal(ctx *IrCtx) SsaName {
	mdIdx := strconv.Itoa(ctx.moduleIdx)
	name := strconv.Itoa(*ctx.nextSsa)
	(*ctx.nextSsa)++
	return ssaName("%." + mdIdx + name)
}

func irSsaGlobal(ctx *IrCtx) SsaName {
	mdIdx := strconv.Itoa(ctx.moduleIdx)
	name := strconv.Itoa(*ctx.nextSsa)
	(*ctx.nextSsa)++
	// Keep the module and per-module counter separated. Concatenating them is
	// ambiguous: module 3/global 211 and module 32/global 11 both become 3211.
	return ssaName("@." + mdIdx + "." + name)
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

func irGlVarDef(ctx *IrCtx, vd *t.NodeExprVarDef) error {
	cpy := *ctx
	cpy.bld.Body = ctx.bld.Head

	//irWritef(&cpy, "@%s = internal global ", vd.AbsName)
	irWritef(&cpy, "@%s = private thread_local global ", vd.AbsName)
	//irWritef(&cpy, "@%s = private static thread_local global ", vd.AbsName)

	e := irType(&cpy, vd.Type)
	if e != nil {
		return e
	}

	irWrite(&cpy, " zeroinitializer\n")
	return nil
}

func irConstValue(ctx *IrCtx, expected *t.NodeType, expr t.NodeExpr) error {
	switch n := expr.(type) {
	case *t.NodeExprLit:
		switch n.LitType {
		case t.TokLitNum, t.TokLitBool:
			irWrite(ctx, n.Value)
			return nil
		case t.TokLitNone:
			irWrite(ctx, "null")
			return nil
		default:
			return fmt.Errorf("string literals are not supported in global constants")
		}
	case *t.NodeExprName:
		fn, ok := n.AssociatedNode.(*t.NodeFuncDef)
		if !ok {
			return fmt.Errorf("constant name '%s' is not a function symbol", flattenName(n.Name))
		}
		name := fn.AbsName
		if fn.NoAliasName != "" {
			name = fn.NoAliasName
		}
		irWritef(ctx, "@%s", name)
		return nil
	case *t.NodeExprAddrof:
		name, ok := n.Expr.(*t.NodeExprName)
		if !ok {
			return fmt.Errorf("constant addrof requires a global name")
		}
		varDef, ok := name.AssociatedNode.(*t.NodeExprVarDef)
		if !ok || !varDef.IsGlobal {
			return fmt.Errorf("constant addrof requires a global name")
		}
		irWritef(ctx, "@%s", varDef.AbsName)
		return nil
	case *t.NodeExprStructInit:
		fields := slices.Clone(n.Fields)
		slices.SortFunc(fields, func(a, b t.NodeStructFieldInit) int { return a.FieldIndex - b.FieldIndex })
		irWrite(ctx, "{ ")
		for i, field := range fields {
			if e := irType(ctx, field.FieldType); e != nil {
				return e
			}
			irWrite(ctx, " ")
			if e := irConstValue(ctx, field.FieldType, field.Expression); e != nil {
				return e
			}
			if i+1 < len(fields) {
				irWrite(ctx, ", ")
			}
		}
		irWrite(ctx, " }")
		return nil
	default:
		return fmt.Errorf("expression %T is not supported in a global constant", expr)
	}
}

func irConstDef(ctx *IrCtx, def *t.NodeConstDef) error {
	irWriteGlf(ctx, "@%s = private constant ", def.VarDef.AbsName)
	cpy := *ctx
	cpy.bld.Body = ctx.bld.Global
	cpy.bld.Head = ctx.bld.Global
	cpy.bld.Tail = ctx.bld.Global
	if e := irType(&cpy, def.VarDef.Type); e != nil {
		return e
	}
	irWriteGl(ctx, " ")
	if e := irConstValue(&cpy, def.VarDef.Type, def.Initializer); e != nil {
		return e
	}
	irWriteGl(ctx, "\n")
	return nil
}

func irVarDef(ctx *IrCtx, vd *t.NodeExprVarDef) (SsaName, error) {
	if vd.IrName == "" {
		vd.IrName = irSsaLocal(ctx).Repr
	}
	allocSsa := SsaName{Repr: vd.IrName}

	/* DEPRECATED
	if vd.Type.Destructor != nil {
		irWrite(ctx, "  ; has destructor\n")
	}*/

	irWriteHdf(ctx, "  %s = alloca ", allocSsa.Repr)

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
	irWritef(ctx, " zeroinitializer, ptr %s\n", allocSsa.Repr)

	if isSliceType(vd.Type) {
		sliceT := vd.Type.KindNode.(*t.NodeTypeSlice)
		elemType := makeTypeFromKind(sliceT.ElemKind)

		if sliceT.HasSize {
			arrSsa := irSsaLocal(ctx)

			// making dud ctx to redirect name IR to head
			cpy := *ctx
			cpy.bld = ScopeBuilder{
				Global: ctx.bld.Global,
				Head:   ctx.bld.Head,
				Tail:   ctx.bld.Head,
				Body:   ctx.bld.Head,
			}

			irWritef(&cpy, "  %s = alloca [ %d x ", arrSsa.Repr, sliceT.Size)
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

			totalSizeSsa := irSsaLocal(ctx)
			irWritef(ctx, "  %s = mul i64 ", totalSizeSsa.Repr)
			irPossibleLitSsa(ctx, sizeofSsa)
			irWritef(ctx, ", %d\n", sliceT.Size)

			irWritef(ctx, "  call void @llvm.memset.p0i8.i64(ptr %s, i8 0, i64 %s, i32 1, i1 0)\n", arrSsa.Repr, totalSizeSsa.Repr)

			fldPtrSsa := irSsaLocal(ctx)
			fldSizSsa := irSsaLocal(ctx)

			irWritef(ctx, "  %s = insertvalue %%type.slice zeroinitializer, ptr %s, 0\n", fldPtrSsa.Repr, arrSsa.Repr)
			irWritef(ctx, "  %s = insertvalue %%type.slice %s, i64 %d, 1\n", fldSizSsa.Repr, fldPtrSsa.Repr, sliceT.Size)
			irWritef(ctx, "  store %%type.slice %s, ptr %s\n", fldSizSsa.Repr, allocSsa.Repr)
		}
	}

	return allocSsa, nil
}

func irPossibleLitSsa(ctx *IrCtx, ssa SsaName) {
	if ssa.IsLiteral {
		irWrite(ctx, ssa.Repr)
	} else {
		irWritef(ctx, "%s", ssa.Repr)
	}
}

func irVarDefAssign(ctx *IrCtx, vda *t.NodeExprVarDefAssign) (SsaName, error) {
	assignSsa, e := irExpression(ctx, vda.VarDef.Type, vda.AssignExpr, false)
	if e != nil {
		return ssaName(""), e
	}

	allocSsa, e := irVarDef(ctx, vda.VarDef)
	if e != nil {
		return ssaName(""), e
	}

	// TODO: this assumes we correctly infer the expression type during type checking,
	// but we don't, we need to make sure the inference rules mirror number promotion
	/*
		if isNumberType(vda.GetInferredType()) {
			if !isSameNumType(vda.GetInferredType(), vda.AssignExpr.GetInferredType()) {
				if !assignSsa.IsLiteral {
					return SsaName{}, comp_err.CompilationErrorToken(
						ctx.fCtx,
						&vda.Tk,
						"implicit number cast is forbidden on assignment",
						fmt.Sprintf("left side type is: %s, right side type is: %s", flattenType(vda.GetInferredType()), flattenType(vda.AssignExpr.GetInferredType())),
					)
				}
			}
		}*/

	irWrite(ctx, "  store ")
	e = irType(ctx, vda.VarDef.Type)
	if e != nil {
		return ssaName(""), e
	}

	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, assignSsa)

	irWritef(ctx, ", ptr %s\n", allocSsa.Repr)
	return allocSsa, nil
}

func irExprStructInit(ctx *IrCtx, init *t.NodeExprStructInit) (SsaName, error) {
	current := SsaName{Repr: "zeroinitializer", IsLiteral: true}
	for _, field := range init.Fields {
		value, e := irExpression(ctx, field.FieldType, field.Expression, false)
		if e != nil {
			return SsaName{}, e
		}
		next := irSsaLocal(ctx)
		irWritef(ctx, "  %s = insertvalue ", next.Repr)
		if e := irType(ctx, init.Type); e != nil {
			return SsaName{}, e
		}
		irWrite(ctx, " ")
		irPossibleLitSsa(ctx, current)
		irWrite(ctx, ", ")
		if e := irType(ctx, field.FieldType); e != nil {
			return SsaName{}, e
		}
		irWrite(ctx, " ")
		irPossibleLitSsa(ctx, value)
		irWritef(ctx, ", %d\n", field.FieldIndex)
		current = next
	}
	return current, nil
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

func irExprCallFuncPtr(ctx *IrCtx, fnCall *t.NodeExprCall, topLevel bool) (SsaName, error) {
	irWrite(ctx, "  ; call fnptr\n")

	fnType := fnCall.FuncPtrType.KindNode.(*t.NodeTypeFunc)

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

	e = irFuncPtrType(ctx, fnCall.FuncPtrType.KindNode.(*t.NodeTypeFunc))
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
	irWritef(ctx, "  ; call %s\n", fnCall.AssociatedFnDef.AbsName)

	argsSsa := make([]SsaName, len(fnCall.Args))
	for i, expr := range fnCall.Args {
		// TODO: will crash if wrong number of args
		argT := fnCall.AssociatedFnDef.Class.ArgsNode.Args[i].TypeNode

		exprSsa, e := irExpression(ctx, argT, expr, false)
		if e != nil {
			return ssaName(""), e
		}

		switch expr.(type) {
		case *t.NodeExprUnary, *t.NodeExprBinary:
		default:
			if isNumberType(argT) && !exprSsa.IsLiteral {
				if !isSameNumType(expr.GetInferredType(), argT) {
					outSsa, e := irPromoteSingleToNum(ctx, argT, exprSsa, expr.GetInferredType())
					if e != nil {
						return SsaName{}, e
					}
					exprSsa = outSsa
				}
			}
		}

		argsSsa[i] = exprSsa
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
	irWritef(ctx, "  ; call member %s\n", fnCall.AssociatedFnDef.AbsName)

	argsSsa := make([]SsaName, len(fnCall.Args))
	for i, expr := range fnCall.Args {
		argT := fnCall.AssociatedFnDef.Class.ArgsNode.Args[i+1].TypeNode

		exprSsa, e := irExpression(ctx, argT, expr, false)
		if e != nil {
			return ssaName(""), e
		}

		switch expr.(type) {
		case *t.NodeExprUnary, *t.NodeExprBinary:
		default:
			if isNumberType(argT) && !exprSsa.IsLiteral {
				if !isSameNumType(expr.GetInferredType(), argT) {
					outSsa, e := irPromoteSingleToNum(ctx, argT, exprSsa, expr.GetInferredType())
					if e != nil {
						return SsaName{}, e
					}
					exprSsa = outSsa
				}
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
			isSsaOwner = n.IsSsa
		}

		ownerSsa, e = irExprNameLvalue(ctx, fnCall.MemberOwnerName)
		if e != nil {
			return SsaName{}, e
		}

		if !isSsaOwner && isPointerType(fnCall.MemberOwnerType) {
			// Non-SSA pointer locals and arguments live in an alloca. A member
			// call needs the stored pointer value as `this`, not the address of
			// that pointer slot (which would incorrectly pass T** as T*).
			loadedOwner := irSsaLocal(ctx)
			irWritef(ctx, "  %s = load ", loadedOwner.Repr)
			e = irType(ctx, fnCall.MemberOwnerType)
			if e != nil {
				return SsaName{}, e
			}
			irWritef(ctx, ", ptr %s\n", ownerSsa.Repr)
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

/* DEPRECATED
func irExprDestructor(ctx *IrCtx, destructor *t.NodeExprDestructor) (SsaName, error) {
	continueLbl := irSsaName(ctx)
	preventLbl := irSsaName(ctx)


	if destructor.VarDef.IsReturned {
		condSsa := irSsaLocal(ctx)
		irWritef(ctx, "  %s = load i1, ptr %%.destr%s\n", condSsa.Repr, destructor.VarDef.RetFlagId)
		irWritef(ctx, "  br i1 %s, label %%%s, label %%%s\n", condSsa.Repr, preventLbl.Repr, continueLbl.Repr)
		irWritef(ctx, "%s:\n", continueLbl.Repr)
	}

	irWrite(ctx, "  call void @")
	irWrite(ctx, destructor.Destructor.AbsName)

	// TODO: use AbsName instead
	varPtr := destructor.VarDef.IrName
	if varPtr == "" {
		varPtr = "%" + destructor.VarDef.Name.(*t.NodeNameSingle).Name
	}
	irWritef(ctx, "(ptr %s)\n", varPtr)

	if destructor.VarDef.IsReturned {
		irWritef(ctx, "  br label %%%s\n", preventLbl.Repr)
		irWritef(ctx, "%s:\n", preventLbl.Repr)
	}

	return SsaName{}, nil
}
*/

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
	case *t.NodeTypePointer, *t.NodeTypeRfc, *t.NodeTypeFunc:
		ptrBytes := 8 // TODO: make machine specific
		return SsaName{Repr: strconv.Itoa(ptrBytes), IsLiteral: true}, nil
	case *t.NodeTypeNamed:
		if nn, ok := n.NameNode.(*t.NodeNameSingle); ok {
			if nn.Name == "ptr" {
				ptrBytes := 8 // TODO: make machine specific
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
		forceMaterialize = name.IsSsa && len(name.MemberAccesses) == 0
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

	irWritef(ctx, ", ptr %s, i32 0, i32 %d\n", basePtr.Repr, fieldIndex)
	return fieldPtr, nil
}

func irExprName(ctx *IrCtx, nameExpr *t.NodeExprName) (SsaName, error) {
	ptrSsa := irNameSsa(ctx, nameExpr.Name, false)
	ssa := irSsaLocal(ctx)

	var typeNd *t.NodeType = nil
	isMemberAccess := false

	isFuncName := false
	var fnDef *t.NodeFuncDef = nil

	switch n := nameExpr.Name.(type) {
	case *t.NodeNameComposite:

		// TODO: we should not be making these kind of assumptions at IR level
		isMemberAccess = true
		ptrSsa = irNameSsa(ctx, &t.NodeNameSingle{
			Name: n.Parts[0],
		}, false)
	}

	switch n := nameExpr.AssociatedNode.(type) {
	case *t.NodeExprVarDef:
		typeNd = n.Type

		if n.IsGlobal {
			ptrSsa = SsaName{Repr: "@" + n.AbsName}
		} else if n.IrName != "" {
			ptrSsa = SsaName{Repr: n.IrName}
		}
	case *t.NodeExprVarDefAssign:
		typeNd = n.VarDef.Type

		if n.VarDef.IsGlobal {
			ptrSsa = SsaName{Repr: "@" + n.VarDef.AbsName}
		} else if n.VarDef.IrName != "" {
			ptrSsa = SsaName{Repr: n.VarDef.IrName}
		}
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
		irWritef(ctx, "  %s = bitcast ptr @", ssa.Repr)

		if fnDef.NoAliasName != "" {
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

		e := irFuncPtrType(ctx, typeNd.KindNode.(*t.NodeTypeFunc))
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
		if len(nameExpr.MemberAccesses) == 0 {
			// TODO: compilation error
			//fmt.Printf("member access but no member access history\n")
			//fmt.Printf("name: %s\n", flattenName(nameExpr.Name))
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

func irExprMemberAccess(ctx *IrCtx, member *t.NodeExprMemberAccess) (SsaName, error) {
	if member.Access == nil {
		return SsaName{}, fmt.Errorf("member access '%s' has no resolved access info", member.Member)
	}

	targetSsa, e := irExpression(ctx, member.Target.GetInferredType(), member.Target, false)
	if e != nil {
		return SsaName{}, e
	}

	return irMemberAccess(
		ctx,
		member.Target.GetInferredType(),
		targetSsa,
		member.Access.FieldNb,
		member.Access.Type,
		member.Access.PtrDeref,
	)
}

func irExprMemberAccessLvalue(ctx *IrCtx, member *t.NodeExprMemberAccess) (SsaName, error) {
	if member.Access == nil {
		return SsaName{}, fmt.Errorf("member access '%s' has no resolved access info", member.Member)
	}

	var basePtr SsaName
	var e error
	if name, ok := member.Target.(*t.NodeExprName); ok && name.IsSsa {
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

	return irMemberAddress(ctx, basePtr, member.Target.GetInferredType(), member.Access.FieldNb)
}

func irExprNameLvalue(ctx *IrCtx, nameExpr *t.NodeExprName) (SsaName, error) {
	basePtr := irNameSsa(ctx, nameExpr.Name, false)

	switch n := nameExpr.Name.(type) {
	case *t.NodeNameComposite:
		basePtr = irNameSsa(ctx, &t.NodeNameSingle{
			Name: n.Parts[0],
		}, false)
	}

	// TODO: pretty sure this might break field access on globals
	switch n := nameExpr.AssociatedNode.(type) {
	case *t.NodeExprVarDef:
		if n.IsGlobal {
			basePtr = SsaName{Repr: "@" + n.AbsName}
		} else if n.IrName != "" {
			basePtr = SsaName{Repr: n.IrName}
		}
	case *t.NodeExprVarDefAssign:
		if n.VarDef.IsGlobal {
			basePtr = SsaName{Repr: "@" + n.VarDef.AbsName}
		} else if n.VarDef.IrName != "" {
			basePtr = SsaName{Repr: n.VarDef.IrName}
		}
	}

	if nameExpr.IsSsa && len(nameExpr.MemberAccesses) > 0 {
		var rootType *t.NodeType
		switch definition := nameExpr.AssociatedNode.(type) {
		case *t.NodeExprVarDef:
			rootType = definition.Type
		case *t.NodeExprVarDefAssign:
			rootType = definition.VarDef.Type
		}
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
	var curType *t.NodeType = nil

	switch n := nameExpr.AssociatedNode.(type) {
	case *t.NodeExprVarDef:
		curType = n.Type
	case *t.NodeExprVarDefAssign:
		curType = n.VarDef.Type
	}

	// A non-SSA pointer local is an alloca containing the pointer value. Load
	// that value before taking the address of one of the pointee's fields.
	if isPointerType(curType) && !nameExpr.IsSsa {
		loaded := irSsaLocal(ctx)
		irWritef(ctx, "  %s = load ptr, ptr %s\n", loaded.Repr, basePtr.Repr)
		basePtr = loaded
		curPtr = loaded
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
	subsExpr, e := irExpression(ctx, makeNamedType("i64"), subs.Expr, false)
	if e != nil {
		return SsaName{}, e
	}

	switch subs.BoxType.KindNode.(type) {
	case *t.NodeTypeSlice:
		var loadedTarget SsaName
		if subs.IsTargetSsa {
			loadedTarget, e = irExpression(ctx, makeNamedType("slice"), subs.Target, false)
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
	subsExpr, e := irExpression(ctx, makeNamedType("i64"), subs.Expr, false)
	if e != nil {
		return SsaName{}, e
	}

	var targetPtrSsa SsaName

	switch subs.BoxType.KindNode.(type) {
	case *t.NodeTypeSlice:
		var loadedTarget SsaName
		if subs.IsTargetSsa {
			loadedTarget, e = irExpression(ctx, makeNamedType("slice"), subs.Target, false)
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
						fmt.Sprintf("left side type is: %s, right side type is: %s", flattenType(lhs.GetInferredType()), flattenType(rhs.GetInferredType())),
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

func irExtendFlt(ctx *IrCtx, valSsa SsaName, prevType *t.NodeType, newType *t.NodeType) (SsaName, error) {
	outSsa := irSsaLocal(ctx)
	irWritef(ctx, "  %s = fpext ", outSsa.Repr)

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
	outSsa := irSsaLocal(ctx)

	if signed {
		irWritef(ctx, "  %s = sext ", outSsa.Repr)
	} else {
		irWritef(ctx, "  %s = zext ", outSsa.Repr)
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
	outSsa := irSsaLocal(ctx)

	irWritef(ctx, "  %s = trunc ", outSsa.Repr)

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
	outSsa := irSsaLocal(ctx)

	irWritef(ctx, "  %s = fptrunc ", outSsa.Repr)

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
	ssa := irSsaLocal(ctx)
	irWritef(ctx, "  %s = ptrtoint ptr %s to i64", ssa.Repr, valSsa.Repr)
	return ssa
}

func irIntToPtr(ctx *IrCtx, valSsa SsaName) SsaName {
	ssa := irSsaLocal(ctx)
	irWritef(ctx, "  %s = inttoptr i64 %s to ptr", ssa.Repr, valSsa.Repr)
	return ssa
}

func irIntToFloat(ctx *IrCtx, valSsa SsaName, numType *t.NodeType, toType *t.NodeType) (SsaName, error) {
	numDesc := getNumDesc(numType)

	// here target is guaranteed to be integer type
	outSsa := irSsaLocal(ctx)
	irWritef(ctx, "  %s = ", outSsa.Repr)

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
	outSsa := irSsaLocal(ctx)
	irWritef(ctx, "  %s = ", outSsa.Repr)

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
	operandSsa, e := irExpression(ctx, operandType, unaryExpr.Operand, false)
	if e != nil {
		return SsaName{}, e
	}

	switch unaryExpr.Operator {
	case t.KwAsterisk:
		resSsa := irSsaLocal(ctx)
		irWritef(ctx, "  %s = load ", resSsa.Repr)
		e = irType(ctx, unaryExpr.InfType)
		if e != nil {
			return SsaName{}, e
		}
		irWritef(ctx, ", ptr %s\n", operandSsa.Repr)
		return resSsa, nil
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

		resSsa := irSsaLocal(ctx)
		irWritef(ctx, "  %s = xor ", resSsa.Repr)
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

	leftSsa, e := irExpression(ctx, leftType, binaryExpr.Left, false)
	if e != nil {
		return SsaName{}, e
	}

	rightSsa, e := irExpression(ctx, rightType, binaryExpr.Right, false)
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

	resSsa := irSsaLocal(ctx)
	irWritef(ctx, "  %s = ", resSsa.Repr)

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

	leftSsa, e := irExpression(ctx, leftType, binaryExpr.Left, false)
	if e != nil {
		return SsaName{}, e
	}

	rightSsa, e := irExpression(ctx, rightType, binaryExpr.Right, false)
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

	resSsa := irSsaLocal(ctx)
	irWritef(ctx, "  %s = ", resSsa.Repr)

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

	leftSsa, e := irExpression(ctx, boolType, binaryExpr.Left, false)
	if e != nil {
		return SsaName{}, e
	}

	// Allocate the result in the function entry (head) so we can avoid `phi`.
	resultPtr := irSsaLocal(ctx)
	cpy := *ctx
	cpy.bld.Body = ctx.bld.Head
	irWritef(&cpy, " %s = alloca i1\n", resultPtr.Repr)

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

	rightSsa, e := irExpression(ctx, boolType, binaryExpr.Right, false)
	if e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, "  store i1 ")
	irPossibleLitSsa(ctx, rightSsa)
	irWritef(ctx, ", ptr %s\n", resultPtr.Repr)
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
	irWritef(ctx, ", ptr %s\n", resultPtr.Repr)
	irWritef(ctx, "  br label %%%s\n", endLabel.Repr)

	irWritef(ctx, "%s:\n", endLabel.Repr)

	resSsa := irSsaLocal(ctx)
	irWritef(ctx, "  %s = load i1, ptr %s\n", resSsa.Repr, resultPtr.Repr)
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

	lhsSsa, e := irExpression(ctx, leftType, binaryExpr.Left, false)
	if e != nil {
		return SsaName{}, e
	}

	rhsSsa, e := irExpression(ctx, rightType, binaryExpr.Right, false)
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

	resSsa := irSsaLocal(ctx)
	irWritef(ctx, "  %s = ", resSsa.Repr)

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
	lhsSsa, e := irExpression(ctx, binaryExpr.Left.GetInferredType(), binaryExpr.Left, false)
	if e != nil {
		return SsaName{}, e
	}

	rhsSsa, e := irExpression(ctx, binaryExpr.Right.GetInferredType(), binaryExpr.Right, false)
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

	resSsa := irSsaLocal(ctx)
	irWritef(ctx, "  %s = ", resSsa.Repr)

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
	lhsSsa, e := irExpression(ctx, binaryExpr.Left.GetInferredType(), binaryExpr.Left, false)
	if e != nil {
		return SsaName{}, e
	}

	// Generate IR for the right-hand side expression
	rhsSsa, e := irExpression(ctx, binaryExpr.Right.GetInferredType(), binaryExpr.Right, false)
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
	resSsa := irSsaLocal(ctx)

	// Start writing the IR instruction
	irWritef(ctx, "  %s = ", resSsa.Repr)

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

func irExprBinDivision(ctx *IrCtx, expectedType *t.NodeType, binaryExpr *t.NodeExprBinary) (SsaName, error) {
	// Generate IR for the left-hand side expression
	lhsSsa, e := irExpression(ctx, binaryExpr.Left.GetInferredType(), binaryExpr.Left, false)
	if e != nil {
		return SsaName{}, e
	}

	// Generate IR for the right-hand side expression
	rhsSsa, e := irExpression(ctx, binaryExpr.Right.GetInferredType(), binaryExpr.Right, false)
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
	resSsa := irSsaLocal(ctx)

	// Start writing the IR instruction
	irWritef(ctx, "  %s = ", resSsa.Repr)

	// Write the appropriate multiplication opcode
	if isFloatType(newType) {
		irWrite(ctx, "fdiv ")
	} else {
		numDes := getNumDesc(newType)
		if numDes.IsSigned {
			irWrite(ctx, "sdiv ")
		} else {
			irWrite(ctx, "udiv ")
		}
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

func irExprBinModulo(ctx *IrCtx, expectedType *t.NodeType, binaryExpr *t.NodeExprBinary) (SsaName, error) {
	// Generate IR for the left-hand side expression
	lhsSsa, e := irExpression(ctx, binaryExpr.Left.GetInferredType(), binaryExpr.Left, false)
	if e != nil {
		return SsaName{}, e
	}

	// Generate IR for the right-hand side expression
	rhsSsa, e := irExpression(ctx, binaryExpr.Right.GetInferredType(), binaryExpr.Right, false)
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
	resSsa := irSsaLocal(ctx)

	// Start writing the IR instruction
	irWritef(ctx, "  %s = ", resSsa.Repr)

	// Write the appropriate multiplication opcode
	if isFloatType(newType) {
		irWrite(ctx, "frem ")
	} else {
		numDes := getNumDesc(newType)
		if numDes.IsSigned {
			irWrite(ctx, "srem ")
		} else {
			irWrite(ctx, "urem ")
		}
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
	lhsSsa, e := irExpression(ctx, binaryExpr.Left.GetInferredType(), binaryExpr.Left, false)
	if e != nil {
		return SsaName{}, e
	}

	rhsSsa, e := irExpression(ctx, binaryExpr.Right.GetInferredType(), binaryExpr.Right, false)
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

	resSsa := irSsaLocal(ctx)
	cmpPref := "i"
	if isFloat {
		cmpPref = "f"
	}

	irWritef(ctx, "  %s = %scmp ", resSsa.Repr, cmpPref)

	switch binaryExpr.Operator {
	case t.KwCmpEq:
		if isFloat {
			irWrite(ctx, "oeq ")
		} else {
			irWrite(ctx, "eq ")
		}
	case t.KwCmpNeq:
		if isFloat {
			irWrite(ctx, "une ")
		} else {
			irWrite(ctx, "ne ")
		}
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
	case t.KwSlash:
		return irExprBinDivision(ctx, expectedType, binaryExpr)
	case t.KwPercent:
		return irExprBinModulo(ctx, expectedType, binaryExpr)
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

func irTryCall(ctx *IrCtx, callRetSsa SsaName, fnCall *t.NodeExprCall, pos t.FilePos) (SsaName, error) {
	errSsa := irSsaLocal(ctx)

	irWritef(ctx, "  %s = extractvalue ", errSsa.Repr)

	e := irThrowingType(ctx, fnCall.InfType)
	if e != nil {
		return SsaName{}, e
	}

	irWritef(ctx, " %s, 0\n", callRetSsa.Repr)

	e = irThrowSsa(ctx, errSsa, ctx.CurrFunc, pos)
	if e != nil {
		return SsaName{}, e
	}

	if !isVoidType(fnCall.InfType) {
		valSsa := irSsaLocal(ctx)

		irWritef(ctx, "  %s = extractvalue ", valSsa.Repr)

		e = irThrowingType(ctx, fnCall.InfType)
		if e != nil {
			return SsaName{}, e
		}

		irWritef(ctx, " %s, 1\n", callRetSsa.Repr)
		return valSsa, nil
	}

	return SsaName{Repr: "<void ret>"}, nil
}

func irExpression(ctx *IrCtx, expectedType *t.NodeType, expr t.NodeExpr, topLevel bool) (SsaName, error) {
	switch ne := expr.(type) {
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
	// DEPRECATED
	/*case *t.NodeExprDestructor:
	return irExprDestructor(ctx, ne)*/
	case *t.NodeExprUnary:
		return irExprUnary(ctx, expectedType, ne)
	case *t.NodeExprTry:
		callSsa, e := irExprFuncCall(ctx, ne.Call.(*t.NodeExprCall), true, false)
		if e != nil {
			return SsaName{}, e
		}
		return irTryCall(ctx, callSsa, ne.Call.(*t.NodeExprCall), ne.Pos)
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
	case *t.NodeExprName:
		return irExprName(ctx, ne)
	case *t.NodeExprMemberAccess:
		return irExprMemberAccess(ctx, ne)
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
	case *t.NodeExprMemberAccess:
		return irExprMemberAccessLvalue(ctx, ne)
	case *t.NodeExprUnary:
		if ne.Operator == t.KwAsterisk {
			return irExpression(ctx, ne.Operand.GetInferredType(), ne.Operand, false)
		}
	}
	return ssaName(""), fmt.Errorf("expr not lvalue")
}

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
	irWritef(ctx, "  %s = extractvalue %%type.error %s, 2\n", fieldSsa.Repr, errSsa.Repr)

	// if errcode != 0
	irWritef(ctx, "  %s = icmp ne i32 %s, 0\n", compSsa.Repr, fieldSsa.Repr)
	irWritef(ctx, "  br i1 %s, label %%%s, label %%%s, !prof !9000\n", compSsa.Repr, neqLabel.Repr, eqLabel.Repr)

	// throw = err; return 0
	irWritef(ctx, "%s:\n", neqLabel.Repr)

	// Add source metadata only on the failing edge. The runtime uses bounded
	// static storage, so this cannot allocate or invalidate an older trace.
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

	//if ctx.CurrFunc.HasDefer {

	irWrite(ctx, "  store i1 1, ptr %.defer.ret\n")

	irWrite(ctx, "  store ")
	e := irThrowingType(ctx, fnDef.ReturnType)
	if e != nil {
		return e
	}
	irWrite(ctx, " ")
	irWritef(ctx, "%s", retValSsa.Repr)
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
	exprSsa, e := irExpression(ctx, makeNamedType("error"), stmtThrow.Expression, false)
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

	callSsa, e := irExprFuncCall(ctx, expr.Call, true, false)
	if e != nil {
		return SsaName{}, e
	}

	// Extract error
	errVal := irSsaLocal(ctx)
	irWritef(ctx, "  %s = extractvalue ", errVal.Repr)
	e = irThrowingType(ctx, expr.Call.InfType)
	if e != nil {
		return SsaName{}, e
	}
	irWritef(ctx, " %s, 0\n", callSsa.Repr)

	irWrite(ctx, "  store ")
	e = irType(ctx, expr.ErrDef.Type)
	if e != nil {
		return SsaName{}, e
	}
	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, errVal)
	irWritef(ctx, ", ptr %s\n", errPtr.Repr)

	// Extract value (if any)
	if !isVoidType(expr.Call.InfType) {
		valVal := irSsaLocal(ctx)
		irWritef(ctx, "  %s = extractvalue ", valVal.Repr)
		e = irThrowingType(ctx, expr.Call.InfType)
		if e != nil {
			return SsaName{}, e
		}
		irWritef(ctx, " %s, 1\n", callSsa.Repr)

		irWrite(ctx, "  store ")
		e = irType(ctx, expr.ValueDef.Type)
		if e != nil {
			return SsaName{}, e
		}
		irWrite(ctx, " ")
		irPossibleLitSsa(ctx, valVal)
		irWritef(ctx, ", ptr %s\n", valPtr.Repr)
	}

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
	case *t.NodeStmtContinue:
		e = irStmtContinue(ctx, s)
	case *t.NodeStmtBreak:
		e = irStmtBreak(ctx, s)
	}
	return e
}

func irStmtIf(ctx *IrCtx, ifStmt *t.NodeStmtIf, fnDef *t.NodeFuncDef) error {
	condSsa, e := irExpression(ctx, makeNamedType("bool"), ifStmt.CondExpr, false)
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

	condSsa, e := irExpression(ctx, makeNamedType("bool"), ifStmt.CondExpr, false)
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
			/* DEPRECATED
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
				}*/
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
			_, e := irExpression(&cpy, nil, def.Expression, false)
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

	_, isMemberFunc := fnDef.Class.NameNode.(*t.NodeNameComposite)
	for i, arg := range fnDef.Class.ArgsNode.Args {
		if isMemberFunc && i == 0 {
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
	irWrite(&cpy, "  %.defer.brk = alloca i1\n")
	irWrite(&cpy, "  %.defer.cont = alloca i1\n")

	irWrite(&cpy, "  store i1 0, ptr %.defer.ret\n")
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
			/* DEPRECATED
			case *t.NodeStmtExpr:
				switch n2 := n.Expression.(type) {
				case *t.NodeExprVarDef:
					if n2.Type.Destructor != nil {
						cpy.CurrDeferIdx++
						n2.RetFlagId = irSsaName(ctx).Repr
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
						n2.VarDef.RetFlagId = irSsaName(ctx).Repr
						fnDef.Deferred = append(fnDef.Deferred, &t.NodeStmtDefer{
							Expression: &t.NodeExprDestructor{
								VarDef:     n2.VarDef,
								Destructor: n2.VarDef.Type.Destructor,
							},
						})
					}
				}*/
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
			_, e := irExpression(&cpy, nil, def.Expression, false)
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
	irWrite(ctx, "define i32 @main(i32 %argc, ptr %argv) {\n") // alwaysinline
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
		irWrite(ctx, "  %ecd = extractvalue %type.error %e, 2\n")
		irWrite(ctx, "  %isnz = icmp ne i32 %ecd, 0\n")
		irWrite(ctx, "  br i1 %isnz, label %enz, label %ez, !prof !9000\n")
		irWrite(ctx, "enz:\n")
		irWrite(ctx, "  call void @magma.error.print(%type.error %e)\n")
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

func irFunDefAliased(ctx *IrCtx, fnDefNode *t.NodeFuncDef) error {
	irWrite(ctx, "declare ")
	e := irThrowingType(ctx, fnDefNode.ReturnType)
	if e != nil {
		return e
	}

	irWrite(ctx, " @")
	irWrite(ctx, fnDefNode.NoAliasName)

	irWrite(ctx, "(")
	bound := len(fnDefNode.Class.ArgsNode.Args)
	for i, field := range fnDefNode.Class.ArgsNode.Args {
		e = irType(ctx, field.TypeNode)
		if e != nil {
			return e
		}

		if i < bound-1 {
			irWrite(ctx, ", ")
		}
	}
	irWrite(ctx, ")\n")
	return nil
}

func irFuncDef(ctx *IrCtx, fnDefNode *t.NodeFuncDef) error {
	if fnDefNode.NoAliasName != "" {
		// func declared elsewhere, just emit declaration
		return irFunDefAliased(ctx, fnDefNode)
	}

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
	irWrite(ctx, fnDefNode.AbsName)

	/*
		e = irName(ctx, fnDefNode.Class.NameNode, true)
		if e != nil {
			return e
		}*/

	e = irArgsList(ctx, &fnDefNode.Class.ArgsNode, isMemberFunc)
	if e != nil {
		return e
	}

	ctx.CurrFunc = fnDefNode
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
	ctx.CurrFunc = nil
	return nil
}

func assignExprIrNames(ctx *IrCtx, expr t.NodeExpr) {
	switch n := expr.(type) {
	case *t.NodeExprVarDef:
		if n.IrName == "" {
			n.IrName = irSsaLocal(ctx).Repr
		}
	case *t.NodeExprVarDefAssign:
		assignExprIrNames(ctx, n.VarDef)
	case *t.NodeExprDestructureAssign:
		assignExprIrNames(ctx, &n.ValueDef)
		assignExprIrNames(ctx, &n.ErrDef)
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

func irNameSingle(ctx *IrCtx, nameNode *t.NodeNameSingle, withPackage bool) error {
	if withPackage {
		irWrite(ctx, ctx.fCtx.PackageName)
		irWrite(ctx, ".")
	}
	irWrite(ctx, nameNode.Name)
	return nil
}

func irNameSingleSsa(ctx *IrCtx, nameNode *t.NodeNameSingle, withPackage bool) SsaName {
	ssa := "%"

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
	ssa := "%"

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
	traceStrings *traceStringPool,
	i int,
) error {
	nextSsa := 0
	seenScopes := 0
	nestedLoop := 0

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
		traceStrings:     traceStrings,
		SeenNestedScopes: &seenScopes,
		NestedLoopCnt:    &nestedLoop,
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

	traceSlots := shared.ErrorTraceSlots
	if traceSlots == 0 {
		traceSlots = 1024
	}
	utilsFragment, err := llvmfragments.RenderUtils(traceSlots)
	if err != nil {
		return nil, err
	}
	llvmFragments := [][]byte{
		header,
		{},
		utilsFragment,
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
	traceStrings := newTraceStringPool()

	i := fragLen
	for _, v := range filesMap {

		localI := i
		go func(idx int) {
			defer wg.Done()

			// module local builder
			moduleBld := &bytes.Buffer{}
			glBld := &bytes.Buffer{}
			e := irWriteModule(shared, v, moduleBld, glBld, structDefBld, &structDefBldM, traceStrings, idx)
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
	traceStringBld := &bytes.Buffer{}
	traceStrings.writeTo(traceStringBld)
	results[1].S = append(results[1].S, traceStringBld.Bytes()...)

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
