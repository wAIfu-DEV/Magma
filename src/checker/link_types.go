package checker

import (
	"Magma/src/comp_err"
	magmatypes "Magma/src/magma_types"
	t "Magma/src/types"
	"fmt"
)

type typeUsage uint8

const (
	typeUsageValue typeUsage = iota
	typeUsageReturn
	typeUsageSizeof
)

func intrinsicName(node *t.NodeType) (string, *t.Token, bool) {
	if node == nil {
		return "", nil, false
	}
	named, ok := node.KindNode.(*t.NodeTypeNamed)
	if !ok {
		return "", nil, false
	}
	single, ok := named.NameNode.(*t.NodeNameSingle)
	if !ok {
		return "", nil, false
	}
	if _, ok := magmatypes.BasicTypes[single.Name]; !ok {
		return "", nil, false
	}
	return single.Name, &single.Tk, true
}

// validateIntrinsicTypeUsage enforces the context-sensitive part of primitive
// type checking after aliases and compiler-known types have been resolved.
// Void has no value representation and is restricted to function returns.
// Magma's canonical opaque pointer type is ptr, so void* is intentionally not
// accepted as a second spelling for the same type.
func validateIntrinsicTypeUsage(c *ctx, node *t.NodeType, usage typeUsage, context string) error {
	if node == nil || node.KindNode == nil {
		return nil
	}
	if name, token, ok := intrinsicName(node); ok && name == "void" && usage != typeUsageReturn {
		message := fmt.Sprintf("void cannot be used as %s", context)
		additional := "void is only valid as a function return type; use 'ptr' for an opaque pointer value"
		return comp_err.CompilationErrorToken(c.FileCtx, token, message, additional)
	}

	switch kind := node.KindNode.(type) {
	case *t.NodeTypePointer:
		return validateIntrinsicTypeUsage(c, &t.NodeType{KindNode: kind.Kind}, typeUsageValue, "a pointer element type")
	case *t.NodeTypeRfc:
		return validateIntrinsicTypeUsage(c, &t.NodeType{KindNode: kind.Kind}, typeUsageValue, "a reference element type")
	case *t.NodeTypeSlice:
		return validateIntrinsicTypeUsage(c, &t.NodeType{KindNode: kind.ElemKind}, typeUsageValue, "a slice element type")
	case *t.NodeTypeFunc:
		for _, argument := range kind.Args {
			if err := validateIntrinsicTypeUsage(c, argument, typeUsageValue, "a function parameter type"); err != nil {
				return err
			}
		}
		return validateIntrinsicTypeUsage(c, kind.RetType, typeUsageReturn, "a function return type")
	}
	return nil
}

func clTypeKind(c *ctx, parentType *t.NodeType, kind t.NodeTypeKind, topLevel bool) (t.NodeTypeKind, error) {
	switch n := kind.(type) {
	case *t.NodeTypeAbsolute:
		// absolute types have already been checked
		return nil, nil
	case *t.NodeTypeCompilerKnown:
		resolved, ok := c.Shared.Target.CompilerKnownTypes[n.Name]
		if !ok {
			return nil, comp_err.CompilationErrorToken(c.FileCtx, &n.Tk, fmt.Sprintf("compiler-known type '%s' is unavailable for target '%s'", n.Name, c.Shared.Target.Triple), "")
		}
		if _, ok := magmatypes.BasicTypes[resolved]; !ok {
			return nil, fmt.Errorf("compiler-known type %q resolved to invalid Magma type %q", n.Name, resolved)
		}
		return &t.NodeTypeNamed{NameNode: &t.NodeNameSingle{Tk: n.Tk, Name: resolved}}, nil
	case *t.NodeTypeNamed:
		if alias, owner, e := clFindTypeAlias(c, n.NameNode); e != nil {
			return nil, e
		} else if alias != nil {
			key := alias.Module + "." + alias.Name
			if c.AliasStack[key] {
				return nil, comp_err.CompilationErrorToken(c.FileCtx, lastNameToken(n.NameNode), fmt.Sprintf("cyclic type alias involving '%s'", flattenName(n.NameNode)), "type aliases cannot directly or indirectly refer to themselves")
			}
			c.AliasStack[key] = true
			defer delete(c.AliasStack, key)
			resolved := cloneAliasType(alias.Target)
			previousGlobal := c.GlobalNode
			c.GlobalNode = owner
			e := clType(c, resolved)
			c.GlobalNode = previousGlobal
			if e != nil {
				return nil, e
			}
			return resolved.KindNode, nil
		}
		switch nn := n.NameNode.(type) {
		case *t.NodeNameSingle:
			_, ok := magmatypes.BasicTypes[nn.Name]
			if ok {
				return nil, nil // is intrinsic type
			}

			sd, e := clGetStructDefFromName(c, n.NameNode)

			/* DEPRECATED
			if e == nil && sd.Destructor != nil && topLevel {
				parentType.Destructor = sd.Destructor
			}*/

			if e == nil {
				return &t.NodeTypeAbsolute{
					AbsoluteName: sd.Module + "." + sd.Name,
				}, nil
			}
			return nil, comp_err.CompilationErrorToken(
				c.FileCtx,
				lastNameToken(n.NameNode),
				fmt.Sprintf("unknown type '%s'", flattenName(n.NameNode)),
				"",
			)
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
			if private, ok := e.(*privateSymbolError); ok {
				return nil, comp_err.CompilationErrorToken(c.FileCtx, lastNameToken(n.NameNode), private.Error(), "add 'pub' to the struct declaration to export it")
			}
			return nil, comp_err.CompilationErrorToken(
				c.FileCtx,
				lastNameToken(n.NameNode),
				fmt.Sprintf("unknown type '%s'", flattenName(n.NameNode)),
				"",
			)
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

	token := &t.Token{}
	switch node := kind.(type) {
	case *t.NodeTypeNamed:
		token = lastNameToken(node.NameNode)
	case *t.NodeTypeCompilerKnown:
		token = &node.Tk
	}
	return nil, comp_err.CompilationErrorToken(c.FileCtx, token, "unsupported type expression", "use a named, pointer, reference, slice, or function type")
}

func clType(c *ctx, typeNd *t.NodeType) error {
	if typeNd == nil {
		return nil
	}

	newT, e := clTypeKind(c, typeNd, typeNd.KindNode, true)
	if e != nil {
		return e
	}
	if newT != nil {
		typeNd.KindNode = newT
	}
	return nil
}

func clTypeForUsage(c *ctx, typeNd *t.NodeType, usage typeUsage, context string) error {
	if err := clType(c, typeNd); err != nil {
		return err
	}
	return validateIntrinsicTypeUsage(c, typeNd, usage, context)
}
