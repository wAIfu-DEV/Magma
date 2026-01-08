package types

type TokType uint8

const (
	TokNone TokType = iota
	TokName
	TokLitStr
	TokLitNum
	TokLitBool
	TokKeyword
)

var TokTypeToRepr []string = []string{
	TokNone:    "None",
	TokName:    "Name",
	TokLitStr:  "LitStr",
	TokLitNum:  "LitNum",
	TokLitBool: "LitBool",
	TokKeyword: "Keyword",
}

type KwType uint8

const (
	KwNone KwType = iota
	KwEqual
	KwParenOp
	KwParenCl
	KwBrackOp
	KwBrackCl
	KwColon
	KwDot
	KwDots
	KwExclam
	KwModule
	KwUse
	KwPublic
	KwReturn
	KwNewline
	KwComma
	KwMinus
	KwPlus
	KwDollar
	KwAsterisk
	KwAmpersand
	KwThrow
	KwLlvm
	KwIf
	KwElif
	KwElse
	KwTrue
	KwFalse
	KwWhile
	KwCmpEq
	KwCmpNeq
	KwTry
	KwDefer
	KwCmpLt
	KwCmpGt
	KwCmpLtEq
	KwCmpGtEq
	KwPipe
	KwCaret
	KwTilde
	KwShiftLeft
	KwShiftRight
	KwAndAnd
	KwOrOr
	KwSizeof
	KwContinue
	KwBreak
)

var KwTypeToRepr []string = []string{
	KwNone:       "",
	KwEqual:      "=",
	KwParenOp:    "(",
	KwParenCl:    ")",
	KwBrackOp:    "[",
	KwBrackCl:    "]",
	KwColon:      ":",
	KwDot:        ".",
	KwDots:       "..",
	KwExclam:     "!",
	KwModule:     "mod",
	KwUse:        "use",
	KwPublic:     "pub",
	KwReturn:     "ret",
	KwNewline:    "\n",
	KwComma:      ",",
	KwMinus:      "-",
	KwPlus:       "+",
	KwDollar:     "$",
	KwAsterisk:   "*",
	KwAmpersand:  "&",
	KwThrow:      "throw",
	KwLlvm:       "llvm",
	KwIf:         "if",
	KwElif:       "elif",
	KwElse:       "else",
	KwTrue:       "true",
	KwFalse:      "false",
	KwWhile:      "while",
	KwCmpEq:      "==",
	KwCmpNeq:     "!=",
	KwTry:        "try",
	KwDefer:      "defer",
	KwCmpLt:      "<",
	KwCmpGt:      ">",
	KwCmpLtEq:    "<=",
	KwCmpGtEq:    ">=",
	KwPipe:       "|",
	KwCaret:      "^",
	KwTilde:      "~",
	KwShiftLeft:  "<<",
	KwShiftRight: ">>",
	KwAndAnd:     "&&",
	KwOrOr:       "||",
	KwSizeof:     "sizeof",
	KwContinue:   "continue",
	KwBreak:      "break",
}

var KwReprToType map[string]KwType = map[string]KwType{
	"":         KwNone,
	"=":        KwEqual,
	"(":        KwParenOp,
	")":        KwParenCl,
	"[":        KwBrackOp,
	"]":        KwBrackCl,
	":":        KwColon,
	".":        KwDot,
	"..":       KwDots,
	"!":        KwExclam,
	"mod":      KwModule,
	"use":      KwUse,
	"pub":      KwPublic,
	"ret":      KwReturn,
	"\n":       KwNewline,
	",":        KwComma,
	"-":        KwMinus,
	"+":        KwPlus,
	"$":        KwDollar,
	"*":        KwAsterisk,
	"&":        KwAmpersand,
	"throw":    KwThrow,
	"llvm":     KwLlvm,
	"if":       KwIf,
	"elif":     KwElif,
	"else":     KwElse,
	"true":     KwTrue,
	"false":    KwFalse,
	"while":    KwWhile,
	"==":       KwCmpEq,
	"!=":       KwCmpNeq,
	"try":      KwTry,
	"defer":    KwDefer,
	"<":        KwCmpLt,
	">":        KwCmpGt,
	"<=":       KwCmpLtEq,
	">=":       KwCmpGtEq,
	"|":        KwPipe,
	"^":        KwCaret,
	"~":        KwTilde,
	"<<":       KwShiftLeft,
	">>":       KwShiftRight,
	"&&":       KwAndAnd,
	"||":       KwOrOr,
	"sizeof":   KwSizeof,
	"continue": KwContinue,
	"break":    KwBreak,
}

type Token struct {
	Repr     string
	Pos      FilePos
	Type     TokType
	KeywType KwType
}

type FilePos struct {
	Line uint32
	Col  uint32
}
