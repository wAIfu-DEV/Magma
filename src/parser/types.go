package parser

import (
	"Magma/src/comp_err"
	magmatypes "Magma/src/magma_types"
	t "Magma/src/types"
	"errors"
	"fmt"
)

func parseName(ctx *ParseCtx, tk t.Token, allowComposite bool) (t.NodeName, error) {
	parts := []string{}
	tokens := []t.Token{}
	afterDot := false

	for {
		namePart, e := peek(ctx)
		if e != nil {
			return nil, e
		}

		if namePart.Type != t.TokName {
			description := "syntax error: expected name"
			if afterDot {
				description = "syntax error: expected name after dot"
			}
			return nil, comp_err.CompilationErrorToken(
				ctx.Fctx,
				&namePart,
				description,
				"",
			)
		}

		parts = append(parts, namePart.Repr)
		tokens = append(tokens, namePart)
		consume(ctx)
		afterDot = false

		maybeDot, e := peek(ctx)
		if e != nil {
			if errors.Is(e, errOutOfBounds) {
				break
			}
			return nil, e
		}

		if maybeDot.KeywType != t.KwDot {
			break
		}

		if !allowComposite {
			return nil, comp_err.CompilationErrorToken(
				ctx.Fctx,
				&tk,
				"syntax error: context does not allow for name to be a composite name",
				"a name chain joined by '.' is a composite name, some contexts do not allow them.",
			)
		}
		consume(ctx)
		afterDot = true
	}

	switch len(parts) {
	case 0:
		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&tk,
			"syntax error: name parsing failure, unexpected state",
			"",
		)
	case 1:
		return &t.NodeNameSingle{
			Tk:   tokens[0],
			Name: parts[0],
		}, nil
	default:
		return &t.NodeNameComposite{
			Tokens: tokens,
			Parts:  parts,
		}, nil
	}
}

type parsedDeclName struct {
	NameNode        t.NodeName
	TypeParams      []string
	OwnerTypeParams []string
}

func parseDeclNameWithGenerics(ctx *ParseCtx) (*parsedDeclName, error) {
	firstTk, e := peek(ctx)
	if e != nil {
		return nil, e
	}
	if firstTk.Type != t.TokName {
		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&firstTk,
			fmt.Sprintf("syntax error: expected declaration name but got '%s'", firstTk.Repr),
			"",
		)
	}

	firstName := firstTk.Repr
	consume(ctx)

	firstParams := []string{}
	maybeOpen, e := peek(ctx)
	if e != nil {
		return nil, e
	}
	if maybeOpen.KeywType == t.KwBrackOp {
		firstParams, e = parseTypeParamList(ctx)
		if e != nil {
			return nil, e
		}
	}

	maybeDot, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	if maybeDot.KeywType != t.KwDot {
		return &parsedDeclName{
			NameNode:   &t.NodeNameSingle{Tk: firstTk, Name: firstName},
			TypeParams: firstParams,
		}, nil
	}

	consume(ctx) // dot

	secondTk, e := peek(ctx)
	if e != nil {
		return nil, e
	}
	if secondTk.Type != t.TokName {
		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&secondTk,
			fmt.Sprintf("syntax error: expected member name after '.' but got '%s'", secondTk.Repr),
			"",
		)
	}

	secondName := secondTk.Repr
	consume(ctx)

	secondParams := []string{}
	maybeOpen2, e := peek(ctx)
	if e != nil {
		return nil, e
	}
	if maybeOpen2.KeywType == t.KwBrackOp {
		secondParams, e = parseTypeParamList(ctx)
		if e != nil {
			return nil, e
		}
	}

	return &parsedDeclName{
		NameNode: &t.NodeNameComposite{
			Tokens: []t.Token{firstTk, secondTk},
			Parts:  []string{firstName, secondName},
		},
		TypeParams:      secondParams,
		OwnerTypeParams: firstParams,
	}, nil
}

func parseArgument(ctx *ParseCtx) (t.NodeArg, error) {
	name, e := peek(ctx)
	if e != nil {
		return t.NodeArg{}, e
	}

	if name.Type != t.TokName {
		return t.NodeArg{}, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&name,
			fmt.Sprintf("syntax error: expected argument name but got '%s'", name.Repr),
			"expected: `(name type, ...)`",
		)
	}

	if _, basic := magmatypes.BasicTypes[name.Repr]; basic {
		afterName, afterErr := peekNth(ctx, 1)
		if afterErr == nil && (afterName.KeywType == t.KwComma || afterName.KeywType == t.KwParenCl) {
			return t.NodeArg{}, comp_err.CompilationErrorToken(
				ctx.Fctx,
				&name,
				fmt.Sprintf("syntax error: expected a parameter or field name before type '%s'", name.Repr),
				"declarations use `name type`, for example `value u64`",
			)
		}
	}

	consume(ctx)

	typeTk, e := peek(ctx)
	if e != nil {
		return t.NodeArg{}, e
	}

	ndType, e := parseType(ctx, typeTk, false)
	if e != nil {
		return t.NodeArg{}, e
	}

	return t.NodeArg{
		Tk:       name,
		Name:     name.Repr,
		TypeNode: ndType,
	}, nil
}

func parseArgsList(ctx *ParseCtx) (t.NodeArgList, error) {
	n := t.NodeArgList{
		Args: make([]t.NodeArg, 0),
	}

	openPar, e := peek(ctx)
	if e != nil {
		return t.NodeArgList{}, e
	}
	if openPar.KeywType != t.KwParenOp {
		return t.NodeArgList{}, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&openPar,
			fmt.Sprintf("syntax error: expected '(' but got '%s'", openPar.Repr),
			"",
		)
	}
	consume(ctx)

	for {
		tk, e := peek(ctx)
		if e != nil {
			return t.NodeArgList{}, e
		}

		// TODO: func drainNewLines()
		if tk.KeywType == t.KwNewline {
			consume(ctx)
			continue
		}

		if tk.KeywType == t.KwParenCl {
			consume(ctx)
			return n, nil
		}

		argNode, e := parseArgument(ctx)
		if e != nil {
			return t.NodeArgList{}, e
		}
		n.Args = append(n.Args, argNode)

		tk, e = peek(ctx)
		if e != nil {
			return t.NodeArgList{}, e
		}

		if tk.KeywType != t.KwParenCl && tk.KeywType != t.KwComma && tk.KeywType != t.KwNewline {
			return t.NodeArgList{}, comp_err.CompilationErrorToken(
				ctx.Fctx,
				&tk,
				fmt.Sprintf("syntax error: unexpected '%s' when expected ',' or ')'", tk.Repr),
				"expected args list format: `()`, `(name type)`, `(name type, ...)`",
			)
		}

		if tk.KeywType == t.KwComma {
			consume(ctx)
		}
	}
}

func parseTypeParamList(ctx *ParseCtx) ([]string, error) {
	params := []string{}
	seen := map[string]bool{}

	open, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	if open.KeywType != t.KwBrackOp {
		return params, nil
	}

	consume(ctx)

	for {
		tk, e := peek(ctx)
		if e != nil {
			return nil, e
		}

		if tk.KeywType == t.KwBrackCl {
			consume(ctx)
			if len(params) == 0 {
				return nil, comp_err.CompilationErrorToken(
					ctx.Fctx,
					&tk,
					"syntax error: empty generic parameter list",
					"expected at least one type parameter name inside '[' and ']'",
				)
			}
			return params, nil
		}

		if tk.Type != t.TokName {
			return nil, comp_err.CompilationErrorToken(
				ctx.Fctx,
				&tk,
				fmt.Sprintf("syntax error: expected generic type parameter name but got '%s'", tk.Repr),
				"expected: `[T]`, `[T, U]`, ...",
			)
		}

		if seen[tk.Repr] {
			return nil, comp_err.CompilationErrorToken(
				ctx.Fctx,
				&tk,
				fmt.Sprintf("duplicate generic type parameter '%s'", tk.Repr),
				"each generic parameter in a declaration must have a unique name",
			)
		}
		seen[tk.Repr] = true
		params = append(params, tk.Repr)
		consume(ctx)

		sep, e := peek(ctx)
		if e != nil {
			return nil, e
		}

		if sep.KeywType == t.KwComma {
			consume(ctx)
			continue
		}
		if sep.KeywType == t.KwBrackCl {
			continue
		}

		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&sep,
			fmt.Sprintf("syntax error: expected ',' or ']' in generic parameter list but got '%s'", sep.Repr),
			"",
		)
	}
}

func parseTypeArgList(ctx *ParseCtx) ([]*t.NodeType, error) {
	out := []*t.NodeType{}

	open, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	if open.KeywType != t.KwBrackOp {
		return out, nil
	}

	consume(ctx)

	for {
		tk, e := peek(ctx)
		if e != nil {
			return nil, e
		}

		if tk.KeywType == t.KwBrackCl {
			consume(ctx)
			if len(out) == 0 {
				return nil, comp_err.CompilationErrorToken(
					ctx.Fctx,
					&tk,
					"syntax error: empty generic argument list",
					"expected at least one type argument inside '[' and ']'",
				)
			}
			return out, nil
		}

		typeNd, e := parseType(ctx, tk, false)
		if e != nil {
			return nil, e
		}
		out = append(out, typeNd)

		sep, e := peek(ctx)
		if e != nil {
			return nil, e
		}

		if sep.KeywType == t.KwComma {
			consume(ctx)
			continue
		}
		if sep.KeywType == t.KwBrackCl {
			continue
		}

		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&sep,
			fmt.Sprintf("syntax error: expected ',' or ']' in generic argument list but got '%s'", sep.Repr),
			"",
		)
	}
}

func parseGenericClass(ctx *ParseCtx, nameNode t.NodeName, typeParams []string, ownerTypeParams []string) (t.NodeGenericClass, error) {
	n := t.NodeGenericClass{
		NameNode:        nameNode,
		TypeParams:      typeParams,
		OwnerTypeParams: ownerTypeParams,
	}
	al, e := parseArgsList(ctx)
	if e != nil {
		return t.NodeGenericClass{}, e
	}
	n.ArgsNode = al
	return n, nil
}

func parseFuncType(ctx *ParseCtx) (*t.NodeType, error) {
	outT := &t.NodeType{}

	fnT := &t.NodeTypeFunc{
		Args: []*t.NodeType{},
	}

	outT.KindNode = fnT

	tk, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	if tk.KeywType != t.KwParenOp {
		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&tk,
			"expected function type but type does not start with '('",
			"",
		)
	}
	consume(ctx)

	expectComma := false

	for {
		tk, e = peek(ctx)
		if e != nil {
			return nil, e
		}

		if tk.KeywType == t.KwParenCl {
			consume(ctx)
			break
		}

		if expectComma && tk.KeywType != t.KwComma {
			return nil, comp_err.CompilationErrorToken(
				ctx.Fctx,
				&tk,
				fmt.Sprintf("expected ',' after argument type in function type definition. instead got '%s'", tk.Repr),
				"",
			)
		}

		if tk.KeywType == t.KwComma {
			expectComma = false
			consume(ctx)
			tk, e = peek(ctx)
			if e != nil {
				return nil, e
			}
		}

		n, e := parseType(ctx, tk, false)
		if e != nil {
			return nil, e
		}

		fnT.Args = append(fnT.Args, n)
		expectComma = true
	}

	tk, e = peek(ctx)
	if e != nil {
		return nil, e
	}

	n, e := parseType(ctx, tk, true)
	if e != nil {
		return nil, e
	}

	fnT.RetType = n

	return outT, nil
}

func parseTypePostfix(ctx *ParseCtx, inType *t.NodeType) (*t.NodeType, error) {

	outT := inType

	for {
		after, e := peek(ctx)
		if e != nil {
			return nil, e
		}

		// Typed slice suffix. Array storage is created by `array T[length]`.
		if after.KeywType == t.KwBrackOp {
			consume(ctx)

			maybeCl, e := peek(ctx)
			if e != nil {
				return nil, e
			}

			if maybeCl.KeywType == t.KwBrackCl {
				consume(ctx)

				sliceKind := &t.NodeTypeSlice{
					ElemKind: outT.KindNode,
				}

				sliceT := &t.NodeType{
					Throws:   outT.Throws,
					Owned:    outT.Owned,
					KindNode: sliceKind,
				}

				outT = sliceT
				continue
			}

			return nil, comp_err.CompilationErrorToken(
				ctx.Fctx,
				&maybeCl,
				"syntax error: expected ']' in slice type",
				"use `Type[]` for a slice or `array Type[length]` to create local backing storage",
			)
		}

		if after.KeywType == t.KwAsterisk {
			consume(ctx)

			sliceKind := &t.NodeTypePointer{
				Kind: outT.KindNode,
			}

			ptrT := &t.NodeType{
				Throws:   outT.Throws,
				Owned:    outT.Owned,
				KindNode: sliceKind,
			}

			outT = ptrT
			continue
		}

		if after.KeywType == t.KwDollar {
			consume(ctx)

			sliceKind := &t.NodeTypeRfc{
				Kind: outT.KindNode,
			}

			ptrT := &t.NodeType{
				Throws:   outT.Throws,
				Owned:    outT.Owned,
				KindNode: sliceKind,
			}

			outT = ptrT
			continue
		}

		break
	}

	return outT, nil
}

func parseType(ctx *ParseCtx, tk t.Token, allowThrow bool) (*t.NodeType, error) {
	isThrowing := false
	if tk.KeywType == t.KwExclam {
		if !allowThrow {
			return nil, comp_err.CompilationErrorToken(
				ctx.Fctx,
				&tk,
				"syntax error: context does not allow for type to be a throwing type",
				"a type prefixed by '!' is a throwing type, some contexts do not allow them.",
			)
		}
		isThrowing = true
		consume(ctx)
	}

	tk, e := peek(ctx)
	if e != nil {
		return nil, e
	}

	contextABI := t.ContextABIContextful
	if tk.KeywType == t.KwNoCtx {
		contextABI = t.ContextABIContextless
		consume(ctx)
		tk, e = peek(ctx)
		if e != nil {
			return nil, e
		}
	}

	// owned marker
	isOwned := false
	if tk.KeywType == t.KwDollar {
		isOwned = true
		consume(ctx)
		tk, e = peek(ctx)
		if e != nil {
			return nil, e
		}
	}

	if tk.KeywType == t.KwParenOp {
		n, e := parseFuncType(ctx)
		if e != nil {
			return nil, e
		}
		n.Owned = isOwned
		n.KindNode.(*t.NodeTypeFunc).ContextABI = contextABI
		return n, nil
	}
	if contextABI == t.ContextABIContextless {
		return nil, comp_err.CompilationErrorToken(ctx.Fctx, &tk, "syntax error: 'noctx' requires a function type", "expected: `noctx (<args>) <return type>`")
	}

	if tk.Type != t.TokName {
		return nil, comp_err.CompilationErrorToken(
			ctx.Fctx,
			&tk,
			fmt.Sprintf("syntax error: expected a type name but got '%s'", tk.Repr),
			"a type must follow the declaration name",
		)
	}

	named, e := parseName(ctx, tk, true)
	if e != nil {
		return nil, e
	}

	outT := &t.NodeType{
		Throws: isThrowing,
		Owned:  isOwned,
		KindNode: &t.NodeTypeNamed{
			NameNode: named,
		},
	}

	tk, e = peek(ctx)
	if e != nil {
		return nil, e
	}

	if tk.KeywType == t.KwBrackOp {
		maybeInner, e := peekNth(ctx, 1)
		if e != nil {
			return nil, e
		}

		// In type context, [] is a slice suffix. Non-empty brackets are
		// generic arguments; array lengths only occur in `array` expressions.
		if maybeInner.KeywType != t.KwBrackCl {
			if simple, ok := named.(*t.NodeNameSingle); ok {
				if _, basic := magmatypes.BasicTypes[simple.Name]; basic {
					return nil, comp_err.CompilationErrorToken(
						ctx.Fctx,
						&maybeInner,
						"syntax error: basic types do not take generic arguments",
						"use `Type[]` for a slice or `array Type[length]` to create local backing storage",
					)
				}
			}
			typeArgs, e := parseTypeArgList(ctx)
			if e != nil {
				return nil, e
			}

			outT.KindNode.(*t.NodeTypeNamed).GenericArgs = typeArgs
		}
	}

	tk, e = peek(ctx)
	if e != nil {
		return nil, e
	}

	outTpost, e := parseTypePostfix(ctx, outT)
	if e != nil {
		return nil, e
	}

	return outTpost, nil
}
