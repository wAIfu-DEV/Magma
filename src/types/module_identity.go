package types

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
)

const moduleIdentityVersion = "magma-module-v1"

// ModuleID is the stable identity of one backing source module. Source import
// aliases and declared module names are intentionally not part of it.
type ModuleID string

// DeclarationID identifies a source declaration within its backing module.
// Concrete generic arguments are appended by the specialization layer.
type DeclarationID struct {
	Module ModuleID
	Kind   string
	Name   string
}

func (id DeclarationID) String() string {
	return string(id.Module) + "\x00" + id.Kind + "\x00" + id.Name
}

// ResolveModuleID derives an identity from the backing path. Files below the
// standard-library root and workspace root remain stable when those roots move.
// Files outside both roots retain an explicit external absolute-path identity
// until Magma gains package manifests capable of naming external package roots.
func ResolveModuleID(path, workspaceRoot, stdRoot string) (ModuleID, error) {
	canonicalPath, err := canonicalIdentityPath(path)
	if err != nil {
		return "", fmt.Errorf("canonicalize module identity path %q: %w", path, err)
	}
	canonicalStd, err := canonicalIdentityPath(stdRoot)
	if err != nil {
		return "", fmt.Errorf("canonicalize standard-library root %q: %w", stdRoot, err)
	}
	if relative, ok := pathWithinRoot(canonicalPath, canonicalStd); ok {
		return ModuleID(moduleIdentityVersion + ":std:v1:" + filepath.ToSlash(relative)), nil
	}
	canonicalWorkspace, err := canonicalIdentityPath(workspaceRoot)
	if err != nil {
		return "", fmt.Errorf("canonicalize workspace root %q: %w", workspaceRoot, err)
	}
	if relative, ok := pathWithinRoot(canonicalPath, canonicalWorkspace); ok {
		return ModuleID(moduleIdentityVersion + ":workspace:" + filepath.ToSlash(relative)), nil
	}
	return ModuleID(moduleIdentityVersion + ":external:" + filepath.ToSlash(canonicalPath)), nil
}

// StablePackageName preserves the historical readable-name plus ten-character
// suffix shape while making the suffix a deterministic function of ModuleID.
func StablePackageName(moduleName string, id ModuleID) string {
	digest := sha256.Sum256([]byte(id))
	return moduleName + "_" + hex.EncodeToString(digest[:5])
}

func canonicalIdentityPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	absolute = filepath.Clean(absolute)
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		absolute = filepath.Clean(resolved)
	}
	return absolute, nil
}

func pathWithinRoot(path, root string) (string, bool) {
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", false
	}
	return filepath.Clean(relative), true
}
