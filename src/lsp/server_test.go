package lsp

import (
	"Magma/src/types"
	"strings"
	"testing"
)

func TestFormatFunctionUsesMagmaSyntax(t *testing.T) {
	strType := &types.NodeType{KindNode: &types.NodeTypeNamed{NameNode: name("str")}}
	u64Type := &types.NodeType{KindNode: &types.NodeTypeNamed{NameNode: name("u64")}, Throws: true}
	function := &types.NodeFuncDef{
		Class: types.NodeGenericClass{
			NameNode: name("printLn"),
			ArgsNode: types.NodeArgList{Args: []types.NodeArg{{Name: "bytes", TypeNode: strType}}},
		},
		ReturnType: u64Type,
	}

	if got, want := formatFunction(function), "printLn(bytes str) !u64"; got != want {
		t.Fatalf("formatFunction() = %q, want %q", got, want)
	}
}

func TestArgumentAndVariableHoverUseMagmaSyntax(t *testing.T) {
	u64Type := &types.NodeType{KindNode: &types.NodeTypeNamed{NameNode: name("u64")}}
	argument := types.NodeArg{
		Tk:       types.Token{Repr: "count", Type: types.TokName, Pos: types.FilePos{Line: 2, Col: 5}},
		Name:     "count",
		TypeNode: u64Type,
	}
	finder := hoverFinder{pos: position{Line: 1, Character: 4}, seen: map[uintptr]bool{}}
	finder.inspect(argument)
	if got, want := finder.value, code("count u64"); got != want {
		t.Fatalf("argument hover = %q, want %q", got, want)
	}

	variable := &types.NodeExprVarDef{Name: name("total"), Type: u64Type}
	if got, want := formatVariable(variable), "total u64"; got != want {
		t.Fatalf("variable hover = %q, want %q", got, want)
	}
}

func TestFieldAccessHoverUsesFieldType(t *testing.T) {
	u64Type := &types.NodeType{KindNode: &types.NodeTypeNamed{NameNode: name("u64")}}
	expression := &types.NodeExprName{
		Name: &types.NodeNameComposite{
			Parts: []string{"value", "count"},
			Tokens: []types.Token{
				{Repr: "value", Type: types.TokName, Pos: types.FilePos{Line: 1, Col: 1}},
				{Repr: "count", Type: types.TokName, Pos: types.FilePos{Line: 1, Col: 7}},
			},
		},
		MemberAccesses: []*types.MemberAccess{{Type: u64Type}},
	}
	if got, want := hoverExpression(expression, position{Line: 0, Character: 6}), code("count u64"); got != want {
		t.Fatalf("field hover = %q, want %q", got, want)
	}
}

func TestArgumentUsageSurvivesGenericTemplatePruning(t *testing.T) {
	u64Type := &types.NodeType{KindNode: &types.NodeTypeNamed{NameNode: name("u64")}}
	usageToken := types.Token{Repr: "index", Type: types.TokName, Pos: types.FilePos{Line: 3, Col: 8}}
	function := &types.NodeFuncDef{
		Class: types.NodeGenericClass{
			ArgsNode: types.NodeArgList{
				Args: []types.NodeArg{
					{
						Tk:       types.Token{Repr: "index", Type: types.TokName, Pos: types.FilePos{Line: 1, Col: 14}},
						Name:     "index",
						TypeNode: u64Type,
					},
				},
			},
		},
		Body: types.NodeBody{
			Statements: []types.NodeStatement{
				&types.NodeStmtExpr{
					Expression: &types.NodeExprName{
						Name: &types.NodeNameSingle{Tk: usageToken, Name: "index"},
					},
				},
			},
		},
	}
	index := &docIndex{valueHovers: map[string]string{}}
	index.indexFunctionValueUsages("test_module", function)
	if got, want := index.valueHovers[scopedTokenPositionKey("test_module", usageToken)], code("index u64"); got != want {
		t.Fatalf("argument usage hover = %q, want %q", got, want)
	}
}

func TestLocalUsageSurvivesGenericTemplatePruning(t *testing.T) {
	u64Type := &types.NodeType{KindNode: &types.NodeTypeNamed{NameNode: name("u64")}}
	declarationToken := types.Token{Repr: "idx", Type: types.TokName, Pos: types.FilePos{Line: 2, Col: 5}}
	usageToken := types.Token{Repr: "idx", Type: types.TokName, Pos: types.FilePos{Line: 3, Col: 22}}
	local := &types.NodeExprVarDef{Name: &types.NodeNameSingle{Tk: declarationToken, Name: "idx"}, Type: u64Type}
	usage := &types.NodeExprName{Name: &types.NodeNameSingle{Tk: usageToken, Name: "idx"}}
	function := &types.NodeFuncDef{Body: types.NodeBody{Statements: []types.NodeStatement{
		&types.NodeStmtExpr{Expression: local},
		&types.NodeStmtExpr{Expression: usage},
	}}}
	index := &docIndex{valueHovers: map[string]string{}}
	index.indexFunctionValueUsages("test_module", function)
	if got, want := index.valueHovers[scopedTokenPositionKey("test_module", usageToken)], code("idx u64"); got != want {
		t.Fatalf("local usage hover = %q, want %q", got, want)
	}
}

func TestHoverRejectsGeneratedTokenPositionCollision(t *testing.T) {
	positionToken := types.FilePos{Line: 2, Col: 9}
	generation := &types.NodeExprVarDef{
		Name: &types.NodeNameComposite{
			Tokens: []types.Token{{Repr: "Async", Type: types.TokName, Pos: positionToken}},
			Parts:  []string{"generation"},
		},
		Type: &types.NodeType{KindNode: &types.NodeTypePointer{Kind: &types.NodeTypeNamed{NameNode: name("u32")}}},
	}
	a := &analysis{
		file: &types.FileCtx{
			PackageName: "async_1234567890",
			ImportAlias: map[string]string{},
			Tokens:      []types.Token{{Repr: "Async", Type: types.TokName, Pos: positionToken}},
			GlNode:      &types.NodeGlobal{Declarations: []types.NodeGlobalDecl{generation}},
		},
		docs: &docIndex{
			hoverSymbols: map[string]string{"async_1234567890\x00Async": code("struct Async")},
			valueHovers:  map[string]string{},
			modules:      map[string]string{},
			symbols:      map[string]string{},
			byNode:       map[any]string{},
		},
	}
	got := a.hover(position{Line: 1, Character: 8})
	if got != code("struct Async") {
		t.Fatalf("hover selected a generated token at the same position: %q", got)
	}
}

func TestValueIndexRejectsSyntheticNameWithRetainedSourceToken(t *testing.T) {
	token := types.Token{Repr: "Async", Type: types.TokName, Pos: types.FilePos{Line: 2, Col: 9}}
	synthetic := &types.NodeExprVarDef{
		Name: &types.NodeNameSingle{Tk: token, Name: "generation"},
		Type: &types.NodeType{KindNode: &types.NodeTypePointer{Kind: &types.NodeTypeNamed{NameNode: name("u32")}}},
	}
	index := &docIndex{valueHovers: map[string]string{}}
	index.indexValueDeclarations("test_module", synthetic)
	if got := index.valueHovers[scopedTokenPositionKey("test_module", token)]; got != "" {
		t.Fatalf("synthetic variable polluted source hover index: %q", got)
	}
}

func TestFormatFunctionUsesGenericDisplayName(t *testing.T) {
	u64Type := &types.NodeType{KindNode: &types.NodeTypeNamed{NameNode: name("u64")}}
	function := &types.NodeFuncDef{
		Class: types.NodeGenericClass{
			NameNode: name("identity__g__N_u64"),
			ArgsNode: types.NodeArgList{Args: []types.NodeArg{{Name: "value", TypeNode: u64Type}}},
		},
		ReturnType:  u64Type,
		DisplayName: "identity[u64]",
	}

	if got, want := formatFunction(function), "identity[u64](value u64) u64"; got != want {
		t.Fatalf("formatFunction() = %q, want %q", got, want)
	}
}

func TestHoverFormattingHidesGenericTypeMangling(t *testing.T) {
	typeNode := &types.NodeType{KindNode: &types.NodeTypeAbsolute{AbsoluteName: "collections_a1b2c3d4e5.Box__g__N_u64"}}
	if got, want := formatType(typeNode), "collections.Box"; got != want {
		t.Fatalf("formatType() = %q, want %q", got, want)
	}
	if got, want := flattenName(name("Box__g__N_u64")), "Box"; got != want {
		t.Fatalf("flattenName() = %q, want %q", got, want)
	}
}

func TestHoverFormattingHidesImportedModuleMangling(t *testing.T) {
	typeNode := &types.NodeType{KindNode: &types.NodeTypeAbsolute{AbsoluteName: "reader_Z8f4Q1w2Er.Reader"}}
	if got, want := formatType(typeNode), "reader.Reader"; got != want {
		t.Fatalf("formatType() = %q, want %q", got, want)
	}
	if got, want := sourceName("value_1234567890"), "value_1234567890"; got != want {
		t.Fatalf("sourceName() changed an unqualified user identifier: got %q, want %q", got, want)
	}
}

func TestGenericStructCloneRetainsDocumentationOnHover(t *testing.T) {
	clone := &types.StructDef{Module: "collections_a1b2c3d4e5", Name: "Box__g__N_u64"}
	a := &analysis{docs: &docIndex{
		byNode:       map[any]string{},
		modules:      map[string]string{},
		symbols:      map[string]string{"collections_a1b2c3d4e5\x00Box": "Stores one value."},
		hoverSymbols: map[string]string{},
	}}

	got := a.withDocs(code("struct collections.Box"), clone)
	if !strings.Contains(got, "Stores one value.") {
		t.Fatalf("hover omitted documentation for monomorphized type clone: %q", got)
	}
}

func TestGenericStructDeclarationCloneRetainsDocumentationOnHover(t *testing.T) {
	clone := &types.NodeStructDef{Class: types.NodeGenericClass{NameNode: name("Box__g__N_u64")}}
	a := &analysis{
		file: &types.FileCtx{PackageName: "collections_a1b2c3d4e5"},
		docs: &docIndex{
			byNode:       map[any]string{},
			modules:      map[string]string{},
			symbols:      map[string]string{"collections_a1b2c3d4e5\x00Box": "Stores one value."},
			hoverSymbols: map[string]string{},
		},
	}

	got := a.withDocs(code("struct Box"), clone)
	if !strings.Contains(got, "Stores one value.") {
		t.Fatalf("declaration hover omitted documentation for monomorphized type clone: %q", got)
	}
}

func TestReturnTypeDocumentationFallsBackToSourceToken(t *testing.T) {
	a := &analysis{
		file: &types.FileCtx{
			PackageName: "main_1234567890",
			ImportAlias: map[string]string{},
			Tokens:      []types.Token{{Repr: "Result", Type: types.TokName, Pos: types.FilePos{Line: 3, Col: 18}}},
			GlNode:      &types.NodeGlobal{},
		},
		docs: &docIndex{
			byNode:  map[any]string{},
			modules: map[string]string{},
			symbols: map[string]string{"main_1234567890\x00Result": "The result of an operation."},
		},
	}

	got := a.hover(position{Line: 2, Character: 17})
	if !strings.Contains(got, "The result of an operation.") {
		t.Fatalf("return type hover omitted documentation: %q", got)
	}
}

func TestQualifiedListTypeDocumentationFallsBackToSourceToken(t *testing.T) {
	a := &analysis{
		file: &types.FileCtx{
			PackageName: "main_1234567890",
			ImportAlias: map[string]string{"net_types": "types_a1b2c3d4e5"},
			Tokens: []types.Token{
				{Repr: "net_types", Type: types.TokName, Pos: types.FilePos{Line: 4, Col: 12}},
				{Repr: ".", Type: types.TokKeyword, KeywType: types.KwDot, Pos: types.FilePos{Line: 4, Col: 21}},
				{Repr: "Address", Type: types.TokName, Pos: types.FilePos{Line: 4, Col: 22}},
			},
			GlNode: &types.NodeGlobal{},
		},
		docs: &docIndex{
			byNode:  map[any]string{},
			modules: map[string]string{},
			symbols: map[string]string{"types_a1b2c3d4e5\x00Address": "A network endpoint address."},
		},
	}

	got := a.hover(position{Line: 3, Character: 21})
	if !strings.Contains(got, "A network endpoint address.") || !strings.Contains(got, "net_types.Address") {
		t.Fatalf("qualified list type hover was incomplete: %q", got)
	}
}

func TestResolvedImportedTypeUsesOwningModuleDocumentation(t *testing.T) {
	node := &types.NodeTypeNamed{NameNode: &types.NodeNameComposite{
		Parts: []string{"allocator_a1b2c3d4e5", "Allocator"},
	}}
	a := &analysis{
		file: &types.FileCtx{
			PackageName: "main_1234567890",
			ImportAlias: map[string]string{"alc": `C:\std\allocator.mg`},
			GlNode: &types.NodeGlobal{ImportAlias: map[string]string{
				"alc": "allocator_a1b2c3d4e5",
			}},
		},
		docs: &docIndex{
			byNode:  map[any]string{},
			modules: map[string]string{},
			symbols: map[string]string{"allocator_a1b2c3d4e5\x00Allocator": "Provides memory allocation."},
		},
	}

	got := a.hoverType(node)
	if !strings.Contains(got, "Provides memory allocation.") {
		t.Fatalf("resolved imported type hover omitted documentation: %q", got)
	}
	if strings.Contains(got, "a1b2c3d4e5") {
		t.Fatalf("resolved imported type hover exposed its package identifier: %q", got)
	}
}

func TestParseDocumentation(t *testing.T) {
	source := `mod strings
# String helpers.

# Frees an allocated string.
# @param a allocator
# @param s allocated slice
# @returns nothing
pub free(a Allocator, s $str) void:
    # This body comment must not become documentation.
    s.free(a)
..
`
	byLine, module := parseDocumentation(source)
	if got, want := module.markdown(), "String helpers."; got != want {
		t.Fatalf("module docs = %q, want %q", got, want)
	}
	got := byLine[8].markdown()
	for _, want := range []string{"Frees an allocated string.", "`a` — allocator", "`s` — allocated slice", "**Returns:** nothing"} {
		if !strings.Contains(got, want) {
			t.Errorf("declaration docs %q do not contain %q", got, want)
		}
	}
	if len(byLine) != 1 {
		t.Fatalf("parsed %d declaration doc blocks, want 1", len(byLine))
	}
}

func TestModuleDocumentationMustImmediatelyFollowModule(t *testing.T) {
	_, module := parseDocumentation("mod sample\n\n# Not module docs.\nThing()\n")
	if got := module.markdown(); got != "" {
		t.Fatalf("module docs = %q, want none", got)
	}
}

func TestDocumentationTagsRenderAsMarkdown(t *testing.T) {
	doc := parseDocBlock([]string{
		"Allocates a resource.",
		"@warning The memory is uninitialized.",
		"@note Allocation depends on the backend.",
		"@complexity O(N) for zeroing.",
		"@throws outOfMemory if allocation fails",
		"@ownership The caller owns the return value.",
		"@safety byteCount must fit in addressable memory.",
		"@mustcall free",
		"@platform windows, linux",
		"@deprecated Use allocZero instead.",
		"@see allocZero",
		"@example",
		"  block := try alloc(a, 16)",
		"  a.free(block)",
	})
	got := doc.markdown()
	wants := []string{
		"> **⚠ Warning:** The memory is uninitialized.",
		"> **Note:** Allocation depends on the backend.",
		"**Complexity:** O(N) for zeroing.",
		"**Throws:** outOfMemory if allocation fails",
		"**Ownership:** The caller owns the return value.",
		"> **Safety:** byteCount must fit in addressable memory.",
		"**Must call:** `free`",
		"**Platforms:** windows, linux",
		"> **Deprecated:** Use allocZero instead.",
		"**See also:** `allocZero`",
		"```magma\nblock := try alloc(a, 16)\na.free(block)\n```",
	}
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Errorf("rendered docs do not contain %q:\n%s", want, got)
		}
	}
}

func TestMethodDocumentationUsesMemberTokenLine(t *testing.T) {
	methodName := &types.NodeNameComposite{
		Parts: []string{"Allocator", "alloc"},
		Tokens: []types.Token{
			{Repr: "Allocator", Type: types.TokName, Pos: types.FilePos{Line: 12, Col: 1}},
			{Repr: "alloc", Type: types.TokName, Pos: types.FilePos{Line: 12, Col: 11}},
		},
	}
	if got, want := nameLine(methodName), uint32(12); got != want {
		t.Fatalf("method documentation line = %d, want %d", got, want)
	}
}

func TestTagConsumesLinesUntilNextTag(t *testing.T) {
	doc := parseDocBlock([]string{
		"Returns a str from a pointer and a length in bytes.",
		"@warning This ties the lifetime of the input pointer to the output Magma str,",
		"if the input pointer is deallocated, it will result in invalid reads.",
		"Prefer using fromPtr when unsure about lifetimes.",
		"@complexity O(1)",
		"@param s input string",
	})
	got := doc.markdown()
	warning := "> **⚠ Warning:** This ties the lifetime of the input pointer to the output Magma str,\n" +
		"> if the input pointer is deallocated, it will result in invalid reads.\n" +
		"> Prefer using fromPtr when unsure about lifetimes."
	if !strings.Contains(got, warning) {
		t.Fatalf("multiline warning was not kept together:\n%s", got)
	}
	if !strings.Contains(got, "**Complexity:** O(1)") || !strings.Contains(got, "`s` — input string") {
		t.Fatalf("following tags were not parsed independently:\n%s", got)
	}
	if strings.Contains(got, "output Magma str,\nif the input") {
		t.Fatalf("warning continuation leaked into plain description:\n%s", got)
	}
}

func name(value string) *types.NodeNameSingle {
	return &types.NodeNameSingle{Name: value}
}
