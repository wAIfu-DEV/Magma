package clang

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

var versionPattern = regexp.MustCompile(`(?i)clang version\s+([0-9]+(?:\.[0-9]+)*)`)

// Resolve finds a Clang executable and verifies its reported version. An empty
// requestedVersion accepts any version. MAGMA_CLANG has highest priority and
// may name either an executable or its absolute path.
func Resolve(requestedVersion string) (string, string, error) {
	requestedVersion = strings.TrimSpace(strings.TrimPrefix(requestedVersion, "v"))
	if requestedVersion != "" && !regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)*$`).MatchString(requestedVersion) {
		return "", "", fmt.Errorf("invalid Clang version %q (expected e.g. 18 or 18.1)", requestedVersion)
	}

	candidates := candidateNames(requestedVersion)
	var checked []string
	var mismatched []string
	seen := map[string]bool{}
	for _, candidate := range candidates {
		path, err := exec.LookPath(candidate)
		if err != nil {
			if filepath.IsAbs(candidate) {
				checked = append(checked, candidate)
			}
			continue
		}
		path, _ = filepath.Abs(path)
		key := strings.ToLower(filepath.Clean(path))
		if seen[key] {
			continue
		}
		seen[key] = true
		checked = append(checked, path)
		version, err := Version(path)
		if err != nil {
			mismatched = append(mismatched, fmt.Sprintf("%s (%v)", path, err))
			continue
		}
		if requestedVersion != "" && version != requestedVersion && !strings.HasPrefix(version, requestedVersion+".") {
			mismatched = append(mismatched, fmt.Sprintf("%s (version %s)", path, version))
			continue
		}
		return path, version, nil
	}

	detail := ""
	if len(mismatched) > 0 {
		detail = "; rejected: " + strings.Join(mismatched, ", ")
	}
	return "", "", fmt.Errorf("could not find a usable Clang%s; set MAGMA_CLANG to the executable or add it to PATH%s (checked %d candidates)", requestedLabel(requestedVersion), detail, len(checked))
}

func Version(path string) (string, error) {
	out, err := exec.Command(path, "--version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to run --version: %w", err)
	}
	match := versionPattern.FindSubmatch(out)
	if len(match) != 2 {
		return "", fmt.Errorf("unrecognized --version output")
	}
	return string(match[1]), nil
}

func candidateNames(version string) []string {
	var names []string
	if configured := strings.TrimSpace(os.Getenv("MAGMA_CLANG")); configured != "" {
		names = append(names, configured)
	}
	if version != "" {
		major := strings.Split(version, ".")[0]
		names = append(names, "clang-"+version, "clang"+version)
		if major != version {
			names = append(names, "clang-"+major, "clang"+major)
		}
	}
	names = append(names, "clang")

	for _, env := range []string{"LLVM_HOME", "LLVM_PATH"} {
		if root := strings.TrimSpace(os.Getenv(env)); root != "" {
			names = append(names, root, filepath.Join(root, executableName()), filepath.Join(root, "bin", executableName()))
		}
	}
	if runtime.GOOS == "windows" {
		for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)"} {
			if root := os.Getenv(env); root != "" {
				names = append(names, filepath.Join(root, "LLVM", "bin", "clang.exe"))
				patterns := []string{
					filepath.Join(root, "Microsoft Visual Studio", "*", "*", "VC", "Tools", "Llvm", "x64", "bin", "clang.exe"),
					filepath.Join(root, "Microsoft Visual Studio", "*", "*", "VC", "Tools", "Llvm", "bin", "clang.exe"),
				}
				for _, pattern := range patterns {
					matches, _ := filepath.Glob(pattern)
					sort.Sort(sort.Reverse(sort.StringSlice(matches)))
					names = append(names, matches...)
				}
			}
		}
		if chocolatey := os.Getenv("ChocolateyInstall"); chocolatey != "" {
			names = append(names, filepath.Join(chocolatey, "bin", "clang.exe"))
		}
	} else {
		for _, root := range []string{"/usr/bin", "/usr/local/bin", "/opt/homebrew/opt/llvm/bin", "/usr/local/opt/llvm/bin", "/opt/llvm/bin"} {
			if version != "" {
				names = append(names, filepath.Join(root, "clang-"+strings.Split(version, ".")[0]))
			}
			names = append(names, filepath.Join(root, "clang"))
		}
	}
	return names
}

func executableName() string {
	if runtime.GOOS == "windows" {
		return "clang.exe"
	}
	return "clang"
}

func requestedLabel(version string) string {
	if version == "" {
		return ""
	}
	return " version " + version
}
