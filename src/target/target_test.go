package target

import "testing"

func TestI386UsesNativePlatform(t *testing.T) {
	got := replaceArch("x86_64-pc-windows-msvc", canonicalArch("i386"))
	if got != "i386-pc-windows-msvc" {
		t.Fatalf("i386 target = %q", got)
	}
}

func TestTripleProperties(t *testing.T) {
	got := fromTriple("i386-unknown-linux-gnu")
	if got.Arch != "i386" || got.OS != "linux" || got.ABI != "gnu" || got.PointerBits != 32 || got.Endian != LittleEndian {
		t.Fatalf("unexpected target: %+v", got)
	}
}

func TestBigEndianTriple(t *testing.T) {
	got := fromTriple("s390x-unknown-linux-gnu")
	if got.Endian != BigEndian || got.PointerBits != 64 {
		t.Fatalf("unexpected target: %+v", got)
	}
}

func TestFallbackCABIMemoryModels(t *testing.T) {
	windows64 := HostFallback("windows", "amd64").CompilerKnownTypes
	if windows64["c.long"] != "i32" || windows64["c.size_t"] != "u64" || windows64["c.wchar_t"] != "u16" {
		t.Fatalf("unexpected Windows LLP64 types: %v", windows64)
	}
	linux64 := HostFallback("linux", "amd64").CompilerKnownTypes
	if linux64["c.long"] != "i64" || linux64["c.size_t"] != "u64" || linux64["c.wchar_t"] != "i32" {
		t.Fatalf("unexpected Linux LP64 types: %v", linux64)
	}
	linux32 := HostFallback("linux", "386").CompilerKnownTypes
	if linux32["c.long"] != "i32" || linux32["c.size_t"] != "u32" || linux32["c.intptr_t"] != "i32" {
		t.Fatalf("unexpected i386 ILP32 types: %v", linux32)
	}
}
