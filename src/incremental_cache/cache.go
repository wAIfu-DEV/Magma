// Package incrementalcache implements the persistent, content-addressed cache
// used by the incremental object pipelines. It deliberately knows nothing
// about ASTs: semantic interfaces, LLVM bitcode, and build metadata are stored
// as separate immutable objects.
package incrementalcache

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const SchemaVersion = "magma-incremental-cache-v1"

type Pair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Inputs contains every semantic and code-generation input to one module's
// implementation bitcode. Lists must identify the consumed dependency
// interfaces, never dependency implementation objects.
type Inputs struct {
	ModuleID        string `json:"module_id"`
	SourceHash      string `json:"source_hash"`
	CompilerVersion string `json:"compiler_version"`
	InterfaceSchema string `json:"interface_schema"`
	BackendVersion  string `json:"backend_version"`
	LLVMVersion     string `json:"llvm_version"`
	TargetTriple    string `json:"target_triple"`
	DataLayout      string `json:"data_layout"`
	SafetyMode      string `json:"safety_mode"`
	Dependencies    []Pair `json:"dependencies"`
	CompilerArgs    []Pair `json:"compiler_args"`
	Codegen         []Pair `json:"codegen"`
}

type Metadata struct {
	Schema        string `json:"schema"`
	Key           string `json:"key"`
	Inputs        Inputs `json:"inputs"`
	InterfaceHash string `json:"interface_hash"`
	BitcodeHash   string `json:"bitcode_hash"`
}

type Result struct {
	Hit       bool
	Key       string
	Reason    string
	Interface []byte
	Bitcode   []byte
	Metadata  Metadata
}

type Cache struct {
	root       string
	diagnostic func(string)
}

// LatestInterface returns the most recently published, hash-validated semantic
// interface for a stable module identity. It is used only to seed warm parsing;
// the implementation bitcode still requires an exact Inputs lookup later.
func (c *Cache) LatestInterface(moduleID string) ([]byte, bool, string) {
	data, err := os.ReadFile(c.refPath(moduleID))
	if err != nil {
		return nil, false, "no previous entry"
	}
	var ref struct {
		Key string `json:"key"`
	}
	if json.Unmarshal(data, &ref) != nil || !validDigest(ref.Key) {
		return nil, false, "invalid previous-entry pointer"
	}
	data, err = os.ReadFile(c.objectPath("metadata", ref.Key, ".json"))
	if err != nil {
		return nil, false, "missing previous metadata"
	}
	var metadata Metadata
	if json.Unmarshal(data, &metadata) != nil || metadata.Schema != SchemaVersion || metadata.Inputs.ModuleID != moduleID || !validDigest(metadata.InterfaceHash) {
		return nil, false, "invalid previous metadata"
	}
	data, err = os.ReadFile(c.objectPath("interfaces", metadata.InterfaceHash, ".mgi"))
	if err != nil || digest(data) != metadata.InterfaceHash {
		return nil, false, "missing or corrupt interface"
	}
	return data, true, "hit"
}

// LatestResult returns the latest exact entry selected by the module pointer,
// with the same object validation as Lookup.
func (c *Cache) LatestResult(moduleID string) (Result, error) {
	data, err := os.ReadFile(c.refPath(moduleID))
	if err != nil {
		return Result{Reason: "no previous entry"}, nil
	}
	var ref struct {
		Key string `json:"key"`
	}
	if json.Unmarshal(data, &ref) != nil || !validDigest(ref.Key) {
		return Result{Reason: "invalid previous-entry pointer"}, nil
	}
	data, err = os.ReadFile(c.objectPath("metadata", ref.Key, ".json"))
	if err != nil {
		return Result{Reason: "missing previous metadata"}, nil
	}
	var metadata Metadata
	if json.Unmarshal(data, &metadata) != nil || metadata.Inputs.ModuleID != moduleID {
		return Result{Reason: "invalid previous metadata"}, nil
	}
	return c.Lookup(metadata.Inputs)
}

// LatestInputs returns the validated key inputs selected by a module's latest
// pointer. Callers use this to reject a semantic interface when the backing
// implementation source has changed since that interface was published.
func (c *Cache) LatestInputs(moduleID string) (Inputs, bool) {
	result, err := c.LatestResult(moduleID)
	if err != nil || !result.Hit {
		return Inputs{}, false
	}
	return result.Metadata.Inputs, true
}

func DefaultRoot() (string, error) {
	if value := os.Getenv("MAGMA_CACHE_DIR"); value != "" {
		return filepath.Abs(value)
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "magma", "incremental"), nil
}

// New rejects a cache nested in workspaceRoot so generated entries cannot
// silently become source-tree artifacts.
func New(root, workspaceRoot string, diagnostic func(string)) (*Cache, error) {
	if root == "" {
		var err error
		root, err = DefaultRoot()
		if err != nil {
			return nil, err
		}
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if workspaceRoot != "" {
		workspace, err := filepath.Abs(workspaceRoot)
		if err != nil {
			return nil, err
		}
		relative, err := filepath.Rel(workspace, absolute)
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("incremental cache %q must be outside workspace %q", absolute, workspace)
		}
	}
	return &Cache{root: absolute, diagnostic: diagnostic}, nil
}

func SourceHash(source []byte) string { return digest(source) }

func (c *Cache) LookupArtifact(key string) ([]byte, bool) {
	if !validDigest(key) {
		return nil, false
	}
	data, err := os.ReadFile(c.objectPath("artifacts", key, ".o"))
	if err != nil || len(data) < 65 || data[64] != '\n' || !validDigest(string(data[:64])) {
		return nil, false
	}
	payload := data[65:]
	if digest(payload) != string(data[:64]) {
		return nil, false
	}
	return payload, len(payload) != 0
}

func (c *Cache) PublishArtifact(key string, data []byte) error {
	if !validDigest(key) || len(data) == 0 {
		return fmt.Errorf("invalid final artifact cache entry")
	}
	envelope := append([]byte(digest(data)+"\n"), data...)
	return c.publishImmutable(c.objectPath("artifacts", key, ".o"), envelope)
}

func Key(input Inputs) (string, error) {
	canonical, err := canonicalInputs(input)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	return digest(data), nil
}

func (c *Cache) Lookup(input Inputs) (Result, error) {
	canonical, err := canonicalInputs(input)
	if err != nil {
		return Result{}, err
	}
	key, _ := Key(canonical)
	result := Result{Key: key}
	metadataBytes, err := os.ReadFile(c.objectPath("metadata", key, ".json"))
	if errors.Is(err, os.ErrNotExist) {
		result.Reason = c.explainMiss(canonical)
		return result, nil
	}
	if err != nil {
		return result, err
	}
	if err := json.Unmarshal(metadataBytes, &result.Metadata); err != nil {
		return c.corrupt(result, "metadata is malformed"), nil
	}
	metadata := result.Metadata
	if metadata.Schema != SchemaVersion || metadata.Key != key || !equalInputs(metadata.Inputs, canonical) {
		return c.corrupt(result, "metadata version or inputs do not match"), nil
	}
	if !validDigest(metadata.InterfaceHash) || !validDigest(metadata.BitcodeHash) {
		return c.corrupt(result, "metadata contains an invalid object hash"), nil
	}
	result.Interface, err = os.ReadFile(c.objectPath("interfaces", metadata.InterfaceHash, ".mgi"))
	if err != nil || digest(result.Interface) != metadata.InterfaceHash {
		return c.corrupt(result, "interface object is missing or corrupt"), nil
	}
	result.Bitcode, err = os.ReadFile(c.objectPath("bitcode", metadata.BitcodeHash, ".bc"))
	if err != nil || digest(result.Bitcode) != metadata.BitcodeHash || !validBitcodeSignature(result.Bitcode) {
		return c.corrupt(result, "bitcode object is missing or corrupt"), nil
	}
	result.Hit = true
	result.Reason = "exact cache key matched"
	return result, nil
}

// Publish atomically installs immutable objects, then metadata, then the
// module's latest-metadata pointer. Concurrent identical writers are benign.
func (c *Cache) Publish(input Inputs, interfaceBytes, bitcode []byte) (Metadata, error) {
	canonical, err := canonicalInputs(input)
	if err != nil {
		return Metadata{}, err
	}
	if len(interfaceBytes) == 0 {
		return Metadata{}, fmt.Errorf("cannot cache an empty module interface")
	}
	if !validBitcodeSignature(bitcode) {
		return Metadata{}, fmt.Errorf("cannot cache invalid LLVM bitcode")
	}
	key, _ := Key(canonical)
	metadata := Metadata{Schema: SchemaVersion, Key: key, Inputs: canonical, InterfaceHash: digest(interfaceBytes), BitcodeHash: digest(bitcode)}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return Metadata{}, err
	}
	encoded = append(encoded, '\n')
	for _, object := range []struct {
		kind, hash, suffix string
		data               []byte
	}{
		{"interfaces", metadata.InterfaceHash, ".mgi", interfaceBytes},
		{"bitcode", metadata.BitcodeHash, ".bc", bitcode},
		{"metadata", key, ".json", encoded},
	} {
		if err := c.publishImmutable(c.objectPath(object.kind, object.hash, object.suffix), object.data); err != nil {
			return Metadata{}, err
		}
	}
	ref, _ := json.Marshal(struct {
		Key string `json:"key"`
	}{key})
	if err := c.publishPointer(c.refPath(canonical.ModuleID), append(ref, '\n')); err != nil {
		return Metadata{}, err
	}
	return metadata, nil
}

func canonicalInputs(input Inputs) (Inputs, error) {
	if input.ModuleID == "" || input.SourceHash == "" || input.CompilerVersion == "" || input.InterfaceSchema == "" || input.BackendVersion == "" || input.LLVMVersion == "" || input.TargetTriple == "" || input.DataLayout == "" || input.SafetyMode == "" {
		return Inputs{}, fmt.Errorf("incremental cache inputs are incomplete")
	}
	if !validDigest(input.SourceHash) {
		return Inputs{}, fmt.Errorf("source hash is not SHA-256")
	}
	var err error
	if input.Dependencies, err = canonicalPairs(input.Dependencies); err != nil {
		return Inputs{}, fmt.Errorf("dependencies: %w", err)
	}
	for _, dependency := range input.Dependencies {
		if !validDigest(dependency.Value) {
			return Inputs{}, fmt.Errorf("dependency %q interface hash is not SHA-256", dependency.Name)
		}
	}
	if input.CompilerArgs, err = canonicalPairs(input.CompilerArgs); err != nil {
		return Inputs{}, fmt.Errorf("compiler arguments: %w", err)
	}
	if input.Codegen, err = canonicalPairs(input.Codegen); err != nil {
		return Inputs{}, fmt.Errorf("code generation options: %w", err)
	}
	return input, nil
}

func canonicalPairs(values []Pair) ([]Pair, error) {
	out := append([]Pair(nil), values...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	for i := range out {
		if out[i].Name == "" {
			return nil, fmt.Errorf("entry has an empty name")
		}
		if i > 0 && out[i-1].Name == out[i].Name {
			return nil, fmt.Errorf("duplicate entry %q", out[i].Name)
		}
	}
	return out, nil
}

func equalInputs(a, b Inputs) bool {
	aBytes, _ := json.Marshal(a)
	bBytes, _ := json.Marshal(b)
	return bytes.Equal(aBytes, bBytes)
}

func (c *Cache) corrupt(result Result, reason string) Result {
	result.Hit = false
	result.Interface = nil
	result.Bitcode = nil
	result.Reason = reason + "; treating as cache miss"
	if c.diagnostic != nil {
		c.diagnostic(result.Reason)
	}
	return result
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validBitcodeSignature(data []byte) bool {
	return len(data) >= 4 && bytes.Equal(data[:4], []byte{'B', 'C', 0xc0, 0xde})
}

func (c *Cache) objectPath(kind, hash, suffix string) string {
	return filepath.Join(c.root, SchemaVersion, kind, "sha256", hash[:2], hash+suffix)
}

func (c *Cache) refPath(moduleID string) string {
	return filepath.Join(c.root, SchemaVersion, "refs", digest([]byte(moduleID))+".json")
}

func (c *Cache) publishImmutable(path string, data []byte) error {
	if existing, err := os.ReadFile(path); err == nil {
		if bytes.Equal(existing, data) {
			return nil
		}
		return fmt.Errorf("immutable cache object collision at %q", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".publish-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Link(temporaryPath, path); err != nil {
		if existing, readErr := os.ReadFile(path); readErr == nil && bytes.Equal(existing, data) {
			return nil
		}
		return err
	}
	return nil
}

func (c *Cache) publishPointer(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".ref-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func (c *Cache) explainMiss(current Inputs) string {
	data, err := os.ReadFile(c.refPath(current.ModuleID))
	if err != nil {
		return "no previous entry for module"
	}
	var ref struct {
		Key string `json:"key"`
	}
	if json.Unmarshal(data, &ref) != nil || len(ref.Key) != 64 {
		return "previous-entry pointer is corrupt"
	}
	data, err = os.ReadFile(c.objectPath("metadata", ref.Key, ".json"))
	if err != nil {
		return "previous metadata is unavailable"
	}
	var previous Metadata
	if json.Unmarshal(data, &previous) != nil {
		return "previous metadata is corrupt"
	}
	return firstDifference(previous.Inputs, current)
}

func firstDifference(old, new Inputs) string {
	fields := []struct{ name, old, new string }{
		{"source content", old.SourceHash, new.SourceHash}, {"compiler version", old.CompilerVersion, new.CompilerVersion},
		{"interface schema", old.InterfaceSchema, new.InterfaceSchema}, {"backend version", old.BackendVersion, new.BackendVersion},
		{"LLVM version", old.LLVMVersion, new.LLVMVersion}, {"target triple", old.TargetTriple, new.TargetTriple},
		{"data layout", old.DataLayout, new.DataLayout}, {"safety mode", old.SafetyMode, new.SafetyMode},
	}
	for _, field := range fields {
		if field.old != field.new {
			return field.name + " changed"
		}
	}
	if !equalPairs(old.Dependencies, new.Dependencies) {
		return "consumed dependency interface changed"
	}
	if !equalPairs(old.CompilerArgs, new.CompilerArgs) {
		return "compiler argument changed"
	}
	if !equalPairs(old.Codegen, new.Codegen) {
		return "code-generation option changed"
	}
	return "cache entry is unavailable"
}

func equalPairs(a, b []Pair) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
