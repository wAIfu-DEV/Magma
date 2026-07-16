package scopeinfo

import (
	t "Magma/src/types"
	"fmt"
)

type sh *t.SharedState

type lcx struct {
	GlScope   *t.Scope
	CurrScope *t.Scope
}

func declVarInStack(ctx *lcx, v *t.NodeExprVarDef) error {
	n, ok := v.Name.(*t.NodeNameSingle)
	if !ok {
		return fmt.Errorf("variable declarations require a simple name")
	}
	for scope := ctx.CurrScope; scope != nil; scope = scope.Parent {
		if _, exists := scope.DeclVars[n.Name]; exists {
			return fmt.Errorf("variable '%s' is already declared in this or an enclosing scope; shadowing is not allowed", n.Name)
		}
		if _, exists := scope.DeclFuncs[n.Name]; exists {
			return fmt.Errorf("variable '%s' conflicts with a function declared in this or an enclosing scope; shadowing is not allowed", n.Name)
		}
	}
	ctx.CurrScope.DeclVars[n.Name] = v
	return nil
}

func declFuncInStack(ctx *lcx, f *t.NodeFuncDef) (*t.Scope, error) {
	s := ctx.CurrScope

	newScope := &t.Scope{
		Name:       f.Class.NameNode,
		Parent:     s,
		Associated: f,
		ReturnType: f.ReturnType,

		DeclVars:    map[string]*t.NodeExprVarDef{},
		DeclFuncs:   map[string]t.FnScope{},
		DeclStructs: map[string]*t.NodeStructDef{},
	}

	fnScope := t.FnScope{
		Func:  f,
		Scope: newScope,
	}

	switch n := f.Class.NameNode.(type) {
	case *t.NodeNameSingle:
		if _, exists := s.DeclVars[n.Name]; exists {
			return nil, fmt.Errorf("function '%s' conflicts with a variable declared in this scope; shadowing is not allowed", n.Name)
		}
		if _, exists := s.DeclFuncs[n.Name]; exists {
			return nil, fmt.Errorf("function '%s' is already declared in this scope", n.Name)
		}
		s.DeclFuncs[n.Name] = fnScope
	case *t.NodeNameComposite:
		name := ""
		for _, x := range n.Parts {
			if name != "" {
				name += "."
			}
			name += x
		}

		if _, exists := s.DeclVars[name]; exists {
			return nil, fmt.Errorf("function '%s' conflicts with a variable declared in this scope; shadowing is not allowed", name)
		}
		if _, exists := s.DeclFuncs[name]; exists {
			return nil, fmt.Errorf("function '%s' is already declared in this scope", name)
		}
		s.DeclFuncs[name] = fnScope
	}

	return newScope, nil
}

func declStructInStack(ctx *lcx, st *t.NodeStructDef) {
	// TODO: check existance
	s := ctx.CurrScope

	switch n := st.Class.NameNode.(type) {
	case *t.NodeNameSingle:
		s.DeclStructs[n.Name] = st
	case *t.NodeNameComposite:
		// TODO: Error
	}
}

func bldExpr(ctx *lcx, expr t.NodeExpr) error {
	switch n := expr.(type) {
	case *t.NodeExprVarDefAssign:
		e := bldExpr(ctx, n.AssignExpr)
		if e != nil {
			return e
		}
		return declVarInStack(ctx, n.VarDef)
	case *t.NodeExprVarDef:
		return declVarInStack(ctx, n)
	case *t.NodeExprDestructureAssign:
		e := bldExpr(ctx, n.Call)
		if e != nil {
			return e
		}
		if e := declVarInStack(ctx, &n.ValueDef); e != nil {
			return e
		}
		return declVarInStack(ctx, &n.ErrDef)
	case *t.NodeExprStructInit:
		for _, field := range n.Fields {
			if e := bldExpr(ctx, field.Expression); e != nil {
				return e
			}
		}
	}
	return nil
}

func bldReturn(ctx *lcx, ret *t.NodeStmtRet) error {
	e := bldExpr(ctx, ret.Expression)
	if e != nil {
		return e
	}
	return nil
}

func bldBody(ctx *lcx, bdy *t.NodeBody, makeScope bool) error {
	if makeScope {
		scope := &t.Scope{
			Parent:      ctx.CurrScope,
			DeclVars:    map[string]*t.NodeExprVarDef{},
			DeclFuncs:   map[string]t.FnScope{},
			DeclStructs: map[string]*t.NodeStructDef{},
		}
		bdy.Scope = scope
		ctx.CurrScope = scope
		defer func() { ctx.CurrScope = scope.Parent }()
	}

	for _, stmt := range bdy.Statements {
		//fmt.Printf("iter bldBody\n")

		switch n := stmt.(type) {
		case *t.NodeStmtRet:
			e := bldReturn(ctx, n)
			if e != nil {
				return e
			}
		case *t.NodeStmtExpr:
			e := bldExpr(ctx, n.Expression)
			if e != nil {
				return e
			}
		case *t.NodeStmtIf:
			e := bldBody(ctx, &n.Body, true)
			if e != nil {
				return e
			}

			if n.NextCondStmt != nil {
				n2 := n.NextCondStmt
				for n2 != nil {
					switch n3 := n2.(type) {
					case *t.NodeStmtIf:
						//fmt.Printf("stmt: if\n")
						e := bldBody(ctx, &n3.Body, true)
						if e != nil {
							return e
						}

						if n3.NextCondStmt != nil {
							n2 = n3.NextCondStmt
							continue
						}
						n2 = nil
					case *t.NodeStmtElse:
						//fmt.Printf("stmt: else\n")
						e := bldBody(ctx, &n3.Body, true)
						if e != nil {
							return e
						}
						n2 = nil
					default:
						n2 = nil
					}
				}
			}

		case *t.NodeStmtWhile:
			e := bldBody(ctx, &n.Body, true)
			if e != nil {
				return e
			}
		}
	}
	return nil
}

func bldFuncDef(ctx *lcx, fnDef *t.NodeFuncDef) error {
	_, isMemberFunc := fnDef.Class.NameNode.(*t.NodeNameComposite)
	for i, arg := range fnDef.Class.ArgsNode.Args {
		definition := &t.NodeExprVarDef{
			Name:   &t.NodeNameSingle{Name: arg.Name},
			Type:   arg.TypeNode,
			IrName: "%" + arg.Name + ".addr",
		}
		if isMemberFunc && i == 0 {
			definition.IsSsa = true
			definition.IrName = ""
		}
		e := declVarInStack(ctx, definition)
		if e != nil {
			return e
		}
	}

	e := bldBody(ctx, &fnDef.Body, false)
	if e != nil {
		return e
	}
	return nil
}

func bldGlDecl(ctx *lcx, glDecl t.NodeGlobalDecl) error {
	switch n := glDecl.(type) {
	case *t.NodeFuncDef:
		s, e := declFuncInStack(ctx, n)
		if e != nil {
			return e
		}
		ctx.CurrScope = s
		defer func() {
			ctx.CurrScope = s.Parent
		}()
		return bldFuncDef(ctx, n)
	case *t.NodeStructDef:
		declStructInStack(ctx, n)
	case *t.NodeExprVarDef:
		return declVarInStack(ctx, n)
	case *t.NodeConstDef:
		if e := declVarInStack(ctx, n.VarDef); e != nil {
			return e
		}
		return bldExpr(ctx, n.Initializer)
	}
	return nil
}

func bldGlobal(ctx *lcx, gl *t.NodeGlobal) error {
	glScope := &t.Scope{
		Name:       &t.NodeNameSingle{Name: "global"},
		Parent:     nil,
		Associated: nil,
		ReturnType: nil,

		DeclVars:    map[string]*t.NodeExprVarDef{},
		DeclFuncs:   map[string]t.FnScope{},
		DeclStructs: map[string]*t.NodeStructDef{},
	}
	ctx.GlScope = glScope
	ctx.CurrScope = glScope

	// Reserve the complete module namespace before building function bodies so
	// no-shadowing checks are independent of declaration order.
	for _, dcl := range gl.Declarations {
		switch n := dcl.(type) {
		case *t.NodeFuncDef:
			if _, e := declFuncInStack(ctx, n); e != nil {
				return e
			}
		case *t.NodeStructDef:
			declStructInStack(ctx, n)
		case *t.NodeExprVarDef:
			if e := declVarInStack(ctx, n); e != nil {
				return e
			}
		case *t.NodeConstDef:
			if e := declVarInStack(ctx, n.VarDef); e != nil {
				return e
			}
		}
	}

	for _, dcl := range gl.Declarations {
		switch n := dcl.(type) {
		case *t.NodeFuncDef:
			var name string
			switch fnName := n.Class.NameNode.(type) {
			case *t.NodeNameSingle:
				name = fnName.Name
			case *t.NodeNameComposite:
				for i, part := range fnName.Parts {
					if i != 0 {
						name += "."
					}
					name += part
				}
			}
			fnScope := glScope.DeclFuncs[name]
			ctx.CurrScope = fnScope.Scope
			if e := bldFuncDef(ctx, n); e != nil {
				return e
			}
			ctx.CurrScope = glScope
		case *t.NodeConstDef:
			if e := bldExpr(ctx, n.Initializer); e != nil {
				return e
			}
		}
	}
	return nil
}

func BuildScopeTree(gl *t.NodeGlobal) (t.Scope, error) {
	ctx := &lcx{}

	e := bldGlobal(ctx, gl)
	return *ctx.GlScope, e
}

func PrintScopeTree(s *t.Scope, indent int) {
	t.PrintIndent(indent)
	fmt.Print("Scope: ")

	if s.Name != nil {
		s.Name.Print(0)
	} else {
		fmt.Println("nil")
	}

	childScopes := []*t.Scope{}

	if len(s.DeclVars) > 0 {
		t.PrintIndent(indent + 1)
		fmt.Println("Vars:")
		for k, _ := range s.DeclVars {
			t.PrintIndent(indent + 2)
			fmt.Println(k)
		}
	}

	if len(s.DeclFuncs) > 0 {
		t.PrintIndent(indent + 1)
		fmt.Println("Funcs:")
		for k, v := range s.DeclFuncs {
			t.PrintIndent(indent + 2)
			fmt.Println(k)

			childScopes = append(childScopes, v.Scope)
		}
	}

	if len(s.DeclStructs) > 0 {
		t.PrintIndent(indent + 1)
		fmt.Println("Structs:")
		for k, _ := range s.DeclStructs {
			t.PrintIndent(indent + 2)
			fmt.Println(k)
		}
	}

	if len(childScopes) > 0 {
		t.PrintIndent(indent + 1)
		fmt.Println("Children Scopes:")
		for _, c := range childScopes {
			PrintScopeTree(c, indent+2)
		}
	}
}
