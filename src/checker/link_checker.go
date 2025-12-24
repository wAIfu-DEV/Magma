package checker

import (
	magmatypes "Magma/src/magma_types"
	t "Magma/src/types"
	"fmt"
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
	}
	return nil, fmt.Errorf("failed to get struct def from type")
}

func clGetFuncDefFromModule(c *ctx, name parsedName) (*t.NodeFuncDef, error) {
	if !name.HasParts {
		// TODO: compiler error
		return nil, fmt.Errorf("cannot get function def from module with simply named struct")
	}

	moduleAlias := name.First
	fnName := name.Parts[0]

	// resolve alias
	moduleName, ok := c.GlobalNode.ImportAlias[moduleAlias]

	if !ok {
		// TODO: compilation error
		return nil, fmt.Errorf("module alias does not exist in file")
	}

	moduleGlNode := c.ModuleBundle.Modules[moduleName]
	fnDef, ok := moduleGlNode.FuncDefs[fnName]

	if !ok {
		// TODO: compilation error
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

func clVarNameChainValid(c *ctx, scope *t.Scope, name *parsedName, varName string, varType *t.NodeType) (lastIsFunc bool, e error) {

	// get struct def for type
	structDef, e := clGetStructDefFromType(c, varType)
	if e != nil {
		return false, e
	}

	foundMemberFunc := false
	memberName := ""

	for _, part := range name.Parts {
		if foundMemberFunc {
			// TODO: compiler error
			return false, fmt.Errorf("tried to access inexistent field '%s' from member function '%s' of '%s'", part, memberName, structDef.Name)
		}

		// check if member name exists in struct def
		fieldType, ok := structDef.Fields[part]

		if ok {
			structDef, e = clGetStructDefFromType(c, fieldType)
			if e != nil {
				return false, e
			}
			continue
		}

		_, ok = structDef.Funcs[part]

		if ok {
			foundMemberFunc = true
			continue
		}
	}

	return foundMemberFunc, nil
}

func clExistsInScope(c *ctx, scope *t.Scope, name t.NodeName, ent entryType) (bool, t.Node, bool, error) {
	parsed := parseName(name)

	switch ent {
	case enumEntAll:
		fallthrough
	case enumEntVar:
		for _, v := range scope.DeclVars {
			vName := parseName(v.Name)
			if (!parsed.HasParts && !vName.HasParts) && parsed.First == vName.First {
				return true, v, v.IsSsa, nil
			}

			if parsed.First != vName.First {
				continue
			}

			// TODO: this won't work for access to vars declared in other modules
			_, e := clVarNameChainValid(c, scope, &parsed, vName.First, v.Type)

			if e != nil {
				return false, nil, false, e
			}
			return true, v, v.IsSsa, nil
		}

		if ent != enumEntAll {
			return false, nil, false, nil
		}
		fallthrough
	case enumEntFunc:
		for _, f := range scope.DeclFuncs {
			fName := parseName(f.Func.Class.NameNode)
			if (!parsed.HasParts && !fName.HasParts) && parsed.First == fName.First {
				return true, f.Func, false, nil
			}

			_, e := clGetFuncDefFromName(c, f.Func.Class.NameNode)
			if e != nil {
				return false, nil, false, e
			}
			return true, f.Func, false, nil
		}

		if ent != enumEntAll {
			return false, nil, false, nil
		}
		fallthrough
	case enumEntStruct:
		s, e := clGetStructDefFromName(c, name)
		if e == nil {
			return true, s, false, nil
		}

		if ent != enumEntAll {
			break
		}
	}

	return false, nil, false, nil
}

func clExistsInScopeTree(c *ctx, name t.NodeName, ent entryType) (found bool, expr t.Node, isSsa bool, err error) {
	currScope := c.CurrScope

	for {
		if currScope == nil {
			return false, nil, false, nil
		}

		found, expr, isSsa, e := clExistsInScope(c, currScope, name, ent)
		if e != nil {
			return false, nil, false, e
		}

		if found {
			return true, expr, isSsa, nil
		}

		currScope = currScope.Parent
	}
}

func clName(c *ctx, name *t.NodeExprName) error {
	// TODO: get associated node for easier type checking later
	found, expr, isSsa, err := clExistsInScopeTree(c, name.Name, enumEntVar)

	if err != nil {
		return err
	}

	if !found {
		return fmt.Errorf("name expression: %s does not refer to any defined vars", flattenName(name.Name))
	}

	name.IsSsa = isSsa
	name.AssociatedNode = expr
	return nil
}

func clExprCall(c *ctx, call *t.NodeExprCall) error {
	fmt.Printf("check expr call\n")

	switch call.Callee.(type) {
	case *t.NodeExprName:
		break
	default:
		return fmt.Errorf("cannot call expression that is not a name")
	}

	for _, arg := range call.Args {
		e := clExpr(c, arg)
		if e != nil {
			return e
		}
	}

	fnDef, e := clGetFuncDefFromName(c, call.Callee.(*t.NodeExprName).Name)
	if e != nil {
		return e
	}

	call.AssociatedFnDef = fnDef

	if fnDef == nil {
		return fmt.Errorf("associated function def is null")
	}

	return nil
}

func clExpr(c *ctx, expr t.NodeExpr) error {
	switch n := expr.(type) {
	case *t.NodeExprVoid:
		return nil
	case *t.NodeExprCall:
		return clExprCall(c, n)
	case *t.NodeExprVarDefAssign:
		e := clExpr(c, n.AssignExpr)
		if e != nil {
			return e
		}
	case *t.NodeExprName:
		e := clName(c, n)
		if e != nil {
			return e
		}
	}
	return nil
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
		}
	}
	return nil
}

func clType(c *ctx, typeNd *t.NodeType) error {
	switch n := typeNd.KindNode.(type) {
	case *t.NodeTypeNamed:

		switch nn := n.NameNode.(type) {
		case *t.NodeNameSingle:
			_, ok := magmatypes.BasicTypes[nn.Name]
			if ok {
				return nil // is intrinsic type
			}

			_, e := clGetStructDefFromName(c, n.NameNode)
			return e
		}
	}
	// TODO: compiler error
	return fmt.Errorf("failed to find definition for type")
}

func clFuncDef(c *ctx, fnDef *t.NodeFuncDef) error {
	var scope *t.Scope = nil

	for _, f := range c.CurrScope.DeclFuncs {
		if f.Func == fnDef {
			scope = f.Scope
		}
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

func clGlDecl(c *ctx, glDecl t.NodeGlobalDecl) error {
	switch n := glDecl.(type) {
	case *t.NodeFuncDef:
		return clFuncDef(c, n)
	case *t.NodeStructDef:
		return nil // TODO: check type names of arguments
	}
	return nil
}

func clGlobal(c *ctx, gl *t.NodeGlobal) error {
	enterScope(c, c.ScopeTree)
	defer leaveScope(c)

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
