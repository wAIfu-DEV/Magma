package tokenizer

import (
	"Magma/src/comp_err"
	"Magma/src/types"
	"bytes"
	"strings"
	"testing"
)

func TestInvalidUtf8ProducesSourceDiagnostic(t *testing.T) {
	ctx := &types.FileCtx{FilePath: "invalid.mg", Content: []byte{'m', 'o', 'd', ' ', 0xff}}
	_, err := Tokenize(ctx, ctx.Content)
	if err == nil {
		t.Fatal("invalid UTF-8 was silently accepted")
	}
	diagnostics := comp_err.Diagnostics(err)
	if len(diagnostics) != 1 || diagnostics[0].FilePath != ctx.FilePath || diagnostics[0].Token.Pos.Line != 1 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	var output bytes.Buffer
	comp_err.Fprint(&output, err)
	if rendered := output.String(); strings.Contains(rendered, "fatal error") || !strings.Contains(rendered, "invalid.mg:l1:") {
		t.Fatalf("opaque tokenizer error:\n%s", rendered)
	}
}

func TestSingleTrailingNumberDoesNotBecomeDecodeFailure(t *testing.T) {
	ctx := &types.FileCtx{FilePath: "number.mg", Content: []byte("0")}
	tokens, err := Tokenize(ctx, ctx.Content)
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 1 || tokens[0].Repr != "0" || tokens[0].Type != types.TokLitNum {
		t.Fatalf("tokens = %#v", tokens)
	}
}

func TestNotIsKeyword(t *testing.T) {
	ctx := &types.FileCtx{FilePath: "not.mg", Content: []byte("not value")}
	tokens, err := Tokenize(ctx, ctx.Content)
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 2 || tokens[0].Type != types.TokKeyword || tokens[0].KeywType != types.KwNot {
		t.Fatalf("tokens = %#v", tokens)
	}
}

func TestJSONBracesAreTokens(t *testing.T) {
	ctx := &types.FileCtx{FilePath: "json.mg", Content: []byte(`json {field: "value"}`)}
	tokens, err := Tokenize(ctx, ctx.Content)
	if err != nil {
		t.Fatal(err)
	}
	foundOpen, foundClose := false, false
	for _, token := range tokens {
		foundOpen = foundOpen || token.KeywType == types.KwBraceOp
		foundClose = foundClose || token.KeywType == types.KwBraceCl
	}
	if !foundOpen || !foundClose {
		t.Fatalf("brace tokens missing: %#v", tokens)
	}
}
