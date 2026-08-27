package lsp

import (
	"strings"
	"testing"

	"Magma/src/types"
)

func TestEveryWordKeywordHasHoverDocumentation(t *testing.T) {
	for kind, repr := range types.KwTypeToRepr {
		if repr == "" || strings.IndexFunc(repr, func(r rune) bool { return r < 'a' || r > 'z' }) >= 0 {
			continue
		}
		if keywordHovers[repr] == "" {
			t.Errorf("keyword %q (kind %d) has no hover documentation", repr, kind)
		}
	}
}

func TestContextualKeywordHover(t *testing.T) {
	name := func(value string) types.Token { return types.Token{Type: types.TokName, Repr: value} }
	keyword := func(kind types.KwType) types.Token {
		return types.Token{Type: types.TokKeyword, KeywType: kind, Repr: types.KwTypeToRepr[kind]}
	}

	cases := []struct {
		name   string
		tokens []types.Token
		index  int
		want   bool
	}{
		{"array expression", []types.Token{name("array"), name("u8"), keyword(types.KwBrackOp), {Type: types.TokLitNum, Repr: "8"}, keyword(types.KwBrackCl)}, 0, true},
		{"array function", []types.Token{name("array"), keyword(types.KwParenOp), keyword(types.KwParenCl)}, 0, false},
		{"move transfer", []types.Token{name("move"), name("value")}, 0, true},
		{"move call", []types.Token{name("move"), keyword(types.KwParenOp)}, 0, false},
		{"prototype declaration", []types.Token{name("proto"), name("Writer"), keyword(types.KwParenOp)}, 0, true},
		{"proto call", []types.Token{name("proto"), keyword(types.KwParenOp)}, 0, false},
		{"implementation clause", []types.Token{name("File"), name("impl"), name("Reader")}, 1, true},
		{"impl call", []types.Token{name("impl"), keyword(types.KwParenOp)}, 0, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			a := &analysis{file: &types.FileCtx{Tokens: test.tokens}}
			if got := a.keywordHover(test.index) != ""; got != test.want {
				t.Fatalf("keyword hover present = %v, want %v", got, test.want)
			}
		})
	}
}

func TestDirectiveKeywordHoverRequiresAtSign(t *testing.T) {
	a := &analysis{file: &types.FileCtx{Tokens: []types.Token{{Type: types.TokKeyword, KeywType: types.KwAt, Repr: "@"}, {Type: types.TokName, Repr: "platform"}}}}
	if got := a.keywordHover(1); !strings.Contains(got, "target operating systems") {
		t.Fatalf("directive hover = %q", got)
	}
}
