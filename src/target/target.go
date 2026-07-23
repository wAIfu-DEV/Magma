package target

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

type Arch string
type OS string
type ABI string
type Endian string

const (
	LittleEndian Endian = "little"
	BigEndian    Endian = "big"
)

// Target describes the machine for which Magma is generating code. Triple and
// DataLayout are deliberately retained verbatim: LLVM, rather than Magma, is
// the authority on the complete set of machines and their layouts.
type Target struct {
	Triple      string
	DataLayout  string
	Arch        Arch
	OS          OS
	ABI         ABI
	Endian      Endian
	PointerBits int
	// CompilerKnownTypes maps stable compiler-owned names to concrete Magma
	// types. It is consumed by @compiler_known_type in trusted library aliases.
	CompilerKnownTypes map[string]string
}

// HostFallback keeps library users and unit tests that construct a SharedState
// directly compatible. The command-line compiler replaces this with Clang's
// authoritative target before compilation begins.
func HostFallback(goos, goarch string) Target {
	arch := canonicalArch(goarch)
	t := Target{Arch: Arch(arch), OS: OS(goos), Endian: LittleEndian}
	switch arch {
	case "i386", "arm", "wasm32", "riscv32":
		t.PointerBits = 32
	default:
		t.PointerBits = 64
	}
	t.CompilerKnownTypes = fallbackCTypes(goos, arch, t.PointerBits)
	return t
}

var moduleLayout = regexp.MustCompile(`(?m)^target datalayout = "([^"]+)"`)
var moduleTriple = regexp.MustCompile(`(?m)^target triple = "([^"]+)"`)

// Resolve asks Clang to select and canonicalize a target. An empty request
// selects Clang's native target. Architecture-only requests (notably i386) use
// the native OS and ABI while replacing its architecture.
func Resolve(clangPath, requested string) (Target, error) {
	native, err := dumpMachine(clangPath, "")
	if err != nil {
		return Target{}, err
	}
	triple := requested
	if triple == "" || triple == "native" {
		triple = native
	} else if !strings.Contains(triple, "-") {
		triple = replaceArch(native, canonicalArch(triple))
	}

	cmd := exec.Command(clangPath, "--target="+triple, "-S", "-emit-llvm", "-x", "c", "-", "-o", "-")
	cmd.Stdin = strings.NewReader("")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return Target{}, fmt.Errorf("resolve target %q with Clang: %w: %s", triple, err, strings.TrimSpace(stderr.String()))
	}
	if match := moduleTriple.FindSubmatch(out); len(match) == 2 {
		triple = string(match[1])
	}
	layout := ""
	if match := moduleLayout.FindSubmatch(out); len(match) == 2 {
		layout = string(match[1])
	}

	result := fromTriple(triple)
	result.DataLayout = layout
	known, err := queryCompilerKnownTypes(clangPath, triple)
	if err != nil {
		return Target{}, err
	}
	result.CompilerKnownTypes = known
	return result, nil
}

func queryCompilerKnownTypes(clangPath, triple string) (map[string]string, error) {
	cmd := exec.Command(clangPath, "--target="+triple, "-dM", "-E", "-x", "c", "-")
	cmd.Stdin = strings.NewReader("")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("query compiler-known types for target %q: %w: %s", triple, err, strings.TrimSpace(string(out)))
	}
	macros := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == "#define" {
			macros[fields[1]] = strings.Join(fields[2:], " ")
		}
	}
	bytesOf := func(name string, fallback int) int {
		var value int
		if _, scanErr := fmt.Sscanf(macros[name], "%d", &value); scanErr == nil && value > 0 {
			return value
		}
		return fallback
	}
	pointerBytes := bytesOf("__SIZEOF_POINTER__", 8)
	known := makeCTypes(
		bytesOf("__SIZEOF_SHORT__", 2),
		bytesOf("__SIZEOF_INT__", 4),
		bytesOf("__SIZEOF_LONG__", 4),
		bytesOf("__SIZEOF_LONG_LONG__", 8),
		bytesOf("__SIZEOF_SIZE_T__", pointerBytes),
		bytesOf("__SIZEOF_PTRDIFF_T__", pointerBytes),
		bytesOf("__SIZEOF_WCHAR_T__", 4),
		strings.Contains(macros["__CHAR_UNSIGNED__"], "1"),
		strings.Contains(macros["__WCHAR_TYPE__"], "unsigned"),
	)
	return known, nil
}

func magmaInteger(bytes int, signed bool) string {
	prefix := "u"
	if signed {
		prefix = "i"
	}
	return fmt.Sprintf("%s%d", prefix, bytes*8)
}

func makeCTypes(short, intSize, long, longLong, size, ptrdiff, wchar int, charUnsigned, wcharUnsigned bool) map[string]string {
	return map[string]string{
		"c.char":               magmaInteger(1, !charUnsigned),
		"c.signed_char":        "i8",
		"c.unsigned_char":      "u8",
		"c.short":              magmaInteger(short, true),
		"c.unsigned_short":     magmaInteger(short, false),
		"c.int":                magmaInteger(intSize, true),
		"c.unsigned_int":       magmaInteger(intSize, false),
		"c.long":               magmaInteger(long, true),
		"c.unsigned_long":      magmaInteger(long, false),
		"c.long_long":          magmaInteger(longLong, true),
		"c.unsigned_long_long": magmaInteger(longLong, false),
		"c.size_t":             magmaInteger(size, false),
		"c.ptrdiff_t":          magmaInteger(ptrdiff, true),
		"c.intptr_t":           magmaInteger(ptrdiff, true),
		"c.uintptr_t":          magmaInteger(ptrdiff, false),
		"c.wchar_t":            magmaInteger(wchar, !wcharUnsigned),
	}
}

func fallbackCTypes(goos, arch string, pointerBits int) map[string]string {
	longBytes := pointerBits / 8
	if goos == "windows" || pointerBits == 32 {
		longBytes = 4
	}
	wcharBytes, wcharUnsigned := 4, false
	if goos == "windows" {
		wcharBytes, wcharUnsigned = 2, true
	}
	return makeCTypes(2, 4, longBytes, 8, pointerBits/8, pointerBits/8, wcharBytes, false, wcharUnsigned)
}

func dumpMachine(clangPath, triple string) (string, error) {
	args := []string{}
	if triple != "" {
		args = append(args, "--target="+triple)
	}
	args = append(args, "-dumpmachine")
	out, err := exec.Command(clangPath, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("query Clang target: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func canonicalArch(arch string) string {
	switch strings.ToLower(arch) {
	case "386", "x86", "i686", "i586", "i486", "i386":
		return "i386"
	case "amd64", "x64", "x86_64":
		return "x86_64"
	case "arm64", "aarch64":
		return "aarch64"
	default:
		return arch
	}
}

func replaceArch(native, arch string) string {
	parts := strings.Split(native, "-")
	if len(parts) == 0 {
		return arch
	}
	parts[0] = arch
	return strings.Join(parts, "-")
}

func fromTriple(triple string) Target {
	parts := strings.Split(strings.ToLower(triple), "-")
	t := Target{Triple: triple, Endian: LittleEndian}
	if len(parts) > 0 {
		t.Arch = Arch(parts[0])
	}
	for _, part := range parts[1:] {
		switch part {
		case "windows", "win32":
			t.OS = "windows"
		case "linux":
			t.OS = "linux"
		case "darwin", "macos":
			t.OS = "darwin"
		case "freebsd", "netbsd", "openbsd", "android", "ios":
			t.OS = OS(part)
		case "msvc", "gnu", "gnueabi", "gnueabihf", "musl", "eabi", "elf":
			t.ABI = ABI(part)
		}
	}
	switch t.Arch {
	case "i386", "i486", "i586", "i686", "arm", "thumb", "wasm32", "riscv32":
		t.PointerBits = 32
	case "x86_64", "aarch64", "wasm64", "riscv64", "ppc64", "ppc64le", "s390x":
		t.PointerBits = 64
	}
	if t.Arch == "s390x" || t.Arch == "ppc64" {
		t.Endian = BigEndian
	}
	return t
}
