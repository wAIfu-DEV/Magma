package checker

import (
	"Magma/src/comp_err"
	t "Magma/src/types"
	"fmt"
)

func memberNameToken(name *t.NodeExprName, partIndex int) *t.Token {
	return memberNameTokenAtOffset(name, partIndex, 0)
}

func memberNameTokenAtOffset(name *t.NodeExprName, partIndex, partOffset int) *t.Token {
	if composite, ok := name.Name.(*t.NodeNameComposite); ok && partIndex+partOffset+1 < len(composite.Tokens) {
		return &composite.Tokens[partIndex+partOffset+1]
	}
	return &name.Tk
}

func clVarNameChainValid(c *ctx, scope *t.Scope, source *t.NodeExprName, name *parsedName, varName string, varType *t.NodeType, lvalue bool) (lastIsFunc bool, accesses []*t.MemberAccess, e error) {
	return clVarNameChainValidAtOffset(c, scope, source, name, varName, varType, lvalue, 0)
}

func clVarNameChainValidAtOffset(c *ctx, scope *t.Scope, source *t.NodeExprName, name *parsedName, varName string, varType *t.NodeType, lvalue bool, memberTokenOffset int) (lastIsFunc bool, accesses []*t.MemberAccess, e error) {
	e = clType(c, varType)
	if e != nil {
		return false, nil, e
	}

	//fmt.Println("VarName:", varName)
	//fmt.Println("VarType:", varType)
	//fmt.Println("ParsedName:", name.First, name.Parts)

	var lastDerefType *t.NodeType = varType
	currentOwnerType := varType

	switch n := varType.KindNode.(type) {
	case *t.NodeTypePointer:
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
			memberNameTokenAtOffset(source, 0, memberTokenOffset),
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
				memberNameTokenAtOffset(source, i, memberTokenOffset),
				fmt.Sprintf("cannot access member '%s' after method '%s' on type '%s'", part, memberName, structDef.Name),
				"a method must be called before accessing a value from its result",
			)
		}

		// check if member name exists in struct def
		fieldType, ok := structDef.Fields[part]

		if ok {
			access, resolveErr := clResolveFieldAccess(c, currentOwnerType, part, lvalue)
			e = resolveErr
			if e != nil {
				return false, nil, e
			}

			derefFieldType := fieldType
			switch n := fieldType.KindNode.(type) {
			case *t.NodeTypePointer:
				derefFieldType = &t.NodeType{
					Throws:   fieldType.Throws,
					KindNode: n.Kind,
				}
			}

			if i == last {
				accesses = append(accesses, access)
				return foundMemberFunc, accesses, nil
			}

			// A field can itself have a primitive type with methods supplied by
			// the implicit core module (for example, value.failure.nok()).  Such
			// a field is a complete access path; it must not be looked up as a
			// struct before resolving the final primitive method.
			if primitive, primitiveField := primitiveTypeName(derefFieldType); primitiveField {
				accesses = append(accesses, access)
				next := name.Parts[i+1]
				if i+1 == last {
					if _, found := c.PrimitiveMethods[primitive+"."+next]; found {
						return true, accesses, nil
					}
				}
				return false, nil, comp_err.CompilationErrorToken(
					c.FileCtx,
					memberNameTokenAtOffset(source, i+1, memberTokenOffset),
					fmt.Sprintf("type '%s' has no member function '%s'", primitive, next),
					"",
				)
			}

			structDef, e = clGetStructDefFromType(c, derefFieldType)
			if e != nil {
				return false, nil, comp_err.CompilationErrorToken(
					c.FileCtx,
					memberNameTokenAtOffset(source, i+1, memberTokenOffset),
					fmt.Sprintf("cannot access member '%s' on field '%s' of type '%s'", name.Parts[i+1], part, flattenType(derefFieldType)),
					"member access requires a struct value or pointer to a struct value",
				)
			}

			accesses = append(accesses, access)

			lastDerefType = derefFieldType
			currentOwnerType = fieldType
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
			memberNameTokenAtOffset(source, i, memberTokenOffset),
			fmt.Sprintf("type '%s' has no member named '%s'", structDef.Name, part),
			"",
		)
	}

	return foundMemberFunc, accesses, nil
}

func clImportedVariable(c *ctx, source *t.NodeExprName, parsed parsedName, lvalue bool) (found bool, lastIsFunc bool, variable *t.NodeExprVarDef, accesses []*t.MemberAccess, err error) {
	if !parsed.HasParts || len(parsed.Parts) == 0 {
		return false, false, nil, nil, nil
	}

	moduleName := c.GlobalNode.ImportAlias[parsed.First]
	if moduleName == "" {
		return false, false, nil, nil, nil
	}
	module := c.ModuleBundle.Modules[moduleName]
	if module == nil {
		return false, false, nil, nil, nil
	}

	rootName := parsed.Parts[0]
	for _, declaration := range module.Declarations {
		var candidate *t.NodeExprVarDef
		switch node := declaration.(type) {
		case *t.NodeConstDef:
			candidate = node.VarDef
		case *t.NodeExprVarDef:
			candidate = node
		}
		if candidate == nil || flattenName(candidate.Name) != rootName {
			continue
		}
		if !candidate.IsPublic {
			kind := "global"
			if candidate.IsConst {
				kind = "constant"
			}
			return false, false, nil, nil, &privateSymbolError{kind: kind, module: parsed.First, name: rootName}
		}

		if len(parsed.Parts) == 1 {
			return true, false, candidate, nil, nil
		}
		members := parsedName{First: rootName, Parts: parsed.Parts[1:], HasParts: true}
		lastFunc, memberAccesses, chainErr := clVarNameChainValidAtOffset(c, c.CurrScope, source, &members, rootName, candidate.Type, lvalue, 1)
		if chainErr != nil {
			return false, false, nil, nil, chainErr
		}
		return true, lastFunc, candidate, memberAccesses, nil
	}

	return false, false, nil, nil, nil
}

func clExistsInScope(c *ctx, scope *t.Scope, name *t.NodeExprName, ent entryType, lvalue bool) (exists bool, lastIsFunc bool, associated t.Node, isSsa bool, e error) {
	parsed := parseName(name.Name)

	switch ent {
	case enumEntAll, enumEntFuncAndVar:
		fallthrough
	case enumEntVar:
		if found, lastFunc, variable, accesses, importedErr := clImportedVariable(c, name, parsed, lvalue); importedErr != nil {
			return false, false, nil, false, importedErr
		} else if found {
			name.MemberAccesses = accesses
			return true, lastFunc, variable, variable.Storage.IsSSA(), nil
		}
		for _, v := range scope.DeclVars {
			vName := parseName(v.Name)
			if (!parsed.HasParts && !vName.HasParts) && parsed.First == vName.First {

				e := clType(c, v.Type)
				if e != nil {
					return false, false, nil, false, e
				}
				return true, false, v, v.Storage.IsSSA(), nil
			}

			if parsed.First != vName.First {
				continue
			}

			lastFunc, accesses, e := clVarNameChainValid(c, scope, name, &parsed, vName.First, v.Type, lvalue)

			if e != nil {
				return false, false, nil, false, e
			}

			name.MemberAccesses = accesses
			return true, lastFunc, v, v.Storage.IsSSA(), nil
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

	if isSsa {
		name.Storage = t.VariableStorageSSA
	} else if variable, ok := expr.(*t.NodeExprVarDef); ok {
		name.Storage = variable.Storage
	} else if assignment, ok := expr.(*t.NodeExprVarDefAssign); ok && assignment.VarDef != nil {
		name.Storage = assignment.VarDef.Storage
	}
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
