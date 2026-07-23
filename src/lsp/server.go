// Package lsp exposes Magma's semantic pipeline over the Language Server Protocol.
package lsp

import (
	"Magma/src/checker"
	"Magma/src/monomorph"
	"Magma/src/pipeline"
	"Magma/src/shared"
	"Magma/src/types"
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"
)

type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}
type position struct {
	Line      uint32 `json:"line"`
	Character uint32 `json:"character"`
}
type document struct {
	URI, Text string
	Version   int
	result    *analysis
}
type server struct {
	in        *bufio.Reader
	out       io.Writer
	documents map[string]*document
}
type analysis struct {
	file *types.FileCtx
	err  error
	docs *docIndex
}

// Serve processes LSP messages until exit or end-of-file.
func Serve(input io.Reader, output io.Writer) error {
	s := &server{bufio.NewReader(input), output, map[string]*document{}}
	for {
		payload, err := readMessage(s.in)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		var msg message
		if json.Unmarshal(payload, &msg) != nil {
			continue
		}
		if msg.Method == "exit" {
			return nil
		}
		if err := s.handle(msg); err != nil && len(msg.ID) != 0 {
			_ = s.respondError(msg.ID, -32603, err.Error())
		}
	}
}

func readMessage(r *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if ok && strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			length, err = strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, err
			}
		}
	}
	if length < 0 {
		return nil, fmt.Errorf("missing Content-Length")
	}
	p := make([]byte, length)
	_, err := io.ReadFull(r, p)
	return p, err
}

func (s *server) handle(msg message) error {
	switch msg.Method {
	case "initialize":
		return s.respond(msg.ID, map[string]any{"capabilities": map[string]any{"textDocumentSync": 1, "hoverProvider": true}})
	case "shutdown":
		return s.respond(msg.ID, nil)
	case "initialized", "$/cancelRequest", "textDocument/didSave":
		return nil
	case "textDocument/didOpen":
		var p struct {
			TextDocument struct {
				URI     string `json:"uri"`
				Text    string `json:"text"`
				Version int    `json:"version"`
			} `json:"textDocument"`
		}
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return err
		}
		s.documents[p.TextDocument.URI] = &document{URI: p.TextDocument.URI, Text: p.TextDocument.Text, Version: p.TextDocument.Version}
	case "textDocument/didChange":
		var p struct {
			TextDocument struct {
				URI     string `json:"uri"`
				Version int    `json:"version"`
			} `json:"textDocument"`
			ContentChanges []struct {
				Text string `json:"text"`
			} `json:"contentChanges"`
		}
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return err
		}
		if d := s.documents[p.TextDocument.URI]; d != nil && len(p.ContentChanges) > 0 {
			d.Text = p.ContentChanges[len(p.ContentChanges)-1].Text
			d.Version = p.TextDocument.Version
			d.result = nil
		}
	case "textDocument/didClose":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return err
		}
		delete(s.documents, p.TextDocument.URI)
	case "textDocument/hover":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			Position position `json:"position"`
		}
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return err
		}
		d := s.documents[p.TextDocument.URI]
		if d == nil {
			return s.respond(msg.ID, nil)
		}
		if d.result == nil {
			d.result = analyze(d.URI, d.Text)
		}
		value := d.result.hover(p.Position)
		if value == "" {
			return s.respond(msg.ID, nil)
		}
		return s.respond(msg.ID, map[string]any{"contents": map[string]string{"kind": "markdown", "value": value}})
	default:
		if len(msg.ID) != 0 {
			return s.respondError(msg.ID, -32601, "method not found")
		}
	}
	return nil
}

func (s *server) respond(id json.RawMessage, result any) error {
	return s.write(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}
func (s *server) respondError(id json.RawMessage, code int, text string) error {
	return s.write(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": text}})
}
func (s *server) write(v any) error {
	p, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(s.out, "Content-Length: %d\r\n\r\n%s", len(p), p)
	return err
}

func analyze(rawURI, source string) *analysis {
	path, err := uriPath(rawURI)
	if err != nil {
		return &analysis{err: err}
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return &analysis{err: err}
	}
	state, err := shared.MakeShared(filepath.Dir(path))
	if err != nil {
		return &analysis{err: err}
	}
	state.SourceOverrides[path] = []byte(source)
	err = pipeline.DoMain(state, path)
	state.WaitGroup.Wait()
	for _, result := range state.ImportedFiles {
		if e := <-result; e != nil && err == nil {
			err = e
		}
	}
	file := state.Files[path]
	docs := buildDocIndex(state)
	if err != nil || file == nil || file.GlNode == nil {
		if err != nil {
			fmt.Fprintf(os.Stderr, "magma-lsp: analysis failed for %s: %v\n", path, err)
		}
		return &analysis{file: file, err: err, docs: docs}
	}
	if err = monomorph.Run(state); err == nil {
		err = checker.CheckLinks(state)
	}
	if err == nil {
		err = checker.TypeChecker(state)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "magma-lsp: semantic analysis failed for %s: %v\n", path, err)
	}
	return &analysis{file: file, err: err, docs: docs}
}

func uriPath(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme != "file" {
		return "", fmt.Errorf("unsupported URI scheme %q", u.Scheme)
	}
	p := filepath.FromSlash(u.Path)
	if len(p) >= 3 && p[0] == filepath.Separator && p[2] == ':' {
		p = p[1:]
	}
	return p, nil
}

func (a *analysis) hover(pos position) string {
	// Linking and type checking intentionally retain the successfully resolved
	// portion of the tree. Editor buffers are often temporarily invalid, so a
	// later diagnostic must not suppress hover information for earlier nodes.
	if a == nil || a.file == nil || a.file.GlNode == nil {
		return ""
	}
	var sourceToken *types.Token
	sourceTokenIndex := -1
	for i := range a.file.Tokens {
		if tokenAt(a.file.Tokens[i], pos) {
			sourceToken = &a.file.Tokens[i]
			sourceTokenIndex = i
			break
		}
	}
	// Prefer information indexed from the intact source tree. Transformed nodes
	// may deliberately reuse source positions and tokens, while this index is
	// keyed by the actual token in the editor buffer.
	if sourceTokenIndex >= 0 && sourceToken.Type == types.TokName && a.docs != nil {
		if value := a.tokenTypeHover(sourceTokenIndex); value != "" {
			return value
		}
	}
	f := hoverFinder{pos: pos, seen: map[uintptr]bool{}, analysis: a, sourceToken: sourceToken}
	f.walk(reflect.ValueOf(a.file.GlNode))
	if f.value == "" && a.docs != nil {
		for i, token := range a.file.Tokens {
			if token.Type == types.TokName && tokenAt(token, pos) {
				if module := a.importedPackage(token.Repr); module != "" {
					return a.docs.modules[module]
				}
				if value := a.tokenTypeHover(i); value != "" {
					return value
				}
			}
		}
	}
	return f.value
}

// tokenTypeHover covers type occurrences whose named AST node was replaced by
// an absolute type during semantic analysis. Absolute types intentionally have
// no token, so parameter lists, struct field lists, and return types need to be
// resolved from the source token that remains in FileCtx.
func (a *analysis) tokenTypeHover(index int) string {
	if a == nil || a.file == nil || a.docs == nil || index < 0 || index >= len(a.file.Tokens) {
		return ""
	}
	token := a.file.Tokens[index]
	if hover := a.docs.valueHovers[scopedTokenPositionKey(a.file.PackageName, token)]; hover != "" {
		return hover
	}
	module := a.file.PackageName
	displayName := token.Repr
	var receiver string
	if index >= 2 && a.file.Tokens[index-1].KeywType == types.KwDot {
		qualifier := a.file.Tokens[index-2]
		if qualifier.Type == types.TokName {
			receiver = qualifier.Repr
			imported := a.importedPackage(qualifier.Repr)
			if imported != "" {
				module = imported
				displayName = qualifier.Repr + "." + token.Repr
			}
		}
	}
	key := module + "\x00" + token.Repr
	if hover := a.docs.hoverSymbols[key]; hover != "" {
		return hover
	}
	docs := a.docs.symbols[key]
	if docs != "" {
		return joinHover(code(displayName), docs)
	}
	if receiver != "" {
		if hover := a.receiverMemberHover(index, receiver, token.Repr); hover != "" {
			return hover
		}
	}
	if hover := a.docs.hoverByName[token.Repr]; hover != "" {
		return hover
	}
	return ""
}

// receiverMemberHover recovers member information in generic bodies pruned by
// monomorphization. It looks backward for the receiver's nearest typed
// declaration, such as `a alc.Allocator` or `items Array[T]*`, and then queries
// the pre-monomorphization symbol index with that owner type.
func (a *analysis) receiverMemberHover(index int, receiver, member string) string {
	tokens := a.file.Tokens
	for i := index - 3; i >= 0; i-- {
		if tokens[i].Type != types.TokName || tokens[i].Repr != receiver || i+1 >= len(tokens) {
			continue
		}
		first := tokens[i+1]
		if first.Type != types.TokName {
			continue
		}
		module := a.file.PackageName
		owner := first.Repr
		if i+3 < len(tokens) && tokens[i+2].KeywType == types.KwDot && tokens[i+3].Type == types.TokName {
			imported := a.importedPackage(first.Repr)
			if imported == "" {
				continue
			}
			module = imported
			owner = tokens[i+3].Repr
		}
		if hover := a.docs.hoverSymbols[module+"\x00"+owner+"."+member]; hover != "" {
			return hover
		}
	}
	// `this` is inserted implicitly and has no source declaration token to scan.
	// A unique owner.member entry within the current module still identifies the
	// field safely (and also helps temporarily incomplete receiver declarations).
	prefix := a.file.PackageName + "\x00"
	suffix := "." + member
	match := ""
	for key, hover := range a.docs.hoverSymbols {
		if strings.HasPrefix(key, prefix) && strings.HasSuffix(key, suffix) {
			if match != "" && match != hover {
				return ""
			}
			match = hover
		}
	}
	if receiver == "this" {
		return match
	}
	return ""
}

type hoverFinder struct {
	pos         position
	seen        map[uintptr]bool
	value       string
	analysis    *analysis
	sourceToken *types.Token
}

func (f *hoverFinder) tokenAt(token types.Token) bool {
	if !tokenAt(token, f.pos) {
		return false
	}
	return f.sourceToken == nil || (token.Pos == f.sourceToken.Pos && token.Repr == f.sourceToken.Repr)
}

func (f *hoverFinder) nameAt(name types.NodeName) bool {
	switch node := name.(type) {
	case *types.NodeNameSingle:
		if !f.tokenAt(node.Tk) {
			return false
		}
		// Monomorphized/generated nodes can retain a source token while changing
		// the semantic name stored beside it. Reject those stale-token aliases;
		// sourceName still permits an ordinary generic specialization.
		return f.sourceToken == nil || sourceName(node.Name) == f.sourceToken.Repr
	case *types.NodeNameComposite:
		for i, token := range node.Tokens {
			if f.tokenAt(token) {
				if f.sourceToken == nil || (i < len(node.Parts) && sourceName(node.Parts[i]) == f.sourceToken.Repr) {
					return true
				}
			}
		}
	}
	return false
}

func (f *hoverFinder) walk(v reflect.Value) {
	if f.value != "" || !v.IsValid() {
		return
	}
	if v.Kind() == reflect.Interface {
		if !v.IsNil() {
			f.walk(v.Elem())
		}
		return
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() || f.seen[v.Pointer()] {
			return
		}
		f.seen[v.Pointer()] = true
		f.inspect(v.Interface())
		f.walk(v.Elem())
		return
	}
	switch v.Kind() {
	case reflect.Struct:
		if v.CanInterface() {
			f.inspect(v.Interface())
			if f.value != "" {
				return
			}
		}
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			field := t.Field(i)
			if field.PkgPath != "" || skipField(field.Name) {
				continue
			}
			f.walk(v.Field(i))
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			f.walk(v.Index(i))
		}
	case reflect.Map:
		iter := v.MapRange()
		for iter.Next() {
			f.walk(iter.Value())
		}
	}
}
func skipField(name string) bool {
	switch name {
	case "Parent", "Scope", "AssociatedNode", "AssociatedFnDef", "Destructor", "Destructors":
		return true
	}
	return false
}
func (f *hoverFinder) inspect(value any) {
	switch n := value.(type) {
	case *types.NodeExprName:
		if f.nameAt(n.Name) {
			if module := f.analysis.importedModuleAt(n.Name, f.pos); module != "" {
				f.value = f.analysis.docs.modules[module]
			} else {
				f.value = f.analysis.withDocs(hoverExpression(n, f.pos), n.AssociatedNode)
			}
		}
	case *types.NodeExprVarDef:
		if f.nameAt(n.Name) {
			f.value = code(formatVariable(n))
		}
	case types.NodeArg:
		if f.tokenAt(n.Tk) {
			f.value = code(n.Name + " " + formatType(n.TypeNode))
		}
	case types.NodeStructFieldInit:
		if f.tokenAt(n.Tk) && n.FieldType != nil {
			f.value = code(n.Name + " " + formatType(n.FieldType))
		}
	case *types.NodeExprMemberAccess:
		if f.tokenAt(n.Tk) && n.Access != nil {
			f.value = code(n.Member + " " + formatType(n.Access.Type))
		}
	case *types.NodeFuncDef:
		if f.nameAt(n.Class.NameNode) {
			f.value = f.analysis.withDocs(code(formatFunction(n)), n)
		}
	case *types.NodeStructDef:
		if f.nameAt(n.Class.NameNode) {
			f.value = f.analysis.withDocs(code("struct "+flattenName(n.Class.NameNode)), n)
		}
	case *types.NodeTypeAlias:
		if n.Alias != nil && f.tokenAt(n.Alias.Tk) {
			f.value = f.analysis.withDocs(code("alias "+n.Alias.Name+" = "+formatType(n.Alias.Target)), n.Alias)
		}
	case *types.NodeTypeNamed:
		if f.nameAt(n.NameNode) {
			f.value = f.analysis.hoverType(n)
		}
	}
}
func hoverExpression(n *types.NodeExprName, pos position) string {
	if composite, ok := n.Name.(*types.NodeNameComposite); ok {
		for i, token := range composite.Tokens {
			if i > 0 && tokenAt(token, pos) && i-1 < len(n.MemberAccesses) && n.MemberAccesses[i-1] != nil {
				return code(composite.Parts[i] + " " + formatType(n.MemberAccesses[i-1].Type))
			}
		}
	}
	switch d := n.AssociatedNode.(type) {
	case *types.NodeFuncDef:
		return code(formatFunction(d))
	case *types.NodeExprVarDef:
		return code(formatVariableWithType(d, n.InfType))
	case *types.NodeStructDef:
		return code("struct " + flattenName(d.Class.NameNode))
	case *types.StructDef:
		return code("struct " + d.Name)
	}
	if n.InfType != nil {
		return code(flattenName(n.Name) + ": " + formatType(n.InfType))
	}
	return ""
}

func formatVariable(variable *types.NodeExprVarDef) string {
	return formatVariableWithType(variable, variable.Type)
}

func formatVariableWithType(variable *types.NodeExprVarDef, valueType *types.NodeType) string {
	prefix := ""
	if variable.IsConst {
		prefix = "const "
	}
	return prefix + flattenName(variable.Name) + " " + formatType(valueType)
}
func code(s string) string { return "```magma\n" + s + "\n```" }
func nameAt(name types.NodeName, pos position) bool {
	switch n := name.(type) {
	case *types.NodeNameSingle:
		return tokenAt(n.Tk, pos)
	case *types.NodeNameComposite:
		for _, tk := range n.Tokens {
			if tokenAt(tk, pos) {
				return true
			}
		}
	}
	return false
}
func tokenAt(tk types.Token, pos position) bool {
	if tk.Pos.Line != pos.Line+1 {
		return false
	}
	start := tk.Pos.Col - 1
	end := start + uint32(utf8.RuneCountInString(tk.Repr))
	return pos.Character >= start && pos.Character < end
}
func flattenName(name types.NodeName) string {
	return sourceName(flattenInternalName(name))
}
func flattenInternalName(name types.NodeName) string {
	switch n := name.(type) {
	case *types.NodeNameSingle:
		return n.Name
	case *types.NodeNameComposite:
		return strings.Join(n.Parts, ".")
	}
	return "?"
}

// sourceName converts a compiler identifier back to the spelling users wrote.
// Monomorphized declarations retain a __g__ suffix because the backend needs a
// unique symbol for every specialization; that suffix is never Magma syntax and
// must not be exposed by editor features.
func sourceName(name string) string {
	parts := strings.Split(name, ".")
	// Package identifiers are made unique internally as <module>_<10 chars>.
	// Only qualified names can contain a package component; leaving an
	// unqualified identifier alone avoids changing a legitimate user symbol
	// that happens to end in the same shape.
	if len(parts) > 1 {
		if suffix := strings.LastIndex(parts[0], "_"); suffix >= 0 && len(parts[0])-suffix-1 == 10 {
			parts[0] = parts[0][:suffix]
		}
	}
	for i, part := range parts {
		if generic := strings.Index(part, "__g__"); generic >= 0 {
			parts[i] = part[:generic]
		}
	}
	return strings.Join(parts, ".")
}
func formatFunction(fn *types.NodeFuncDef) string {
	args := make([]string, 0, len(fn.Class.ArgsNode.Args))
	for _, a := range fn.Class.ArgsNode.Args {
		args = append(args, a.Name+" "+formatType(a.TypeNode))
	}
	name := flattenName(fn.Class.NameNode)
	if fn.DisplayName != "" {
		name = sourceName(fn.DisplayName)
	}
	return name + "(" + strings.Join(args, ", ") + ") " + formatType(fn.ReturnType)
}
func formatType(node *types.NodeType) string {
	if node == nil {
		return "?"
	}
	var out string
	switch k := node.KindNode.(type) {
	case *types.NodeTypeNamed:
		out = flattenName(k.NameNode)
		if len(k.GenericArgs) > 0 {
			a := []string{}
			for _, x := range k.GenericArgs {
				a = append(a, formatType(x))
			}
			out += "[" + strings.Join(a, ", ") + "]"
		}
	case *types.NodeTypeAbsolute:
		out = sourceName(k.AbsoluteName)
	case *types.NodeTypeCompilerKnown:
		out = k.Name
	case *types.NodeTypePointer:
		out = "*" + formatType(&types.NodeType{KindNode: k.Kind})
	case *types.NodeTypeRfc:
		out = "&" + formatType(&types.NodeType{KindNode: k.Kind})
	case *types.NodeTypeSlice:
		out = "[]" + formatType(&types.NodeType{KindNode: k.ElemKind})
	case *types.NodeTypeFunc:
		a := []string{}
		for _, x := range k.Args {
			a = append(a, formatType(x))
		}
		out = "fn(" + strings.Join(a, ", ") + "): " + formatType(k.RetType)
	default:
		out = "?"
	}
	if node.Owned {
		out = "owned " + out
	}
	if node.Throws {
		out = "!" + out
	}
	return out
}
