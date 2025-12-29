package magmatypes

import (
	"io"
)

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
