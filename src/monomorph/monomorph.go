package monomorph

import (
	scopeinfo "Magma/src/scope_info"
	t "Magma/src/types"
	"fmt"
	"strings"
)

type monoCtx struct {
	shared *t.SharedState

	modules map[string]*t.NodeGlobal

	structTemplates map[string]*t.NodeStructDef
	funcTemplates   map[string]*t.NodeFuncDef
	memberTemplates map[string]*t.NodeFuncDef

	structInstances map[string]string
	funcInstances   map[string]string
	memberInstances map[string]string

	queuedStruct map[*t.NodeStructDef]bool
	queuedFunc   map[*t.NodeFuncDef]bool
	queuedVar    map[*t.NodeExprVarDef]bool

	structQueue []structWorkItem
	funcQueue   []*t.NodeFuncDef
	varQueue    []*t.NodeExprVarDef
}

type structWorkItem struct {
	module string
	st     *t.NodeStructDef
}

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
		return &t.NodeNameSingle{Name: n.Name}
	case *t.NodeNameComposite:
		parts := make([]string, len(n.Parts))
		copy(parts, n.Parts)
		return &t.NodeNameComposite{Parts: parts}
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
		out.KindNode = &t.NodeTypeAbsolute{AbsoluteName: n.AbsoluteName}
	case *t.NodeTypePointer:
		out.KindNode = &t.NodeTypePointer{Kind: cloneType(&t.NodeType{KindNode: n.Kind}).KindNode}
	case *t.NodeTypeRfc:
		out.KindNode = &t.NodeTypeRfc{Kind: cloneType(&t.NodeType{KindNode: n.Kind}).KindNode}
	case *t.NodeTypeSlice:
		out.KindNode = &t.NodeTypeSlice{
			HasSize:  n.HasSize,
			Size:     n.Size,
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
		return &t.NodeExprUnary{Operator: n.Operator, Operand: cloneExpr(n.Operand), InfType: cloneType(n.InfType)}
	case *t.NodeExprLit:
		return &t.NodeExprLit{Value: n.Value, LitType: n.LitType, InfType: cloneType(n.InfType)}
	case *t.NodeExprName:
		genericArgs := make([]*t.NodeType, len(n.GenericArgs))
		for i, g := range n.GenericArgs {
			genericArgs[i] = cloneType(g)
		}
		return &t.NodeExprName{Name: cloneName(n.Name), GenericArgs: genericArgs, InfType: cloneType(n.InfType)}
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
			Callee:      cloneExpr(n.Callee),
			Args:        args,
			GenericArgs: typeArgs,
			InfType:     cloneType(n.InfType),
		}
	case *t.NodeExprMemberAccess:
		return &t.NodeExprMemberAccess{
			Target:  cloneExpr(n.Target),
			Member:  n.Member,
			Access:  n.Access,
			InfType: cloneType(n.InfType),
		}
	case *t.NodeExprSubscript:
		return &t.NodeExprSubscript{
			Target:   cloneExpr(n.Target),
			Expr:     cloneExpr(n.Expr),
			BoxType:  cloneType(n.BoxType),
			ElemType: cloneType(n.ElemType),
		}
	case *t.NodeExprBinary:
		return &t.NodeExprBinary{
			Operator: n.Operator,
			Left:     cloneExpr(n.Left),
			Right:    cloneExpr(n.Right),
			InfType:  cloneType(n.InfType),
		}
	case *t.NodeExprVarDef:
		return &t.NodeExprVarDef{
			Name:       cloneName(n.Name),
			Type:       cloneType(n.Type),
			AbsName:    n.AbsName,
			RetFlagId:  n.RetFlagId,
			IsSsa:      n.IsSsa,
			IsReturned: n.IsReturned,
			IsGlobal:   n.IsGlobal,
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
		return &t.NodeExprTry{Call: cloneExpr(n.Call)}
	case *t.NodeExprSizeof:
		return &t.NodeExprSizeof{Type: cloneType(n.Type), InfType: cloneType(n.InfType)}
	case *t.NodeExprAddrof:
		return &t.NodeExprAddrof{Expr: cloneExpr(n.Expr), InfType: cloneType(n.InfType)}
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
		return &t.NodeStmtRet{Expression: cloneExpr(n.Expression), OwnerFuncType: cloneType(n.OwnerFuncType)}
	case *t.NodeStmtContinue:
		return &t.NodeStmtContinue{}
	case *t.NodeStmtBreak:
		return &t.NodeStmtBreak{}
	case *t.NodeStmtExpr:
		return &t.NodeStmtExpr{Expression: cloneExpr(n.Expression)}
	case *t.NodeStmtThrow:
		return &t.NodeStmtThrow{Expression: cloneExpr(n.Expression)}
	case *t.NodeStmtIf:
		out := &t.NodeStmtIf{
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
		ReturnType:   cloneType(in.ReturnType),
		Body:         cloneBody(&in.Body),
		AbsName:      in.AbsName,
		NoAliasName:  in.NoAliasName,
		DeferCnt:     in.DeferCnt,
		HasDefer:     in.HasDefer,
		IsDestructor: in.IsDestructor,
		IsExternal:   in.IsExternal,
	}
	for i, a := range in.Class.ArgsNode.Args {
		out.Class.ArgsNode.Args[i] = t.NodeArg{
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
		Class: t.NodeGenericClass{
			NameNode:        cloneName(in.Class.NameNode),
			ArgsNode:        t.NodeArgList{Args: make([]t.NodeArg, len(in.Class.ArgsNode.Args))},
			TypeParams:      append([]string{}, in.Class.TypeParams...),
			OwnerTypeParams: append([]string{}, in.Class.OwnerTypeParams...),
		},
	}
	for i, a := range in.Class.ArgsNode.Args {
		out.Class.ArgsNode.Args[i] = t.NodeArg{
			Name:     a.Name,
			TypeNode: cloneType(a.TypeNode),
		}
	}
	return out
}

func CanonicalTypeSignature(tp *t.NodeType) string {
	if tp == nil {
		return "nil"
	}
	switch n := tp.KindNode.(type) {
	case *t.NodeTypeAbsolute:
		return "A_" + strings.ReplaceAll(n.AbsoluteName, ".", "__")
	case *t.NodeTypeNamed:
		base := "N_" + strings.ReplaceAll(flattenName(n.NameNode), ".", "__")
		if len(n.GenericArgs) == 0 {
			return base
		}
		parts := make([]string, len(n.GenericArgs))
		for i, g := range n.GenericArgs {
			parts[i] = CanonicalTypeSignature(g)
		}
		return base + "__G__" + strings.Join(parts, "__")
	case *t.NodeTypePointer:
		return "P__" + CanonicalTypeSignature(&t.NodeType{KindNode: n.Kind})
	case *t.NodeTypeRfc:
		return "R__" + CanonicalTypeSignature(&t.NodeType{KindNode: n.Kind})
	case *t.NodeTypeSlice:
		prefix := "S"
		if n.HasSize {
			prefix = fmt.Sprintf("A%d", n.Size)
		}
		return prefix + "__" + CanonicalTypeSignature(&t.NodeType{KindNode: n.ElemKind})
	case *t.NodeTypeFunc:
		argParts := make([]string, len(n.Args))
		for i, a := range n.Args {
			argParts[i] = CanonicalTypeSignature(a)
		}
		return "F__" + strings.Join(argParts, "__") + "__RET__" + CanonicalTypeSignature(n.RetType)
	default:
		return "Undef"
	}
}

func MangleSpecializedName(base string, args []*t.NodeType) string {
	if len(args) == 0 {
		return base
	}
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = CanonicalTypeSignature(a)
	}
	return base + "__g__" + strings.Join(parts, "__")
}

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
				HasSize:  n.HasSize,
				Size:     n.Size,
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
	case *t.NodeExprCall:
		substituteExpr(n.Callee, subst)
		for _, a := range n.Args {
			substituteExpr(a, subst)
		}
		for i := range n.GenericArgs {
			n.GenericArgs[i] = substituteType(n.GenericArgs[i], subst)
		}
	case *t.NodeExprSubscript:
		substituteExpr(n.Target, subst)
		substituteExpr(n.Expr, subst)
		n.BoxType = substituteType(n.BoxType, subst)
		n.ElemType = substituteType(n.ElemType, subst)
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

func makeTemplateKey(module string, name string) string {
	return module + "." + name
}

func makeMemberTemplateKey(module string, ownerName string, memberName string) string {
	return module + "." + ownerName + "." + memberName
}

func makeInstanceKey(module string, name string, args []*t.NodeType) string {
	return module + "." + name + "[" + strings.Join(func() []string {
		out := make([]string, len(args))
		for i, a := range args {
			out[i] = CanonicalTypeSignature(a)
		}
		return out
	}(), ",") + "]"
}

func makeMemberInstanceKey(module string, ownerName string, memberName string, args []*t.NodeType) string {
	return makeInstanceKey(module, ownerName+"."+memberName, args)
}

func splitAbsoluteStructName(absName string) (string, string, error) {
	i := strings.Index(absName, ".")
	if i < 0 || i == len(absName)-1 {
		return "", "", fmt.Errorf("invalid absolute type name: %s", absName)
	}
	return absName[:i], absName[i+1:], nil
}

func cloneEnv(in map[string]*t.NodeType) map[string]*t.NodeType {
	out := map[string]*t.NodeType{}
	for k, v := range in {
		out[k] = cloneType(v)
	}
	return out
}

func resolveQualifiedName(module string, gl *t.NodeGlobal, name t.NodeName) (string, string, error) {
	switch n := name.(type) {
	case *t.NodeNameSingle:
		return module, n.Name, nil
	case *t.NodeNameComposite:
		if len(n.Parts) < 2 {
			return "", "", fmt.Errorf("invalid composite name")
		}
		targetModule, ok := gl.ImportAlias[n.Parts[0]]
		if !ok {
			return "", "", fmt.Errorf("unknown module alias '%s'", n.Parts[0])
		}
		return targetModule, n.Parts[1], nil
	}
	return "", "", fmt.Errorf("invalid name node")
}

func (m *monoCtx) queueStruct(module string, st *t.NodeStructDef) {
	if st == nil || m.queuedStruct[st] {
		return
	}
	m.queuedStruct[st] = true
	m.structQueue = append(m.structQueue, structWorkItem{
		module: module,
		st:     st,
	})
}

func (m *monoCtx) queueFunc(fn *t.NodeFuncDef) {
	if fn == nil || m.queuedFunc[fn] {
		return
	}
	m.queuedFunc[fn] = true
	m.funcQueue = append(m.funcQueue, fn)
}

func (m *monoCtx) queueVar(v *t.NodeExprVarDef) {
	if v == nil || m.queuedVar[v] {
		return
	}
	m.queuedVar[v] = true
	m.varQueue = append(m.varQueue, v)
}

func (m *monoCtx) getStructDefFromType(currModule string, currGl *t.NodeGlobal, tp *t.NodeType) (*t.StructDef, string, string, error) {
	if tp == nil {
		return nil, "", "", fmt.Errorf("nil type")
	}

	switch n := tp.KindNode.(type) {
	case *t.NodeTypePointer:
		return m.getStructDefFromType(currModule, currGl, &t.NodeType{KindNode: n.Kind})
	case *t.NodeTypeRfc:
		return m.getStructDefFromType(currModule, currGl, &t.NodeType{KindNode: n.Kind})
	case *t.NodeTypeAbsolute:
		module, name, e := splitAbsoluteStructName(n.AbsoluteName)
		if e != nil {
			return nil, "", "", e
		}
		gl := m.modules[module]
		if gl == nil {
			return nil, "", "", fmt.Errorf("missing module '%s'", module)
		}
		sd, ok := gl.StructDefs[name]
		if !ok {
			return nil, "", "", fmt.Errorf("missing struct '%s' in module '%s'", name, module)
		}
		return sd, module, name, nil
	case *t.NodeTypeNamed:
		switch nn := n.NameNode.(type) {
		case *t.NodeNameSingle:
			sd, ok := currGl.StructDefs[nn.Name]
			if !ok {
				return nil, "", "", fmt.Errorf("missing struct '%s' in module '%s'", nn.Name, currModule)
			}
			return sd, currModule, nn.Name, nil
		case *t.NodeNameComposite:
			if len(nn.Parts) < 2 {
				return nil, "", "", fmt.Errorf("invalid composite name")
			}
			alias := nn.Parts[0]
			name := nn.Parts[1]
			module, ok := currGl.ImportAlias[alias]
			if !ok {
				return nil, "", "", fmt.Errorf("unknown module alias '%s'", alias)
			}
			gl := m.modules[module]
			if gl == nil {
				return nil, "", "", fmt.Errorf("missing module '%s'", module)
			}
			sd, ok := gl.StructDefs[name]
			if !ok {
				return nil, "", "", fmt.Errorf("missing struct '%s' in module '%s'", name, module)
			}
			return sd, module, name, nil
		}
	}
	return nil, "", "", fmt.Errorf("type is not a struct type")
}

func (m *monoCtx) inferOwnerTypeFromCallee(currModule string, currGl *t.NodeGlobal, parts []string, env map[string]*t.NodeType) (*t.NodeType, string, string, error) {
	if len(parts) == 0 {
		return nil, "", "", fmt.Errorf("missing owner expression")
	}

	first := parts[0]
	baseType, ok := env[first]
	if !ok {
		return nil, "", "", fmt.Errorf("owner root '%s' not found in scope", first)
	}

	currType := cloneType(baseType)
	sd, sdModule, sdName, e := m.getStructDefFromType(currModule, currGl, currType)
	if e != nil {
		return nil, "", "", e
	}

	for i := 1; i < len(parts); i++ {
		fieldType, ok := sd.Fields[parts[i]]
		if !ok {
			return nil, "", "", fmt.Errorf("field '%s' does not exist on owner chain", parts[i])
		}
		currType = cloneType(fieldType)
		sd, sdModule, sdName, e = m.getStructDefFromType(currModule, currGl, currType)
		if e != nil {
			if i != len(parts)-1 {
				return nil, "", "", e
			}
		}
	}

	return currType, sdModule, sdName, nil
}

func trackExprVarDefs(expr t.NodeExpr, env map[string]*t.NodeType) {
	switch n := expr.(type) {
	case *t.NodeExprVarDef:
		if s, ok := n.Name.(*t.NodeNameSingle); ok {
			env[s.Name] = cloneType(n.Type)
		}
	case *t.NodeExprVarDefAssign:
		if s, ok := n.VarDef.Name.(*t.NodeNameSingle); ok {
			env[s.Name] = cloneType(n.VarDef.Type)
		}
	case *t.NodeExprDestructureAssign:
		if s, ok := n.ValueDef.Name.(*t.NodeNameSingle); ok {
			env[s.Name] = cloneType(n.ValueDef.Type)
		}
		if s, ok := n.ErrDef.Name.(*t.NodeNameSingle); ok {
			env[s.Name] = cloneType(n.ErrDef.Type)
		}
	}
}

func (m *monoCtx) instantiateStruct(module string, baseName string, args []*t.NodeType) (string, error) {
	templateKey := makeTemplateKey(module, baseName)
	template, ok := m.structTemplates[templateKey]
	if !ok {
		return "", fmt.Errorf("unknown generic struct template: %s", templateKey)
	}

	if len(template.Class.TypeParams) != len(args) {
		return "", fmt.Errorf("generic struct '%s' expects %d type args but got %d", baseName, len(template.Class.TypeParams), len(args))
	}

	instanceKey := makeInstanceKey(module, baseName, args)
	if n, ok := m.structInstances[instanceKey]; ok {
		return n, nil
	}

	specName := MangleSpecializedName(baseName, args)
	m.structInstances[instanceKey] = specName

	gl := m.modules[module]

	specStruct := cloneStructDef(template)
	specStruct.Class.NameNode = &t.NodeNameSingle{Name: specName}
	specStruct.Class.TypeParams = nil

	subst := map[string]*t.NodeType{}
	for i, p := range template.Class.TypeParams {
		subst[p] = cloneType(args[i])
	}

	for i := range specStruct.Class.ArgsNode.Args {
		specStruct.Class.ArgsNode.Args[i].TypeNode = substituteType(specStruct.Class.ArgsNode.Args[i].TypeNode, subst)
	}

	stDef := &t.StructDef{
		Module:     module,
		Name:       specName,
		TypeParams: nil,
		FieldNb:    map[string]int{},
		Fields:     map[string]*t.NodeType{},
		Funcs:      map[string]*t.NodeFuncDef{},
	}
	for i, fld := range specStruct.Class.ArgsNode.Args {
		stDef.FieldNb[fld.Name] = i
		stDef.Fields[fld.Name] = cloneType(fld.TypeNode)
	}

	origDef := gl.StructDefs[baseName]
	for memberName, fnTpl := range origDef.Funcs {
		if len(fnTpl.Class.TypeParams) > 0 {
			memberTpl := cloneFuncDef(fnTpl)
			memberTpl.Class.OwnerTypeParams = nil
			memberTpl.Class.NameNode = &t.NodeNameComposite{
				Parts: []string{specName, memberName},
			}
			for i := range memberTpl.Class.ArgsNode.Args {
				memberTpl.Class.ArgsNode.Args[i].TypeNode = substituteType(memberTpl.Class.ArgsNode.Args[i].TypeNode, subst)
			}
			memberTpl.ReturnType = substituteType(memberTpl.ReturnType, subst)
			for _, s := range memberTpl.Body.Statements {
				substituteStmt(s, subst)
			}
			memberTpl.AbsName = module + "." + flattenName(memberTpl.Class.NameNode)
			m.memberTemplates[makeMemberTemplateKey(module, specName, memberName)] = memberTpl
			continue
		}

		specFn := cloneFuncDef(fnTpl)
		specFn.Class.OwnerTypeParams = nil
		specFn.Class.NameNode = &t.NodeNameComposite{
			Parts: []string{specName, memberName},
		}
		for i := range specFn.Class.ArgsNode.Args {
			specFn.Class.ArgsNode.Args[i].TypeNode = substituteType(specFn.Class.ArgsNode.Args[i].TypeNode, subst)
		}
		specFn.ReturnType = substituteType(specFn.ReturnType, subst)
		for _, s := range specFn.Body.Statements {
			substituteStmt(s, subst)
		}
		specFn.AbsName = module + "." + flattenName(specFn.Class.NameNode)

		key := specName + "." + memberName
		gl.FuncDefs[key] = specFn
		gl.Declarations = append(gl.Declarations, specFn)
		stDef.Funcs[memberName] = specFn
		if specFn.IsDestructor {
			stDef.Destructors = append(stDef.Destructors, specFn)
			if stDef.Destructor == nil {
				stDef.Destructor = specFn
			}
		}
		m.queueFunc(specFn)
	}

	gl.StructDefs[specName] = stDef
	gl.Declarations = append(gl.Declarations, specStruct)
	m.queueStruct(module, specStruct)

	return specName, nil
}

func (m *monoCtx) instantiateFunc(module string, baseName string, args []*t.NodeType) (string, error) {
	templateKey := makeTemplateKey(module, baseName)
	template, ok := m.funcTemplates[templateKey]
	if !ok {
		return "", fmt.Errorf("unknown generic function template: %s", templateKey)
	}

	if len(template.Class.TypeParams) != len(args) {
		return "", fmt.Errorf("generic function '%s' expects %d type args but got %d", baseName, len(template.Class.TypeParams), len(args))
	}

	instanceKey := makeInstanceKey(module, baseName, args)
	if n, ok := m.funcInstances[instanceKey]; ok {
		return n, nil
	}

	specName := MangleSpecializedName(baseName, args)
	m.funcInstances[instanceKey] = specName

	gl := m.modules[module]
	specFn := cloneFuncDef(template)
	specFn.Class.TypeParams = nil
	specFn.Class.NameNode = &t.NodeNameSingle{Name: specName}

	subst := map[string]*t.NodeType{}
	for i, p := range template.Class.TypeParams {
		subst[p] = cloneType(args[i])
	}

	for i := range specFn.Class.ArgsNode.Args {
		specFn.Class.ArgsNode.Args[i].TypeNode = substituteType(specFn.Class.ArgsNode.Args[i].TypeNode, subst)
	}
	specFn.ReturnType = substituteType(specFn.ReturnType, subst)
	for _, s := range specFn.Body.Statements {
		substituteStmt(s, subst)
	}

	specFn.AbsName = module + "." + specName
	gl.FuncDefs[specName] = specFn
	gl.Declarations = append(gl.Declarations, specFn)
	m.queueFunc(specFn)
	return specName, nil
}

func (m *monoCtx) instantiateMemberFunc(module string, ownerName string, memberName string, args []*t.NodeType) (string, error) {
	templateKey := makeMemberTemplateKey(module, ownerName, memberName)
	template, ok := m.memberTemplates[templateKey]
	if !ok {
		return "", fmt.Errorf("unknown generic member function template: %s", templateKey)
	}

	if len(template.Class.TypeParams) != len(args) {
		return "", fmt.Errorf(
			"generic member function '%s.%s' expects %d type args but got %d",
			ownerName,
			memberName,
			len(template.Class.TypeParams),
			len(args),
		)
	}

	instanceKey := makeMemberInstanceKey(module, ownerName, memberName, args)
	if n, ok := m.memberInstances[instanceKey]; ok {
		return n, nil
	}

	specMemberName := MangleSpecializedName(memberName, args)
	m.memberInstances[instanceKey] = specMemberName

	gl := m.modules[module]
	if gl == nil {
		return "", fmt.Errorf("missing module '%s'", module)
	}

	specFn := cloneFuncDef(template)
	specFn.Class.OwnerTypeParams = nil
	specFn.Class.TypeParams = nil
	specFn.Class.NameNode = &t.NodeNameComposite{
		Parts: []string{ownerName, specMemberName},
	}

	subst := map[string]*t.NodeType{}
	for i, p := range template.Class.TypeParams {
		subst[p] = cloneType(args[i])
	}

	for i := range specFn.Class.ArgsNode.Args {
		specFn.Class.ArgsNode.Args[i].TypeNode = substituteType(specFn.Class.ArgsNode.Args[i].TypeNode, subst)
	}
	specFn.ReturnType = substituteType(specFn.ReturnType, subst)
	for _, s := range specFn.Body.Statements {
		substituteStmt(s, subst)
	}

	specFn.AbsName = module + "." + flattenName(specFn.Class.NameNode)
	gl.FuncDefs[ownerName+"."+specMemberName] = specFn
	gl.Declarations = append(gl.Declarations, specFn)
	m.queueFunc(specFn)

	stDef, ok := gl.StructDefs[ownerName]
	if !ok {
		return "", fmt.Errorf("missing owner struct definition '%s' in module '%s'", ownerName, module)
	}
	stDef.Funcs[specMemberName] = specFn

	return specMemberName, nil
}

func (m *monoCtx) rewriteType(module string, gl *t.NodeGlobal, tp *t.NodeType) error {
	if tp == nil {
		return nil
	}
	switch n := tp.KindNode.(type) {
	case *t.NodeTypeNamed:
		for _, g := range n.GenericArgs {
			if e := m.rewriteType(module, gl, g); e != nil {
				return e
			}
		}
		if len(n.GenericArgs) == 0 {
			// Concrete user types substituted into a generic retain the spelling
			// from the call site. Qualify them before the specialized declaration
			// is moved into the generic's module.
			targetModule, baseName, e := resolveQualifiedName(module, gl, n.NameNode)
			if e == nil {
				if target := m.modules[targetModule]; target != nil {
					if _, ok := target.StructDefs[baseName]; ok {
						tp.KindNode = &t.NodeTypeAbsolute{
							AbsoluteName: targetModule + "." + baseName,
						}
					}
				}
			}
			return nil
		}

		targetModule, baseName, e := resolveQualifiedName(module, gl, n.NameNode)
		if e != nil {
			return e
		}

		specName, e := m.instantiateStruct(targetModule, baseName, n.GenericArgs)
		if e != nil {
			return e
		}

		tp.KindNode = &t.NodeTypeAbsolute{
			AbsoluteName: targetModule + "." + specName,
		}
		return nil
	case *t.NodeTypePointer:
		tmp := &t.NodeType{KindNode: n.Kind}
		if e := m.rewriteType(module, gl, tmp); e != nil {
			return e
		}
		n.Kind = tmp.KindNode
		return nil
	case *t.NodeTypeRfc:
		tmp := &t.NodeType{KindNode: n.Kind}
		if e := m.rewriteType(module, gl, tmp); e != nil {
			return e
		}
		n.Kind = tmp.KindNode
		return nil
	case *t.NodeTypeSlice:
		tmp := &t.NodeType{KindNode: n.ElemKind}
		if e := m.rewriteType(module, gl, tmp); e != nil {
			return e
		}
		n.ElemKind = tmp.KindNode
		return nil
	case *t.NodeTypeFunc:
		for _, a := range n.Args {
			if e := m.rewriteType(module, gl, a); e != nil {
				return e
			}
		}
		return m.rewriteType(module, gl, n.RetType)
	}
	return nil
}

func (m *monoCtx) rewriteExpr(module string, gl *t.NodeGlobal, expr t.NodeExpr, env map[string]*t.NodeType) error {
	switch n := expr.(type) {
	case *t.NodeExprName:
		for _, g := range n.GenericArgs {
			if e := m.rewriteType(module, gl, g); e != nil {
				return e
			}
		}
		if len(n.GenericArgs) == 0 {
			return nil
		}
		targetModule, baseName, e := resolveQualifiedName(module, gl, n.Name)
		if e != nil {
			return e
		}
		specName, e := m.instantiateFunc(targetModule, baseName, n.GenericArgs)
		if e != nil {
			return e
		}
		switch name := n.Name.(type) {
		case *t.NodeNameSingle:
			n.Name = &t.NodeNameSingle{Name: specName}
		case *t.NodeNameComposite:
			parts := append([]string{}, name.Parts...)
			parts[len(parts)-1] = specName
			n.Name = &t.NodeNameComposite{Parts: parts}
		}
		n.GenericArgs = nil
		return nil
	case *t.NodeExprUnary:
		return m.rewriteExpr(module, gl, n.Operand, env)
	case *t.NodeExprMemberAccess:
		return m.rewriteExpr(module, gl, n.Target, env)
	case *t.NodeExprCall:
		if e := m.rewriteExpr(module, gl, n.Callee, env); e != nil {
			return e
		}
		for _, a := range n.Args {
			if e := m.rewriteExpr(module, gl, a, env); e != nil {
				return e
			}
		}
		for _, g := range n.GenericArgs {
			if e := m.rewriteType(module, gl, g); e != nil {
				return e
			}
		}
		if len(n.GenericArgs) == 0 {
			return nil
		}
		nameExpr, ok := n.Callee.(*t.NodeExprName)
		if !ok {
			return fmt.Errorf("generic call syntax requires a named callee")
		}
		switch nm := nameExpr.Name.(type) {
		case *t.NodeNameComposite:
			if len(nm.Parts) >= 2 {
				if _, isAlias := gl.ImportAlias[nm.Parts[0]]; !isAlias {
					memberName := nm.Parts[len(nm.Parts)-1]
					ownerParts := nm.Parts[:len(nm.Parts)-1]

					_, ownerModule, ownerSpecName, e := m.inferOwnerTypeFromCallee(module, gl, ownerParts, env)
					if e != nil {
						return e
					}

					specMemberName, e := m.instantiateMemberFunc(ownerModule, ownerSpecName, memberName, n.GenericArgs)
					if e != nil {
						return e
					}

					nextParts := make([]string, len(nm.Parts))
					copy(nextParts, nm.Parts)
					nextParts[len(nextParts)-1] = specMemberName
					nameExpr.Name = &t.NodeNameComposite{Parts: nextParts}
					n.GenericArgs = nil
					return nil
				}
			}
			targetModule, baseName, e := resolveQualifiedName(module, gl, nameExpr.Name)
			if e != nil {
				return e
			}
			specName, e := m.instantiateFunc(targetModule, baseName, n.GenericArgs)
			if e != nil {
				return e
			}
			nameExpr.Name = &t.NodeNameComposite{Parts: []string{nm.Parts[0], specName}}
		case *t.NodeNameSingle:
			targetModule, baseName, e := resolveQualifiedName(module, gl, nameExpr.Name)
			if e != nil {
				return e
			}
			specName, e := m.instantiateFunc(targetModule, baseName, n.GenericArgs)
			if e != nil {
				return e
			}
			nameExpr.Name = &t.NodeNameSingle{Name: specName}
		default:
			return fmt.Errorf("unsupported callee name shape in generic call")
		}
		n.GenericArgs = nil
		return nil

	case *t.NodeExprSubscript:
		if e := m.rewriteExpr(module, gl, n.Target, env); e != nil {
			return e
		}
		return m.rewriteExpr(module, gl, n.Expr, env)
	case *t.NodeExprBinary:
		if e := m.rewriteExpr(module, gl, n.Left, env); e != nil {
			return e
		}
		return m.rewriteExpr(module, gl, n.Right, env)
	case *t.NodeExprVarDef:
		return m.rewriteType(module, gl, n.Type)
	case *t.NodeExprVarDefAssign:
		if e := m.rewriteType(module, gl, n.VarDef.Type); e != nil {
			return e
		}
		return m.rewriteExpr(module, gl, n.AssignExpr, env)
	case *t.NodeExprAssign:
		if e := m.rewriteExpr(module, gl, n.Left, env); e != nil {
			return e
		}
		return m.rewriteExpr(module, gl, n.Right, env)
	case *t.NodeExprTry:
		return m.rewriteExpr(module, gl, n.Call, env)
	case *t.NodeExprSizeof:
		return m.rewriteType(module, gl, n.Type)
	case *t.NodeExprAddrof:
		return m.rewriteExpr(module, gl, n.Expr, env)
	case *t.NodeExprDestructureAssign:
		if e := m.rewriteType(module, gl, n.ValueDef.Type); e != nil {
			return e
		}
		if e := m.rewriteType(module, gl, n.ErrDef.Type); e != nil {
			return e
		}
		return m.rewriteExpr(module, gl, n.Call, env)
	}
	return nil
}

func (m *monoCtx) rewriteStmt(module string, gl *t.NodeGlobal, stmt t.NodeStatement, env map[string]*t.NodeType) error {
	switch n := stmt.(type) {
	case *t.NodeStmtRet:
		return m.rewriteExpr(module, gl, n.Expression, env)
	case *t.NodeStmtExpr:
		if e := m.rewriteExpr(module, gl, n.Expression, env); e != nil {
			return e
		}
		trackExprVarDefs(n.Expression, env)
		return nil
	case *t.NodeStmtThrow:
		return m.rewriteExpr(module, gl, n.Expression, env)
	case *t.NodeStmtIf:
		if e := m.rewriteExpr(module, gl, n.CondExpr, env); e != nil {
			return e
		}
		ifEnv := cloneEnv(env)
		for _, s := range n.Body.Statements {
			if e := m.rewriteStmt(module, gl, s, ifEnv); e != nil {
				return e
			}
		}
		if n.NextCondStmt != nil {
			return m.rewriteStmt(module, gl, n.NextCondStmt, cloneEnv(env))
		}
	case *t.NodeStmtElse:
		elseEnv := cloneEnv(env)
		for _, s := range n.Body.Statements {
			if e := m.rewriteStmt(module, gl, s, elseEnv); e != nil {
				return e
			}
		}
	case *t.NodeStmtWhile:
		if e := m.rewriteExpr(module, gl, n.CondExpr, env); e != nil {
			return e
		}
		loopEnv := cloneEnv(env)
		for _, s := range n.Body.Statements {
			if e := m.rewriteStmt(module, gl, s, loopEnv); e != nil {
				return e
			}
		}
	case *t.NodeStmtDefer:
		if n.IsBody {
			deferEnv := cloneEnv(env)
			for _, s := range n.Body.Statements {
				if e := m.rewriteStmt(module, gl, s, deferEnv); e != nil {
					return e
				}
			}
		} else {
			return m.rewriteExpr(module, gl, n.Expression, env)
		}
	}
	return nil
}

func isGenericStructDecl(st *t.NodeStructDef) bool {
	return st != nil && len(st.Class.TypeParams) > 0
}

func isGenericFuncDecl(fn *t.NodeFuncDef) bool {
	return fn != nil && (len(fn.Class.TypeParams) > 0 || len(fn.Class.OwnerTypeParams) > 0)
}

func (m *monoCtx) moduleForGlobal(gl *t.NodeGlobal) string {
	for module, n := range m.modules {
		if n == gl {
			return module
		}
	}
	return ""
}

func syncStructDefFields(gl *t.NodeGlobal, st *t.NodeStructDef) {
	if gl == nil || st == nil {
		return
	}

	name := flattenName(st.Class.NameNode)
	sd, ok := gl.StructDefs[name]
	if !ok {
		return
	}

	if sd.Fields == nil {
		sd.Fields = map[string]*t.NodeType{}
	}

	for _, fld := range st.Class.ArgsNode.Args {
		sd.Fields[fld.Name] = cloneType(fld.TypeNode)
	}
}

func (m *monoCtx) pruneTemplates() {
	for module, gl := range m.modules {
		filtered := make([]t.NodeGlobalDecl, 0, len(gl.Declarations))
		for _, d := range gl.Declarations {
			switch n := d.(type) {
			case *t.NodeStructDef:
				if isGenericStructDecl(n) {
					continue
				}
			case *t.NodeFuncDef:
				if isGenericFuncDecl(n) {
					continue
				}
			}
			filtered = append(filtered, d)
		}
		gl.Declarations = filtered

		for name, st := range gl.StructDefs {
			if len(st.TypeParams) > 0 {
				delete(gl.StructDefs, name)
				continue
			}

			// Generic member functions are templates too. They are registered on
			// the owning StructDef as well as in the module's declarations/function
			// maps, so pruning only the latter leaves the link checker trying to
			// resolve their unsubstituted type parameters (for example `T`).
			for memberName, fn := range st.Funcs {
				if isGenericFuncDecl(fn) {
					delete(st.Funcs, memberName)
				}
			}
		}

		for name, fn := range gl.FuncDefs {
			if isGenericFuncDecl(fn) {
				delete(gl.FuncDefs, name)
			}
		}

		_ = module
	}
}

func Run(shared *t.SharedState) error {
	ctx := &monoCtx{
		shared: shared,

		modules:         map[string]*t.NodeGlobal{},
		structTemplates: map[string]*t.NodeStructDef{},
		funcTemplates:   map[string]*t.NodeFuncDef{},
		memberTemplates: map[string]*t.NodeFuncDef{},
		structInstances: map[string]string{},
		funcInstances:   map[string]string{},
		memberInstances: map[string]string{},
		queuedStruct:    map[*t.NodeStructDef]bool{},
		queuedFunc:      map[*t.NodeFuncDef]bool{},
		queuedVar:       map[*t.NodeExprVarDef]bool{},
	}

	for _, f := range shared.Files {
		ctx.modules[f.PackageName] = f.GlNode
	}

	for module, gl := range ctx.modules {
		for name, st := range gl.StructDefs {
			if len(st.TypeParams) > 0 {
				if d, ok := func() (*t.NodeStructDef, bool) {
					for _, x := range gl.Declarations {
						if s, ok := x.(*t.NodeStructDef); ok && flattenName(s.Class.NameNode) == name {
							return s, true
						}
					}
					return nil, false
				}(); ok {
					ctx.structTemplates[makeTemplateKey(module, name)] = d
				}
			}
		}

		for name, fn := range gl.FuncDefs {
			if len(fn.Class.TypeParams) > 0 {
				ctx.funcTemplates[makeTemplateKey(module, name)] = fn
			}
		}
	}

	for module, gl := range ctx.modules {
		for _, d := range gl.Declarations {
			switch n := d.(type) {
			case *t.NodeStructDef:
				if !isGenericStructDecl(n) {
					ctx.queueStruct(module, n)
				}
			case *t.NodeFuncDef:
				if !isGenericFuncDecl(n) {
					ctx.queueFunc(n)
				}
			case *t.NodeExprVarDef:
				ctx.queueVar(n)
			}
		}
		_ = module
	}

	for len(ctx.structQueue) > 0 || len(ctx.funcQueue) > 0 || len(ctx.varQueue) > 0 {
		for len(ctx.structQueue) > 0 {
			item := ctx.structQueue[0]
			ctx.structQueue = ctx.structQueue[1:]
			module := item.module
			st := item.st
			gl := ctx.modules[module]
			if gl == nil {
				continue
			}
			for _, fld := range st.Class.ArgsNode.Args {
				if e := ctx.rewriteType(module, gl, fld.TypeNode); e != nil {
					return e
				}
			}
			syncStructDefFields(gl, st)
		}

		for len(ctx.funcQueue) > 0 {
			fn := ctx.funcQueue[0]
			ctx.funcQueue = ctx.funcQueue[1:]
			module := strings.Split(fn.AbsName, ".")[0]
			gl := ctx.modules[module]
			if gl == nil {
				continue
			}

			for _, a := range fn.Class.ArgsNode.Args {
				if e := ctx.rewriteType(module, gl, a.TypeNode); e != nil {
					return e
				}
			}
			if e := ctx.rewriteType(module, gl, fn.ReturnType); e != nil {
				return e
			}
			env := map[string]*t.NodeType{}
			for _, a := range fn.Class.ArgsNode.Args {
				env[a.Name] = cloneType(a.TypeNode)
			}
			for _, s := range fn.Body.Statements {
				if e := ctx.rewriteStmt(module, gl, s, env); e != nil {
					return e
				}
			}
		}

		for len(ctx.varQueue) > 0 {
			v := ctx.varQueue[0]
			ctx.varQueue = ctx.varQueue[1:]
			module := strings.Split(v.AbsName, ".")[0]
			gl := ctx.modules[module]
			if gl == nil {
				continue
			}
			if e := ctx.rewriteType(module, gl, v.Type); e != nil {
				return e
			}
		}
	}

	ctx.pruneTemplates()

	for _, f := range shared.Files {
		scope, e := scopeinfo.BuildScopeTree(f.GlNode)
		if e != nil {
			return e
		}
		f.ScopeTree = scope
	}

	return nil
}
