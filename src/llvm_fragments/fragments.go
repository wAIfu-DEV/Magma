package llvmfragments

import (
	_ "embed"
)

//go:embed utils.ll
var Utils []byte
