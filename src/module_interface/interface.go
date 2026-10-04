// Package moduleinterface defines Magma's deterministic, backend-independent
// semantic module interface format.
package moduleinterface

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	llvmir "Magma/src/llvm_ir"
	t "Magma/src/types"
)

const SchemaVersion = 3

type Interface struct {
	Schema               int             `json:"schema"`
	Compiler             string          `json:"compiler"`
	Language             string          `json:"language"`
	ModuleID             string          `json:"module_id"`
	ModuleName           string          `json:"module_name"`
	Dependencies         []string        `json:"dependencies,omitempty"`
	Imports              []Import        `json:"imports,omitempty"`
	Reexports            []Reexport      `json:"reexports,omitempty"`
	Functions            []Function      `json:"functions,omitempty"`
	Structs              []Struct        `json:"structs,omitempty"`
	Unions               []Union         `json:"unions,omitempty"`
	Prototypes           []Prototype     `json:"prototypes,omitempty"`
	Aliases              []Alias         `json:"aliases,omitempty"`
	Globals              []Global        `json:"globals,omitempty"`
	Constants            []Constant      `json:"constants,omitempty"`
	PrimitiveDestructors []DestructorSet `json:"primitive_destructors,omitempty"`
}

type Reexport struct {
	Alias    string `json:"alias"`
	ModuleID string `json:"module_id"`
}

type Import struct {
	Alias    string `json:"alias"`
	ModuleID string `json:"module_id"`
}

type Type struct {
	Kind       string `json:"kind"`
	Name       string `json:"name,omitempty"`
	Owned      bool   `json:"owned,omitempty"`
	Throws     bool   `json:"throws,omitempty"`
	ContextABI string `json:"context_abi,omitempty"`
	Element    *Type  `json:"element,omitempty"`
	Arguments  []Type `json:"arguments,omitempty"`
	Result     *Type  `json:"result,omitempty"`
	Generic    []Type `json:"generic,omitempty"`
}

type Argument struct {
	Name         string `json:"name"`
	Type         Type   `json:"type"`
	BoundedCount string `json:"bounded_count,omitempty"`
}

type Function struct {
	Name            string     `json:"name"`
	Symbol          string     `json:"symbol"`
	Arguments       []Argument `json:"arguments,omitempty"`
	Result          Type       `json:"result"`
	TypeParams      []string   `json:"type_params,omitempty"`
	OwnerTypeParams []string   `json:"owner_type_params,omitempty"`
	ContextABI      string     `json:"context_abi"`
	Destructor      bool       `json:"destructor,omitempty"`
	Member          bool       `json:"member,omitempty"`
	External        bool       `json:"external,omitempty"`
	NoRetain        bool       `json:"no_retain,omitempty"`
	ExportName      string     `json:"export_name,omitempty"`
	ExportABI       string     `json:"export_abi,omitempty"`
}

type Field struct {
	Name string `json:"name"`
	Type Type   `json:"type"`
}

type Struct struct {
	Name             string   `json:"name"`
	Symbol           string   `json:"symbol"`
	Private          bool     `json:"private,omitempty"`
	TypeParams       []string `json:"type_params,omitempty"`
	Fields           []Field  `json:"fields,omitempty"`
	Destructors      []string `json:"destructors,omitempty"`
	Implements       []Type   `json:"implements,omitempty"`
	StorageSize      int      `json:"storage_size,omitempty"`
	StorageAlignment int      `json:"storage_alignment,omitempty"`
}

type UnionVariant struct {
	Name   string     `json:"name"`
	Tag    int        `json:"tag"`
	Fields []Argument `json:"fields,omitempty"`
}
type Union struct {
	Name       string         `json:"name"`
	Variants   []UnionVariant `json:"variants"`
	Implements []Type         `json:"implements,omitempty"`
}
type Prototype struct {
	Name       string     `json:"name"`
	TypeParams []string   `json:"type_params,omitempty"`
	Methods    []Function `json:"methods"`
}
type Alias struct {
	Name   string `json:"name"`
	Target Type   `json:"target"`
}
type Global struct {
	Name         string `json:"name"`
	Symbol       string `json:"symbol"`
	Type         Type   `json:"type"`
	Process      bool   `json:"process,omitempty"`
	External     bool   `json:"external,omitempty"`
	ExternalName string `json:"external_name,omitempty"`
}
type Constant struct {
	Name   string `json:"name"`
	Symbol string `json:"symbol"`
	Type   Type   `json:"type"`
	Value  string `json:"value"`
}
type DestructorSet struct {
	Type    string   `json:"type"`
	Symbols []string `json:"symbols"`
}

func Generate(state *t.SharedState, file *t.FileCtx, compilerVersion string) (*Interface, error) {
	if state == nil || file == nil || file.GlNode == nil || file.ModuleID == "" {
		return nil, fmt.Errorf("module interface requires a parsed module with stable identity")
	}
	if compilerVersion == "" {
		return nil, fmt.Errorf("module interface requires a compiler version")
	}
	out := &Interface{Schema: SchemaVersion, Compiler: compilerVersion, Language: "magma-v1", ModuleID: string(file.ModuleID), ModuleName: file.ModuleName}
	byPackage := make(map[string]*t.FileCtx, len(state.Files))
	for _, candidate := range state.Files {
		byPackage[candidate.PackageName] = candidate
	}
	seenDeps := map[string]bool{}
	for alias, packageName := range file.GlNode.ImportAlias {
		if dependency := byPackage[packageName]; dependency != nil {
			id := string(dependency.ModuleID)
			out.Imports = append(out.Imports, Import{Alias: alias, ModuleID: id})
			if !seenDeps[id] {
				out.Dependencies = append(out.Dependencies, id)
				seenDeps[id] = true
			}
		}
	}
	sort.Strings(out.Dependencies)
	sort.Slice(out.Imports, func(i, j int) bool { return out.Imports[i].Alias < out.Imports[j].Alias })
	for alias := range file.GlNode.PublicImportAlias {
		dependency := byPackage[file.GlNode.ImportAlias[alias]]
		if dependency == nil {
			return nil, fmt.Errorf("public re-export %q has no loaded module", alias)
		}
		out.Reexports = append(out.Reexports, Reexport{Alias: alias, ModuleID: string(dependency.ModuleID)})
	}
	sort.Slice(out.Reexports, func(i, j int) bool { return out.Reexports[i].Alias < out.Reexports[j].Alias })

	requiredFunctionSymbols := map[string]bool{}
	for _, definition := range file.GlNode.StructDefs {
		if definition == nil || !definition.IsPublic {
			continue
		}
		for _, destructor := range definition.Destructors {
			if destructor != nil {
				requiredFunctionSymbols[destructor.AbsName] = true
			}
		}
		if definition.Destructor != nil {
			requiredFunctionSymbols[definition.Destructor.AbsName] = true
		}
	}
	functionNames := sortedKeys(file.GlNode.FuncDefs)
	for _, name := range functionNames {
		fn := file.GlNode.FuncDefs[name]
		publicOwnerMethod := false
		if fn != nil && fn.IsMember {
			if composite, ok := fn.Class.NameNode.(*t.NodeNameComposite); ok && len(composite.Parts) > 1 {
				if owner := file.GlNode.StructDefs[composite.Parts[0]]; owner != nil {
					publicOwnerMethod = owner.IsPublic
				}
			}
		}
		if fn != nil && (fn.IsPublic || publicOwnerMethod || requiredFunctionSymbols[fn.AbsName]) {
			converted, err := function(fn)
			if err != nil {
				return nil, err
			}
			out.Functions = append(out.Functions, converted)
		}
	}
	for _, name := range sortedKeys(file.GlNode.StructDefs) {
		definition := file.GlNode.StructDefs[name]
		if definition == nil || definition.IsProto || file.GlNode.UnionDefs[name] != nil {
			continue
		}
		// Private structs can be layout dependencies of public types (for
		// example a public handle containing a pointer to a private state
		// record). Preserve their definitions in the interface without making
		// their names accessible to clients.
		value := Struct{Name: definition.Name, Symbol: definition.Module + "." + definition.Name, Private: !definition.IsPublic, TypeParams: append([]string(nil), definition.TypeParams...)}
		for _, fieldName := range definition.FieldOrder {
			typ, err := typeOf(definition.Fields[fieldName])
			if err != nil {
				return nil, err
			}
			value.Fields = append(value.Fields, Field{Name: fieldName, Type: typ})
		}
		for _, destructor := range definition.Destructors {
			if destructor != nil {
				value.Destructors = append(value.Destructors, destructor.AbsName)
			}
		}
		if len(value.Destructors) == 0 && definition.Destructor != nil {
			value.Destructors = append(value.Destructors, definition.Destructor.AbsName)
		}
		for _, implementation := range definition.Implements {
			if implementation == nil {
				continue
			}
			implemented, err := typeOf(implementation.Type)
			if err != nil {
				return nil, err
			}
			value.Implements = append(value.Implements, implemented)
		}
		if len(value.Implements) != 0 {
			size, alignment, err := llvmir.TypeSizeAndAlignment(state, &t.NodeType{KindNode: &t.NodeTypeAbsolute{AbsoluteName: definition.Module + "." + definition.Name}})
			if err != nil {
				return nil, err
			}
			value.StorageSize, value.StorageAlignment = size, alignment
		}
		sort.Strings(value.Destructors)
		out.Structs = append(out.Structs, value)
	}
	for _, name := range sortedKeys(file.GlNode.UnionDefs) {
		definition := file.GlNode.UnionDefs[name]
		if definition == nil || !definition.IsPublic {
			continue
		}
		value := Union{Name: definition.Name}
		if layout := file.GlNode.StructDefs[name]; layout != nil {
			for _, implementation := range layout.Implements {
				if implementation == nil {
					continue
				}
				implemented, err := typeOf(implementation.Type)
				if err != nil {
					return nil, err
				}
				value.Implements = append(value.Implements, implemented)
			}
		}
		for _, variant := range definition.Variants {
			item := UnionVariant{Name: variant.Name, Tag: variant.Tag}
			for _, arg := range variant.Fields {
				typ, err := typeOf(arg.TypeNode)
				if err != nil {
					return nil, err
				}
				item.Fields = append(item.Fields, Argument{Name: arg.Name, Type: typ})
			}
			value.Variants = append(value.Variants, item)
		}
		out.Unions = append(out.Unions, value)
	}
	for _, name := range sortedKeys(file.GlNode.ProtoDefs) {
		definition := file.GlNode.ProtoDefs[name]
		if definition == nil || !definition.IsPublic {
			continue
		}
		value := Prototype{Name: definition.Name, TypeParams: append([]string(nil), definition.TypeParams...)}
		for _, method := range definition.Methods {
			converted, err := protoFunction(method)
			if err != nil {
				return nil, err
			}
			value.Methods = append(value.Methods, converted)
		}
		out.Prototypes = append(out.Prototypes, value)
	}
	for _, name := range sortedKeys(file.GlNode.TypeAliases) {
		definition := file.GlNode.TypeAliases[name]
		if definition == nil || !definition.IsPublic {
			continue
		}
		typ, err := typeOf(definition.Target)
		if err != nil {
			return nil, err
		}
		out.Aliases = append(out.Aliases, Alias{Name: definition.Name, Target: typ})
	}
	for _, declaration := range file.GlNode.Declarations {
		switch node := declaration.(type) {
		case *t.NodeExprVarDef:
			if !node.IsPublic {
				continue
			}
			typ, err := typeOf(node.Type)
			if err != nil {
				return nil, err
			}
			out.Globals = append(out.Globals, Global{Name: nameOf(node.Name), Symbol: node.AbsName, Type: typ, Process: node.IsProcessGlobal, External: node.IsExternal, ExternalName: node.ExternalName})
		case *t.NodeConstDef:
			if node.VarDef == nil || !node.VarDef.IsPublic {
				continue
			}
			typ, err := typeOf(node.VarDef.Type)
			if err != nil {
				return nil, err
			}
			value, err := constantExpression(state, file, node.Initializer, map[*t.NodeExprVarDef]bool{})
			if err != nil {
				return nil, fmt.Errorf("public constant %s: %w", nameOf(node.VarDef.Name), err)
			}
			out.Constants = append(out.Constants, Constant{Name: nameOf(node.VarDef.Name), Symbol: node.VarDef.AbsName, Type: typ, Value: value})
		}
	}
	for _, primitive := range sortedKeys(file.GlNode.PrimitiveDestructors) {
		set := DestructorSet{Type: primitive}
		for _, destructor := range file.GlNode.PrimitiveDestructors[primitive] {
			if destructor != nil {
				set.Symbols = append(set.Symbols, destructor.AbsName)
			}
		}
		sort.Strings(set.Symbols)
		if len(set.Symbols) > 0 {
			out.PrimitiveDestructors = append(out.PrimitiveDestructors, set)
		}
	}
	sort.Slice(out.Globals, func(i, j int) bool { return out.Globals[i].Name < out.Globals[j].Name })
	sort.Slice(out.Constants, func(i, j int) bool { return out.Constants[i].Name < out.Constants[j].Name })
	if err := Validate(out); err != nil {
		return nil, err
	}
	return out, nil
}

func Encode(value *Interface) ([]byte, error) {
	if err := Validate(value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}
func Decode(data []byte) (*Interface, error) {
	var value Interface
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode module interface: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("decode module interface: trailing data")
	}
	if err := Validate(&value); err != nil {
		return nil, err
	}
	return &value, nil
}
func Hash(value *Interface) (string, error) {
	data, err := Encode(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func VerifyImplementation(state *t.SharedState, file *t.FileCtx, expected *Interface, compilerVersion string) error {
	actual, err := Generate(state, file, compilerVersion)
	if err != nil {
		return err
	}
	actualBytes, err := Encode(actual)
	if err != nil {
		return err
	}
	expectedBytes, err := Encode(expected)
	if err != nil {
		return err
	}
	if !bytes.Equal(actualBytes, expectedBytes) {
		return fmt.Errorf("module %q implementation does not match its published interface", file.ModuleName)
	}
	return nil
}

func WriteFile(path string, value *Interface) error {
	encoded, err := Encode(value)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, encoded, 0o666); err != nil {
		return fmt.Errorf("write module interface %q: %w", path, err)
	}
	return nil
}

func ReadFile(path string) (*Interface, error) {
	encoded, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read module interface %q: %w", path, err)
	}
	return Decode(encoded)
}

func ValidateCompiler(value *Interface, compilerVersion string) error {
	if err := Validate(value); err != nil {
		return err
	}
	if compilerVersion == "" {
		return fmt.Errorf("expected compiler version is empty")
	}
	if value.Compiler != compilerVersion {
		return fmt.Errorf("module interface compiler %q is incompatible with %q", value.Compiler, compilerVersion)
	}
	return nil
}

func Validate(value *Interface) error {
	if value == nil {
		return fmt.Errorf("module interface is nil")
	}
	if value.Schema != SchemaVersion {
		return fmt.Errorf("unsupported module interface schema %d", value.Schema)
	}
	if value.Language != "magma-v1" {
		return fmt.Errorf("unsupported module interface language %q", value.Language)
	}
	if value.Compiler == "" || value.ModuleID == "" || value.ModuleName == "" {
		return fmt.Errorf("module interface identity is incomplete")
	}
	seen := map[string]string{}
	add := func(kind, name string) error {
		key := kind + "\x00" + name
		if prior := seen[key]; prior != "" {
			return fmt.Errorf("duplicate %s %q", kind, name)
		}
		seen[key] = kind
		return nil
	}
	for _, fn := range value.Functions {
		if err := add("function", fn.Name); err != nil {
			return err
		}
		if err := validateType(fn.Result); err != nil {
			return err
		}
		for _, arg := range fn.Arguments {
			if err := validateType(arg.Type); err != nil {
				return err
			}
		}
	}
	for _, item := range value.Structs {
		if err := add("type", item.Name); err != nil {
			return err
		}
		if len(item.Implements) != 0 && (item.StorageSize < 0 || item.StorageAlignment < 1) {
			return fmt.Errorf("public implementation %q lacks valid storage layout", item.Name)
		}
		for _, field := range item.Fields {
			if err := validateType(field.Type); err != nil {
				return err
			}
		}
		for _, implemented := range item.Implements {
			if err := validateType(implemented); err != nil {
				return err
			}
		}
	}
	for _, item := range value.Unions {
		if err := add("type", item.Name); err != nil {
			return err
		}
		for _, variant := range item.Variants {
			for _, field := range variant.Fields {
				if err := validateType(field.Type); err != nil {
					return err
				}
			}
		}
		for _, implemented := range item.Implements {
			if err := validateType(implemented); err != nil {
				return err
			}
		}
	}
	for _, item := range value.Prototypes {
		if err := add("type", item.Name); err != nil {
			return err
		}
		for _, method := range item.Methods {
			if err := validateType(method.Result); err != nil {
				return err
			}
			for _, argument := range method.Arguments {
				if err := validateType(argument.Type); err != nil {
					return err
				}
			}
		}
	}
	for _, item := range value.Aliases {
		if err := add("type", item.Name); err != nil {
			return err
		}
		if err := validateType(item.Target); err != nil {
			return err
		}
	}
	for _, item := range value.Globals {
		if err := add("global", item.Name); err != nil {
			return err
		}
		if err := validateType(item.Type); err != nil {
			return err
		}
	}
	for _, item := range value.Constants {
		if err := add("constant", item.Name); err != nil {
			return err
		}
		if err := validateType(item.Type); err != nil {
			return err
		}
		if item.Value == "" {
			return fmt.Errorf("public constant %q has no canonical value", item.Name)
		}
	}
	reexports := map[string]bool{}
	for _, item := range value.Reexports {
		if item.Alias == "" || item.ModuleID == "" {
			return fmt.Errorf("invalid public re-export")
		}
		if reexports[item.Alias] {
			return fmt.Errorf("duplicate public re-export %q", item.Alias)
		}
		reexports[item.Alias] = true
	}
	imports := map[string]bool{}
	for _, item := range value.Imports {
		if item.Alias == "" || item.ModuleID == "" {
			return fmt.Errorf("invalid module import")
		}
		if imports[item.Alias] {
			return fmt.Errorf("duplicate module import %q", item.Alias)
		}
		imports[item.Alias] = true
	}
	primitiveSets := map[string]bool{}
	for _, item := range value.PrimitiveDestructors {
		if item.Type == "" || len(item.Symbols) == 0 {
			return fmt.Errorf("invalid primitive destructor set")
		}
		if primitiveSets[item.Type] {
			return fmt.Errorf("duplicate primitive destructor set %q", item.Type)
		}
		primitiveSets[item.Type] = true
	}
	return nil
}

func validateType(value Type) error {
	switch value.Kind {
	case "named", "absolute", "compiler_known":
		if value.Name == "" {
			return fmt.Errorf("interface %s type has no name", value.Kind)
		}
	case "pointer", "reference", "slice":
		if value.Element == nil {
			return fmt.Errorf("interface %s type has no element", value.Kind)
		}
	case "function":
		if value.Result == nil {
			return fmt.Errorf("interface function type has no result")
		}
	default:
		return fmt.Errorf("interface contains unknown type kind %q", value.Kind)
	}
	if value.Element != nil {
		if err := validateType(*value.Element); err != nil {
			return err
		}
	}
	for _, item := range value.Arguments {
		if err := validateType(item); err != nil {
			return err
		}
	}
	if value.Result != nil {
		return validateType(*value.Result)
	}
	for _, item := range value.Generic {
		if err := validateType(item); err != nil {
			return err
		}
	}
	return nil
}
func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
func nameOf(value t.NodeName) string {
	switch n := value.(type) {
	case *t.NodeNameSingle:
		return n.Name
	case *t.NodeNameComposite:
		return strings.Join(n.Parts, ".")
	}
	return ""
}

func function(fn *t.NodeFuncDef) (Function, error) {
	result, err := typeOf(fn.ReturnType)
	if err != nil {
		return Function{}, err
	}
	value := Function{Name: nameOf(fn.Class.NameNode), Symbol: fn.AbsName, Result: result, TypeParams: append([]string(nil), fn.Class.TypeParams...), OwnerTypeParams: append([]string(nil), fn.Class.OwnerTypeParams...), ContextABI: contextABI(fn.ContextABI), Destructor: fn.IsDestructor, Member: fn.IsMember, External: fn.IsExternal, NoRetain: fn.NoRetain, ExportName: fn.ExportName, ExportABI: fn.ExportABI}
	for _, arg := range fn.Class.ArgsNode.Args {
		typ, err := typeOf(arg.TypeNode)
		if err != nil {
			return Function{}, err
		}
		value.Arguments = append(value.Arguments, Argument{Name: arg.Name, Type: typ, BoundedCount: arg.BoundedCount})
	}
	return value, nil
}
func protoFunction(method *t.ProtoMethod) (Function, error) {
	value := Function{Name: method.Name, ContextABI: contextABI(method.ContextABI)}
	for _, arg := range method.Args {
		typ, err := typeOf(arg.TypeNode)
		if err != nil {
			return Function{}, err
		}
		value.Arguments = append(value.Arguments, Argument{Name: arg.Name, Type: typ, BoundedCount: arg.BoundedCount})
	}
	var err error
	value.Result, err = typeOf(method.Ret)
	return value, err
}
func contextABI(value t.ContextABI) string {
	if value == t.ContextABIContextless {
		return "contextless"
	}
	return "contextful"
}

func typeOf(node *t.NodeType) (Type, error) {
	if node == nil {
		return Type{}, fmt.Errorf("interface contains unresolved nil type")
	}
	out := Type{Owned: node.Owned, Throws: node.Throws}
	switch kind := node.KindNode.(type) {
	case *t.NodeTypeNamed:
		out.Kind = "named"
		out.Name = nameOf(kind.NameNode)
		for _, arg := range kind.GenericArgs {
			v, e := typeOf(arg)
			if e != nil {
				return Type{}, e
			}
			out.Generic = append(out.Generic, v)
		}
	case *t.NodeTypeAbsolute:
		out.Kind = "absolute"
		out.Name = kind.AbsoluteName
	case *t.NodeTypeCompilerKnown:
		out.Kind = "compiler_known"
		out.Name = kind.Name
	case *t.NodeTypePointer:
		out.Kind = "pointer"
		v, e := typeKind(kind.Kind)
		if e != nil {
			return Type{}, e
		}
		out.Element = &v
	case *t.NodeTypeRfc:
		out.Kind = "reference"
		v, e := typeKind(kind.Kind)
		if e != nil {
			return Type{}, e
		}
		out.Element = &v
	case *t.NodeTypeSlice:
		out.Kind = "slice"
		v, e := typeKind(kind.ElemKind)
		if e != nil {
			return Type{}, e
		}
		out.Element = &v
	case *t.NodeTypeFunc:
		out.Kind = "function"
		out.ContextABI = contextABI(kind.ContextABI)
		for _, arg := range kind.Args {
			v, e := typeOf(arg)
			if e != nil {
				return Type{}, e
			}
			out.Arguments = append(out.Arguments, v)
		}
		v, e := typeOf(kind.RetType)
		if e != nil {
			return Type{}, e
		}
		out.Result = &v
	default:
		return Type{}, fmt.Errorf("unsupported interface type %T", node.KindNode)
	}
	return out, nil
}
func typeKind(kind t.NodeTypeKind) (Type, error) { return typeOf(&t.NodeType{KindNode: kind}) }

func constantExpression(state *t.SharedState, file *t.FileCtx, expr t.NodeExpr, visiting map[*t.NodeExprVarDef]bool) (string, error) {
	switch n := expr.(type) {
	case *t.NodeExprLit:
		return fmt.Sprintf("lit:%d:%s", n.LitType, n.Value), nil
	case *t.NodeExprName:
		if variable := referencedConstant(state, file, n); variable != nil {
			if visiting[variable] {
				return "", fmt.Errorf("constant reference cycle through %s", nameOf(variable.Name))
			}
			visiting[variable] = true
			value, err := constantExpression(state, fileForConstant(state, variable), variable.Initializer, visiting)
			delete(visiting, variable)
			return value, err
		}
		return "name:" + nameOf(n.Name), nil
	case *t.NodeExprUnary:
		v, e := constantExpression(state, file, n.Operand, visiting)
		return fmt.Sprintf("unary:%d:(%s)", n.Operator, v), e
	case *t.NodeExprBinary:
		left, e := constantExpression(state, file, n.Left, visiting)
		if e != nil {
			return "", e
		}
		right, e := constantExpression(state, file, n.Right, visiting)
		return fmt.Sprintf("binary:%d:(%s):(%s)", n.Operator, left, right), e
	case *t.NodeExprSizeof:
		typ, e := typeOf(n.Type)
		if e != nil {
			return "", e
		}
		data, _ := json.Marshal(typ)
		return "sizeof:" + string(data), nil
	default:
		return "", fmt.Errorf("unsupported initializer %T", expr)
	}
}

func referencedConstant(state *t.SharedState, file *t.FileCtx, expression *t.NodeExprName) *t.NodeExprVarDef {
	if variable, ok := expression.AssociatedNode.(*t.NodeExprVarDef); ok && variable.IsConst {
		return variable
	}
	parts := strings.Split(nameOf(expression.Name), ".")
	target := file
	name := parts[len(parts)-1]
	if len(parts) > 1 {
		packageName := file.GlNode.ImportAlias[parts[0]]
		for _, candidate := range state.Files {
			if candidate.PackageName == packageName {
				target = candidate
				break
			}
		}
	}
	if target == nil || target.GlNode == nil {
		return nil
	}
	for _, declaration := range target.GlNode.Declarations {
		if constant, ok := declaration.(*t.NodeConstDef); ok && constant.VarDef != nil && nameOf(constant.VarDef.Name) == name {
			return constant.VarDef
		}
	}
	return nil
}

func fileForConstant(state *t.SharedState, variable *t.NodeExprVarDef) *t.FileCtx {
	for _, file := range state.Files {
		if file == nil || file.GlNode == nil {
			continue
		}
		for _, declaration := range file.GlNode.Declarations {
			if constant, ok := declaration.(*t.NodeConstDef); ok && constant.VarDef == variable {
				return file
			}
		}
	}
	return nil
}
