package monomorph

import (
	t "Magma/src/types"
	"strings"
)

func isGenericStructDecl(st *t.NodeStructDef) bool {
	return st != nil && len(st.Class.TypeParams) > 0
}

func isGenericFuncDecl(fn *t.NodeFuncDef) bool {
	return fn != nil && (len(fn.Class.TypeParams) > 0 || len(fn.Class.OwnerTypeParams) > 0)
}

func (m *monoCtx) moduleForGlobal(gl *t.NodeGlobal) string {
	for module, n := range m.modules {
		if n == gl {
			return module
		}
	}
	return ""
}

func syncStructDefFields(gl *t.NodeGlobal, st *t.NodeStructDef) {
	if gl == nil || st == nil {
		return
	}

	name := flattenName(st.Class.NameNode)
	sd, ok := gl.StructDefs[name]
	if !ok {
		return
	}

	if sd.Fields == nil {
		sd.Fields = map[string]*t.NodeType{}
	}
	sd.FieldOrder = sd.FieldOrder[:0]

	for _, fld := range st.Class.ArgsNode.Args {
		sd.Fields[fld.Name] = cloneType(fld.TypeNode)
		sd.FieldOrder = append(sd.FieldOrder, fld.Name)
	}
}

func (m *monoCtx) registerFuncTemplate(module string, mapName string, fn *t.NodeFuncDef) {
	if fn == nil || len(fn.Class.TypeParams) == 0 {
		return
	}

	if name, ok := fn.Class.NameNode.(*t.NodeNameComposite); ok && len(name.Parts) >= 2 {
		ownerName := strings.Join(name.Parts[:len(name.Parts)-1], ".")
		memberName := name.Parts[len(name.Parts)-1]
		m.memberTemplates[makeMemberTemplateKey(module, ownerName, memberName)] = fn
		return
	}

	m.funcTemplates[makeTemplateKey(module, mapName)] = fn
}

func (m *monoCtx) pruneTemplates() {
	for module, gl := range m.modules {
		filtered := make([]t.NodeGlobalDecl, 0, len(gl.Declarations))
		for _, d := range gl.Declarations {
			switch n := d.(type) {
			case *t.NodeStructDef:
				if isGenericStructDecl(n) {
					continue
				}
			case *t.NodeFuncDef:
				if isGenericFuncDecl(n) {
					continue
				}
			}
			filtered = append(filtered, d)
		}
		gl.Declarations = filtered

		for name, st := range gl.StructDefs {
			if len(st.TypeParams) > 0 {
				delete(gl.StructDefs, name)
				continue
			}

			// Generic member functions are templates too. They are registered on
			// the owning StructDef as well as in the module's declarations/function
			// maps, so pruning only the latter leaves the link checker trying to
			// resolve their unsubstituted type parameters (for example `T`).
			for memberName, fn := range st.Funcs {
				if isGenericFuncDecl(fn) {
					delete(st.Funcs, memberName)
				}
			}
		}

		for name, fn := range gl.FuncDefs {
			if isGenericFuncDecl(fn) {
				delete(gl.FuncDefs, name)
			}
		}

		_ = module
	}
}
