package checker

import (
	magmatypes "Magma/src/magma_types"
	t "Magma/src/types"
	"fmt"
	"strings"
)

type sh *t.SharedState
type ctx struct {
	Shared       sh
	ScopeTree    *t.Scope
	GlobalNode   *t.NodeGlobal
	ModuleBundle *t.ModuleBundle
	LastFuncDef  *t.NodeFuncDef

	CurrScope *t.Scope
}

type entryType int

type parsedName struct {
	First    string
	Parts    []string
	HasParts bool
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
		return nil, fmt.Errorf("module alias does not exist in file")
	}

	moduleGlNode := c.ModuleBundle.Modules[moduleName]
	structDef, ok := moduleGlNode.StructDefs[structName]

	if !ok {
		// TODO: compilation error
		fmt.Printf("struct name: %s\n", structName)
		return nil, fmt.Errorf("struct name not defined in file")
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
		// TODO: compilation error
		fmt.Printf("struct name: %s\n", structName)
		return nil, fmt.Errorf("struct name not defined in file")
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
		// TODO: compilation error
		fmt.Printf("struct name: %s\n", structName.First)
		return nil, fmt.Errorf("struct name not defined in file")
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

func clGetStructDefFromType(c *ctx, typeNode *t.NodeType) (*t.StructDef, error) {
	switch n := typeNode.KindNode.(type) {
	case *t.NodeTypeNamed:
		return clGetStructDefFromName(c, n.NameNode)
	case *t.NodeTypeAbsolute:
		return clGetStructDefFromAbsolute(c, n.AbsoluteName)
	}
	return nil, fmt.Errorf("failed to get struct def from type")
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
		fmt.Printf("alias: %s\n", moduleAlias)
		fmt.Printf("full name: %s\n", name.First+"."+strings.Join(name.Parts, "."))
		return nil, fmt.Errorf("module alias does not exist in file")
	}

	moduleGlNode, ok := c.ModuleBundle.Modules[moduleName]

	if !ok {
		fmt.Printf("module name: %s\n", moduleName)
		fmt.Print("available modules:\n")

		for k := range c.ModuleBundle.Modules {
			fmt.Printf("- %s\n", k)
		}

		return nil, fmt.Errorf("failed to find module in module bundle")
	}

	fnDef, ok := moduleGlNode.FuncDefs[fnName]

	if !ok {
		// TODO: compilation error
		fmt.Printf("(in other module)\n")
		fmt.Printf("name: %s\n", fnName)
		return nil, fmt.Errorf("function name not defined in file")
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
		fmt.Printf("(in this module)\n")
		fmt.Printf("name: %s\n", fnName.First)
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

func clVarNameChainValid(c *ctx, scope *t.Scope, name *parsedName, varName string, varType *t.NodeType) (lastIsFunc bool, accesses []*t.MemberAccess, e error) {
	e = clType(c, varType)
	if e != nil {
		return false, nil, e
	}

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

	// get struct def for type
	structDef, e := clGetStructDefFromType(c, lastDerefType)
	if e != nil {
		return false, nil, e
	}

	foundMemberFunc := false
	memberName := ""

	accesses = []*t.MemberAccess{}

	last := len(name.Parts) - 1
	for i, part := range name.Parts {
		if foundMemberFunc {
			// TODO: compiler error
			return false, nil, fmt.Errorf("tried to access inexistent field '%s' from member function '%s' of '%s'", part, memberName, structDef.Name)
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
					Type:     derefFieldType,
					FieldNb:  fieldNb,
					PtrDeref: isFromPtrType,
				})
				return foundMemberFunc, accesses, nil
			}

			structDef, e = clGetStructDefFromType(c, derefFieldType)
			if e != nil {
				fmt.Printf("from ptr type: %t\n", isFromPtrType)
				fmt.Printf("problem type: ")
				fieldType.Print(0)
				return false, nil, e
			}

			accesses = append(accesses, &t.MemberAccess{
				Type:     typeFromStructDef(c, structDef),
				FieldNb:  fieldNb,
				PtrDeref: isFromPtrType,
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
			continue
		}
	}

	return foundMemberFunc, accesses, nil
}

func clExistsInScope(c *ctx, scope *t.Scope, name *t.NodeExprName, ent entryType) (exists bool, lastIsFunc bool, associated t.Node, isSsa bool, e error) {
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
			lastFunc, accesses, e := clVarNameChainValid(c, scope, &parsed, vName.First, v.Type)

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

		if ent != enumEntAll {
			break
		}
	}

	return false, false, nil, false, nil
}

func clExistsInScopeTree(c *ctx, name *t.NodeExprName, ent entryType) (found bool, isLastFunc bool, expr t.Node, isSsa bool, err error) {
	currScope := c.CurrScope

	for {
		if currScope == nil {
			return false, false, nil, false, nil
		}

		found, isLastFunc, expr, isSsa, e := clExistsInScope(c, currScope, name, ent)
		if e != nil {
			return false, false, nil, false, e
		}

		if found {
			return true, isLastFunc, expr, isSsa, nil
		}

		currScope = currScope.Parent
	}
}

func clName(c *ctx, name *t.NodeExprName, expected entryType) error {
	// TODO: get associated node for easier type checking later
	found, _, expr, isSsa, err := clExistsInScopeTree(c, name, expected)

	if err != nil {
		return err
	}

	if !found {
		return fmt.Errorf("name expression: %s does not refer to any defined vars", flattenName(name.Name))
	}

	name.IsSsa = isSsa
	name.AssociatedNode = expr

	fmt.Printf("name: %s\n", flattenName(name.Name))
	fmt.Printf("associated: ")
	name.AssociatedNode.Print(0)

	if name.AssociatedNode == nil {
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
		found, imc, expr, isSsa, err := clExistsInScopeTree(c, n, enumEntFuncAndVar)

		isMemberCall = imc

		if err != nil {
			return err
		}

		if !found {
			return fmt.Errorf("name expression in call: %s does not refer to any defined vars", flattenName(n.Name))
		}

		n.IsSsa = isSsa
		isSsaOwner = isSsa
		n.AssociatedNode = expr

		if n.AssociatedNode == nil {
			return fmt.Errorf("name expression: %s does not point to any existing vars, even though there was no errors?", flattenName(n.Name))
		}

		ownerExpr = expr
		nameExpr = n
	default:
		return fmt.Errorf("cannot call expression that is not a name")
	}

	for _, arg := range call.Args {
		e := clExpr(c, arg)
		if e != nil {
			return e
		}
	}

	fmt.Printf("call to: %s\n", flattenName(nameExpr.Name))

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
				fmt.Printf("from member access: ")
				last := nameExpr.MemberAccesses[len(nameExpr.MemberAccesses)-1]
				ownerType = last.Type
				isPointerOwner = last.PtrDeref
			}

			fmt.Printf("owner is ptr deref: %t\n", isPointerOwner)
			fmt.Printf("owner struct def: ")
			ownerType.Print(0)

			// check if is member func on struct
			strt, e := clGetStructDefFromType(c, ownerType)
			if e == nil {
				fmt.Printf("found owner struct def of member call\n")

				for mn, v := range strt.Funcs {
					fmt.Printf("member: %s\n", mn)
					if mn == memberName {
						fmt.Printf("is member func call\n")

						call.IsMemberFunc = true
						call.MemberOwnerType = ownerType
						call.AssociatedFnDef = v
						call.MemberOwnerIsPtr = isPointerOwner
						call.MemberOwnerName = ownerName
						call.MemberOwnerModule = strt.Module
						call.MemberOwnerName.MemberAccesses = nameExpr.MemberAccesses
						return nil
					}
				}
			}

			if isShallowPtr {
				strt, e = clGetStructDefFromType(c, shallowPtrType)
				if e == nil {
					fmt.Printf("found owner struct def of member call after owner deref\n")

					for mn, v := range strt.Funcs {
						fmt.Printf("member: %s\n", mn)
						if mn == memberName {
							fmt.Printf("is member func call\n")

							call.IsMemberFunc = true
							call.MemberOwnerType = ownerType
							call.AssociatedFnDef = v
							call.MemberOwnerIsPtr = true
							call.MemberOwnerName = ownerName
							call.MemberOwnerModule = strt.Module
							call.MemberOwnerName.MemberAccesses = nameExpr.MemberAccesses
							return nil
						}
					}
				}
			}

			fmt.Printf("failed to find owner struct def\n")
		}

		if len(nameExpr.MemberAccesses) > 0 {
			fnType = nameExpr.MemberAccesses[len(nameExpr.MemberAccesses)-1].Type
		}

		fmt.Printf("is func ptr call\n")

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

		fmt.Printf("is func call\n")

		call.AssociatedFnDef = fnDef
	}

	return nil
}

func clExprSubscript(c *ctx, subs *t.NodeExprSubscript) error {
	fmt.Printf("check expr subscript\n")

	switch n := subs.Target.(type) {
	case *t.NodeExprName:
		e := clName(c, n, enumEntVar)
		if e != nil {
			return e
		}
		subs.AssociatedNode = n.AssociatedNode
		subs.IsTargetSsa = n.IsSsa
	default:
		return fmt.Errorf("cannot subscript expression that is not a name")
	}

	e := clExpr(c, subs.Expr)
	if e != nil {
		return e
	}
	return nil
}

func clExpr(c *ctx, expr t.NodeExpr) error {
	switch n := expr.(type) {
	case *t.NodeExprVoid:
		return nil
	case *t.NodeExprSizeof:
		return clType(c, n.Type)
	case *t.NodeExprAddrof:
		return clExpr(c, n.Expr)
	case *t.NodeExprCall:
		return clExprCall(c, n)
	case *t.NodeExprTry:
		return clExprCall(c, n.Call.(*t.NodeExprCall))
	case *t.NodeExprSubscript:
		return clExprSubscript(c, n)
	case *t.NodeExprVarDefAssign:
		e := clExpr(c, n.AssignExpr)
		if e != nil {
			return e
		}
		e = clType(c, n.VarDef.Type)
		if e != nil {
			return e
		}
	case *t.NodeExprVarDef:
		e := clType(c, n.Type)
		if e != nil {
			return e
		}
	case *t.NodeExprAssign:
		e := clExpr(c, n.Left)
		if e != nil {
			return e
		}
		e = clExpr(c, n.Right)
		if e != nil {
			return e
		}
	case *t.NodeExprDestructureAssign:
		return clExprCall(c, n.Call)
	case *t.NodeExprName:
		e := clName(c, n, enumEntFuncAndVar)
		if e != nil {
			return e
		}
	case *t.NodeExprBinary:
		e := clExpr(c, n.Left)
		if e != nil {
			return e
		}
		e = clExpr(c, n.Right)
		if e != nil {
			return e
		}
	}
	return nil
}

func clDefer(c *ctx, def *t.NodeStmtDefer) error {
	if def.IsBody {
		return clBody(c, &def.Body)
	} else {
		return clExpr(c, def.Expression)
	}
}

func clReturn(c *ctx, ret *t.NodeStmtRet) error {
	e := clExpr(c, ret.Expression)
	if e != nil {
		return e
	}
	return nil
}

func clThrow(c *ctx, throw *t.NodeStmtThrow) error {
	e := clExpr(c, throw.Expression)
	if e != nil {
		return e
	}
	return nil
}

func clIf(c *ctx, ifStmt *t.NodeStmtIf) error {
	e := clExpr(c, ifStmt.CondExpr)
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
	e := clExpr(c, whileStmt.CondExpr)
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
	for _, stmt := range bdy.Statements {
		switch n := stmt.(type) {
		case *t.NodeStmtRet:
			e := clReturn(c, n)
			if e != nil {
				return e
			}
		case *t.NodeStmtExpr:
			e := clExpr(c, n.Expression)
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
	case *t.NodeTypeNamed:
		switch nn := n.NameNode.(type) {
		case *t.NodeNameSingle:
			_, ok := magmatypes.BasicTypes[nn.Name]
			if ok {
				return nil, nil // is intrinsic type
			}

			sd, e := clGetStructDefFromName(c, n.NameNode)

			if e == nil && sd.Destructor != nil && topLevel {
				parentType.Destructor = sd.Destructor
			}

			if e == nil {
				return &t.NodeTypeAbsolute{
					AbsoluteName: sd.Module + "." + sd.Name,
				}, nil
			}
			return nil, e
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
			return nil, e
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

	fmt.Print("problem type: ")
	kind.Print(0)
	return nil, fmt.Errorf("failed to find definition for type")
}

func clType(c *ctx, typeNd *t.NodeType) error {
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
		return clExpr(c, n)
	}
	return nil
}

func clGlobal(c *ctx, gl *t.NodeGlobal) error {
	enterScope(c, c.ScopeTree)
	defer leaveScope(c)

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
	}

	for _, v := range s.Files {
		ctx.ModuleBundle.Modules[v.PackageName] = v.GlNode
	}

	for _, fCtx := range s.Files {
		n := fCtx.GlNode
		ctx.GlobalNode = n
		ctx.ScopeTree = &fCtx.ScopeTree

		fmt.Printf("check links of: %s\n", fCtx.PackageName)
		e := clGlobal(ctx, n)
		if e != nil {
			return e
		}
	}

	return nil
}
