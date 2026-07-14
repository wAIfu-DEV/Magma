package clang

import "testing"

func TestVersionPattern(t *testing.T) {
	match := versionPattern.FindStringSubmatch("clang version 21.1.7 (release build)")
	if len(match) != 2 || match[1] != "21.1.7" {
		t.Fatalf("unexpected match: %q", match)
	}
}

func TestCandidateNamesStartsWithConfiguredClang(t *testing.T) {
	t.Setenv("MAGMA_CLANG", "C:/custom/clang.exe")
	candidates := candidateNames("")
	if len(candidates) == 0 || candidates[0] != "C:/custom/clang.exe" {
		t.Fatalf("first candidate = %q", candidates[0])
	}
}
