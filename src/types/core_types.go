package types

// CoreTypeRole identifies a source-defined type with language-level meaning.
// Its storage layout still comes exclusively from the corresponding StructDef.
type CoreTypeRole uint8

const (
	CoreTypeNone CoreTypeRole = iota
	CoreTypeString
	CoreTypeError
	CoreTypeSlice
)

func CoreTypeRoleForName(name string) CoreTypeRole {
	switch name {
	case "str":
		return CoreTypeString
	case "error":
		return CoreTypeError
	case "slice":
		return CoreTypeSlice
	default:
		return CoreTypeNone
	}
}

func (role CoreTypeRole) Name() string {
	switch role {
	case CoreTypeString:
		return "str"
	case CoreTypeError:
		return "error"
	case CoreTypeSlice:
		return "slice"
	default:
		return ""
	}
}

func (role CoreTypeRole) LLVMName() string {
	if name := role.Name(); name != "" {
		return "%type." + name
	}
	return ""
}

// CoreTypeRoleOf reports the language role carried by a resolved or unresolved
// type node. Resolved absolute types carry the role assigned from the early
// parsed core definition, so their spelling is never used as identity.
func CoreTypeRoleOf(node *NodeType) CoreTypeRole {
	if node == nil {
		return CoreTypeNone
	}
	switch kind := node.KindNode.(type) {
	case *NodeTypeNamed:
		if single, ok := kind.NameNode.(*NodeNameSingle); ok {
			return CoreTypeRoleForName(single.Name)
		}
	case *NodeTypeCompilerKnown:
		return CoreTypeRoleForName(kind.Name)
	case *NodeTypeAbsolute:
		return kind.CoreRole
	}
	return CoreTypeNone
}
