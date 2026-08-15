package monomorph

import (
	"fmt"

	t "Magma/src/types"
)

// inferGenericExpr performs intentionally shallow inference before the regular
// checker runs. It only uses a surrounding expected type and expression types
// which are already explicit in the lexical environment.
func (m *monoCtx) inferGenericExpr(module string, gl *t.NodeGlobal, expr t.NodeExpr, expected *t.NodeType, env map[string]*t.NodeType) error {
	if expr == nil {
		return nil
	}
	switch n := expr.(type) {
	case *t.NodeExprProtoView:
		if n.ProtoType == nil {
			if expected == nil {
				return fmt.Errorf("cannot infer prototype type for .proto(); use a typed expectation or .proto[Prototype]()")
			}
			definition, _, _, err := m.getStructDefFromType(module, gl, expected)
			if err != nil || definition == nil || !definition.IsProto {
				return fmt.Errorf(".proto() expectation '%s' is not a prototype type", m.displayType(expected))
			}
			n.ProtoType = inferenceValueType(expected)
		}
		return m.inferGenericExpr(module, gl, n.Target, nil, env)

	case *t.NodeExprCall:
		for _, arg := range n.Args {
			if err := m.inferGenericExpr(module, gl, arg, nil, env); err != nil {
				return err
			}
		}
		if len(n.GenericArgs) != 0 {
			return nil
		}
		template, err := m.genericCallTemplate(module, gl, n, env)
		if err != nil || template == nil {
			return err
		}
		bindings := map[string]*t.NodeType{}
		params := template.Class.ArgsNode.Args
		if _, member := template.Class.NameNode.(*t.NodeNameComposite); member && len(params) > 0 && params[0].Name == "this" {
			params = params[1:]
		}
		for i := 0; i < len(params) && i < len(n.Args); i++ {
			actual := m.shallowExprType(module, gl, n.Args[i], env)
			if actual != nil {
				if err := inferTypeBindings(params[i].TypeNode, actual, template.Class.TypeParams, bindings); err != nil {
					return fmt.Errorf("cannot infer generic call at line %d: %w", n.Tk.Pos.Line, err)
				}
			}
		}
		if expected != nil {
			if err := inferTypeBindings(template.ReturnType, expected, template.Class.TypeParams, bindings); err != nil {
				return fmt.Errorf("cannot infer generic call at line %d: %w", n.Tk.Pos.Line, err)
			}
		}
		inferred := make([]*t.NodeType, len(template.Class.TypeParams))
		for i, param := range template.Class.TypeParams {
			inferred[i] = bindings[param]
			if inferred[i] == nil {
				return fmt.Errorf("cannot infer generic type parameter '%s' for call at line %d; provide explicit type arguments", param, n.Tk.Pos.Line)
			}
		}
		n.GenericArgs = inferred
	}
	return nil
}

func (m *monoCtx) genericCallTemplate(module string, gl *t.NodeGlobal, call *t.NodeExprCall, env map[string]*t.NodeType) (*t.NodeFuncDef, error) {
	name, ok := call.Callee.(*t.NodeExprName)
	if !ok {
		return nil, nil
	}
	if composite, ok := name.Name.(*t.NodeNameComposite); ok && len(composite.Parts) >= 2 {
		if _, imported := gl.ImportAlias[composite.Parts[0]]; !imported {
			member := composite.Parts[len(composite.Parts)-1]
			_, ownerModule, ownerName, err := m.inferOwnerTypeFromCallee(module, gl, composite.Parts[:len(composite.Parts)-1], env)
			if err != nil {
				return nil, nil // ordinary unresolved calls are diagnosed by the checker
			}
			return m.memberTemplates[makeMemberTemplateKey(ownerModule, ownerName, member)], nil
		}
	}
	targetModule, function, err := resolveQualifiedName(m.modules, module, gl, name.Name)
	if err != nil {
		return nil, nil
	}
	return m.funcTemplates[makeTemplateKey(targetModule, function)], nil
}

func (m *monoCtx) shallowExprType(module string, gl *t.NodeGlobal, expr t.NodeExpr, env map[string]*t.NodeType) *t.NodeType {
	switch n := expr.(type) {
	case *t.NodeExprName:
		if single, ok := n.Name.(*t.NodeNameSingle); ok {
			return cloneType(env[single.Name])
		}
	case *t.NodeExprStructInit:
		return cloneType(n.Type)
	case *t.NodeExprProtoView:
		return cloneType(n.ProtoType)
	case *t.NodeExprAddrof:
		if inner := m.shallowExprType(module, gl, n.Expr, env); inner != nil {
			return &t.NodeType{KindNode: &t.NodeTypePointer{Kind: inner.KindNode}}
		}
	case *t.NodeExprCall:
		name, ok := n.Callee.(*t.NodeExprName)
		if !ok {
			return nil
		}
		targetModule, function, err := resolveQualifiedName(m.modules, module, gl, name.Name)
		if err == nil && m.modules[targetModule] != nil && m.modules[targetModule].FuncDefs[function] != nil {
			return cloneType(m.modules[targetModule].FuncDefs[function].ReturnType)
		}
	}
	return nil
}

func inferTypeBindings(pattern, actual *t.NodeType, params []string, bindings map[string]*t.NodeType) error {
	if pattern == nil || actual == nil {
		return nil
	}
	if named, ok := pattern.KindNode.(*t.NodeTypeNamed); ok && len(named.GenericArgs) == 0 {
		if single, ok := named.NameNode.(*t.NodeNameSingle); ok && containsString(params, single.Name) {
			if prior := bindings[single.Name]; prior != nil && CanonicalTypeSignature(prior) != CanonicalTypeSignature(actual) {
				return fmt.Errorf("type parameter '%s' is constrained to both '%s' and '%s'", single.Name, CanonicalTypeSignature(prior), CanonicalTypeSignature(actual))
			}
			bindings[single.Name] = inferenceValueType(actual)
			return nil
		}
	}
	switch p := pattern.KindNode.(type) {
	case *t.NodeTypePointer:
		if a, ok := actual.KindNode.(*t.NodeTypePointer); ok {
			return inferTypeBindings(&t.NodeType{KindNode: p.Kind}, &t.NodeType{KindNode: a.Kind}, params, bindings)
		}
	case *t.NodeTypeRfc:
		if a, ok := actual.KindNode.(*t.NodeTypeRfc); ok {
			return inferTypeBindings(&t.NodeType{KindNode: p.Kind}, &t.NodeType{KindNode: a.Kind}, params, bindings)
		}
	case *t.NodeTypeSlice:
		if a, ok := actual.KindNode.(*t.NodeTypeSlice); ok {
			return inferTypeBindings(&t.NodeType{KindNode: p.ElemKind}, &t.NodeType{KindNode: a.ElemKind}, params, bindings)
		}
	case *t.NodeTypeNamed:
		if a, ok := actual.KindNode.(*t.NodeTypeNamed); ok && len(p.GenericArgs) == len(a.GenericArgs) {
			for i := range p.GenericArgs {
				if err := inferTypeBindings(p.GenericArgs[i], a.GenericArgs[i], params, bindings); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// inferenceValueType removes contextual function effects from a type copied
// into an expression or generic argument. Throws belongs to the enclosing
// function signature; it is not part of the value produced on a successful
// return path.
func inferenceValueType(value *t.NodeType) *t.NodeType {
	out := cloneType(value)
	if out != nil {
		out.Throws = false
	}
	return out
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
