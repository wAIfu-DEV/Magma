package monomorph

import t "Magma/src/types"

// resolveGenericCandidateExpressions resolves the only expression grammar
// ambiguity that needs cross-module knowledge: `name[type]` can be either a
// specialized generic function value or an ordinary subscript. Parsing keeps
// both interpretations; this pass runs after all modules and templates exist.
func (m *monoCtx) resolveGenericCandidateExpressions() {
	seenFunctions := map[*t.NodeFuncDef]bool{}
	for module, gl := range m.modules {
		for _, fn := range gl.FuncDefs {
			if fn == nil || seenFunctions[fn] {
				continue
			}
			seenFunctions[fn] = true
			for _, stmt := range fn.Body.Statements {
				m.resolveCandidateStmt(module, gl, stmt)
			}
		}
		for _, declaration := range gl.Declarations {
			switch node := declaration.(type) {
			case *t.NodeConstDef:
				node.Initializer = m.resolveCandidateExpr(module, gl, node.Initializer)
			case *t.NodeExprVarDef:
				node.Initializer = m.resolveCandidateExpr(module, gl, node.Initializer)
			}
		}
	}
}

func (m *monoCtx) resolveCandidateBody(module string, gl *t.NodeGlobal, body *t.NodeBody) {
	if body == nil {
		return
	}
	for _, stmt := range body.Statements {
		m.resolveCandidateStmt(module, gl, stmt)
	}
}

func (m *monoCtx) resolveCandidateStmt(module string, gl *t.NodeGlobal, stmt t.NodeStatement) {
	switch node := stmt.(type) {
	case *t.NodeStmtRet:
		node.Expression = m.resolveCandidateExpr(module, gl, node.Expression)
	case *t.NodeStmtExpr:
		node.Expression = m.resolveCandidateExpr(module, gl, node.Expression)
	case *t.NodeStmtThrow:
		node.Expression = m.resolveCandidateExpr(module, gl, node.Expression)
	case *t.NodeStmtIf:
		node.CondExpr = m.resolveCandidateExpr(module, gl, node.CondExpr)
		m.resolveCandidateBody(module, gl, &node.Body)
		if node.NextCondStmt != nil {
			m.resolveCandidateStmt(module, gl, node.NextCondStmt)
		}
	case *t.NodeStmtElse:
		m.resolveCandidateBody(module, gl, &node.Body)
	case *t.NodeStmtWhile:
		node.CondExpr = m.resolveCandidateExpr(module, gl, node.CondExpr)
		m.resolveCandidateBody(module, gl, &node.Body)
	case *t.NodeStmtDefer:
		if node.IsBody {
			m.resolveCandidateBody(module, gl, &node.Body)
		} else {
			node.Expression = m.resolveCandidateExpr(module, gl, node.Expression)
		}
	}
}

func (m *monoCtx) resolveCandidateExpr(module string, gl *t.NodeGlobal, expr t.NodeExpr) t.NodeExpr {
	if expr == nil {
		return nil
	}
	switch node := expr.(type) {
	case *t.NodeExprUnary:
		node.Operand = m.resolveCandidateExpr(module, gl, node.Operand)
	case *t.NodeExprArray:
		node.Length = m.resolveCandidateExpr(module, gl, node.Length)
		for i := range node.Entries {
			node.Entries[i].Index = m.resolveCandidateExpr(module, gl, node.Entries[i].Index)
			node.Entries[i].Value = m.resolveCandidateExpr(module, gl, node.Entries[i].Value)
		}
	case *t.NodeExprCall:
		node.Callee = m.resolveCandidateExpr(module, gl, node.Callee)
		for i := range node.Args {
			node.Args[i] = m.resolveCandidateExpr(module, gl, node.Args[i])
		}
	case *t.NodeExprStructInit:
		for i := range node.Fields {
			node.Fields[i].Expression = m.resolveCandidateExpr(module, gl, node.Fields[i].Expression)
		}
	case *t.NodeExprMemberAccess:
		node.Target = m.resolveCandidateExpr(module, gl, node.Target)
	case *t.NodeExprSubscript:
		node.Target = m.resolveCandidateExpr(module, gl, node.Target)
		node.Expr = m.resolveCandidateExpr(module, gl, node.Expr)
		if len(node.GenericCandidate) > 0 {
			if name, ok := node.Target.(*t.NodeExprName); ok && m.isGenericFunctionName(module, gl, name.Name) {
				return &t.NodeExprName{Tk: name.Tk, Name: name.Name, GenericArgs: node.GenericCandidate}
			}
			node.GenericCandidate = nil
		}
	case *t.NodeExprBinary:
		node.Left = m.resolveCandidateExpr(module, gl, node.Left)
		node.Right = m.resolveCandidateExpr(module, gl, node.Right)
	case *t.NodeExprVarDef:
		node.Initializer = m.resolveCandidateExpr(module, gl, node.Initializer)
	case *t.NodeExprVarDefAssign:
		node.AssignExpr = m.resolveCandidateExpr(module, gl, node.AssignExpr)
	case *t.NodeExprAssign:
		node.Left = m.resolveCandidateExpr(module, gl, node.Left)
		node.Right = m.resolveCandidateExpr(module, gl, node.Right)
	case *t.NodeExprTry:
		node.Call = m.resolveCandidateExpr(module, gl, node.Call)
	case *t.NodeExprAddrof:
		node.Expr = m.resolveCandidateExpr(module, gl, node.Expr)
	case *t.NodeExprDestructureAssign:
		if call, ok := m.resolveCandidateExpr(module, gl, node.Call).(*t.NodeExprCall); ok {
			node.Call = call
		}
	}
	return expr
}

func (m *monoCtx) isGenericFunctionName(module string, gl *t.NodeGlobal, name t.NodeName) bool {
	targetModule, baseName, err := resolveQualifiedName(m.modules, module, gl, name)
	if err != nil {
		return false
	}
	_, found := m.funcTemplates[makeTemplateKey(targetModule, baseName)]
	return found
}
