package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestCompilerVersion(t *testing.T) {
	if got := compilerVersion(); !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(got) {
		t.Fatalf("compilerVersion() = %q, want a semantic version", got)
	}
}

func TestCopyBundles(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "vendor")
	outputDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(sourceDir, "raylib.dll")
	if err := os.WriteFile(source, []byte("raylib"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := copyBundles(filepath.Join(outputDir, "game.exe"), []string{source, source}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(outputDir, "raylib.dll"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "raylib" {
		t.Fatalf("bundled contents = %q, want %q", got, "raylib")
	}
}

func TestCopyBundlesRejectsOutputNameCollision(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first", "shared.dll")
	second := filepath.Join(root, "second", "shared.dll")
	err := copyBundles(filepath.Join(root, "game.exe"), []string{first, second})
	if err == nil || !strings.Contains(err.Error(), "same output name") {
		t.Fatalf("error = %v, want output-name collision", err)
	}
}

func TestErrorTraceSlotsOption(t *testing.T) {
	opts, err := parseArgs([]string{"--error-trace-slots", "2048", "input.mg"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.errorTraceSlots != 2048 {
		t.Fatalf("errorTraceSlots = %d, want 2048", opts.errorTraceSlots)
	}
}

func TestErrorTraceSlotsDefault(t *testing.T) {
	opts, err := parseArgs([]string{"input.mg"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.errorTraceSlots != 1024 {
		t.Fatalf("errorTraceSlots = %d, want 1024", opts.errorTraceSlots)
	}
}

func TestErrorTraceSlotsValidation(t *testing.T) {
	for _, value := range []string{"0", "3", "65537"} {
		_, err := parseArgs([]string{"--error-trace-slots", value, "input.mg"})
		if err == nil || !strings.Contains(err.Error(), "--error-trace-slots") {
			t.Errorf("value %s: error = %v", value, err)
		}
	}
}

func TestTargetOption(t *testing.T) {
	opts, err := parseArgs([]string{"--target", "i386", "input.mg"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.target != "i386" {
		t.Fatalf("target = %q, want i386", opts.target)
	}
}
