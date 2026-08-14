// Package loweringvalidate checks the semantic AST contract required by LLVM
// lowering. Validation is deliberately read-only: it rejects incomplete stage
// output without normalizing or otherwise changing valid programs.
package loweringvalidate

import (
	"Magma/src/comp_err"
	magmatypes "Magma/src/magma_types"
	t "Magma/src/types"
	"fmt"
	"sort"
)

// Validate checks every compilation unit after linking and type checking.
func Validate(shared *t.SharedState) error {
	shared.FilesM.Lock()
	paths := make([]string, 0, len(shared.Files))
	files := make(map[string]*t.FileCtx, len(shared.Files))
	for path, file := range shared.Files {
		paths = append(paths, path)
		files[path] = file
	}
	shared.FilesM.Unlock()
	sort.Strings(paths)

	for _, path := range paths {
		file := files[path]
		if file == nil || file.GlNode == nil {
			continue
		}
		for _, declaration := range file.GlNode.Declarations {
			if err := declarationValid(file, declaration); err != nil {
				return err
			}
		}
	}
	return nil
}

func invalid(file *t.FileCtx, token *t.Token, detail string) error {
	if token == nil {
		token = &t.Token{}
	}
	return comp_err.CompilationErrorToken(file, token, "compiler stage produced incomplete lowering input", detail)
}

func typeToken(node *t.NodeType) *t.Token {
	if node == nil {
		return nil
	}
	switch kind := node.KindNode.(type) {
	case *t.NodeTypeNamed:
		switch name := kind.NameNode.(type) {
		case *t.NodeNameSingle:
			return &name.Tk
		case *t.NodeNameComposite:
			if len(name.Tokens) > 0 {
				return &name.Tokens[len(name.Tokens)-1]
			}
		}
	case *t.NodeTypeCompilerKnown:
		return &kind.Tk
	}
	return nil
}

func typeValid(file *t.FileCtx, node *t.NodeType, context string) error {
	if node == nil || node.KindNode == nil {
		return invalid(file, typeToken(node), context+" has no concrete type")
	}
	switch kind := node.KindNode.(type) {
	case *t.NodeTypeNamed:
		name, ok := kind.NameNode.(*t.NodeNameSingle)
		if !ok || name.Name == "" {
			return invalid(file, typeToken(node), context+" contains an unresolved named type")
		}
		if _, ok := magmatypes.BasicTypes[name.Name]; !ok {
			return invalid(file, &name.Tk, context+" contains a named type that was not resolved before lowering")
		}
		if len(kind.GenericArgs) != 0 {
			return invalid(file, &name.Tk, context+" contains unresolved generic type arguments")
		}
	case *t.NodeTypeAbsolute:
		if kind.AbsoluteName == "" {
			return invalid(file, typeToken(node), context+" contains an empty absolute type name")
		}
	case *t.NodeTypePointer:
		return typeKindValid(file, kind.Kind, context+" pointer element")
	case *t.NodeTypeRfc:
		return typeKindValid(file, kind.Kind, context+" reference element")
	case *t.NodeTypeSlice:
		return typeKindValid(file, kind.ElemKind, context+" slice element")
	case *t.NodeTypeFunc:
		for i, argument := range kind.Args {
			if err := typeValid(file, argument, fmt.Sprintf("%s function argument %d", context, i+1)); err != nil {
				return err
			}
		}
		return typeValid(file, kind.RetType, context+" function return")
	case *t.NodeTypeCompilerKnown:
		return invalid(file, &kind.Tk, context+" still contains a compiler-known placeholder type")
	default:
		return invalid(file, typeToken(node), fmt.Sprintf("%s contains unsupported type node %T", context, node.KindNode))
	}
	return nil
}

func typeKindValid(file *t.FileCtx, kind t.NodeTypeKind, context string) error {
	return typeValid(file, &t.NodeType{KindNode: kind}, context)
}

func intrinsicUsageValid(file *t.FileCtx, node *t.NodeType, allowVoid bool, context string) error {
	if node == nil || node.KindNode == nil {
		return nil
	}
	if kind, ok := node.KindNode.(*t.NodeTypeNamed); ok {
		if name, ok := kind.NameNode.(*t.NodeNameSingle); ok && name.Name == "void" && !allowVoid {
			return invalid(file, &name.Tk, context+" uses void where a value type is required")
		}
	}
	switch kind := node.KindNode.(type) {
	case *t.NodeTypePointer:
		return intrinsicUsageValid(file, &t.NodeType{KindNode: kind.Kind}, false, context+" pointer element")
	case *t.NodeTypeRfc:
		return intrinsicUsageValid(file, &t.NodeType{KindNode: kind.Kind}, false, context+" reference element")
	case *t.NodeTypeSlice:
		return intrinsicUsageValid(file, &t.NodeType{KindNode: kind.ElemKind}, false, context+" slice element")
	case *t.NodeTypeFunc:
		for i, argument := range kind.Args {
			if err := intrinsicUsageValid(file, argument, false, fmt.Sprintf("%s function argument %d", context, i+1)); err != nil {
				return err
			}
		}
		return intrinsicUsageValid(file, kind.RetType, true, context+" function return")
	}
	return nil
}

func valueTypeValid(file *t.FileCtx, node *t.NodeType, context string) error {
	if err := typeValid(file, node, context); err != nil {
		return err
	}
	return intrinsicUsageValid(file, node, false, context)
}

func returnTypeValid(file *t.FileCtx, node *t.NodeType, context string) error {
	if err := typeValid(file, node, context); err != nil {
		return err
	}
	return intrinsicUsageValid(file, node, true, context)
}

func nameToken(name *t.NodeExprName) *t.Token {
	if name == nil {
		return nil
	}
	switch node := name.Name.(type) {
	case *t.NodeNameSingle:
		return &node.Tk
	case *t.NodeNameComposite:
		if len(node.Tokens) > 0 {
			return &node.Tokens[len(node.Tokens)-1]
		}
	}
	return &name.Tk
}

func typeComplete(node *t.NodeType) bool {
	return node != nil && node.KindNode != nil
}

func sameResolvedType(left *t.NodeType, right *t.NodeType) bool {
	if left == nil || right == nil || left.Throws != right.Throws || left.Owned != right.Owned {
		return left == right
	}
	return sameResolvedTypeKind(left.KindNode, right.KindNode)
}

func sameResolvedTypeKind(left t.NodeTypeKind, right t.NodeTypeKind) bool {
	switch leftKind := left.(type) {
	case *t.NodeTypeNamed:
		rightKind, ok := right.(*t.NodeTypeNamed)
		if !ok || len(leftKind.GenericArgs) != len(rightKind.GenericArgs) {
			return false
		}
		leftName, leftOK := leftKind.NameNode.(*t.NodeNameSingle)
		rightName, rightOK := rightKind.NameNode.(*t.NodeNameSingle)
		if !leftOK || !rightOK || leftName.Name != rightName.Name {
			return false
		}
		for i := range leftKind.GenericArgs {
			if !sameResolvedType(leftKind.GenericArgs[i], rightKind.GenericArgs[i]) {
				return false
			}
		}
		return true
	case *t.NodeTypeAbsolute:
		rightKind, ok := right.(*t.NodeTypeAbsolute)
		return ok && leftKind.AbsoluteName == rightKind.AbsoluteName
	case *t.NodeTypePointer:
		rightKind, ok := right.(*t.NodeTypePointer)
		return ok && sameResolvedTypeKind(leftKind.Kind, rightKind.Kind)
	case *t.NodeTypeRfc:
		rightKind, ok := right.(*t.NodeTypeRfc)
		return ok && sameResolvedTypeKind(leftKind.Kind, rightKind.Kind)
	case *t.NodeTypeSlice:
		rightKind, ok := right.(*t.NodeTypeSlice)
		return ok && sameResolvedTypeKind(leftKind.ElemKind, rightKind.ElemKind)
	case *t.NodeTypeFunc:
		rightKind, ok := right.(*t.NodeTypeFunc)
		if !ok || len(leftKind.Args) != len(rightKind.Args) || !sameResolvedType(leftKind.RetType, rightKind.RetType) {
			return false
		}
		for i := range leftKind.Args {
			if !sameResolvedType(leftKind.Args[i], rightKind.Args[i]) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func namedTypeIs(node *t.NodeType, expected string) bool {
	if node == nil || node.Throws {
		return false
	}
	switch kind := node.KindNode.(type) {
	case *t.NodeTypeNamed:
		name, ok := kind.NameNode.(*t.NodeNameSingle)
		return ok && name.Name == expected && len(kind.GenericArgs) == 0
	case *t.NodeTypeAbsolute:
		return kind.AbsoluteName == expected
	default:
		return false
	}
}

func resolvedIntegerType(node *t.NodeType) bool {
	if node == nil || node.Throws {
		return false
	}
	named, ok := node.KindNode.(*t.NodeTypeNamed)
	if !ok {
		return false
	}
	name, ok := named.NameNode.(*t.NodeNameSingle)
	if !ok || len(named.GenericArgs) != 0 {
		return false
	}
	descriptor, ok := magmatypes.NumberTypes[name.Name]
	return ok && !descriptor.IsFloat
}

func variableValid(file *t.FileCtx, variable *t.NodeExprVarDef, context string) error {
	if variable == nil || variable.Name == nil {
		return invalid(file, nil, context+" has no variable definition")
	}
	name, ok := variable.Name.(*t.NodeNameSingle)
	if !ok || name.Name == "" {
		return invalid(file, nil, context+" has no concrete local name")
	}
	if variable.Storage != t.VariableStorageLocal {
		return invalid(file, &name.Tk, context+" has no resolved local storage")
	}
	return valueTypeValid(file, variable.Type, context+" type")
}

func nameValid(file *t.FileCtx, name *t.NodeExprName, inferredTypeRequired bool) error {
	if name == nil || name.Name == nil {
		return invalid(file, nameToken(name), "name expression has no source name")
	}
	switch node := name.Name.(type) {
	case *t.NodeNameSingle:
		if node.Name == "" {
			return invalid(file, &node.Tk, "name expression is empty")
		}
	case *t.NodeNameComposite:
		if len(node.Parts) == 0 {
			return invalid(file, nameToken(name), "composite name has no parts")
		}
	default:
		return invalid(file, nameToken(name), "name expression has an unsupported name node")
	}
	if inferredTypeRequired {
		if err := typeValid(file, name.InfType, "name expression inferred type"); err != nil {
			return err
		}
	}

	var rootType *t.NodeType
	var storage t.VariableStorage
	switch definition := name.AssociatedNode.(type) {
	case *t.NodeExprVarDef:
		if definition.IsGlobal && definition.AbsName == "" {
			return invalid(file, nameToken(name), "global name has no resolved symbol")
		}
		rootType = definition.Type
		storage = definition.Storage
	case *t.NodeExprVarDefAssign:
		if definition.VarDef == nil {
			return invalid(file, nameToken(name), "name points to an incomplete variable definition")
		}
		if definition.VarDef.IsGlobal && definition.VarDef.AbsName == "" {
			return invalid(file, nameToken(name), "global name has no resolved symbol")
		}
		rootType = definition.VarDef.Type
		storage = definition.VarDef.Storage
	case *t.NodeFuncDef:
		if definition.ReturnType == nil {
			return invalid(file, nameToken(name), "name points to a function with no return type")
		}
		if err := typeValid(file, name.InfType, "function value inferred type"); err != nil {
			return err
		}
		functionType, ok := name.InfType.KindNode.(*t.NodeTypeFunc)
		if !ok {
			return invalid(file, nameToken(name), "function value does not have a resolved function type")
		}
		if len(functionType.Args) != len(definition.Class.ArgsNode.Args) {
			return invalid(file, nameToken(name), "function value signature does not match its resolved declaration")
		}
		for i, argument := range definition.Class.ArgsNode.Args {
			if !sameResolvedType(functionType.Args[i], argument.TypeNode) {
				return invalid(file, nameToken(name), "function value argument types do not match its resolved declaration")
			}
		}
		if !sameResolvedType(functionType.RetType, definition.ReturnType) {
			return invalid(file, nameToken(name), "function value return type does not match its resolved declaration")
		}
		rootType = name.InfType
	default:
		return invalid(file, nameToken(name), "name has no supported resolved declaration")
	}
	if storage != t.VariableStorageUnresolved {
		if name.Storage != storage {
			return invalid(file, nameToken(name), "name storage does not match its resolved variable")
		}
	} else if _, isFunction := name.AssociatedNode.(*t.NodeFuncDef); !isFunction {
		return invalid(file, nameToken(name), "resolved variable has no storage classification")
	}
	if err := typeValid(file, rootType, "resolved name root"); err != nil {
		return err
	}
	currentType := rootType
	for _, access := range name.MemberAccesses {
		if access == nil || access.FieldNb < 0 {
			return invalid(file, nameToken(name), "name contains incomplete member-access metadata")
		}
		if err := typeValid(file, access.OwnerType, "name member-access owner"); err != nil {
			return err
		}
		if err := typeValid(file, access.Type, "name member access"); err != nil {
			return err
		}
		if !sameResolvedType(currentType, access.OwnerType) {
			return invalid(file, nameToken(name), "name member-access owner does not match the preceding result type")
		}
		_, pointerOwner := currentType.KindNode.(*t.NodeTypePointer)
		if pointerOwner != access.PtrDeref {
			return invalid(file, nameToken(name), "name member-access pointer transition is inconsistent")
		}
		currentType = access.Type
	}
	if len(name.MemberAccesses) > 0 && inferredTypeRequired && !sameResolvedType(name.InfType, currentType) {
		return invalid(file, nameToken(name), "name inferred type does not match its member-access result")
	}
	return nil
}

func declarationValid(file *t.FileCtx, declaration t.NodeGlobalDecl) error {
	switch node := declaration.(type) {
	case *t.NodeFuncDef:
		_, compositeName := node.Class.NameNode.(*t.NodeNameComposite)
		if node.IsMember != compositeName {
			return invalid(file, nil, "function member metadata does not match its resolved declaration")
		}
		if node.IsMember && node.IsEntryPoint {
			return invalid(file, nil, "member function is incorrectly marked as an entry point")
		}
		if node.IsMember {
			if len(node.Class.ArgsNode.Args) == 0 || node.Class.ArgsNode.Args[0].Name != "this" {
				return invalid(file, nil, "member function has no resolved receiver argument")
			}
			receiverType := node.Class.ArgsNode.Args[0].TypeNode
			if receiverType == nil {
				return invalid(file, &node.Class.ArgsNode.Args[0].Tk, "member function receiver has no resolved type")
			}
			if _, ok := receiverType.KindNode.(*t.NodeTypePointer); !ok {
				return invalid(file, &node.Class.ArgsNode.Args[0].Tk, "member function receiver is not a resolved pointer")
			}
		}
		for i, argument := range node.Class.ArgsNode.Args {
			if err := valueTypeValid(file, argument.TypeNode, fmt.Sprintf("function argument %d", i+1)); err != nil {
				return err
			}
		}
		if err := returnTypeValid(file, node.ReturnType, "function return"); err != nil {
			return err
		}
		return bodyValid(file, &node.Body, node.ReturnType)
	case *t.NodeStructDef:
		name, ok := node.Class.NameNode.(*t.NodeNameSingle)
		if !ok || name.Name == "" {
			return invalid(file, nil, "struct declaration has no concrete local name")
		}
		definition := file.GlNode.StructDefs[name.Name]
		if definition == nil {
			return invalid(file, &name.Tk, "struct declaration has no resolved definition")
		}
		expectedAbsName := definition.Module + "." + definition.Name
		if node.AbsName == "" || node.AbsName != expectedAbsName {
			return invalid(file, &name.Tk, "struct declaration has an inconsistent resolved symbol")
		}
		for i, field := range node.Class.ArgsNode.Args {
			if err := valueTypeValid(file, field.TypeNode, fmt.Sprintf("struct field %d", i+1)); err != nil {
				return err
			}
		}
	case *t.NodeExprVarDef:
		if node.Storage != t.VariableStorageGlobal || !node.IsGlobal || node.AbsName == "" {
			return invalid(file, nil, "global variable has incomplete storage metadata")
		}
		if err := valueTypeValid(file, node.Type, "global variable"); err != nil {
			return err
		}
		if node.Initializer != nil {
			return expressionValid(file, node.Initializer)
		}
	case *t.NodeConstDef:
		if node.VarDef == nil {
			return invalid(file, &node.Tk, "constant has no variable definition")
		}
		if node.VarDef.Storage != t.VariableStorageGlobal || !node.VarDef.IsGlobal || node.VarDef.AbsName == "" {
			return invalid(file, &node.Tk, "constant has incomplete global storage metadata")
		}
		if err := valueTypeValid(file, node.VarDef.Type, "constant"); err != nil {
			return err
		}
		return expressionValid(file, node.Initializer)
	}
	return nil
}

func bodyValid(file *t.FileCtx, body *t.NodeBody, ownerReturn *t.NodeType) error {
	return bodyValidAtLoopDepth(file, body, ownerReturn, 0)
}

func bodyValidAtLoopDepth(file *t.FileCtx, body *t.NodeBody, ownerReturn *t.NodeType, loopDepth int) error {
	for _, statement := range body.Statements {
		var err error
		switch node := statement.(type) {
		case *t.NodeStmtRet:
			if err = typeValid(file, node.OwnerFuncType, "return statement owner"); err == nil {
				if ownerReturn == nil || node.OwnerFuncType.Throws != ownerReturn.Throws {
					err = invalid(file, &node.Tk, "return statement owner does not match its enclosing function")
				} else {
					err = expressionValid(file, node.Expression)
				}
			}
		case *t.NodeStmtExpr:
			err = expressionValid(file, node.Expression)
		case *t.NodeStmtThrow:
			err = expressionValid(file, node.Expression)
		case *t.NodeStmtIf:
			err = expressionValid(file, node.CondExpr)
			if err == nil && !namedTypeIs(node.CondExpr.GetInferredType(), "bool") {
				err = invalid(file, &node.Tk, "if condition does not have resolved type 'bool'")
			}
			if err == nil {
				err = bodyValidAtLoopDepth(file, &node.Body, ownerReturn, loopDepth)
			}
			if err == nil && node.NextCondStmt != nil {
				switch next := node.NextCondStmt.(type) {
				case *t.NodeStmtIf:
					err = bodyValidAtLoopDepth(file, &t.NodeBody{Statements: []t.NodeStatement{next}}, ownerReturn, loopDepth)
				case *t.NodeStmtElse:
					err = bodyValidAtLoopDepth(file, &next.Body, ownerReturn, loopDepth)
				default:
					err = invalid(file, &node.Tk, fmt.Sprintf("if chain contains unsupported continuation %T", node.NextCondStmt))
				}
			}
		case *t.NodeStmtWhile:
			err = expressionValid(file, node.CondExpr)
			if err == nil && !namedTypeIs(node.CondExpr.GetInferredType(), "bool") {
				err = invalid(file, &node.Tk, "while condition does not have resolved type 'bool'")
			}
			if err == nil {
				err = bodyValidAtLoopDepth(file, &node.Body, ownerReturn, loopDepth+1)
			}
		case *t.NodeStmtFor:
			decl, ok := node.DeclExpr.(*t.NodeExprVarDefAssign)
			if !ok || decl.VarDef == nil {
				err = invalid(file, &node.Tk, "for loop has no initialized index declaration")
			} else if err = expressionValid(file, node.DeclExpr); err == nil {
				if !resolvedIntegerType(decl.VarDef.Type) {
					err = invalid(file, &node.Tk, "for loop index does not have a resolved integer type")
				} else if err = expressionValid(file, node.BoundExpr); err == nil {
					boundType := node.BoundExpr.GetInferredType()
					if !resolvedIntegerType(boundType) {
						err = invalid(file, &node.Tk, "for loop bound does not have a resolved integer type")
					} else if _, literal := node.BoundExpr.(*t.NodeExprLit); !literal && !sameResolvedType(decl.VarDef.Type, boundType) {
						err = invalid(file, &node.Tk, "for loop index and bound types do not match")
					}
				}
			}
			if err == nil {
				err = bodyValidAtLoopDepth(file, &node.Body, ownerReturn, loopDepth+1)
			}
		case *t.NodeStmtBounded:
			if len(node.Predicates) == 0 {
				err = invalid(file, &node.Tk, "bounded statement has no predicates")
			}
			for _, predicate := range node.Predicates {
				if err == nil {
					err = expressionValid(file, predicate)
				}
				binary, ok := predicate.(*t.NodeExprBinary)
				if err == nil && (!ok || (binary.Operator != t.KwCmpLt && binary.Operator != t.KwCmpLtEq)) {
					err = invalid(file, &node.Tk, "bounded statement contains an invalid predicate")
				}
			}
			if err == nil {
				err = bodyValidAtLoopDepth(file, &node.Body, ownerReturn, loopDepth)
			}
		case *t.NodeStmtUnsafe:
			err = bodyValidAtLoopDepth(file, &node.Body, ownerReturn, loopDepth)
		case *t.NodeStmtDefer:
			if node.IsBody {
				if node.Expression != nil {
					err = invalid(file, nil, "defer body also contains an expression")
				} else {
					err = bodyValidAtLoopDepth(file, &node.Body, ownerReturn, loopDepth)
				}
			} else {
				if len(node.Body.Statements) != 0 {
					err = invalid(file, nil, "defer expression also contains a body")
				} else {
					err = expressionValid(file, node.Expression)
				}
			}
		case *t.NodeStmtBreak:
			if loopDepth == 0 {
				err = invalid(file, &node.Tk, "break statement is not inside a loop")
			}
		case *t.NodeStmtContinue:
			if loopDepth == 0 {
				err = invalid(file, &node.Tk, "continue statement is not inside a loop")
			}
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func callValid(file *t.FileCtx, call *t.NodeExprCall) error {
	if call == nil {
		return invalid(file, nil, "lowering requires a function call")
	}
	if call.Callee == nil {
		return invalid(file, &call.Tk, "call has no callee")
	}
	if err := typeValid(file, call.InfType, "call inferred return"); err != nil {
		return err
	}

	expected := 0
	if call.IsFuncPointer {
		if call.FuncPtrType == nil {
			return invalid(file, &call.Tk, "function-pointer call has no resolved function type")
		}
		functionType, ok := call.FuncPtrType.KindNode.(*t.NodeTypeFunc)
		if !ok || functionType.RetType == nil {
			return invalid(file, &call.Tk, "function-pointer call does not contain a complete function type")
		}
		if err := typeValid(file, call.FuncPtrType, "function-pointer call target"); err != nil {
			return err
		}
		expected = len(functionType.Args)
	} else {
		if call.AssociatedFnDef == nil {
			return invalid(file, &call.Tk, "direct call has no resolved function definition")
		}
		expected = len(call.AssociatedFnDef.Class.ArgsNode.Args)
		if call.IsMemberFunc {
			expected--
			if call.MemberOwnerType == nil || (call.MemberOwnerExpr == nil && call.MemberOwnerName == nil) {
				return invalid(file, &call.Tk, "member call has no resolved owner")
			}
			if err := typeValid(file, call.MemberOwnerType, "member-call owner"); err != nil {
				return err
			}
		}
	}
	if expected < 0 || len(call.Args) != expected {
		return invalid(file, &call.Tk, fmt.Sprintf("resolved call expects %d argument(s), but contains %d", expected, len(call.Args)))
	}
	if calleeName, ok := call.Callee.(*t.NodeExprName); ok {
		if err := nameValid(file, calleeName, false); err != nil {
			return err
		}
	} else if memberCallee, ok := call.Callee.(*t.NodeExprMemberAccess); ok && call.IsMemberFunc {
		// A method-selection expression is resolved onto the call itself and is
		// not lowered as a field access, so it intentionally has no Access entry.
		if memberCallee.Target == nil || !typeComplete(memberCallee.Target.GetInferredType()) {
			return invalid(file, &memberCallee.Tk, "member-call target has no resolved type")
		}
		if err := expressionValid(file, memberCallee.Target); err != nil {
			return err
		}
	} else if err := expressionValid(file, call.Callee); err != nil {
		return err
	}
	for _, argument := range call.Args {
		if err := expressionValid(file, argument); err != nil {
			return err
		}
	}
	return nil
}

func expressionValid(file *t.FileCtx, expression t.NodeExpr) error {
	if expression == nil {
		return invalid(file, nil, "lowering encountered a missing expression")
	}
	switch node := expression.(type) {
	case *t.NodeExprCall:
		return callValid(file, node)
	case *t.NodeExprTry:
		if err := typeValid(file, node.InfType, "try expression result"); err != nil {
			return err
		}
		call, ok := node.Call.(*t.NodeExprCall)
		if !ok {
			return invalid(file, &node.Tk, "try expression does not contain a function call")
		}
		return callValid(file, call)
	case *t.NodeExprDestructureAssign:
		if err := variableValid(file, &node.ValueDef, "destructuring value binding"); err != nil {
			return err
		}
		if err := variableValid(file, &node.ErrDef, "destructuring error binding"); err != nil {
			return err
		}
		if node.Call == nil {
			return invalid(file, nil, "destructuring assignment has no call")
		}
		if err := callValid(file, node.Call); err != nil {
			return err
		}
		if !node.Call.InfType.Throws {
			return invalid(file, &node.Call.Tk, "destructuring assignment call is not throwing")
		}
		callValue := *node.Call.InfType
		callValue.Throws = false
		if namedTypeIs(&callValue, "void") {
			return invalid(file, &node.Call.Tk, "destructuring assignment call has no value result")
		}
		if !namedTypeIs(node.ErrDef.Type, "error") {
			return invalid(file, &node.Call.Tk, "destructuring error binding does not have type 'error'")
		}
		return nil
	case *t.NodeExprDestructor:
		return invalid(file, nil, "deprecated destructor expression reached lowering")
	case *t.NodeExprUnary:
		if err := typeValid(file, node.InfType, "unary expression result"); err != nil {
			return err
		}
		return expressionValid(file, node.Operand)
	case *t.NodeExprBinary:
		if err := typeValid(file, node.InfType, "binary expression result"); err != nil {
			return err
		}
		if node.OperandType != nil {
			if err := typeValid(file, node.OperandType, "binary expression operand"); err != nil {
				return err
			}
		}
		if err := expressionValid(file, node.Left); err != nil {
			return err
		}
		return expressionValid(file, node.Right)
	case *t.NodeExprArray:
		if err := valueTypeValid(file, node.ElemType, "array element"); err != nil {
			return err
		}
		if err := valueTypeValid(file, node.LengthType, "array length representation"); err != nil {
			return err
		}
		if !namedTypeIs(node.LengthType, "u64") {
			return invalid(file, &node.Tk, "array length representation is not u64")
		}
		if err := typeValid(file, node.InfType, "array expression result"); err != nil {
			return err
		}
		if err := expressionValid(file, node.Length); err != nil {
			return err
		}
		for _, entry := range node.Entries {
			if entry.Index != nil {
				if err := expressionValid(file, entry.Index); err != nil {
					return err
				}
			}
			if err := expressionValid(file, entry.Value); err != nil {
				return err
			}
		}
	case *t.NodeExprAddrof:
		if err := typeValid(file, node.InfType, "address-of expression result"); err != nil {
			return err
		}
		return expressionValid(file, node.Expr)
	case *t.NodeExprMove:
		if err := typeValid(file, node.InfType, "move expression result"); err != nil {
			return err
		}
		return expressionValid(file, node.Expr)
	case *t.NodeExprStructInit:
		if err := typeValid(file, node.Type, "struct initializer result"); err != nil {
			return err
		}
		for _, field := range node.Fields {
			if field.FieldIndex < 0 || field.Name == "" {
				return invalid(file, &field.Tk, "struct initializer contains incomplete field metadata")
			}
			if err := typeValid(file, field.FieldType, "struct initializer field"); err != nil {
				return err
			}
			if err := expressionValid(file, field.Expression); err != nil {
				return err
			}
		}
	case *t.NodeExprSubscript:
		if !typeComplete(node.BoxType) || !typeComplete(node.ElemType) || !typeComplete(node.IndexType) {
			return invalid(file, &node.Tk, "subscript has incomplete resolved type metadata")
		}
		if err := typeValid(file, node.BoxType, "subscript container"); err != nil {
			return err
		}
		if err := typeValid(file, node.ElemType, "subscript element"); err != nil {
			return err
		}
		if err := valueTypeValid(file, node.IndexType, "subscript index representation"); err != nil {
			return err
		}
		if !namedTypeIs(node.IndexType, "i64") {
			return invalid(file, &node.Tk, "subscript index representation is not i64")
		}
		switch node.BoxType.KindNode.(type) {
		case *t.NodeTypeSlice, *t.NodeTypePointer, *t.NodeTypeRfc:
		default:
			return invalid(file, &node.Tk, "subscript container is not a slice, pointer, or reference")
		}
		if err := expressionValid(file, node.Target); err != nil {
			return err
		}
		return expressionValid(file, node.Expr)
	case *t.NodeExprMemberAccess:
		if node.Access == nil || !typeComplete(node.Access.OwnerType) || !typeComplete(node.Access.Type) || node.Access.FieldNb < 0 {
			return invalid(file, &node.Tk, "member expression has no resolved field metadata")
		}
		if node.Target == nil || !typeComplete(node.Target.GetInferredType()) || !typeComplete(node.InfType) {
			return invalid(file, &node.Tk, "member expression has no resolved target or result type")
		}
		if err := typeValid(file, node.Access.Type, "member-access field"); err != nil {
			return err
		}
		if err := typeValid(file, node.Access.OwnerType, "member-access owner"); err != nil {
			return err
		}
		if err := typeValid(file, node.Target.GetInferredType(), "member-access target"); err != nil {
			return err
		}
		if err := typeValid(file, node.InfType, "member-access result"); err != nil {
			return err
		}
		if !sameResolvedType(node.Target.GetInferredType(), node.Access.OwnerType) || !sameResolvedType(node.InfType, node.Access.Type) {
			return invalid(file, &node.Tk, "member expression types do not match its resolved field metadata")
		}
		_, pointerOwner := node.Access.OwnerType.KindNode.(*t.NodeTypePointer)
		if pointerOwner != node.Access.PtrDeref {
			return invalid(file, &node.Tk, "member expression pointer transition is inconsistent")
		}
		return expressionValid(file, node.Target)
	case *t.NodeExprName:
		return nameValid(file, node, true)
	case *t.NodeExprVarDef:
		return variableValid(file, node, "local variable")
	case *t.NodeExprVarDefAssign:
		if node.VarDef == nil {
			return invalid(file, &node.Tk, "local variable assignment has no variable definition")
		}
		if node.VarDef.Storage != t.VariableStorageLocal {
			return invalid(file, &node.Tk, "local variable has no resolved local storage")
		}
		if err := valueTypeValid(file, node.VarDef.Type, "local variable"); err != nil {
			return err
		}
		return expressionValid(file, node.AssignExpr)
	case *t.NodeExprAssign:
		if err := typeValid(file, node.InfType, "assignment result"); err != nil {
			return err
		}
		if err := expressionValid(file, node.Left); err != nil {
			return err
		}
		return expressionValid(file, node.Right)
	case *t.NodeExprLit:
		return typeValid(file, node.InfType, "literal inferred type")
	case *t.NodeExprSizeof:
		if err := valueTypeValid(file, node.Type, "sizeof operand"); err != nil {
			return err
		}
		return typeValid(file, node.InfType, "sizeof result")
	case *t.NodeExprVoid:
		return typeValid(file, node.VoidType, "void expression")
	}
	return nil
}
