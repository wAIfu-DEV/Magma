package checker

import (
	"Magma/src/comp_err"
	t "Magma/src/types"
	"fmt"
)

func unionForType(c *ctx, typ *t.NodeType) *t.UnionDef {
	switch n := typ.KindNode.(type) {
	case *t.NodeTypeNamed:
		if name, ok := n.NameNode.(*t.NodeNameSingle); ok {
			return c.GlobalNode.UnionDefs[name.Name]
		}
	case *t.NodeTypeAbsolute:
		for _, global := range c.ModuleBundle.Modules {
			for _, union := range global.UnionDefs {
				if union.Module+"."+union.Name == n.AbsoluteName {
					return union
				}
			}
		}
	}
	return nil
}

func clMatch(c *ctx, stmt *t.NodeStmtMatch) error {
	if err := clExpr(c, stmt.Expression, false); err != nil {
		return err
	}
	if err := ctExpr(c, stmt.Expression); err != nil {
		return err
	}
	union := unionForType(c, stmt.Expression.GetInferredType())
	if union == nil {
		return comp_err.CompilationErrorToken(c.FileCtx, &stmt.Tk, fmt.Sprintf("cannot match non-union type '%s'", flattenType(stmt.Expression.GetInferredType())), "")
	}
	seen := map[string]bool{}
	for _, arm := range stmt.Cases {
		name, ok := arm.VariantName.(*t.NodeNameComposite)
		if !ok || len(name.Parts) < 2 || name.Parts[len(name.Parts)-2] != union.Name {
			return comp_err.CompilationErrorToken(c.FileCtx, &arm.Tk, fmt.Sprintf("case must name a variant of '%s'", union.Name), "")
		}
		if len(name.Parts) > 2 {
			moduleName, consumed, resolveErr := t.ResolveModulePrefix(c.ModuleBundle.Modules, c.GlobalNode, name.Parts)
			if resolveErr != nil || consumed != len(name.Parts)-2 || moduleName != union.Module {
				return comp_err.CompilationErrorToken(c.FileCtx, &arm.Tk, fmt.Sprintf("case must name a variant of '%s'", union.Name), "")
			}
		}
		for _, variant := range union.Variants {
			if variant.Name == name.Parts[len(name.Parts)-1] {
				arm.Variant = variant
				break
			}
		}
		if arm.Variant == nil {
			return comp_err.CompilationErrorToken(c.FileCtx, &arm.Tk, fmt.Sprintf("union '%s' has no variant '%s'", union.Name, name.Parts[len(name.Parts)-1]), "")
		}
		if seen[arm.Variant.Name] {
			return comp_err.CompilationErrorToken(c.FileCtx, &arm.Tk, fmt.Sprintf("duplicate case '%s.%s'", union.Name, arm.Variant.Name), "")
		}
		seen[arm.Variant.Name] = true
		arm.Binding.Type = &t.NodeType{KindNode: &t.NodeTypeAbsolute{AbsoluteName: union.Module + ".__union_" + union.Name + "_" + arm.Variant.Name, DisplayName: union.Name + "." + arm.Variant.Name}}
		if err := clBody(c, &arm.Body); err != nil {
			return err
		}
	}
	if stmt.ElseBody != nil {
		return clBody(c, stmt.ElseBody)
	}
	if len(seen) != len(union.Variants) {
		return comp_err.CompilationErrorToken(c.FileCtx, &stmt.Tk, "non-exhaustive match", "add the missing cases or an else arm")
	}
	return nil
}

func clDefer(c *ctx, def *t.NodeStmtDefer) error {
	if def.IsBody {
		return clBody(c, &def.Body)
	} else {
		return clExpr(c, def.Expression, false)
	}
}

func clReturn(c *ctx, ret *t.NodeStmtRet) error {
	e := clExpr(c, ret.Expression, false)
	if e != nil {
		return e
	}
	return nil
}

func clThrow(c *ctx, throw *t.NodeStmtThrow) error {
	e := clExpr(c, throw.Expression, false)
	if e != nil {
		return e
	}
	return nil
}

func clIf(c *ctx, ifStmt *t.NodeStmtIf) error {
	e := clExpr(c, ifStmt.CondExpr, false)
	if e != nil {
		return e
	}

	e = clBody(c, &ifStmt.Body)
	if e != nil {
		return e
	}

	if ifStmt.NextCondStmt != nil {
		switch n := ifStmt.NextCondStmt.(type) {
		case *t.NodeStmtIf:
			e = clIf(c, n)
		case *t.NodeStmtElse:
			e = clBody(c, &n.Body)
		}
		if e != nil {
			return e
		}
	}

	return nil
}

func clWhile(c *ctx, whileStmt *t.NodeStmtWhile) error {
	e := clExpr(c, whileStmt.CondExpr, false)
	if e != nil {
		return e
	}

	e = clBody(c, &whileStmt.Body)
	if e != nil {
		return e
	}
	return nil
}

func clFor(c *ctx, forStmt *t.NodeStmtFor) error {
	enterScope(c, forStmt.Body.Scope)
	defer leaveScope(c)

	decl, hasDecl := forStmt.DeclExpr.(*t.NodeExprVarDefAssign)
	inferredIndex := hasDecl && decl.VarDef != nil && decl.VarDef.Type == nil

	if e := clExpr(c, forStmt.DeclExpr, false); e != nil {
		return e
	}
	if e := clExpr(c, forStmt.BoundExpr, false); e != nil {
		return e
	}
	// A numeric literal has no intrinsic integer width. In an inferred for-loop
	// declaration, use the bound as its context so `for i := 0 to count():`
	// selects count's integer type instead of the literal's default i64.
	if inferredIndex {
		literal, numericLiteral := decl.AssignExpr.(*t.NodeExprLit)
		if numericLiteral && literal.LitType == t.TokLitNum {
			if e := ctExpr(c, forStmt.BoundExpr); e != nil {
				return e
			}
			if boundType := forStmt.BoundExpr.GetInferredType(); isIntegerType(boundType) {
				decl.VarDef.Type = boundType
			}
		}
	}
	body := forStmt.Body
	body.Scope = nil
	return clBody(c, &body)
}

func clBody(c *ctx, bdy *t.NodeBody) error {
	if bdy.Scope != nil {
		enterScope(c, bdy.Scope)
		defer leaveScope(c)
	}
	for _, stmt := range bdy.Statements {
		switch n := stmt.(type) {
		case *t.NodeStmtRet:
			e := clReturn(c, n)
			if e != nil {
				return e
			}
		case *t.NodeStmtExpr:
			e := clExpr(c, n.Expression, false)
			if e != nil {
				return e
			}
		case *t.NodeStmtThrow:
			e := clThrow(c, n)
			if e != nil {
				return e
			}
		case *t.NodeStmtIf:
			e := clIf(c, n)
			if e != nil {
				return e
			}
		case *t.NodeStmtMatch:
			if e := clMatch(c, n); e != nil {
				return e
			}
		case *t.NodeStmtWhile:
			e := clWhile(c, n)
			if e != nil {
				return e
			}
		case *t.NodeStmtFor:
			if e := clFor(c, n); e != nil {
				return e
			}
		case *t.NodeStmtBounded:
			for _, predicate := range n.Predicates {
				if e := clExpr(c, predicate, false); e != nil {
					return e
				}
			}
			if e := clBody(c, &n.Body); e != nil {
				return e
			}
		case *t.NodeStmtUnsafe:
			if e := clBody(c, &n.Body); e != nil {
				return e
			}
		case *t.NodeStmtDefer:
			e := clDefer(c, n)
			if e != nil {
				return e
			}
		}
	}
	return nil
}
