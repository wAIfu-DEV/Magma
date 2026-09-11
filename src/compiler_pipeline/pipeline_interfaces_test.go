package compilerpipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	moduleinterface "Magma/src/module_interface"
	"Magma/src/shared"
	"Magma/src/types"
)

const interfaceLibrarySource = `mod library
pub Resource(value u64)
destr Resource.close() void:
..
pub makeResource(value u64) $Resource:
    ret Resource(value=value)
..
pub consume(value $Resource) void:
    value.close()
..
pub inspect(value Resource) u64:
    ret value.value
..
pub maybe() !u64:
    ret 7
..
pub proto Reader(
    read() u64
)
pub current u64 = 9
pub const LIMIT u64 = 7
`

func interfaceBackedProgram(t *testing.T, mainSource string) (ValidatedProgram, string) {
	t.Helper()
	directory := t.TempDir()
	libraryPath := filepath.Join(directory, "library.mg")
	mainPath := filepath.Join(directory, "main.mg")
	if err := os.WriteFile(libraryPath, []byte(interfaceLibrarySource), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mainPath, []byte(mainSource), 0o600); err != nil {
		t.Fatal(err)
	}
	stdRoot, err := filepath.Abs(filepath.Join("..", "..", "std"))
	if err != nil {
		t.Fatal(err)
	}
	sourceState, err := shared.MakeShared(directory, stdRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(sourceState, mainPath); err != nil {
		t.Fatal(err)
	}
	contract, err := moduleinterface.Generate(sourceState, sourceState.Files[libraryPath], "test-compiler")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(libraryPath); err != nil {
		t.Fatal(err)
	}
	state, err := shared.MakeShared(directory, stdRoot)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseWithInterfaces(state, mainPath, "test-compiler", map[string]*moduleinterface.Interface{libraryPath: contract})
	if err != nil {
		t.Fatal(err)
	}
	imported := state.Files[libraryPath]
	if imported == nil || !imported.InterfaceOnly || len(imported.Content) != 0 {
		t.Fatalf("dependency was not interface-only: %#v", imported)
	}
	for _, declaration := range imported.GlNode.Declarations {
		if function, ok := declaration.(*types.NodeFuncDef); ok && len(function.Body.Statements) != 0 {
			t.Fatalf("imported function %s retained a body", function.AbsName)
		}
	}
	specialized, err := Specialize(parsed)
	if err != nil {
		t.Fatal(err)
	}
	linked, err := Link(specialized)
	if err != nil {
		t.Fatal(err)
	}
	typed, err := CheckTypes(linked)
	if err != nil {
		t.Fatal(err)
	}
	validated, err := ValidateLowering(typed)
	if err != nil {
		t.Fatal(err)
	}
	return validated, libraryPath
}

func TestInterfaceBackedModuleChecksWithoutDependencySource(t *testing.T) {
	validated, _ := interfaceBackedProgram(t, `mod main
use "library.mg" as library
main() !void:
    value := library.makeResource(1)
    borrowed u64 = library.inspect(value)
    result := try library.maybe()
    globalValue u64 = library.current
    limit u64 = library.LIMIT
    reader library.Reader
    library.consume(move value)
..
`)
	if _, err := CheckSafety(validated, false); err != nil {
		t.Fatal(err)
	}
}

func TestInterfaceOwnedParameterEnforcesMoveAtClient(t *testing.T) {
	validated, _ := interfaceBackedProgram(t, `mod main
use "library.mg" as library
main() void:
    value := library.makeResource(1)
    library.consume(value)
..
`)
	_, err := CheckSafety(validated, false)
	if err == nil || !strings.Contains(err.Error(), "requires 'move'") {
		t.Fatalf("ownership error = %v", err)
	}
}

func TestInterfaceGenericReparsesProviderOnlyWhenRequested(t *testing.T) {
	directory := t.TempDir()
	libraryPath := filepath.Join(directory, "library.mg")
	mainPath := filepath.Join(directory, "main.mg")
	library := "mod library\npub identity[T](value T) T:\n    ret value\n..\n"
	main := "mod main\nuse \"library.mg\" as library\nmain() void:\n    value u64 = library.identity[u64](7)\n..\n"
	if err := os.WriteFile(libraryPath, []byte(library), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mainPath, []byte(main), 0o600); err != nil {
		t.Fatal(err)
	}
	stdRoot, _ := filepath.Abs(filepath.Join("..", "..", "std"))
	source, _ := shared.MakeShared(directory, stdRoot)
	if _, err := Parse(source, mainPath); err != nil {
		t.Fatal(err)
	}
	contract, err := moduleinterface.Generate(source, source.Files[libraryPath], "test-compiler")
	if err != nil {
		t.Fatal(err)
	}
	state, _ := shared.MakeShared(directory, stdRoot)
	parsed, err := ParseWithInterfaces(state, mainPath, "test-compiler", map[string]*moduleinterface.Interface{libraryPath: contract})
	if err != nil {
		t.Fatal(err)
	}
	if !state.Files[libraryPath].InterfaceOnly {
		t.Fatal("provider source loaded before a specialization was requested")
	}
	if _, err := Specialize(parsed); err != nil {
		t.Fatal(err)
	}
	provider := state.Files[libraryPath]
	if provider == nil || provider.InterfaceOnly {
		t.Fatal("generic provider was not reparsed on demand")
	}
	found := false
	for name := range provider.GlNode.FuncDefs {
		if strings.Contains(name, "identity__g__") {
			found = true
		}
	}
	if !found {
		t.Fatal("provider-owned concrete specialization was not generated")
	}
}

func TestInterfaceBackedPublicGenericStructAndMethod(t *testing.T) {
	directory := t.TempDir()
	libraryPath := filepath.Join(directory, "library.mg")
	mainPath := filepath.Join(directory, "main.mg")
	library := "mod library\npub Box[T](value T)\npub Box[T].get() T:\n    ret this.value\n..\n"
	main := "mod main\nuse \"library.mg\" as library\nmain() void:\n    box := library.Box[u64](value=7)\n    value u64 = box.get()\n..\n"
	if err := os.WriteFile(libraryPath, []byte(library), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mainPath, []byte(main), 0o600); err != nil {
		t.Fatal(err)
	}
	stdRoot, _ := filepath.Abs(filepath.Join("..", "..", "std"))
	source, _ := shared.MakeShared(directory, stdRoot)
	if _, err := Parse(source, mainPath); err != nil {
		t.Fatal(err)
	}
	contract, err := moduleinterface.Generate(source, source.Files[libraryPath], "test-compiler")
	if err != nil {
		t.Fatal(err)
	}
	state, _ := shared.MakeShared(directory, stdRoot)
	parsed, err := ParseWithInterfaces(state, mainPath, "test-compiler", map[string]*moduleinterface.Interface{libraryPath: contract})
	if err != nil {
		t.Fatal(err)
	}
	specialized, err := Specialize(parsed)
	if err != nil {
		t.Fatal(err)
	}
	linked, err := Link(specialized)
	if err != nil {
		t.Fatal(err)
	}
	typed, err := CheckTypes(linked)
	if err != nil {
		t.Fatal(err)
	}
	validated, err := ValidateLowering(typed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CheckSafety(validated, false); err != nil {
		t.Fatal(err)
	}
}
