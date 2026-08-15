package monomorph

import (
	t "Magma/src/types"
	"fmt"
)

type genericInstantiationFailure struct {
	kind     string
	name     string
	expected int
	got      int
	unknown  bool
}

func (e *genericInstantiationFailure) Error() string {
	if e.unknown {
		return fmt.Sprintf("unknown generic %s template: %s", e.kind, e.name)
	}
	return fmt.Sprintf("generic %s '%s' expects %d type args but got %d", e.kind, e.name, e.expected, e.got)
}

func (m *monoCtx) instantiateStruct(module string, baseName string, args []*t.NodeType) (string, error) {
	templateKey := makeTemplateKey(module, baseName)
	template, ok := m.structTemplates[templateKey]
	if !ok {
		return "", &genericInstantiationFailure{kind: "struct", name: baseName, unknown: true}
	}

	if len(template.Class.TypeParams) != len(args) {
		return "", &genericInstantiationFailure{kind: "struct", name: baseName, expected: len(template.Class.TypeParams), got: len(args)}
	}

	instanceKey := makeInstanceKey(module, baseName, args)
	if n, ok := m.structInstances[instanceKey]; ok {
		return n, nil
	}

	specName := MangleSpecializedName(baseName, args)
	m.structInstances[instanceKey] = specName
	structDisplayName := m.genericDisplayName(sourceModuleName(module)+"."+baseName, args)
	m.structDisplayNames[module+"."+specName] = structDisplayName

	gl := m.modules[module]

	specStruct := cloneStructDef(template)
	specStruct.Class.NameNode = &t.NodeNameSingle{Name: specName}
	specStruct.AbsName = module + "." + specName
	specStruct.Class.TypeParams = nil

	subst := map[string]*t.NodeType{}
	for i, p := range template.Class.TypeParams {
		subst[p] = cloneType(args[i])
	}

	for i := range specStruct.Class.ArgsNode.Args {
		specStruct.Class.ArgsNode.Args[i].TypeNode = substituteType(specStruct.Class.ArgsNode.Args[i].TypeNode, subst)
	}

	origDef := gl.StructDefs[baseName]
	stDef := &t.StructDef{
		Module:     module,
		Name:       specName,
		IsPublic:   origDef.IsPublic,
		TypeParams: nil,
		FieldNb:    map[string]int{},
		Fields:     map[string]*t.NodeType{},
		FieldOrder: []string{},
		Funcs:      map[string]*t.NodeFuncDef{},
	}
	for _, relation := range origDef.Implements {
		stDef.Implements = append(stDef.Implements, &t.ProtoImpl{Type: substituteType(cloneType(relation.Type), subst), Tk: relation.Tk})
	}
	for i, fld := range specStruct.Class.ArgsNode.Args {
		stDef.FieldNb[fld.Name] = i
		stDef.Fields[fld.Name] = cloneType(fld.TypeNode)
		stDef.FieldOrder = append(stDef.FieldOrder, fld.Name)
	}

	for memberName, fnTpl := range origDef.Funcs {
		if len(fnTpl.Class.TypeParams) > 0 {
			memberTpl := cloneFuncDef(fnTpl)
			memberTpl.Class.OwnerTypeParams = nil
			memberTpl.Class.NameNode = &t.NodeNameComposite{
				Parts: []string{specName, memberName},
			}
			for i := range memberTpl.Class.ArgsNode.Args {
				memberTpl.Class.ArgsNode.Args[i].TypeNode = substituteType(memberTpl.Class.ArgsNode.Args[i].TypeNode, subst)
			}
			memberTpl.ReturnType = substituteType(memberTpl.ReturnType, subst)
			for _, s := range memberTpl.Body.Statements {
				substituteStmt(s, subst)
			}
			memberTpl.AbsName = module + "." + flattenName(memberTpl.Class.NameNode)
			m.memberTemplates[makeMemberTemplateKey(module, specName, memberName)] = memberTpl
			continue
		}

		specFn := cloneFuncDef(fnTpl)
		specFn.Class.OwnerTypeParams = nil
		specFn.Class.NameNode = &t.NodeNameComposite{
			Parts: []string{specName, memberName},
		}
		for i := range specFn.Class.ArgsNode.Args {
			specFn.Class.ArgsNode.Args[i].TypeNode = substituteType(specFn.Class.ArgsNode.Args[i].TypeNode, subst)
		}
		specFn.ReturnType = substituteType(specFn.ReturnType, subst)
		for _, s := range specFn.Body.Statements {
			substituteStmt(s, subst)
		}
		specFn.AbsName = module + "." + flattenName(specFn.Class.NameNode)
		specFn.DisplayName = unqualifiedDisplayName(structDisplayName) + "." + memberName

		key := specName + "." + memberName
		gl.FuncDefs[key] = specFn
		gl.Declarations = append(gl.Declarations, specFn)
		stDef.Funcs[memberName] = specFn
		if specFn.IsDestructor {
			stDef.Destructors = append(stDef.Destructors, specFn)
			if stDef.Destructor == nil {
				stDef.Destructor = specFn
			}
		}
		m.queueFunc(specFn)
	}

	gl.StructDefs[specName] = stDef
	gl.Declarations = append(gl.Declarations, specStruct)
	m.queueStruct(module, specStruct)

	return specName, nil
}

func (m *monoCtx) instantiateFunc(module string, baseName string, args []*t.NodeType) (string, error) {
	templateKey := makeTemplateKey(module, baseName)
	template, ok := m.funcTemplates[templateKey]
	if !ok {
		return "", &genericInstantiationFailure{kind: "function", name: baseName, unknown: true}
	}

	if len(template.Class.TypeParams) != len(args) {
		return "", &genericInstantiationFailure{kind: "function", name: baseName, expected: len(template.Class.TypeParams), got: len(args)}
	}

	instanceKey := makeInstanceKey(module, baseName, args)
	if n, ok := m.funcInstances[instanceKey]; ok {
		return n, nil
	}

	specName := MangleSpecializedName(baseName, args)
	m.funcInstances[instanceKey] = specName

	gl := m.modules[module]
	specFn := cloneFuncDef(template)
	specFn.Class.TypeParams = nil
	specFn.Class.NameNode = &t.NodeNameSingle{Name: specName}
	specFn.DisplayName = m.genericDisplayName(baseName, args)

	subst := map[string]*t.NodeType{}
	for i, p := range template.Class.TypeParams {
		subst[p] = cloneType(args[i])
	}

	for i := range specFn.Class.ArgsNode.Args {
		specFn.Class.ArgsNode.Args[i].TypeNode = substituteType(specFn.Class.ArgsNode.Args[i].TypeNode, subst)
	}
	specFn.ReturnType = substituteType(specFn.ReturnType, subst)
	for _, s := range specFn.Body.Statements {
		substituteStmt(s, subst)
	}

	specFn.AbsName = module + "." + specName
	gl.FuncDefs[specName] = specFn
	gl.Declarations = append(gl.Declarations, specFn)
	m.queueFunc(specFn)
	return specName, nil
}

func (m *monoCtx) instantiateMemberFunc(module string, ownerName string, memberName string, args []*t.NodeType) (string, error) {
	templateKey := makeMemberTemplateKey(module, ownerName, memberName)
	template, ok := m.memberTemplates[templateKey]
	if !ok {
		return "", &genericInstantiationFailure{kind: "member function", name: ownerName + "." + memberName, unknown: true}
	}

	if len(template.Class.TypeParams) != len(args) {
		return "", &genericInstantiationFailure{kind: "member function", name: ownerName + "." + memberName, expected: len(template.Class.TypeParams), got: len(args)}
	}

	instanceKey := makeMemberInstanceKey(module, ownerName, memberName, args)
	if n, ok := m.memberInstances[instanceKey]; ok {
		return n, nil
	}

	specMemberName := MangleSpecializedName(memberName, args)
	m.memberInstances[instanceKey] = specMemberName

	gl := m.modules[module]
	if gl == nil {
		return "", fmt.Errorf("missing module '%s'", module)
	}

	specFn := cloneFuncDef(template)
	specFn.Class.OwnerTypeParams = nil
	specFn.Class.TypeParams = nil
	specFn.Class.NameNode = &t.NodeNameComposite{
		Parts: []string{ownerName, specMemberName},
	}
	ownerDisplayName := ownerName
	if display, ok := m.structDisplayNames[module+"."+ownerName]; ok {
		ownerDisplayName = unqualifiedDisplayName(display)
	}
	specFn.DisplayName = ownerDisplayName + "." + m.genericDisplayName(memberName, args)

	subst := map[string]*t.NodeType{}
	for i, p := range template.Class.TypeParams {
		subst[p] = cloneType(args[i])
	}

	for i := range specFn.Class.ArgsNode.Args {
		specFn.Class.ArgsNode.Args[i].TypeNode = substituteType(specFn.Class.ArgsNode.Args[i].TypeNode, subst)
	}
	specFn.ReturnType = substituteType(specFn.ReturnType, subst)
	for _, s := range specFn.Body.Statements {
		substituteStmt(s, subst)
	}

	specFn.AbsName = module + "." + flattenName(specFn.Class.NameNode)
	gl.FuncDefs[ownerName+"."+specMemberName] = specFn
	gl.Declarations = append(gl.Declarations, specFn)
	m.queueFunc(specFn)

	stDef, ok := gl.StructDefs[ownerName]
	if !ok {
		return "", fmt.Errorf("missing owner struct definition '%s' in module '%s'", ownerName, module)
	}
	stDef.Funcs[specMemberName] = specFn

	return specMemberName, nil
}
