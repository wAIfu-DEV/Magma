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

	_, err := os.Stat(joined)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	if !os.IsNotExist(err) {
		return joined, nil
	}

	absPath, err := filepath.Abs(relative)
	if err != nil {
		return "", err
	}

	_, err = os.Stat(absPath)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	if os.IsExist(err) {
		return absPath, nil
	}

	return "", fmt.Errorf("file '%s' does not exist", relative)
}
