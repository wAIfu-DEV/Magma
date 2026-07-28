package llvmir

import (
	magmatypes "Magma/src/magma_types"
	t "Magma/src/types"
	"bytes"
	"fmt"
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
	constStrings map[*t.NodeExprLit]SsaName
	localSlots   map[*t.NodeExprVarDef]SsaName

	CurrNestedScopeIdx int
	SeenNestedScopes   *int
	CurrDeferIdx       int
	NestedLoopCnt      *int

	LoopCondLbl SsaName
	LoopExitLbl SsaName

	IsTopLevel bool
}

// traceStringPool gives trace metadata one constant per distinct byte string.
// IDs are assigned from a sorted prepass, so names stay compact while parallel
// module generation cannot affect either references or final IR ordering.
type traceStringPool struct {
	mu     sync.Mutex
	values map[string]struct{}
	names  map[string]string
}

func newTraceStringPool(candidates []string) *traceStringPool {
	unique := make(map[string]struct{}, len(candidates))
	for _, value := range candidates {
		unique[value] = struct{}{}
	}
	ordered := make([]string, 0, len(unique))
	for value := range unique {
		ordered = append(ordered, value)
	}
	slices.Sort(ordered)
	names := make(map[string]string, len(ordered))
	for id, value := range ordered {
		names[value] = "@.mt" + strconv.Itoa(id)
	}
	return &traceStringPool{values: make(map[string]struct{}), names: names}
}

func (p *traceStringPool) intern(value string) SsaName {
	p.mu.Lock()
	name, ok := p.names[value]
	if !ok {
		p.mu.Unlock()
		panic(fmt.Sprintf("trace string %q was not collected before LLVM emission", value))
	}
	p.values[value] = struct{}{}
	p.mu.Unlock()
	return ssaName(name)
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
			p.names[value], len(value)+1, escapeCString(value))
	}
}

func collectTraceStrings(files map[string]*t.FileCtx) []string {
	values := []string{"<global>"}
	for _, file := range files {
		values = append(values, filepath.Base(file.FilePath))
		if file.GlNode == nil {
			continue
		}
		for _, declaration := range file.GlNode.Declarations {
			fn, ok := declaration.(*t.NodeFuncDef)
			if !ok || fn.NoAliasName != "" {
				continue
			}
			name := fn.DisplayName
			if name == "" {
				name = traceDisplayName(fn.Class.NameNode)
			}
			values = append(values, name)
		}
	}
	return values
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
	return t.SourceName(flattenName(name))
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
