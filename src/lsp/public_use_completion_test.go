package lsp

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestPublicUseModuleAliasCompletion(t *testing.T) {
	directory := t.TempDir()
	mustWriteCompletionSource(t, filepath.Join(directory, "nested.mg"), "mod nested\npub allocator() void:\n..\n")
	mustWriteCompletionSource(t, filepath.Join(directory, "library.mg"), "mod library\npub use \"./nested.mg\" heap\n")

	tests := []struct {
		name   string
		body   string
		column uint32
		want   string
	}{
		{name: "exported alias", body: "    lib.\n", column: 8, want: "heap"},
		{name: "nested module member", body: "    lib.heap.\n", column: 13, want: "allocator"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := "mod main\nuse \"./library.mg\" lib\nmain() void:\n" + test.body + "..\n"
			path := filepath.Join(directory, "main.mg")
			mustWriteCompletionSource(t, path, source)
			uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
			items := complete(uri, source, position{Line: 3, Character: test.column}, testStdRoot())
			for _, item := range items {
				if item.Label == test.want {
					return
				}
			}
			t.Fatalf("completion labels %#v do not include %q", items, test.want)
		})
	}
}

func mustWriteCompletionSource(t *testing.T, path, source string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
}
