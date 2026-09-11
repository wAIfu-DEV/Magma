package llvmir_test

import (
	"Magma/src/clang"
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

// BenchmarkEmbeddedPayloadC23 measures a target-independent asset-object path:
// Clang's C23 #embed reads each payload and emits the target's native object
// format. The surrounding setup and link are identical to the other variants.
func BenchmarkEmbeddedPayloadC23(b *testing.B) {
	clangPath, _, err := clang.Resolve("")
	if err != nil {
		b.Skipf("Clang is required: %v", err)
	}

	megabytes := embeddedBenchmarkMegabytes(b)
	bytesPerAsset := int64(megabytes) * 1024 * 1024
	for _, test := range embeddedBenchmarkCases() {
		b.Run(test.name, func(b *testing.B) {
			caseBytesPerAsset := bytesPerAsset / int64(test.sizeDivisor)
			directory := b.TempDir()
			programObject := filepath.Join(directory, "program.o")
			assetObject := filepath.Join(directory, "assets.o")
			assetSource := filepath.Join(directory, "assets.c")
			executablePath := filepath.Join(directory, "benchmark-exe")
			payloads := createBenchmarkPayloads(b, directory, test.assetCount, caseBytesPerAsset)
			if err := writeC23EmbedSource(assetSource, payloads); err != nil {
				b.Fatal(err)
			}
			programIR := filepath.Join(directory, "program.ll")
			if err := os.WriteFile(programIR, []byte("define i32 @main() { ret i32 0 }\n"), 0600); err != nil {
				b.Fatal(err)
			}
			if output, err := exec.Command(clangPath, "-O0", "-c", programIR, "-o", programObject).CombinedOutput(); err != nil {
				b.Fatalf("compile program object: %v\n%s", err, output)
			}

			b.ReportMetric(float64(test.assetCount)*float64(caseBytesPerAsset)/(1024*1024), "payload-MiB")
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				linkArgs := []string{programObject}
				if test.assetCount > 0 {
					if output, err := exec.Command(clangPath, "-std=c23", "-O0", "-c", assetSource, "-o", assetObject).CombinedOutput(); err != nil {
						b.Fatalf("create C23 asset object (Clang may lack #embed support): %v\n%s", err, output)
					}
					linkArgs = append(linkArgs, assetObject)
				}
				linkArgs = append(linkArgs, "-o", executablePath)
				if output, err := exec.Command(clangPath, linkArgs...).CombinedOutput(); err != nil {
					b.Fatalf("link objects: %v\n%s", err, output)
				}
			}
			b.StopTimer()
			verifyEmbeddedOutputs(b, assetObject, executablePath, int64(test.assetCount)*caseBytesPerAsset)
		})
	}
}

func createBenchmarkPayloads(tb testing.TB, directory string, count int, bytesPerAsset int64) []string {
	tb.Helper()
	payloads := make([]string, count)
	for i := range payloads {
		payloads[i] = filepath.Join(directory, fmt.Sprintf("asset-%d.bin", i))
		file, err := os.Create(payloads[i])
		if err != nil {
			tb.Fatal(err)
		}
		if err := file.Truncate(bytesPerAsset); err != nil {
			file.Close()
			tb.Fatal(err)
		}
		if err := file.Close(); err != nil {
			tb.Fatal(err)
		}
	}
	return payloads
}

func writeC23EmbedSource(path string, payloads []string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	writer := bufio.NewWriter(file)
	for i, payload := range payloads {
		if _, err = fmt.Fprintf(writer, "const unsigned char magma_asset_%d[] = {\n#embed %s\n};\n", i, strconv.Quote(filepath.ToSlash(payload))); err != nil {
			break
		}
	}
	if flushErr := writer.Flush(); err == nil {
		err = flushErr
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}

// BenchmarkEmbeddedPayloadObjects measures the complete alternative path in
// flat, directly comparable rows: create one asset object with .incbin, then
// link it with an already-built program object. With no assets, it measures
// only the program-object link. Fixture creation is outside the timed region.
func BenchmarkEmbeddedPayloadObjects(b *testing.B) {
	clangPath, _, err := clang.Resolve("")
	if err != nil {
		b.Skipf("Clang is required: %v", err)
	}
	if runtime.GOOS == "windows" {
		b.Skip("the .incbin benchmark fixture is currently Unix-only")
	}

	megabytes := embeddedBenchmarkMegabytes(b)
	bytesPerAsset := int64(megabytes) * 1024 * 1024
	for _, test := range embeddedBenchmarkCases() {
		b.Run(test.name, func(b *testing.B) {
			caseBytesPerAsset := bytesPerAsset / int64(test.sizeDivisor)
			directory := b.TempDir()
			programObject := filepath.Join(directory, "program.o")
			assetObject := filepath.Join(directory, "assets.o")
			assemblyPath := filepath.Join(directory, "assets.s")
			executablePath := filepath.Join(directory, "benchmark-exe")
			payloads := createBenchmarkPayloads(b, directory, test.assetCount, caseBytesPerAsset)
			if err := writeIncbinAssembly(assemblyPath, payloads); err != nil {
				b.Fatal(err)
			}
			programIR := filepath.Join(directory, "program.ll")
			if err := os.WriteFile(programIR, []byte("define i32 @main() { ret i32 0 }\n"), 0600); err != nil {
				b.Fatal(err)
			}
			if output, err := exec.Command(clangPath, "-O0", "-c", programIR, "-o", programObject).CombinedOutput(); err != nil {
				b.Fatalf("compile program object: %v\n%s", err, output)
			}

			b.ReportMetric(float64(test.assetCount)*float64(caseBytesPerAsset)/(1024*1024), "payload-MiB")
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				linkArgs := []string{programObject}
				if test.assetCount > 0 {
					if output, err := exec.Command(clangPath, "-c", assemblyPath, "-o", assetObject).CombinedOutput(); err != nil {
						b.Fatalf("create asset object: %v\n%s", err, output)
					}
					linkArgs = append(linkArgs, assetObject)
				}
				linkArgs = append(linkArgs, "-o", executablePath)
				if output, err := exec.Command(clangPath, linkArgs...).CombinedOutput(); err != nil {
					b.Fatalf("link objects: %v\n%s", err, output)
				}
			}
			b.StopTimer()
			verifyEmbeddedOutputs(b, assetObject, executablePath, int64(test.assetCount)*caseBytesPerAsset)
		})
	}
}

type embeddedBenchmarkCase struct {
	name        string
	assetCount  int
	sizeDivisor int
}

func embeddedBenchmarkCases() []embeddedBenchmarkCase {
	return []embeddedBenchmarkCase{
		{name: "none", assetCount: 0, sizeDivisor: 1},
		{name: "one_large_file", assetCount: 1, sizeDivisor: 1},
		{name: "four_files_same_total", assetCount: 4, sizeDivisor: 4},
		{name: "four_large_files", assetCount: 4, sizeDivisor: 1},
	}
}

func embeddedBenchmarkMegabytes(tb testing.TB) int {
	tb.Helper()
	megabytes := 32
	if value := os.Getenv("MAGMA_EMBED_BENCH_MB"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 {
			tb.Fatalf("MAGMA_EMBED_BENCH_MB must be a positive integer, got %q", value)
		}
		megabytes = parsed
	}
	return megabytes
}

func writeIncbinAssembly(path string, payloads []string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	writer := bufio.NewWriter(file)
	if runtime.GOOS == "darwin" {
		_, err = fmt.Fprintln(writer, ".section __DATA,__const")
	} else {
		_, err = fmt.Fprintln(writer, ".section .rodata.magma_embed,\"a\",@progbits")
	}
	if err == nil {
		for i, payload := range payloads {
			// %q produces a quoted, escaped assembly string on supported hosts.
			if _, err = fmt.Fprintf(writer, ".globl magma_asset_%d_start\nmagma_asset_%d_start:\n.incbin %q\n.globl magma_asset_%d_end\nmagma_asset_%d_end:\n", i, i, payload, i, i); err != nil {
				break
			}
		}
	}
	if flushErr := writer.Flush(); err == nil {
		err = flushErr
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}

// BenchmarkEmbeddedPayloadIR measures the complete direct-IR alternative. It
// compiles the payload IR to an asset object and links it with the same kind of
// prebuilt program object used by BenchmarkEmbeddedPayloadObjects. With no
// assets, it measures only the program-object link. IR construction and file
// writing happen before the timer starts.
//
// The default payload is 32 MiB per asset. Override it for quicker smoke tests
// or larger production-like inputs with MAGMA_EMBED_BENCH_MB.
func BenchmarkEmbeddedPayloadIR(b *testing.B) {
	clangPath, _, err := clang.Resolve("")
	if err != nil {
		b.Skipf("Clang is required: %v", err)
	}

	megabytes := embeddedBenchmarkMegabytes(b)
	bytesPerAsset := int64(megabytes) * 1024 * 1024

	for _, test := range embeddedBenchmarkCases() {
		b.Run(test.name, func(b *testing.B) {
			caseBytesPerAsset := bytesPerAsset / int64(test.sizeDivisor)
			directory := b.TempDir()
			irPath := filepath.Join(directory, "payload.ll")
			assetObject := filepath.Join(directory, "payload.o")
			programIR := filepath.Join(directory, "program.ll")
			programObject := filepath.Join(directory, "program.o")
			executablePath := filepath.Join(directory, "benchmark-exe")
			if err := writePayloadBenchmarkIR(irPath, test.assetCount, caseBytesPerAsset); err != nil {
				b.Fatal(err)
			}
			if err := os.WriteFile(programIR, []byte("define i32 @main() { ret i32 0 }\n"), 0600); err != nil {
				b.Fatal(err)
			}
			if output, err := exec.Command(clangPath, "-O0", "-c", programIR, "-o", programObject).CombinedOutput(); err != nil {
				b.Fatalf("compile program object: %v\n%s", err, output)
			}
			info, err := os.Stat(irPath)
			if err != nil {
				b.Fatal(err)
			}

			b.ReportMetric(float64(test.assetCount)*float64(caseBytesPerAsset)/(1024*1024), "payload-MiB")
			b.ReportMetric(float64(info.Size())/(1024*1024), "IR-MiB")
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				linkArgs := []string{programObject}
				if test.assetCount > 0 {
					command := exec.Command(clangPath, "-Wno-override-module", "-O0", "-c", irPath, "-o", assetObject)
					if output, err := command.CombinedOutput(); err != nil {
						b.Fatalf("create asset object: %v\n%s", err, output)
					}
					linkArgs = append(linkArgs, assetObject)
				}
				linkArgs = append(linkArgs, "-o", executablePath)
				if output, err := exec.Command(clangPath, linkArgs...).CombinedOutput(); err != nil {
					b.Fatalf("link objects: %v\n%s", err, output)
				}
			}
			b.StopTimer()
			verifyEmbeddedOutputs(b, assetObject, executablePath, int64(test.assetCount)*caseBytesPerAsset)
		})
	}
}

func verifyEmbeddedOutputs(b *testing.B, assetObject, executable string, payloadBytes int64) {
	b.Helper()
	if payloadBytes > 0 {
		info, err := os.Stat(assetObject)
		if err != nil {
			b.Fatalf("inspect asset object: %v", err)
		}
		if info.Size() < payloadBytes {
			b.Fatalf("asset object is only %d bytes for a %d-byte payload; section may have been discarded", info.Size(), payloadBytes)
		}
		b.ReportMetric(float64(info.Size())/(1024*1024), "object-MiB")
	}
	info, err := os.Stat(executable)
	if err != nil {
		b.Fatalf("inspect linked executable: %v", err)
	}
	if info.Size() < payloadBytes {
		b.Fatalf("executable is only %d bytes for a %d-byte payload; section may have been discarded", info.Size(), payloadBytes)
	}
	b.ReportMetric(float64(info.Size())/(1024*1024), "exe-MiB")
}

func writePayloadBenchmarkIR(path string, assetCount int, bytesPerAsset int64) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	writer := bufio.NewWriterSize(file, 1024*1024)
	if _, err := fmt.Fprintln(writer, "; embedded-payload parsing benchmark"); err != nil {
		file.Close()
		return err
	}

	for asset := 0; asset < assetCount; asset++ {
		if _, err := fmt.Fprintf(writer, "@asset.%d = constant [%d x i8] c\"", asset, bytesPerAsset); err != nil {
			file.Close()
			return err
		}
		// Escaped bytes model arbitrary binary data. Each payload byte occupies
		// three textual IR bytes (for example, \A5), which is the relevant cost.
		const chunkBytes = 256
		chunk := make([]byte, 0, chunkBytes*3)
		for offset := 0; offset < chunkBytes; offset++ {
			chunk = fmt.Appendf(chunk, "\\%02X", byte(offset+asset))
		}
		for remaining := bytesPerAsset; remaining > 0; {
			count := int64(chunkBytes)
			if remaining < count {
				count = remaining
			}
			if _, err := writer.Write(chunk[:count*3]); err != nil {
				file.Close()
				return err
			}
			remaining -= count
		}
		if _, err := fmt.Fprintln(writer, "\""); err != nil {
			file.Close()
			return err
		}
	}
	if err := writer.Flush(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return nil
}
