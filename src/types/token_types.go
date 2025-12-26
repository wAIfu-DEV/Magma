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
)

var KwTypeToRepr []string = []string{
	KwNone:      "",
	KwEqual:     "=",
	KwParenOp:   "(",
	KwParenCl:   ")",
	KwColon:     ":",
	KwDot:       ".",
	KwDots:      "..",
	KwExclam:    "!",
	KwModule:    "mod",
	KwUse:       "use",
	KwPublic:    "pub",
	KwReturn:    "ret",
	KwNewline:   "\n",
	KwComma:     ",",
	KwMinus:     "-",
	KwPlus:      "+",
	KwDollar:    "$",
	KwAsterisk:  "*",
	KwAmpersand: "&",
	KwThrow:     "throw",
	KwLlvm:      "llvm",
	KwIf:        "if",
	KwElif:      "elif",
	KwElse:      "else",
	KwTrue:      "true",
	KwFalse:     "false",
}

var KwReprToType map[string]KwType = map[string]KwType{
	"":      KwNone,
	"=":     KwEqual,
	"(":     KwParenOp,
	")":     KwParenCl,
	":":     KwColon,
	".":     KwDot,
	"..":    KwDots,
	"!":     KwExclam,
	"mod":   KwModule,
	"use":   KwUse,
	"pub":   KwPublic,
	"ret":   KwReturn,
	"\n":    KwNewline,
	",":     KwComma,
	"-":     KwMinus,
	"+":     KwPlus,
	"$":     KwDollar,
	"*":     KwAsterisk,
	"&":     KwAmpersand,
	"throw": KwThrow,
	"llvm":  KwLlvm,
	"if":    KwIf,
	"elif":  KwElif,
	"else":  KwElse,
	"true":  KwTrue,
	"false": KwFalse,
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
