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

	fmt.Printf("imported from: %s\n", importedFromAbs)

	fromDir := filepath.Dir(importedFromAbs)

	fmt.Printf("from dir: %s\n", fromDir)

	joined := filepath.Join(fromDir, relative)

	fmt.Printf("joined: %s\n", joined)

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
