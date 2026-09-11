package monomorph

import (
	t "Magma/src/types"
	"fmt"
	"strings"
)

func CanonicalTypeSignature(tp *t.NodeType) string {
	if tp == nil {
		return "nil"
	}
	prefix := ""
	if tp.Owned {
		prefix += "OWN__"
	}
	if tp.Throws {
		prefix += "THROW__"
	}
	switch n := tp.KindNode.(type) {
	case *t.NodeTypeAbsolute:
		return prefix + "A_" + strings.ReplaceAll(n.AbsoluteName, ".", "__")
	case *t.NodeTypeCompilerKnown:
		return prefix + "C_" + n.Name
	case *t.NodeTypeNamed:
		base := "N_" + strings.ReplaceAll(flattenName(n.NameNode), ".", "__")
		if len(n.GenericArgs) == 0 {
			return prefix + base
		}
		parts := make([]string, len(n.GenericArgs))
		for i, g := range n.GenericArgs {
			parts[i] = CanonicalTypeSignature(g)
		}
		return prefix + base + "__G__" + strings.Join(parts, "__")
	case *t.NodeTypePointer:
		return prefix + "P__" + CanonicalTypeSignature(&t.NodeType{KindNode: n.Kind})
	case *t.NodeTypeRfc:
		return prefix + "R__" + CanonicalTypeSignature(&t.NodeType{KindNode: n.Kind})
	case *t.NodeTypeSlice:
		return prefix + "S__" + CanonicalTypeSignature(&t.NodeType{KindNode: n.ElemKind})
	case *t.NodeTypeFunc:
		argParts := make([]string, len(n.Args))
		for i, a := range n.Args {
			argParts[i] = CanonicalTypeSignature(a)
		}
		return prefix + fmt.Sprintf("F%d__", n.ContextABI) + strings.Join(argParts, "__") + "__RET__" + CanonicalTypeSignature(n.RetType)
	default:
		return prefix + "Undef"
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
