package checker

import (
	t "Magma/src/types"
	"strconv"
	"strings"
)

func sameType(a *t.NodeType, b *t.NodeType) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Throws != b.Throws {
		return false
	}
	return sameTypeKind(a.KindNode, b.KindNode)
}

func compatibleInitializer(c *ctx, expected *t.NodeType, expr t.NodeExpr) bool {
	actual := expr.GetInferredType()
	if literal, ok := expr.(*t.NodeExprLit); ok && literal.LitType == t.TokLitNum && isNumberType(expected) {
		// Numeric literals are contextually typed. Recording that decision in
		// the checked AST keeps both LLVM lowerings from first materializing a
		// potentially narrower default integer and converting it afterward.
		literal.InfType = expected
		actual = expected
	}
	// `none` is the only raw-pointer value that may acquire a typed pointer
	// type implicitly. Other ptr -> T* conversions must go through cast.reinterpret.
	if lit, ok := expr.(*t.NodeExprLit); ok && lit.LitType == t.TokLitNone && isPointerType(expected) {
		return true
	}
	if c != nil && c.UnsafeDepth > 0 && isPointerLike(expected) && isPointerLike(actual) {
		return true
	}
	if compatibleTypes(expected, actual) {
		markContextAdapter(expected, actual, expr)
		return true
	}
	if lit, ok := expr.(*t.NodeExprLit); ok && lit.LitType == t.TokLitNum && isNumberType(expected) {
		return true
	}
	return false
}

func isPointerLike(node *t.NodeType) bool {
	return isPointerType(node) || isRawPointerType(node) || isFunctionType(node)
}

func markContextAdapter(expected, actual *t.NodeType, expr t.NodeExpr) {
	expectedFn, expectedOK := expected.KindNode.(*t.NodeTypeFunc)
	actualFn, actualOK := actual.KindNode.(*t.NodeTypeFunc)
	if !expectedOK || !actualOK || expectedFn.ContextABI != t.ContextABIContextful || actualFn.ContextABI != t.ContextABIContextless {
		return
	}
	if moved, ok := expr.(*t.NodeExprMove); ok {
		expr = moved.Expr
	}
	if name, ok := expr.(*t.NodeExprName); ok {
		name.ContextAdapter = true
		if function, ok := name.AssociatedNode.(*t.NodeFuncDef); ok {
			function.NeedsContextAdapter = true
		}
	}
}

func compatibleNativeCallback(expected, actual *t.NodeType, expr t.NodeExpr) bool {
	expectedFn, expectedOK := expected.KindNode.(*t.NodeTypeFunc)
	actualFn, actualOK := actual.KindNode.(*t.NodeTypeFunc)
	if !expectedOK || !actualOK || expectedFn.ContextABI != t.ContextABIContextless || actualFn.ContextABI != t.ContextABIContextful || expected.Throws != actual.Throws || len(expectedFn.Args) != len(actualFn.Args) {
		return false
	}
	for i := range expectedFn.Args {
		if !compatibleTypes(expectedFn.Args[i], actualFn.Args[i]) {
			return false
		}
	}
	if !compatibleTypes(expectedFn.RetType, actualFn.RetType) {
		return false
	}
	name, ok := expr.(*t.NodeExprName)
	if !ok {
		return false
	}
	function, ok := name.AssociatedNode.(*t.NodeFuncDef)
	if !ok || function.IsExternal {
		return false
	}
	name.NativeContextThunk = true
	function.NeedsNativeContextThunk = true
	return true
}

func constArrayIndex(expr t.NodeExpr) (uint64, bool) {
	switch n := expr.(type) {
	case *t.NodeExprLit:
		if n.LitType != t.TokLitNum {
			return 0, false
		}
		repr := strings.ReplaceAll(n.Value, "_", "")
		base := 10
		if strings.HasPrefix(repr, "0x") || strings.HasPrefix(repr, "0X") || strings.HasPrefix(repr, "0b") || strings.HasPrefix(repr, "0B") || strings.HasPrefix(repr, "0o") || strings.HasPrefix(repr, "0O") {
			base = 0
		}
		value, err := strconv.ParseUint(repr, base, 64)
		return value, err == nil
	case *t.NodeExprName:
		variable, ok := n.AssociatedNode.(*t.NodeExprVarDef)
		if !ok || !variable.IsConst || variable.Initializer == nil {
			return 0, false
		}
		return constArrayIndex(variable.Initializer)
	default:
		return 0, false
	}
}

func compatibleTypes(expected *t.NodeType, actual *t.NodeType) bool {
	if expected == nil || actual == nil {
		return false
	}
	// A typed pointer may be erased to ptr implicitly. Recovering a pointee type,
	// or changing one pointee type into another, is an explicit reinterpretation.
	if isRawPointerType(expected) && (isPointerType(actual) || isFunctionType(actual)) {
		return true
	}
	// Function values use ptr as their explicit type-erased callback ABI. Unlike
	// data pointers, there is no pointee type that cast.reinterpret can name.
	if isFunctionType(expected) && isRawPointerType(actual) {
		return true
	}
	if expected.Throws != actual.Throws {
		return false
	}
	if sameType(expected, actual) {
		return true
	}
	if intrinsicBackingCompatible(expected, actual) {
		return true
	}
	expectedFunc, expectedIsFunc := expected.KindNode.(*t.NodeTypeFunc)
	actualFunc, actualIsFunc := actual.KindNode.(*t.NodeTypeFunc)
	if expectedIsFunc || actualIsFunc {
		if !expectedIsFunc || !actualIsFunc || (expectedFunc.ContextABI != actualFunc.ContextABI && !(expectedFunc.ContextABI == t.ContextABIContextful && actualFunc.ContextABI == t.ContextABIContextless)) || len(expectedFunc.Args) != len(actualFunc.Args) {
			return false
		}
		for i := range expectedFunc.Args {
			if !compatibleTypes(expectedFunc.Args[i], actualFunc.Args[i]) {
				return false
			}
		}
		return compatibleTypes(expectedFunc.RetType, actualFunc.RetType)
	}
	if isNumberType(expected) && isNumberType(actual) {
		return true
	}
	expectedSlice, expectedIsSlice := expected.KindNode.(*t.NodeTypeSlice)
	actualSlice, actualIsSlice := actual.KindNode.(*t.NodeTypeSlice)
	if expectedIsSlice && actualIsSlice {
		return compatibleTypes(&t.NodeType{KindNode: expectedSlice.ElemKind}, &t.NodeType{KindNode: actualSlice.ElemKind})
	}
	if (isUntypedSlice(expected) && isTypedSlice(actual)) || (isTypedSlice(expected) && isUntypedSlice(actual)) {
		return true
	}
	return false
}

func intrinsicBackingCompatible(a, b *t.NodeType) bool {
	left, right := t.CoreTypeRoleOf(a), t.CoreTypeRoleOf(b)
	return left != t.CoreTypeNone && left == right
}

func isRawPointerType(node *t.NodeType) bool {
	if node == nil || node.Throws {
		return false
	}
	named, ok := node.KindNode.(*t.NodeTypeNamed)
	if !ok || len(named.GenericArgs) != 0 {
		return false
	}
	name, ok := named.NameNode.(*t.NodeNameSingle)
	return ok && name.Name == "ptr"
}

func isFunctionType(node *t.NodeType) bool {
	if node == nil {
		return false
	}
	_, ok := node.KindNode.(*t.NodeTypeFunc)
	return ok
}

func sameTypeKind(a t.NodeTypeKind, b t.NodeTypeKind) bool {
	switch ta := a.(type) {
	case *t.NodeTypeNamed:
		tb, ok := b.(*t.NodeTypeNamed)
		if !ok {
			return false
		}
		if len(ta.GenericArgs) != len(tb.GenericArgs) {
			return false
		}
		for i := range ta.GenericArgs {
			if !sameType(ta.GenericArgs[i], tb.GenericArgs[i]) {
				return false
			}
		}
		return flattenName(ta.NameNode) == flattenName(tb.NameNode)
	case *t.NodeTypePointer:
		tb, ok := b.(*t.NodeTypePointer)
		if !ok {
			return false
		}
		return sameTypeKind(ta.Kind, tb.Kind)
	case *t.NodeTypeRfc:
		tb, ok := b.(*t.NodeTypeRfc)
		if !ok {
			return false
		}
		return sameTypeKind(ta.Kind, tb.Kind)
	case *t.NodeTypeSlice:
		tb, ok := b.(*t.NodeTypeSlice)
		if !ok {
			return false
		}
		return sameTypeKind(ta.ElemKind, tb.ElemKind)
	case *t.NodeTypeFunc:
		tb, ok := b.(*t.NodeTypeFunc)
		if !ok {
			return false
		}
		if ta.ContextABI != tb.ContextABI || len(ta.Args) != len(tb.Args) {
			return false
		}
		for i := range ta.Args {
			if !sameType(ta.Args[i], tb.Args[i]) {
				return false
			}
		}
		return sameType(ta.RetType, tb.RetType)
	case *t.NodeTypeAbsolute:
		tb, ok := b.(*t.NodeTypeAbsolute)
		if !ok {
			return false
		}
		return ta.AbsoluteName == tb.AbsoluteName
	default:
		return false
	}
}
