package tokenizer

import (
	"Magma/src/comp_err"
	"Magma/src/types"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

type tkMode uint8

const (
	tkModeNormal tkMode = iota
	tkModeString
	tkModeComment
)

type TkCtx struct {
	fCtx *types.FileCtx

	Content []byte

	CurrTok types.Token
	TokReprBuff []rune

	Tokens []types.Token

	Pos types.FilePos

	Idx int
	Mode tkMode
	IsEscaped bool
}

func hasData(ctx *TkCtx) bool {
	_, err := peek(ctx)
	if err != nil {
		return false
	}
	return true
}

func decodeFirst(ctx *TkCtx) (rune, int, error) {
	r, size := utf8.DecodeRune(ctx.Content[ctx.Idx:])

	if r == utf8.RuneError {
		return r, size, fmt.Errorf("decode failure")
	} else if size == 0 {
		return r, size, fmt.Errorf("decoded size 0")
	}
	return r, size, nil
}

func decodeFrom(ctx *TkCtx, from int) (rune, int, error) {
	if from >= len(ctx.Content) {
		return 0, 0, fmt.Errorf("out of bounds decode")
	}

	r, size := utf8.DecodeRune(ctx.Content[from:])

	if r == utf8.RuneError {
		return r, size, fmt.Errorf("decode failure")
	} else if size == 0 {
		return r, size, fmt.Errorf("decoded size 0")
	}
	return r, size, nil
}

func peek(ctx *TkCtx) (rune, error) {
	r, _, err := decodeFirst(ctx)
	return r, err
}

func peekMany(ctx *TkCtx, n int) ([]rune, error) {
	runes := make([]rune, n)
	offset := 0
	for i := range n {
		r, size, err := decodeFrom(ctx, ctx.Idx + offset)
		if err != nil {
			return nil, err
		}
		offset += size
		runes[i] = r
	}
	return runes, nil
}

func consumeSize(ctx *TkCtx, size int) {
	ctx.Idx += size
}

func consume(ctx *TkCtx) (rune, error) {
	r, size, err := decodeFirst(ctx)
    consumeSize(ctx, size)
	return r, err
}

func toggleMode(ctx *TkCtx, mode tkMode) {
	if ctx.Mode == mode {
		ctx.Mode = tkModeNormal
	} else {
		ctx.Mode = mode
	}
}

func pushToken(ctx *TkCtx, tk types.Token) {
	ctx.Tokens = append(ctx.Tokens, tk)
}

func clearTokenBuff(ctx *TkCtx) {
	ctx.CurrTok = types.Token{
		Type: types.TokName,
		Pos: ctx.Pos,
	}
	ctx.CurrTok.Pos.Col++

	ctx.TokReprBuff = make([]rune, 0, 16)
}

func pushTokenAndClearBuff(ctx *TkCtx, tk types.Token) {
	ctx.Tokens = append(ctx.Tokens, tk)
	clearTokenBuff(ctx)
}

func pushTokenBuff(ctx *TkCtx) {
	if len(ctx.TokReprBuff) == 0 {
		return
	}

	ctx.CurrTok.Repr = string(ctx.TokReprBuff)

	kwType, ok := types.KwReprToType[ctx.CurrTok.Repr]
	if ok {
		ctx.CurrTok.Type = types.TokKeyword
		ctx.CurrTok.KeywType = kwType
	}

	pushToken(ctx, ctx.CurrTok)
	clearTokenBuff(ctx)
}

func handleNonAlphaKeyword(ctx *TkCtx, first rune) (types.Token, int, error) {
	bestMatch := ""
	bestSize := 0
	var bestKwType types.KwType = types.KwNone

	for i, kw := range types.KwTypeToRepr {
		if i == 0 { continue }

		kwLen := len(kw)
		kwRuneCnt := utf8.RuneCountInString(kw)

		peeked, err := peekMany(ctx, kwRuneCnt)
		if err != nil {
			continue
		}

		peekedStr := string(peeked)
		if strings.HasPrefix(peekedStr, kw) && kwLen > bestSize {
			bestMatch = kw
			bestSize = kwLen
			bestKwType = types.KwType(i)
		}
	}

	if bestMatch == "" {
		return types.Token{}, 0,
		comp_err.CompilationErrorToken(
			ctx.fCtx,
			&ctx.CurrTok,
			fmt.Sprintf("'%c' is not a valid keyword", first),
			fmt.Sprintf("non alphanumeric character '%c' was not recognized as a valid keyword", first),
		)
	}

	return types.Token{
		Repr: bestMatch,
		Pos: ctx.Pos,
		Type: types.TokKeyword,
		KeywType: bestKwType,
	}, bestSize, nil
}

func Tokenize(fCtx *types.FileCtx, bytes []byte) ([]types.Token, error) {
	ctx := &TkCtx{
		fCtx: fCtx,
		Content: bytes,

		CurrTok: types.Token{
			Type: types.TokName,
			Pos: types.FilePos{
			 	Line: 1,
				Col: 0,
		 	},
		},
		TokReprBuff: make([]rune, 0, 16),
		Tokens: make([]types.Token, 0, 256),

		Pos: types.FilePos{
			Line: 1,
			Col: 0,
		},

		Idx: 0,
		Mode: tkModeNormal,
		IsEscaped: false,
	}

	for hasData(ctx) {
		r, err := peek(ctx)
		if err != nil {
			return nil, err
		}

		ctx.Pos.Col++

		if r == '\n' && ctx.Mode == tkModeComment {
			ctx.Pos.Line++
			ctx.Pos.Col = 0

			ctx.Mode = tkModeNormal
			consume(ctx)
			continue
		}

		if ctx.Mode == tkModeComment {
			// drop rune
			consume(ctx)
			continue
		}

		if r == '#' && ctx.Mode == tkModeNormal {
			if ctx.Mode == tkModeNormal {
				pushTokenBuff(ctx)
			}
			toggleMode(ctx, tkModeComment)

			consume(ctx)
			continue
		}

		if r == '"' && !ctx.IsEscaped {
			pushTokenBuff(ctx)
			toggleMode(ctx, tkModeString)

			if ctx.Mode == tkModeString {
				ctx.CurrTok.Type = types.TokLitStr
			}
			consume(ctx)
			continue
		}

		ctx.IsEscaped = false

		if r == '\\' && ctx.Mode == tkModeString {
			ctx.IsEscaped = true
			consume(ctx)
			continue
		}

		if unicode.IsSpace(r) {
			pushTokenBuff(ctx)

			if r == '\n' {
				pushToken(ctx, types.Token{
					Repr: "\n",
					Pos: ctx.Pos,
					Type: types.TokKeyword,
					KeywType: types.KwNewline,
				})

				ctx.Pos.Line++
				ctx.Pos.Col = 0
			}

			consume(ctx)
			continue
		}

		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			pushTokenBuff(ctx)

			tk, size, err := handleNonAlphaKeyword(ctx, r)
			if err != nil {
				return nil, err
			}
			pushTokenAndClearBuff(ctx, tk)
			consumeSize(ctx, size)
			continue
		}

		if unicode.IsDigit(r) && len(ctx.TokReprBuff) == 0 {
			ctx.CurrTok.Type = types.TokLitNum
		}

		if ctx.CurrTok.Type == types.TokLitNum {
			if unicode.IsDigit(r) || r == '.' {
				consume(ctx)
				continue
			}
			return nil, comp_err.CompilationErrorToken(
				ctx.fCtx,
				 &ctx.CurrTok,
				fmt.Sprintf("invalid character '%c' in number literal", r),
				"valid characters in number literal are: [0-9.]",
			)
		}

		if len(ctx.TokReprBuff) == 0 {
			ctx.CurrTok.Pos = ctx.Pos
		}

		ctx.TokReprBuff = append(ctx.TokReprBuff, r)
		consume(ctx)
		continue
	}

	if len(ctx.TokReprBuff) > 0 {
		pushTokenBuff(ctx)
	}
	return ctx.Tokens, nil
}

func PrintTokens(toks []types.Token) {
	fmt.Printf("Tokens:\n")
	for _, t := range toks {
		if t.KeywType == types.KwNewline {
			fmt.Printf("Newline,\n")
			continue
		}
		fmt.Printf("%s<%s>(l%d,c%d), ",
		 types.TokTypeToRepr[t.Type],
			t.Repr,
			t.Pos.Line,
			t.Pos.Col,
		)
	}
}
