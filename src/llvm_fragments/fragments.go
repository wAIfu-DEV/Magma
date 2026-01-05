package llvmfragments

import (
	_ "embed"
)

//go:embed utils.ll
var Utils []byte

//go:embed utf8.ll
var Utf8 []byte
