package tokenizer

import (
	"Magma/src/comp_err"
	t "Magma/src/types"
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
	tkModeNumber
	tkModeHexNum
)

type TkCtx struct {
	fCtx *t.FileCtx

	Content []byte

	CurrTok     t.Token
	TokReprBuff []rune

	Tokens []t.Token

	Pos t.FilePos

	Idx       int
	Mode      tkMode
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
		r, size, err := decodeFrom(ctx, ctx.Idx+offset)
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

func pushToken(ctx *TkCtx, tk t.Token) {
	ctx.Tokens = append(ctx.Tokens, tk)
}

func clearTokenBuff(ctx *TkCtx) {
	ctx.CurrTok = t.Token{
		Type: t.TokName,
		Pos:  ctx.Pos,
	}
	ctx.CurrTok.Pos.Col++

	ctx.TokReprBuff = make([]rune, 0, 16)
}

func pushTokenAndClearBuff(ctx *TkCtx, tk t.Token) {
	ctx.Tokens = append(ctx.Tokens, tk)
	clearTokenBuff(ctx)
}

func pushTokenBuff(ctx *TkCtx) {
	if len(ctx.TokReprBuff) == 0 && ctx.CurrTok.Type != t.TokLitStr {
		return
	}

	ctx.CurrTok.Repr = string(ctx.TokReprBuff)

	if ctx.CurrTok.Type == t.TokNone || ctx.CurrTok.Type == t.TokName {
		kwType, ok := t.KwReprToType[ctx.CurrTok.Repr]
		if ok {
			ctx.CurrTok.Type = t.TokKeyword
			ctx.CurrTok.KeywType = kwType
		}
	}

	pushToken(ctx, ctx.CurrTok)
	clearTokenBuff(ctx)
}

func handleNonAlphaKeyword(ctx *TkCtx, first rune) (t.Token, int, error) {
	bestMatch := ""
	bestSize := 0
	var bestKwType t.KwType = t.KwNone

	for i, kw := range t.KwTypeToRepr {
		if i == 0 {
			continue
		}

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
			bestKwType = t.KwType(i)
		}
	}

	if bestMatch == "" {
		return t.Token{}, 0,
			comp_err.CompilationErrorToken(
				ctx.fCtx,
				&t.Token{Repr: string(first), Pos: ctx.Pos},
				fmt.Sprintf("'%c' is not a valid keyword", first),
				fmt.Sprintf("non alphanumeric character '%c' was not recognized as a valid keyword", first),
			)
	}

	return t.Token{
		Repr:     bestMatch,
		Pos:      ctx.Pos,
		Type:     t.TokKeyword,
		KeywType: bestKwType,
	}, bestSize, nil
}

func Tokenize(fCtx *t.FileCtx, bytes []byte) ([]t.Token, error) {
	ctx := &TkCtx{
		fCtx:    fCtx,
		Content: bytes,

		CurrTok: t.Token{
			Type: t.TokName,
			Pos: t.FilePos{
				Line: 1,
				Col:  0,
			},
		},
		TokReprBuff: make([]rune, 0, 16),
		Tokens:      make([]t.Token, 0, 256),

		Pos: t.FilePos{
			Line: 1,
			Col:  0,
		},

		Idx:       0,
		Mode:      tkModeNormal,
		IsEscaped: false,
	}

	for hasData(ctx) {
		r, err := peek(ctx)
		if err != nil {
			return nil, err
		}

		ctx.Pos.Col++

		if r == '\n' && ctx.Mode == tkModeComment {
			ctx.Mode = tkModeNormal

			pushToken(ctx, t.Token{
				Repr:     "\n",
				Pos:      ctx.Pos,
				Type:     t.TokKeyword,
				KeywType: t.KwNewline,
			})

			ctx.Pos.Line++
			ctx.Pos.Col = 0
			consume(ctx)
			continue
		}

		if ctx.Mode == tkModeComment {
			// drop rune
			consume(ctx)
			continue
		}

		if (ctx.Mode == tkModeNormal || ctx.Mode == tkModeNumber || ctx.Mode == tkModeHexNum) && r == '#' {
			pushTokenBuff(ctx)
			toggleMode(ctx, tkModeComment)

			consume(ctx)
			continue
		}

		if r == '"' && !ctx.IsEscaped {
			pushTokenBuff(ctx)
			toggleMode(ctx, tkModeString)

			if ctx.Mode == tkModeString {
				ctx.CurrTok.Type = t.TokLitStr
			}
			consume(ctx)
			continue
		}

		if ctx.Mode == tkModeString && ctx.IsEscaped {
			switch r {
			case 'a':
				r = '\a'
			case 'b':
				r = '\b'
			case 'f':
				r = '\f'
			case 'n':
				r = '\n'
			case 'r':
				r = '\r'
			case 't':
				r = '\t'
			case 'v':
				r = '\v'
			case '\\':
				r = '\\'
			case '\'':
				r = '\''
			case '"':
				r = '"'
			}
		}

		if ctx.Mode == tkModeString && r == '\\' && !ctx.IsEscaped {
			ctx.IsEscaped = true
			consume(ctx)
			continue
		}

		ctx.IsEscaped = false

		if (ctx.Mode == tkModeNormal || ctx.Mode == tkModeNumber || ctx.Mode == tkModeHexNum) && unicode.IsSpace(r) {
			pushTokenBuff(ctx)

			if r == '\n' {
				pushToken(ctx, t.Token{
					Repr:     "\n",
					Pos:      ctx.Pos,
					Type:     t.TokKeyword,
					KeywType: t.KwNewline,
				})

				ctx.Pos.Line++
				ctx.Pos.Col = 0
			}

			ctx.Mode = tkModeNormal
			consume(ctx)
			continue
		}

		if ctx.Mode == tkModeNormal && (unicode.IsDigit(r) || r == '-') && len(ctx.TokReprBuff) == 0 {
			prev := ctx.CurrTok.Type
			ctx.CurrTok.Type = t.TokLitNum

			ctx.Mode = tkModeNumber

			if r == '-' {
				r2, err := peekMany(ctx, 2)
				if err != nil {
					return nil, err
				}
				if !unicode.IsDigit(r2[1]) {
					ctx.CurrTok.Type = prev
				} else {
					goto write_num
				}
			}

			if r == '0' {
				runes, err := peekMany(ctx, 2)
				if err != nil {
					return nil, err
				}
				if runes[1] == 'x' || runes[1] == 'X' {
					ctx.TokReprBuff = append(ctx.TokReprBuff, 'u') // prefix for LLVM
					ctx.Mode = tkModeHexNum
				} else {
					goto write_num
				}
			}
		}

		if (ctx.Mode == tkModeNormal || ctx.Mode == tkModeNumber || ctx.Mode == tkModeHexNum) && !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			pushTokenBuff(ctx)

			tk, size, err := handleNonAlphaKeyword(ctx, r)
			if err != nil {
				return nil, err
			}
			pushTokenAndClearBuff(ctx, tk)
			consumeSize(ctx, size)
			continue
		}

	write_num:
		if ctx.Mode == tkModeNumber {
			if !(unicode.IsDigit(r) || r == '.' || r == '-') {
				ctx.TokReprBuff = append(ctx.TokReprBuff, r)
				return nil, comp_err.CompilationErrorToken(
					ctx.fCtx,
					&t.Token{Repr: string(ctx.TokReprBuff), Pos: ctx.CurrTok.Pos},
					fmt.Sprintf("invalid character '%c' in number literal", r),
					"valid characters in number literal are: [0-9.]",
				)
			}
		}

		if ctx.Mode == tkModeHexNum {
			if !(unicode.IsDigit(r) || unicode.IsLetter(r)) {
				ctx.TokReprBuff = append(ctx.TokReprBuff, r)
				return nil, comp_err.CompilationErrorToken(
					ctx.fCtx,
					&t.Token{Repr: string(ctx.TokReprBuff), Pos: ctx.CurrTok.Pos},
					fmt.Sprintf("invalid character '%c' in hex number literal", r),
					"valid characters in hex number literal are: [0-9a-zA-Z]",
				)
			}
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

func PrintTokens(toks []t.Token) {
	fmt.Printf("Tokens:\n")
	for _, tk := range toks {
		if tk.KeywType == t.KwNewline {
			fmt.Printf("Newline,\n")
			continue
		}
		fmt.Printf("%s<%s>(l%d,c%d), ",
			t.TokTypeToRepr[tk.Type],
			tk.Repr,
			tk.Pos.Line,
			tk.Pos.Col,
		)
	}
}
