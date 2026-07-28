package monomorph

import (
	t "Magma/src/types"
	"strings"
)

func CanonicalTypeSignature(tp *t.NodeType) string {
	if tp == nil {
		return "nil"
	}
	switch n := tp.KindNode.(type) {
	case *t.NodeTypeAbsolute:
		return "A_" + strings.ReplaceAll(n.AbsoluteName, ".", "__")
	case *t.NodeTypeNamed:
		base := "N_" + strings.ReplaceAll(flattenName(n.NameNode), ".", "__")
		if len(n.GenericArgs) == 0 {
			return base
		}
		parts := make([]string, len(n.GenericArgs))
		for i, g := range n.GenericArgs {
			parts[i] = CanonicalTypeSignature(g)
		}
		return base + "__G__" + strings.Join(parts, "__")
	case *t.NodeTypePointer:
		return "P__" + CanonicalTypeSignature(&t.NodeType{KindNode: n.Kind})
	case *t.NodeTypeRfc:
		return "R__" + CanonicalTypeSignature(&t.NodeType{KindNode: n.Kind})
	case *t.NodeTypeSlice:
		return "S__" + CanonicalTypeSignature(&t.NodeType{KindNode: n.ElemKind})
	case *t.NodeTypeFunc:
		argParts := make([]string, len(n.Args))
		for i, a := range n.Args {
			argParts[i] = CanonicalTypeSignature(a)
		}
		return "F__" + strings.Join(argParts, "__") + "__RET__" + CanonicalTypeSignature(n.RetType)
	default:
		return "Undef"
	}
}

func MangleSpecializedName(base string, args []*t.NodeType) string {
	if len(args) == 0 {
		return base
	}
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = CanonicalTypeSignature(a)
	}
	return base + "__g__" + strings.Join(parts, "__")
}

func sourceModuleName(name string) string {
	if i := strings.LastIndex(name, "_"); i >= 0 && len(name)-i-1 == 10 {
		return name[:i]
	}
	return name
}

func (m *monoCtx) displayType(tp *t.NodeType) string {
	if tp == nil {
		return "nil"
	}
	switch n := tp.KindNode.(type) {
	case *t.NodeTypeAbsolute:
		if display, ok := m.structDisplayNames[n.AbsoluteName]; ok {
			return display
		}
		parts := strings.Split(n.AbsoluteName, ".")
		for i := range parts {
			if generic := strings.Index(parts[i], "__g__"); generic >= 0 {
				parts[i] = parts[i][:generic]
			}
		}
		if len(parts) > 1 {
			parts[0] = sourceModuleName(parts[0])
		}
		return strings.Join(parts, ".")
	case *t.NodeTypeNamed:
		name := flattenName(n.NameNode)
		if len(n.GenericArgs) == 0 {
			return name
		}
		args := make([]string, len(n.GenericArgs))
		for i, arg := range n.GenericArgs {
			args[i] = m.displayType(arg)
		}
		return name + "[" + strings.Join(args, ", ") + "]"
	case *t.NodeTypePointer:
		return m.displayType(&t.NodeType{KindNode: n.Kind}) + "*"
	case *t.NodeTypeRfc:
		return m.displayType(&t.NodeType{KindNode: n.Kind}) + "&"
	case *t.NodeTypeSlice:
		return m.displayType(&t.NodeType{KindNode: n.ElemKind}) + "[]"
	case *t.NodeTypeFunc:
		args := make([]string, len(n.Args))
		for i, arg := range n.Args {
			args[i] = m.displayType(arg)
		}
		return "(" + strings.Join(args, ", ") + ") " + m.displayType(n.RetType)
	default:
		return "undef"
	}
}

func (m *monoCtx) genericDisplayName(base string, args []*t.NodeType) string {
	displayArgs := make([]string, len(args))
	for i, arg := range args {
		displayArgs[i] = m.displayType(arg)
	}
	return base + "[" + strings.Join(displayArgs, ", ") + "]"
}

func unqualifiedDisplayName(name string) string {
	if i := strings.Index(name, "."); i >= 0 {
		return name[i+1:]
	}
	return name
}
