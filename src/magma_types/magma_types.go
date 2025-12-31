package magmatypes

import (
	"io"
)

type NumberType struct {
	ByteSize int
	IsSigned bool
	IsFloat  bool
}

var FloatTypes = map[string]NumberType{
	"f16":  {ByteSize: 16, IsFloat: true},
	"f32":  {ByteSize: 32, IsFloat: true},
	"f64":  {ByteSize: 64, IsFloat: true},
	"f128": {ByteSize: 128, IsFloat: true},
}

var NumberTypes = map[string]NumberType{
	"i8":   {ByteSize: 8, IsSigned: true},
	"u8":   {ByteSize: 8, IsSigned: false},
	"i16":  {ByteSize: 16, IsSigned: true},
	"u16":  {ByteSize: 16, IsSigned: false},
	"i32":  {ByteSize: 32, IsSigned: true},
	"u32":  {ByteSize: 32, IsSigned: false},
	"i64":  {ByteSize: 64, IsSigned: true},
	"u64":  {ByteSize: 64, IsSigned: false},
	"i128": {ByteSize: 128, IsSigned: true},
	"u128": {ByteSize: 128, IsSigned: false},
	"f16":  {ByteSize: 16, IsFloat: true},
	"f32":  {ByteSize: 32, IsFloat: true},
	"f64":  {ByteSize: 64, IsFloat: true},
	"f128": {ByteSize: 128, IsFloat: true},
}

var BasicTypes = map[string]string{
	"void": "void",
	"bool": "i1",

	"i8":   "i8",
	"i16":  "i16",
	"i32":  "i32",
	"i64":  "i64",
	"i128": "i128",

	"u8":   "i8",
	"u16":  "i16",
	"u32":  "i32",
	"u64":  "i64",
	"u128": "i128",

	"f16":  "half",
	"f32":  "float",
	"f64":  "double",
	"f128": "fp128",

	"error": "%type.error",
	"str":   "%type.str",
	"slice": "%type.slice",
}

func WriteIrBasicTypes(b io.StringWriter) {
	b.WriteString("%type.error = type { i32, %type.str }\n")
	b.WriteString("%type.str = type { ptr, i64 }\n")
	b.WriteString("%type.slice = type { ptr, i64 }\n")
}
