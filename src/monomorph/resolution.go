package monomorph

import (
	t "Magma/src/types"
	"fmt"
	"strings"
)

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

func resolveQualifiedName(modules map[string]*t.NodeGlobal, module string, gl *t.NodeGlobal, name t.NodeName) (string, string, error) {
	switch n := name.(type) {
	case *t.NodeNameSingle:
		return module, n.Name, nil
	case *t.NodeNameComposite:
		if len(n.Parts) < 2 {
			return "", "", fmt.Errorf("invalid composite name")
		}
		targetModule, consumed, err := t.ResolveModulePrefix(modules, gl, n.Parts)
		if err != nil || consumed >= len(n.Parts) {
			if err != nil {
				return "", "", err
			}
			return "", "", fmt.Errorf("qualified name has no symbol")
		}
		return targetModule, n.Parts[consumed], nil
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
			module, consumed, err := t.ResolveModulePrefix(m.modules, currGl, nn.Parts)
			if err != nil || consumed >= len(nn.Parts) {
				return nil, "", "", fmt.Errorf("cannot resolve qualified type '%v'", nn.Parts)
			}
			name := nn.Parts[consumed]
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

func (m *monoCtx) trackExprVarDefs(module string, gl *t.NodeGlobal, expr t.NodeExpr, env map[string]*t.NodeType) {
	switch n := expr.(type) {
	case *t.NodeExprVarDef:
		if s, ok := n.Name.(*t.NodeNameSingle); ok && n.Type != nil {
			env[s.Name] = cloneType(n.Type)
		}
	case *t.NodeExprVarDefAssign:
		if s, ok := n.VarDef.Name.(*t.NodeNameSingle); ok {
			varType := n.VarDef.Type
			if varType == nil {
				// Inferred locals have not reached the type checker yet. Preserve
				// enough information from concrete constructors for member-method
				// specialization later in the same body.
				if init, isStructInit := n.AssignExpr.(*t.NodeExprStructInit); isStructInit {
					varType = init.Type
				}
				if call, isCall := n.AssignExpr.(*t.NodeExprCall); isCall {
					if callee, isName := call.Callee.(*t.NodeExprName); isName {
						if targetModule, functionName, err := resolveQualifiedName(m.modules, module, gl, callee.Name); err == nil {
							if target := m.modules[targetModule]; target != nil {
								if definition := target.FuncDefs[functionName]; definition != nil {
									varType = cloneType(definition.ReturnType)
									_ = m.rewriteType(targetModule, target, varType)
								}
							}
						}
					}
				}
			}
			if varType != nil {
				env[s.Name] = cloneType(varType)
			}
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
