package llvmir_test

import (
	clangresolver "Magma/src/clang"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// TestExportNameCInterop proves that an exported Magma definition can be
// declared and called as an ordinary C function. This deliberately compiles
// and links the generated LLVM rather than only inspecting its text.
func TestExportNameCInterop(t *testing.T) {
	clangPath, _, err := clangresolver.Resolve("")
	if err != nil {
		t.Skipf("Clang is required for the C interoperability test: %v", err)
	}

	ir, err := compileSource(t, `mod interop

@export_name("magma_add")
add(a i32, b i32) i32:
    ret a + b
..
`)
	if err != nil {
		t.Fatalf("compile Magma export: %v", err)
	}

	dir := t.TempDir()
	llvmPath := filepath.Join(dir, "magma_export.ll")
	cPath := filepath.Join(dir, "caller.c")
	exeName := "caller"
	if runtime.GOOS == "windows" {
		exeName += ".exe"
	}
	exePath := filepath.Join(dir, exeName)

	if err := os.WriteFile(llvmPath, []byte(ir), 0600); err != nil {
		t.Fatalf("write generated LLVM: %v", err)
	}
	const caller = `#include <stdint.h>

extern int32_t magma_add(int32_t a, int32_t b);

int main(void) {
    return magma_add(19, 23) == 42 ? 0 : 1;
}
`
	if err := os.WriteFile(cPath, []byte(caller), 0600); err != nil {
		t.Fatalf("write C caller: %v", err)
	}

	link := exec.Command(clangPath, "-Wno-override-module", llvmPath, cPath, "-o", exePath)
	if output, err := link.CombinedOutput(); err != nil {
		t.Fatalf("link Magma export with C caller: %v\n%s", err, output)
	}

	run := exec.Command(exePath)
	if output, err := run.CombinedOutput(); err != nil {
		t.Fatalf("C caller did not receive 42 from magma_add: %v\n%s", err, output)
	}
}

func TestExportInitializesContextOnForeignNativeThread(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pthread foreign-thread fixture is Unix-only")
	}
	clangPath, _, err := clangresolver.Resolve("")
	if err != nil {
		t.Skipf("Clang is required for the C interoperability test: %v", err)
	}
	ir, err := compileSource(t, `mod interop
@export_name("magma_add_foreign")
add(a i32, b i32) i32:
    ret a + b
..
`)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	llvmPath := filepath.Join(dir, "foreign.ll")
	cPath := filepath.Join(dir, "foreign.c")
	exePath := filepath.Join(dir, "foreign")
	if err := os.WriteFile(llvmPath, []byte(ir), 0600); err != nil {
		t.Fatal(err)
	}
	const caller = `#include <pthread.h>
#include <stdint.h>
extern int32_t magma_add_foreign(int32_t, int32_t);
static void *run(void *unused) {
    (void)unused;
    return (void *)(uintptr_t)(magma_add_foreign(20, 22) != 42);
}
int main(void) {
    pthread_t thread;
    void *result = 0;
    if (pthread_create(&thread, 0, run, 0) != 0) return 2;
    if (pthread_join(thread, &result) != 0) return 3;
    return (int)(uintptr_t)result;
}
`
	if err := os.WriteFile(cPath, []byte(caller), 0600); err != nil {
		t.Fatal(err)
	}
	link := exec.Command(clangPath, "-Wno-override-module", llvmPath, cPath, "-pthread", "-o", exePath)
	if output, err := link.CombinedOutput(); err != nil {
		t.Fatalf("link foreign-thread fixture: %v\n%s", err, output)
	}
	if output, err := exec.Command(exePath).CombinedOutput(); err != nil {
		t.Fatalf("foreign-created thread failed to receive a context: %v\n%s", err, output)
	}
}
