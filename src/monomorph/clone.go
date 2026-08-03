package monomorph

import (
	t "Magma/src/types"
	"strings"
)

func flattenName(name t.NodeName) string {
	switch n := name.(type) {
	case *t.NodeNameSingle:
		return n.Name
	case *t.NodeNameComposite:
		return strings.Join(n.Parts, ".")
	}
	return ""
}

func cloneName(name t.NodeName) t.NodeName {
	switch n := name.(type) {
	case *t.NodeNameSingle:
		return &t.NodeNameSingle{Tk: n.Tk, Name: n.Name}
	case *t.NodeNameComposite:
		parts := make([]string, len(n.Parts))
		copy(parts, n.Parts)
		tokens := make([]t.Token, len(n.Tokens))
		copy(tokens, n.Tokens)
		return &t.NodeNameComposite{Tokens: tokens, Parts: parts}
	}
	return nil
}

func cloneType(in *t.NodeType) *t.NodeType {
	if in == nil {
		return nil
	}

	out := &t.NodeType{Throws: in.Throws, Owned: in.Owned, Destructor: in.Destructor}

	switch n := in.KindNode.(type) {
	case *t.NodeTypeNamed:
		n2 := &t.NodeTypeNamed{NameNode: cloneName(n.NameNode)}
		if len(n.GenericArgs) > 0 {
			n2.GenericArgs = make([]*t.NodeType, len(n.GenericArgs))
			for i, a := range n.GenericArgs {
				n2.GenericArgs[i] = cloneType(a)
			}
		}
		out.KindNode = n2
	case *t.NodeTypeAbsolute:
		out.KindNode = &t.NodeTypeAbsolute{AbsoluteName: n.AbsoluteName, DisplayName: n.DisplayName}
	case *t.NodeTypePointer:
		out.KindNode = &t.NodeTypePointer{Kind: cloneType(&t.NodeType{KindNode: n.Kind}).KindNode}
	case *t.NodeTypeRfc:
		out.KindNode = &t.NodeTypeRfc{Kind: cloneType(&t.NodeType{KindNode: n.Kind}).KindNode}
	case *t.NodeTypeSlice:
		out.KindNode = &t.NodeTypeSlice{
			ElemKind: cloneType(&t.NodeType{KindNode: n.ElemKind}).KindNode,
		}
	case *t.NodeTypeFunc:
		n2 := &t.NodeTypeFunc{
			Args:    make([]*t.NodeType, len(n.Args)),
			RetType: cloneType(n.RetType),
		}
		for i, a := range n.Args {
			n2.Args[i] = cloneType(a)
		}
		out.KindNode = n2
	}
	return out
}

func cloneExpr(in t.NodeExpr) t.NodeExpr {
	switch n := in.(type) {
	case *t.NodeExprVoid:
		return &t.NodeExprVoid{VoidType: cloneType(n.VoidType)}
	case *t.NodeExprUnary:
		return &t.NodeExprUnary{Tk: n.Tk, Operator: n.Operator, Operand: cloneExpr(n.Operand), InfType: cloneType(n.InfType)}
	case *t.NodeExprLit:
		return &t.NodeExprLit{Tk: n.Tk, Value: n.Value, LitType: n.LitType, InfType: cloneType(n.InfType)}
	case *t.NodeExprArray:
		entries := make([]t.NodeArrayInitEntry, len(n.Entries))
		for i, entry := range n.Entries {
			entries[i] = t.NodeArrayInitEntry{Tk: entry.Tk, Index: cloneExpr(entry.Index), Value: cloneExpr(entry.Value), ResolvedIndex: entry.ResolvedIndex}
		}
		return &t.NodeExprArray{Tk: n.Tk, ElemType: cloneType(n.ElemType), Length: cloneExpr(n.Length), LengthType: cloneType(n.LengthType), Entries: entries, InfType: cloneType(n.InfType)}
	case *t.NodeExprName:
		genericArgs := make([]*t.NodeType, len(n.GenericArgs))
		for i, g := range n.GenericArgs {
			genericArgs[i] = cloneType(g)
		}
		return &t.NodeExprName{Tk: n.Tk, Name: cloneName(n.Name), GenericArgs: genericArgs, InfType: cloneType(n.InfType)}
	case *t.NodeExprCall:
		args := make([]t.NodeExpr, len(n.Args))
		for i, a := range n.Args {
			args[i] = cloneExpr(a)
		}
		typeArgs := make([]*t.NodeType, len(n.GenericArgs))
		for i, g := range n.GenericArgs {
			typeArgs[i] = cloneType(g)
		}
		return &t.NodeExprCall{
			Tk:          n.Tk,
			Callee:      cloneExpr(n.Callee),
			Args:        args,
			GenericArgs: typeArgs,
			InfType:     cloneType(n.InfType),
		}
	case *t.NodeExprStructInit:
		fields := make([]t.NodeStructFieldInit, len(n.Fields))
		for i, field := range n.Fields {
			fields[i] = t.NodeStructFieldInit{Tk: field.Tk, Name: field.Name, Expression: cloneExpr(field.Expression), FieldIndex: field.FieldIndex, FieldType: cloneType(field.FieldType)}
		}
		return &t.NodeExprStructInit{Tk: n.Tk, Type: cloneType(n.Type), Fields: fields}
	case *t.NodeExprMemberAccess:
		return &t.NodeExprMemberAccess{
			Tk:      n.Tk,
			Target:  cloneExpr(n.Target),
			Member:  n.Member,
			Access:  n.Access,
			InfType: cloneType(n.InfType),
		}
	case *t.NodeExprSubscript:
		candidate := make([]*t.NodeType, len(n.GenericCandidate))
		for i, arg := range n.GenericCandidate {
			candidate[i] = cloneType(arg)
		}
		return &t.NodeExprSubscript{
			Tk:               n.Tk,
			Target:           cloneExpr(n.Target),
			Expr:             cloneExpr(n.Expr),
			GenericCandidate: candidate,
			BoxType:          cloneType(n.BoxType),
			ElemType:         cloneType(n.ElemType),
			IndexType:        cloneType(n.IndexType),
		}
	case *t.NodeExprBinary:
		return &t.NodeExprBinary{
			Tk:          n.Tk,
			Operator:    n.Operator,
			Left:        cloneExpr(n.Left),
			Right:       cloneExpr(n.Right),
			InfType:     cloneType(n.InfType),
			OperandType: cloneType(n.OperandType),
		}
	case *t.NodeExprVarDef:
		return &t.NodeExprVarDef{
			Name:        cloneName(n.Name),
			Type:        cloneType(n.Type),
			Initializer: cloneExpr(n.Initializer),
			IsConst:     n.IsConst,
			AbsName:     n.AbsName,
			RetFlagId:   n.RetFlagId,
			Storage:     n.Storage,
			IsReturned:  n.IsReturned,
			IsGlobal:    n.IsGlobal,
		}
	case *t.NodeExprVarDefAssign:
		return &t.NodeExprVarDefAssign{
			Tk:         n.Tk,
			VarDef:     cloneExpr(n.VarDef).(*t.NodeExprVarDef),
			AssignExpr: cloneExpr(n.AssignExpr),
		}
	case *t.NodeExprAssign:
		return &t.NodeExprAssign{
			Tk:      n.Tk,
			Left:    cloneExpr(n.Left),
			Right:   cloneExpr(n.Right),
			InfType: cloneType(n.InfType),
		}
	case *t.NodeExprTry:
		return &t.NodeExprTry{Call: cloneExpr(n.Call), Tk: n.Tk, Pos: n.Pos, InfType: cloneType(n.InfType)}
	case *t.NodeExprSizeof:
		return &t.NodeExprSizeof{Tk: n.Tk, Type: cloneType(n.Type), InfType: cloneType(n.InfType)}
	case *t.NodeExprAddrof:
		return &t.NodeExprAddrof{Tk: n.Tk, Expr: cloneExpr(n.Expr), InfType: cloneType(n.InfType)}
	case *t.NodeExprDestructureAssign:
		return &t.NodeExprDestructureAssign{
			ValueDef: *cloneExpr(&n.ValueDef).(*t.NodeExprVarDef),
			ErrDef:   *cloneExpr(&n.ErrDef).(*t.NodeExprVarDef),
			Call:     cloneExpr(n.Call).(*t.NodeExprCall),
		}
	case *t.NodeExprDestructor:
		return &t.NodeExprDestructor{
			VarDef:     cloneExpr(n.VarDef).(*t.NodeExprVarDef),
			Destructor: n.Destructor,
		}
	}
	return nil
}

func cloneStmt(in t.NodeStatement) t.NodeStatement {
	switch n := in.(type) {
	case *t.NodeStmtRet:
		return &t.NodeStmtRet{Tk: n.Tk, Expression: cloneExpr(n.Expression), OwnerFuncType: cloneType(n.OwnerFuncType)}
	case *t.NodeStmtContinue:
		return &t.NodeStmtContinue{}
	case *t.NodeStmtBreak:
		return &t.NodeStmtBreak{}
	case *t.NodeStmtExpr:
		return &t.NodeStmtExpr{Expression: cloneExpr(n.Expression)}
	case *t.NodeStmtThrow:
		return &t.NodeStmtThrow{Tk: n.Tk, Expression: cloneExpr(n.Expression), Pos: n.Pos}
	case *t.NodeStmtIf:
		out := &t.NodeStmtIf{
			Tk:       n.Tk,
			CondExpr: cloneExpr(n.CondExpr),
			Body:     cloneBody(&n.Body),
		}
		if n.NextCondStmt != nil {
			out.NextCondStmt = cloneStmt(n.NextCondStmt)
		}
		return out
	case *t.NodeStmtElse:
		return &t.NodeStmtElse{Body: cloneBody(&n.Body)}
	case *t.NodeStmtWhile:
		return &t.NodeStmtWhile{
			Tk:       n.Tk,
			CondExpr: cloneExpr(n.CondExpr),
			Body:     cloneBody(&n.Body),
		}
	case *t.NodeLlvm:
		return &t.NodeLlvm{Text: n.Text}
	case *t.NodeStmtDefer:
		return &t.NodeStmtDefer{
			Expression: cloneExpr(n.Expression),
			Body:       cloneBody(&n.Body),
			IsBody:     n.IsBody,
			OnError:    n.OnError,
		}
	}
	return nil
}

func cloneBody(in *t.NodeBody) t.NodeBody {
	if in == nil {
		return t.NodeBody{}
	}
	out := t.NodeBody{Statements: make([]t.NodeStatement, len(in.Statements))}
	for i, s := range in.Statements {
		out.Statements[i] = cloneStmt(s)
	}
	return out
}

func cloneFuncDef(in *t.NodeFuncDef) *t.NodeFuncDef {
	if in == nil {
		return nil
	}
	out := &t.NodeFuncDef{
		Class: t.NodeGenericClass{
			NameNode:        cloneName(in.Class.NameNode),
			ArgsNode:        t.NodeArgList{Args: make([]t.NodeArg, len(in.Class.ArgsNode.Args))},
			TypeParams:      append([]string{}, in.Class.TypeParams...),
			OwnerTypeParams: append([]string{}, in.Class.OwnerTypeParams...),
		},
		ReturnType:     cloneType(in.ReturnType),
		Body:           cloneBody(&in.Body),
		AbsName:        in.AbsName,
		NoAliasName:    in.NoAliasName,
		DisplayName:    in.DisplayName,
		IsDestructor:   in.IsDestructor,
		IsMember:       in.IsMember,
		IsEntryPoint:   in.IsEntryPoint,
		IsExternal:     in.IsExternal,
		IsPublic:       in.IsPublic,
		ExportName:     in.ExportName,
		ExportABI:      in.ExportABI,
		ErrorPredicate: in.ErrorPredicate,
	}
	for i, a := range in.Class.ArgsNode.Args {
		out.Class.ArgsNode.Args[i] = t.NodeArg{
			Tk:       a.Tk,
			Name:     a.Name,
			TypeNode: cloneType(a.TypeNode),
		}
	}
	return out
}

func cloneStructDef(in *t.NodeStructDef) *t.NodeStructDef {
	if in == nil {
		return nil
	}
	out := &t.NodeStructDef{
		IsPublic: in.IsPublic,
		AbsName:  in.AbsName,
		Class: t.NodeGenericClass{
			NameNode:        cloneName(in.Class.NameNode),
			ArgsNode:        t.NodeArgList{Args: make([]t.NodeArg, len(in.Class.ArgsNode.Args))},
			TypeParams:      append([]string{}, in.Class.TypeParams...),
			OwnerTypeParams: append([]string{}, in.Class.OwnerTypeParams...),
		},
	}
	for i, a := range in.Class.ArgsNode.Args {
		out.Class.ArgsNode.Args[i] = t.NodeArg{
			Tk:       a.Tk,
			Name:     a.Name,
			TypeNode: cloneType(a.TypeNode),
		}
	}
	return out
}
