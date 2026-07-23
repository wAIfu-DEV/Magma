package checker

import (
	"Magma/src/comp_err"
	magmatypes "Magma/src/magma_types"
	t "Magma/src/types"
	"fmt"
	"sort"
	"strings"
)

type sh *t.SharedState
type ctx struct {
	Shared           sh
	ScopeTree        *t.Scope
	GlobalNode       *t.NodeGlobal
	ModuleBundle     *t.ModuleBundle
	LastFuncDef      *t.NodeFuncDef
	CurrentTypeFunc  *t.NodeFuncDef
	FileCtx          *t.FileCtx
	LoopDepth        int
	PrimitiveMethods map[string]primitiveMethod

	CurrScope  *t.Scope
	AliasStack map[string]bool
}

type primitiveMethod struct {
	Function *t.NodeFuncDef
	Module   string
}

type entryType int

type parsedName struct {
	First    string
	Parts    []string
	HasParts bool
}

type privateSymbolError struct {
	kind   string
	module string
	name   string
}

func (e *privateSymbolError) Error() string {
	return fmt.Sprintf("%s '%s.%s' is private and cannot be used from another module", e.kind, e.module, e.name)
}

func privateSymbolDiagnostic(c *ctx, token *t.Token, err error) error {
	private, ok := err.(*privateSymbolError)
	if !ok {
		return err
	}
	return comp_err.CompilationErrorToken(c.FileCtx, token, private.Error(), "add 'pub' to the declaration to export it")
}

const (
	enumEntAll entryType = iota
	enumEntVar
	enumEntFunc
	enumEntStruct
	enumEntFuncAndVar
)

func enterScope(c *ctx, scope *t.Scope) {
	c.CurrScope = scope
}

func leaveScope(c *ctx) {
	if c.CurrScope.Parent != nil {
		c.CurrScope = c.CurrScope.Parent
	}
}

func flattenName(name t.NodeName) string {
	s := ""

	parsed := parseName(name)

	s += parsed.First
	if parsed.HasParts {
		for _, x := range parsed.Parts {
			s += "." + x
		}
	}
	return s
}

func parseName(name t.NodeName) parsedName {
	switch n := name.(type) {
	case *t.NodeNameSingle:
		return parsedName{
			First:    n.Name,
			HasParts: false,
		}
	case *t.NodeNameComposite:
		return parsedName{
			First:    n.Parts[0],
			HasParts: true,
			Parts:    n.Parts[1:],
		}
	}
	return parsedName{}
}

func lastNameToken(name t.NodeName) *t.Token {
	switch n := name.(type) {
	case *t.NodeNameSingle:
		return &n.Tk
	case *t.NodeNameComposite:
		if len(n.Tokens) > 0 {
			return &n.Tokens[len(n.Tokens)-1]
		}
	}
	return &t.Token{}
}

func ptrTypeFromStructDef(c *ctx, strct *t.StructDef) *t.NodeType {
	tp := typeFromStructDef(c, strct)

	return &t.NodeType{
		Throws: false,
		KindNode: &t.NodeTypePointer{
			Kind: tp.KindNode,
		},
	}
}

func typeFromStructDef(c *ctx, strct *t.StructDef) *t.NodeType {
	/*
		name := &t.NodeNameComposite{
			Parts: []string{
				strct.Module,
				strct.Name,
			},
		}

		if !strings.HasPrefix(strct.Name, strct.Module) {
			name.Parts = []string{strct.Name}
		}*/

	return &t.NodeType{
		Throws: false,
		KindNode: &t.NodeTypeAbsolute{
			AbsoluteName: strct.Module + "." + strct.Name,
		},
	}
}

func clGetStructDefFromModule(c *ctx, name parsedName) (*t.StructDef, error) {
	if !name.HasParts {
		// TODO: compiler error
		return nil, fmt.Errorf("cannot get struct def from module with simply named struct")
	}

	moduleAlias := name.First
	structName := name.Parts[0]

	// resolve alias
	moduleName, ok := c.GlobalNode.ImportAlias[moduleAlias]

	if !ok {
		// TODO: compilation error
		//fmt.Printf("alias: %s\n", moduleAlias)
		//fmt.Printf("full name: %s\n", name.First+"."+strings.Join(name.Parts, "."))
		return nil, fmt.Errorf("module alias %s does not exist in file", moduleAlias)
	}

	moduleGlNode := c.ModuleBundle.Modules[moduleName]

	// TODO: fix null pointer deref
	if moduleGlNode.StructDefs == nil {
		return nil, fmt.Errorf("struct defs map is null in module node")
	}

	structDef, ok := moduleGlNode.StructDefs[structName]

	if !ok {
		return nil, fmt.Errorf("struct %q is not defined in module %q (imported as %q)", structName, moduleName, moduleAlias)
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
		// TODO: compiler error
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
		if len(n.Parts) != 2 {
			return nil, nil, nil
		}
		moduleName, ok := c.GlobalNode.ImportAlias[n.Parts[0]]
		if !ok {
			return nil, nil, nil
		}
		owner := c.ModuleBundle.Modules[moduleName]
		if owner == nil {
			return nil, nil, nil
		}
		alias := owner.TypeAliases[n.Parts[1]]
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
		out.KindNode = &t.NodeTypeAbsolute{AbsoluteName: n.AbsoluteName}
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
		// TODO: compiler error
		return nil, fmt.Errorf("cannot get function def from module with simply named function")
	}

	moduleAlias := name.First
	fnName := name.Parts[0]

	// resolve alias
	moduleName, ok := c.GlobalNode.ImportAlias[moduleAlias]

	if !ok {
		// Might be member func
		fullName := name.First + "." + strings.Join(name.Parts, ".")
		memberFunc, ok := c.GlobalNode.FuncDefs[fullName]
		if ok {
			return memberFunc, nil
		}

		// TODO: compilation error
		//fmt.Printf("alias: %s\n", moduleAlias)
		//fmt.Printf("full name: %s\n", name.First+"."+strings.Join(name.Parts, "."))
		return nil, fmt.Errorf("module alias %s does not exist in file", moduleAlias)
	}

	moduleGlNode, ok := c.ModuleBundle.Modules[moduleName]

	if !ok {
		// TODO: Compilation error

		//fmt.Printf("module name: %s\n", moduleName)
		//fmt.Print("available modules:\n")

		//for k := range c.ModuleBundle.Modules {
		//	fmt.Printf("- %s\n", k)
		//}

		return nil, fmt.Errorf("failed to find module in module bundle")
	}

	fnDef, ok := moduleGlNode.FuncDefs[fnName]

	if !ok {
		// TODO: compilation error
		//fmt.Printf("(in other module)\n")
		//fmt.Printf("name: %s\n", fnName)
		return nil, fmt.Errorf("function name not defined in file")
	}
	if !fnDef.IsPublic {
		return nil, &privateSymbolError{kind: "function", module: moduleAlias, name: fnName}
	}

	return fnDef, nil
}

func clGetFuncDefFromThisModule(c *ctx, fnName parsedName) (*t.NodeFuncDef, error) {
	if fnName.HasParts {
		// TODO: compiler error
		return nil, fmt.Errorf("cannot get function def from this module using complex name")
	}

	fnDef, ok := c.GlobalNode.FuncDefs[fnName.First]

	if !ok {
		// TODO: compilation error
		//fmt.Printf("(in this module)\n")
		//fmt.Printf("name: %s\n", fnName.First)
		return nil, fmt.Errorf("function name not defined in file")
	}

	return fnDef, nil
}

func clGetFuncDefFromName(c *ctx, nameNode t.NodeName) (*t.NodeFuncDef, error) {
	switch nameNode.(type) {
	case *t.NodeNameComposite:
		return clGetFuncDefFromModule(c, parseName(nameNode))
	case *t.NodeNameSingle:
		return clGetFuncDefFromThisModule(c, parseName(nameNode))
	}
	return nil, fmt.Errorf("failed to get struct def from name")
}

func memberNameToken(name *t.NodeExprName, partIndex int) *t.Token {
	if composite, ok := name.Name.(*t.NodeNameComposite); ok && partIndex+1 < len(composite.Tokens) {
		return &composite.Tokens[partIndex+1]
	}
	return &name.Tk
}

func clVarNameChainValid(c *ctx, scope *t.Scope, source *t.NodeExprName, name *parsedName, varName string, varType *t.NodeType, lvalue bool) (lastIsFunc bool, accesses []*t.MemberAccess, e error) {
	e = clType(c, varType)
	if e != nil {
		return false, nil, e
	}

	//fmt.Println("VarName:", varName)
	//fmt.Println("VarType:", varType)
	//fmt.Println("ParsedName:", name.First, name.Parts)

	var lastDerefType *t.NodeType = varType

	isFromPtrType := false
	switch n := varType.KindNode.(type) {
	case *t.NodeTypePointer:
		isFromPtrType = true
		lastDerefType = &t.NodeType{
			Throws:   varType.Throws,
			KindNode: n.Kind,
		}
	}
	if primitive, ok := primitiveTypeName(lastDerefType); ok {
		if len(name.Parts) == 1 {
			if _, found := c.PrimitiveMethods[primitive+"."+name.Parts[0]]; found {
				return true, []*t.MemberAccess{}, nil
			}
		}
		return false, nil, comp_err.CompilationErrorToken(
			c.FileCtx,
			memberNameToken(source, 0),
			fmt.Sprintf("type '%s' has no member function '%s'", primitive, name.Parts[0]),
			"",
		)
	}

	// get struct def for type
	structDef, e := clGetStructDefFromType(c, lastDerefType)
	if e != nil {
		return false, nil, comp_err.CompilationErrorToken(
			c.FileCtx,
			memberNameToken(source, 0),
			fmt.Sprintf("type '%s' has no members", flattenType(lastDerefType)),
			"member access requires a struct value or pointer to a struct value",
		)
	}

	foundMemberFunc := false
	memberName := ""

	accesses = []*t.MemberAccess{}

	last := len(name.Parts) - 1
	for i, part := range name.Parts {
		if foundMemberFunc {
			return false, nil, comp_err.CompilationErrorToken(
				c.FileCtx,
				memberNameToken(source, i),
				fmt.Sprintf("cannot access member '%s' after method '%s' on type '%s'", part, memberName, structDef.Name),
				"a method must be called before accessing a value from its result",
			)
		}

		// check if member name exists in struct def
		fieldType, ok := structDef.Fields[part]

		if ok {
			e = clType(c, fieldType)
			if e != nil {
				return false, nil, e
			}

			fieldNb := structDef.FieldNb[part]

			derefFieldType := fieldType
			switch n := fieldType.KindNode.(type) {
			case *t.NodeTypePointer:
				derefFieldType = &t.NodeType{
					Throws:   fieldType.Throws,
					KindNode: n.Kind,
				}
			}

			if i == last {
				accesses = append(accesses, &t.MemberAccess{
					Type:        fieldType,
					FieldNb:     fieldNb,
					PtrDeref:    isFromPtrType,
					ResultIsPtr: isPointerType(fieldType),
				})
				return foundMemberFunc, accesses, nil
			}

			// A field can itself have a primitive type with methods supplied by
			// the implicit core module (for example, value.failure.nok()).  Such
			// a field is a complete access path; it must not be looked up as a
			// struct before resolving the final primitive method.
			if primitive, primitiveField := primitiveTypeName(derefFieldType); primitiveField {
				accesses = append(accesses, &t.MemberAccess{
					Type:        fieldType,
					FieldNb:     fieldNb,
					PtrDeref:    isFromPtrType,
					ResultIsPtr: isPointerType(fieldType),
				})
				next := name.Parts[i+1]
				if i+1 == last {
					if _, found := c.PrimitiveMethods[primitive+"."+next]; found {
						return true, accesses, nil
					}
				}
				return false, nil, comp_err.CompilationErrorToken(
					c.FileCtx,
					memberNameToken(source, i+1),
					fmt.Sprintf("type '%s' has no member function '%s'", primitive, next),
					"",
				)
			}

			structDef, e = clGetStructDefFromType(c, derefFieldType)
			if e != nil {
				// TODO: compilation error? unsure
				//fmt.Printf("from ptr type: %t\n", isFromPtrType)
				//fmt.Printf("problem type: ")
				//fieldType.Print(0)
				return false, nil, e
			}

			accesses = append(accesses, &t.MemberAccess{
				Type:        typeFromStructDef(c, structDef),
				FieldNb:     fieldNb,
				PtrDeref:    isFromPtrType,
				ResultIsPtr: isPointerType(fieldType),
			})

			isFromPtrType = false
			switch fieldType.KindNode.(type) {
			case *t.NodeTypePointer:
				isFromPtrType = true
			}
			continue
		}

		_, ok = structDef.Funcs[part]

		if ok {
			foundMemberFunc = true
			memberName = part
			continue
		}

		return false, nil, comp_err.CompilationErrorToken(
			c.FileCtx,
			memberNameToken(source, i),
			fmt.Sprintf("type '%s' has no member named '%s'", structDef.Name, part),
			"",
		)
	}

	return foundMemberFunc, accesses, nil
}

func clExistsInScope(c *ctx, scope *t.Scope, name *t.NodeExprName, ent entryType, lvalue bool) (exists bool, lastIsFunc bool, associated t.Node, isSsa bool, e error) {
	parsed := parseName(name.Name)

	switch ent {
	case enumEntAll, enumEntFuncAndVar:
		fallthrough
	case enumEntVar:
		for _, v := range scope.DeclVars {
			vName := parseName(v.Name)
			if (!parsed.HasParts && !vName.HasParts) && parsed.First == vName.First {

				e := clType(c, v.Type)
				if e != nil {
					return false, false, nil, false, e
				}
				return true, false, v, v.IsSsa, nil
			}

			if parsed.First != vName.First {
				continue
			}

			// TODO: this won't work for access to vars declared in other modules
			lastFunc, accesses, e := clVarNameChainValid(c, scope, name, &parsed, vName.First, v.Type, lvalue)

			if e != nil {
				return false, false, nil, false, e
			}

			name.MemberAccesses = accesses
			return true, lastFunc, v, v.IsSsa, nil
		}

		if ent != enumEntAll && ent != enumEntFuncAndVar {
			return false, false, nil, false, nil
		}
		fallthrough
	case enumEntFunc:
		for _, f := range scope.DeclFuncs {
			fName := parseName(f.Func.Class.NameNode)
			if (!parsed.HasParts && !fName.HasParts) && parsed.First == fName.First {
				/*fmt.Printf("from simple name: %s\n", parsed.First)
				fmt.Printf(" found func:")
				f.Func.Print(0)*/
				return true, false, f.Func, false, nil
			}

			/*
				_, e := clGetFuncDefFromName(c, f.Func.Class.NameNode)
				if e != nil {
					return false, false, nil, false, e
				}*/
			/*fmt.Printf("from name: %s\n", parsed.First)
			fmt.Printf(" found func:")
			f.Func.Print(0)*/

			lastName := parsed.First

			if parsed.HasParts {
				lastName = parsed.Parts[len(parsed.Parts)-1]
			}

			lastFuncName := fName.First

			if fName.HasParts {
				lastFuncName = fName.Parts[len(fName.Parts)-1]
			}

			if lastName != lastFuncName {
				continue
			}

			return true, false, f.Func, false, nil
		}

		fnDef, e := clGetFuncDefFromName(c, name.Name)
		if e != nil {
			if _, private := e.(*privateSymbolError); private {
				return false, false, nil, false, e
			}
			return false, false, nil, false, nil // we drop error, is that correct?
		} else if fnDef != nil {
			return true, false, fnDef, false, nil
		}

		if ent != enumEntAll {
			return false, false, nil, false, nil
		}
		fallthrough
	case enumEntStruct:
		s, e := clGetStructDefFromName(c, name.Name)
		if e == nil {
			return true, false, s, false, nil
		}
		if _, private := e.(*privateSymbolError); private {
			return false, false, nil, false, e
		}

		if ent != enumEntAll {
			break
		}
	}

	return false, false, nil, false, nil
}

func clExistsInScopeTree(c *ctx, name *t.NodeExprName, ent entryType, lvalue bool) (found bool, isLastFunc bool, expr t.Node, isSsa bool, err error) {
	currScope := c.CurrScope

	for {
		if currScope == nil {
			return false, false, nil, false, nil
		}

		found, isLastFunc, expr, isSsa, e := clExistsInScope(c, currScope, name, ent, lvalue)
		if e != nil {
			return false, false, nil, false, e
		}

		if found {
			return true, isLastFunc, expr, isSsa, nil
		}

		currScope = currScope.Parent
	}
}

func clName(c *ctx, name *t.NodeExprName, expected entryType, lvalue bool) error {
	// TODO: get associated node for easier type checking later
	found, _, expr, isSsa, err := clExistsInScopeTree(c, name, expected, lvalue)

	if err != nil {
		return privateSymbolDiagnostic(c, lastNameToken(name.Name), err)
	}

	if !found {
		description := fmt.Sprintf("unknown name '%s'", flattenName(name.Name))
		if lvalue {
			description = fmt.Sprintf("unknown variable '%s'", flattenName(name.Name))
		}
		return comp_err.CompilationErrorToken(c.FileCtx, lastNameToken(name.Name), description, "")
	}

	name.IsSsa = isSsa
	name.AssociatedNode = expr

	// fmt.Printf("name: %s\n", flattenName(name.Name))
	// fmt.Printf("associated: ")
	// name.AssociatedNode.Print(0)

	if name.AssociatedNode == nil {
		// compilation error
		return fmt.Errorf("name expression: %s does not point to any existing vars, even though there was no errors?", flattenName(name.Name))
	}
	return nil
}

func clExprCall(c *ctx, call *t.NodeExprCall) error {
	var ownerExpr t.Node = nil
	var nameExpr *t.NodeExprName = nil

	var isMemberCall = false
	var isSsaOwner = false

	switch n := call.Callee.(type) {
	case *t.NodeExprName:
		found, imc, expr, isSsa, err := clExistsInScopeTree(c, n, enumEntFuncAndVar, false)

		isMemberCall = imc

		if err != nil {
			return privateSymbolDiagnostic(c, lastNameToken(n.Name), err)
		}

		if !found {
			return comp_err.CompilationErrorToken(
				c.FileCtx,
				lastNameToken(n.Name),
				fmt.Sprintf("unknown function '%s'", flattenName(n.Name)),
				"",
			)
		}

		n.IsSsa = isSsa
		isSsaOwner = isSsa
		n.AssociatedNode = expr

		if n.AssociatedNode == nil {
			return fmt.Errorf("name expression: %s does not point to any existing vars, even though there was no errors?", flattenName(n.Name))
		}

		ownerExpr = expr
		nameExpr = n
	case *t.NodeExprMemberAccess:
		if err := clExpr(c, n.Target, false); err != nil {
			return err
		}
		if err := ctExpr(c, n.Target); err != nil {
			return err
		}

		ownerType := n.Target.GetInferredType()
		fnDef, memberOwnerType, isPointerOwner, ownerModule, err := clResolveMemberFunc(c, ownerType, n.Member)
		if err != nil {
			return err
		}

		call.IsMemberFunc = true
		call.MemberOwnerType = memberOwnerType
		call.AssociatedFnDef = fnDef
		call.MemberOwnerIsPtr = isPointerOwner
		call.MemberOwnerExpr = n.Target
		call.MemberOwnerModule = ownerModule
		n.InfType = fnDef.ReturnType
	default:
		if err := clExpr(c, n, false); err != nil {
			return err
		}
		if err := ctExpr(c, n); err != nil {
			return err
		}

		fnType := n.GetInferredType()
		if fnType == nil {
			return fmt.Errorf("cannot call expression with unknown type")
		}
		if _, ok := fnType.KindNode.(*t.NodeTypeFunc); !ok {
			return fmt.Errorf("cannot call expression of type %s", flattenType(fnType))
		}

		call.IsFuncPointer = true
		call.FuncPtrType = fnType
	}

	for _, arg := range call.Args {
		e := clExpr(c, arg, false)
		if e != nil {
			return e
		}
	}

	if nameExpr == nil {
		return nil
	}

	//fmt.Printf("call to: %s\n", flattenName(nameExpr.Name))

	switch n := ownerExpr.(type) {
	case *t.NodeExprVarDef:
		// TODO: handle func pointer
		fnType := n.Type

		if isMemberCall {
			calleeName := nameExpr.Name.(*t.NodeNameComposite)
			memberName := calleeName.Parts[len(calleeName.Parts)-1]
			ownerNameParts := calleeName.Parts[0 : len(calleeName.Parts)-1]

			ownerName := &t.NodeExprName{
				InfType:        n.Type,
				AssociatedNode: n,
				IsSsa:          isSsaOwner,
			}

			if len(ownerNameParts) == 1 {
				ownerName.Name = &t.NodeNameSingle{
					Name: ownerNameParts[0],
				}
			} else {
				ownerName.Name = &t.NodeNameComposite{
					Parts: ownerNameParts,
				}
			}

			ownerType := n.Type

			isShallowPtr := false // allow auto deref
			var shallowPtrType *t.NodeType = nil

			isPointerOwner := false

			if isPointerType(ownerType) {
				elemKind := ownerType.KindNode.(*t.NodeTypePointer).Kind
				elemType := &t.NodeType{KindNode: elemKind}
				if !isPointerType(elemType) {
					isShallowPtr = true
					shallowPtrType = elemType
				}
			}

			if len(nameExpr.MemberAccesses) > 0 {
				//fmt.Printf("from member access: ")
				last := nameExpr.MemberAccesses[len(nameExpr.MemberAccesses)-1]
				ownerType = last.Type
				isPointerOwner = last.PtrDeref
			}

			//fmt.Printf("owner is ptr deref: %t\n", isPointerOwner)
			//fmt.Printf("owner struct def: ")
			//ownerType.Print(0)

			if fn, resolvedOwner, ptrOwner, module, e := clResolveMemberFunc(c, ownerType, memberName); e == nil {
				call.IsMemberFunc = true
				call.MemberOwnerType = resolvedOwner
				if ptrOwner {
					call.MemberOwnerType = ownerType
				}
				call.AssociatedFnDef = fn
				call.MemberOwnerIsPtr = isPointerOwner || ptrOwner
				call.MemberOwnerName = ownerName
				call.MemberOwnerModule = module
				call.MemberOwnerName.MemberAccesses = nameExpr.MemberAccesses
				return nil
			}

			if isShallowPtr {
				if fn, resolvedOwner, _, module, e := clResolveMemberFunc(c, shallowPtrType, memberName); e == nil {
					call.IsMemberFunc = true
					_ = resolvedOwner
					call.MemberOwnerType = ownerType
					call.AssociatedFnDef = fn
					call.MemberOwnerIsPtr = true
					call.MemberOwnerName = ownerName
					call.MemberOwnerModule = module
					call.MemberOwnerName.MemberAccesses = nameExpr.MemberAccesses
					return nil
				}
			}

			//fmt.Printf("failed to find owner struct def\n")
		}

		if len(nameExpr.MemberAccesses) > 0 {
			fnType = nameExpr.MemberAccesses[len(nameExpr.MemberAccesses)-1].Type
		}

		//fmt.Printf("is func ptr call\n")

		call.IsFuncPointer = true
		call.FuncPtrOwner = nameExpr
		call.FuncPtrType = fnType
	case *t.NodeFuncDef:
		fnDef, e := clGetFuncDefFromName(c, call.Callee.(*t.NodeExprName).Name)
		if e != nil {
			return e
		}
		if fnDef == nil {
			return fmt.Errorf("associated function def is null")
		}

		//fmt.Printf("is func call\n")

		call.AssociatedFnDef = fnDef
	}

	return nil
}

func clExprMemberAccess(c *ctx, member *t.NodeExprMemberAccess, lvalue bool) error {
	e := clExpr(c, member.Target, false)
	if e != nil {
		return e
	}

	e = ctExpr(c, member.Target)
	if e != nil {
		return e
	}

	access, e := clResolveFieldAccess(c, member.Target.GetInferredType(), member.Member, lvalue)
	if e != nil {
		return e
	}

	member.Access = access
	member.InfType = access.Type
	return nil
}

func clExprSubscript(c *ctx, subs *t.NodeExprSubscript) error {
	if e := clExpr(c, subs.Target, true); e != nil {
		return e
	}
	if n, ok := subs.Target.(*t.NodeExprName); ok {
		subs.AssociatedNode = n.AssociatedNode
		subs.IsTargetSsa = n.IsSsa
	} else {
		subs.IsTargetSsa = true
	}

	e := clExpr(c, subs.Expr, false)
	if e != nil {
		return e
	}
	return nil
}

func clExpr(c *ctx, expr t.NodeExpr, lvalue bool) error {
	switch n := expr.(type) {
	case *t.NodeExprVoid:
		return nil
	case *t.NodeExprSizeof:
		return clType(c, n.Type)
	case *t.NodeExprArray:
		if e := clType(c, n.ElemType); e != nil {
			return e
		}
		return clExpr(c, n.Length, false)
	case *t.NodeExprAddrof:
		return clExpr(c, n.Expr, lvalue)
	case *t.NodeExprCall:
		return clExprCall(c, n)
	case *t.NodeExprStructInit:
		if e := clType(c, n.Type); e != nil {
			return e
		}
		def, e := clGetStructDefFromType(c, n.Type)
		if e != nil {
			return comp_err.CompilationErrorToken(
				c.FileCtx,
				&n.Tk,
				fmt.Sprintf("cannot construct value of non-struct type '%s'", flattenType(n.Type)),
				"struct construction requires a declared struct type",
			)
		}
		seen := map[string]bool{}
		for i := range n.Fields {
			field := &n.Fields[i]
			if seen[field.Name] {
				return comp_err.CompilationErrorToken(c.FileCtx, &field.Tk, fmt.Sprintf("duplicate field '%s' in '%s' constructor", field.Name, def.Name), "constructor fields must be unique")
			}
			seen[field.Name] = true
			fieldType, ok := def.Fields[field.Name]
			if !ok {
				return comp_err.CompilationErrorToken(c.FileCtx, &field.Tk, fmt.Sprintf("type '%s' has no field named '%s'", def.Name, field.Name), "")
			}
			field.FieldIndex = def.FieldNb[field.Name]
			field.FieldType = fieldType
			if e := clExpr(c, field.Expression, false); e != nil {
				return e
			}
		}
		for _, name := range def.FieldOrder {
			if !seen[name] {
				return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, fmt.Sprintf("missing field '%s' in '%s' constructor", name, def.Name), "all struct fields must be initialized")
			}
		}
		return nil
	case *t.NodeExprTry:
		call, ok := n.Call.(*t.NodeExprCall)
		if !ok {
			return fmt.Errorf("try requires a throwing function call")
		}
		return clExprCall(c, call)
	case *t.NodeExprSubscript:
		return clExprSubscript(c, n)
	case *t.NodeExprMemberAccess:
		return clExprMemberAccess(c, n, lvalue)
	case *t.NodeExprVarDefAssign:
		e := clExpr(c, n.AssignExpr, lvalue)
		if e != nil {
			return e
		}

		infer := false
		if n.VarDef.Type == nil {
			infer = true
			// TODO: see if better way to do it
			// used for type inference
			e = ctExpr(c, n.AssignExpr)
			if e != nil {
				return e
			}
			n.VarDef.Type = n.AssignExpr.GetInferredType()
		}

		e = clType(c, n.VarDef.Type)
		if e != nil {
			return e
		}

		if infer {
			//fmt.Println("Infered Type:")
			//n.VarDef.Type.Print(0)
		}
	case *t.NodeExprVarDef:
		e := clType(c, n.Type)
		if e != nil {
			return e
		}
	case *t.NodeExprAssign:
		e := clExpr(c, n.Left, true)
		if e != nil {
			return e
		}
		e = clExpr(c, n.Right, lvalue)
		if e != nil {
			return e
		}
	case *t.NodeExprDestructureAssign:
		if e := clExprCall(c, n.Call); e != nil {
			return e
		}
		// Inferred destructuring bindings must have concrete types before later
		// statements are linked against their scope entries.
		return ctExpr(c, n)
	case *t.NodeExprName:
		e := clName(c, n, enumEntFuncAndVar, lvalue)
		if e != nil {
			return e
		}
	case *t.NodeExprBinary:
		e := clExpr(c, n.Left, lvalue)
		if e != nil {
			return e
		}
		e = clExpr(c, n.Right, lvalue)
		if e != nil {
			return e
		}
	case *t.NodeExprUnary:
		// Unary operators consume the value of their operand.  In particular,
		// dereferencing produces an lvalue, but the pointer expression itself is
		// still evaluated as a value.
		return clExpr(c, n.Operand, false)
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
		case *t.NodeStmtWhile:
			e := clWhile(c, n)
			if e != nil {
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

func clTypeKind(c *ctx, parentType *t.NodeType, kind t.NodeTypeKind, topLevel bool) (t.NodeTypeKind, error) {
	switch n := kind.(type) {
	case *t.NodeTypeAbsolute:
		// absolute types have already been checked
		return nil, nil
	case *t.NodeTypeCompilerKnown:
		resolved, ok := c.Shared.Target.CompilerKnownTypes[n.Name]
		if !ok {
			return nil, comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, fmt.Sprintf("compiler-known type '%s' is unavailable for target '%s'", n.Name, c.Shared.Target.Triple), "")
		}
		if _, ok := magmatypes.BasicTypes[resolved]; !ok {
			return nil, fmt.Errorf("compiler-known type %q resolved to invalid Magma type %q", n.Name, resolved)
		}
		return &t.NodeTypeNamed{NameNode: &t.NodeNameSingle{Tk: n.Tk, Name: resolved}}, nil
	case *t.NodeTypeNamed:
		if alias, owner, e := clFindTypeAlias(c, n.NameNode); e != nil {
			return nil, e
		} else if alias != nil {
			key := alias.Module + "." + alias.Name
			if c.AliasStack[key] {
				return nil, comp_err.CompilationErrorToken(c.FileCtx, lastNameToken(n.NameNode), fmt.Sprintf("cyclic type alias involving '%s'", flattenName(n.NameNode)), "type aliases cannot directly or indirectly refer to themselves")
			}
			c.AliasStack[key] = true
			defer delete(c.AliasStack, key)
			resolved := cloneAliasType(alias.Target)
			previousGlobal := c.GlobalNode
			c.GlobalNode = owner
			e := clType(c, resolved)
			c.GlobalNode = previousGlobal
			if e != nil {
				return nil, e
			}
			return resolved.KindNode, nil
		}
		switch nn := n.NameNode.(type) {
		case *t.NodeNameSingle:
			_, ok := magmatypes.BasicTypes[nn.Name]
			if ok {
				return nil, nil // is intrinsic type
			}

			sd, e := clGetStructDefFromName(c, n.NameNode)

			/* DEPRECATED
			if e == nil && sd.Destructor != nil && topLevel {
				parentType.Destructor = sd.Destructor
			}*/

			if e == nil {
				return &t.NodeTypeAbsolute{
					AbsoluteName: sd.Module + "." + sd.Name,
				}, nil
			}
			return nil, comp_err.CompilationErrorToken(
				c.FileCtx,
				lastNameToken(n.NameNode),
				fmt.Sprintf("unknown type '%s'", flattenName(n.NameNode)),
				"",
			)
		case *t.NodeNameComposite:
			sd, e := clGetStructDefFromModule(c, parseName(nn))

			if e == nil && sd.Destructor != nil && topLevel {
				parentType.Destructor = sd.Destructor
			}

			if e == nil {
				return &t.NodeTypeAbsolute{
					AbsoluteName: sd.Module + "." + sd.Name,
				}, nil
			}
			if private, ok := e.(*privateSymbolError); ok {
				return nil, comp_err.CompilationErrorToken(c.FileCtx, lastNameToken(n.NameNode), private.Error(), "add 'pub' to the struct declaration to export it")
			}
			return nil, comp_err.CompilationErrorToken(
				c.FileCtx,
				lastNameToken(n.NameNode),
				fmt.Sprintf("unknown type '%s'", flattenName(n.NameNode)),
				"",
			)
		}
	case *t.NodeTypeSlice:
		newT, e := clTypeKind(c, parentType, n.ElemKind, false)
		if e != nil {
			return nil, e
		}
		if newT != nil {
			n.ElemKind = newT
		}
		return nil, e
	case *t.NodeTypePointer:
		newT, e := clTypeKind(c, parentType, n.Kind, false)
		if e != nil {
			return nil, e
		}
		if newT != nil {
			n.Kind = newT
		}
		return nil, e
	case *t.NodeTypeRfc:
		newT, e := clTypeKind(c, parentType, n.Kind, false)
		if e != nil {
			return nil, e
		}
		if newT != nil {
			n.Kind = newT
		}
		return nil, e
	case *t.NodeTypeFunc:
		for _, n2 := range n.Args {
			newT, e := clTypeKind(c, parentType, n2.KindNode, false)
			if e != nil {
				return nil, e
			}
			if newT != nil {
				n2.KindNode = newT
			}
		}
		newT, e := clTypeKind(c, parentType, n.RetType.KindNode, false)
		if e != nil {
			return nil, e
		}
		if newT != nil {
			n.RetType.KindNode = newT
		}
		return nil, e
	}

	// TODO: Compilation error
	//fmt.Print("problem type: ")
	//kind.Print(0)
	return nil, fmt.Errorf("failed to find definition for type")
}

func clType(c *ctx, typeNd *t.NodeType) error {
	if typeNd == nil {
		return nil
	}

	newT, e := clTypeKind(c, typeNd, typeNd.KindNode, true)
	if e != nil {
		return e
	}
	if newT != nil {
		typeNd.KindNode = newT
	}
	return nil
}

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
		e := clType(c, arg.TypeNode)
		if e != nil {
			return e
		}
	}

	e := clType(c, fnDef.ReturnType)
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
		e := clType(c, arg.TypeNode)
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
			if e := clType(c, n.VarDef.Type); e != nil {
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
			e := clType(c, arg.TypeNode)
			if e != nil {
				return e
			}
		}
		e := clType(c, fn.ReturnType)
		if e != nil {
			return e
		}
	}

	for _, st := range gl.StructDefs {
		for _, fld := range st.Fields {
			e := clType(c, fld)
			if e != nil {
				return e
			}
		}

		for _, fn := range st.Funcs {
			for _, arg := range fn.Class.ArgsNode.Args {
				e := clType(c, arg.TypeNode)
				if e != nil {
					return e
				}
			}

			e := clType(c, fn.ReturnType)
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
