package checker

import (
	t "Magma/src/types"
	"fmt"
	"sort"
)

func clFuncDef(c *ctx, fnDef *t.NodeFuncDef) error {
	var scope *t.Scope = nil

	for _, f := range c.CurrScope.DeclFuncs {
		if f.Func == fnDef {
			scope = f.Scope
		}
	}

	if scope == nil {
		return fmt.Errorf("failed to find declaration of function '%s' in scope '%s'", flattenName(fnDef.Class.NameNode), flattenName(c.CurrScope.Name))
	}

	enterScope(c, scope)
	defer leaveScope(c)

	for _, arg := range fnDef.Class.ArgsNode.Args {
		e := clTypeForUsage(c, arg.TypeNode, typeUsageValue, "a function parameter type")
		if e != nil {
			return e
		}
	}

	e := clTypeForUsage(c, fnDef.ReturnType, typeUsageReturn, "a function return type")
	if e != nil {
		return e
	}

	e = clBody(c, &fnDef.Body)
	if e != nil {
		return e
	}
	return nil
}

func clStructDef(c *ctx, stDef *t.NodeStructDef) error {
	for _, arg := range stDef.Class.ArgsNode.Args {
		e := clTypeForUsage(c, arg.TypeNode, typeUsageValue, "a struct field type")
		if e != nil {
			return e
		}
	}
	return nil
}

func clGlDecl(c *ctx, glDecl t.NodeGlobalDecl) error {
	switch n := glDecl.(type) {
	case *t.NodeFuncDef:
		return clFuncDef(c, n)
	case *t.NodeStructDef:
		return clStructDef(c, n)
	case *t.NodeExprVarDef:
		if n.Initializer != nil {
			if e := clExpr(c, n.Initializer, false); e != nil {
				return e
			}
			if n.Type == nil {
				if e := ctExpr(c, n.Initializer); e != nil {
					return e
				}
				n.Type = n.Initializer.GetInferredType()
			}
		}
		return clExpr(c, n, false)
	case *t.NodeConstDef:
		if n.VarDef.Type != nil {
			if e := clTypeForUsage(c, n.VarDef.Type, typeUsageValue, "a constant type"); e != nil {
				return e
			}
		}
		if e := clExpr(c, n.Initializer, false); e != nil {
			return e
		}
		if n.VarDef.Type == nil {
			if e := ctExpr(c, n.Initializer); e != nil {
				return e
			}
			n.VarDef.Type = n.Initializer.GetInferredType()
		}
		return nil
	}
	return nil
}

func clGlobal(c *ctx, gl *t.NodeGlobal) error {
	enterScope(c, c.ScopeTree)
	defer leaveScope(c)

	aliasNames := make([]string, 0, len(gl.TypeAliases))
	for name := range gl.TypeAliases {
		aliasNames = append(aliasNames, name)
	}
	sort.Strings(aliasNames)
	for _, name := range aliasNames {
		alias := gl.TypeAliases[name]
		key := alias.Module + "." + alias.Name
		c.AliasStack[key] = true
		resolved := cloneAliasType(alias.Target)
		e := clType(c, resolved)
		delete(c.AliasStack, key)
		if e != nil {
			return e
		}
	}

	for _, fn := range gl.FuncDefs {
		for _, arg := range fn.Class.ArgsNode.Args {
			e := clTypeForUsage(c, arg.TypeNode, typeUsageValue, "a function parameter type")
			if e != nil {
				return e
			}
		}
		e := clTypeForUsage(c, fn.ReturnType, typeUsageReturn, "a function return type")
		if e != nil {
			return e
		}
	}

	for _, st := range gl.StructDefs {
		for _, fld := range st.Fields {
			e := clTypeForUsage(c, fld, typeUsageValue, "a struct field type")
			if e != nil {
				return e
			}
		}

		for _, fn := range st.Funcs {
			for _, arg := range fn.Class.ArgsNode.Args {
				e := clTypeForUsage(c, arg.TypeNode, typeUsageValue, "a function parameter type")
				if e != nil {
					return e
				}
			}

			e := clTypeForUsage(c, fn.ReturnType, typeUsageReturn, "a function return type")
			if e != nil {
				return e
			}
		}
	}

	for _, dcl := range gl.Declarations {
		e := clGlDecl(c, dcl)
		if e != nil {
			return e
		}
	}
	return nil
}

func CheckLinks(s *t.SharedState) error {
	ctx := &ctx{
		Shared: s,
		ModuleBundle: &t.ModuleBundle{
			Modules: map[string]*t.NodeGlobal{},
		},
		PrimitiveMethods: map[string]primitiveMethod{},
		AliasStack:       map[string]bool{},
	}

	// Sorted by dependency resolution order
	// Needed for type inference on assignment
	// Otherwise link checker would have no idea of the shape of
	// inferred variables.
	// In-order link checking allows resolution of absolute names for types
	// allowing in turn early type checking needed for type inference across modules.

	pathCtxMap := map[string]*t.FileCtx{}

	for _, v := range s.Files {
		ctx.ModuleBundle.Modules[v.PackageName] = v.GlNode
		pathCtxMap[v.FilePath] = v
		for primitive, methods := range v.GlNode.PrimitiveMethods {
			for name, function := range methods {
				key := primitive + "." + name
				if _, exists := ctx.PrimitiveMethods[key]; exists {
					return fmt.Errorf("primitive method '%s' is defined more than once", key)
				}
				ctx.PrimitiveMethods[key] = primitiveMethod{Function: function, Module: v.PackageName}
			}
		}
	}

	queue := []*t.FileCtx{}
	sorted := []*t.FileCtx{}
	graph := map[*t.FileCtx][]*t.FileCtx{}
	n_deps := map[*t.FileCtx]int{}

	for _, fCtx := range s.Files {
		n := len(fCtx.Imports)
		n_deps[fCtx] = n

		//fmt.Println(fCtx.FilePath+":", n)

		if n == 0 {
			//fmt.Println("Added to queue (baseline):", fCtx.FilePath)
			queue = append(queue, fCtx)
		}

		if graph[fCtx] == nil {
			graph[fCtx] = []*t.FileCtx{}
		}

		for _, path := range fCtx.Imports {
			p := pathCtxMap[path]
			if graph[p] == nil {
				graph[p] = []*t.FileCtx{}
			}
			graph[p] = append(graph[p], fCtx)
		}
	}

	if len(queue) == 0 {
		return fmt.Errorf("found no file with 0 dependencies, this may be sign of circular dependcy.")
	}

	//fmt.Println("Resolving dependency order...")

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		sorted = append(sorted, curr)

		for _, dep := range graph[curr] {
			n_deps[dep] = n_deps[dep] - 1

			//fmt.Println(dep.FilePath+":", n_deps[dep])

			if n_deps[dep] <= 0 {
				//fmt.Println("Added to queue:", dep.FilePath)
				queue = append(queue, dep)
			}
		}
	}

	if len(sorted) != len(s.Files) {
		return fmt.Errorf("failed to produce dependency-sorted file list, this may be sign of a circular dependency.")
	}

	for _, fCtx := range sorted {
		n := fCtx.GlNode
		ctx.GlobalNode = n
		ctx.ScopeTree = &fCtx.ScopeTree
		ctx.FileCtx = fCtx

		//fmt.Printf("check links of: %s\n", fCtx.PackageName)
		e := clGlobal(ctx, n)
		if e != nil {
			return e
		}
	}

	return nil
}
