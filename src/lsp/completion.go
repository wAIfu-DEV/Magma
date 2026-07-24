package lsp

import (
	"Magma/src/types"
	"reflect"
	"sort"
	"strings"
	"unicode"
)

type completionItem struct {
	Label         string         `json:"label"`
	Kind          int            `json:"kind,omitempty"`
	Detail        string         `json:"detail,omitempty"`
	Documentation map[string]any `json:"documentation,omitempty"`
}

type completionContext struct {
	receiver   string
	prefix     string
	lineOffset int
	startByte  int
	dotByte    int
	endByte    int
}

type expressionCompletionContext struct {
	prefix     string
	lineOffset int
	lineEnd    int
}

func complete(uri, source string, pos position) []completionItem {
	context, ok := completionAt(source, pos)
	if !ok {
		expression, expressionOK := expressionCompletionAt(source, pos)
		if !expressionOK {
			return []completionItem{}
		}
		clean := source[:expression.lineOffset] + source[expression.lineEnd:]
		result := analyze(uri, clean)
		if result == nil || result.file == nil || result.docs == nil {
			return []completionItem{}
		}
		return result.expressionCompletions(expression.prefix, pos.Line+1)
	}
	// A selector without a member is intentionally invalid Magma. Removing the
	// dot makes the preceding program analyzable, allowing normal inference to
	// determine the receiver type.
	cleanStart := context.dotByte
	cleanEnd := context.endByte
	if strings.TrimSpace(source[context.lineOffset:context.startByte]) == "" {
		cleanStart = context.lineOffset
		if cleanEnd < len(source) && source[cleanEnd] == '\r' {
			cleanEnd++
		}
		if cleanEnd < len(source) && source[cleanEnd] == '\n' {
			cleanEnd++
		}
	}
	clean := source[:cleanStart] + source[cleanEnd:]
	result := analyze(uri, clean)
	if result == nil || result.file == nil || result.docs == nil {
		return []completionItem{}
	}
	receiverParts := strings.Split(context.receiver, ".")
	if len(receiverParts) == 1 {
		if module := result.importedPackage(context.receiver); module != "" {
			return result.docs.moduleCompletions(module, context.prefix)
		}
	}
	receiverType := result.docs.completionTypeAt(result.file.PackageName, receiverParts[0], pos.Line+1)
	if receiverType == nil {
		receiverType = findValueType(result.file.GlNode, receiverParts[0])
	}
	module, owner := completionType(result, receiverType)
	for _, field := range receiverParts[1:] {
		if owner == "" {
			return []completionItem{}
		}
		receiverType = result.docs.memberTypes[module+"\x00"+owner+"."+field]
		module, owner = completionType(result, receiverType)
	}
	if owner == "" {
		return []completionItem{}
	}
	return result.docs.memberCompletions(module, owner, context.prefix)
}

func expressionCompletionAt(source string, pos position) (expressionCompletionContext, bool) {
	lines := strings.SplitAfter(source, "\n")
	if int(pos.Line) >= len(lines) {
		return expressionCompletionContext{}, false
	}
	lineOffset := 0
	for i := 0; i < int(pos.Line); i++ {
		lineOffset += len(lines[i])
	}
	line := strings.TrimSuffix(strings.TrimSuffix(lines[pos.Line], "\n"), "\r")
	runes := []rune(line)
	if int(pos.Character) > len(runes) {
		return expressionCompletionContext{}, false
	}
	before := string(runes[:pos.Character])
	start := len(before)
	for start > 0 {
		r, size := lastRune(before[:start])
		if !isIdentRune(r) {
			break
		}
		start -= size
	}
	prefix := before[start:]
	if !identifier(prefix) || (start > 0 && before[start-1] == '.') {
		return expressionCompletionContext{}, false
	}
	// Struct fields and function statements are both indented in Magma. Walk
	// back to the enclosing top-level declaration and only enable expression
	// completion when that declaration opened a function body with `:`.
	if !insideFunctionBody(lines, int(pos.Line)) {
		return expressionCompletionContext{}, false
	}
	lineEnd := lineOffset + len(lines[pos.Line])
	return expressionCompletionContext{prefix: prefix, lineOffset: lineOffset, lineEnd: lineEnd}, true
}

func insideFunctionBody(lines []string, line int) bool {
	if line < 0 || line >= len(lines) {
		return false
	}
	for i := line - 1; i >= 0; i-- {
		text := strings.TrimRight(lines[i], "\r\n")
		trimmed := strings.TrimSpace(text)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if len(text) != len(strings.TrimLeft(text, " \t")) {
			continue
		}
		return strings.HasSuffix(trimmed, ":")
	}
	return false
}

func (a *analysis) expressionCompletions(prefix string, line uint32) []completionItem {
	items := map[string]completionItem{}
	for name, item := range a.docs.expressionSymbols[a.file.PackageName] {
		if strings.HasPrefix(name, prefix) {
			items[name] = item
		}
	}
	for _, binding := range a.docs.expressionBindingsAt(a.file.PackageName, line) {
		if !strings.HasPrefix(binding.name, prefix) {
			continue
		}
		detail := binding.name + " " + formatType(binding.valueType)
		items[binding.name] = completionItem{Label: binding.name, Kind: 6, Detail: detail, Documentation: markdownContent(code(detail))}
	}
	for alias, module := range a.importedPackages() {
		if strings.HasPrefix(alias, "__") || !strings.HasPrefix(alias, prefix) {
			continue
		}
		items[alias] = completionItem{Label: alias, Kind: 9, Detail: "module " + alias, Documentation: markdownContent(a.docs.modules[module])}
	}
	result := make([]completionItem, 0, len(items))
	for _, item := range items {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Label < result[j].Label })
	return result
}

func markdownContent(value string) map[string]any {
	if value == "" {
		return nil
	}
	return map[string]any{"kind": "markdown", "value": value}
}

func completionAt(source string, pos position) (completionContext, bool) {
	lines := strings.SplitAfter(source, "\n")
	if int(pos.Line) >= len(lines) {
		return completionContext{}, false
	}
	lineStart := 0
	for i := 0; i < int(pos.Line); i++ {
		lineStart += len(lines[i])
	}
	line := strings.TrimSuffix(strings.TrimSuffix(lines[pos.Line], "\n"), "\r")
	runes := []rune(line)
	if int(pos.Character) > len(runes) {
		return completionContext{}, false
	}
	before := string(runes[:pos.Character])
	dot := strings.LastIndexByte(before, '.')
	if dot < 0 {
		return completionContext{}, false
	}
	prefix := before[dot+1:]
	if !identifier(prefix) {
		return completionContext{}, false
	}
	start := dot
	for start > 0 {
		r, size := lastRune(before[:start])
		if !isIdentRune(r) && r != '.' {
			break
		}
		start -= size
	}
	receiver := before[start:dot]
	if receiver == "" || !identifierPath(receiver) {
		return completionContext{}, false
	}
	return completionContext{receiver: receiver, prefix: prefix, startByte: lineStart + len(before[:start]), dotByte: lineStart + len(before[:dot]), endByte: lineStart + len(before), lineOffset: lineStart}, true
}

func lastRune(value string) (rune, int) {
	runes := []rune(value)
	r := runes[len(runes)-1]
	return r, len(string(r))
}

func identifier(value string) bool {
	for _, r := range value {
		if !isIdentRune(r) {
			return false
		}
	}
	return true
}

func identifierPath(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) == 0 {
		return false
	}
	for _, part := range parts {
		if part == "" || !identifier(part) {
			return false
		}
	}
	return true
}

func isIdentRune(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }

func findValueType(root any, name string) *types.NodeType {
	var found *types.NodeType
	seen := map[uintptr]bool{}
	var walk func(reflect.Value)
	walk = func(value reflect.Value) {
		if found != nil || !value.IsValid() {
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
			if variable, ok := value.Interface().(*types.NodeExprVarDef); ok && flattenName(variable.Name) == name {
				found = variable.Type
				return
			}
			walk(value.Elem())
			return
		}
		if value.Kind() == reflect.Struct {
			t := value.Type()
			for i := 0; i < value.NumField(); i++ {
				if t.Field(i).PkgPath == "" && !skipField(t.Field(i).Name) {
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
			iter := value.MapRange()
			for iter.Next() {
				walk(iter.Value())
			}
		}
	}
	walk(reflect.ValueOf(root))
	return found
}

func completionType(a *analysis, node *types.NodeType) (string, string) {
	if node == nil {
		return "", ""
	}
	switch kind := node.KindNode.(type) {
	case *types.NodeTypePointer:
		return completionType(a, &types.NodeType{KindNode: kind.Kind})
	case *types.NodeTypeRfc:
		return completionType(a, &types.NodeType{KindNode: kind.Kind})
	case *types.NodeTypeAbsolute:
		parts := strings.Split(sourceName(kind.AbsoluteName), ".")
		if len(parts) < 2 {
			return a.file.PackageName, parts[0]
		}
		module := strings.Split(kind.AbsoluteName, ".")[0]
		return module, documentationSymbolName(parts[len(parts)-1])
	case *types.NodeTypeNamed:
		parts := strings.Split(flattenInternalName(kind.NameNode), ".")
		module := a.file.PackageName
		if len(parts) > 1 {
			module = a.importedPackage(parts[0])
			if module == "" {
				module = parts[0]
			}
		}
		return module, documentationSymbolName(parts[len(parts)-1])
	}
	return "", ""
}

func (d *docIndex) moduleCompletions(module, prefix string) []completionItem {
	return d.completions(module+"\x00", "", prefix)
}

func (d *docIndex) memberCompletions(module, owner, prefix string) []completionItem {
	return d.completions(module+"\x00"+owner+".", owner+".", prefix)
}

func (d *docIndex) completions(keyPrefix, forbiddenDotPrefix, typedPrefix string) []completionItem {
	items := []completionItem{}
	seen := map[string]bool{}
	for key, hover := range d.hoverSymbols {
		if !strings.HasPrefix(key, keyPrefix) {
			continue
		}
		if d.completionVisible != nil && !d.completionVisible[key] {
			continue
		}
		name := strings.TrimPrefix(key, keyPrefix)
		if name == "" || strings.Contains(name, ".") || (forbiddenDotPrefix != "" && strings.HasPrefix(name, forbiddenDotPrefix)) || !strings.HasPrefix(name, typedPrefix) || seen[name] {
			continue
		}
		seen[name] = true
		kind := 3
		if forbiddenDotPrefix != "" && !strings.Contains(firstCodeLine(hover), "(") {
			kind = 5
		}
		items = append(items, completionItem{Label: name, Kind: kind, Detail: firstCodeLine(hover), Documentation: map[string]any{"kind": "markdown", "value": hover}})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Label < items[j].Label })
	return items
}

func firstCodeLine(markdown string) string {
	lines := strings.Split(markdown, "\n")
	if len(lines) > 1 && strings.HasPrefix(lines[0], "```") {
		return lines[1]
	}
	return ""
}
