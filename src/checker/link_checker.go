package checker

import (
	"Magma/src/comp_err"
	t "Magma/src/types"
	"fmt"
	"path/filepath"
	"sort"
)

func clResolveImplementations(c *ctx, st *t.StructDef) error {
	seen := map[string]bool{}
	for _, implementation := range st.Implements {
		if err := clType(c, implementation.Type); err != nil {
			return err
		}
		protoStruct, err := clGetStructDefFromType(c, implementation.Type)
		if err != nil || protoStruct == nil || !protoStruct.IsProto || protoStruct.Proto == nil {
			return comp_err.CompilationErrorToken(c.FileCtx, &implementation.Tk, fmt.Sprintf("type '%s' is not a prototype", t.DisplayType(implementation.Type)), "only types declared with `proto` may follow `impl`")
		}
		proto := protoStruct.Proto
		key := proto.Module + "." + proto.Name
		if seen[key] {
			return comp_err.CompilationErrorToken(c.FileCtx, &implementation.Tk, fmt.Sprintf("prototype '%s' is implemented more than once", proto.Name), "")
		}
		seen[key] = true
		implementation.Proto = proto
		implementation.Owner = st
		for _, requirement := range proto.Methods {
			method := st.Funcs[requirement.Name]
			if method == nil {
				return comp_err.CompilationErrorToken(c.FileCtx, &implementation.Tk, fmt.Sprintf("type '%s' does not implement '%s': missing method '%s'", st.Name, proto.Name, requirement.Name), fmt.Sprintf("declare `%s.%s` with the prototype signature", st.Name, requirement.Name))
			}
			if method.ContextABI != requirement.ContextABI {
				return comp_err.CompilationErrorToken(c.FileCtx, &implementation.Tk, fmt.Sprintf("method '%s.%s' has an incompatible context calling convention", st.Name, requirement.Name), "match the prototype method's noctx modifier")
			}
			actual := method.Class.ArgsNode.Args
			if len(actual) == 0 || len(actual)-1 != len(requirement.Args) {
				return comp_err.CompilationErrorToken(c.FileCtx, &implementation.Tk, fmt.Sprintf("method '%s.%s' does not satisfy '%s.%s': expected %d argument(s), got %d", st.Name, requirement.Name, proto.Name, requirement.Name, len(requirement.Args), max(0, len(actual)-1)), "")
			}
			for i := range requirement.Args {
				if !sameType(actual[i+1].TypeNode, requirement.Args[i].TypeNode) {
					return comp_err.CompilationErrorToken(c.FileCtx, &actual[i+1].Tk, fmt.Sprintf("method '%s.%s' parameter %d has type '%s', expected '%s'", st.Name, requirement.Name, i+1, t.DisplayType(actual[i+1].TypeNode), t.DisplayType(requirement.Args[i].TypeNode)), "")
				}
				requiredCount, actualCount := -1, -1
				for j, parameter := range requirement.Args {
					if parameter.Name == requirement.Args[i].BoundedCount && parameter.Name != "" {
						requiredCount = j
					}
				}
				for j, parameter := range actual[1:] {
					if parameter.Name == actual[i+1].BoundedCount && parameter.Name != "" {
						actualCount = j
					}
				}
				if requiredCount != actualCount {
					return comp_err.CompilationErrorToken(c.FileCtx, &actual[i+1].Tk, fmt.Sprintf("method '%s.%s' parameter %d has a mismatched bounded extent contract", st.Name, requirement.Name, i+1), "match the prototype's bounded parameter contract")
				}
				if requiredCount == -1 && requirement.Args[i].BoundedCount != actual[i+1].BoundedCount {
					return comp_err.CompilationErrorToken(c.FileCtx, &actual[i+1].Tk, fmt.Sprintf("method '%s.%s' parameter %d has a mismatched bounded extent contract", st.Name, requirement.Name, i+1), "match the prototype's bounded parameter contract")
				}
			}
			if !sameType(method.ReturnType, requirement.Ret) {
				return comp_err.CompilationErrorToken(c.FileCtx, &implementation.Tk, fmt.Sprintf("method '%s.%s' returns '%s', expected '%s'", st.Name, requirement.Name, t.DisplayType(method.ReturnType), t.DisplayType(requirement.Ret)), "")
			}
		}
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

	if fnDef.ImplicitContext != nil {
		if fnDef.ImplicitContext.Type == nil {
			fnDef.ImplicitContext.Type = t.ImplicitContextType(c.Shared)
		}
	}

	for _, arg := range fnDef.Class.ArgsNode.Args {
		e := clTypeForUsage(c, arg.TypeNode, typeUsageValue, "a function parameter type")
		if e != nil {
			return e
		}
	}

	e := clTypeForUsage(c, fnDef.ReturnType, typeUsageReturn, "a function return type")
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
		e := clTypeForUsage(c, arg.TypeNode, typeUsageValue, "a struct field type")
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
			if e := clTypeForUsage(c, n.VarDef.Type, typeUsageValue, "a constant type"); e != nil {
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

func clGlobalValue(c *ctx, declaration t.NodeGlobalDecl) error {
	switch declaration.(type) {
	case *t.NodeExprVarDef, *t.NodeConstDef:
		return clGlDecl(c, declaration)
	default:
		return nil
	}
}

func clGlobalBody(c *ctx, declaration t.NodeGlobalDecl) error {
	if function, ok := declaration.(*t.NodeFuncDef); ok {
		return clFuncDef(c, function)
	}
	return nil
}

// clGlobalInterface resolves every declaration shape which may be consumed by
// another module.  It deliberately does not visit initializers or function
// bodies: all module interfaces must be complete before any implementation is
// linked, otherwise a circular import makes success depend on map iteration
// order.
func clGlobalInterface(c *ctx, gl *t.NodeGlobal) error {
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
			e := clTypeForUsage(c, arg.TypeNode, typeUsageValue, "a function parameter type")
			if e != nil {
				return e
			}
		}
		e := clTypeForUsage(c, fn.ReturnType, typeUsageReturn, "a function return type")
		if e != nil {
			return e
		}
	}

	for _, st := range gl.StructDefs {
		for _, fld := range st.Fields {
			e := clTypeForUsage(c, fld, typeUsageValue, "a struct field type")
			if e != nil {
				return e
			}
		}

		for _, fn := range st.Funcs {
			for _, arg := range fn.Class.ArgsNode.Args {
				e := clTypeForUsage(c, arg.TypeNode, typeUsageValue, "a function parameter type")
				if e != nil {
					return e
				}
			}

			e := clTypeForUsage(c, fn.ReturnType, typeUsageReturn, "a function return type")
			if e != nil {
				return e
			}
		}
	}
	// StructDef.Fields is the lookup-facing view, while the declaration retains
	// the field nodes consumed by lowering. Resolve both representations before
	// implementations are linked; they are not guaranteed to share NodeType
	// pointers after parsing or specialization.
	for _, declaration := range gl.Declarations {
		if definition, ok := declaration.(*t.NodeStructDef); ok {
			if err := clStructDef(c, definition); err != nil {
				return err
			}
		}
	}
	for _, st := range gl.StructDefs {
		if err := clResolveImplementations(c, st); err != nil {
			return err
		}
	}

	return nil
}

func clGlobalValues(c *ctx, gl *t.NodeGlobal) error {
	enterScope(c, c.ScopeTree)
	defer leaveScope(c)

	var firstErr error
	for _, dcl := range gl.Declarations {
		if e := clGlobalValue(c, dcl); e != nil {
			if firstErr == nil {
				firstErr = e
			}
		}
	}
	return firstErr
}

func clGlobalBodies(c *ctx, gl *t.NodeGlobal) error {
	enterScope(c, c.ScopeTree)
	defer leaveScope(c)

	for _, dcl := range gl.Declarations {
		if e := clGlobalBody(c, dcl); e != nil {
			return e
		}
	}
	return nil
}

func unresolvedGlobalTypes(files []*t.FileCtx) int {
	unresolved := 0
	for _, file := range files {
		for _, declaration := range file.GlNode.Declarations {
			switch node := declaration.(type) {
			case *t.NodeExprVarDef:
				if node.Type == nil {
					unresolved++
				}
			case *t.NodeConstDef:
				if node.VarDef.Type == nil {
					unresolved++
				}
			}
		}
	}
	return unresolved
}

func firstUnresolvedGlobal(files []*t.FileCtx) (*t.FileCtx, *t.Token, string) {
	for _, file := range files {
		for _, declaration := range file.GlNode.Declarations {
			switch node := declaration.(type) {
			case *t.NodeExprVarDef:
				if node.Type == nil {
					return file, lastNameToken(node.Name), flattenName(node.Name)
				}
			case *t.NodeConstDef:
				if node.VarDef.Type == nil {
					return file, lastNameToken(node.VarDef.Name), flattenName(node.VarDef.Name)
				}
			}
		}
	}
	return &t.FileCtx{}, &t.Token{Pos: t.FilePos{Line: 1, Col: 1}}, "<unknown>"
}

// dependencyOrderedFiles keeps the historical dependency-before-importer
// behavior for acyclic graphs while tolerating back-edges. Interfaces are
// resolved in a separate whole-program pass, so either order inside a strongly
// connected component is valid; sorting roots and edges makes it reproducible.
func dependencyOrderedFiles(files map[string]*t.FileCtx) []*t.FileCtx {
	byPath := make(map[string]*t.FileCtx, len(files))
	paths := make([]string, 0, len(files))
	for path, file := range files {
		clean := filepath.Clean(path)
		byPath[clean] = file
		paths = append(paths, clean)
	}
	sort.Strings(paths)

	state := map[*t.FileCtx]uint8{}
	ordered := make([]*t.FileCtx, 0, len(files))
	var visit func(*t.FileCtx)
	visit = func(file *t.FileCtx) {
		if file == nil || state[file] == 2 {
			return
		}
		if state[file] == 1 {
			return
		}
		state[file] = 1
		imports := append([]string(nil), file.Imports...)
		for i := range imports {
			imports[i] = filepath.Clean(imports[i])
		}
		sort.Strings(imports)
		for _, path := range imports {
			visit(byPath[path])
		}
		state[file] = 2
		ordered = append(ordered, file)
	}
	for _, path := range paths {
		visit(byPath[path])
	}
	return ordered
}

type constantEntry struct {
	file       *t.FileCtx
	definition *t.NodeConstDef
}

// checkConstantInitializerCycles protects both semantic checking and LLVM
// constant expansion, which recursively follow references to other constants.
// Import DAGs used to make cross-module cycles impossible; accepting import
// SCCs means the declaration graph itself must now carry that validation.
func checkConstantInitializerCycles(files []*t.FileCtx) error {
	constants := map[*t.NodeExprVarDef]constantEntry{}
	constantOrder := []*t.NodeExprVarDef{}
	for _, file := range files {
		for _, declaration := range file.GlNode.Declarations {
			if definition, ok := declaration.(*t.NodeConstDef); ok {
				constants[definition.VarDef] = constantEntry{file: file, definition: definition}
				constantOrder = append(constantOrder, definition.VarDef)
			}
		}
	}

	visiting := map[*t.NodeExprVarDef]bool{}
	visited := map[*t.NodeExprVarDef]bool{}
	var visitConstant func(*t.NodeExprVarDef) error
	var visitExpression func(t.NodeExpr) error
	visitExpression = func(expression t.NodeExpr) error {
		switch node := expression.(type) {
		case *t.NodeExprName:
			if variable, ok := node.AssociatedNode.(*t.NodeExprVarDef); ok {
				if _, isConstant := constants[variable]; isConstant {
					return visitConstant(variable)
				}
			}
		case *t.NodeExprArray:
			if err := visitExpression(node.Length); err != nil {
				return err
			}
			for _, item := range node.Entries {
				if item.Index != nil {
					if err := visitExpression(item.Index); err != nil {
						return err
					}
				}
				if err := visitExpression(item.Value); err != nil {
					return err
				}
			}
		case *t.NodeExprStructInit:
			for _, field := range node.Fields {
				if err := visitExpression(field.Expression); err != nil {
					return err
				}
			}
		}
		return nil
	}
	visitConstant = func(variable *t.NodeExprVarDef) error {
		if visiting[variable] {
			entry := constants[variable]
			return comp_err.CompilationErrorToken(entry.file, &entry.definition.Tk, fmt.Sprintf("constant initializer cycle involving '%s'", flattenName(variable.Name)), "break the cycle by replacing one reference with a literal value")
		}
		if visited[variable] {
			return nil
		}
		entry, ok := constants[variable]
		if !ok {
			return nil
		}
		visiting[variable] = true
		if err := visitExpression(entry.definition.Initializer); err != nil {
			return err
		}
		delete(visiting, variable)
		visited[variable] = true
		return nil
	}

	for _, variable := range constantOrder {
		if err := visitConstant(variable); err != nil {
			return err
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

	corePath := ""
	if s.StdRoot != "" {
		corePath = filepath.Clean(filepath.Join(s.StdRoot, "core.mg"))
	}

	for _, v := range s.Files {
		ctx.ModuleBundle.Modules[v.PackageName] = v.GlNode
		if (corePath != "" && filepath.Clean(v.FilePath) == corePath) || (corePath == "" && v.ModuleName == "core") {
			ctx.CoreGlobal = v.GlNode
		}
		for primitive, methods := range v.GlNode.PrimitiveMethods {
			for name, function := range methods {
				key := primitive + "." + name
				if _, exists := ctx.PrimitiveMethods[key]; exists {
					return comp_err.CompilationErrorToken(v, lastNameToken(function.Class.NameNode), fmt.Sprintf("primitive method '%s' is defined more than once", key), "primitive methods must have a single definition in the program")
				}
				ctx.PrimitiveMethods[key] = primitiveMethod{Function: function, Module: v.PackageName}
			}
		}
	}
	if ctx.CoreGlobal != nil {
		s.CoreTypes = make(map[t.CoreTypeRole]*t.StructDef, 3)
		s.CoreMethods = make(map[string]*t.NodeFuncDef)
		for _, definition := range ctx.CoreGlobal.StructDefs {
			if definition.CoreRole != t.CoreTypeNone {
				s.CoreTypes[definition.CoreRole] = definition
			}
		}
		for primitive, methods := range ctx.CoreGlobal.PrimitiveMethods {
			for name, function := range methods {
				s.CoreMethods[primitive+"."+name] = function
			}
		}
	}

	// Parsing has already populated every module's declaration tables. Resolve
	// interfaces for the complete program before linking implementations. This
	// is the semantic equivalent of forward declarations and permits import
	// strongly-connected components without splitting the generated program.
	ordered := dependencyOrderedFiles(s.Files)

	for _, fCtx := range ordered {
		n := fCtx.GlNode
		ctx.GlobalNode = n
		ctx.ScopeTree = &fCtx.ScopeTree
		ctx.FileCtx = fCtx
		if e := clGlobalInterface(ctx, n); e != nil {
			return comp_err.EnsureDiagnostic(fCtx, &t.Token{Pos: t.FilePos{Line: 1, Col: 1}}, e)
		}
	}

	// Module-level inferred values can depend on values in another member of an
	// import cycle. Revisit them until their types stabilize. A nil result is
	// intentionally not progress; it represents a declaration whose dependency
	// has not acquired a type yet.
	previousUnresolved := -1
	pendingErrors := map[*t.FileCtx]error{}
	for unresolved := unresolvedGlobalTypes(ordered); unresolved > 0 && unresolved != previousUnresolved; unresolved = unresolvedGlobalTypes(ordered) {
		previousUnresolved = unresolved
		for _, fCtx := range ordered {
			ctx.GlobalNode = fCtx.GlNode
			ctx.ScopeTree = &fCtx.ScopeTree
			ctx.FileCtx = fCtx
			if e := clGlobalValues(ctx, fCtx.GlNode); e != nil {
				pendingErrors[fCtx] = e
			} else {
				delete(pendingErrors, fCtx)
			}
		}
	}
	if unresolved := unresolvedGlobalTypes(ordered); unresolved > 0 {
		for _, file := range ordered {
			if err := pendingErrors[file]; err != nil {
				return comp_err.EnsureDiagnostic(file, &t.Token{Pos: t.FilePos{Line: 1, Col: 1}}, err)
			}
		}
		file, token, name := firstUnresolvedGlobal(ordered)
		return comp_err.CompilationErrorToken(file, token, fmt.Sprintf("cannot infer cyclic module-level declaration '%s'", name), "add an explicit type to at least one declaration in the inference cycle")
	}

	// Explicitly typed values were not part of the inference worklist, but their
	// initializers still need name linking and validation once interfaces exist.
	for _, fCtx := range ordered {
		if fCtx.InterfaceOnly {
			continue
		}
		ctx.GlobalNode = fCtx.GlNode
		ctx.ScopeTree = &fCtx.ScopeTree
		ctx.FileCtx = fCtx
		if e := clGlobalValues(ctx, fCtx.GlNode); e != nil {
			return comp_err.EnsureDiagnostic(fCtx, &t.Token{Pos: t.FilePos{Line: 1, Col: 1}}, e)
		}
	}
	if err := checkConstantInitializerCycles(ordered); err != nil {
		return err
	}

	for _, fCtx := range ordered {
		if fCtx.InterfaceOnly {
			continue
		}
		ctx.GlobalNode = fCtx.GlNode
		ctx.ScopeTree = &fCtx.ScopeTree
		ctx.FileCtx = fCtx
		if e := clGlobalBodies(ctx, fCtx.GlNode); e != nil {
			return comp_err.EnsureDiagnostic(fCtx, &t.Token{Pos: t.FilePos{Line: 1, Col: 1}}, e)
		}
	}

	return nil
}

func firstFileByPath(files map[string]*t.FileCtx, include func(*t.FileCtx) bool) *t.FileCtx {
	paths := make([]string, 0, len(files))
	for path, file := range files {
		if include == nil || include(file) {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return &t.FileCtx{}
	}
	return files[paths[0]]
}
