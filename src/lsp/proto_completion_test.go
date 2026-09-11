package lsp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTopLevelCompletionOffersMissingProtoMethodDefinition(t *testing.T) {
	source := "mod completion\nproto Writer(write(bytes str) !u64)\nSink impl Writer(value u64)\nSink.\n"
	path := filepath.Join(t.TempDir(), "completion.mg")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	items := complete(fileURI(path), source, position{Line: 3, Character: 5}, testStdRoot())
	if len(items) != 1 {
		t.Fatalf("completion items = %#v, want one missing method", items)
	}
	item := items[0]
	if item.Label != "write" || item.Kind != 2 || item.Detail != "write(bytes str) !u64" || item.InsertText != "write(bytes str) !u64:\n.." {
		t.Fatalf("write completion = %#v", item)
	}
}

func TestTopLevelProtoCompletionExcludesImplementedMethods(t *testing.T) {
	source := "mod completion\nproto Writer(write(bytes str) !u64, flush() void)\nSink impl Writer(value u64)\nSink.write(bytes str) !u64:\n    ret bytes.count()\n..\nSink.f\n"
	path := filepath.Join(t.TempDir(), "completion.mg")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	items := complete(fileURI(path), source, position{Line: 6, Character: 6}, testStdRoot())
	if len(items) != 1 || items[0].Label != "flush" || items[0].InsertText != "flush() void:\n.." {
		t.Fatalf("completion items = %#v, want only flush", items)
	}
}

func TestProtoCompletionUsesSourceSyntaxForOwnedSlices(t *testing.T) {
	source := "mod completion\nproto Reader(read(buffer $u8[]) !$u8[])\nSource impl Reader(value u64)\nSource.\n"
	path := filepath.Join(t.TempDir(), "completion.mg")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	items := complete(fileURI(path), source, position{Line: 3, Character: 7}, testStdRoot())
	if len(items) != 1 || items[0].InsertText != "read(buffer $u8[]) !$u8[]:\n.." {
		t.Fatalf("completion items = %#v", items)
	}
}

func TestProtoCompletionAddsRelativeImportForTransitiveType(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "models.mg"), []byte("mod models\npub Payload(value u64)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "contract.mg"), []byte("mod contract\nuse \"./models\" models\npub proto Writer(write(value models.Payload) void)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	source := "mod completion\n\n# Module documentation.\n\nuse \"./contract\" contract\nSink impl contract.Writer(value u64)\nSink.\n"
	path := filepath.Join(directory, "completion.mg")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	items := complete(fileURI(path), source, position{Line: 6, Character: 5}, testStdRoot())
	if len(items) != 1 || items[0].InsertText != "write(value models.Payload) void:\n.." {
		t.Fatalf("completion items = %#v", items)
	}
	if len(items[0].AdditionalTextEdits) != 1 {
		t.Fatalf("additional edits = %#v", items[0].AdditionalTextEdits)
	}
	edit := items[0].AdditionalTextEdits[0]
	if edit.NewText != "use \"./models\" as models\n" || edit.Range.Start != (position{Line: 5, Character: 0}) || edit.Range.End != edit.Range.Start {
		t.Fatalf("import edit = %#v", edit)
	}
}

func TestCompletionImportSpecifierDistinguishesStandardAndRelativeModules(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "project", "main.mg")
	standard := filepath.Join(root, "std", "net", "address.mg")
	local := filepath.Join(root, "shared", "payload.mg")
	if got, ok := completionImportSpecifier(current, standard, filepath.Join(root, "std")); !ok || got != "std:net/address" {
		t.Fatalf("standard specifier = %q, %v", got, ok)
	}
	if got, ok := completionImportSpecifier(current, local, filepath.Join(root, "std")); !ok || got != "../shared/payload" {
		t.Fatalf("relative specifier = %q, %v", got, ok)
	}
}
