package parser

import (
	"Magma/src/tokenizer"
	mt "Magma/src/types"
	"strings"
	"testing"
)

func parseTestSource(tt *testing.T, source string) (*mt.NodeGlobal, error) {
	tt.Helper()
	fctx := &mt.FileCtx{
		FilePath:    "test.mg",
		Content:     []byte(source),
		ImportAlias: map[string]string{},
	}
	tokens, err := tokenizer.Tokenize(fctx, fctx.Content)
	if err != nil {
		tt.Fatalf("tokenize: %v", err)
	}
	fctx.Tokens = tokens
	shared := &mt.SharedState{ExportedSymbols: map[string]string{}}
	return Parse(shared, fctx)
}

func TestParseCharacterizesDeclarationsAndGenerics(t *testing.T) {
	global, err := parseTestSource(t, `mod main
Point[T](value T)
identity[T](value T) T:
    ret value
..
main() void:
    value u64 = identity[u64](9)
..
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(global.Declarations) != 3 {
		t.Fatalf("declarations = %d, want 3", len(global.Declarations))
	}
	point, ok := global.StructDefs["Point"]
	if !ok || len(point.TypeParams) != 1 || point.TypeParams[0] != "T" {
		t.Fatalf("Point generic parameters = %#v, want [T]", point)
	}
	pointDecl, ok := global.Declarations[0].(*mt.NodeStructDef)
	expectedPointSymbol := point.Module + "." + point.Name
	if !ok || pointDecl.AbsName != expectedPointSymbol {
		t.Fatalf("Point declaration symbol = %#v, want %s", pointDecl, expectedPointSymbol)
	}
	identity, ok := global.FuncDefs["identity"]
	if !ok {
		t.Fatal("identity function missing")
	}
	if got := identity.Class.TypeParams; len(got) != 1 || got[0] != "T" {
		t.Fatalf("identity generic parameters = %#v, want [T]", got)
	}
	if len(identity.Body.Statements) != 1 {
		t.Fatalf("identity statements = %d, want 1", len(identity.Body.Statements))
	}
}

func TestGenericDeclarationValidation(t *testing.T) {
	tests := map[string]struct {
		source string
		want   string
	}{
		"duplicate parameter": {
			source: "mod main\nPair[T, T](left T, right T)\n",
			want:   "duplicate generic type parameter 'T'",
		},
		"member owner mismatch": {
			source: "mod main\nBox[T](value T)\nBox[U].get() U:\n    ret this.value\n..\n",
			want:   "generic parameters on member owner 'Box' do not match",
		},
		"member shadows owner parameter": {
			source: "mod main\nBox[T](value T)\nBox[T].map[T](value T) T:\n    ret value\n..\n",
			want:   "generic member parameter 'T' duplicates an owner parameter",
		},
		"generic primitive owner": {
			source: "mod main\nu64[T].invalid() void:\n..\n",
			want:   "primitive owner 'u64' does not take generic parameters",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := parseTestSource(t, test.source)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want it to contain %q", err, test.want)
			}
		})
	}
}

func TestNoCtxFunctionAndFunctionType(t *testing.T) {
	global, err := parseTestSource(t, `mod main
noctx bootstrap(callback noctx (u64) void) void:
..
ordinary(callback (u64) void) void:
..
`)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap := global.FuncDefs["bootstrap"]
	if bootstrap == nil || bootstrap.ContextABI != mt.ContextABIContextless {
		t.Fatalf("bootstrap ABI = %#v", bootstrap)
	}
	callback := bootstrap.Class.ArgsNode.Args[0].TypeNode.KindNode.(*mt.NodeTypeFunc)
	if callback.ContextABI != mt.ContextABIContextless {
		t.Fatalf("callback ABI = %v", callback.ContextABI)
	}
	ordinary := global.FuncDefs["ordinary"]
	if ordinary == nil || ordinary.ContextABI != mt.ContextABIContextful {
		t.Fatalf("ordinary ABI = %#v", ordinary)
	}
	ordinaryCallback := ordinary.Class.ArgsNode.Args[0].TypeNode.KindNode.(*mt.NodeTypeFunc)
	if ordinaryCallback.ContextABI != mt.ContextABIContextful {
		t.Fatalf("ordinary callback ABI = %v", ordinaryCallback.ContextABI)
	}
}

func TestNoCtxRequiresFunctionType(t *testing.T) {
	_, err := parseTestSource(t, "mod main\nvalue noctx u64\n")
	if err == nil || !strings.Contains(err.Error(), "noctx' requires a function type") {
		t.Fatalf("error = %v", err)
	}
}

func TestNoCtxRejectsNonFunctionDeclarations(t *testing.T) {
	tests := map[string]string{
		"struct":            "noctx Value(field u64)\n",
		"global":            "noctx value u64\n",
		"function variable": "noctx callback (u64) u64\n",
		"inferred global":   "noctx value := 1\n",
		"alias":             "noctx alias Value = u64\n",
		"constant":          "noctx const value u64 = 1\n",
		"prototype":         "noctx proto Value(run() void)\n",
	}
	for name, declaration := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := parseTestSource(t, "mod main\n"+declaration)
			if err == nil || !strings.Contains(err.Error(), "noctx modifier cannot be applied") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestNoCtxPrototypeMethodCarriesABI(t *testing.T) {
	global, err := parseTestSource(t, "mod main\nproto Bootstrap(noctx start() void)\n")
	if err != nil {
		t.Fatal(err)
	}
	proto := global.ProtoDefs["Bootstrap"]
	if proto == nil || len(proto.Methods) != 1 || proto.Methods[0].ContextABI != mt.ContextABIContextless {
		t.Fatalf("prototype = %#v", proto)
	}
	if proto.Methods[0].FnDef == nil || proto.Methods[0].FnDef.ContextABI != mt.ContextABIContextless {
		t.Fatalf("wrapper = %#v", proto.Methods[0].FnDef)
	}
}

func TestGenericMemberMayDeclareDistinctParameters(t *testing.T) {
	_, err := parseTestSource(t, `mod main
Box[T](value T)
Box[T].replace[U](value U) T:
    ret this.value
..
`)
	if err != nil {
		t.Fatalf("valid generic member rejected: %v", err)
	}
}

func TestParseCharacterizesConditionalChain(t *testing.T) {
	global, err := parseTestSource(t, `mod main
main() void:
    if false:
    elif true:
    else:
    ..
..
`)
	if err != nil {
		t.Fatal(err)
	}
	mainFn := global.FuncDefs["main"]
	if mainFn == nil || len(mainFn.Body.Statements) != 1 {
		t.Fatalf("main body = %#v, want one statement", mainFn)
	}
	ifStmt, ok := mainFn.Body.Statements[0].(*mt.NodeStmtIf)
	if !ok {
		t.Fatalf("statement type = %T, want *NodeStmtIf", mainFn.Body.Statements[0])
	}
	elif, ok := ifStmt.NextCondStmt.(*mt.NodeStmtIf)
	if !ok {
		t.Fatalf("elif type = %T, want *NodeStmtIf", ifStmt.NextCondStmt)
	}
	if _, ok := elif.NextCondStmt.(*mt.NodeStmtElse); !ok {
		t.Fatalf("else type = %T, want *NodeStmtElse", elif.NextCondStmt)
	}
}

func TestParseCharacterizesForLoop(t *testing.T) {
	global, err := parseTestSource(t, `mod main
main() void:
    for i u64 = 1 to 10:
        continue
    ..
..
`)
	if err != nil {
		t.Fatal(err)
	}
	mainFn := global.FuncDefs["main"]
	if mainFn == nil || len(mainFn.Body.Statements) != 1 {
		t.Fatalf("main body = %#v, want one statement", mainFn)
	}
	loop, ok := mainFn.Body.Statements[0].(*mt.NodeStmtFor)
	if !ok {
		t.Fatalf("statement = %T, want *NodeStmtFor", mainFn.Body.Statements[0])
	}
	if _, ok := loop.DeclExpr.(*mt.NodeExprVarDefAssign); !ok {
		t.Fatalf("index declaration = %T, want initialized variable", loop.DeclExpr)
	}
	if len(loop.Body.Statements) != 1 {
		t.Fatalf("loop body statements = %d, want 1", len(loop.Body.Statements))
	}
}

func TestMoveIsContextualAndPreserved(t *testing.T) {
	global, err := parseTestSource(t, `mod main
consume(value $str) void:
..
move(value str) void:
..
main() void:
    value $str = "owned"
    consume(move value)
    move("ordinary call")
..
`)
	if err != nil {
		t.Fatal(err)
	}
	mainFn := global.FuncDefs["main"]
	consumeStmt := mainFn.Body.Statements[1].(*mt.NodeStmtExpr)
	consumeCall := consumeStmt.Expression.(*mt.NodeExprCall)
	if _, ok := consumeCall.Args[0].(*mt.NodeExprMove); !ok {
		t.Fatalf("consume argument = %T, want move expression", consumeCall.Args[0])
	}
	ordinaryStmt := mainFn.Body.Statements[2].(*mt.NodeStmtExpr)
	if _, ok := ordinaryStmt.Expression.(*mt.NodeExprCall); !ok {
		t.Fatalf("ordinary move call = %T, want call expression", ordinaryStmt.Expression)
	}
}

func TestForLoopSyntaxErrors(t *testing.T) {
	tests := map[string]struct {
		body string
		want string
	}{
		"missing declaration": {body: "for 0 to 10:\n    ..", want: "expected index variable declaration"},
		"missing to":          {body: "for i := 0 10:\n    ..", want: "expected 'to' keyword"},
		"missing colon":       {body: "for i := 0 to 10\n    ..", want: "expected body opening ':'"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := parseTestSource(t, "mod main\nmain() void:\n    "+test.body+"\n..\n")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want it to contain %q", err, test.want)
			}
		})
	}
}

func TestParseCharacterizesPrematureEOF(t *testing.T) {
	_, err := parseTestSource(t, "mod main\nmain() void:\n")
	if err == nil || !strings.Contains(err.Error(), "reached end of file prematurely") {
		t.Fatalf("error = %v, want premature EOF diagnostic", err)
	}
}
