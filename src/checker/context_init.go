package checker

import (
	"Magma/src/comp_err"
	t "Magma/src/types"
)

type contextInitState struct {
	file        *t.FileCtx
	initialized bool
}

func contextInitError(state *contextInitState, token *t.Token) error {
	return comp_err.CompilationErrorToken(state.file, token,
		"implicit 'ctx' may be used before it is initialized",
		"a noctx function must assign a context.Ctx to 'ctx' on every path before reading it or calling contextful code")
}

func implicitContextName(expr t.NodeExpr) bool {
	name, ok := expr.(*t.NodeExprName)
	if !ok {
		return false
	}
	variable, ok := name.AssociatedNode.(*t.NodeExprVarDef)
	return ok && variable.IsImplicitContext
}

func checkContextExpr(state *contextInitState, expr t.NodeExpr, assignmentTarget bool) error {
	if expr == nil {
		return nil
	}
	switch n := expr.(type) {
	case *t.NodeExprVoid, *t.NodeExprLit, *t.NodeExprSizeof, *t.NodeExprVarDef, *t.NodeExprDestructor:
		return nil
	case *t.NodeExprName:
		if !assignmentTarget && implicitContextName(n) && !state.initialized {
			return contextInitError(state, &n.Tk)
		}
	case *t.NodeExprUnary:
		return checkContextExpr(state, n.Operand, false)
	case *t.NodeExprArray:
		if err := checkContextExpr(state, n.Length, false); err != nil {
			return err
		}
		for _, entry := range n.Entries {
			if err := checkContextExpr(state, entry.Index, false); err != nil {
				return err
			}
			if err := checkContextExpr(state, entry.Value, false); err != nil {
				return err
			}
		}
	case *t.NodeExprCall:
		contextful := n.AssociatedFnDef != nil && !n.AssociatedFnDef.IsExternal && n.AssociatedFnDef.ContextABI == t.ContextABIContextful
		if n.IsFuncPointer && n.FuncPtrType != nil {
			if function, ok := n.FuncPtrType.KindNode.(*t.NodeTypeFunc); ok {
				contextful = function.ContextABI == t.ContextABIContextful
			}
		}
		if contextful && !state.initialized {
			return contextInitError(state, &n.Tk)
		}
		if err := checkContextExpr(state, n.Callee, false); err != nil {
			return err
		}
		if err := checkContextExpr(state, n.MemberOwnerExpr, false); err != nil {
			return err
		}
		for _, argument := range n.Args {
			if err := checkContextExpr(state, argument, false); err != nil {
				return err
			}
		}
	case *t.NodeExprStructInit:
		for _, field := range n.Fields {
			if err := checkContextExpr(state, field.Expression, false); err != nil {
				return err
			}
		}
	case *t.NodeExprProtoView:
		return checkContextExpr(state, n.Target, false)
	case *t.NodeExprSubscript:
		if err := checkContextExpr(state, n.Target, false); err != nil {
			return err
		}
		return checkContextExpr(state, n.Expr, false)
	case *t.NodeExprMemberAccess:
		return checkContextExpr(state, n.Target, false)
	case *t.NodeExprBinary:
		if err := checkContextExpr(state, n.Left, false); err != nil {
			return err
		}
		return checkContextExpr(state, n.Right, false)
	case *t.NodeExprVarDefAssign:
		return checkContextExpr(state, n.AssignExpr, false)
	case *t.NodeExprAssign:
		if implicitContextName(n.Left) {
			if err := checkContextExpr(state, n.Right, false); err != nil {
				return err
			}
			state.initialized = true
			return nil
		}
		if err := checkContextExpr(state, n.Left, true); err != nil {
			return err
		}
		return checkContextExpr(state, n.Right, false)
	case *t.NodeExprTry:
		return checkContextExpr(state, n.Call, false)
	case *t.NodeExprDestructureAssign:
		return checkContextExpr(state, n.Call, false)
	case *t.NodeExprAddrof:
		return checkContextExpr(state, n.Expr, false)
	case *t.NodeExprMove:
		return checkContextExpr(state, n.Expr, false)
	}
	return nil
}

func checkContextBody(state *contextInitState, body *t.NodeBody) (bool, error) {
	for _, statement := range body.Statements {
		switch n := statement.(type) {
		case *t.NodeStmtExpr:
			if err := checkContextExpr(state, n.Expression, false); err != nil {
				return true, err
			}
		case *t.NodeStmtRet:
			if err := checkContextExpr(state, n.Expression, false); err != nil {
				return true, err
			}
			return false, nil
		case *t.NodeStmtThrow:
			if err := checkContextExpr(state, n.Expression, false); err != nil {
				return true, err
			}
			return false, nil
		case *t.NodeStmtIf:
			if err := checkContextExpr(state, n.CondExpr, false); err != nil {
				return true, err
			}
			incoming := state.initialized
			outputs := []bool{}
			branch := &contextInitState{file: state.file, initialized: incoming}
			falls, err := checkContextBody(branch, &n.Body)
			if err != nil {
				return true, err
			}
			if falls {
				outputs = append(outputs, branch.initialized)
			}
			hasElse := false
			for next := n.NextCondStmt; next != nil; {
				switch alternative := next.(type) {
				case *t.NodeStmtIf:
					branch = &contextInitState{file: state.file, initialized: incoming}
					if err := checkContextExpr(branch, alternative.CondExpr, false); err != nil {
						return true, err
					}
					falls, err = checkContextBody(branch, &alternative.Body)
					if err != nil {
						return true, err
					}
					if falls {
						outputs = append(outputs, branch.initialized)
					}
					next = alternative.NextCondStmt
				case *t.NodeStmtElse:
					hasElse = true
					branch = &contextInitState{file: state.file, initialized: incoming}
					falls, err = checkContextBody(branch, &alternative.Body)
					if err != nil {
						return true, err
					}
					if falls {
						outputs = append(outputs, branch.initialized)
					}
					next = nil
				default:
					next = nil
				}
			}
			if !hasElse {
				outputs = append(outputs, incoming)
			}
			if len(outputs) == 0 {
				return false, nil
			}
			state.initialized = true
			for _, initialized := range outputs {
				state.initialized = state.initialized && initialized
			}
		case *t.NodeStmtWhile:
			if err := checkContextExpr(state, n.CondExpr, false); err != nil {
				return true, err
			}
			branch := &contextInitState{file: state.file, initialized: state.initialized}
			if _, err := checkContextBody(branch, &n.Body); err != nil {
				return true, err
			}
		case *t.NodeStmtFor:
			if err := checkContextExpr(state, n.DeclExpr, false); err != nil {
				return true, err
			}
			if err := checkContextExpr(state, n.BoundExpr, false); err != nil {
				return true, err
			}
			branch := &contextInitState{file: state.file, initialized: state.initialized}
			if _, err := checkContextBody(branch, &n.Body); err != nil {
				return true, err
			}
		case *t.NodeStmtBounded:
			for _, predicate := range n.Predicates {
				if err := checkContextExpr(state, predicate, false); err != nil {
					return true, err
				}
			}
			if _, err := checkContextBody(state, &n.Body); err != nil {
				return true, err
			}
		case *t.NodeStmtUnsafe:
			if _, err := checkContextBody(state, &n.Body); err != nil {
				return true, err
			}
		case *t.NodeStmtDefer:
			if n.IsBody {
				branch := &contextInitState{file: state.file, initialized: state.initialized}
				if _, err := checkContextBody(branch, &n.Body); err != nil {
					return true, err
				}
			} else if err := checkContextExpr(state, n.Expression, false); err != nil {
				return true, err
			}
		case *t.NodeStmtBreak, *t.NodeStmtContinue:
			return false, nil
		}
	}
	return true, nil
}

func checkContextInitialization(file *t.FileCtx, fn *t.NodeFuncDef) error {
	if fn.IsExternal || fn.ContextABI == t.ContextABIContextful {
		return nil
	}
	state := &contextInitState{file: file}
	_, err := checkContextBody(state, &fn.Body)
	return err
}
