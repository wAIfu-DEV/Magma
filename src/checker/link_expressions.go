package checker

import (
	"Magma/src/comp_err"
	t "Magma/src/types"
	"fmt"
)

func jsonValueType(c *ctx, call *t.NodeExprCall, node *t.NodeType) bool {
	if node == nil {
		return false
	}
	targetModule, ok := c.GlobalNode.ImportAlias[call.JSONModuleAlias]
	if !ok {
		return false
	}
	canonicalName := targetModule + ".Value"
	if module := c.ModuleBundle.Modules[targetModule]; module != nil {
		if definition := module.UnionDefs["Value"]; definition != nil {
			canonicalName = definition.Module + "." + definition.Name
		}
	}
	switch kind := node.KindNode.(type) {
	case *t.NodeTypeAbsolute:
		return kind.AbsoluteName == canonicalName
	case *t.NodeTypeNamed:
		name, ok := kind.NameNode.(*t.NodeNameComposite)
		if !ok || len(name.Parts) < 2 || name.Parts[len(name.Parts)-1] != "Value" {
			return false
		}
		resolved, consumed, err := t.ResolveModulePrefix(c.ModuleBundle.Modules, c.GlobalNode, name.Parts)
		return err == nil && consumed == len(name.Parts)-1 && resolved == targetModule
	default:
		return false
	}
}

func clExprCall(c *ctx, call *t.NodeExprCall) error {
	if name, ok := call.Callee.(*t.NodeExprName); ok && len(call.Args) == 0 {
		if variant, ownerType := clResolveUnionConstructor(c, &t.NodeType{KindNode: &t.NodeTypeNamed{NameNode: name.Name}}); variant != nil {
			if len(variant.Fields) != 0 {
				return comp_err.CompilationErrorToken(c.FileCtx, &call.Tk, fmt.Sprintf("variant '%s.%s' requires fields", variant.Owner.Name, variant.Name), "use named field construction")
			}
			call.UnionVariant, call.InfType = variant, ownerType
			return nil
		}
	}
	var ownerExpr t.Node = nil
	var nameExpr *t.NodeExprName = nil

	var isMemberCall = false

	switch n := call.Callee.(type) {
	case *t.NodeExprName:
		found, imc, expr, isSsa, err := clExistsInScopeTree(c, n, enumEntFuncAndVar, false)

		isMemberCall = imc

		if err != nil {
			return privateSymbolDiagnostic(c, lastNameToken(n.Name), err)
		}

		if !found {
			hint := ""
			description := fmt.Sprintf("unknown function '%s'", flattenName(n.Name))
			if function := enclosingFunction(c); function != nil && function.IsLambda {
				description = fmt.Sprintf("captureless lambda cannot reference unknown function '%s'", flattenName(n.Name))
				hint = "lambdas cannot capture enclosing locals; pass the callable as a lambda parameter"
			}
			return comp_err.CompilationErrorToken(
				c.FileCtx,
				lastNameToken(n.Name),
				description,
				hint,
			)
		}

		if isSsa {
			n.Storage = t.VariableStorageSSA
		} else if variable, ok := expr.(*t.NodeExprVarDef); ok {
			n.Storage = variable.Storage
		} else if assignment, ok := expr.(*t.NodeExprVarDefAssign); ok && assignment.VarDef != nil {
			n.Storage = assignment.VarDef.Storage
		}
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

	if call.JSONLiteral {
		for i, arg := range call.Args {
			if err := ctExpr(c, arg); err != nil {
				return err
			}
			if jsonValueType(c, call, arg.GetInferredType()) {
				continue
			}
			typeName := flattenType(arg.GetInferredType())
			helper := ""
			throwing := false
			switch typeName {
			case "bool":
				helper = "bool"
			case "i64":
				helper = "numberInt"
			case "f64":
				helper = "numberFloat"
			case "str":
				helper = "string"
				throwing = true
			}
			if helper != "" {
				token := *expressionSourceToken(arg)
				name := &t.NodeExprName{Tk: token, Name: &t.NodeNameComposite{Parts: []string{call.JSONModuleAlias, helper}, Tokens: []t.Token{token, token}}}
				conversionArg := arg
				if shorthand, ok := arg.(*t.NodeExprMove); ok {
					conversionArg = shorthand.Expr
				}
				conversion := &t.NodeExprCall{Tk: token, Callee: name, Args: []t.NodeExpr{conversionArg}}
				var converted t.NodeExpr = conversion
				if throwing {
					converted = &t.NodeExprTry{Tk: token, Pos: token.Pos, Call: conversion}
				}
				if err := clExpr(c, converted, false); err != nil {
					return err
				}
				call.Args[i] = converted
				continue
			}
			slice, ok := arg.GetInferredType().KindNode.(*t.NodeTypeSlice)
			if !ok {
				return comp_err.CompilationErrorToken(c.FileCtx, expressionSourceToken(arg), fmt.Sprintf("JSON literals cannot encode a value of type '%s'", typeName), "supported value types are bool, i64, f64, str, json.Value, and supported typed slices")
			}
			element := &t.NodeType{KindNode: slice.ElemKind}
			helper = ""
			if jsonValueType(c, call, element) {
				helper = "sliceValue"
			}
			switch flattenType(element) {
			case "bool":
				helper = "sliceBool"
			case "i64":
				helper = "sliceInt"
			case "f64":
				helper = "sliceFloat"
			case "str":
				helper = "sliceString"
			default:
				if helper == "" {
					return comp_err.CompilationErrorToken(c.FileCtx, expressionSourceToken(arg), fmt.Sprintf("JSON literals cannot encode a slice of '%s'", flattenType(element)), "supported slice element types are bool, i64, f64, str, and json.Value")
				}
			}
			token := *expressionSourceToken(arg)
			name := &t.NodeExprName{Tk: token, Name: &t.NodeNameComposite{Parts: []string{call.JSONModuleAlias, helper}, Tokens: []t.Token{token, token}}}
			conversionArg := arg
			if shorthand, ok := arg.(*t.NodeExprMove); ok {
				conversionArg = shorthand.Expr
			}
			conversion := &t.NodeExprCall{Tk: token, Callee: name, Args: []t.NodeExpr{conversionArg}}
			attempt := &t.NodeExprTry{Tk: token, Pos: token.Pos, Call: conversion}
			if err := clExpr(c, attempt, false); err != nil {
				return err
			}
			call.Args[i] = attempt
		}
	}

	if nameExpr == nil {
		return nil
	}

	//fmt.Printf("call to: %s\n", flattenName(nameExpr.Name))

	switch n := ownerExpr.(type) {
	case *t.NodeExprVarDef:
		fnType := n.Type

		if isMemberCall {
			calleeName := nameExpr.Name.(*t.NodeNameComposite)
			memberName := calleeName.Parts[len(calleeName.Parts)-1]
			ownerNameParts := calleeName.Parts[0 : len(calleeName.Parts)-1]

			ownerName := &t.NodeExprName{
				InfType:        n.Type,
				AssociatedNode: n,
				Storage:        n.Storage,
			}
			if len(calleeName.Tokens) != 0 {
				ownerName.Tk = calleeName.Tokens[0]
			}

			if len(ownerNameParts) == 1 {
				owner := &t.NodeNameSingle{Name: ownerNameParts[0]}
				if len(calleeName.Tokens) != 0 {
					owner.Tk = calleeName.Tokens[0]
				}
				ownerName.Name = owner
			} else {
				ownerTokens := calleeName.Tokens
				if len(ownerTokens) > len(ownerNameParts) {
					ownerTokens = ownerTokens[:len(ownerNameParts)]
				}
				ownerName.Name = &t.NodeNameComposite{Parts: ownerNameParts, Tokens: ownerTokens}
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

	// Calls already resolve member functions in clExprCall. For a standalone
	// member expression, prefer the same method lookup and expose the method as
	// an unbound function value whose first argument is the implicit receiver.
	// This keeps function values pointer-sized and lets callers store the
	// receiver separately as an opaque context pointer.
	if !lvalue {
		if fnDef, _, _, _, methodErr := clResolveMemberFunc(c, member.Target.GetInferredType(), member.Member); methodErr == nil {
			member.MethodDef = fnDef
			member.InfType = makeFuncPtrTypeFromDef(fnDef)
			return nil
		}
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
		subs.IsTargetSsa = n.Storage.IsSSA()
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
		return clTypeForUsage(c, n.Type, typeUsageSizeof, "the operand of sizeof")
	case *t.NodeExprArray:
		if e := clTypeForUsage(c, n.ElemType, typeUsageValue, "an array element type"); e != nil {
			return e
		}
		if e := clExpr(c, n.Length, false); e != nil {
			return e
		}
		for _, entry := range n.Entries {
			if entry.Index != nil {
				if e := clExpr(c, entry.Index, false); e != nil {
					return e
				}
			}
			if e := clExpr(c, entry.Value, false); e != nil {
				return e
			}
		}
		return nil
	case *t.NodeExprAddrof:
		return clExpr(c, n.Expr, lvalue)
	case *t.NodeExprMove:
		return clExpr(c, n.Expr, false)
	case *t.NodeExprLlvm:
		for _, arg := range n.Args {
			if err := clExpr(c, arg, false); err != nil {
				return err
			}
		}
		// Statement operations explicitly use void as their result marker; the
		// operation registry later rejects void for value-producing operations.
		return clType(c, n.ResultType)
	case *t.NodeExprCall:
		return clExprCall(c, n)
	case *t.NodeExprStructInit:
		if variant, ownerType := clResolveUnionConstructor(c, n.Type); variant != nil {
			n.UnionVariant = variant
			n.Type = ownerType
			seen := map[string]bool{}
			for i := range n.Fields {
				field := &n.Fields[i]
				if seen[field.Name] {
					return comp_err.CompilationErrorToken(c.FileCtx, &field.Tk, fmt.Sprintf("duplicate field '%s' in '%s.%s' constructor", field.Name, variant.Owner.Name, variant.Name), "")
				}
				seen[field.Name] = true
				found := false
				for index, arg := range variant.Fields {
					if arg.Name == field.Name {
						field.FieldIndex, field.FieldType, found = index, arg.TypeNode, true
						break
					}
				}
				if !found {
					return comp_err.CompilationErrorToken(c.FileCtx, &field.Tk, fmt.Sprintf("variant '%s.%s' has no field named '%s'", variant.Owner.Name, variant.Name, field.Name), "")
				}
				if e := clExpr(c, field.Expression, false); e != nil {
					return e
				}
			}
			for _, arg := range variant.Fields {
				if !seen[arg.Name] {
					return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, fmt.Sprintf("missing field '%s' in '%s.%s' constructor", arg.Name, variant.Owner.Name, variant.Name), "")
				}
			}
			return nil
		}
		if e := clType(c, n.Type); e != nil {
			return e
		}
		def, e := clGetFieldStructDefFromType(c, n.Type)
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
	case *t.NodeExprProtoView:
		if _, ok := n.Target.(*t.NodeExprName); !ok {
			return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, "prototype views require a named, addressable implementation value", "store the implementation in a local before calling `.proto[...]()`")
		}
		if e := clExpr(c, n.Target, true); e != nil {
			return e
		}
		if e := ctExpr(c, n.Target); e != nil {
			return e
		}
		if n.ProtoType == nil {
			ownerType := n.Target.GetInferredType()
			if dereferenced, isPointer := clDerefOne(ownerType); isPointer {
				ownerType = dereferenced
			}
			owner, err := clGetStructDefFromType(c, ownerType)
			if err != nil || owner == nil || len(owner.Implements) != 1 || owner.Implements[0].Proto == nil {
				return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, "cannot infer prototype type", "specify the prototype with `.proto[Prototype]()` or `.protoBorrow[Prototype]()`")
			}
			proto := owner.Implements[0].Proto
			n.ProtoType = &t.NodeType{KindNode: &t.NodeTypeAbsolute{AbsoluteName: proto.Module + "." + proto.Name, DisplayName: proto.Name}}
		}
		if e := clType(c, n.ProtoType); e != nil {
			return e
		}
		protoStruct, e := clGetStructDefFromType(c, n.ProtoType)
		if e != nil || protoStruct == nil || !protoStruct.IsProto || protoStruct.Proto == nil {
			return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, fmt.Sprintf("type '%s' is not a prototype", t.DisplayType(n.ProtoType)), "")
		}
		ownerType := n.Target.GetInferredType()
		if dereferenced, isPointer := clDerefOne(ownerType); isPointer {
			if !n.Borrowed {
				return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, "owning `.proto()` requires an implementation value", "use `.protoBorrow()` for a pointer-backed view")
			}
			ownerType = dereferenced
			n.TargetIsPointer = true
		}
		owner, e := clGetStructDefFromType(c, ownerType)
		if e != nil || owner == nil {
			return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, "prototype views require a concrete struct implementation", "")
		}
		for _, implementation := range owner.Implements {
			if implementation.Proto == protoStruct.Proto {
				n.Implementation = implementation
				n.InfType = n.ProtoType
				return nil
			}
		}
		return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, fmt.Sprintf("type '%s' does not implement prototype '%s'", owner.Name, protoStruct.Proto.Name), "")
	case *t.NodeExprTry:
		call, ok := n.Call.(*t.NodeExprCall)
		if !ok {
			return fmt.Errorf("try requires a throwing function call")
		}
		previousBoundary := c.ErrorBoundary
		c.ErrorBoundary = 1
		err := clExprCall(c, call)
		c.ErrorBoundary = previousBoundary
		return err
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

		e = clTypeForUsage(c, n.VarDef.Type, typeUsageValue, "a variable type")
		if e != nil {
			return e
		}

		if infer {
			//fmt.Println("Infered Type:")
			//n.VarDef.Type.Print(0)
		}
	case *t.NodeExprVarDef:
		e := clTypeForUsage(c, n.Type, typeUsageValue, "a variable type")
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
		previousBoundary := c.ErrorBoundary
		c.ErrorBoundary = 2
		e := clExprCall(c, n.Call)
		c.ErrorBoundary = previousBoundary
		if e != nil {
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

func clResolveUnionConstructor(c *ctx, typ *t.NodeType) (*t.UnionVariant, *t.NodeType) {
	named, ok := typ.KindNode.(*t.NodeTypeNamed)
	if !ok {
		return nil, nil
	}
	name, ok := named.NameNode.(*t.NodeNameComposite)
	if !ok || len(name.Parts) < 2 {
		return nil, nil
	}
	parts := name.Parts
	global := c.GlobalNode
	unionIndex := 0
	if len(parts) > 2 {
		moduleName, consumed, err := t.ResolveModulePrefix(c.ModuleBundle.Modules, c.GlobalNode, parts)
		if err != nil || consumed+1 >= len(parts) {
			return nil, nil
		}
		global = c.ModuleBundle.Modules[moduleName]
		unionIndex = consumed
	}
	union := global.UnionDefs[parts[unionIndex]]
	if union == nil || unionIndex+1 != len(parts)-1 {
		return nil, nil
	}
	for _, variant := range union.Variants {
		if variant.Name == parts[unionIndex+1] {
			if global != c.GlobalNode && !union.IsPublic {
				return nil, nil
			}
			return variant, &t.NodeType{KindNode: &t.NodeTypeAbsolute{AbsoluteName: union.Module + "." + union.Name, DisplayName: union.Name}}
		}
	}
	return nil, nil
}
