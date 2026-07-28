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

func compatibleInitializer(expected *t.NodeType, expr t.NodeExpr) bool {
	actual := expr.GetInferredType()
	if compatibleTypes(expected, actual) {
		return true
	}
	if lit, ok := expr.(*t.NodeExprLit); ok && lit.LitType == t.TokLitNum && isNumberType(expected) {
		return true
	}
	return false
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
	if isPointerType(expected) && isPointerType(actual) {
		return true
	}
	if expected.Throws != actual.Throws {
		return false
	}
	if sameType(expected, actual) {
		return true
	}
	expectedFunc, expectedIsFunc := expected.KindNode.(*t.NodeTypeFunc)
	actualFunc, actualIsFunc := actual.KindNode.(*t.NodeTypeFunc)
	if expectedIsFunc || actualIsFunc {
		if (expectedIsFunc && isPointerType(actual)) || (actualIsFunc && isPointerType(expected)) {
			return true
		}
		if !expectedIsFunc || !actualIsFunc || len(expectedFunc.Args) != len(actualFunc.Args) {
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
		if len(ta.Args) != len(tb.Args) {
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
