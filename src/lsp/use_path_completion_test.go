package lsp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUsePathCompletion(t *testing.T) {
	project := t.TempDir()
	stdRoot := t.TempDir()
	mustWriteCompletionFile(t, filepath.Join(project, "local_module.mg"))
	mustWriteCompletionFile(t, filepath.Join(project, "nested", "child.mg"))
	mustWriteCompletionFile(t, filepath.Join(project, "ignored.txt"))
	mustWriteCompletionFile(t, filepath.Join(stdRoot, "heap.mg"))
	mustWriteCompletionFile(t, filepath.Join(stdRoot, "unix", "file.mg"))
	if err := os.Mkdir(filepath.Join(stdRoot, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	document := filepath.Join(project, "main.mg")
	uri := "file:///" + filepath.ToSlash(document)

	tests := []struct {
		name, source string
		character    uint32
		want         []string
		notWant      []string
	}{
		{name: "empty path", source: "use \"", character: 5, want: []string{"std:", "local_module", "nested/"}},
		{name: "local", source: "use \"./lo", character: 9, want: []string{"local_module"}, notWant: []string{"ignored"}},
		{name: "local directory", source: "use \"./", character: 7, want: []string{"nested/", "local_module"}},
		{name: "nested local", source: "use \"./nested/ch", character: 16, want: []string{"child"}},
		{name: "standard library protocol", source: "use \"std:", character: 9, want: []string{"heap", "unix/"}, notWant: []string{"local_module", "nested/", ".git/"}},
		{name: "standard library", source: "use \"std:h", character: 10, want: []string{"heap"}},
		{name: "nested standard library", source: "pub use \"std:unix/f", character: 19, want: []string{"file"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			items := complete(uri, test.source, position{Line: 0, Character: test.character}, stdRoot)
			if test.name == "empty path" && (len(items) == 0 || items[0].Label != "std:") {
				t.Fatalf("first empty-path completion = %#v, want std:", items)
			}
			if test.name == "empty path" && items[0].SortText != "0:std:" {
				t.Fatalf("std completion sort text = %q, want priority sort text", items[0].SortText)
			}
			byLabel := map[string]completionItem{}
			for _, item := range items {
				byLabel[item.Label] = item
			}
			for _, label := range test.want {
				item, ok := byLabel[label]
				if !ok {
					t.Errorf("completion labels %#v do not include %q", byLabel, label)
					continue
				}
				if item.TextEdit == nil || item.TextEdit.NewText != label {
					t.Errorf("completion %q text edit = %#v", label, item.TextEdit)
				}
				if item.FilterText == "" {
					t.Errorf("completion %q has empty filter text", label)
				}
			}
			for _, label := range test.notWant {
				if _, ok := byLabel[label]; ok {
					t.Errorf("completion unexpectedly includes %q", label)
				}
			}
		})
	}
}

func TestUsePathCompletionSupportsParentDirectory(t *testing.T) {
	parent := t.TempDir()
	project := filepath.Join(parent, "project")
	mustWriteCompletionFile(t, filepath.Join(parent, "shared.mg"))
	uri := "file:///" + filepath.ToSlash(filepath.Join(project, "main.mg"))
	items := complete(uri, "use \"../sh", position{Line: 0, Character: 10}, t.TempDir())
	if len(items) != 1 || items[0].Label != "shared" {
		t.Fatalf("parent completion = %#v, want shared", items)
	}
}

func TestStdUsePathCompletionPreservesProtocol(t *testing.T) {
	stdRoot := t.TempDir()
	mustWriteCompletionFile(t, filepath.Join(stdRoot, "io.mg"))
	source := "use \"std:i"
	uri := "file:///" + filepath.ToSlash(filepath.Join(t.TempDir(), "main.mg"))
	items := complete(uri, source, position{Line: 0, Character: 10}, stdRoot)
	if len(items) != 1 || items[0].Label != "io" || items[0].TextEdit == nil {
		t.Fatalf("std completion = %#v, want io with text edit", items)
	}
	edit := items[0].TextEdit
	runes := []rune(source)
	completed := string(runes[:edit.Range.Start.Character]) + edit.NewText + string(runes[edit.Range.End.Character:])
	if completed != "use \"std:io" {
		t.Fatalf("completed source = %q, want %q", completed, "use \"std:io")
	}
}

func mustWriteCompletionFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("mod completion\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}
