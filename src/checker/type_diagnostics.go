package checker

import (
	t "Magma/src/types"
	"fmt"
	"strings"
)

func flattenCallee(expr t.NodeExpr) string {
	switch n := expr.(type) {
	case *t.NodeExprName:
		return flattenName(n.Name)
	case *t.NodeExprMemberAccess:
		return flattenCallee(n.Target) + "." + n.Member
	case *t.NodeExprCall:
		return flattenCallee(n.Callee) + "()"
	default:
		return "<expr>"
	}
}

func unmangledName(name string) string {
	return t.SourceName(name)
}

func qualifiedDisplayName(raw string, display string) string {
	raw = unmangledName(raw)
	if strings.Contains(display, ".") {
		return display
	}
	if separator := strings.LastIndex(raw, "."); separator >= 0 {
		return raw[:separator+1] + display
	}
	return display
}

func callDisplayName(call *t.NodeExprCall) string {
	if call.AssociatedFnDef != nil && call.AssociatedFnDef.DisplayName != "" {
		return qualifiedDisplayName(flattenCallee(call.Callee), call.AssociatedFnDef.DisplayName)
	}
	return unmangledName(flattenCallee(call.Callee))
}

func expressionDisplayName(expr t.NodeExpr) string {
	if call, ok := expr.(*t.NodeExprCall); ok {
		return callDisplayName(call)
	}
	if name, ok := expr.(*t.NodeExprName); ok {
		if fn, ok := name.AssociatedNode.(*t.NodeFuncDef); ok && fn.DisplayName != "" {
			return qualifiedDisplayName(flattenCallee(expr), fn.DisplayName)
		}
	}
	return unmangledName(flattenCallee(expr))
}

func expressionSourceToken(expr t.NodeExpr) *t.Token {
	switch n := expr.(type) {
	case *t.NodeExprName:
		return &n.Tk
	case *t.NodeExprLit:
		return &n.Tk
	case *t.NodeExprUnary:
		return &n.Tk
	case *t.NodeExprBinary:
		return &n.Tk
	case *t.NodeExprCall:
		return &n.Tk
	case *t.NodeExprMemberAccess:
		return &n.Tk
	case *t.NodeExprSubscript:
		return &n.Tk
	case *t.NodeExprTry:
		return &n.Tk
	case *t.NodeExprSizeof:
		return &n.Tk
	case *t.NodeExprAddrof:
		return &n.Tk
	case *t.NodeExprMove:
		return &n.Tk
	case *t.NodeExprAssign:
		return &n.Tk
	case *t.NodeExprVarDefAssign:
		return &n.Tk
	}
	return &t.Token{}
}

func functionPointerMismatchHint(expected *t.NodeType, actual *t.NodeType, argument t.NodeExpr) string {
	expectedFunc, expectedIsFunc := expected.KindNode.(*t.NodeTypeFunc)
	actualFunc, actualIsFunc := actual.KindNode.(*t.NodeTypeFunc)
	if !expectedIsFunc || !actualIsFunc {
		return ""
	}

	if expectedFunc.RetType.Throws != actualFunc.RetType.Throws {
		return fmt.Sprintf(
			"function pointer '%s' returns '%s', but this parameter requires '%s'; throwing and non-throwing function pointers are not interchangeable",
			expressionDisplayName(argument),
			flattenType(actualFunc.RetType),
			flattenType(expectedFunc.RetType),
		)
	}

	return "pass a function pointer whose parameter and return types match the required signature"
}
