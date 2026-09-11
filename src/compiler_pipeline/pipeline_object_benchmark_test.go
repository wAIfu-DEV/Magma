//go:build llvm_object

package compilerpipeline

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"Magma/src/clang"
	llvmobject "Magma/src/llvm_object"
	"Magma/src/shared"
)

var benchmarkLoweringBytes []byte

// BenchmarkWholeProgramBaseline measures complete compiler-library work from a
// fresh SharedState through lowering. Native linking is intentionally measured
// by benchmarks/incremental_baseline.sh so this benchmark remains useful for
// attribution and allocation comparisons.
func BenchmarkWholeProgramBaseline(b *testing.B) {
	repository, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		b.Fatal(err)
	}
	root := filepath.Join(repository, "tests", "incremental_baseline", "main.mg")
	stdRoot := filepath.Join(repository, "std")

	b.Run("textual", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			program := baselineCheckedProgram(b, repository, stdRoot, root)
			result, err := LowerReachable(program)
			if err != nil {
				b.Fatal(err)
			}
			benchmarkLoweringBytes = result
		}
	})
	b.Run("object", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			program := baselineCheckedProgram(b, repository, stdRoot, root)
			result, err := LowerObjectBytes(program, llvmobject.TargetOptions{Optimization: llvmobject.OptimizationNone})
			if err != nil {
				b.Fatal(err)
			}
			benchmarkLoweringBytes = result
		}
	})
}

func baselineCheckedProgram(b *testing.B, repository, stdRoot, root string) SafetyCheckedProgram {
	b.Helper()
	state, err := shared.MakeShared(repository, stdRoot)
	if err != nil {
		b.Fatal(err)
	}
	parsed, err := Parse(state, root)
	if err != nil {
		b.Fatal(err)
	}
	if err := RequireMainModule(parsed, root); err != nil {
		b.Fatal(err)
	}
	specialized, err := Specialize(parsed)
	if err != nil {
		b.Fatal(err)
	}
	linked, err := Link(specialized)
	if err != nil {
		b.Fatal(err)
	}
	typed, err := CheckTypes(linked)
	if err != nil {
		b.Fatal(err)
	}
	validated, err := ValidateLowering(typed)
	if err != nil {
		b.Fatal(err)
	}
	ready, err := CheckSafety(validated, false)
	if err != nil {
		b.Fatal(err)
	}
	return ready
}

// BenchmarkBackendCompilation compares only backend work. Parsing,
// specialization, type checking, and ownership checking are performed once
// before the timer because both backends consume the same checked program.
func BenchmarkBackendCompilation(b *testing.B) {
	program := benchmarkCheckedProgram(b)
	clangPath, _, err := clang.Resolve("")
	if err != nil {
		b.Fatalf("resolve Clang used by textual lowering: %v", err)
	}
	directory := b.TempDir()
	irPath := filepath.Join(directory, "program.ll")
	objectPath := filepath.Join(directory, "program.o")
	executablePath := filepath.Join(directory, "program")

	b.Run("module/textual", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			result, err := LowerReachable(program)
			if err != nil {
				b.Fatal(err)
			}
			benchmarkLoweringBytes = result
		}
	})
	b.Run("module/object", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			result, err := LowerObjectIR(program)
			if err != nil {
				b.Fatal(err)
			}
			benchmarkLoweringBytes = result
		}
	})
	b.Run("native-object/textual-via-clang", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			result, err := LowerReachable(program)
			if err != nil {
				b.Fatal(err)
			}
			oracle := textualIRWithRecordedNativeDeclarations(program, result)
			if err := os.WriteFile(irPath, oracle, 0o600); err != nil {
				b.Fatal(err)
			}
			if output, err := exec.Command(clangPath, "-O0", "-c", "-x", "ir", irPath, "-o", objectPath).CombinedOutput(); err != nil {
				b.Fatalf("compile textual IR: %v\n%s", err, output)
			}
		}
	})
	b.Run("native-object/object", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			result, err := LowerObjectBytes(program, llvmobject.TargetOptions{Optimization: llvmobject.OptimizationNone})
			if err != nil {
				b.Fatal(err)
			}
			if err := os.WriteFile(objectPath, result, 0o600); err != nil {
				b.Fatal(err)
			}
			benchmarkLoweringBytes = result
		}
	})
	b.Run("executable/textual-via-clang", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			result, err := LowerReachable(program)
			if err != nil {
				b.Fatal(err)
			}
			oracle := textualIRWithRecordedNativeDeclarations(program, result)
			if err := os.WriteFile(irPath, oracle, 0o600); err != nil {
				b.Fatal(err)
			}
			if output, err := exec.Command(clangPath, "-O0", "-x", "ir", irPath, "-o", executablePath).CombinedOutput(); err != nil {
				b.Fatalf("compile and link textual IR: %v\n%s", err, output)
			}
		}
	})
	b.Run("executable/object", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			result, err := LowerObjectBytes(program, llvmobject.TargetOptions{Optimization: llvmobject.OptimizationNone, PIC: true})
			if err != nil {
				b.Fatal(err)
			}
			if err := os.WriteFile(objectPath, result, 0o600); err != nil {
				b.Fatal(err)
			}
			if output, err := exec.Command(clangPath, objectPath, "-o", executablePath).CombinedOutput(); err != nil {
				b.Fatalf("link object-built program: %v\n%s", err, output)
			}
			benchmarkLoweringBytes = result
		}
	})
}

func textualIRWithRecordedNativeDeclarations(program SafetyCheckedProgram, ir []byte) []byte {
	state := program.State()
	state.NativeDeclarationsM.Lock()
	symbols := make([]string, 0, len(state.NativeDeclarations))
	for symbol := range state.NativeDeclarations {
		symbols = append(symbols, symbol)
	}
	sort.Strings(symbols)
	var declarations strings.Builder
	text := string(ir)
	for _, symbol := range symbols {
		declaration := state.NativeDeclarations[symbol]
		if strings.Contains(text, strings.TrimSpace(declaration)) {
			continue
		}
		declarations.WriteString(declaration)
	}
	state.NativeDeclarationsM.Unlock()
	declarations.Write(ir)
	return []byte(declarations.String())
}

func benchmarkCheckedProgram(b *testing.B) SafetyCheckedProgram {
	b.Helper()
	repository, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		b.Fatal(err)
	}
	path := filepath.Join(repository, "std", "tests", "hash_map.mg")
	state, err := shared.MakeShared(repository, filepath.Join(repository, "std"))
	if err != nil {
		b.Fatal(err)
	}
	// Keep this benchmark focused on lowering and code generation. The null
	// context avoids allocator/native-library declarations that are linked by
	// the production driver but are unrelated to either backend's throughput.
	state.NullContext = true
	parsed, err := Parse(state, path)
	if err != nil {
		b.Fatal(err)
	}
	specialized, err := Specialize(parsed)
	if err != nil {
		b.Fatal(err)
	}
	linked, err := Link(specialized)
	if err != nil {
		b.Fatal(err)
	}
	typed, err := CheckTypes(linked)
	if err != nil {
		b.Fatal(err)
	}
	validated, err := ValidateLowering(typed)
	if err != nil {
		b.Fatal(err)
	}
	ready, err := CheckSafety(validated, false)
	if err != nil {
		b.Fatal(err)
	}
	return ready
}
