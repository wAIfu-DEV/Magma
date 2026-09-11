//go:build llvm_object

package compilerpipeline

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	clangresolver "Magma/src/clang"
	llvmobject "Magma/src/llvm_object"
	"Magma/src/shared"
	magmatarget "Magma/src/target"
	"Magma/src/types"
)

func TestTextualAndObjectLoweringUseSharedStableModuleIdentity(t *testing.T) {
	parsed, path := testProgram(t, "mod main\nmain() void:\n..\n")
	if err := RequireMainModule(*parsed, path); err != nil {
		t.Fatal(err)
	}
	specialized, err := Specialize(*parsed)
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
	ready, err := CheckSafety(validated, false)
	if err != nil {
		t.Fatal(err)
	}
	packageName := parsed.State().Files[path].PackageName
	textual, err := LowerReachable(ready)
	if err != nil {
		t.Fatal(err)
	}
	objectIR, err := LowerObjectIR(ready)
	if err != nil {
		t.Fatal(err)
	}
	want := "@" + packageName + ".main"
	if !bytes.Contains(textual, []byte(want)) || !bytes.Contains(objectIR, []byte(want)) {
		t.Fatalf("shared symbol %q missing: textual=%t object=%t", want, bytes.Contains(textual, []byte(want)), bytes.Contains(objectIR, []byte(want)))
	}
}

func TestSpecializationsCacheSeparatelyFromProvider(t *testing.T) {
	repository, _ := filepath.Abs(filepath.Join("..", ".."))
	directory := t.TempDir()
	libraryPath := filepath.Join(directory, "library.mg")
	mainPath := filepath.Join(directory, "main.mg")
	librarySource := "mod library\npub identity[T](value T) T:\n    ret value\n..\npub inner[T](value T) T:\n    ret value\n..\npub outer[T](value T) T:\n    ret inner[T](value)\n..\n"
	if err := os.WriteFile(libraryPath, []byte(librarySource), 0o600); err != nil {
		t.Fatal(err)
	}
	build := func(main string) SafetyCheckedProgram {
		if err := os.WriteFile(mainPath, []byte(main), 0o600); err != nil {
			t.Fatal(err)
		}
		state, err := shared.MakeShared(directory, filepath.Join(repository, "std"))
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := Parse(state, mainPath)
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
		ready, err := CheckSafety(validated, false)
		if err != nil {
			t.Fatal(err)
		}
		return ready
	}
	cacheRoot := filepath.Join(t.TempDir(), "cache")
	first := build("mod main\nuse \"library.mg\" as library\nmain() void:\n    a u64 = library.identity[u64](1)\n    nested u64 = library.outer[u64](a)\n..\n")
	if _, err := LowerCachedIncrementalObjectBytes(first, llvmobject.TargetOptions{}, cacheRoot, "test-compiler", "strict", nil); err != nil {
		t.Fatal(err)
	}
	second := build("mod main\nuse \"library.mg\" as library\nmain() void:\n    a u64 = library.identity[u64](1)\n    b u32 = library.identity[u32](2)\n    nested u64 = library.outer[u64](a)\n..\n")
	messages := []string{}
	optimized, err := LowerCachedIncrementalObjectBytes(second, llvmobject.TargetOptions{Optimization: llvmobject.OptimizationAggressive}, cacheRoot, "test-compiler", "strict", func(message string) { messages = append(messages, message) })
	if err != nil {
		t.Fatal(err)
	}
	objectPath := filepath.Join(t.TempDir(), "optimized.o")
	if err := os.WriteFile(objectPath, optimized, 0o600); err != nil {
		t.Fatal(err)
	}
	if symbols, err := exec.Command("nm", objectPath).CombinedOutput(); err == nil && bytes.Contains(symbols, []byte("identity__g__")) {
		t.Fatalf("cross-module specialization was not inlined/dead-stripped:\n%s", symbols)
	}
	joined := strings.Join(messages, "\n")
	libraryID := string(second.State().Files[libraryPath].ModuleID)
	if !strings.Contains(joined, libraryID+": hit") {
		t.Fatalf("ordinary provider was invalidated by a new instance:\n%s", joined)
	}
	if !strings.Contains(joined, "identity__g__N_u64: hit") {
		t.Fatalf("existing specialization missed:\n%s", joined)
	}
	if !strings.Contains(joined, "identity__g__N_u32: miss") {
		t.Fatalf("new specialization did not miss independently:\n%s", joined)
	}
	if !strings.Contains(joined, "inner__g__N_u64: hit") {
		t.Fatalf("nested cached specialization was not discovered:\n%s", joined)
	}
	if err := os.WriteFile(libraryPath, []byte(strings.Replace(librarySource, "ret value\n..\npub inner", "ret value + 0\n..\npub inner", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	third := build("mod main\nuse \"library.mg\" as library\nmain() void:\n    a u64 = library.identity[u64](1)\n    b u32 = library.identity[u32](2)\n    nested u64 = library.outer[u64](a)\n..\n")
	messages = nil
	if _, err := LowerCachedIncrementalObjectBytes(third, llvmobject.TargetOptions{Optimization: llvmobject.OptimizationAggressive}, cacheRoot, "test-compiler", "strict", func(message string) { messages = append(messages, message) }); err != nil {
		t.Fatal(err)
	}
	joined = strings.Join(messages, "\n")
	if !strings.Contains(joined, libraryID+": hit") {
		t.Fatalf("generic body edit invalidated ordinary provider:\n%s", joined)
	}
	if !strings.Contains(joined, "identity__g__N_u64: miss") || !strings.Contains(joined, "identity__g__N_u32: miss") {
		t.Fatalf("generic body edit retained stale specializations:\n%s", joined)
	}
}

func TestObjectPipelineStandardLibraryCorpus(t *testing.T) {
	if os.Getenv("MAGMA_OBJECT_CORPUS") != "1" {
		t.Skip("set MAGMA_OBJECT_CORPUS=1 to run the in-progress full object-lowering corpus")
	}
	repository, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join(repository, "std", "tests", "*.mg"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("standard-library corpus is empty")
	}
	clangPath, _, err := clangresolver.Resolve("")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			state, err := shared.MakeShared(repository, filepath.Join(repository, "std"))
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := Parse(state, path)
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
			ready, err := CheckSafety(validated, false)
			if err != nil {
				t.Fatal(err)
			}
			if os.Getenv("MAGMA_OBJECT_GDB") == "1" {
				ir, err := LowerObjectIR(ready)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(os.TempDir(), "magma-object-debug.ll"), ir, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			object, err := LowerObjectBytes(ready, llvmobject.TargetOptions{PIC: true})
			if err != nil {
				t.Fatal(err)
			}
			directory := t.TempDir()
			objectPath := filepath.Join(directory, "program.o")
			executablePath := filepath.Join(directory, "program")
			if err := os.WriteFile(objectPath, object, 0o600); err != nil {
				t.Fatal(err)
			}
			linkArgs := []string{objectPath}
			libraries := objectCorpusNativeLibraries(state)
			for _, library := range libraries {
				if filepath.IsAbs(library) {
					linkArgs = append(linkArgs, library)
				} else {
					linkArgs = append(linkArgs, "-l"+library)
				}
			}
			linkArgs = append(linkArgs, "-Wl,-rpath,"+repository, "-o", executablePath)
			if output, err := exec.Command(clangPath, linkArgs...).CombinedOutput(); err != nil {
				t.Fatalf("link object executable: %v\n%s", err, output)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, executablePath)
			command.Dir = repository
			if output, err := command.CombinedOutput(); err != nil {
				if ctx.Err() != nil {
					t.Fatalf("object executable timed out: %v\n%s", ctx.Err(), output)
				}
				if os.Getenv("MAGMA_OBJECT_GDB") == "1" {
					debugOutput, _ := exec.Command("gdb", "--batch", "-ex", "run", "-ex", "bt", "-ex", "info registers", "-ex", "x/16i $pc-24", executablePath).CombinedOutput()
					output = append(output, debugOutput...)
				}
				t.Fatalf("object executable failed: %v\n%s", err, output)
			}
		})
	}
}

func objectCorpusNativeLibraries(state *types.SharedState) []string {
	seen := make(map[string]bool)
	for _, file := range state.Files {
		if file == nil {
			continue
		}
		for _, library := range file.NativeLibraries {
			seen[library] = true
		}
	}
	libraries := make([]string, 0, len(seen))
	for library := range seen {
		libraries = append(libraries, library)
	}
	sort.Strings(libraries)
	return libraries
}

func TestCheckedProgramLowersThroughWholeObjectPipeline(t *testing.T) {
	validated := validateTestProgram(t, `mod main
pub main(args str[]) !void:
..
`)
	ready, err := CheckSafety(validated, false)
	if err != nil {
		t.Fatal(err)
	}
	foundEntry := false
	entrySymbol := ""
	for _, file := range ready.State().Files {
		if file == nil || file.GlNode == nil {
			continue
		}
		for _, declaration := range file.GlNode.Declarations {
			if function, ok := declaration.(*types.NodeFuncDef); ok && function.IsEntryPoint {
				foundEntry = true
				entrySymbol = function.AbsName
			}
		}
	}
	if !foundEntry {
		t.Fatal("checked program has no entry marker")
	}
	ir, err := LowerObjectIR(ready)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ir), "define internal void @"+entrySymbol+"(") {
		if !strings.Contains(string(ir), "@"+entrySymbol+"(") {
			t.Fatalf("object module lacks checked function:\n%s", ir)
		}
	}
	if !strings.Contains(string(ir), "define i32 @main(i32") || !strings.Contains(string(ir), "call %type.slice @magma.argsToSlice") {
		t.Fatalf("object module lacks native argument/entry wrapper")
	}
	object, err := LowerObjectBytes(ready, llvmobject.TargetOptions{PIC: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(object) < 4 || !bytes.Equal(object[:4], []byte{0x7f, 'E', 'L', 'F'}) {
		t.Fatalf("whole-program object lacks ELF header")
	}
	directory := t.TempDir()
	objectPath := filepath.Join(directory, "program.o")
	executablePath := filepath.Join(directory, "program")
	if err := os.WriteFile(objectPath, object, 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("clang", objectPath, "-o", executablePath).CombinedOutput(); err != nil {
		t.Fatalf("link object program: %v\n%s", err, output)
	}
	if output, err := exec.Command(executablePath, "one", "two").CombinedOutput(); err != nil {
		t.Fatalf("run object program: %v\n%s", err, output)
	}
}

func TestObjectPipelineBuildsContextDiscardAdapter(t *testing.T) {
	validated := validateTestProgram(t, `mod adapter
noctx plusOne(value u64) u64:
    ret value + 1
..
invoke(callback (u64) u64, value u64) u64:
    ret callback(value)
..
exercise(value u64) u64:
    ret invoke(plusOne, value)
..
pub main(args str[]) !void:
    value := exercise(41)
..
`)
	ready, err := CheckSafety(validated, false)
	if err != nil {
		t.Fatal(err)
	}
	ir, err := LowerObjectIR(ready)
	if err != nil {
		t.Fatal(err)
	}
	text := string(ir)
	if !strings.Contains(text, ".__ctx_adapter(ptr") || !strings.Contains(text, "alwaysinline") {
		t.Fatalf("object module lacks context-discard adapter:\n%s", text)
	}
}

func TestObjectPipelineBuildsNativeContextThunk(t *testing.T) {
	validated := validateTestProgram(t, `mod callback
ext ext_install install(callback noctx (ptr) u64) void
worker(raw ptr) u64:
    ret 0
..
pub main(args str[]) !void:
    ext_install(worker)
..
`)
	ready, err := CheckSafety(validated, false)
	if err != nil {
		t.Fatal(err)
	}
	ir, err := LowerObjectIR(ready)
	if err != nil {
		t.Fatal(err)
	}
	text := string(ir)
	if !strings.Contains(text, ".__native_ctx_thunk(ptr") || !strings.Contains(text, "call void @install(ptr @") {
		t.Fatalf("object module lacks native-context callback bridge:\n%s", text)
	}
}

func TestObjectPipelinePreservesContextualWideIntegerLiteral(t *testing.T) {
	validated := validateTestProgram(t, `mod wide
maximum() i128:
    ret 170141183460469231731687303715884105727
..
pub main(args str[]) !void:
    value := maximum()
..
`)
	ready, err := CheckSafety(validated, false)
	if err != nil {
		t.Fatal(err)
	}
	ir, err := LowerObjectIR(ready)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ir), "ret i128 170141183460469231731687303715884105727") {
		t.Fatalf("object module lost contextual i128 literal:\n%s", ir)
	}
}

func TestObjectPipelineBuildsFloatingGlobalInitializer(t *testing.T) {
	validated := validateTestProgram(t, `mod float_global
global ratio f64 = 1.5
pub main() !void:
    value := ratio
..
`)
	ready, err := CheckSafety(validated, false)
	if err != nil {
		t.Fatal(err)
	}
	ir, err := LowerObjectIR(ready)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ir), "global double 1.500000e+00") {
		t.Fatalf("object module lacks floating global initializer:\n%s", ir)
	}
}

func TestObjectPipelineCExportInterop(t *testing.T) {
	validated := validateTestProgram(t, `mod interop
@export_name("magma_add")
add(left i32, right i32) i32:
    ret left + right
..
`)
	ready, err := CheckSafety(validated, false)
	if err != nil {
		t.Fatal(err)
	}
	object, err := LowerObjectBytes(ready, llvmobject.TargetOptions{PIC: true})
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	objectPath := filepath.Join(directory, "export.o")
	callerPath := filepath.Join(directory, "caller.c")
	executablePath := filepath.Join(directory, "caller")
	if err := os.WriteFile(objectPath, object, 0o600); err != nil {
		t.Fatal(err)
	}
	const caller = `#include <pthread.h>
#include <stdint.h>
extern int32_t magma_add(int32_t, int32_t);
static void *run(void *unused) {
    (void)unused;
    return (void *)(uintptr_t)(magma_add(20, 22) != 42);
}
int main(void) {
    if (magma_add(19, 23) != 42) return 1;
    pthread_t thread;
    void *result = 0;
    if (pthread_create(&thread, 0, run, 0) != 0) return 2;
    if (pthread_join(thread, &result) != 0) return 3;
    return (int)(uintptr_t)result;
}
`
	if err := os.WriteFile(callerPath, []byte(caller), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("clang", objectPath, callerPath, "-pthread", "-o", executablePath).CombinedOutput(); err != nil {
		t.Fatalf("link C caller with object export: %v\n%s", err, output)
	}
	if output, err := exec.Command(executablePath).CombinedOutput(); err != nil {
		t.Fatalf("C caller received the wrong export result: %v\n%s", err, output)
	}
}

func TestObjectPipelineAggregateCExportInterop(t *testing.T) {
	validated := validateTestProgram(t, `mod aggregate_interop
Pair(x f32, y f32)
@export_name("magma_pair_sum")
sum(pair Pair) f32:
    ret pair.x + pair.y
..
@export_name("magma_pair_make")
makePair(x f32, y f32) Pair:
    ret Pair(x=x, y=y)
..
Large(a u64, b u64, c u64)
@export_name("magma_large_echo")
echoLarge(value Large) Large:
    ret value
..
`)
	ready, err := CheckSafety(validated, false)
	if err != nil {
		t.Fatal(err)
	}
	ir, err := LowerObjectIR(ready)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ir), "define float @magma_pair_sum(<2 x float>") {
		t.Fatalf("SysV aggregate export was not vector-coerced:\n%s", ir)
	}
	if !strings.Contains(string(ir), "define <2 x float> @magma_pair_make(") || !strings.Contains(string(ir), "define void @magma_large_echo(ptr sret(") || !strings.Contains(string(ir), "byval(") {
		t.Fatalf("object module lacks aggregate return/byval/sret ABI forms:\n%s", ir)
	}
	object, err := LowerObjectBytes(ready, llvmobject.TargetOptions{PIC: true})
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	objectPath := filepath.Join(directory, "aggregate-export.o")
	callerPath := filepath.Join(directory, "aggregate-caller.c")
	executablePath := filepath.Join(directory, "aggregate-caller")
	if err := os.WriteFile(objectPath, object, 0o600); err != nil {
		t.Fatal(err)
	}
	const caller = `#include <stdint.h>
typedef struct { float x; float y; } Pair;
typedef struct { uint64_t a; uint64_t b; uint64_t c; } Large;
extern float magma_pair_sum(Pair);
extern Pair magma_pair_make(float, float);
extern Large magma_large_echo(Large);
int main(void) {
    if (magma_pair_sum((Pair){19.0f, 23.0f}) != 42.0f) return 1;
    Pair pair = magma_pair_make(19.0f, 23.0f);
    if (pair.x != 19.0f || pair.y != 23.0f) return 2;
    Large large = magma_large_echo((Large){11, 22, 33});
    return large.a == 11 && large.b == 22 && large.c == 33 ? 0 : 3;
}
`
	if err := os.WriteFile(callerPath, []byte(caller), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("clang", objectPath, callerPath, "-pthread", "-o", executablePath).CombinedOutput(); err != nil {
		t.Fatalf("link aggregate C caller: %v\n%s", err, output)
	}
	if output, err := exec.Command(executablePath).CombinedOutput(); err != nil {
		t.Fatalf("aggregate C export returned the wrong result: %v\n%s", err, output)
	}
}

func TestObjectPipelineAggregateExternalCallInterop(t *testing.T) {
	validated := validateTestProgram(t, `mod aggregate_calls
Pair(x f32, y f32)
Large(a u64, b u64, c u64)
ext ext_pair_sum c_pair_sum(pair Pair) f32
ext ext_pair_make c_pair_make(x f32, y f32) Pair
ext ext_large_echo c_large_echo(value Large) Large
@export_name("magma_check_external_aggregates")
check() i32:
    pair := ext_pair_make(19.0, 23.0)
    if ext_pair_sum(pair) != 42.0:
        ret 1
    ..
    large := ext_large_echo(Large(a=11, b=22, c=33))
    if large.a != 11 || large.b != 22 || large.c != 33:
        ret 2
    ..
    ret 0
..
`)
	ready, err := CheckSafety(validated, false)
	if err != nil {
		t.Fatal(err)
	}
	object, err := LowerObjectBytes(ready, llvmobject.TargetOptions{PIC: true})
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	objectPath := filepath.Join(directory, "aggregate-calls.o")
	callerPath := filepath.Join(directory, "aggregate-calls.c")
	executablePath := filepath.Join(directory, "aggregate-calls")
	if err := os.WriteFile(objectPath, object, 0o600); err != nil {
		t.Fatal(err)
	}
	const caller = `#include <stdint.h>
typedef struct { float x; float y; } Pair;
typedef struct { uint64_t a; uint64_t b; uint64_t c; } Large;
float c_pair_sum(Pair pair) { return pair.x + pair.y; }
Pair c_pair_make(float x, float y) { return (Pair){x, y}; }
Large c_large_echo(Large value) { return value; }
extern int32_t magma_check_external_aggregates(void);
int main(void) { return magma_check_external_aggregates(); }
`
	if err := os.WriteFile(callerPath, []byte(caller), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("clang", objectPath, callerPath, "-pthread", "-o", executablePath).CombinedOutput(); err != nil {
		t.Fatalf("link aggregate external-call fixture: %v\n%s", err, output)
	}
	if output, err := exec.Command(executablePath).CombinedOutput(); err != nil {
		t.Fatalf("aggregate external calls used the wrong ABI: %v\n%s", err, output)
	}
}

func TestObjectPipelineBuildsWindowsWmainWithoutArguments(t *testing.T) {
	validated := validateTestProgram(t, `mod windows_entry
pub main() !void:
..
`)
	ready, err := CheckSafety(validated, false)
	if err != nil {
		t.Fatal(err)
	}
	ready.State().Target = magmatarget.Target{Arch: "x86_64", OS: "windows", PointerBits: 64, Triple: "x86_64-pc-windows-msvc"}
	ir, err := LowerObjectIR(ready)
	if err != nil {
		t.Fatal(err)
	}
	text := string(ir)
	if !strings.Contains(text, "define i32 @wmain(i32") || !strings.Contains(text, "call i32 @SetConsoleOutputCP(i32 65001)") {
		t.Fatalf("object module lacks Windows entry setup:\n%s", text)
	}
}

func TestObjectPipelineBuildsWindowsUTF16ArgumentBridge(t *testing.T) {
	validated := validateTestProgram(t, `mod windows_args
pub main(args str[]) !void:
..
`)
	ready, err := CheckSafety(validated, false)
	if err != nil {
		t.Fatal(err)
	}
	ready.State().Target = magmatarget.Target{Arch: "x86_64", OS: "windows", PointerBits: 64, Triple: "x86_64-pc-windows-msvc"}
	ir, err := LowerObjectIR(ready)
	if err != nil {
		t.Fatal(err)
	}
	text := string(ir)
	for _, fragment := range []string{
		"define internal i1 @magma.argsFromUtf16",
		"define internal void @magma.freeUtf8Args",
		"call i32 @WideCharToMultiByte",
		"call i1 @magma.argsFromUtf16",
		"call void @magma.freeUtf8Args",
		"args.failed:",
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("object module lacks Windows argument fragment %q:\n%s", fragment, text)
		}
	}
	object, err := LowerObjectBytes(ready, llvmobject.TargetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(object) < 2 || object[0] != 0x64 || object[1] != 0x86 {
		t.Fatalf("Windows lowering did not emit an AMD64 COFF object: %x", object[:min(len(object), 8)])
	}
}

func TestObjectPipelineEmitsMachO(t *testing.T) {
	validated := validateTestProgram(t, `mod macho_entry
pub main() !void:
..
`)
	ready, err := CheckSafety(validated, false)
	if err != nil {
		t.Fatal(err)
	}
	ready.State().Target = magmatarget.Target{Arch: "x86_64", OS: "darwin", PointerBits: 64, Triple: "x86_64-apple-darwin"}
	object, err := LowerObjectBytes(ready, llvmobject.TargetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(object) < 4 || !bytes.Equal(object[:4], []byte{0xcf, 0xfa, 0xed, 0xfe}) {
		t.Fatalf("Darwin lowering did not emit a 64-bit Mach-O object: %x", object[:min(len(object), 8)])
	}
}
