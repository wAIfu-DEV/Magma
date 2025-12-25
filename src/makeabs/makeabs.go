package makeabs

import (
	"fmt"
	"os"
	"path/filepath"
)

func MakeAbs(relative string, importedFromAbs string) (string, error) {
	if filepath.IsAbs(relative) {
		return relative, nil
	}

	fromDir := filepath.Dir(importedFromAbs)
	joined := filepath.Join(fromDir, relative)

	s, err := os.Stat(joined)
	if err == nil && !s.IsDir() {
		return joined, nil
	}

	absPath, err := filepath.Abs(relative)
	if err != nil {
		return "", err
	}

	s, err = os.Stat(absPath)
	if err == nil && !s.IsDir() {
		return absPath, nil
	}

	return "", fmt.Errorf("file '%s' does not exist", relative)
}
