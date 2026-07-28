package checker

import (
	"Magma/src/comp_err"
	t "Magma/src/types"
	"fmt"
)

func ctFuncDef(c *ctx, fnDef *t.NodeFuncDef) error {
	c.LastFuncDef = fnDef
	previousTypeFunc := c.CurrentTypeFunc
	c.CurrentTypeFunc = fnDef
	defer func() { c.CurrentTypeFunc = previousTypeFunc }()

	e := ctBody(c, &fnDef.Body)
	if e != nil {
		return e
	}
	return nil
}

func isSimpleConstInitializer(expr t.NodeExpr) bool {
	switch n := expr.(type) {
	case *t.NodeExprLit:
		return true
	case *t.NodeExprArray:
		if _, ok := constArrayIndex(n.Length); !ok {
			return false
		}
		for _, entry := range n.Entries {
			if entry.Index != nil {
				if _, ok := constArrayIndex(entry.Index); !ok {
					return false
				}
			}
			if !isSimpleConstInitializer(entry.Value) {
				return false
			}
		}
		return true
	case *t.NodeExprName:
		switch associated := n.AssociatedNode.(type) {
		case *t.NodeFuncDef:
			return true
		case *t.NodeExprVarDef:
			return associated.IsConst && associated.Initializer != nil
		}
		return false
	case *t.NodeExprAddrof:
		name, ok := n.Expr.(*t.NodeExprName)
		if !ok {
			return false
		}
		variable, ok := name.AssociatedNode.(*t.NodeExprVarDef)
		return ok && variable.IsGlobal
	case *t.NodeExprStructInit:
		for _, field := range n.Fields {
			if !isSimpleConstInitializer(field.Expression) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func ctGlDecl(c *ctx, glDecl t.NodeGlobalDecl) error {
	switch n := glDecl.(type) {
	case *t.NodeFuncDef:
		return ctFuncDef(c, n)
	case *t.NodeExprVarDef:
		if n.Initializer != nil {
			if e := ctExpr(c, n.Initializer); e != nil {
				return e
			}
			if n.Type == nil {
				n.Type = n.Initializer.GetInferredType()
			}
			if !compatibleInitializer(n.Type, n.Initializer) {
				return comp_err.CompilationErrorToken(c.FileCtx, &t.Token{}, fmt.Sprintf("cannot initialize global '%s' of type '%s' with expression of type '%s'", flattenName(n.Name), flattenType(n.Type), flattenType(n.Initializer.GetInferredType())), "")
			}
			warnNumericConversion(c, n.Type, n.Initializer, "global initialization")
		}
		return ctExpr(c, n)
	case *t.NodeStructDef:
		return nil // TODO: check type names of arguments
	case *t.NodeConstDef:
		if !isSimpleConstInitializer(n.Initializer) {
			return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, "constant initializer must be a literal, constant value, function value, or struct constructor", "general constant expressions are not supported")
		}
		if e := ctExpr(c, n.Initializer); e != nil {
			return e
		}
		if n.VarDef.Type == nil {
			n.VarDef.Type = n.Initializer.GetInferredType()
			return nil
		}
		if !compatibleInitializer(n.VarDef.Type, n.Initializer) {
			return fmt.Errorf("constant %s expects %s but initializer has type %s", flattenName(n.VarDef.Name), flattenType(n.VarDef.Type), flattenType(n.Initializer.GetInferredType()))
		}
		warnNumericConversion(c, n.VarDef.Type, n.Initializer, "constant initialization")
		return nil
	}
	return nil
}

func ctGlobal(c *ctx, gl *t.NodeGlobal) error {
	for _, dcl := range gl.Declarations {
		e := ctGlDecl(c, dcl)
		if e != nil {
			return e
		}
	}
	return nil
}

func TypeChecker(s *t.SharedState) error {
	ctx := &ctx{
		Shared: s,
	}

	for _, fCtx := range s.Files {
		// fmt.Printf("check types of: %s\n", fCtx.PackageName)

		n := fCtx.GlNode
		ctx.GlobalNode = n
		ctx.FileCtx = fCtx
		e := ctGlobal(ctx, n)
		if e != nil {
			return e
		}
	}

	return nil
}
