package monomorph

import t "Magma/src/types"

func substituteType(tp *t.NodeType, subst map[string]*t.NodeType) *t.NodeType {
	if tp == nil {
		return nil
	}
	switch n := tp.KindNode.(type) {
	case *t.NodeTypeNamed:
		if nn, ok := n.NameNode.(*t.NodeNameSingle); ok && len(n.GenericArgs) == 0 {
			if v, ok := subst[nn.Name]; ok {
				out := cloneType(v)
				out.Throws = tp.Throws
				out.Owned = tp.Owned
				out.Destructor = tp.Destructor
				return out
			}
		}
		out := cloneType(tp)
		nt := out.KindNode.(*t.NodeTypeNamed)
		for i := range nt.GenericArgs {
			nt.GenericArgs[i] = substituteType(nt.GenericArgs[i], subst)
		}
		return out
	case *t.NodeTypePointer:
		return &t.NodeType{
			Throws: tp.Throws, Owned: tp.Owned, Destructor: tp.Destructor,
			KindNode: &t.NodeTypePointer{
				Kind: substituteType(&t.NodeType{KindNode: n.Kind}, subst).KindNode,
			},
		}
	case *t.NodeTypeRfc:
		return &t.NodeType{
			Throws: tp.Throws, Owned: tp.Owned, Destructor: tp.Destructor,
			KindNode: &t.NodeTypeRfc{
				Kind: substituteType(&t.NodeType{KindNode: n.Kind}, subst).KindNode,
			},
		}
	case *t.NodeTypeSlice:
		return &t.NodeType{
			Throws: tp.Throws, Owned: tp.Owned, Destructor: tp.Destructor,
			KindNode: &t.NodeTypeSlice{
				ElemKind: substituteType(&t.NodeType{KindNode: n.ElemKind}, subst).KindNode,
			},
		}
	case *t.NodeTypeFunc:
		out := &t.NodeTypeFunc{
			Args:    make([]*t.NodeType, len(n.Args)),
			RetType: substituteType(n.RetType, subst),
		}
		for i, a := range n.Args {
			out.Args[i] = substituteType(a, subst)
		}
		return &t.NodeType{Throws: tp.Throws, Owned: tp.Owned, Destructor: tp.Destructor, KindNode: out}
	default:
		return cloneType(tp)
	}
}

func substituteExpr(expr t.NodeExpr, subst map[string]*t.NodeType) {
	switch n := expr.(type) {
	case *t.NodeExprName:
		for i := range n.GenericArgs {
			n.GenericArgs[i] = substituteType(n.GenericArgs[i], subst)
		}
	case *t.NodeExprUnary:
		substituteExpr(n.Operand, subst)
	case *t.NodeExprArray:
		n.ElemType = substituteType(n.ElemType, subst)
		n.LengthType = substituteType(n.LengthType, subst)
		n.InfType = substituteType(n.InfType, subst)
		substituteExpr(n.Length, subst)
		for i := range n.Entries {
			substituteExpr(n.Entries[i].Index, subst)
			substituteExpr(n.Entries[i].Value, subst)
		}
	case *t.NodeExprCall:
		substituteExpr(n.Callee, subst)
		for _, a := range n.Args {
			substituteExpr(a, subst)
		}
		for i := range n.GenericArgs {
			n.GenericArgs[i] = substituteType(n.GenericArgs[i], subst)
		}
	case *t.NodeExprStructInit:
		n.Type = substituteType(n.Type, subst)
		for i := range n.Fields {
			n.Fields[i].FieldType = substituteType(n.Fields[i].FieldType, subst)
			substituteExpr(n.Fields[i].Expression, subst)
		}
	case *t.NodeExprSubscript:
		substituteExpr(n.Target, subst)
		substituteExpr(n.Expr, subst)
		for i := range n.GenericCandidate {
			n.GenericCandidate[i] = substituteType(n.GenericCandidate[i], subst)
		}
		n.BoxType = substituteType(n.BoxType, subst)
		n.ElemType = substituteType(n.ElemType, subst)
		n.IndexType = substituteType(n.IndexType, subst)
	case *t.NodeExprBinary:
		substituteExpr(n.Left, subst)
		substituteExpr(n.Right, subst)
	case *t.NodeExprVarDef:
		n.Type = substituteType(n.Type, subst)
	case *t.NodeExprVarDefAssign:
		n.VarDef.Type = substituteType(n.VarDef.Type, subst)
		substituteExpr(n.AssignExpr, subst)
	case *t.NodeExprAssign:
		substituteExpr(n.Left, subst)
		substituteExpr(n.Right, subst)
	case *t.NodeExprTry:
		substituteExpr(n.Call, subst)
	case *t.NodeExprSizeof:
		n.Type = substituteType(n.Type, subst)
	case *t.NodeExprAddrof:
		substituteExpr(n.Expr, subst)
	case *t.NodeExprDestructureAssign:
		n.ValueDef.Type = substituteType(n.ValueDef.Type, subst)
		n.ErrDef.Type = substituteType(n.ErrDef.Type, subst)
		substituteExpr(n.Call, subst)
	case *t.NodeExprDestructor:
		n.VarDef.Type = substituteType(n.VarDef.Type, subst)
	}
}

func substituteStmt(stmt t.NodeStatement, subst map[string]*t.NodeType) {
	switch n := stmt.(type) {
	case *t.NodeStmtRet:
		substituteExpr(n.Expression, subst)
	case *t.NodeStmtExpr:
		substituteExpr(n.Expression, subst)
	case *t.NodeStmtThrow:
		substituteExpr(n.Expression, subst)
	case *t.NodeStmtIf:
		substituteExpr(n.CondExpr, subst)
		for _, s := range n.Body.Statements {
			substituteStmt(s, subst)
		}
		if n.NextCondStmt != nil {
			substituteStmt(n.NextCondStmt, subst)
		}
	case *t.NodeStmtElse:
		for _, s := range n.Body.Statements {
			substituteStmt(s, subst)
		}
	case *t.NodeStmtWhile:
		substituteExpr(n.CondExpr, subst)
		for _, s := range n.Body.Statements {
			substituteStmt(s, subst)
		}
	case *t.NodeStmtFor:
		substituteExpr(n.DeclExpr, subst)
		substituteExpr(n.BoundExpr, subst)
		for _, s := range n.Body.Statements {
			substituteStmt(s, subst)
		}
	case *t.NodeStmtDefer:
		if n.IsBody {
			for _, s := range n.Body.Statements {
				substituteStmt(s, subst)
			}
		} else {
			substituteExpr(n.Expression, subst)
		}
	}
}
