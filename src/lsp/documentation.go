package lsp

import (
	"Magma/src/types"
	"fmt"
	"reflect"
	"strings"
)

type documentation struct {
	description []string
	params      []docParam
	returns     string
	warnings    []string
	notes       []string
	complexity  []string
	throws      []string
	ownership   []string
	safety      []string
	mustCall    []string
	platforms   []string
	deprecated  []string
	see         []string
	examples    []string
}

type docParam struct{ name, text string }

type docIndex struct {
	byNode       map[any]string
	modules      map[string]string
	symbols      map[string]string
	hoverSymbols map[string]string
	hoverByName  map[string]string
	valueHovers  map[string]string
}

func buildDocIndex(state *types.SharedState) *docIndex {
	index := &docIndex{byNode: map[any]string{}, modules: map[string]string{}, symbols: map[string]string{}, hoverSymbols: map[string]string{}, hoverByName: map[string]string{}, valueHovers: map[string]string{}}
	for _, file := range state.Files {
		if file == nil || file.GlNode == nil {
			continue
		}
		byLine, module := parseDocumentation(string(file.Content))
		if text := module.markdown(); text != "" {
			index.modules[file.PackageName] = text
		}
		for _, declaration := range file.GlNode.Declarations {
			switch node := declaration.(type) {
			case *types.NodeFuncDef:
				name := flattenName(node.Class.NameNode)
				docs := index.add(file, name, nameLine(node.Class.NameNode), node, byLine)
				index.addHover(file.PackageName, name, joinHover(code(formatFunction(node)), docs))
				index.indexFunctionValueUsages(file.PackageName, node)
			case *types.NodeStructDef:
				name := flattenName(node.Class.NameNode)
				text := index.add(file, name, nameLine(node.Class.NameNode), node, byLine)
				index.addHover(file.PackageName, name, joinHover(code("struct "+name), text))
				for _, field := range node.Class.ArgsNode.Args {
					index.hoverSymbols[file.PackageName+"\x00"+name+"."+field.Name] = code(field.Name + " " + formatType(field.TypeNode))
				}
				if definition := file.GlNode.StructDefs[name]; definition != nil && text != "" {
					index.byNode[definition] = text
				}
			case *types.NodeTypeAlias:
				if node.Alias != nil {
					docs := index.add(file, node.Alias.Name, node.Alias.Tk.Pos.Line, node.Alias, byLine)
					index.addHover(file.PackageName, node.Alias.Name, joinHover(code("alias "+node.Alias.Name+" = "+formatType(node.Alias.Target)), docs))
				}
			}
		}
		index.indexValueDeclarations(file.PackageName, file.GlNode)
	}
	return index
}

// indexFunctionValueUsages preserves argument and local-variable references
// inside generic function bodies. Those bodies may be pruned before the
// semantic hover walker runs, but their explicitly declared types are already
// available while the source tree is intact.
func (d *docIndex) indexFunctionValueUsages(module string, function *types.NodeFuncDef) {
	bindings := map[string]*types.NodeType{}
	for _, argument := range function.Class.ArgsNode.Args {
		if argument.Tk.Pos.Line != 0 {
			bindings[argument.Name] = argument.TypeNode
		}
	}
	seen := map[uintptr]bool{}
	var walk func(reflect.Value)
	walk = func(value reflect.Value) {
		if !value.IsValid() {
			return
		}
		if value.Kind() == reflect.Interface {
			if !value.IsNil() {
				walk(value.Elem())
			}
			return
		}
		if value.Kind() == reflect.Pointer {
			if value.IsNil() || seen[value.Pointer()] {
				return
			}
			seen[value.Pointer()] = true
			if variable, ok := value.Interface().(*types.NodeExprVarDef); ok && variable.Type != nil {
				if single, ok := variable.Name.(*types.NodeNameSingle); ok {
					bindings[single.Name] = variable.Type
				}
			}
			if expression, ok := value.Interface().(*types.NodeExprName); ok {
				switch name := expression.Name.(type) {
				case *types.NodeNameSingle:
					if valueType := bindings[name.Name]; valueType != nil {
						d.valueHovers[scopedTokenPositionKey(module, name.Tk)] = code(name.Name + " " + formatType(valueType))
					}
				case *types.NodeNameComposite:
					if len(name.Parts) != 0 && len(name.Tokens) != 0 {
						if valueType := bindings[name.Parts[0]]; valueType != nil {
							d.valueHovers[scopedTokenPositionKey(module, name.Tokens[0])] = code(name.Parts[0] + " " + formatType(valueType))
						}
					}
				}
			}
			walk(value.Elem())
			return
		}
		if value.Kind() == reflect.Struct {
			valueType := value.Type()
			for i := 0; i < value.NumField(); i++ {
				field := valueType.Field(i)
				if field.PkgPath == "" && !skipField(field.Name) {
					walk(value.Field(i))
				}
			}
			return
		}
		switch value.Kind() {
		case reflect.Slice, reflect.Array:
			for i := 0; i < value.Len(); i++ {
				walk(value.Index(i))
			}
		case reflect.Map:
			iterator := value.MapRange()
			for iterator.Next() {
				walk(iterator.Value())
			}
		}
	}
	walk(reflect.ValueOf(&function.Body))
}

func tokenPositionKey(token types.Token) string {
	return fmt.Sprintf("%d:%d", token.Pos.Line, token.Pos.Col)
}

func scopedTokenPositionKey(module string, token types.Token) string {
	return module + "\x00" + tokenPositionKey(token)
}

// indexValueDeclarations runs before monomorphization, preserving arguments,
// struct fields, and explicitly typed variables from generic templates that
// will later be removed from the semantic AST.
func (d *docIndex) indexValueDeclarations(module string, root any) {
	seen := map[uintptr]bool{}
	var walk func(reflect.Value)
	walk = func(value reflect.Value) {
		if !value.IsValid() {
			return
		}
		if value.Kind() == reflect.Interface {
			if !value.IsNil() {
				walk(value.Elem())
			}
			return
		}
		if value.Kind() == reflect.Pointer {
			if value.IsNil() || seen[value.Pointer()] {
				return
			}
			seen[value.Pointer()] = true
			if variable, ok := value.Interface().(*types.NodeExprVarDef); ok && variable.Type != nil {
				if single, ok := variable.Name.(*types.NodeNameSingle); ok && single.Tk.Pos.Line != 0 && sourceName(single.Name) == single.Tk.Repr {
					d.valueHovers[scopedTokenPositionKey(module, single.Tk)] = code(formatVariable(variable))
				}
			}
			walk(value.Elem())
			return
		}
		if value.Kind() == reflect.Struct {
			if value.CanInterface() {
				if argument, ok := value.Interface().(types.NodeArg); ok && argument.Tk.Pos.Line != 0 {
					d.valueHovers[scopedTokenPositionKey(module, argument.Tk)] = code(argument.Name + " " + formatType(argument.TypeNode))
				}
			}
			valueType := value.Type()
			for i := 0; i < value.NumField(); i++ {
				field := valueType.Field(i)
				if field.PkgPath == "" && !skipField(field.Name) {
					walk(value.Field(i))
				}
			}
			return
		}
		switch value.Kind() {
		case reflect.Slice, reflect.Array:
			for i := 0; i < value.Len(); i++ {
				walk(value.Index(i))
			}
		case reflect.Map:
			iterator := value.MapRange()
			for iterator.Next() {
				walk(iterator.Value())
			}
		}
	}
	walk(reflect.ValueOf(root))
}

func (d *docIndex) addHover(module, name, value string) {
	if value == "" {
		return
	}
	d.hoverSymbols[module+"\x00"+name] = value
	simple := name
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		simple = name[dot+1:]
		key := module + "\x00" + simple
		if d.hoverSymbols[key] == "" {
			d.hoverSymbols[key] = value
		}
	}
	if previous, exists := d.hoverByName[simple]; !exists || previous == value {
		d.hoverByName[simple] = value
	} else {
		// An unqualified fallback is safe only while the source name identifies
		// one declaration across the loaded module graph.
		d.hoverByName[simple] = ""
	}
}

func (d *docIndex) add(file *types.FileCtx, name string, line uint32, node any, byLine map[uint32]documentation) string {
	doc, ok := byLine[line]
	if !ok {
		return ""
	}
	text := doc.markdown()
	if text == "" {
		return ""
	}
	d.byNode[node] = text
	d.symbols[file.PackageName+"\x00"+name] = text
	return text
}

func parseDocumentation(source string) (map[uint32]documentation, documentation) {
	lines := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	result := map[uint32]documentation{}
	var module documentation
	moduleLine := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "mod ") && strings.TrimLeft(line, " \t") == line {
			moduleLine = i
			break
		}
	}
	for i := 0; i < len(lines); {
		// Indented comments belong to function bodies and are never docs.
		if !strings.HasPrefix(lines[i], "#") {
			i++
			continue
		}
		start := i
		comments := []string{}
		for i < len(lines) && strings.HasPrefix(lines[i], "#") {
			text := strings.TrimPrefix(lines[i], "#")
			comments = append(comments, strings.TrimPrefix(text, " "))
			i++
		}
		doc := parseDocBlock(comments)
		if start == moduleLine+1 {
			module = doc
			continue
		}
		if i < len(lines) && strings.TrimSpace(lines[i]) != "" && strings.TrimLeft(lines[i], " \t") == lines[i] {
			// Source and token lines are one-based.
			result[uint32(i+1)] = doc
		}
	}
	return result, module
}

func parseDocBlock(lines []string) documentation {
	doc := documentation{}
	// A tag owns every following comment line until the next tag. Folding the
	// block first keeps the individual tag parsers simple while preserving
	// intentional line breaks (especially for examples and warnings).
	folded := make([]string, 0, len(lines))
	for _, line := range lines {
		if len(folded) != 0 && strings.HasPrefix(folded[len(folded)-1], "@") && !strings.HasPrefix(line, "@") {
			folded[len(folded)-1] += "\n" + line
			continue
		}
		folded = append(folded, line)
	}
	lines = folded
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if rest, ok := strings.CutPrefix(line, "@param "); ok {
			name, text, _ := strings.Cut(strings.TrimSpace(rest), " ")
			doc.params = append(doc.params, docParam{name: name, text: strings.TrimSpace(text)})
			continue
		}
		if rest, ok := strings.CutPrefix(line, "@returns "); ok {
			doc.returns = strings.TrimSpace(rest)
			continue
		}
		if rest, ok := strings.CutPrefix(line, "@return "); ok {
			doc.returns = strings.TrimSpace(rest)
			continue
		}
		if rest, ok := strings.CutPrefix(line, "@warning "); ok {
			doc.warnings = append(doc.warnings, strings.TrimSpace(rest))
			continue
		}
		if rest, ok := strings.CutPrefix(line, "@note "); ok {
			doc.notes = append(doc.notes, strings.TrimSpace(rest))
			continue
		}
		if rest, ok := strings.CutPrefix(line, "@complexity "); ok {
			doc.complexity = append(doc.complexity, strings.TrimSpace(rest))
			continue
		}
		if rest, ok := strings.CutPrefix(line, "@throws "); ok {
			doc.throws = append(doc.throws, strings.TrimSpace(rest))
			continue
		}
		if rest, ok := strings.CutPrefix(line, "@ownership "); ok {
			doc.ownership = append(doc.ownership, strings.TrimSpace(rest))
			continue
		}
		if rest, ok := strings.CutPrefix(line, "@safety "); ok {
			doc.safety = append(doc.safety, strings.TrimSpace(rest))
			continue
		}
		if rest, ok := strings.CutPrefix(line, "@mustcall "); ok {
			doc.mustCall = append(doc.mustCall, strings.TrimSpace(rest))
			continue
		}
		if rest, ok := strings.CutPrefix(line, "@platform "); ok {
			doc.platforms = append(doc.platforms, strings.TrimSpace(rest))
			continue
		}
		if rest, ok := strings.CutPrefix(line, "@deprecated "); ok {
			doc.deprecated = append(doc.deprecated, strings.TrimSpace(rest))
			continue
		}
		if rest, ok := strings.CutPrefix(line, "@see "); ok {
			doc.see = append(doc.see, strings.TrimSpace(rest))
			continue
		}
		if line == "@example" || strings.HasPrefix(line, "@example ") || strings.HasPrefix(line, "@example\n") {
			example := strings.TrimPrefix(line, "@example")
			exampleLines := strings.Split(strings.Trim(example, "\n"), "\n")
			for i := range exampleLines {
				exampleLines[i] = strings.TrimPrefix(exampleLines[i], "  ")
			}
			doc.examples = append(doc.examples, strings.TrimSpace(strings.Join(exampleLines, "\n")))
			continue
		}
		doc.description = append(doc.description, line)
	}
	return doc
}

func (d documentation) markdown() string {
	parts := []string{}
	if text := strings.TrimSpace(strings.Join(d.description, "\n")); text != "" {
		parts = append(parts, text)
	}
	for _, text := range d.deprecated {
		parts = append(parts, blockquote("Deprecated", text))
	}
	for _, text := range d.warnings {
		parts = append(parts, blockquote("⚠ Warning", text))
	}
	for _, text := range d.safety {
		parts = append(parts, blockquote("Safety", text))
	}
	for _, text := range d.notes {
		parts = append(parts, blockquote("Note", text))
	}
	if len(d.params) != 0 {
		lines := []string{"**Parameters**"}
		for _, param := range d.params {
			line := fmt.Sprintf("- `%s`", param.name)
			if param.text != "" {
				line += " — " + param.text
			}
			lines = append(lines, line)
		}
		parts = append(parts, strings.Join(lines, "\n"))
	}
	if d.returns != "" {
		parts = append(parts, "**Returns:** "+d.returns)
	}
	if len(d.throws) != 0 {
		parts = append(parts, markdownList("Throws", d.throws, false))
	}
	if len(d.ownership) != 0 {
		parts = append(parts, markdownList("Ownership", d.ownership, false))
	}
	if len(d.mustCall) != 0 {
		parts = append(parts, markdownList("Must call", d.mustCall, true))
	}
	if len(d.complexity) != 0 {
		parts = append(parts, markdownList("Complexity", d.complexity, false))
	}
	if len(d.platforms) != 0 {
		parts = append(parts, markdownList("Platforms", d.platforms, false))
	}
	for _, example := range d.examples {
		parts = append(parts, "**Example**\n\n```magma\n"+example+"\n```")
	}
	if len(d.see) != 0 {
		quoted := make([]string, 0, len(d.see))
		for _, symbol := range d.see {
			quoted = append(quoted, "`"+symbol+"`")
		}
		parts = append(parts, "**See also:** "+strings.Join(quoted, ", "))
	}
	return strings.Join(parts, "\n\n")
}

func blockquote(title, text string) string {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return "> **" + title + ":**"
	}
	lines[0] = "> **" + title + ":** " + lines[0]
	for i := 1; i < len(lines); i++ {
		lines[i] = "> " + lines[i]
	}
	return strings.Join(lines, "\n")
}

func markdownList(title string, values []string, codeValues bool) string {
	if len(values) == 1 {
		value := values[0]
		if codeValues {
			value = "`" + value + "`"
		}
		return "**" + title + ":** " + value
	}
	lines := []string{"**" + title + "**"}
	for _, value := range values {
		if codeValues {
			value = "`" + value + "`"
		}
		lines = append(lines, "- "+value)
	}
	return strings.Join(lines, "\n")
}

func nameLine(name types.NodeName) uint32 {
	switch node := name.(type) {
	case *types.NodeNameSingle:
		return node.Tk.Pos.Line
	case *types.NodeNameComposite:
		if len(node.Tokens) != 0 {
			// For member declarations the owner token may be inherited from a
			// generic type node. The final token is the declared member itself and
			// therefore identifies the line immediately following its docs.
			return node.Tokens[len(node.Tokens)-1].Pos.Line
		}
	}
	return 0
}

func (a *analysis) withDocs(base string, node any) string {
	if a == nil || a.docs == nil {
		return base
	}
	docs := a.docs.byNode[node]
	if docs == "" {
		switch n := node.(type) {
		case *types.NodeStructDef:
			docs = a.docs.symbols[a.file.PackageName+"\x00"+documentationSymbolName(flattenName(n.Class.NameNode))]
		case *types.StructDef:
			docs = a.docs.symbols[n.Module+"\x00"+documentationSymbolName(n.Name)]
		}
	}
	return joinHover(base, docs)
}

// documentationSymbolName maps a specialized display/internal name back to
// the declaration name used when the pre-monomorphization documentation index
// was built.
func documentationSymbolName(name string) string {
	name = sourceName(name)
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		name = name[dot+1:]
	}
	if generic := strings.Index(name, "["); generic >= 0 {
		name = name[:generic]
	}
	return name
}

func joinHover(signature, docs string) string {
	if signature == "" {
		return docs
	}
	if docs == "" {
		return signature
	}
	return signature + "\n\n" + docs
}

func (a *analysis) importedModuleAt(name types.NodeName, pos position) string {
	composite, ok := name.(*types.NodeNameComposite)
	if !ok || len(composite.Tokens) == 0 || !tokenAt(composite.Tokens[0], pos) || len(composite.Parts) == 0 {
		return ""
	}
	return a.importedPackage(composite.Parts[0])
}

func (a *analysis) importedPackage(alias string) string {
	if a == nil || a.file == nil {
		return ""
	}
	// FileCtx.ImportAlias is the parser's alias -> absolute path table. The
	// global node owns the semantic alias -> unique package identifier table
	// used by resolved types and by the documentation index.
	if a.file.GlNode != nil {
		if module := a.file.GlNode.ImportAlias[alias]; module != "" {
			return module
		}
	}
	return a.file.ImportAlias[alias]
}

func (a *analysis) importedPackages() map[string]string {
	if a != nil && a.file != nil && a.file.GlNode != nil && a.file.GlNode.ImportAlias != nil {
		return a.file.GlNode.ImportAlias
	}
	if a != nil && a.file != nil {
		return a.file.ImportAlias
	}
	return nil
}

func (a *analysis) hoverType(node *types.NodeTypeNamed) string {
	base := code(formatType(&types.NodeType{KindNode: node}))
	if a.docs == nil {
		return base
	}
	// Linking may replace the source alias with the imported package's unique
	// internal identifier. Keep that identifier for documentation lookup while
	// formatType independently renders the source-level spelling.
	internalName := flattenInternalName(node.NameNode)
	parts := strings.Split(internalName, ".")
	name := documentationSymbolName(parts[len(parts)-1])
	module := a.file.PackageName
	if len(parts) > 1 {
		qualifier := parts[0]
		if imported := a.importedPackage(qualifier); imported != "" {
			module = imported
		} else {
			for _, imported := range a.importedPackages() {
				if imported == qualifier || sourceName(imported+".symbol") == sourceName(qualifier+".symbol") {
					module = imported
					break
				}
			}
		}
	}
	return joinHover(base, a.docs.symbols[module+"\x00"+name])
}
