package parser

import (
	"Magma/src/comp_err"
	magmatypes "Magma/src/magma_types"
	t "Magma/src/types"
	"errors"
	"fmt"
	"slices"
	"strings"
)

func ensureSimpleName(ctx *ParseCtx, tk t.Token, name t.NodeName) error {
	switch n := name.(type) {
	case *t.NodeNameComposite:
		// TODO: associate nodes with tokens for better error reporting
		return comp_err.CompilationErrorToken(
			ctx.Fctx,
			&tk,
			fmt.Sprintf("syntax error: complex name: '%s' not allowed in this context, expected simple name", strings.Join(n.Parts, ".")),
			"cannot define a struct with a name containing a '.'",
		)
	}
	return nil
}

func parseStructDef(ctx *ParseCtx, tk t.Token, gncls t.NodeGenericClass) (*t.NodeStructDef, error) {
	if ctx.PruneNext {
		return nil, nil
	}

	// check if struct name is valid (complex name not allowed)
	e := ensureSimpleName(ctx, tk, gncls.NameNode)
	if e != nil {
		return nil, e
	}

	simpleName := gncls.NameNode.(*t.NodeNameSingle)
	if _, exists := ctx.GlobalNode.TypeAliases[simpleName.Name]; exists {
		return nil, comp_err.CompilationErrorToken(ctx.Fctx, &simpleName.Tk, fmt.Sprintf("type name '%s' is already declared as an alias", simpleName.Name), "type aliases and structs share a namespace")
	}

	// create struct def in global node for easir type checking later
	structMap := &t.StructDef{
		Module:     ctx.Fctx.PackageName,
		Name:       simpleName.Name,
		TypeParams: gncls.TypeParams,
		Fields:     map[string]*t.NodeType{},
		Funcs:      map[string]*t.NodeFuncDef{},
		FieldNb:    map[string]int{},
		FieldOrder: []string{},
	}

	for i, arg := range gncls.ArgsNode.Args {
		if _, exists := structMap.Fields[arg.Name]; exists {
			return nil, comp_err.CompilationErrorToken(
				ctx.Fctx,
				&arg.Tk,
				fmt.Sprintf("duplicate field '%s' in struct '%s'", arg.Name, simpleName.Name),
				"field names within a struct must be unique",
			)
		}
		structMap.Fields[arg.Name] = arg.TypeNode
		structMap.FieldNb[arg.Name] = i
		structMap.FieldOrder = append(structMap.FieldOrder, arg.Name)
	}

	ctx.GlobalNode.StructDefs[simpleName.Name] = structMap

	return &t.NodeStructDef{
		Class:   gncls,
		AbsName: ctx.Fctx.PackageName + "." + simpleName.Name,
	}, nil
}

func syntheticNamed(name string) *t.NodeType {
	return &t.NodeType{KindNode: &t.NodeTypeNamed{NameNode: &t.NodeNameSingle{Name: name}}}
}

func parseProtoDef(ctx *ParseCtx, protoTk t.Token) (t.NodeGlobalDecl, error) {
	modifiers := slices.Clone(ctx.NextModifiers)
	ctx.NextModifiers = []ModifierType{}
	consume(ctx) // proto
	decl, err := parseDeclNameWithGenerics(ctx)
	if err != nil {
		return nil, err
	}
	name, ok := decl.NameNode.(*t.NodeNameSingle)
	if !ok {
		return nil, comp_err.CompilationErrorToken(ctx.Fctx, &protoTk, "prototype names must be simple", "")
	}
	if len(decl.TypeParams) != 0 {
		return nil, comp_err.CompilationErrorToken(ctx.Fctx, &name.Tk, "generic prototype declarations are not yet supported", "ordinary methods declared on a prototype may still be generic")
	}
	if _, exists := ctx.GlobalNode.StructDefs[name.Name]; exists {
		return nil, comp_err.CompilationErrorToken(ctx.Fctx, &name.Tk, fmt.Sprintf("type '%s' is already declared", name.Name), "")
	}
	if _, exists := ctx.GlobalNode.TypeAliases[name.Name]; exists {
		return nil, comp_err.CompilationErrorToken(ctx.Fctx, &name.Tk, fmt.Sprintf("type '%s' is already declared as an alias", name.Name), "")
	}
	impls := []*t.ProtoImpl{}
	open, err := peek(ctx)
	if err == nil && open.Type == t.TokName && open.Repr == "impl" {
		consume(ctx)
		for {
			current, currentErr := peek(ctx)
			if currentErr != nil {
				return nil, currentErr
			}
			if current.KeywType == t.KwParenOp {
				open = current
				break
			}
			implementedType, typeErr := parseType(ctx, current, false)
			if typeErr != nil {
				return nil, typeErr
			}
			impls = append(impls, &t.ProtoImpl{Type: implementedType, Tk: current})
		}
	}
	if err != nil || open.KeywType != t.KwParenOp {
		return nil, comp_err.CompilationErrorToken(ctx.Fctx, &name.Tk, "prototype declaration requires a method list", "expected: `proto Name(method(args) Return)`")
	}
	consume(ctx)
	proto := &t.ProtoDef{Module: ctx.Fctx.PackageName, Name: name.Name, IsPublic: slices.Contains(modifiers, MdPublic), TypeParams: decl.TypeParams, MethodMap: map[string]*t.ProtoMethod{}, VtableName: "__proto_" + name.Name + "_vtable"}
	for {
		tk, e := peek(ctx)
		if e != nil {
			return nil, e
		}
		if tk.KeywType == t.KwNewline || tk.KeywType == t.KwComma {
			consume(ctx)
			continue
		}
		if tk.KeywType == t.KwParenCl {
			consume(ctx)
			break
		}
		if tk.Type != t.TokName {
			return nil, comp_err.CompilationErrorToken(ctx.Fctx, &tk, "expected prototype method name", "")
		}
		consume(ctx)
		if next, _ := peek(ctx); next.KeywType == t.KwBrackOp {
			return nil, comp_err.CompilationErrorToken(ctx.Fctx, &tk, "prototype requirements cannot declare method-specific generic parameters", "declare a generic ordinary method on the prototype instead")
		}
		args, e := parseArgsList(ctx)
		if e != nil {
			return nil, e
		}
		retTk, e := peek(ctx)
		if e != nil {
			return nil, e
		}
		ret, e := parseType(ctx, retTk, true)
		if e != nil {
			return nil, e
		}
		if _, duplicate := proto.MethodMap[tk.Repr]; duplicate {
			return nil, comp_err.CompilationErrorToken(ctx.Fctx, &tk, fmt.Sprintf("duplicate prototype method '%s'", tk.Repr), "")
		}
		method := &t.ProtoMethod{Name: tk.Repr, Args: args.Args, Ret: ret, Slot: len(proto.Methods), Tk: tk}
		method.Proto = proto
		proto.Methods = append(proto.Methods, method)
		proto.MethodMap[method.Name] = method
	}

	vtArgs := make([]t.NodeArg, 0, len(proto.Methods))
	for _, method := range proto.Methods {
		fnArgs := []*t.NodeType{syntheticNamed("ptr")}
		for _, arg := range method.Args {
			fnArgs = append(fnArgs, arg.TypeNode)
		}
		vtArgs = append(vtArgs, t.NodeArg{Name: method.Name, Tk: method.Tk, TypeNode: &t.NodeType{KindNode: &t.NodeTypeFunc{Args: fnArgs, RetType: method.Ret}}})
	}
	vtName := &t.NodeNameSingle{Name: proto.VtableName, Tk: name.Tk}
	vtClass := t.NodeGenericClass{NameNode: vtName, ArgsNode: t.NodeArgList{Args: vtArgs}}
	vtNode, e := parseStructDef(ctx, name.Tk, vtClass)
	if e != nil {
		return nil, e
	}
	ctx.GlobalNode.Declarations = append(ctx.GlobalNode.Declarations, vtNode)

	protoArgs := []t.NodeArg{{Name: "impl", TypeNode: syntheticNamed("ptr")}, {Name: "vtable", TypeNode: &t.NodeType{KindNode: &t.NodeTypePointer{Kind: &t.NodeTypeNamed{NameNode: vtName}}}}}
	protoClass := t.NodeGenericClass{NameNode: name, TypeParams: decl.TypeParams, ArgsNode: t.NodeArgList{Args: protoArgs}}
	protoNode, e := parseStructDef(ctx, name.Tk, protoClass)
	if e != nil {
		return nil, e
	}
	protoNode.IsPublic = proto.IsPublic
	def := ctx.GlobalNode.StructDefs[name.Name]
	def.IsPublic = proto.IsPublic
	def.IsProto = true
	def.Proto = proto
	def.Implements = impls
	ctx.GlobalNode.ProtoDefs[name.Name] = proto

	for _, method := range proto.Methods {
		owner := &t.NodeTypeNamed{NameNode: &t.NodeNameSingle{Name: name.Name}}
		thisType := &t.NodeType{KindNode: &t.NodeTypePointer{Kind: owner}}
		args := []t.NodeArg{{Name: "this", TypeNode: thisType}}
		args = append(args, method.Args...)
		fn := &t.NodeFuncDef{Class: t.NodeGenericClass{NameNode: &t.NodeNameComposite{Parts: []string{name.Name, method.Name}, Tokens: []t.Token{name.Tk, method.Tk}}, ArgsNode: t.NodeArgList{Args: args}, OwnerTypeParams: decl.TypeParams}, ReturnType: method.Ret, AbsName: ctx.Fctx.PackageName + "." + name.Name + "." + method.Name, IsMember: true, ProtoDispatch: method}
		method.FnDef = fn
		def.Funcs[method.Name] = fn
		ctx.GlobalNode.FuncDefs[name.Name+"."+method.Name] = fn
		ctx.GlobalNode.Declarations = append(ctx.GlobalNode.Declarations, fn)
	}
	return protoNode, nil
}

func parseAliasDecl(ctx *ParseCtx, aliasTk t.Token) (t.NodeGlobalDecl, error) {
	pruned := ctx.PruneNext
	ctx.PruneNext = false
	modifiers := slices.Clone(ctx.NextModifiers)
	ctx.NextModifiers = []ModifierType{}
	if slices.Contains(modifiers, MdDestructor) {
		return nil, comp_err.CompilationErrorToken(ctx.Fctx, &aliasTk, "destructor modifier cannot be applied to a type alias", "")
	}
	consume(ctx)
	nameTk, e := peek(ctx)
	if e != nil || nameTk.Type != t.TokName {
		return nil, comp_err.CompilationErrorToken(ctx.Fctx, &aliasTk, "expected a name after 'alias'", "expected: `alias name = Type`")
	}
	consume(ctx)
	equalTk, e := peek(ctx)
	if e != nil || equalTk.KeywType != t.KwEqual {
		return nil, comp_err.CompilationErrorToken(ctx.Fctx, &nameTk, "type alias is missing '='", "expected: `alias name = Type`")
	}
	consume(ctx)
	typeTk, e := peek(ctx)
	if e != nil {
		return nil, e
	}
	var target *t.NodeType
	if typeTk.KeywType == t.KwAt {
		target, e = parseCompilerKnownType(ctx, typeTk)
	} else {
		target, e = parseType(ctx, typeTk, false)
	}
	if e != nil {
		return nil, e
	}
	if _, exists := ctx.GlobalNode.TypeAliases[nameTk.Repr]; exists {
		return nil, comp_err.CompilationErrorToken(ctx.Fctx, &nameTk, fmt.Sprintf("type alias '%s' is already declared", nameTk.Repr), "alias names must be unique within a module")
	}
	if _, exists := ctx.GlobalNode.StructDefs[nameTk.Repr]; exists {
		return nil, comp_err.CompilationErrorToken(ctx.Fctx, &nameTk, fmt.Sprintf("type name '%s' is already declared as a struct", nameTk.Repr), "type aliases and structs share a namespace")
	}
	alias := &t.TypeAlias{Name: nameTk.Repr, Module: ctx.Fctx.PackageName, Target: target, IsPublic: slices.Contains(modifiers, MdPublic), Tk: nameTk}
	if pruned {
		return nil, nil
	}
	ctx.GlobalNode.TypeAliases[nameTk.Repr] = alias
	return &t.NodeTypeAlias{Alias: alias}, nil
}

func parseCompilerKnownType(ctx *ParseCtx, atTk t.Token) (*t.NodeType, error) {
	consume(ctx)
	nameTk, e := peek(ctx)
	if e != nil || nameTk.Type != t.TokName || nameTk.Repr != "compiler_known_type" {
		return nil, comp_err.CompilationErrorToken(ctx.Fctx, &atTk, "unknown compiler type directive", "expected: `@compiler_known_type(\"name\")`")
	}
	consume(ctx)
	openTk, e := peek(ctx)
	if e != nil || openTk.KeywType != t.KwParenOp {
		return nil, comp_err.CompilationErrorToken(ctx.Fctx, &nameTk, "compiler_known_type is missing '('", "expected: `@compiler_known_type(\"name\")`")
	}
	consume(ctx)
	valueTk, e := peek(ctx)
	if e != nil || valueTk.Type != t.TokLitStr {
		return nil, comp_err.CompilationErrorToken(ctx.Fctx, &nameTk, "compiler_known_type requires one string argument", "expected: `@compiler_known_type(\"name\")`")
	}
	consume(ctx)
	closeTk, e := peek(ctx)
	if e != nil || closeTk.KeywType != t.KwParenCl {
		return nil, comp_err.CompilationErrorToken(ctx.Fctx, &valueTk, "compiler_known_type is missing ')'", "expected: `@compiler_known_type(\"name\")`")
	}
	consume(ctx)
	return parseTypePostfix(ctx, &t.NodeType{KindNode: &t.NodeTypeCompilerKnown{Tk: valueTk, Name: valueTk.Repr}})
}

func parseFuncDef(ctx *ParseCtx, nameTk t.Token, after t.Token, gncls t.NodeGenericClass, alias string) (*t.NodeFuncDef, error) {
	isMemberFunc := false
	fnNameSimple := ""

	switch n := gncls.NameNode.(type) {
	case *t.NodeNameComposite:
		isMemberFunc = true
		fnNameSimple = strings.Join(n.Parts, ".")
	case *t.NodeNameSingle:
		fnNameSimple = n.Name
	}

	fnDef := &t.NodeFuncDef{
		Class:        gncls,
		IsMember:     isMemberFunc && alias == "",
		IsEntryPoint: !isMemberFunc && alias == "" && fnNameSimple == "main",
		IsExternal:   alias != "",
		NoRetain:     ctx.NextNoRetain,
	}
	ctx.NextNoRetain = false
	if ctx.NextExportName != "" {
		if alias != "" {
			return nil, comp_err.CompilationErrorToken(ctx.Fctx, &nameTk, "syntax error: 'export_name' cannot be applied to an external declaration", "apply it to a function definition")
		}
		if isMemberFunc {
			return nil, comp_err.CompilationErrorToken(ctx.Fctx, &nameTk, "syntax error: 'export_name' cannot be applied to a member function", "export a top-level, non-generic function")
		}
		if len(gncls.TypeParams) != 0 {
			return nil, comp_err.CompilationErrorToken(ctx.Fctx, &nameTk, "syntax error: 'export_name' cannot be applied to a generic function", "export a top-level, non-generic function")
		}
		fnDef.ExportName = ctx.NextExportName
		fnDef.ExportABI = ctx.NextExportABI
		ctx.NextExportName = ""
		ctx.NextExportABI = ""
	}

	if alias != "" {
		fnDef.NoAliasName = flattenName(gncls.NameNode)
		aliasedNameNode := &t.NodeNameSingle{Name: alias}
		fnDef.Class.NameNode = aliasedNameNode
		fnNameSimple = aliasedNameNode.Name
	}

	ctx.CurrentFunction = fnDef
	defer func() {
		ctx.CurrentFunction = nil
	}()

	typeNode, e := parseType(ctx, after, true)
	if e != nil {
		return nil, e
	}

	if alias == "" {
		bodyStart, e := peek(ctx)
		if e != nil {
			return nil, e
		}

		bodyNode, e := parseBody(ctx, bodyStart)
		if e != nil {
			return nil, e
		}
		fnDef.Body = bodyNode
	}

	fnDef.ReturnType = typeNode
	if fnDef.ExportName != "" && typeNode.Throws {
		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&nameTk,
			"syntax error: throwing functions cannot be exported",
			"use a non-throwing ABI wrapper that converts the error to a C-compatible result",
		)
	}
	fnDef.AbsName = ctx.Fctx.PackageName + "." + flattenName(gncls.NameNode)

	// =========================================================================
	// pruning should result in NO SIDE EFFECT
	// section beyond this point is basically all side effects and nothing else
	if ctx.PruneNext {
		return nil, nil
	}
	// =========================================================================
	if fnDef.ExportName != "" {
		ctx.Shared.ExportedSymbolsM.Lock()
		if ctx.Shared.ExportedSymbols == nil {
			ctx.Shared.ExportedSymbols = map[string]string{}
		}
		previous, duplicate := ctx.Shared.ExportedSymbols[fnDef.ExportName]
		if !duplicate {
			ctx.Shared.ExportedSymbols[fnDef.ExportName] = ctx.Fctx.FilePath + ":" + fnNameSimple
		}
		ctx.Shared.ExportedSymbolsM.Unlock()
		if duplicate {
			return nil, comp_err.CompilationErrorToken(
				ctx.Fctx,
				&nameTk,
				fmt.Sprintf("duplicate exported symbol name '%s'", fnDef.ExportName),
				fmt.Sprintf("the symbol was already exported by %s", previous),
			)
		}
	}

	if isMemberFunc && alias == "" { // alias == "" since aliased functions cannot be also member funcs
		complexName := gncls.NameNode.(*t.NodeNameComposite)
		if len(complexName.Parts) > 2 {
			return nil, comp_err.CompilationErrorToken(
				ctx.Fctx,
				&nameTk,
				fmt.Sprintf("syntax error: too many parts in complex name: '%s' a function definition should have 1 or 2 parts, no more", strings.Join(complexName.Parts, ".")),
				"expected: `<name> (<args>) <type>:` or `<structname>.<name> (<args>) <type>:` ",
			)
		}

		ownerName := complexName.Parts[0]
		memberName := complexName.Parts[1]

		ownerStruct, isStruct := ctx.GlobalNode.StructDefs[ownerName]
		_, isPrimitive := magmatypes.BasicTypes[ownerName]
		if !isStruct && !isPrimitive {
			return nil, comp_err.CompilationErrorToken(
				ctx.Fctx,
				&nameTk,
				fmt.Sprintf("syntax error: defined member function for '%s', but the struct was not defined in this file", ownerName),
				"member functions need a built-in owner type or a struct defined earlier in the same file",
			)
		}

		if isPrimitive && len(fnDef.Class.OwnerTypeParams) != 0 {
			return nil, comp_err.CompilationErrorToken(
				ctx.Fctx,
				&nameTk,
				fmt.Sprintf("primitive owner '%s' does not take generic parameters", ownerName),
				fmt.Sprintf("declare the member as `%s.%s[...]`, without parameters on the owner", ownerName, memberName),
			)
		}
		if isStruct {
			expected := ownerStruct.TypeParams
			actual := fnDef.Class.OwnerTypeParams
			if !slices.Equal(expected, actual) {
				return nil, comp_err.CompilationErrorToken(
					ctx.Fctx,
					&nameTk,
					fmt.Sprintf("generic parameters on member owner '%s' do not match its declaration", ownerName),
					fmt.Sprintf("use `%s[%s].%s` to match the owner declaration", ownerName, strings.Join(expected, ", "), memberName),
				)
			}
		}
		ownerParams := map[string]bool{}
		for _, parameter := range fnDef.Class.OwnerTypeParams {
			ownerParams[parameter] = true
		}
		for _, parameter := range fnDef.Class.TypeParams {
			if ownerParams[parameter] {
				return nil, comp_err.CompilationErrorToken(
					ctx.Fctx,
					&nameTk,
					fmt.Sprintf("generic member parameter '%s' duplicates an owner parameter", parameter),
					"member-specific generic parameters must use names distinct from the owner's parameters",
				)
			}
		}

		//fmt.Printf("added implicit this to: %s.%s()\n", ownerName, memberName)

		thisOwnerNamed := &t.NodeTypeNamed{
			NameNode: &t.NodeNameSingle{Name: ownerName},
		}
		if len(fnDef.Class.OwnerTypeParams) > 0 {
			typeArgs := make([]*t.NodeType, 0, len(fnDef.Class.OwnerTypeParams))
			for _, p := range fnDef.Class.OwnerTypeParams {
				typeArgs = append(typeArgs, &t.NodeType{
					KindNode: &t.NodeTypeNamed{
						NameNode: &t.NodeNameSingle{Name: p},
					},
				})
			}
			thisOwnerNamed.GenericArgs = typeArgs
		}

		fnDef.Class.ArgsNode.Args = slices.Insert(fnDef.Class.ArgsNode.Args, 0, t.NodeArg{
			Name: "this",
			TypeNode: &t.NodeType{
				KindNode: &t.NodeTypePointer{
					Kind: thisOwnerNamed,
				},
			},
		})

		if isStruct {
			ownerStruct.Funcs[memberName] = fnDef
		} else {
			if ctx.Fctx.ModuleName == "core" && ownerName == "error" {
				switch memberName {
				case "ok":
					fnDef.ErrorPredicate = t.ErrorPredicateOk
				case "nok":
					fnDef.ErrorPredicate = t.ErrorPredicateNok
				}
			}
			methods := ctx.GlobalNode.PrimitiveMethods[ownerName]
			if methods == nil {
				methods = map[string]*t.NodeFuncDef{}
				ctx.GlobalNode.PrimitiveMethods[ownerName] = methods
			}
			methods[memberName] = fnDef
		}

		/* DEPRECATED: Destructors will not be implemented in language.
		if memberName == "destructor" {

			if len(fnDef.Class.ArgsNode.Args) > 1 {
				return nil, comp_err.CompilationErrorToken(
					ctx.Fctx,
					&nameTk,
					fmt.Sprintf("syntax error: destructor function for '%s' cannot have any defined arguments", ownerName),
					fmt.Sprintf("signature of destructor should be: `%s.destructor() void`", ownerName),
				)
			}
			// TODO: enforce 0 args and non-throwing void type
			ctx.GlobalNode.StructDefs[ownerName].Destructor = fnDef
			// Destructor discovery is intentionally silent; callers can inspect the AST in debug mode.
		}*/
	}

	ctx.GlobalNode.FuncDefs[fnNameSimple] = fnDef
	return fnDef, nil
}

func parseExternalFunc(ctx *ParseCtx, tk t.Token) (t.NodeGlobalDecl, error) {
	consume(ctx) // consume "extern"

	nAlias, e := parseName(ctx, tk, false)
	if e != nil {
		return nil, e
	}

	next, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	if next.Type != t.TokName {
		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&next,
			fmt.Sprintf("syntax error: expected external function name after alias but got '%s'", next.Repr),
			"expected: `extern <alias> <actual name> (<args>) <return type>`",
		)
	}

	n, e := parseName(ctx, next, false)
	if e != nil {
		return nil, e
	}

	next, e = peek(ctx)
	if e != nil {
		return nil, e
	}

	if next.Type != t.TokKeyword {
		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&next,
			fmt.Sprintf("syntax error: unexpected '%s' after name in extern function declaration", next.Repr),
			"expected: `extern <name> (`",
		)
	}

	gncls, e := parseGenericClass(ctx, n, nil, nil)
	if e != nil {
		return nil, e
	}

	after, e := peek(ctx)
	if e != nil && !errors.Is(e, errOutOfBounds) {
		return nil, e
	}

	if errors.Is(e, errOutOfBounds) || after.KeywType == t.KwNewline {
		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&next,
			fmt.Sprintf("syntax error: unexpected '%s' after argument list in external function declaration", next.Repr),
			"expected: `extern <name> (<args>) <return type>",
		)
	}

	alias := flattenName(nAlias)
	return parseFuncDef(ctx, tk, after, gncls, alias)
}

func parseGlobalDeclFromName(ctx *ParseCtx, tk t.Token) (t.NodeGlobalDecl, error) {
	modifiers := slices.Clone(ctx.NextModifiers)
	ctx.NextModifiers = []ModifierType{}
	declName, e := parseDeclNameWithGenerics(ctx)
	if e != nil {
		return nil, e
	}

	next, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	if next.Type == t.TokName && next.Repr == "impl" {
		consume(ctx)
		impls := []*t.ProtoImpl{}
		for {
			current, err := peek(ctx)
			if err != nil {
				return nil, err
			}
			if current.KeywType == t.KwParenOp {
				break
			}
			protoType, err := parseType(ctx, current, false)
			if err != nil {
				return nil, err
			}
			impls = append(impls, &t.ProtoImpl{Type: protoType, Tk: current})
		}
		gncls, err := parseGenericClass(ctx, declName.NameNode, declName.TypeParams, declName.OwnerTypeParams)
		if err != nil {
			return nil, err
		}
		after, err := peek(ctx)
		if err != nil && !errors.Is(err, errOutOfBounds) {
			return nil, err
		}
		if err == nil && after.KeywType != t.KwNewline {
			return nil, comp_err.CompilationErrorToken(ctx.Fctx, &after, "implementation declaration must be a struct", "")
		}
		st, err := parseStructDef(ctx, tk, gncls)
		if err != nil {
			return nil, err
		}
		if st != nil {
			st.IsPublic = slices.Contains(modifiers, MdPublic)
			def := ctx.GlobalNode.StructDefs[declName.NameNode.(*t.NodeNameSingle).Name]
			def.IsPublic = st.IsPublic
			def.Implements = impls
		}
		return st, nil
	}

	switch next.KeywType {
	case t.KwParenOp:
		// A function type starts with the same token as a function declaration.
		// Prefer a global variable when the complete type is followed by a newline
		// or initializer.
		startIdx := ctx.TokIdx
		if typeNode, typeErr := parseType(ctx, next, false); typeErr == nil {
			afterType, afterErr := peek(ctx)
			if afterErr == nil && (afterType.KeywType == t.KwNewline || afterType.KeywType == t.KwEqual) {
				variable := &t.NodeExprVarDef{
					Name: declName.NameNode, AbsName: ctx.Fctx.PackageName + "." + flattenName(declName.NameNode),
					Type: typeNode, IsGlobal: true, IsPublic: slices.Contains(modifiers, MdPublic), Storage: t.VariableStorageGlobal,
				}
				if afterType.KeywType == t.KwEqual {
					consume(ctx)
					first, err := peek(ctx)
					if err != nil {
						return nil, err
					}
					variable.Initializer, err = parseExpression(ctx, first, 0)
					if err != nil {
						return nil, err
					}
				}
				return variable, nil
			}
		}
		ctx.TokIdx = startIdx

		gncls, e := parseGenericClass(ctx, declName.NameNode, declName.TypeParams, declName.OwnerTypeParams)
		if e != nil {
			return nil, e
		}

		after, e := peek(ctx)
		if e != nil && !errors.Is(e, errOutOfBounds) {
			return nil, e
		}

		if errors.Is(e, errOutOfBounds) || after.KeywType == t.KwNewline {
			if slices.Contains(modifiers, MdDestructor) {
				return nil, comp_err.CompilationErrorToken(ctx.Fctx, &tk, "syntax error: destructor modifier requires a member function", "expected: `destructor Type.method() void:`")
			}
			st, e := parseStructDef(ctx, tk, gncls)
			if e == nil && st != nil {
				st.IsPublic = slices.Contains(modifiers, MdPublic)
				name := st.Class.NameNode.(*t.NodeNameSingle).Name
				ctx.GlobalNode.StructDefs[name].IsPublic = st.IsPublic
			}
			return st, e
		}

		fn, e := parseFuncDef(ctx, tk, after, gncls, "")
		if e != nil {
			return nil, e
		}
		if slices.Contains(modifiers, MdDestructor) {
			name, ok := fn.Class.NameNode.(*t.NodeNameComposite)
			if !ok || len(name.Parts) != 2 {
				return nil, comp_err.CompilationErrorToken(ctx.Fctx, &tk, "syntax error: destructor must be a struct member function", "expected: `destructor Type.method() void:`")
			}
			fn.IsDestructor = true
			ownerName := name.Parts[0]
			if owner := ctx.GlobalNode.StructDefs[ownerName]; owner != nil {
				owner.Destructors = append(owner.Destructors, fn)
				if owner.Destructor == nil {
					owner.Destructor = fn
				}
			} else if _, ok := magmatypes.BasicTypes[ownerName]; ok {
				ctx.GlobalNode.PrimitiveDestructors[ownerName] = append(ctx.GlobalNode.PrimitiveDestructors[ownerName], fn)
			} else {
				return nil, comp_err.CompilationErrorToken(ctx.Fctx, &tk, fmt.Sprintf("unknown destructor owner type '%s'", ownerName), "")
			}
		}
		if fn != nil {
			fn.IsPublic = slices.Contains(modifiers, MdPublic)
		}
		return fn, nil
	case t.KwInfer:
		if len(declName.TypeParams) > 0 || len(declName.OwnerTypeParams) > 0 {
			return nil, comp_err.CompilationErrorToken(ctx.Fctx, &next, "syntax error: generic parameters are only valid on struct/function declarations", "")
		}
		consume(ctx)
		first, err := peek(ctx)
		if err != nil {
			return nil, err
		}
		initializer, err := parseExpression(ctx, first, 0)
		if err != nil {
			return nil, err
		}
		return &t.NodeExprVarDef{
			Name: declName.NameNode, AbsName: ctx.Fctx.PackageName + "." + flattenName(declName.NameNode),
			Initializer: initializer, IsGlobal: true, IsPublic: slices.Contains(modifiers, MdPublic), Storage: t.VariableStorageGlobal,
		}, nil
	default:
		if len(declName.TypeParams) > 0 || len(declName.OwnerTypeParams) > 0 {
			return nil, comp_err.CompilationErrorToken(
				ctx.Fctx,
				&next,
				"syntax error: generic parameters are only valid on struct/function declarations",
				"",
			)
		}

		tNode, e := parseType(ctx, next, false)
		if e == nil {
			variable := &t.NodeExprVarDef{
				Name:       declName.NameNode,
				AbsName:    ctx.Fctx.PackageName + "." + flattenName(declName.NameNode),
				Type:       tNode,
				IsGlobal:   true,
				IsPublic:   slices.Contains(modifiers, MdPublic),
				Storage:    t.VariableStorageGlobal,
				IsReturned: false,
			}
			afterType, afterErr := peek(ctx)
			if afterErr == nil && afterType.KeywType == t.KwEqual {
				consume(ctx)
				first, err := peek(ctx)
				if err != nil {
					return nil, err
				}
				variable.Initializer, err = parseExpression(ctx, first, 0)
				if err != nil {
					return nil, err
				}
			}
			return variable, nil
		}

		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&next,
			fmt.Sprintf("syntax error: unexpected '%s' after name in global declaration", next.Repr),
			"expected in global scope: `<name> <type>`, `<name> (",
		)
	}
}

func parseConstDecl(ctx *ParseCtx, constTk t.Token) (t.NodeGlobalDecl, error) {
	modifiers := slices.Clone(ctx.NextModifiers)
	ctx.NextModifiers = []ModifierType{}
	consume(ctx)
	nameTk, e := peek(ctx)
	if e != nil || nameTk.Type != t.TokName {
		return nil, comp_err.CompilationErrorToken(ctx.Fctx, &constTk, "expected a name after 'const'", "expected: `const name Type = expression` or `const name := expression`")
	}
	consume(ctx)

	next, e := peek(ctx)
	if e != nil {
		return nil, e
	}
	var typeNode *t.NodeType
	if next.KeywType == t.KwInfer {
		consume(ctx)
	} else {
		typeNode, e = parseType(ctx, next, false)
		if e != nil {
			return nil, e
		}
		eq, e := peek(ctx)
		if e != nil || eq.KeywType != t.KwEqual {
			return nil, comp_err.CompilationErrorToken(ctx.Fctx, &nameTk, "constant declaration is missing '='", "expected: `const name Type = expression`")
		}
		consume(ctx)
	}

	first, e := peek(ctx)
	if e != nil {
		return nil, e
	}
	initializer, e := parseExpression(ctx, first, 0)
	if e != nil {
		return nil, e
	}
	vd := &t.NodeExprVarDef{
		Name:        &t.NodeNameSingle{Tk: nameTk, Name: nameTk.Repr},
		Type:        typeNode,
		Initializer: initializer,
		IsConst:     true,
		AbsName:     ctx.Fctx.PackageName + "." + nameTk.Repr,
		IsGlobal:    true,
		Storage:     t.VariableStorageGlobal,
		IsPublic:    slices.Contains(modifiers, MdPublic),
	}
	return &t.NodeConstDef{Tk: constTk, VarDef: vd, Initializer: initializer}, nil
}

func validExportName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		letter := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '_'
		if !letter && !(i > 0 && r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func parseGlobalDecl(ctx *ParseCtx, tk t.Token) (t.NodeGlobalDecl, error) {
	var n t.NodeGlobalDecl = nil
	var e error = nil

outer:
	switch tk.Type {
	case t.TokName:
		if tk.Repr == "proto" {
			return parseProtoDef(ctx, tk)
		}
		n, e := parseGlobalDeclFromName(ctx, tk)
		if e != nil {
			return nil, e
		}
		if ctx.PruneNext {
			ctx.PruneNext = false
			return nil, nil
		}
		return n, nil

	case t.TokKeyword:
		switch tk.KeywType {
		case t.KwNewline:
			consume(ctx)
			return nil, nil
		case t.KwPublic:
			e = parseApplyModifier(ctx, tk, MdPublic)
		case t.KwDestructor:
			e = parseApplyModifier(ctx, tk, MdDestructor)
		case t.KwConst:
			n, e = parseConstDecl(ctx, tk)
			return n, e
		case t.KwAlias:
			n, e = parseAliasDecl(ctx, tk)
			return n, e
		case t.KwAt:
			e = parseCompilerDirective(ctx, tk)
		case t.KwModule:
			e = parseModuleDecl(ctx, tk)
		case t.KwUse:
			e = parseUseDecl(ctx, tk, ctx.PruneNext)
			if ctx.PruneNext {
				ctx.PruneNext = false
				return nil, nil
			}
		case t.KwLink:
			e = parseLinkDecl(ctx, tk, ctx.PruneNext)
			if ctx.PruneNext {
				ctx.PruneNext = false
				return nil, nil
			}
		case t.KwBundle:
			e = parseBundleDecl(ctx, tk, ctx.PruneNext)
			if ctx.PruneNext {
				ctx.PruneNext = false
				return nil, nil
			}
		case t.KwLlvm:
			n, e = parseLlvm(ctx, tk)
			if e != nil {
				return nil, e
			}
			if ctx.PruneNext {
				ctx.PruneNext = false
				return nil, nil
			}
			return n, nil
		case t.KwExtern:
			n, e = parseExternalFunc(ctx, tk)
			if ctx.PruneNext {
				ctx.PruneNext = false
				return nil, nil
			}
			return n, nil

		default:
			break outer
		}
		if e != nil {
			return nil, e
		}
		return n, e

	default:
		break

	}
	return nil, comp_err.CompilationErrorToken(
		ctx.Fctx,
		&tk,
		fmt.Sprintf("syntax error: unexpected '%s' in global scope", tk.Repr),
		"expected in global scope: `name type = expr`, `name := expr`, `name ( args, ... ) type`, etc.",
	)
}
