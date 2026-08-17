package types

// ImplicitContextType returns the canonical context.Ctx type loaded for the
// current program. The implicit binding is created before ordinary linking, so
// pre-link passes such as monomorphization must resolve this compiler-owned
// type through the program rather than through a source import spelling.
func ImplicitContextType(shared *SharedState) *NodeType {
	shared.FilesM.Lock()
	defer shared.FilesM.Unlock()
	for _, file := range shared.Files {
		if file.ModuleName != "context" || file.GlNode == nil {
			continue
		}
		if contextDef := file.GlNode.StructDefs["Ctx"]; contextDef != nil {
			return &NodeType{KindNode: &NodeTypeAbsolute{AbsoluteName: contextDef.Module + "." + contextDef.Name}}
		}
	}
	return nil
}
