package monomorph

import (
	t "Magma/src/types"
	"fmt"
)

func (m *monoCtx) rewriteType(module string, gl *t.NodeGlobal, tp *t.NodeType) error {
	if tp == nil {
		return nil
	}
	switch n := tp.KindNode.(type) {
	case *t.NodeTypeNamed:
		displayName := m.displayType(tp)
		for _, g := range n.GenericArgs {
			if e := m.rewriteType(module, gl, g); e != nil {
				return e
			}
		}
		if len(n.GenericArgs) == 0 {
			// Concrete user types substituted into a generic retain the spelling
			// from the call site. Qualify them before the specialized declaration
			// is moved into the generic's module.
			targetModule, baseName, e := resolveQualifiedName(m.modules, module, gl, n.NameNode)
			if e == nil {
				if target := m.modules[targetModule]; target != nil {
					if definition, ok := target.StructDefs[baseName]; ok {
						if targetModule != module && !definition.IsPublic {
							return fmt.Errorf("struct '%s' is private and cannot be used from another module", flattenName(n.NameNode))
						}
						tp.KindNode = &t.NodeTypeAbsolute{
							AbsoluteName: targetModule + "." + baseName,
							DisplayName:  displayName,
						}
					}
				}
			}
			return nil
		}

		targetModule, baseName, e := resolveQualifiedName(m.modules, module, gl, n.NameNode)
		if e != nil {
			return e
		}
		if target := m.modules[targetModule]; targetModule != module && target != nil {
			if definition := target.StructDefs[baseName]; definition != nil && !definition.IsPublic {
				return fmt.Errorf("struct '%s' is private and cannot be used from another module", flattenName(n.NameNode))
			}
		}

		specName, e := m.instantiateStruct(targetModule, baseName, n.GenericArgs)
		if e != nil {
			return m.genericInstantiationError(gl, nameToken(n.NameNode), e)
		}

		tp.KindNode = &t.NodeTypeAbsolute{
			AbsoluteName: targetModule + "." + specName,
			DisplayName:  displayName,
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
		targetModule, baseName, e := resolveQualifiedName(m.modules, module, gl, n.Name)
		if e != nil {
			return e
		}
		if target := m.modules[targetModule]; targetModule != module && target != nil {
			if definition := target.FuncDefs[baseName]; definition != nil && !definition.IsPublic {
				return fmt.Errorf("function '%s' is private and cannot be used from another module", flattenName(n.Name))
			}
		}
		specName, e := m.instantiateFunc(targetModule, baseName, n.GenericArgs)
		if e != nil {
			return m.genericInstantiationError(gl, &n.Tk, e)
		}
		switch name := n.Name.(type) {
		case *t.NodeNameSingle:
			n.Name = &t.NodeNameSingle{Tk: name.Tk, Name: specName}
		case *t.NodeNameComposite:
			parts := append([]string{}, name.Parts...)
			parts[len(parts)-1] = specName
			tokens := append([]t.Token{}, name.Tokens...)
			n.Name = &t.NodeNameComposite{Tokens: tokens, Parts: parts}
		}
		n.GenericArgs = nil
		return nil
	case *t.NodeExprUnary:
		return m.rewriteExpr(module, gl, n.Operand, env)
	case *t.NodeExprArray:
		if e := m.rewriteType(module, gl, n.ElemType); e != nil {
			return e
		}
		if e := m.rewriteType(module, gl, n.LengthType); e != nil {
			return e
		}
		if e := m.rewriteExpr(module, gl, n.Length, env); e != nil {
			return e
		}
		for _, entry := range n.Entries {
			if entry.Index != nil {
				if e := m.rewriteExpr(module, gl, entry.Index, env); e != nil {
					return e
				}
			}
			if e := m.rewriteExpr(module, gl, entry.Value, env); e != nil {
				return e
			}
		}
		return nil
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
						return m.genericInstantiationError(gl, &n.Tk, e)
					}

					nextParts := make([]string, len(nm.Parts))
					copy(nextParts, nm.Parts)
					nextParts[len(nextParts)-1] = specMemberName
					nameExpr.Name = &t.NodeNameComposite{Tokens: append([]t.Token{}, nm.Tokens...), Parts: nextParts}
					n.GenericArgs = nil
					return nil
				}
			}
			targetModule, baseName, e := resolveQualifiedName(m.modules, module, gl, nameExpr.Name)
			if e != nil {
				return e
			}
			if target := m.modules[targetModule]; targetModule != module && target != nil {
				if definition := target.FuncDefs[baseName]; definition != nil && !definition.IsPublic {
					return fmt.Errorf("function '%s' is private and cannot be used from another module", flattenName(nameExpr.Name))
				}
			}
			specName, e := m.instantiateFunc(targetModule, baseName, n.GenericArgs)
			if e != nil {
				return m.genericInstantiationError(gl, &n.Tk, e)
			}
			tokens := append([]t.Token{}, nm.Tokens...)
			if len(tokens) > 2 {
				tokens = []t.Token{tokens[0], tokens[len(tokens)-1]}
			}
			parts := append([]string{}, nm.Parts...)
			parts[len(parts)-1] = specName
			nameExpr.Name = &t.NodeNameComposite{Tokens: tokens, Parts: parts}
		case *t.NodeNameSingle:
			targetModule, baseName, e := resolveQualifiedName(m.modules, module, gl, nameExpr.Name)
			if e != nil {
				return e
			}
			specName, e := m.instantiateFunc(targetModule, baseName, n.GenericArgs)
			if e != nil {
				return m.genericInstantiationError(gl, &n.Tk, e)
			}
			nameExpr.Name = &t.NodeNameSingle{Tk: nm.Tk, Name: specName}
		default:
			return fmt.Errorf("unsupported callee name shape in generic call")
		}
		n.GenericArgs = nil
		return nil

	case *t.NodeExprStructInit:
		if e := m.rewriteType(module, gl, n.Type); e != nil {
			return e
		}
		for i := range n.Fields {
			if e := m.rewriteExpr(module, gl, n.Fields[i].Expression, env); e != nil {
				return e
			}
		}
		return nil

	case *t.NodeExprSubscript:
		if e := m.rewriteType(module, gl, n.IndexType); e != nil {
			return e
		}
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
