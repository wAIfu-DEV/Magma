package lsp

import (
	"Magma/src/magma_types"
	"Magma/src/types"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

type completionItem struct {
	Label               string              `json:"label"`
	Kind                int                 `json:"kind,omitempty"`
	Detail              string              `json:"detail,omitempty"`
	FilterText          string              `json:"filterText,omitempty"`
	InsertText          string              `json:"insertText,omitempty"`
	InsertTextFormat    int                 `json:"insertTextFormat,omitempty"`
	SortText            string              `json:"sortText,omitempty"`
	Documentation       map[string]any      `json:"documentation,omitempty"`
	TextEdit            *completionTextEdit `json:"textEdit,omitempty"`
	AdditionalTextEdits []textEdit          `json:"additionalTextEdits,omitempty"`
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

type structFieldCompletionContext struct {
	typeName string
	prefix   string
	used     map[string]bool
}

type typeCompletionContext struct {
	prefix, moduleAlias string
	startByte, endByte  int
}

func complete(uri, source string, pos position, stdRoot string) []completionItem {
	if context, ok := usePathCompletionAt(source, pos); ok {
		return usePathCompletions(uri, context, stdRoot)
	}
	if prefix, ok := topLevelKeywordCompletionAt(source, pos); ok {
		return topLevelKeywordCompletions(prefix)
	}
	if context, ok := typeCompletionAt(source, pos); ok {
		// Substitute a valid type so declarations whose type is still being typed
		// do not prevent the normal module/import index from being built.
		clean := source[:context.startByte] + "u64" + source[context.endByte:]
		result := analyze(uri, clean, stdRoot)
		if result == nil || result.file == nil || result.docs == nil {
			return []completionItem{}
		}
		if context.moduleAlias != "" {
			module := result.importedPackage(context.moduleAlias)
			if module == "" {
				return []completionItem{}
			}
			return result.docs.typeCompletions(module, context.prefix, true, false)
		}
		items := result.docs.typeCompletions(result.file.PackageName, context.prefix, false, true)
		for alias, module := range result.importedPackages() {
			if strings.HasPrefix(alias, "__") || !strings.HasPrefix(alias, context.prefix) {
				continue
			}
			items = append(items, completionItem{
				Label:         alias,
				Kind:          9, // CompletionItemKind.Module
				Detail:        "module " + alias,
				Documentation: markdownContent(result.docs.modules[module]),
			})
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Label < items[j].Label })
		return items
	}
	if context, ok := structFieldCompletionAt(source, pos); ok {
		analysisSource := sourceWithoutEnclosingFunction(source, pos)
		result := analyze(uri, analysisSource, stdRoot)
		if result == nil || result.file == nil || result.docs == nil {
			return []completionItem{}
		}
		parts := strings.Split(context.typeName, ".")
		module := result.file.PackageName
		owner := parts[len(parts)-1]
		if len(parts) == 2 {
			module = result.importedPackage(parts[0])
		}
		if module == "" {
			return []completionItem{}
		}
		if result.docs.completionKinds[module+"\x00"+owner] == 22 {
			return result.docs.structFieldCompletions(module, owner, context.prefix, context.used)
		}
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
		return result.expressionCompletions(expression.prefix, pos.Line+1, expectedTypeAt(source, pos, result))
	}
	analysisSource := sanitizeOtherSelectors(source, int(pos.Line))
	topLevelReceiver := strings.TrimSpace(analysisSource[context.lineOffset:context.startByte]) == ""
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
	if topLevelReceiver {
		if items := missingProtoMethodCompletions(result, source, stdRoot, context.receiver, context.prefix); len(items) > 0 {
			return items
		}
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
			if result.docs.completionTypeResolutionScore(receiverType) <= 0 {
				if recovered := inferredLocalTypeFromSource(result, analysisSource, receiverParts[0].name, pos.Line+1); recovered != nil {
					receiverType = recovered
				}
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
	// A partially analyzed local can carry a syntactically valid type that does
	// not resolve to a completion owner. Recover its initializer type before
	// falling back to module completions; otherwise completion immediately after
	// the first of two adjacent dots can exit here with an empty result.
	if owner == "" && len(receiverParts) == 1 && !receiverParts[0].call {
		if recovered := inferredLocalTypeFromSource(result, analysisSource, receiverParts[0].name, pos.Line+1); recovered != nil {
			recoveredModule, recoveredOwner := completionType(result, recovered)
			if recoveredOwner != "" {
				module, owner = recoveredModule, recoveredOwner
			}
		}
	}
	if owner == "" {
		return result.docs.moduleCompletions(module, context.prefix)
	}
	items := result.docs.memberCompletions(module, owner, context.prefix)
	if len(items) == 0 && len(receiverParts) == 1 && !receiverParts[0].call {
		if recovered := inferredLocalTypeFromSource(result, analysisSource, receiverParts[0].name, pos.Line+1); recovered != nil {
			recoveredModule, recoveredOwner := completionType(result, recovered)
			if recoveredOwner != "" {
				items = result.docs.memberCompletions(recoveredModule, recoveredOwner, context.prefix)
			}
		}
	}
	return items
}

// missingProtoMethodCompletions turns a top-level `StructName.` selector into
// implementation stubs for requirements that are not already defined. At file
// scope a struct name is not an expression receiver, so these completions are
// deliberately kept separate from ordinary member completion.
func missingProtoMethodCompletions(result *analysis, source, stdRoot, receiver, prefix string) []completionItem {
	if result == nil || result.file == nil || result.file.GlNode == nil || strings.Contains(receiver, ".") {
		return nil
	}
	global := result.file.GlNode
	definition := global.StructDefs[receiver]
	if definition == nil || definition.IsProto {
		return nil
	}
	items := []completionItem{}
	seen := map[string]bool{}
	for _, implementation := range definition.Implements {
		if implementation == nil || implementation.Proto == nil {
			continue
		}
		for _, method := range implementation.Proto.Methods {
			if method == nil || seen[method.Name] || definition.Funcs[method.Name] != nil || !strings.HasPrefix(method.Name, prefix) {
				continue
			}
			seen[method.Name] = true
			formatter := newCompletionTypeFormatter(result, stdRoot)
			args := make([]string, 0, len(method.Args))
			for _, arg := range method.Args {
				args = append(args, arg.Name+" "+formatter.format(arg.TypeNode))
			}
			signature := method.Name + "(" + strings.Join(args, ", ") + ") " + formatter.format(method.Ret)
			insert := signature + ":\n.."
			if method.ContextABI == types.ContextABIContextless {
				insert = "noctx " + insert
			}
			item := completionItem{
				Label:         method.Name,
				Kind:          2, // CompletionItemKind.Method
				Detail:        signature,
				FilterText:    method.Name,
				InsertText:    insert,
				Documentation: markdownContent("Implements `" + implementation.Proto.Name + "." + method.Name + "`."),
			}
			if edit, ok := formatter.importEdit(source); ok {
				item.AdditionalTextEdits = []textEdit{edit}
			}
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Label < items[j].Label })
	return items
}

type completionTypeFormatter struct {
	result  *analysis
	stdRoot string
	aliases map[string]string
	pending map[string]string
}

func newCompletionTypeFormatter(result *analysis, stdRoot string) *completionTypeFormatter {
	aliases := map[string]string{}
	for alias, module := range result.importedPackages() {
		aliases[module] = alias
	}
	return &completionTypeFormatter{result: result, stdRoot: stdRoot, aliases: aliases, pending: map[string]string{}}
}

func (f *completionTypeFormatter) format(node *types.NodeType) string {
	if node == nil {
		return "?"
	}
	copy := *node
	copy.Throws, copy.Owned = false, false
	var out string
	switch kind := node.KindNode.(type) {
	case *types.NodeTypeAbsolute:
		module, name := splitAbsoluteType(kind.AbsoluteName)
		out = name
		if module != "" && module != f.result.file.PackageName {
			if alias := f.aliasFor(module); alias != "" {
				out = alias + "." + name
			}
		}
	case *types.NodeTypeNamed:
		out = flattenName(kind.NameNode)
		if len(kind.GenericArgs) > 0 {
			args := make([]string, len(kind.GenericArgs))
			for i, arg := range kind.GenericArgs {
				args[i] = f.format(arg)
			}
			out += "[" + strings.Join(args, ", ") + "]"
		}
	case *types.NodeTypePointer:
		out = f.format(&types.NodeType{KindNode: kind.Kind}) + "*"
	case *types.NodeTypeRfc:
		out = "&" + f.format(&types.NodeType{KindNode: kind.Kind})
	case *types.NodeTypeSlice:
		out = f.format(&types.NodeType{KindNode: kind.ElemKind}) + "[]"
	case *types.NodeTypeFunc:
		args := make([]string, len(kind.Args))
		for i, arg := range kind.Args {
			args[i] = f.format(arg)
		}
		out = "fn(" + strings.Join(args, ", ") + "): " + f.format(kind.RetType)
	default:
		out = formatType(&copy)
	}
	if node.Owned {
		out = "$" + out
	}
	if node.Throws {
		out = "!" + out
	}
	return out
}

func splitAbsoluteType(name string) (string, string) {
	index := strings.LastIndex(name, ".")
	if index < 0 {
		return "", types.SourceName(name)
	}
	return name[:index], types.SourceName(name[index+1:])
}

func (f *completionTypeFormatter) aliasFor(module string) string {
	if alias := f.aliases[module]; alias != "" {
		return alias
	}
	if alias := f.pending[module]; alias != "" {
		return alias
	}
	base := "module"
	if f.result.docs != nil && f.result.docs.moduleNames[module] != "" {
		base = f.result.docs.moduleNames[module]
	}
	used := map[string]bool{}
	for alias := range f.result.importedPackages() {
		used[alias] = true
	}
	for name := range f.result.file.GlNode.StructDefs {
		used[name] = true
	}
	for name := range f.result.file.GlNode.FuncDefs {
		used[name] = true
	}
	for _, alias := range f.pending {
		used[alias] = true
	}
	alias := base
	for suffix := 2; used[alias]; suffix++ {
		alias = base + fmt.Sprint(suffix)
	}
	f.pending[module] = alias
	return alias
}

func (f *completionTypeFormatter) importEdit(source string) (textEdit, bool) {
	if len(f.pending) == 0 || f.result.docs == nil {
		return textEdit{}, false
	}
	modules := make([]string, 0, len(f.pending))
	for module := range f.pending {
		modules = append(modules, module)
	}
	sort.Slice(modules, func(i, j int) bool { return f.pending[modules[i]] < f.pending[modules[j]] })
	lines := make([]string, 0, len(modules))
	for _, module := range modules {
		path := f.result.docs.modulePaths[module]
		specifier, ok := completionImportSpecifier(f.result.file.FilePath, path, f.stdRoot)
		if !ok {
			continue
		}
		lines = append(lines, "use \""+specifier+"\" "+f.pending[module])
	}
	if len(lines) == 0 {
		return textEdit{}, false
	}
	line := completionImportLine(source)
	position := position{Line: uint32(line), Character: 0}
	newline := "\n"
	if strings.Contains(source, "\r\n") {
		newline = "\r\n"
	}
	return textEdit{Range: rangePosition{Start: position, End: position}, NewText: strings.Join(lines, newline) + newline}, true
}

func completionImportSpecifier(currentPath, targetPath, stdRoot string) (string, bool) {
	if targetPath == "" {
		return "", false
	}
	targetPath = filepath.Clean(targetPath)
	if absoluteStd, err := filepath.Abs(stdRoot); err == nil {
		if relative, err := filepath.Rel(absoluteStd, targetPath); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return "std:" + strings.TrimSuffix(filepath.ToSlash(relative), ".mg"), true
		}
	}
	relative, err := filepath.Rel(filepath.Dir(currentPath), targetPath)
	if err != nil {
		return "", false
	}
	relative = strings.TrimSuffix(filepath.ToSlash(relative), ".mg")
	if !strings.HasPrefix(relative, ".") {
		relative = "./" + relative
	}
	return relative, true
}

func completionImportLine(source string) int {
	lines := strings.Split(source, "\n")
	line := 1
	for line < len(lines) {
		trimmed := strings.TrimSpace(lines[line])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			line++
			continue
		}
		break
	}
	lastUse := -1
	for index := line; index < len(lines); index++ {
		trimmed := strings.TrimSpace(lines[index])
		if strings.HasPrefix(trimmed, "use ") || strings.HasPrefix(trimmed, "pub use ") {
			lastUse = index
			continue
		}
		if trimmed != "" {
			break
		}
	}
	if lastUse >= 0 {
		return lastUse + 1
	}
	return line
}

func inferredLocalTypeFromSource(result *analysis, source, name string, line uint32) *types.NodeType {
	lines := strings.Split(source, "\n")
	limit := int(line)
	if limit > len(lines) {
		limit = len(lines)
	}
	needle := name + " :="
	for index := limit - 1; index >= 0; index-- {
		text := strings.TrimSpace(lines[index])
		if !strings.HasPrefix(text, needle) {
			continue
		}
		rhs := strings.TrimSpace(strings.TrimPrefix(text, needle))
		rhs = strings.TrimSpace(strings.TrimPrefix(rhs, "try "))
		parts, ok := selectorParts(rhs)
		if !ok || len(parts) == 0 {
			return nil
		}
		valueType := result.docs.completionTypeAt(result.file.PackageName, parts[0].name, uint32(index+1))
		module, owner := completionType(result, valueType)
		if imported := result.importedPackage(parts[0].name); imported != "" && !parts[0].call {
			module = imported
			owner = ""
		}
		for _, part := range parts[1:] {
			if part.call {
				if owner == "" {
					valueType = result.docs.functionReturns[module+"\x00"+part.name]
				} else {
					valueType = result.docs.completionMemberReturnType(module, owner, part.name, valueType)
				}
			} else {
				if owner == "" {
					if imported := result.docs.publicModuleAlias(module, part.name); imported != "" {
						module = imported
						continue
					}
					return nil
				}
				valueType = result.docs.canonicalCompletionType(module, result.docs.memberTypes[module+"\x00"+owner+"."+part.name])
			}
			if valueType == nil {
				return nil
			}
			module, owner = completionType(result, valueType)
		}
		return valueType
	}
	return nil
}

// typeCompletionAt recognizes the source positions where the grammar expects
// a type after a declared name. This remains intentionally textual: incomplete
// declarations do not have an AST yet.
func typeCompletionAt(source string, pos position) (typeCompletionContext, bool) {
	lines := strings.SplitAfter(source, "\n")
	if int(pos.Line) >= len(lines) {
		return typeCompletionContext{}, false
	}
	lineStart := 0
	for i := 0; i < int(pos.Line); i++ {
		lineStart += len(lines[i])
	}
	line := strings.TrimSuffix(strings.TrimSuffix(lines[pos.Line], "\n"), "\r")
	runes := []rune(line)
	if int(pos.Character) > len(runes) {
		return typeCompletionContext{}, false
	}
	cursor := lineStart + len(string(runes[:pos.Character]))
	start := cursor
	for start > lineStart && isIdentRune(rune(source[start-1])) {
		start--
	}
	alias := ""
	if start > lineStart && source[start-1] == '.' {
		aliasEnd := start - 1
		aliasStart := aliasEnd
		for aliasStart > lineStart && isIdentRune(rune(source[aliasStart-1])) {
			aliasStart--
		}
		if aliasStart == aliasEnd {
			return typeCompletionContext{}, false
		}
		alias = source[aliasStart:aliasEnd]
		start = aliasStart
	}
	end := cursor
	for end < lineStart+len(line) && (isIdentRune(rune(source[end])) || source[end] == '.') {
		end++
	}
	typed := source[start:cursor]
	prefix := typed
	if alias != "" {
		prefix = strings.TrimPrefix(typed, alias+".")
	}
	if prefix != "" && !identifier(prefix) {
		return typeCompletionContext{}, false
	}
	before := source[lineStart:start]
	if start == cursor && (len(before) == 0 || (before[len(before)-1] != ' ' && before[len(before)-1] != '\t')) {
		return typeCompletionContext{}, false
	}
	trimmed := strings.TrimSpace(before)
	if trimmed == "" {
		return typeCompletionContext{}, false
	}

	// A top-level declaration ending in ')' expects its return type.
	indented := len(before) != len(strings.TrimLeft(before, " \t"))
	expects := !indented && strings.HasSuffix(trimmed, ")")
	// A plain declaration line consists of modifiers followed by the name.
	if !expects && strings.IndexAny(trimmed, "=:.()[],") < 0 {
		fields := strings.Fields(trimmed)
		reserved := map[string]bool{"if": true, "elif": true, "else": true, "while": true, "for": true, "ret": true, "throw": true, "try": true, "defer": true, "onerror": true, "use": true, "mod": true}
		expects = len(fields) > 0 && !reserved[fields[len(fields)-1]]
	}
	// Inside a top-level function/struct declaration, the current comma-delimited
	// argument must contain its name followed by whitespace.
	if !expects {
		open := unmatchedOpenParen(source[:start])
		if open >= 0 {
			declLine := strings.LastIndex(source[:open], "\n") + 1
			if len(source[declLine:]) == len(strings.TrimLeft(source[declLine:], " \t")) {
				segment := source[open+1 : start]
				if comma := strings.LastIndex(segment, ","); comma >= 0 {
					segment = segment[comma+1:]
				}
				fields := strings.Fields(segment)
				expects = len(fields) == 1 && identifier(fields[0]) && len(segment) > len(strings.TrimRight(segment, " \t\r\n"))
			}
		}
	}
	if !expects {
		return typeCompletionContext{}, false
	}
	return typeCompletionContext{prefix: prefix, moduleAlias: alias, startByte: start, endByte: end}, true
}

func sourceWithoutEnclosingFunction(source string, pos position) string {
	lines := strings.SplitAfter(source, "\n")
	if int(pos.Line) >= len(lines) {
		return source
	}
	start := -1
	for i := int(pos.Line) - 1; i >= 0; i-- {
		text := strings.TrimRight(lines[i], "\r\n")
		if strings.TrimSpace(text) == "" || len(text) != len(strings.TrimLeft(text, " \t")) {
			continue
		}
		if strings.HasSuffix(strings.TrimSpace(text), ":") {
			start = i
		}
		break
	}
	if start < 0 {
		return source
	}
	end := len(lines)
	for i := int(pos.Line) + 1; i < len(lines); i++ {
		text := strings.TrimRight(lines[i], "\r\n")
		if len(text) == len(strings.TrimLeft(text, " \t")) && strings.TrimSpace(text) == ".." {
			end = i + 1
			break
		}
	}
	return strings.Join(append(append([]string{}, lines[:start]...), lines[end:]...), "")
}

// structFieldCompletionAt recognizes the unfinished named argument at the
// cursor. It deliberately works on source text because `Type(fi` is not yet a
// valid AST, which is precisely when completion is most useful.
func structFieldCompletionAt(source string, pos position) (structFieldCompletionContext, bool) {
	lines := strings.SplitAfter(source, "\n")
	if int(pos.Line) >= len(lines) {
		return structFieldCompletionContext{}, false
	}
	offset := 0
	for i := 0; i < int(pos.Line); i++ {
		offset += len(lines[i])
	}
	line := strings.TrimSuffix(strings.TrimSuffix(lines[pos.Line], "\n"), "\r")
	runes := []rune(line)
	if int(pos.Character) > len(runes) {
		return structFieldCompletionContext{}, false
	}
	cursor := offset + len(string(runes[:pos.Character]))
	open := unmatchedOpenParen(source[:cursor])
	if open < 0 {
		return structFieldCompletionContext{}, false
	}
	typeName := constructorTypeBefore(source, open)
	if typeName == "" {
		return structFieldCompletionContext{}, false
	}
	body := source[open+1 : cursor]
	segmentStart := topLevelSegmentStart(body)
	segment := strings.TrimSpace(body[segmentStart:])
	if strings.Contains(segment, "=") || !identifier(segment) {
		return structFieldCompletionContext{}, false
	}
	used := map[string]bool{}
	for _, segment := range topLevelSegments(body[:segmentStart]) {
		if equal := strings.IndexByte(segment, '='); equal >= 0 {
			name := strings.TrimSpace(segment[:equal])
			if name != "" && identifier(name) {
				used[name] = true
			}
		}
	}
	return structFieldCompletionContext{typeName: typeName, prefix: segment, used: used}, true
}

func unmatchedOpenParen(source string) int {
	stack := []int{}
	inString, escaped, comment := false, false, false
	for i := 0; i < len(source); i++ {
		c := source[i]
		if comment {
			if c == '\n' {
				comment = false
			}
			continue
		}
		if inString {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		if c == '#' {
			comment = true
			continue
		}
		if c == '"' {
			inString = true
			continue
		}
		if c == '(' {
			stack = append(stack, i)
		}
		if c == ')' && len(stack) > 0 {
			stack = stack[:len(stack)-1]
		}
	}
	if len(stack) == 0 {
		return -1
	}
	return stack[len(stack)-1]
}

func constructorTypeBefore(source string, open int) string {
	i := open
	for i > 0 && (source[i-1] == ' ' || source[i-1] == '\t') {
		i--
	}
	if i > 0 && source[i-1] == ']' {
		depth := 1
		i--
		for i > 0 && depth > 0 {
			i--
			if source[i] == ']' {
				depth++
			} else if source[i] == '[' {
				depth--
			}
		}
		if depth != 0 {
			return ""
		}
	}
	end := i
	for i > 0 && (isIdentRune(rune(source[i-1])) || source[i-1] == '.') {
		i--
	}
	name := source[i:end]
	if !identifierPath(name) {
		return ""
	}
	return name
}

func topLevelSegmentStart(body string) int {
	depth := 0
	start := 0
	for i, r := range body {
		switch r {
		case '(', '[':
			depth++
		case ')', ']':
			if depth > 0 {
				depth--
			}
		case ',', '\n':
			if depth == 0 {
				start = i + len(string(r))
			}
		}
	}
	return start
}

func topLevelSegments(body string) []string {
	segments := []string{}
	start := 0
	depth := 0
	for i, r := range body {
		switch r {
		case '(', '[':
			depth++
		case ')', ']':
			if depth > 0 {
				depth--
			}
		case ',', '\n':
			if depth == 0 {
				segments = append(segments, body[start:i])
				start = i + len(string(r))
			}
		}
	}
	segments = append(segments, body[start:])
	return segments
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

func (a *analysis) expressionCompletions(prefix string, line uint32, expectedTypes ...string) []completionItem {
	expected := ""
	if len(expectedTypes) != 0 {
		expected = expectedTypes[0]
	}
	items := map[string]completionItem{}
	for _, keyword := range []struct{ label, detail, insert string }{
		{"if", "conditional block", "if ${1:condition}:\n    ${0}\n.."},
		{"loop", "conditional loop", "loop ${1:condition}:\n    ${0}\n.."},
		{"for", "index loop", "for ${1:i} := 0 to ${2:bound}:\n    ${0}\n.."},
		{"ret", "return from the current function", "ret "},
		{"throw", "return an error", "throw "},
		{"try", "propagate a failing call", "try "},
		{"defer", "run cleanup when the scope exits", "defer "},
		{"onerror", "run cleanup when the scope fails", "onerror "},
		{"break", "exit the nearest loop", "break"},
		{"continue", "continue the nearest loop", "continue"},
		{"move", "transfer ownership", "move "},
		{"bounded", "establish a range proof", "bounded ${1:condition}:\n    ${0}\n.."},
		{"unsafe", "localize an unverifiable operation", "unsafe:\n    ${0}\n.."},
		{"sizeof", "size of a type", "sizeof "},
		{"addrof", "address of a value", "addrof "},
		{"not", "invert a boolean", "not "},
		{"true", "boolean literal", "true"},
		{"false", "boolean literal", "false"},
		{"none", "null pointer or function value", "none"},
	} {
		if strings.HasPrefix(keyword.label, prefix) {
			item := completionItem{Label: keyword.label, Kind: 14, Detail: keyword.detail, InsertText: keyword.insert}
			if strings.Contains(keyword.insert, "${") {
				item.InsertTextFormat = 2
			}
			item.SortText = "2:" + keyword.label
			items[keyword.label] = item
		}
	}
	for name, item := range a.docs.expressionSymbols[a.file.PackageName] {
		if strings.HasPrefix(name, prefix) {
			item.SortText = completionSortText(item.Detail, expected, name)
			items[name] = item
		}
	}
	for _, binding := range a.docs.expressionBindingsAt(a.file.PackageName, line) {
		if !strings.HasPrefix(binding.name, prefix) {
			continue
		}
		detail := binding.name + " " + formatType(binding.valueType)
		items[binding.name] = completionItem{Label: binding.name, Kind: 6, Detail: detail, SortText: completionSortText(formatType(binding.valueType), expected, binding.name), Documentation: markdownContent(code(detail))}
	}
	for alias, module := range a.importedPackages() {
		if strings.HasPrefix(alias, "__") || !strings.HasPrefix(alias, prefix) {
			continue
		}
		items[alias] = completionItem{Label: alias, Kind: 9, Detail: "module " + alias, SortText: "1:" + alias, Documentation: markdownContent(a.docs.modules[module])}
	}
	result := make([]completionItem, 0, len(items))
	for _, item := range items {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].SortText != result[j].SortText {
			return result[i].SortText < result[j].SortText
		}
		return result[i].Label < result[j].Label
	})
	return result
}

func completionSortText(detail, expected, label string) string {
	if expected != "" && (detail == expected || strings.HasSuffix(detail, " "+expected) || strings.HasSuffix(detail, ") "+expected)) {
		return "0:" + label
	}
	return "1:" + label
}

func expectedTypeAt(source string, pos position, a *analysis) string {
	lines := strings.Split(source, "\n")
	if int(pos.Line) >= len(lines) {
		return ""
	}
	line := lines[pos.Line]
	if equal := strings.Index(line, "="); equal >= 0 && !strings.HasPrefix(strings.TrimSpace(line[equal:]), "==") {
		fields := strings.Fields(strings.TrimSpace(line[:equal]))
		if len(fields) >= 2 && fields[len(fields)-1] != ":" {
			return fields[len(fields)-1]
		}
	}
	if strings.HasPrefix(strings.TrimSpace(line), "throw ") {
		return "error"
	}
	if !strings.HasPrefix(strings.TrimSpace(line), "ret ") || a == nil || a.docs == nil {
		return ""
	}
	for i := int(pos.Line) - 1; i >= 0; i-- {
		candidate := strings.TrimSpace(lines[i])
		if !strings.HasSuffix(candidate, ":") || strings.HasPrefix(lines[i], " ") || strings.HasPrefix(lines[i], "\t") {
			continue
		}
		open := strings.Index(candidate, "(")
		if open <= 0 {
			continue
		}
		name := strings.TrimSpace(candidate[:open])
		if space := strings.LastIndex(name, " "); space >= 0 {
			name = name[space+1:]
		}
		if fn := a.docs.functionDefs[a.file.PackageName+"\x00"+name]; fn != nil {
			return formatType(fn.ReturnType)
		}
	}
	return ""
}

func topLevelKeywordCompletionAt(source string, pos position) (string, bool) {
	lines := strings.SplitAfter(source, "\n")
	if int(pos.Line) >= len(lines) {
		return "", false
	}
	if insideFunctionBody(lines, int(pos.Line)) {
		return "", false
	}
	runes := []rune(strings.TrimSuffix(strings.TrimSuffix(lines[pos.Line], "\n"), "\r"))
	if int(pos.Character) > len(runes) {
		return "", false
	}
	before := string(runes[:pos.Character])
	if before != strings.TrimLeft(before, " \t") {
		return "", false
	}
	if !identifier(before) {
		return "", false
	}
	return before, true
}

func topLevelKeywordCompletions(prefix string) []completionItem {
	definitions := []struct{ label, detail, insert string }{
		{"mod", "declare this file's module", "mod ${1:name}"},
		{"use", "import a module", "use \"${1:path}\" ${2:alias}"},
		{"pub", "export a declaration", "pub "},
		{"const", "declare a module constant", "const ${1:name} ${2:Type} = ${0:value}"},
		{"alias", "declare a type alias", "alias ${1:Name} = ${0:Type}"},
		{"proto", "declare a prototype", "proto ${1:Name}(\n    ${0}\n)"},
		{"noctx", "declare a contextless function", "noctx "},
		{"destr", "declare a destructor method", "destr "},
		{"ext", "declare an external function", "ext ${1:name}(${2}) ${0:void}"},
		{"link", "link a native library", "link \"${0:library}\""},
		{"bundle", "bundle a native object", "bundle \"${0:path}\""},
		{"llvm", "emit inline LLVM", "llvm \"${0}\""},
	}
	items := []completionItem{}
	for _, definition := range definitions {
		if !strings.HasPrefix(definition.label, prefix) {
			continue
		}
		items = append(items, completionItem{Label: definition.label, Kind: 14, Detail: definition.detail, InsertText: definition.insert, InsertTextFormat: 2, SortText: definition.label})
	}
	return items
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
	// Treat a run of dots immediately before the completion prefix as one
	// unfinished selector. This keeps an accidental second dot from turning
	// the first dot into an invalid trailing component of the receiver.
	selectorDot := dot
	for selectorDot > 0 && before[selectorDot-1] == '.' {
		selectorDot--
	}
	start := selectorDot
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
	receiver := before[start:selectorDot]
	if receiver == "" {
		return completionContext{}, false
	}
	if _, ok := selectorParts(receiver); !ok {
		return completionContext{}, false
	}
	return completionContext{receiver: receiver, prefix: prefix, startByte: lineStart + len(before[:start]), dotByte: lineStart + len(before[:selectorDot]), endByte: lineStart + len(before), lineOffset: lineStart}, true
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
	parts, ok := splitSelectorParts(value)
	if !ok {
		return nil, false
	}
	result := make([]selectorPart, 0, len(parts))
	for _, part := range parts {
		name, call, ok := selectorPartName(part)
		if !ok {
			return nil, false
		}
		result = append(result, selectorPart{name: name, call: call})
	}
	return result, len(result) != 0
}

func splitSelectorParts(value string) ([]string, bool) {
	parts := []string{}
	start := 0
	parenDepth := 0
	bracketDepth := 0
	inString := false
	escaped := false
	for i, r := range value {
		if inString {
			if escaped {
				escaped = false
			} else if r == '\\' {
				escaped = true
			} else if r == '"' {
				inString = false
			}
			continue
		}
		switch r {
		case '"':
			inString = true
		case '(':
			parenDepth++
		case ')':
			parenDepth--
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
		case '.':
			if parenDepth == 0 && bracketDepth == 0 {
				parts = append(parts, value[start:i])
				start = i + 1
			}
		}
		if parenDepth < 0 || bracketDepth < 0 {
			return nil, false
		}
	}
	if inString || parenDepth != 0 || bracketDepth != 0 {
		return nil, false
	}
	parts = append(parts, value[start:])
	return parts, true
}

func selectorPartName(part string) (string, bool, bool) {
	open := strings.IndexByte(part, '(')
	if open < 0 {
		return part, false, identifier(part)
	}
	if !strings.HasSuffix(part, ")") {
		return "", false, false
	}
	name := part[:open]
	if generic := strings.IndexByte(name, '['); generic >= 0 {
		if !strings.HasSuffix(name, "]") {
			return "", false, false
		}
		name = name[:generic]
	}
	return name, true, name != "" && identifier(name)
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

func (d *docIndex) typeCompletions(module, prefix string, exportedOnly, intrinsic bool) []completionItem {
	items := []completionItem{}
	keyPrefix := module + "\x00"
	for key, kind := range d.completionKinds {
		if !strings.HasPrefix(key, keyPrefix) || (kind != 22 && kind != 25) {
			continue
		}
		name := strings.TrimPrefix(key, keyPrefix)
		if strings.Contains(name, ".") || !strings.HasPrefix(name, prefix) {
			continue
		}
		if exportedOnly && !d.completionVisible[key] {
			continue
		}
		items = append(items, completionItem{Label: name, Kind: kind, Detail: firstCodeLine(d.hoverSymbols[key]), Documentation: markdownContent(d.hoverSymbols[key])})
	}
	if intrinsic {
		for name := range magmatypes.BasicTypes {
			if strings.HasPrefix(name, prefix) {
				items = append(items, completionItem{Label: name, Kind: 25, Detail: "intrinsic type"})
			}
		}
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

func (d *docIndex) structFieldCompletions(module, owner, prefix string, used map[string]bool) []completionItem {
	keyPrefix := module + "\x00" + owner + "."
	items := []completionItem{}
	for key, fieldType := range d.memberTypes {
		if !strings.HasPrefix(key, keyPrefix) {
			continue
		}
		name := strings.TrimPrefix(key, keyPrefix)
		if strings.Contains(name, ".") || used[name] || !strings.HasPrefix(name, prefix) {
			continue
		}
		detail := name + " " + formatType(fieldType)
		items = append(items, completionItem{Label: name, Kind: 5, Detail: detail, InsertText: name + "="})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Label < items[j].Label })
	return items
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
		} else if name == "proto" && kind == 2 {
			filterText = name
			insertText = "proto()"
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
