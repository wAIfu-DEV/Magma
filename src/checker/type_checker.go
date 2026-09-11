package checker

import (
	"Magma/src/comp_err"
	t "Magma/src/types"
	"fmt"
	"strings"
)

func ctFuncDef(c *ctx, fnDef *t.NodeFuncDef) error {
	c.LastFuncDef = fnDef
	previousTypeFunc := c.CurrentTypeFunc
	c.CurrentTypeFunc = fnDef
	defer func() { c.CurrentTypeFunc = previousTypeFunc }()

	e := ctBody(c, &fnDef.Body)
	if e != nil {
		return e
	}
	if !fnDef.IsExternal && fnDef.ProtoDispatch == nil && !isVoidType(fnDef.ReturnType) && bodyFallsThrough(&fnDef.Body) {
		return comp_err.CompilationErrorToken(c.FileCtx, &fnDef.Body.EndTk, fmt.Sprintf("function returning '%s' can reach the end without returning a value", flattenType(fnDef.ReturnType)), "return or throw on every reachable control-flow path")
	}
	return checkContextInitialization(c.FileCtx, fnDef)
}

// bodyFallsThrough is the checker-side control-flow contract. It deliberately
// answers only whether normal execution can reach the closing `..`; ownership
// and bounded proofs remain separate analyses.
func bodyFallsThrough(body *t.NodeBody) bool {
	canFallThrough := true
	for _, statement := range body.Statements {
		if !canFallThrough {
			break
		}
		canFallThrough = statementFallsThrough(statement)
	}
	return canFallThrough
}

func statementFallsThrough(statement t.NodeStatement) bool {
	switch node := statement.(type) {
	case *t.NodeStmtRet, *t.NodeStmtThrow:
		return false
	case *t.NodeStmtIf:
		if bodyFallsThrough(&node.Body) {
			return true
		}
		if expressionAlwaysTrue(node.CondExpr) {
			return false
		}
		for next := node.NextCondStmt; next != nil; {
			switch branch := next.(type) {
			case *t.NodeStmtIf:
				if bodyFallsThrough(&branch.Body) {
					return true
				}
				if expressionAlwaysTrue(branch.CondExpr) {
					return false
				}
				next = branch.NextCondStmt
			case *t.NodeStmtElse:
				return bodyFallsThrough(&branch.Body)
			default:
				return true
			}
		}
		return true // no final else
	case *t.NodeStmtMatch:
		for _, arm := range node.Cases {
			if bodyFallsThrough(&arm.Body) {
				return true
			}
		}
		return node.ElseBody == nil || bodyFallsThrough(node.ElseBody)
	case *t.NodeStmtUnsafe:
		return bodyFallsThrough(&node.Body)
	case *t.NodeStmtBounded:
		return bodyFallsThrough(&node.Body)
	case *t.NodeStmtWhile:
		literal, always := node.CondExpr.(*t.NodeExprLit)
		return !(always && literal.LitType == t.TokLitBool && literal.Value == "1" && !bodyBreaksCurrentLoop(&node.Body))
	case *t.NodeLlvm:
		// Inline LLVM is currently opaque. Recognize only an explicit terminator
		// line; a future typed IR directive should replace this compatibility path.
		for _, line := range strings.Split(node.Text, "\\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "ret ") || line == "unreachable" {
				return false
			}
		}
	}
	return true
}

func expressionAlwaysTrue(expression t.NodeExpr) bool {
	literal, ok := expression.(*t.NodeExprLit)
	return ok && literal.LitType == t.TokLitBool && literal.Value == "1"
}

func bodyBreaksCurrentLoop(body *t.NodeBody) bool {
	for _, statement := range body.Statements {
		switch node := statement.(type) {
		case *t.NodeStmtBreak:
			return true
		case *t.NodeStmtIf:
			if bodyBreaksCurrentLoop(&node.Body) {
				return true
			}
			for next := node.NextCondStmt; next != nil; {
				switch branch := next.(type) {
				case *t.NodeStmtIf:
					if bodyBreaksCurrentLoop(&branch.Body) {
						return true
					}
					next = branch.NextCondStmt
				case *t.NodeStmtElse:
					if bodyBreaksCurrentLoop(&branch.Body) {
						return true
					}
					next = nil
				default:
					next = nil
				}
			}
		case *t.NodeStmtUnsafe:
			if bodyBreaksCurrentLoop(&node.Body) {
				return true
			}
		case *t.NodeStmtBounded:
			if bodyBreaksCurrentLoop(&node.Body) {
				return true
			}
			// Breaks in nested loops do not exit the loop being summarized.
		}
	}
	return false
}

func isSimpleConstInitializer(expr t.NodeExpr) bool {
	switch n := expr.(type) {
	case *t.NodeExprLit:
		return true
	case *t.NodeExprEmbed:
		return true
	case *t.NodeExprArray:
		if _, ok := constArrayIndex(n.Length); !ok {
			return false
		}
		for _, entry := range n.Entries {
			if entry.Index != nil {
				if _, ok := constArrayIndex(entry.Index); !ok {
					return false
				}
			}
			if !isSimpleConstInitializer(entry.Value) {
				return false
			}
		}
		return true
	case *t.NodeExprName:
		switch associated := n.AssociatedNode.(type) {
		case *t.NodeFuncDef:
			return true
		case *t.NodeExprVarDef:
			return associated.IsConst && associated.Initializer != nil
		}
		return false
	case *t.NodeExprAddrof:
		name, ok := n.Expr.(*t.NodeExprName)
		if !ok {
			return false
		}
		variable, ok := name.AssociatedNode.(*t.NodeExprVarDef)
		return ok && variable.IsGlobal
	case *t.NodeExprStructInit:
		for _, field := range n.Fields {
			if !isSimpleConstInitializer(field.Expression) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func ctGlDecl(c *ctx, glDecl t.NodeGlobalDecl) error {
	switch n := glDecl.(type) {
	case *t.NodeFuncDef:
		return ctFuncDef(c, n)
	case *t.NodeExprVarDef:
		if n.Initializer != nil {
			if e := ctExpr(c, n.Initializer); e != nil {
				return e
			}
			if n.Type == nil {
				n.Type = n.Initializer.GetInferredType()
			}
			if !compatibleInitializer(c, n.Type, n.Initializer) {
				return comp_err.CompilationErrorToken(c.FileCtx, &t.Token{}, fmt.Sprintf("cannot initialize global '%s' of type '%s' with expression of type '%s'", flattenName(n.Name), flattenType(n.Type), flattenType(n.Initializer.GetInferredType())), "")
			}
			warnNumericConversion(c, n.Type, n.Initializer, "global initialization")
		}
		return ctExpr(c, n)
	case *t.NodeStructDef:
		for _, field := range n.Class.ArgsNode.Args {
			if field.TypeNode == nil || field.TypeNode.KindNode == nil {
				return comp_err.CompilationErrorToken(c.FileCtx, &field.Tk, fmt.Sprintf("field '%s' has no resolved type", field.Name), "provide a concrete field type")
			}
			if field.TypeNode.Throws {
				return comp_err.CompilationErrorToken(c.FileCtx, &field.Tk, fmt.Sprintf("field '%s' cannot have a throwing type", field.Name), "throwing types are only valid as function return types")
			}
		}
		return nil
	case *t.NodeConstDef:
		if !isSimpleConstInitializer(n.Initializer) {
			return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, "constant initializer must be a literal, constant value, function value, or struct constructor", "general constant expressions are not supported")
		}
		if e := ctExpr(c, n.Initializer); e != nil {
			return e
		}
		if n.VarDef.Type == nil {
			n.VarDef.Type = n.Initializer.GetInferredType()
			return nil
		}
		if !compatibleInitializer(c, n.VarDef.Type, n.Initializer) {
			return comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, fmt.Sprintf("constant %s expects %s but initializer has type %s", flattenName(n.VarDef.Name), flattenType(n.VarDef.Type), flattenType(n.Initializer.GetInferredType())), "")
		}
		warnNumericConversion(c, n.VarDef.Type, n.Initializer, "constant initialization")
		return nil
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
	if err := checkRecursiveStructLayouts(s); err != nil {
		return err
	}

	for _, fCtx := range s.Files {
		if fCtx.InterfaceOnly {
			continue
		}
		// fmt.Printf("check types of: %s\n", fCtx.PackageName)

		n := fCtx.GlNode
		ctx.GlobalNode = n
		ctx.FileCtx = fCtx
		e := ctGlobal(ctx, n)
		if e != nil {
			return comp_err.EnsureDiagnostic(fCtx, &t.Token{Pos: t.FilePos{Line: 1, Col: 1}}, e)
		}
	}

	return nil
}

func checkRecursiveStructLayouts(s *t.SharedState) error {
	definitions := map[string]structEntry{}
	for _, file := range s.Files {
		if file == nil || file.GlNode == nil {
			continue
		}
		for _, declaration := range file.GlNode.Declarations {
			if definition, ok := declaration.(*t.NodeStructDef); ok {
				definitions[definition.AbsName] = structEntry{file: file, definition: definition}
			}
		}
	}
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var visit func(string) error
	visit = func(name string) error {
		if visiting[name] {
			entry := definitions[name]
			return comp_err.CompilationErrorToken(entry.file, lastNameToken(entry.definition.Class.NameNode), fmt.Sprintf("struct '%s' has an infinitely recursive value layout", flattenName(entry.definition.Class.NameNode)), "break the cycle with a pointer, slice, or function field")
		}
		if visited[name] {
			return nil
		}
		entry, exists := definitions[name]
		if !exists {
			return nil
		}
		visiting[name] = true
		for _, field := range entry.definition.Class.ArgsNode.Args {
			if target := valueStructType(field.TypeNode); target != "" {
				if err := visit(target); err != nil {
					return err
				}
			}
		}
		delete(visiting, name)
		visited[name] = true
		return nil
	}
	for name := range definitions {
		if err := visit(name); err != nil {
			return err
		}
	}
	return nil
}

type structEntry struct {
	file       *t.FileCtx
	definition *t.NodeStructDef
}

func valueStructType(node *t.NodeType) string {
	if node == nil {
		return ""
	}
	absolute, ok := node.KindNode.(*t.NodeTypeAbsolute)
	if !ok {
		return "" // indirection and callable/container types do not recurse by value
	}
	return absolute.AbsoluteName
}
