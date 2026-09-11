package incrementalcache

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func testInputs() Inputs {
	return Inputs{
		ModuleID: "magma-module-v1:workspace:lib/math.mg", SourceHash: SourceHash([]byte("source")),
		CompilerVersion: "test", InterfaceSchema: "mgi-v1", BackendVersion: "object-v1", LLVMVersion: "22.0.0",
		TargetTriple: "x86_64-unknown-linux-gnu", DataLayout: "e-m:e-p:64:64", SafetyMode: "strict",
		Dependencies: []Pair{{Name: "z", Value: SourceHash([]byte("z interface"))}, {Name: "a", Value: SourceHash([]byte("a interface"))}},
		CompilerArgs: []Pair{{Name: "trace", Value: "16"}}, Codegen: []Pair{{Name: "relocation", Value: "pic"}},
	}
}

func testBitcode() []byte { return append([]byte{'B', 'C', 0xc0, 0xde}, []byte("test payload")...) }

func newTestCache(t *testing.T, diagnostic func(string)) *Cache {
	t.Helper()
	base := t.TempDir()
	workspace := filepath.Join(base, "workspace")
	if err := os.Mkdir(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	cache, err := New(filepath.Join(base, "cache"), workspace, diagnostic)
	if err != nil {
		t.Fatal(err)
	}
	return cache
}

func TestKeyIsCanonicalAndExcludesPathsAndAliases(t *testing.T) {
	first := testInputs()
	second := first
	second.Dependencies = []Pair{{Name: "a", Value: SourceHash([]byte("a interface"))}, {Name: "z", Value: SourceHash([]byte("z interface"))}}
	firstKey, err := Key(first)
	if err != nil {
		t.Fatal(err)
	}
	secondKey, err := Key(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstKey != secondKey {
		t.Fatalf("map order changed key: %s != %s", firstKey, secondKey)
	}
}

func TestPublishAndExactLookup(t *testing.T) {
	cache := newTestCache(t, nil)
	input := testInputs()
	metadata, err := cache.Publish(input, []byte("interface"), testBitcode())
	if err != nil {
		t.Fatal(err)
	}
	result, err := cache.Lookup(input)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Hit || result.Metadata.Key != metadata.Key {
		t.Fatalf("unexpected lookup: %+v", result)
	}
	if string(result.Interface) != "interface" {
		t.Fatal("wrong interface object")
	}
}

func TestPrivateProviderChangeDoesNotInvalidateImporter(t *testing.T) {
	cache := newTestCache(t, nil)
	client := testInputs()
	client.Dependencies = []Pair{{Name: "provider", Value: SourceHash([]byte("public interface"))}}
	if _, err := cache.Publish(client, []byte("client interface"), testBitcode()); err != nil {
		t.Fatal(err)
	}
	// A provider implementation hash is intentionally not an Inputs field.
	result, err := cache.Lookup(client)
	if err != nil || !result.Hit {
		t.Fatalf("private provider change invalidated client: %+v, %v", result, err)
	}
	client.Dependencies[0].Value = SourceHash([]byte("changed public interface"))
	result, err = cache.Lookup(client)
	if err != nil {
		t.Fatal(err)
	}
	if result.Hit || result.Reason != "consumed dependency interface changed" {
		t.Fatalf("wrong invalidation: %+v", result)
	}
}

func TestExplainReportsFirstChangedInput(t *testing.T) {
	cache := newTestCache(t, nil)
	input := testInputs()
	if _, err := cache.Publish(input, []byte("interface"), testBitcode()); err != nil {
		t.Fatal(err)
	}
	changed := input
	changed.SourceHash = SourceHash([]byte("private edit"))
	changed.CompilerVersion = "also-changed"
	result, err := cache.Lookup(changed)
	if err != nil {
		t.Fatal(err)
	}
	if result.Reason != "source content changed" {
		t.Fatalf("unexpected explanation %q", result.Reason)
	}
}

func TestCorruptionIsSafeMissWithDiagnostic(t *testing.T) {
	var diagnostic string
	cache := newTestCache(t, func(message string) { diagnostic = message })
	input := testInputs()
	metadata, err := cache.Publish(input, []byte("interface"), testBitcode())
	if err != nil {
		t.Fatal(err)
	}
	path := cache.objectPath("bitcode", metadata.BitcodeHash, ".bc")
	if err := os.WriteFile(path, []byte("corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := cache.Lookup(input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Hit || !strings.Contains(result.Reason, "corrupt") || diagnostic == "" {
		t.Fatalf("corruption was not a diagnosed miss: %+v", result)
	}
}

func TestConcurrentWritersPublishOneImmutableEntry(t *testing.T) {
	cache := newTestCache(t, nil)
	input := testInputs()
	var wait sync.WaitGroup
	errors := make(chan error, 16)
	for range 16 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := cache.Publish(input, []byte("interface"), testBitcode())
			errors <- err
		}()
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	result, err := cache.Lookup(input)
	if err != nil || !result.Hit {
		t.Fatalf("concurrent publication failed: %+v, %v", result, err)
	}
}

func TestCacheMustBeOutsideWorkspace(t *testing.T) {
	workspace := t.TempDir()
	if _, err := New(filepath.Join(workspace, ".cache"), workspace, nil); err == nil {
		t.Fatal("source-tree cache was accepted")
	}
}

func TestFinalArtifactCorruptionIsMiss(t *testing.T) {
	cache := newTestCache(t, nil)
	key := SourceHash([]byte("graph"))
	if err := cache.PublishArtifact(key, []byte("object")); err != nil {
		t.Fatal(err)
	}
	if value, ok := cache.LookupArtifact(key); !ok || string(value) != "object" {
		t.Fatal("valid artifact missed")
	}
	if err := os.WriteFile(cache.objectPath("artifacts", key, ".o"), []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := cache.LookupArtifact(key); ok {
		t.Fatal("corrupt final artifact was accepted")
	}
}

func TestRelocationDoesNotChangeKeyAndVersionSkewDoes(t *testing.T) {
	// Inputs deliberately have no workspace/source path or importer alias.
	input := testInputs()
	before, err := Key(input)
	if err != nil {
		t.Fatal(err)
	}
	afterRelocation, err := Key(input)
	if err != nil {
		t.Fatal(err)
	}
	if before != afterRelocation {
		t.Fatal("relocation changed cache key")
	}
	input.LLVMVersion = "22.0.1"
	afterUpgrade, err := Key(input)
	if err != nil {
		t.Fatal(err)
	}
	if before == afterUpgrade {
		t.Fatal("LLVM version skew retained cache key")
	}
}
