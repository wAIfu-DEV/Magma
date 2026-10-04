package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"Magma/src/types"
)

func TestExecutableManifestReusesUnchangedInputs(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "main.mg")
	executable := filepath.Join(dir, "program")
	if err := os.WriteFile(source, []byte("mod main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(executable, []byte("executable-prefix"), 0755); err != nil {
		t.Fatal(err)
	}
	state := &types.SharedState{Files: map[string]*types.FileCtx{source: {FilePath: source}}}
	config := executableManifestConfig{CompilerVersion: "test", RootModule: source}
	if err := writeExecutableManifest(executable, config, state, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	hit, err := tryReuseExecutable(executable, config)
	if err != nil || !hit {
		t.Fatalf("tryReuseExecutable() = %v, %v; want hit", hit, err)
	}

	if err := os.WriteFile(source, []byte("mod changed\n"), 0644); err != nil {
		t.Fatal(err)
	}
	hit, err = tryReuseExecutable(executable, config)
	if err != nil || hit {
		t.Fatalf("tryReuseExecutable() after edit = %v, %v; want miss", hit, err)
	}
}

func TestExecutableManifestChecksumFallbackAcceptsTouchedInput(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "main.mg")
	executable := filepath.Join(dir, "program")
	if err := os.WriteFile(source, []byte("mod main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(executable, []byte("prefix"), 0755); err != nil {
		t.Fatal(err)
	}
	state := &types.SharedState{Files: map[string]*types.FileCtx{source: {FilePath: source}}}
	config := executableManifestConfig{CompilerVersion: "test"}
	if err := writeExecutableManifest(executable, config, state, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(source, future, future); err != nil {
		t.Fatal(err)
	}
	hit, err := tryReuseExecutable(executable, config)
	if err != nil || !hit {
		t.Fatalf("checksum fallback = %v, %v; want hit", hit, err)
	}
}

func TestExecutableManifestRejectsConfigurationChangeAndCorruption(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "main.mg")
	executable := filepath.Join(dir, "program")
	os.WriteFile(source, []byte("mod main\n"), 0644)
	os.WriteFile(executable, []byte("prefix"), 0755)
	state := &types.SharedState{Files: map[string]*types.FileCtx{source: {FilePath: source}}}
	config := executableManifestConfig{CompilerVersion: "one"}
	if err := writeExecutableManifest(executable, config, state, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if hit, _ := tryReuseExecutable(executable, executableManifestConfig{CompilerVersion: "two"}); hit {
		t.Fatal("configuration change produced a hit")
	}
	file, err := os.OpenFile(executable, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteAt([]byte{'X'}, 7); err != nil {
		t.Fatal(err)
	}
	file.Close()
	// Prefix modifications do not affect the footer. Corrupt the manifest itself.
	info, _ := os.Stat(executable)
	file, _ = os.OpenFile(executable, os.O_RDWR, 0)
	_, _ = file.WriteAt([]byte{'X'}, info.Size()-int64(executableManifestTrailerSize)-1)
	file.Close()
	if hit, _ := tryReuseExecutable(executable, config); hit {
		t.Fatal("corrupt manifest produced a hit")
	}
}

// Assumption: a medium project has 100 modules averaging 32 KiB. The warm
// benchmark measures metadata-only validation; the fallback benchmark forces
// every module through SHA-256 to quantify the conservative path.
func BenchmarkExecutableManifestValidation(b *testing.B) {
	dir := b.TempDir()
	state := &types.SharedState{Files: map[string]*types.FileCtx{}}
	payload := make([]byte, 32<<10)
	for index := 0; index < 100; index++ {
		path := filepath.Join(dir, fmt.Sprintf("module-%03d.mg", index))
		if err := os.WriteFile(path, payload, 0644); err != nil {
			b.Fatal(err)
		}
		state.Files[path] = &types.FileCtx{FilePath: path}
	}
	executable := filepath.Join(dir, "program")
	os.WriteFile(executable, []byte("prefix"), 0755)
	config := executableManifestConfig{CompilerVersion: "benchmark"}
	if err := writeExecutableManifest(executable, config, state, nil, nil, nil); err != nil {
		b.Fatal(err)
	}
	b.Run("metadata-hit", func(b *testing.B) {
		for index := 0; index < b.N; index++ {
			if hit, err := tryReuseExecutable(executable, config); err != nil || !hit {
				b.Fatalf("hit=%v err=%v", hit, err)
			}
		}
	})
	b.Run("checksum-fallback-3.2MiB", func(b *testing.B) {
		future := time.Now().Add(time.Hour)
		for path := range state.Files {
			if err := os.Chtimes(path, future, future); err != nil {
				b.Fatal(err)
			}
		}
		b.ResetTimer()
		for index := 0; index < b.N; index++ {
			if hit, err := tryReuseExecutable(executable, config); err != nil || !hit {
				b.Fatalf("hit=%v err=%v", hit, err)
			}
		}
	})
}
