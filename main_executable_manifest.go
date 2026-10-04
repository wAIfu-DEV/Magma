package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"Magma/src/types"
)

const executableManifestVersion = 1

var executableManifestMagic = [16]byte{'M', 'A', 'G', 'M', 'A', '_', 'B', 'U', 'I', 'L', 'D', '_', 'V', '1', '\r', '\n'}

const executableManifestTrailerSize = 32 + 8 + len(executableManifestMagic)

type executableManifestConfig struct {
	CompilerVersion string            `json:"compiler_version"`
	ClangPath       string            `json:"clang_path"`
	ClangVersion    string            `json:"clang_version"`
	RootModule      string            `json:"root_module"`
	StandardLibrary string            `json:"standard_library"`
	Target          string            `json:"target"`
	TargetOS        string            `json:"target_os"`
	Strategy        string            `json:"strategy"`
	Optimization    int               `json:"optimization"`
	Incremental     bool              `json:"incremental"`
	Jobs            int               `json:"jobs"`
	SafetyWarnings  bool              `json:"safety_warnings"`
	NullContext     bool              `json:"null_context"`
	ErrorTraceSlots uint64            `json:"error_trace_slots"`
	CompilerArgs    map[string]string `json:"compiler_args,omitempty"`
}

type executableManifestInput struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"mtime_ns"`
	SHA256  string `json:"sha256"`
}

type executableManifest struct {
	Version int                       `json:"version"`
	Config  executableManifestConfig  `json:"config"`
	Inputs  []executableManifestInput `json:"inputs"`
	Bundles []string                  `json:"bundles,omitempty"`
}

func manifestConfig(opts options, root, clangPath, clangVersion string) executableManifestConfig {
	args := make(map[string]string, len(opts.compilerArgs))
	for name, value := range opts.compilerArgs {
		args[name] = value
	}
	return executableManifestConfig{
		CompilerVersion: compilerVersion(), ClangPath: clangPath, ClangVersion: clangVersion,
		RootModule: root, StandardLibrary: opts.stdRoot, Target: opts.target, TargetOS: opts.targetOS,
		Strategy: opts.strategy, Optimization: opts.opt, Incremental: opts.incremental, Jobs: opts.jobs,
		SafetyWarnings: opts.safetyWarnings, NullContext: opts.nullContext, ErrorTraceSlots: opts.errorTraceSlots,
		CompilerArgs: args,
	}
}

func tryReuseExecutable(path string, config executableManifestConfig) (bool, error) {
	manifest, err := readExecutableManifest(path)
	if err != nil || manifest.Version != executableManifestVersion {
		return false, nil
	}
	want, _ := json.Marshal(config)
	got, _ := json.Marshal(manifest.Config)
	if string(want) != string(got) {
		return false, nil
	}
	for _, input := range manifest.Inputs {
		info, err := os.Stat(input.Path)
		if err != nil || !info.Mode().IsRegular() || info.Size() != input.Size {
			return false, nil
		}
		if info.ModTime().UnixNano() == input.ModTime {
			continue
		}
		digest, err := fileSHA256(input.Path)
		if err != nil || digest != input.SHA256 {
			return false, nil
		}
	}
	if err := copyBundles(path, manifest.Bundles); err != nil {
		return false, err
	}
	return true, nil
}

func writeExecutableManifest(path string, config executableManifestConfig, state *types.SharedState, assets []embeddedAsset, bundles, libraries []string) error {
	paths := make(map[string]bool)
	for _, file := range state.Files {
		if file != nil && file.FilePath != "" {
			paths[file.FilePath] = true
		}
	}
	for _, asset := range assets {
		paths[asset.Path] = true
	}
	for _, bundle := range bundles {
		paths[bundle] = true
	}
	for _, library := range libraries {
		if filepath.IsAbs(library) {
			paths[library] = true
		}
	}
	ordered := make([]string, 0, len(paths))
	for candidate := range paths {
		absolute, err := filepath.Abs(candidate)
		if err != nil {
			return fmt.Errorf("resolve manifest input %q: %w", candidate, err)
		}
		ordered = append(ordered, filepath.Clean(absolute))
	}
	sort.Strings(ordered)
	inputs := make([]executableManifestInput, 0, len(ordered))
	for _, inputPath := range ordered {
		info, err := os.Stat(inputPath)
		if err != nil {
			return fmt.Errorf("inspect manifest input %q: %w", inputPath, err)
		}
		digest, err := fileSHA256(inputPath)
		if err != nil {
			return err
		}
		inputs = append(inputs, executableManifestInput{Path: inputPath, Size: info.Size(), ModTime: info.ModTime().UnixNano(), SHA256: digest})
	}
	manifestBundles := make([]string, 0, len(bundles))
	for _, bundle := range bundles {
		absolute, err := filepath.Abs(bundle)
		if err != nil {
			return fmt.Errorf("resolve bundle %q for executable manifest: %w", bundle, err)
		}
		manifestBundles = append(manifestBundles, filepath.Clean(absolute))
	}
	manifest := executableManifest{Version: executableManifestVersion, Config: config, Inputs: inputs, Bundles: manifestBundles}
	payload, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("encode executable build manifest: %w", err)
	}
	digest := sha256.Sum256(payload)
	trailer := make([]byte, executableManifestTrailerSize)
	copy(trailer[:32], digest[:])
	binary.LittleEndian.PutUint64(trailer[32:40], uint64(len(payload)))
	copy(trailer[40:], executableManifestMagic[:])
	output, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return fmt.Errorf("open executable for build manifest: %w", err)
	}
	if _, err = output.Write(payload); err == nil {
		_, err = output.Write(trailer)
	}
	closeErr := output.Close()
	if err != nil {
		return fmt.Errorf("append executable build manifest: %w", err)
	}
	return closeErr
}

func readExecutableManifest(path string) (executableManifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return executableManifest{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.Size() < int64(executableManifestTrailerSize) {
		return executableManifest{}, fmt.Errorf("executable has no build manifest")
	}
	trailer := make([]byte, executableManifestTrailerSize)
	if _, err := file.ReadAt(trailer, info.Size()-int64(len(trailer))); err != nil {
		return executableManifest{}, err
	}
	if string(trailer[40:]) != string(executableManifestMagic[:]) {
		return executableManifest{}, fmt.Errorf("executable has no build manifest")
	}
	length := binary.LittleEndian.Uint64(trailer[32:40])
	if length > uint64(info.Size()-int64(len(trailer))) || length > 64<<20 {
		return executableManifest{}, fmt.Errorf("invalid executable build manifest length")
	}
	payload := make([]byte, int(length))
	if _, err := file.ReadAt(payload, info.Size()-int64(len(trailer))-int64(length)); err != nil && err != io.EOF {
		return executableManifest{}, err
	}
	digest := sha256.Sum256(payload)
	if string(digest[:]) != string(trailer[:32]) {
		return executableManifest{}, fmt.Errorf("invalid executable build manifest checksum")
	}
	var manifest executableManifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return executableManifest{}, err
	}
	return manifest, nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
