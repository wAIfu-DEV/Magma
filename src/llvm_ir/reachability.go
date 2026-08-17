package llvmir

import (
	"regexp"

	t "Magma/src/types"
)

// reachableFunctions computes the function bodies required by backend
// emission. Semantic analysis deliberately runs before this pass and still
// checks every declaration, including declarations which are not reachable.
func reachableFunctions(files map[string]*t.FileCtx, nullContext bool) (map[*t.NodeFuncDef]bool, map[string]bool) {
	all := allFunctions(files)
	bySymbol := make(map[string]*t.NodeFuncDef)
	var roots []*t.NodeFuncDef
	hasProgramRoot := false
	var globalLLVM []string
	var globalExpressions []t.NodeExpr

	for _, file := range files {
		if file.GlNode == nil {
			continue
		}
		for _, declaration := range file.GlNode.Declarations {
			switch node := declaration.(type) {
			case *t.NodeFuncDef:
				if node.AbsName != "" {
					bySymbol[node.AbsName] = node
				}
				if node.NoAliasName != "" {
					bySymbol[node.NoAliasName] = node
				}
				if node.ExportName != "" || (node.IsEntryPoint && file.PackageName == file.MainPckgName) {
					roots = append(roots, node)
					hasProgramRoot = true
				}
			case *t.NodeLlvm:
				globalLLVM = append(globalLLVM, node.Text)
			case *t.NodeExprVarDef:
				globalExpressions = append(globalExpressions, node.Initializer)
			case *t.NodeConstDef:
				globalExpressions = append(globalExpressions, node.Initializer)
			}
		}

	}

	// Direct IrWrite users often construct fragments without an executable or
	// export root. Preserve the historical behavior for those library-style
	// inputs instead of silently emitting no functions.
	if !hasProgramRoot {
		return all, allProtoVtables(files)
	}

	contextInitializer := "newDefault"
	if nullContext {
		contextInitializer = "newNull"
	}
	for _, file := range files {
		if file.GlNode != nil && file.ModuleName == "context_default" {
			if fn := file.GlNode.FuncDefs[contextInitializer]; fn != nil {
				roots = append(roots, fn)
			}
		}
	}

	reachable := make(map[*t.NodeFuncDef]bool)
	reachableVtables := make(map[string]bool)
	queue := make([]*t.NodeFuncDef, 0, len(roots))
	enqueue := func(fn *t.NodeFuncDef) {
		if fn != nil && !reachable[fn] {
			reachable[fn] = true
			queue = append(queue, fn)
		}
	}
	for _, root := range roots {
		enqueue(root)
	}

	markLLVMReferences := func(text string) {
		for _, match := range llvmSymbolReference.FindAllStringSubmatch(text, -1) {
			if fn := bySymbol[match[1]]; fn != nil {
				enqueue(fn)
			}
		}
	}
	for _, text := range globalLLVM {
		markLLVMReferences(text)
	}

	walker := reachabilityWalker{
		enqueue:              enqueue,
		markLLVMReferences:   markLLVMReferences,
		reachableProtoTables: reachableVtables,
	}
	for _, expression := range globalExpressions {
		walker.expression(expression)
	}
	for len(queue) > 0 {
		fn := queue[0]
		queue = queue[1:]
		walker.function(fn)
	}
	return reachable, reachableVtables
}

func allFunctions(files map[string]*t.FileCtx) map[*t.NodeFuncDef]bool {
	all := make(map[*t.NodeFuncDef]bool)
	for _, file := range files {
		if file.GlNode == nil {
			continue
		}
		for _, declaration := range file.GlNode.Declarations {
			if fn, ok := declaration.(*t.NodeFuncDef); ok {
				all[fn] = true
			}
		}
	}
	return all
}

func allProtoVtables(files map[string]*t.FileCtx) map[string]bool {
	vtables := make(map[string]bool)
	for _, file := range files {
		if file.GlNode == nil {
			continue
		}
		for _, implementation := range file.GlNode.StructDefs {
			for _, relation := range implementation.Implements {
				if relation != nil && relation.Proto != nil {
					vtables[t.ProtoVtableSymbol(implementation, relation.Proto)] = true
				}
			}
		}
	}
	return vtables
}

var llvmSymbolReference = regexp.MustCompile(`@([A-Za-z$._][A-Za-z0-9$._]*)`)

type reachabilityWalker struct {
	enqueue              func(*t.NodeFuncDef)
	markLLVMReferences   func(string)
	reachableProtoTables map[string]bool
}

func (w *reachabilityWalker) function(fn *t.NodeFuncDef) {
	w.nodeType(fn.ReturnType)
	for i := range fn.Class.ArgsNode.Args {
		w.nodeType(fn.Class.ArgsNode.Args[i].TypeNode)
	}
	for _, statement := range fn.Body.Statements {
		w.statement(statement)
	}
}

func (w *reachabilityWalker) nodeType(node *t.NodeType) {
	if node == nil {
		return
	}
	w.enqueue(node.Destructor)
	switch kind := node.KindNode.(type) {
	case *t.NodeTypeNamed:
		for _, argument := range kind.GenericArgs {
			w.nodeType(argument)
		}
	case *t.NodeTypePointer:
		w.typeKind(kind.Kind)
	case *t.NodeTypeRfc:
		w.typeKind(kind.Kind)
	case *t.NodeTypeSlice:
		w.typeKind(kind.ElemKind)
	case *t.NodeTypeFunc:
		for _, argument := range kind.Args {
			w.nodeType(argument)
		}
		w.nodeType(kind.RetType)
	}
}

func (w *reachabilityWalker) typeKind(kind t.NodeTypeKind) {
	if kind != nil {
		w.nodeType(&t.NodeType{KindNode: kind})
	}
}

func (w *reachabilityWalker) body(body *t.NodeBody) {
	if body == nil {
		return
	}
	for _, statement := range body.Statements {
		w.statement(statement)
	}
}

func (w *reachabilityWalker) statement(statement t.NodeStatement) {
	switch node := statement.(type) {
	case *t.NodeStmtRet:
		w.expression(node.Expression)
	case *t.NodeStmtExpr:
		w.expression(node.Expression)
	case *t.NodeStmtThrow:
		w.expression(node.Expression)
	case *t.NodeStmtIf:
		w.expression(node.CondExpr)
		w.body(&node.Body)
		w.statement(node.NextCondStmt)
	case *t.NodeStmtElse:
		w.body(&node.Body)
	case *t.NodeStmtWhile:
		w.expression(node.CondExpr)
		w.body(&node.Body)
	case *t.NodeStmtFor:
		w.expression(node.DeclExpr)
		w.expression(node.BoundExpr)
		w.body(&node.Body)
	case *t.NodeStmtBounded:
		for _, predicate := range node.Predicates {
			w.expression(predicate)
		}
		w.body(&node.Body)
	case *t.NodeStmtUnsafe:
		w.body(&node.Body)
	case *t.NodeStmtDefer:
		w.expression(node.Expression)
		w.body(&node.Body)
	case *t.NodeLlvm:
		w.markLLVMReferences(node.Text)
	}
}

func (w *reachabilityWalker) expression(expression t.NodeExpr) {
	if expression == nil {
		return
	}
	w.nodeType(expression.GetInferredType())
	switch node := expression.(type) {
	case *t.NodeExprUnary:
		w.expression(node.Operand)
	case *t.NodeExprArray:
		w.expression(node.Length)
		for _, entry := range node.Entries {
			w.expression(entry.Index)
			w.expression(entry.Value)
		}
	case *t.NodeExprName:
		if fn, ok := node.AssociatedNode.(*t.NodeFuncDef); ok {
			w.enqueue(fn)
		}
	case *t.NodeExprCall:
		w.enqueue(node.AssociatedFnDef)
		w.expression(node.Callee)
		w.expression(node.MemberOwnerExpr)
		for _, argument := range node.Args {
			w.expression(argument)
		}
	case *t.NodeExprStructInit:
		w.nodeType(node.Type)
		for _, field := range node.Fields {
			w.expression(field.Expression)
		}
	case *t.NodeExprProtoView:
		w.expression(node.Target)
		if node.Implementation != nil && node.Implementation.Owner != nil && node.Implementation.Proto != nil {
			w.reachableProtoTables[t.ProtoVtableSymbol(node.Implementation.Owner, node.Implementation.Proto)] = true
			for _, method := range node.Implementation.Proto.Methods {
				w.enqueue(node.Implementation.Owner.Funcs[method.Name])
			}
		}
	case *t.NodeExprSubscript:
		w.expression(node.Target)
		w.expression(node.Expr)
	case *t.NodeExprMemberAccess:
		w.expression(node.Target)
	case *t.NodeExprBinary:
		w.expression(node.Left)
		w.expression(node.Right)
	case *t.NodeExprVarDef:
		w.nodeType(node.Type)
		w.expression(node.Initializer)
	case *t.NodeExprVarDefAssign:
		w.expression(node.VarDef)
		w.expression(node.AssignExpr)
	case *t.NodeExprAssign:
		w.expression(node.Left)
		w.expression(node.Right)
	case *t.NodeExprTry:
		w.expression(node.Call)
	case *t.NodeExprAddrof:
		w.expression(node.Expr)
	case *t.NodeExprMove:
		w.expression(node.Expr)
	case *t.NodeExprDestructureAssign:
		w.expression(&node.ValueDef)
		w.expression(&node.ErrDef)
		w.expression(node.Call)
	case *t.NodeExprDestructor:
		w.enqueue(node.Destructor)
		w.expression(node.VarDef)
	}
}
