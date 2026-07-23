package makeabs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func MakeAbs(relative string, importedFromAbs string) (string, error) {
	candidates := []string{}
	if filepath.IsAbs(relative) {
		candidates = append(candidates, filepath.Clean(relative))
	} else {
		candidates = append(candidates, filepath.Join(filepath.Dir(importedFromAbs), relative))
		if absolute, err := filepath.Abs(relative); err == nil {
			candidates = append(candidates, absolute)
		}
	}
	for _, candidate := range candidates {
		for _, path := range withOptionalMagmaExtension(candidate) {
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				absolute, err := filepath.Abs(path)
				if err != nil {
					return "", err
				}
				return filepath.Clean(absolute), nil
			}
		}
	}
	return "", fmt.Errorf("file '%s' does not exist", relative)
}

// ResolveImport resolves normal imports relative to their importing file and
// std: imports relative to the standard library beside the compiler binary.
func ResolveImport(specifier, importedFromAbs, executablePath string) (string, error) {
	if !strings.HasPrefix(specifier, "std:") {
		return MakeAbs(specifier, importedFromAbs)
	}
	name := strings.TrimPrefix(specifier, "std:")
	if name == "" || filepath.IsAbs(name) || filepath.VolumeName(name) != "" {
		return "", fmt.Errorf("invalid standard library import '%s'", specifier)
	}
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("standard library import '%s' escapes the std directory", specifier)
	}
	stdRoot := filepath.Join(filepath.Dir(executablePath), "std")
	for _, candidate := range withOptionalMagmaExtension(filepath.Join(stdRoot, clean)) {
		absolute, err := filepath.Abs(candidate)
		if err != nil {
			return "", err
		}
		relative, err := filepath.Rel(stdRoot, absolute)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("standard library import '%s' escapes the std directory", specifier)
		}
		if info, err := os.Stat(absolute); err == nil && !info.IsDir() {
			return filepath.Clean(absolute), nil
		}
	}
	return "", fmt.Errorf("standard library module '%s' does not exist under '%s'", name, stdRoot)
}

func withOptionalMagmaExtension(path string) []string {
	paths := []string{path}
	if filepath.Ext(path) == "" {
		paths = append(paths, path+".mg")
	}
	return paths
}
