package lsp

import (
	"Magma/src/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

type completionItem struct {
	Label         string              `json:"label"`
	Kind          int                 `json:"kind,omitempty"`
	Detail        string              `json:"detail,omitempty"`
	FilterText    string              `json:"filterText,omitempty"`
	InsertText    string              `json:"insertText,omitempty"`
	SortText      string              `json:"sortText,omitempty"`
	Documentation map[string]any      `json:"documentation,omitempty"`
	TextEdit      *completionTextEdit `json:"textEdit,omitempty"`
}

type completionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []completionItem `json:"items"`
}

type completionTextEdit struct {
	Range   rangePosition `json:"range"`
	NewText string        `json:"newText"`
}

type completionContext struct {
	receiver   string
	prefix     string
	lineOffset int
	startByte  int
	dotByte    int
	endByte    int
}

type selectorPart struct {
	name string
	call bool
}

type expressionCompletionContext struct {
	prefix     string
	lineOffset int
	lineEnd    int
}

func complete(uri, source string, pos position, stdRoot string) []completionItem {
	if context, ok := usePathCompletionAt(source, pos); ok {
		return usePathCompletions(uri, context, stdRoot)
	}
	context, ok := completionAt(source, pos)
	if !ok {
		expression, expressionOK := expressionCompletionAt(source, pos)
		if !expressionOK {
			return []completionItem{}
		}
		analysisSource := sanitizeOtherSelectors(source, int(pos.Line))
		clean := analysisSource[:expression.lineOffset] + analysisSource[expression.lineEnd:]
		result := analyze(uri, clean, stdRoot)
		if result == nil || result.file == nil || result.docs == nil {
			return []completionItem{}
		}
		return result.expressionCompletions(expression.prefix, pos.Line+1)
	}
	analysisSource := sanitizeOtherSelectors(source, int(pos.Line))
	// A selector without a member is intentionally invalid Magma. Removing the
	// dot makes the preceding program analyzable, allowing normal inference to
	// determine the receiver type.
	cleanStart := context.dotByte
	cleanEnd := context.endByte
	replacement := ""
	if strings.TrimSpace(analysisSource[context.lineOffset:context.startByte]) == "" {
		cleanStart = context.lineOffset
		for cleanEnd < len(analysisSource) && analysisSource[cleanEnd] != '\r' && analysisSource[cleanEnd] != '\n' {
			cleanEnd++
		}
		if cleanEnd < len(analysisSource) && analysisSource[cleanEnd] == '\r' {
			cleanEnd++
		}
		if cleanEnd < len(analysisSource) && analysisSource[cleanEnd] == '\n' {
			cleanEnd++
		}
	} else if sourceImportsAlias(analysisSource, context.receiver) {
		// A module alias is not itself a value, so merely removing the dot still
		// fails semantic analysis. Replace only the unfinished selector with a
		// harmless expression; deleting its entire line can orphan nested block
		// bodies when completion occurs in an if/while header.
		cleanStart = context.startByte
		replacement = "0"
	}
	clean := analysisSource[:cleanStart] + replacement + analysisSource[cleanEnd:]
	result := analyze(uri, clean, stdRoot)
	if result == nil || result.file == nil || result.docs == nil {
		return []completionItem{}
	}
	receiverParts, ok := selectorParts(context.receiver)
	if !ok {
		return []completionItem{}
	}
	if len(receiverParts) == 1 && !receiverParts[0].call {
		if module := result.importedPackage(context.receiver); module != "" {
			return result.docs.moduleCompletions(module, context.prefix)
		}
	}
	var receiverType *types.NodeType
	partIndex := 1
	module := result.importedPackage(receiverParts[0].name)
	owner := ""
	if module == "" {
		if receiverParts[0].call {
			receiverType = result.docs.functionReturns[result.file.PackageName+"\x00"+receiverParts[0].name]
		} else {
			receiverType = result.docs.completionTypeAt(result.file.PackageName, receiverParts[0].name, pos.Line+1)
			if receiverType == nil {
				receiverType = findValueType(result.file.GlNode, receiverParts[0].name)
			}
		}
		module, owner = completionType(result, receiverType)
	}
	for ; partIndex < len(receiverParts); partIndex++ {
		part := receiverParts[partIndex]
		if owner == "" && !part.call {
			if target := result.docs.publicModuleAlias(module, part.name); target != "" {
				module = target
				continue
			}
		}
		if part.call {
			key := module + "\x00" + part.name
			if owner != "" {
				key = module + "\x00" + owner + "." + part.name
			}
			receiverType = result.docs.functionReturns[key]
		} else {
			if owner == "" {
				return []completionItem{}
			}
			receiverType = result.docs.memberTypes[module+"\x00"+owner+"."+part.name]
		}
		module, owner = completionType(result, receiverType)
	}
	if owner == "" {
		return result.docs.moduleCompletions(module, context.prefix)
	}
	return result.docs.memberCompletions(module, owner, context.prefix)
}

type usePathCompletionContext struct {
	specifier string
	replace   rangePosition
}

func usePathCompletionAt(source string, pos position) (usePathCompletionContext, bool) {
	lines := strings.SplitAfter(source, "\n")
	if int(pos.Line) >= len(lines) {
		return usePathCompletionContext{}, false
	}
	line := strings.TrimSuffix(strings.TrimSuffix(lines[pos.Line], "\n"), "\r")
	runes := []rune(line)
	if int(pos.Character) > len(runes) {
		return usePathCompletionContext{}, false
	}
	before := string(runes[:pos.Character])
	trimmed := strings.TrimLeft(before, " \t")
	if strings.HasPrefix(trimmed, "pub ") {
		trimmed = strings.TrimLeft(strings.TrimPrefix(trimmed, "pub "), " \t")
	}
	if !strings.HasPrefix(trimmed, "use") {
		return usePathCompletionContext{}, false
	}
	afterUse := strings.TrimPrefix(trimmed, "use")
	if afterUse == "" || (afterUse[0] != ' ' && afterUse[0] != '\t') {
		return usePathCompletionContext{}, false
	}
	afterUse = strings.TrimLeft(afterUse, " \t")
	if !strings.HasPrefix(afterUse, "\"") {
		return usePathCompletionContext{}, false
	}
	specifier := strings.TrimPrefix(afterUse, "\"")
	if strings.Contains(specifier, "\"") {
		return usePathCompletionContext{}, false
	}
	separator := strings.LastIndexAny(specifier, "/\\")
	if strings.HasPrefix(specifier, "std:") && separator < len("std:")-1 {
		separator = len("std:") - 1
	}
	prefixRunes := []rune(specifier[separator+1:])
	start := pos.Character - uint32(len(prefixRunes))
	return usePathCompletionContext{
		specifier: strings.ReplaceAll(specifier, "\\", "/"),
		replace:   rangePosition{Start: position{Line: pos.Line, Character: start}, End: pos},
	}, true
}

func usePathCompletions(uri string, context usePathCompletionContext, stdRoot string) []completionItem {
	documentPath, err := uriPath(uri)
	if err != nil {
		return []completionItem{}
	}
	root := filepath.Dir(documentPath)
	pathPart := context.specifier
	stdProtocol := false
	if strings.HasPrefix(pathPart, "std:") {
		stdProtocol = true
		root = stdRoot
		pathPart = strings.TrimPrefix(pathPart, "std:")
	} else if strings.Contains(pathPart, ":") || filepath.IsAbs(filepath.FromSlash(pathPart)) {
		return []completionItem{}
	}
	directoryPart, prefix := pathPart, ""
	if slash := strings.LastIndex(pathPart, "/"); slash >= 0 {
		directoryPart, prefix = pathPart[:slash], pathPart[slash+1:]
	} else {
		directoryPart, prefix = "", pathPart
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return []completionItem{}
	}
	directory := filepath.Clean(filepath.Join(root, filepath.FromSlash(directoryPart)))
	if stdProtocol {
		relative, relErr := filepath.Rel(root, directory)
		if relErr != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return []completionItem{}
		}
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return []completionItem{}
	}
	items := make([]completionItem, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		kind := 19 // CompletionItemKind.Folder
		if entry.IsDir() {
			name += "/"
		} else {
			if strings.ToLower(filepath.Ext(name)) != ".mg" {
				continue
			}
			name = strings.TrimSuffix(name, filepath.Ext(name))
			kind = 9 // CompletionItemKind.Module
		}
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		items = append(items, completionItem{
			Label: name, Kind: kind,
			Detail:     "import path",
			FilterText: context.specifier[:len(context.specifier)-len(prefix)] + name,
			SortText:   "1:" + name,
			TextEdit:   &completionTextEdit{Range: context.replace, NewText: name},
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Label < items[j].Label })
	if context.specifier == "" {
		items = append([]completionItem{{
			Label:      "std:",
			Kind:       19,
			Detail:     "standard library",
			FilterText: "std:",
			SortText:   "0:std:",
			TextEdit:   &completionTextEdit{Range: context.replace, NewText: "std:"},
		}}, items...)
	}
	return items
}

func sanitizeOtherSelectors(source string, activeLine int) string {
	clean := []byte(source)
	lineStart := 0
	lineNumber := 0
	for lineStart < len(clean) {
		lineEnd := lineStart
		for lineEnd < len(clean) && clean[lineEnd] != '\r' && clean[lineEnd] != '\n' {
			lineEnd++
		}
		lineText := string(clean[lineStart:lineEnd])
		if assign := strings.Index(lineText, ":= try "); assign >= 0 && strings.Contains(lineText[:assign], ",") {
			tryStart := lineStart + assign + len(":= ")
			for i := tryStart; i < tryStart+len("try "); i++ {
				clean[i] = ' '
			}
		}
		if lineNumber != activeLine {
			trimmed := strings.TrimSpace(string(clean[lineStart:lineEnd]))
			if trimmed == "if" || trimmed == "loop" || trimmed == "elif" {
				for i := lineStart; i < lineEnd; i++ {
					clean[i] = ' '
				}
				lineStart = lineEnd
				for lineStart < len(clean) && (clean[lineStart] == '\r' || clean[lineStart] == '\n') {
					lineStart++
				}
				lineNumber++
				continue
			}
			contentEnd := lineEnd
			for contentEnd > lineStart && (clean[contentEnd-1] == ' ' || clean[contentEnd-1] == '\t') {
				contentEnd--
			}
			if contentEnd > lineStart && clean[contentEnd-1] == '.' && (contentEnd-lineStart == 1 || clean[contentEnd-2] != '.') {
				for i := lineStart; i < lineEnd; i++ {
					clean[i] = ' '
				}
			} else {
				for i := lineStart; i+1 < lineEnd; i++ {
					if clean[i] == '.' && clean[i+1] == ')' {
						clean[i] = ' '
					}
				}
			}
		}
		for lineEnd < len(clean) && (clean[lineEnd] == '\r' || clean[lineEnd] == '\n') {
			lineEnd++
		}
		lineStart = lineEnd
		lineNumber++
	}
	return string(clean)
}

func sourceImportsAlias(source, alias string) bool {
	if strings.Contains(alias, ".") {
		return false
	}
	for _, line := range strings.Split(source, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 3 && fields[0] == "use" && fields[len(fields)-1] == alias {
			return true
		}
	}
	return false
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
	depth := 0
	for start > 0 {
		r, size := lastRune(before[:start])
		if r == ')' {
			depth++
		} else if r == '(' {
			if depth == 0 {
				break
			}
			depth--
		} else if depth == 0 && !isIdentRune(r) && r != '.' {
			break
		}
		start -= size
	}
	receiver := before[start:dot]
	if receiver == "" {
		return completionContext{}, false
	}
	if _, ok := selectorParts(receiver); !ok {
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

func selectorParts(value string) ([]selectorPart, bool) {
	parts := strings.Split(value, ".")
	result := make([]selectorPart, 0, len(parts))
	for _, part := range parts {
		call := strings.HasSuffix(part, "()")
		name := part
		if call {
			name = strings.TrimSuffix(part, "()")
		}
		if name == "" || !identifier(name) {
			return nil, false
		}
		result = append(result, selectorPart{name: name, call: call})
	}
	return result, len(result) != 0
}

func isIdentRune(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }

func findValueType(root any, name string) *types.NodeType {
	var found *types.NodeType
	walkAST(root, func(value any) bool {
		variable, ok := value.(*types.NodeExprVarDef)
		if ok && flattenName(variable.Name) == name {
			found = variable.Type
			return false
		}
		return true
	})
	return found
}

func completionType(a *analysis, node *types.NodeType) (string, string) {
	if node == nil {
		return "", ""
	}
	if primitive := completionPrimitiveType(node); primitive != "" {
		return a.docs.primitiveModules[primitive], primitive
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
	items := d.completions(module+"\x00", "", prefix)
	seen := map[string]bool{}
	for _, item := range items {
		seen[item.Label] = true
	}
	for alias := range d.publicModuleAliases[module] {
		if seen[alias] || !strings.HasPrefix(alias, prefix) {
			continue
		}
		items = append(items, completionItem{Label: alias, Kind: 9, Detail: "module " + alias})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Label < items[j].Label })
	return items
}

func (d *docIndex) publicModuleAlias(module, alias string) string {
	if d == nil {
		return ""
	}
	return d.publicModuleAliases[module][alias]
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
		if declaredKind := d.completionKinds[key]; declaredKind != 0 {
			kind = declaredKind
		}
		if forbiddenDotPrefix != "" && !strings.Contains(firstCodeLine(hover), "(") {
			kind = 5
		}
		label := name
		filterText := ""
		insertText := ""
		if d.completionDestructors[key] {
			label = "~" + name
			filterText = name
			insertText = name
		}
		items = append(items, completionItem{Label: label, Kind: kind, Detail: firstCodeLine(hover), FilterText: filterText, InsertText: insertText, Documentation: map[string]any{"kind": "markdown", "value": hover}})
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
