package types

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModuleIdentityCanonicalizesSymlinkAliases(t *testing.T) {
	root := t.TempDir()
	realDirectory := filepath.Join(root, "real")
	if err := os.Mkdir(realDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	realPath := filepath.Join(realDirectory, "library.mg")
	if err := os.WriteFile(realPath, []byte("mod library\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	aliasPath := filepath.Join(root, "alias.mg")
	if err := os.Symlink(realPath, aliasPath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	realID, err := ResolveModuleID(realPath, root, filepath.Join(root, "std"))
	if err != nil {
		t.Fatal(err)
	}
	aliasID, err := ResolveModuleID(aliasPath, root, filepath.Join(root, "std"))
	if err != nil {
		t.Fatal(err)
	}
	if realID != aliasID {
		t.Fatalf("symlink aliases identify one backing file differently: %q != %q", realID, aliasID)
	}
}

func TestWorkspaceModuleIdentitySurvivesRelocation(t *testing.T) {
	first, err := ResolveModuleID(filepath.Join(t.TempDir(), "placeholder"), "/unused", "/std")
	if err != nil || !strings.Contains(string(first), ":external:") {
		t.Fatalf("external identity = %q, %v", first, err)
	}

	leftRoot := filepath.Join(t.TempDir(), "left")
	rightRoot := filepath.Join(t.TempDir(), "right")
	left, err := ResolveModuleID(filepath.Join(leftRoot, "src", "net.mg"), leftRoot, "/std")
	if err != nil {
		t.Fatal(err)
	}
	right, err := ResolveModuleID(filepath.Join(rightRoot, "src", "net.mg"), rightRoot, "/std")
	if err != nil {
		t.Fatal(err)
	}
	if left != right {
		t.Fatalf("relocated identities differ: %q != %q", left, right)
	}
}

func TestStablePackageNameUsesBackingIdentityNotDeclaredNameAlone(t *testing.T) {
	first := ModuleID("magma-module-v1:workspace:first/library.mg")
	second := ModuleID("magma-module-v1:workspace:second/library.mg")
	firstName := StablePackageName("shared", first)
	if again := StablePackageName("shared", first); again != firstName {
		t.Fatalf("stable package name changed: %q != %q", firstName, again)
	}
	if secondName := StablePackageName("shared", second); secondName == firstName {
		t.Fatalf("different backing identities collided at %q", firstName)
	}
	if !strings.HasPrefix(firstName, "shared_") || len(strings.TrimPrefix(firstName, "shared_")) != 10 {
		t.Fatalf("package name %q does not preserve the ten-character suffix shape", firstName)
	}
}

func TestExternalBackingPathsRemainDistinctWithoutPackageManifest(t *testing.T) {
	workspace := t.TempDir()
	firstRoot := t.TempDir()
	secondRoot := t.TempDir()
	first, err := ResolveModuleID(filepath.Join(firstRoot, "src", "library.mg"), workspace, filepath.Join(workspace, "std"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := ResolveModuleID(filepath.Join(secondRoot, "src", "library.mg"), workspace, filepath.Join(workspace, "std"))
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("different external backing files have equal identity %q", first)
	}
}

func TestStandardLibraryIdentityIsDistinctFromWorkspaceIdentity(t *testing.T) {
	root := t.TempDir()
	stdRoot := filepath.Join(root, "std")
	path := filepath.Join(stdRoot, "core.mg")
	standard, err := ResolveModuleID(path, root, stdRoot)
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := ResolveModuleID(filepath.Join(root, "core.mg"), root, stdRoot)
	if err != nil {
		t.Fatal(err)
	}
	if standard == workspace || !strings.Contains(string(standard), ":std:") || !strings.Contains(string(workspace), ":workspace:") {
		t.Fatalf("standard=%q workspace=%q", standard, workspace)
	}
}

func TestDeclarationIdentityIncludesModuleKindAndName(t *testing.T) {
	base := DeclarationID{Module: "module", Kind: "function", Name: "open"}.String()
	for _, other := range []DeclarationID{
		{Module: "other", Kind: "function", Name: "open"},
		{Module: "module", Kind: "global", Name: "open"},
		{Module: "module", Kind: "function", Name: "close"},
	} {
		if other.String() == base {
			t.Fatalf("declaration identity collision with %#v", other)
		}
	}
}
