package checker

import (
	"Magma/src/comp_err"
	t "Magma/src/types"
	"fmt"
)

func clExprCall(c *ctx, call *t.NodeExprCall) error {
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
			return comp_err.CompilationErrorToken(
				c.FileCtx,
				lastNameToken(n.Name),
				fmt.Sprintf("unknown function '%s'", flattenName(n.Name)),
				"",
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
