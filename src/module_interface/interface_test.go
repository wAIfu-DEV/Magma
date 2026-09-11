package moduleinterface_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	compilerpipeline "Magma/src/compiler_pipeline"
	moduleinterface "Magma/src/module_interface"
	"Magma/src/shared"
)

const publicModule = `mod library
pub use "dependency.mg" as dependency

pub Resource(value u64)
destr Resource.close() void:
..
pub Resource.read() u64:
    ret this.value
..
pub create(value u64) $Resource:
    ret Resource(value=value)
..
pub consume(value $Resource) void:
    value.close()
..
pub identity[T](value T) T:
    ret value
..
pub alias Count = u64
pub union Choice(
    Empty
    Value(value u64)
)
pub proto Reader(
    read() u64
)
pub current u64 = 9
pub const LIMIT u64 = 7
privateHelper() u64:
    ret 1
..
`

func loadInterface(t *testing.T, library string) *moduleinterface.Interface {
	t.Helper()
	directory := t.TempDir()
	files := map[string]string{
		"dependency.mg": "mod dependency\npub answer() u64:\n    ret 42\n..\n",
		"library.mg":    library,
		"main.mg":       "mod main\nuse \"library.mg\" as library\nmain() void:\n    value := library.create(1)\n    library.consume(move value)\n..\n",
	}
	for name, source := range files {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	stdRoot, err := filepath.Abs(filepath.Join("..", "..", "std"))
	if err != nil {
		t.Fatal(err)
	}
	state, err := shared.MakeShared(directory, stdRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := compilerpipeline.Parse(state, filepath.Join(directory, "main.mg")); err != nil {
		t.Fatal(err)
	}
	value, err := moduleinterface.Generate(state, state.Files[filepath.Join(directory, "library.mg")], "test-compiler")
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestInterfaceRoundTripPreservesPublicSurfaceAndOwnership(t *testing.T) {
	value := loadInterface(t, publicModule)
	encoded, err := moduleinterface.Encode(value)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := moduleinterface.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	reencoded, err := moduleinterface.Encode(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, reencoded) {
		t.Fatalf("interface encoding is not canonical\n%s\n%s", encoded, reencoded)
	}
	index, err := moduleinterface.NewIndex(decoded)
	if err != nil {
		t.Fatal(err)
	}
	consume := index.Functions["consume"]
	if len(consume.Arguments) != 1 || !consume.Arguments[0].Type.Owned {
		t.Fatalf("consume signature lost ownership: %#v", consume)
	}
	create := index.Functions["create"]
	if !create.Result.Owned {
		t.Fatalf("create result lost ownership: %#v", create.Result)
	}
	if index.Structs["Resource"].Fields[0].Name != "value" || len(index.Structs["Resource"].Destructors) != 1 {
		t.Fatalf("resource definition incomplete: %#v", index.Structs["Resource"])
	}
	if len(index.Functions["identity"].TypeParams) != 1 || index.Aliases["Count"].Target.Name != "u64" {
		t.Fatal("generic or alias information missing")
	}
	if _, ok := index.Functions["privateHelper"]; ok {
		t.Fatal("private function leaked into interface")
	}
	if index.Reexports["dependency"] == "" || index.Constants["LIMIT"].Value == "" || index.Globals["current"].Type.Name != "u64" {
		t.Fatal("re-export, constant, or global missing")
	}
	if err := moduleinterface.ValidateCompiler(decoded, "different-compiler"); err == nil {
		t.Fatal("incompatible compiler version accepted")
	}
	path := filepath.Join(t.TempDir(), "library.mgi")
	if err := moduleinterface.WriteFile(path, decoded); err != nil {
		t.Fatal(err)
	}
	fromFile, err := moduleinterface.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	fromFileBytes, err := moduleinterface.Encode(fromFile)
	if err != nil || !bytes.Equal(encoded, fromFileBytes) {
		t.Fatalf("file round trip changed interface: %v", err)
	}
}

func TestInterfaceHashIgnoresPrivateBodiesAndTracksPublicSemantics(t *testing.T) {
	base := loadInterface(t, publicModule)
	privateChanged := loadInterface(t, bytes.NewBufferString(publicModule).String()[:len(publicModule)-len("    ret 1\n..\n")]+"    ret 2\n..\n")
	publicChanged := loadInterface(t, string(bytes.Replace([]byte(publicModule), []byte("pub const LIMIT u64 = 7"), []byte("pub const LIMIT u64 = 8"), 1)))
	baseHash, err := moduleinterface.Hash(base)
	if err != nil {
		t.Fatal(err)
	}
	privateHash, err := moduleinterface.Hash(privateChanged)
	if err != nil {
		t.Fatal(err)
	}
	publicHash, err := moduleinterface.Hash(publicChanged)
	if err != nil {
		t.Fatal(err)
	}
	if baseHash != privateHash {
		t.Fatalf("private body changed interface hash: %s != %s", baseHash, privateHash)
	}
	if baseHash == publicHash {
		t.Fatal("public constant change did not change interface hash")
	}
	for name, changedSource := range map[string]string{
		"ownership": strings.Replace(publicModule, "pub consume(value $Resource)", "pub consume(value Resource)", 1),
		"layout":    strings.Replace(publicModule, "pub Resource(value u64)", "pub Resource(value u32)", 1),
		"signature": strings.Replace(publicModule, "pub create(value u64)", "pub create(value u32)", 1),
	} {
		changed := loadInterface(t, changedSource)
		changedHash, err := moduleinterface.Hash(changed)
		if err != nil {
			t.Fatal(err)
		}
		if changedHash == baseHash {
			t.Errorf("public %s change did not change interface hash", name)
		}
	}
}

func TestInterfaceCarriesPrivateBackingLayoutsAndPublicOwnerMethods(t *testing.T) {
	value := loadInterface(t, `mod library
pub Handle(state State*)
State(value u64)
pub proto Allocator(
    alloc(count u64) $u8*
)
Allocator.allocT[T](count u64) $T*:
    ret none
..
`)
	privateState := false
	genericMethod := false
	for _, structure := range value.Structs {
		if structure.Name == "State" {
			privateState = structure.Private
		}
	}
	for _, function := range value.Functions {
		if function.Name == "Allocator.allocT" && len(function.TypeParams) == 1 {
			genericMethod = true
		}
	}
	if !privateState || !genericMethod {
		t.Fatalf("incomplete interface: private backing=%t generic owner method=%t", privateState, genericMethod)
	}
	materialized, err := moduleinterface.Materialize(value, map[string]*moduleinterface.Interface{value.ModuleID: value}, "library.mg")
	if err != nil {
		t.Fatal(err)
	}
	if state := materialized.GlNode.StructDefs["State"]; state == nil || state.IsPublic {
		t.Fatalf("private backing type became public: %#v", state)
	}
}

func TestPublicConstantHashExpandsPrivateConstantDependencies(t *testing.T) {
	firstSource := strings.Replace(publicModule, "pub const LIMIT u64 = 7", "const INTERNAL u64 = 7\npub const LIMIT u64 = INTERNAL", 1)
	secondSource := strings.Replace(firstSource, "const INTERNAL u64 = 7", "const INTERNAL u64 = 8", 1)
	first, err := moduleinterface.Hash(loadInterface(t, firstSource))
	if err != nil {
		t.Fatal(err)
	}
	second, err := moduleinterface.Hash(loadInterface(t, secondSource))
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("private constant change affecting public value did not change interface hash")
	}
}

func TestVerifyImplementationRejectsPublicContractDrift(t *testing.T) {
	directory := t.TempDir()
	libraryPath := filepath.Join(directory, "library.mg")
	mainPath := filepath.Join(directory, "main.mg")
	if err := os.WriteFile(libraryPath, []byte(publicModule), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "dependency.mg"), []byte("mod dependency\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mainPath, []byte("mod main\nuse \"library.mg\" as library\nmain() void:\n..\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdRoot, err := filepath.Abs(filepath.Join("..", "..", "std"))
	if err != nil {
		t.Fatal(err)
	}
	state, err := shared.MakeShared(directory, stdRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := compilerpipeline.Parse(state, mainPath); err != nil {
		t.Fatal(err)
	}
	expected, err := moduleinterface.Generate(state, state.Files[libraryPath], "test-compiler")
	if err != nil {
		t.Fatal(err)
	}
	if err := moduleinterface.VerifyImplementation(state, state.Files[libraryPath], expected, "test-compiler"); err != nil {
		t.Fatal(err)
	}
	expected.Constants[0].Value = "lit:2:999"
	if err := moduleinterface.VerifyImplementation(state, state.Files[libraryPath], expected, "test-compiler"); err == nil {
		t.Fatal("public contract drift was accepted")
	}
}

func TestDecodeRejectsUnknownSchemaFieldsAndTrailingData(t *testing.T) {
	value := loadInterface(t, publicModule)
	encoded, err := moduleinterface.Encode(value)
	if err != nil {
		t.Fatal(err)
	}
	unknown := append([]byte(nil), encoded[:len(encoded)-1]...)
	unknown = append(unknown, []byte(`,"unknown":true}`)...)
	if _, err := moduleinterface.Decode(unknown); err == nil {
		t.Fatal("unknown field accepted")
	}
	if _, err := moduleinterface.Decode(append(encoded, []byte(" {}")...)); err == nil {
		t.Fatal("trailing data accepted")
	}
	value.Schema++
	if _, err := moduleinterface.Encode(value); err == nil {
		t.Fatal("unknown schema accepted")
	}
}

func TestInterfaceGenerationCoversLoadedStandardLibraryModules(t *testing.T) {
	repository, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	state, err := shared.MakeShared(repository, filepath.Join(repository, "std"))
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(repository, "std", "tests", "hash_map.mg")
	if _, err := compilerpipeline.Parse(state, root); err != nil {
		t.Fatal(err)
	}
	for path, file := range state.Files {
		value, err := moduleinterface.Generate(state, file, "test-compiler")
		if err != nil {
			t.Fatalf("generate %s: %v", path, err)
		}
		encoded, err := moduleinterface.Encode(value)
		if err != nil {
			t.Fatalf("encode %s: %v", path, err)
		}
		if _, err := moduleinterface.Decode(encoded); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
	}
	available := map[string]*moduleinterface.Interface{}
	for _, file := range state.Files {
		value, err := moduleinterface.Generate(state, file, "test-compiler")
		if err != nil {
			t.Fatal(err)
		}
		available[value.ModuleID] = value
	}
	for path, file := range state.Files {
		value := available[string(file.ModuleID)]
		materialized, err := moduleinterface.Materialize(value, available, path)
		if err != nil {
			t.Fatalf("materialize %s: %v", path, err)
		}
		if !materialized.InterfaceOnly || len(materialized.Content) != 0 {
			t.Fatalf("materialized module %s contains implementation source", path)
		}
	}
}
