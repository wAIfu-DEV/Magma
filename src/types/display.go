package types

import "strings"

// SourceName converts an internal qualified identifier to source spelling.
// Package names carry a random ten-character suffix, and specializations carry
// a __g__ suffix; neither is part of the Magma language.
func SourceName(name string) string {
	parts := strings.Split(name, ".")
	if len(parts) > 1 {
		if suffix := strings.LastIndex(parts[0], "_"); suffix >= 0 && len(parts[0])-suffix-1 == 10 {
			parts[0] = parts[0][:suffix]
		}
	}
	for i, part := range parts {
		if generic := strings.Index(part, "__g__"); generic >= 0 {
			parts[i] = part[:generic]
		}
	}
	return strings.Join(parts, ".")
}

// DisplayType renders a type for diagnostics and editor features. It must not
// be used as a backend identity: internal absolute names remain mangled.
func DisplayType(node *NodeType) string {
	if node == nil {
		return "nil"
	}
	name := displayTypeKind(node.KindNode)
	if node.Throws {
		name = "!" + name
	}
	return name
}

func displayTypeKind(kind NodeTypeKind) string {
	switch n := kind.(type) {
	case *NodeTypeNamed:
		var name string
		switch value := n.NameNode.(type) {
		case *NodeNameSingle:
			name = value.Name
		case *NodeNameComposite:
			name = strings.Join(value.Parts, ".")
		}
		if len(n.GenericArgs) > 0 {
			args := make([]string, len(n.GenericArgs))
			for i, arg := range n.GenericArgs {
				args[i] = DisplayType(arg)
			}
			name += "[" + strings.Join(args, ", ") + "]"
		}
		return name
	case *NodeTypeAbsolute:
		if n.DisplayName != "" {
			return n.DisplayName
		}
		return SourceName(n.AbsoluteName)
	case *NodeTypeCompilerKnown:
		return n.Name
	case *NodeTypePointer:
		return displayTypeKind(n.Kind) + "*"
	case *NodeTypeRfc:
		return displayTypeKind(n.Kind) + "&"
	case *NodeTypeSlice:
		return displayTypeKind(n.ElemKind) + "[]"
	case *NodeTypeFunc:
		args := make([]string, len(n.Args))
		for i, arg := range n.Args {
			args[i] = DisplayType(arg)
		}
		prefix := ""
		if n.ContextABI == ContextABIContextless {
			prefix = "noctx "
		}
		return prefix + "(" + strings.Join(args, ", ") + ") " + DisplayType(n.RetType)
	default:
		return "undef"
	}
}
