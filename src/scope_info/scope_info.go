package scopeinfo

import (
	t "Magma/src/types"
	"fmt"
	"strings"
)

type sh *t.SharedState

type lcx struct {
	GlScope   *t.Scope
	CurrScope *t.Scope
}

func declVarInStack(ctx *lcx, v *t.NodeExprVarDef) {
	// TODO: check existance
	s := ctx.CurrScope

	switch n := v.Name.(type) {
	case *t.NodeNameSingle:
		s.DeclVars[n.Name] = v
	case *t.NodeNameComposite:
		// TODO: Error
	}
}

func declFuncInStack(ctx *lcx, f *t.NodeFuncDef) *t.Scope {
	// TODO: check existance
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
		s.DeclFuncs[n.Name] = fnScope
	case *t.NodeNameComposite:
		name := strings.Join(n.Parts, ".")
		if len(name) > 2 {
			// TODO: Error
		}
		s.DeclFuncs[name] = fnScope
	}

	return newScope
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
		declVarInStack(ctx, &n.VarDef)
	case *t.NodeExprVarDef:
		declVarInStack(ctx, n)
	case *t.NodeExprDestructureAssign:
		e := bldExpr(ctx, n.Call)
		if e != nil {
			return e
		}
		declVarInStack(ctx, &n.ValueDef)
		declVarInStack(ctx, &n.ErrDef)
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

	// TODO: for nested scopes, set makeScope to true

	for _, stmt := range bdy.Statements {
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
		}
	}
	return nil
}

func bldFuncDef(ctx *lcx, fnDef *t.NodeFuncDef) error {
	for _, arg := range fnDef.Class.ArgsNode.Args {
		declVarInStack(ctx, &t.NodeExprVarDef{
			Name: &t.NodeNameSingle{Name: arg.Name},
			Type: arg.TypeNode,

			IsSsa: true,
		})
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
		s := declFuncInStack(ctx, n)
		ctx.CurrScope = s
		defer func() {
			ctx.CurrScope = s.Parent
		}()
		return bldFuncDef(ctx, n)
	case *t.NodeStructDef:
		declStructInStack(ctx, n)
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

	for _, dcl := range gl.Declarations {
		e := bldGlDecl(ctx, dcl)
		if e != nil {
			return e
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
