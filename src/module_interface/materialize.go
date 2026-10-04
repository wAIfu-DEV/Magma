package moduleinterface

import (
	"fmt"
	"strconv"
	"strings"

	scopeinfo "Magma/src/scope_info"
	t "Magma/src/types"
)

// Materialize creates declaration-only compiler state from an interface. The
// returned FileCtx contains no source bytes and no implementation statements.
func Materialize(value *Interface, available map[string]*Interface, sourcePath string) (*t.FileCtx, error) {
	if err := Validate(value); err != nil {
		return nil, err
	}
	packageByID := map[string]string{}
	for id, dependency := range available {
		if dependency == nil {
			continue
		}
		packageByID[id] = t.StablePackageName(dependency.ModuleName, t.ModuleID(id))
	}
	packageName := t.StablePackageName(value.ModuleName, t.ModuleID(value.ModuleID))
	gl := &t.NodeGlobal{ImportAlias: map[string]string{}, PublicImportAlias: map[string]bool{}, StructDefs: map[string]*t.StructDef{}, UnionDefs: map[string]*t.UnionDef{}, ProtoDefs: map[string]*t.ProtoDef{}, TypeAliases: map[string]*t.TypeAlias{}, FuncDefs: map[string]*t.NodeFuncDef{}, PrimitiveMethods: map[string]map[string]*t.NodeFuncDef{}, PrimitiveDestructors: map[string][]*t.NodeFuncDef{}}
	for _, imported := range value.Imports {
		dependencyPackage := packageByID[imported.ModuleID]
		if dependencyPackage == "" {
			return nil, fmt.Errorf("interface import %q references unavailable module %q", imported.Alias, imported.ModuleID)
		}
		gl.ImportAlias[imported.Alias] = dependencyPackage
	}
	for _, reexport := range value.Reexports {
		gl.PublicImportAlias[reexport.Alias] = true
	}

	for _, item := range value.Structs {
		isPublic := !item.Private
		definition := &t.StructDef{Module: packageName, Name: item.Name, IsPublic: isPublic, TypeParams: append([]string(nil), item.TypeParams...), FieldNb: map[string]int{}, Fields: map[string]*t.NodeType{}, Funcs: map[string]*t.NodeFuncDef{}, LayoutSize: item.StorageSize, LayoutAlign: item.StorageAlignment}
		args := make([]t.NodeArg, 0, len(item.Fields))
		for index, field := range item.Fields {
			typ, err := materializeType(field.Type)
			if err != nil {
				return nil, err
			}
			definition.FieldNb[field.Name] = index
			definition.Fields[field.Name] = typ
			definition.FieldOrder = append(definition.FieldOrder, field.Name)
			args = append(args, t.NodeArg{Name: field.Name, TypeNode: typ})
		}
		node := &t.NodeStructDef{Class: t.NodeGenericClass{NameNode: singleName(item.Name), TypeParams: append([]string(nil), item.TypeParams...), ArgsNode: t.NodeArgList{Args: args}}, AbsName: item.Symbol, IsPublic: isPublic}
		gl.StructDefs[item.Name] = definition
		gl.Declarations = append(gl.Declarations, node)
	}
	for _, item := range value.Unions {
		definition := &t.UnionDef{Module: packageName, Name: item.Name, IsPublic: true}
		for _, variant := range item.Variants {
			converted := &t.UnionVariant{Name: variant.Name, Tag: variant.Tag, Owner: definition}
			for _, field := range variant.Fields {
				typ, err := materializeType(field.Type)
				if err != nil {
					return nil, err
				}
				converted.Fields = append(converted.Fields, t.NodeArg{Name: field.Name, TypeNode: typ})
			}
			definition.Variants = append(definition.Variants, converted)
		}
		gl.UnionDefs[item.Name] = definition
		gl.Declarations = append(gl.Declarations, &t.NodeUnionDef{Def: definition})
	}
	for _, item := range value.Prototypes {
		proto := &t.ProtoDef{Module: packageName, Name: item.Name, IsPublic: true, TypeParams: append([]string(nil), item.TypeParams...), MethodMap: map[string]*t.ProtoMethod{}, VtableName: "__proto_" + item.Name + "_vtable"}
		for slot, method := range item.Methods {
			converted, err := materializeProtoMethod(method, proto, slot)
			if err != nil {
				return nil, err
			}
			proto.Methods = append(proto.Methods, converted)
			proto.MethodMap[converted.Name] = converted
		}
		definition := &t.StructDef{Module: packageName, Name: item.Name, IsPublic: true, IsProto: true, Proto: proto, TypeParams: append([]string(nil), item.TypeParams...), FieldNb: map[string]int{}, Fields: map[string]*t.NodeType{}, Funcs: map[string]*t.NodeFuncDef{}}
		gl.ProtoDefs[item.Name] = proto
		gl.StructDefs[item.Name] = definition
	}
	for _, item := range value.Aliases {
		target, err := materializeType(item.Target)
		if err != nil {
			return nil, err
		}
		alias := &t.TypeAlias{Name: item.Name, Module: packageName, Target: target, IsPublic: true}
		gl.TypeAliases[item.Name] = alias
		gl.Declarations = append(gl.Declarations, &t.NodeTypeAlias{Alias: alias})
	}
	functionsBySymbol := map[string]*t.NodeFuncDef{}
	for _, item := range value.Functions {
		fn, err := materializeFunction(item)
		if err != nil {
			return nil, err
		}
		gl.FuncDefs[item.Name] = fn
		gl.Declarations = append(gl.Declarations, fn)
		functionsBySymbol[item.Symbol] = fn
		if fn.IsMember {
			parts := strings.Split(item.Name, ".")
			if len(parts) > 1 && gl.StructDefs[parts[0]] != nil {
				gl.StructDefs[parts[0]].Funcs[parts[len(parts)-1]] = fn
			}
		}
	}
	for _, item := range value.Structs {
		definition := gl.StructDefs[item.Name]
		for _, symbol := range item.Destructors {
			if fn := functionsBySymbol[symbol]; fn != nil {
				definition.Destructors = append(definition.Destructors, fn)
				if definition.Destructor == nil {
					definition.Destructor = fn
				}
			}
		}
	}
	for _, item := range value.Globals {
		typ, err := materializeType(item.Type)
		if err != nil {
			return nil, err
		}
		node := &t.NodeExprVarDef{Name: singleName(item.Name), Type: typ, AbsName: item.Symbol, Storage: t.VariableStorageGlobal, IsGlobal: true, IsProcessGlobal: item.Process, IsExternal: item.External, ExternalName: item.ExternalName, IsPublic: true}
		gl.Declarations = append(gl.Declarations, node)
	}
	for _, item := range value.Constants {
		typ, err := materializeType(item.Type)
		if err != nil {
			return nil, err
		}
		initializer, err := materializeConstant(item.Value, typ)
		if err != nil {
			return nil, err
		}
		variable := &t.NodeExprVarDef{Name: singleName(item.Name), Type: typ, Initializer: initializer, AbsName: item.Symbol, Storage: t.VariableStorageGlobal, IsGlobal: true, IsConst: true, IsPublic: true}
		gl.Declarations = append(gl.Declarations, &t.NodeConstDef{VarDef: variable, Initializer: initializer})
	}
	for _, set := range value.PrimitiveDestructors {
		for _, symbol := range set.Symbols {
			if fn := functionsBySymbol[symbol]; fn != nil {
				gl.PrimitiveDestructors[set.Type] = append(gl.PrimitiveDestructors[set.Type], fn)
			}
		}
	}
	snapshot, err := Encode(value)
	if err != nil {
		return nil, err
	}
	fctx := &t.FileCtx{FilePath: sourcePath, ModuleID: t.ModuleID(value.ModuleID), ModuleName: value.ModuleName, PackageName: packageName, ImportAlias: map[string]string{}, GlNode: gl, InterfaceOnly: true, InterfaceSnapshot: snapshot}
	for alias, target := range gl.ImportAlias {
		fctx.ImportAlias[alias] = target
	}
	scope, err := scopeinfo.BuildScopeTree(fctx, gl)
	if err != nil {
		return nil, err
	}
	fctx.ScopeTree = scope
	return fctx, nil
}

func materializeFunction(item Function) (*t.NodeFuncDef, error) {
	result, err := materializeType(item.Result)
	if err != nil {
		return nil, err
	}
	args := make([]t.NodeArg, 0, len(item.Arguments))
	for _, argument := range item.Arguments {
		typ, err := materializeType(argument.Type)
		if err != nil {
			return nil, err
		}
		args = append(args, t.NodeArg{Name: argument.Name, TypeNode: typ, BoundedCount: argument.BoundedCount})
	}
	return &t.NodeFuncDef{Class: t.NodeGenericClass{NameNode: nodeName(item.Name), TypeParams: append([]string(nil), item.TypeParams...), OwnerTypeParams: append([]string(nil), item.OwnerTypeParams...), ArgsNode: t.NodeArgList{Args: args}}, ReturnType: result, AbsName: item.Symbol, ContextABI: parseContextABI(item.ContextABI), IsDestructor: item.Destructor, IsMember: item.Member, IsExternal: item.External, IsPublic: true, NoRetain: item.NoRetain, ExportName: item.ExportName, ExportABI: item.ExportABI}, nil
}
func materializeProtoMethod(item Function, proto *t.ProtoDef, slot int) (*t.ProtoMethod, error) {
	result, err := materializeType(item.Result)
	if err != nil {
		return nil, err
	}
	method := &t.ProtoMethod{Name: item.Name, Ret: result, ContextABI: parseContextABI(item.ContextABI), Slot: slot, Proto: proto}
	for _, argument := range item.Arguments {
		typ, err := materializeType(argument.Type)
		if err != nil {
			return nil, err
		}
		method.Args = append(method.Args, t.NodeArg{Name: argument.Name, TypeNode: typ, BoundedCount: argument.BoundedCount})
	}
	return method, nil
}
func materializeType(value Type) (*t.NodeType, error) {
	node := &t.NodeType{Owned: value.Owned, Throws: value.Throws}
	switch value.Kind {
	case "named":
		node.KindNode = &t.NodeTypeNamed{NameNode: nodeName(value.Name)}
		named := node.KindNode.(*t.NodeTypeNamed)
		for _, arg := range value.Generic {
			converted, err := materializeType(arg)
			if err != nil {
				return nil, err
			}
			named.GenericArgs = append(named.GenericArgs, converted)
		}
	case "absolute":
		node.KindNode = &t.NodeTypeAbsolute{AbsoluteName: value.Name}
	case "compiler_known":
		node.KindNode = &t.NodeTypeCompilerKnown{Name: value.Name}
	case "pointer", "reference", "slice":
		if value.Element == nil {
			return nil, fmt.Errorf("%s type lacks element", value.Kind)
		}
		element, err := materializeType(*value.Element)
		if err != nil {
			return nil, err
		}
		if value.Kind == "pointer" {
			node.KindNode = &t.NodeTypePointer{Kind: element.KindNode}
		} else if value.Kind == "reference" {
			node.KindNode = &t.NodeTypeRfc{Kind: element.KindNode}
		} else {
			node.KindNode = &t.NodeTypeSlice{ElemKind: element.KindNode}
		}
	case "function":
		fn := &t.NodeTypeFunc{ContextABI: parseContextABI(value.ContextABI)}
		for _, arg := range value.Arguments {
			converted, err := materializeType(arg)
			if err != nil {
				return nil, err
			}
			fn.Args = append(fn.Args, converted)
		}
		result, err := materializeType(*value.Result)
		if err != nil {
			return nil, err
		}
		fn.RetType = result
		node.KindNode = fn
	default:
		return nil, fmt.Errorf("unsupported interface type kind %q", value.Kind)
	}
	return node, nil
}
func materializeConstant(value string, typ *t.NodeType) (t.NodeExpr, error) {
	parts := strings.SplitN(value, ":", 3)
	if len(parts) == 3 && parts[0] == "lit" {
		kind, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, err
		}
		return &t.NodeExprLit{Value: parts[2], LitType: t.TokType(kind), InfType: typ}, nil
	}
	return nil, fmt.Errorf("interface constant encoding %q cannot be materialized", value)
}
func singleName(value string) *t.NodeNameSingle { return &t.NodeNameSingle{Name: value} }
func nodeName(value string) t.NodeName {
	parts := strings.Split(value, ".")
	if len(parts) == 1 {
		return singleName(value)
	}
	return &t.NodeNameComposite{Parts: parts}
}
func parseContextABI(value string) t.ContextABI {
	if value == "contextless" {
		return t.ContextABIContextless
	}
	return t.ContextABIContextful
}
