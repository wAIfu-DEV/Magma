package checker

import (
	t "Magma/src/types"
	"fmt"
)

func makeNamedType(name string) *t.NodeType {
	return &t.NodeType{
		Throws: false,
		KindNode: &t.NodeTypeNamed{
			NameNode: &t.NodeNameSingle{Name: name},
		},
	}
}

func makePtrType(from *t.NodeType) *t.NodeType {
	cpy := *from

	var kind t.NodeTypeKind

	switch n := cpy.KindNode.(type) {
	case *t.NodeTypeNamed:
		kind = &t.NodeTypePointer{
			Kind: n,
		}
	}

	return &t.NodeType{
		Throws:   cpy.Throws,
		KindNode: kind,
	}
}

func isVoidType(node *t.NodeType) bool {
	if node == nil {
		return false
	}

	switch n := node.KindNode.(type) {
	case *t.NodeTypeNamed:
		switch nn := n.NameNode.(type) {
		case *t.NodeNameSingle:
			return nn.Name == "void"
		}
	}
	return false
}

func ctExpr(c *ctx, expr t.NodeExpr) error {
	switch n := expr.(type) {
	case *t.NodeExprVoid:
		n.VoidType = makeNamedType("void")
		return nil
	case *t.NodeExprCall:
		for _, a := range n.Args {
			e := ctExpr(c, a)
			if e != nil {
				return e
			}
		}

		e := ctExpr(c, n.Callee)
		if e != nil {
			return e
		}
		n.InfType = n.AssociatedFnDef.ReturnType
		return nil
	case *t.NodeExprLit:
		switch n.LitType {
		case t.TokLitNum:
			n.InfType = makeNamedType("i64")
			return nil
		case t.TokLitStr:
			n.InfType = makeNamedType("str")
			return nil
		}
	case *t.NodeExprName:
		// TODO: resolve type of named variable
		return nil
	case *t.NodeExprBinary:
		e := ctExpr(c, n.Left)
		if e != nil {
			return e
		}

		e = ctExpr(c, n.Right)
		if e != nil {
			return e
		}

		n.InfType = n.Left.GetInferredType()
		return nil
	case *t.NodeExprUnary:
		switch n.Operator {
		case t.KwAsterisk:
			e := ctExpr(c, n.Operand)
			if e != nil {
				return e
			}
			n.InfType = makePtrType(n.Operand.GetInferredType())
			return nil
		}
		return fmt.Errorf("unexpected unary expression type")
	case *t.NodeExprVarDef:
		return nil
	case *t.NodeExprVarDefAssign:
		e := ctExpr(c, n.AssignExpr)
		if e != nil {
			return e
		}
		n.GetInferredType()
		return nil
	}
	return fmt.Errorf("unexpected expression type")
}

func ctReturn(c *ctx, ret *t.NodeStmtRet) error {
	if c.LastFuncDef == nil {
		// TODO: compiler error
		return fmt.Errorf("return statement outside function")
	}

	if c.LastFuncDef.ReturnType == nil {
		// TODO: compiler error
		return fmt.Errorf("function return type is null when trying to infer ret type")
	}

	ret.OwnerFuncType = c.LastFuncDef.ReturnType

	e := ctExpr(c, ret.Expression)
	if e != nil {
		return e
	}

	retIsVoid := isVoidType(ret.OwnerFuncType)
	exprIsVoid := isVoidType(ret.Expression.GetInferredType())

	if retIsVoid != exprIsVoid {
		if retIsVoid {
			return fmt.Errorf("unexpected expression after 'ret' statment within func with return type 'void'")
		}
		return fmt.Errorf("missing expression after 'ret' statement in function with non-null return type")
	}

	// TODO: check if is same type
	return nil
}

func ctBody(c *ctx, bdy *t.NodeBody) error {
	for _, stmt := range bdy.Statements {
		switch n := stmt.(type) {
		case *t.NodeStmtRet:
			e := ctReturn(c, n)
			if e != nil {
				return e
			}
		case *t.NodeStmtExpr:
			e := ctExpr(c, n.Expression)
			if e != nil {
				return e
			}
		}
	}
	return nil
}

func ctFuncDef(c *ctx, fnDef *t.NodeFuncDef) error {
	c.LastFuncDef = fnDef

	e := ctBody(c, &fnDef.Body)
	if e != nil {
		return e
	}
	return nil
}

func ctGlDecl(c *ctx, glDecl t.NodeGlobalDecl) error {
	switch n := glDecl.(type) {
	case *t.NodeFuncDef:
		return ctFuncDef(c, n)
	case *t.NodeStructDef:
		return nil // TODO: check type names of arguments
	}
	return nil
}

func ctGlobal(c *ctx, gl *t.NodeGlobal) error {
	for _, dcl := range gl.Declarations {
		e := ctGlDecl(c, dcl)
		if e != nil {
			return e
		}
	}
	return nil
}

func TypeChecker(s *t.SharedState) error {
	ctx := &ctx{
		Shared: s,
	}

	for _, fCtx := range s.Files {
		n := fCtx.GlNode
		ctx.GlobalNode = n
		e := ctGlobal(ctx, n)
		if e != nil {
			return e
		}
	}

	return nil
}
