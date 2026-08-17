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
