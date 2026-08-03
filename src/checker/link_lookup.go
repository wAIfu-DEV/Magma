package checker

import (
	"Magma/src/comp_err"
	magmatypes "Magma/src/magma_types"
	t "Magma/src/types"
	"fmt"
	"strings"
)

type lookupError struct {
	kind        string
	moduleAlias string
	symbol      string
}

func (e *lookupError) Error() string {
	switch e.kind {
	case "module alias":
		return fmt.Sprintf("module alias %s does not exist in file", e.moduleAlias)
	case "function":
		return fmt.Sprintf("function %s is not defined in module %s", e.symbol, e.moduleAlias)
	case "struct":
		return fmt.Sprintf("struct %s is not defined in module %s", e.symbol, e.moduleAlias)
	default:
		return "symbol lookup failed"
	}
}

func clGetStructDefFromModule(c *ctx, name parsedName) (*t.StructDef, error) {
	if !name.HasParts {
		// This helper is only valid for qualified names.
		return nil, fmt.Errorf("cannot get struct def from module with simply named struct")
	}

	moduleAlias := name.First
	moduleName, consumed, resolveErr := resolveModuleName(c, name)
	if resolveErr != nil {
		return nil, &lookupError{kind: "module alias", moduleAlias: moduleAlias}
	}
	parts := append([]string{name.First}, name.Parts...)
	if consumed >= len(parts) {
		return nil, &lookupError{kind: "struct", moduleAlias: moduleAlias}
	}
	structName := parts[consumed]

	moduleGlNode := c.ModuleBundle.Modules[moduleName]

	// TODO: fix null pointer deref
	if moduleGlNode.StructDefs == nil {
		return nil, fmt.Errorf("struct defs map is null in module node")
	}

	structDef, ok := moduleGlNode.StructDefs[structName]

	if !ok {
		return nil, &lookupError{kind: "struct", moduleAlias: moduleAlias, symbol: structName}
	}
	if !structDef.IsPublic {
		return nil, &privateSymbolError{kind: "struct", module: moduleAlias, name: structName}
	}

	return structDef, nil
}

func clGetStructDefFromAbsolute(c *ctx, name string) (*t.StructDef, error) {
	splitName := strings.Split(name, ".")

	moduleName := splitName[0]
	structName := splitName[1]

	moduleGlNode := c.ModuleBundle.Modules[moduleName]
	structDef, ok := moduleGlNode.StructDefs[structName]

	if !ok {
		return nil, fmt.Errorf("struct %q is not defined in module %q", structName, moduleName)
	}

	return structDef, nil
}

func clGetStructDefFromThisModule(c *ctx, structName parsedName) (*t.StructDef, error) {
	if structName.HasParts {
		// This helper is only valid for unqualified names.
		return nil, fmt.Errorf("cannot get struct def from this module with complex named struct")
	}

	structDef, ok := c.GlobalNode.StructDefs[structName.First]

	if !ok {
		return nil, fmt.Errorf("struct %q is not defined in module %q", structName.First, c.FileCtx.ModuleName)
	}

	return structDef, nil
}

func clGetStructDefFromName(c *ctx, nameNode t.NodeName) (*t.StructDef, error) {
	switch nameNode.(type) {
	case *t.NodeNameComposite:
		return clGetStructDefFromModule(c, parseName(nameNode))
	case *t.NodeNameSingle:
		return clGetStructDefFromThisModule(c, parseName(nameNode))
	}
	return nil, fmt.Errorf("failed to get struct def from name")
}

func clFindTypeAlias(c *ctx, nameNode t.NodeName) (*t.TypeAlias, *t.NodeGlobal, error) {
	switch n := nameNode.(type) {
	case *t.NodeNameSingle:
		alias := c.GlobalNode.TypeAliases[n.Name]
		return alias, c.GlobalNode, nil
	case *t.NodeNameComposite:
		if len(n.Parts) < 2 {
			return nil, nil, nil
		}
		moduleName, consumed, err := t.ResolveModulePrefix(c.ModuleBundle.Modules, c.GlobalNode, n.Parts)
		if err != nil || consumed >= len(n.Parts) {
			return nil, nil, nil
		}
		owner := c.ModuleBundle.Modules[moduleName]
		if owner == nil {
			return nil, nil, nil
		}
		alias := owner.TypeAliases[n.Parts[consumed]]
		if alias == nil {
			return nil, nil, nil
		}
		if !alias.IsPublic {
			private := &privateSymbolError{kind: "type alias", module: n.Parts[0], name: alias.Name}
			return nil, nil, comp_err.CompilationErrorToken(c.FileCtx, lastNameToken(nameNode), private.Error(), "add 'pub' to the alias declaration to export it")
		}
		return alias, owner, nil
	}
	return nil, nil, nil
}

func cloneAliasType(in *t.NodeType) *t.NodeType {
	if in == nil {
		return nil
	}
	out := &t.NodeType{Throws: in.Throws, Owned: in.Owned, Destructor: in.Destructor}
	switch n := in.KindNode.(type) {
	case *t.NodeTypeNamed:
		args := make([]*t.NodeType, len(n.GenericArgs))
		for i, arg := range n.GenericArgs {
			args[i] = cloneAliasType(arg)
		}
		out.KindNode = &t.NodeTypeNamed{NameNode: n.NameNode, GenericArgs: args}
	case *t.NodeTypeAbsolute:
		out.KindNode = &t.NodeTypeAbsolute{AbsoluteName: n.AbsoluteName, DisplayName: n.DisplayName}
	case *t.NodeTypeCompilerKnown:
		out.KindNode = &t.NodeTypeCompilerKnown{Tk: n.Tk, Name: n.Name}
	case *t.NodeTypePointer:
		out.KindNode = &t.NodeTypePointer{Kind: cloneAliasType(&t.NodeType{KindNode: n.Kind}).KindNode}
	case *t.NodeTypeRfc:
		out.KindNode = &t.NodeTypeRfc{Kind: cloneAliasType(&t.NodeType{KindNode: n.Kind}).KindNode}
	case *t.NodeTypeSlice:
		out.KindNode = &t.NodeTypeSlice{ElemKind: cloneAliasType(&t.NodeType{KindNode: n.ElemKind}).KindNode}
	case *t.NodeTypeFunc:
		args := make([]*t.NodeType, len(n.Args))
		for i, arg := range n.Args {
			args[i] = cloneAliasType(arg)
		}
		out.KindNode = &t.NodeTypeFunc{Args: args, RetType: cloneAliasType(n.RetType)}
	}
	return out
}

func clGetStructDefFromType(c *ctx, typeNode *t.NodeType) (*t.StructDef, error) {
	switch n := typeNode.KindNode.(type) {
	case *t.NodeTypeNamed:
		return clGetStructDefFromName(c, n.NameNode)
	case *t.NodeTypeAbsolute:
		return clGetStructDefFromAbsolute(c, n.AbsoluteName)
	}
	return nil, fmt.Errorf("failed to get struct def from type")
}

func clDerefOne(typeNode *t.NodeType) (*t.NodeType, bool) {
	switch n := typeNode.KindNode.(type) {
	case *t.NodeTypePointer:
		return &t.NodeType{
			Throws:   typeNode.Throws,
			KindNode: n.Kind,
		}, true
	}
	return typeNode, false
}

func clResolveFieldAccess(c *ctx, ownerType *t.NodeType, member string, lvalue bool) (*t.MemberAccess, error) {
	lookupType, ptrDeref := clDerefOne(ownerType)
	structDef, e := clGetStructDefFromType(c, lookupType)
	if e != nil {
		return nil, e
	}

	fieldType, ok := structDef.Fields[member]
	if !ok {
		return nil, fmt.Errorf("type %s has no field '%s'", flattenType(ownerType), member)
	}

	e = clType(c, fieldType)
	if e != nil {
		return nil, e
	}

	return &t.MemberAccess{
		OwnerType:   ownerType,
		Type:        fieldType,
		FieldNb:     structDef.FieldNb[member],
		PtrDeref:    ptrDeref,
		ResultIsPtr: isPointerType(fieldType),
	}, nil
}

func primitiveTypeName(nodeType *t.NodeType) (string, bool) {
	if nodeType == nil {
		return "", false
	}
	// Typed slices have the same runtime representation as the type-erased
	// `slice` primitive and inherit methods declared on it.
	if _, ok := nodeType.KindNode.(*t.NodeTypeSlice); ok {
		return "slice", true
	}
	named, ok := nodeType.KindNode.(*t.NodeTypeNamed)
	if !ok {
		return "", false
	}
	single, ok := named.NameNode.(*t.NodeNameSingle)
	if !ok {
		return "", false
	}
	_, ok = magmatypes.BasicTypes[single.Name]
	return single.Name, ok
}

func clResolveMemberFunc(c *ctx, ownerType *t.NodeType, member string) (*t.NodeFuncDef, *t.NodeType, bool, string, error) {
	lookupType, ptrDeref := clDerefOne(ownerType)
	if primitive, ok := primitiveTypeName(lookupType); ok {
		method, found := c.PrimitiveMethods[primitive+"."+member]
		if !found {
			return nil, nil, false, "", fmt.Errorf("type %s has no member function '%s'", flattenType(ownerType), member)
		}
		return method.Function, lookupType, ptrDeref, method.Module, nil
	}
	structDef, e := clGetStructDefFromType(c, lookupType)
	if e != nil {
		return nil, nil, false, "", e
	}

	fnDef, ok := structDef.Funcs[member]
	if !ok {
		return nil, nil, false, "", fmt.Errorf("type %s has no member function '%s'", flattenType(ownerType), member)
	}

	return fnDef, lookupType, ptrDeref, structDef.Module, nil
}

func clGetFuncDefFromModule(c *ctx, name parsedName) (*t.NodeFuncDef, error) {
	if !name.HasParts {
		// This helper is only valid for qualified names.
		return nil, fmt.Errorf("cannot get function def from module with simply named function")
	}

	moduleAlias := name.First
	moduleName, consumed, resolveErr := resolveModuleName(c, name)
	if resolveErr != nil {
		// Might be member func
		fullName := name.First + "." + strings.Join(name.Parts, ".")
		memberFunc, ok := c.GlobalNode.FuncDefs[fullName]
		if ok {
			return memberFunc, nil
		}

		return nil, &lookupError{kind: "module alias", moduleAlias: moduleAlias}
	}
	parts := append([]string{name.First}, name.Parts...)
	if consumed >= len(parts) {
		return nil, &lookupError{kind: "function", moduleAlias: moduleAlias}
	}
	fnName := parts[consumed]

	moduleGlNode, ok := c.ModuleBundle.Modules[moduleName]

	if !ok {
		return nil, fmt.Errorf("import alias %q resolved to absent module %q", moduleAlias, moduleName)
	}

	fnDef, ok := moduleGlNode.FuncDefs[fnName]

	if !ok {
		return nil, &lookupError{kind: "function", moduleAlias: moduleAlias, symbol: fnName}
	}
	if !fnDef.IsPublic {
		return nil, &privateSymbolError{kind: "function", module: moduleAlias, name: fnName}
	}

	return fnDef, nil
}

func clGetFuncDefFromThisModule(c *ctx, fnName parsedName) (*t.NodeFuncDef, error) {
	if fnName.HasParts {
		// This helper is only valid for unqualified names.
		return nil, fmt.Errorf("cannot get function def from this module using complex name")
	}

	fnDef, ok := c.GlobalNode.FuncDefs[fnName.First]

	if !ok {
		return nil, &lookupError{kind: "function", symbol: fnName.First}
	}

	return fnDef, nil
}

func clGetFuncDefFromName(c *ctx, nameNode t.NodeName) (*t.NodeFuncDef, error) {
	var fnDef *t.NodeFuncDef
	var err error
	switch nameNode.(type) {
	case *t.NodeNameComposite:
		fnDef, err = clGetFuncDefFromModule(c, parseName(nameNode))
	case *t.NodeNameSingle:
		fnDef, err = clGetFuncDefFromThisModule(c, parseName(nameNode))
	default:
		return nil, comp_err.CompilationErrorToken(c.FileCtx, lastNameToken(nameNode), "invalid function name", "")
	}
	if err == nil {
		return fnDef, nil
	}
	if _, private := err.(*privateSymbolError); private {
		return nil, err
	}
	if lookup, ok := err.(*lookupError); ok && lookup.kind == "module alias" {
		token := lastNameToken(nameNode)
		if composite, compositeOK := nameNode.(*t.NodeNameComposite); compositeOK && len(composite.Tokens) > 0 {
			token = &composite.Tokens[0]
		}
		return nil, comp_err.CompilationErrorToken(
			c.FileCtx,
			token,
			fmt.Sprintf("unknown module alias '%s'", lookup.moduleAlias),
			"import the module or use an alias declared in this file",
		)
	}
	return nil, comp_err.CompilationErrorToken(
		c.FileCtx,
		lastNameToken(nameNode),
		fmt.Sprintf("unknown function '%s'", flattenName(nameNode)),
		"function lookup failed: "+err.Error(),
	)
}
