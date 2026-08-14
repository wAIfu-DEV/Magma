package main

import (
	clangresolver "Magma/src/clang"
	"Magma/src/comp_err"
	compilerpipeline "Magma/src/compiler_pipeline"
	"Magma/src/debug"
	"Magma/src/lsp"
	"Magma/src/makeabs"
	"Magma/src/shared"
	magmatarget "Magma/src/target"
	"Magma/src/types"
	_ "embed"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

//go:embed VERSION.txt
var compilerVersionText string

const usage = `usage: magma [options] <input-file>

options:
  --debug                 print compiler diagnostics
  --version, -v           print the compiler version
  --out, -o <path>        output path (default depends on --emit)
  --emit, -e <kind>       llvm, object, or exe (default llvm)
  --opt, -O <0-3>         LLVM optimization level (default 3)
  --error-trace-slots <n> trace slots per runtime shard (default 1024)
  --safety-warnings       downgrade memory-safety diagnostics to warnings
  --target <triple>       compilation target (default: Clang native target)
  --std <directory>       override the Magma standard-library directory
  --lsp                   run the Magma language server over stdio
  --clang-version, -cv    print the resolved Clang version and path`

type options struct {
	inputFile       string
	debug           bool
	version         bool
	out             string
	emit            string
	opt             int
	errorTraceSlots uint64
	safetyWarnings  bool
	clangVersion    bool
	target          string
	targetOS        string
	stdRoot         string
	lsp             bool
}

func parseArgs(args []string) (options, error) {
	var opts options
	args = normalizeArgs(args)
	flags := flag.NewFlagSet("magma", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.BoolVar(&opts.debug, "debug", false, "print compiler diagnostics")
	flags.BoolVar(&opts.version, "version", false, "print compiler version")
	flags.BoolVar(&opts.version, "v", false, "print compiler version")
	flags.StringVar(&opts.out, "out", "", "output path")
	flags.StringVar(&opts.out, "o", "", "output path")
	flags.StringVar(&opts.emit, "emit", "exe", "output kind")
	flags.StringVar(&opts.emit, "e", "exe", "output kind")
	flags.IntVar(&opts.opt, "opt", 3, "optimization level")
	flags.IntVar(&opts.opt, "O", 3, "optimization level")
	flags.Uint64Var(&opts.errorTraceSlots, "error-trace-slots", 1024, "error trace slots per runtime shard")
	flags.BoolVar(&opts.safetyWarnings, "safety-warnings", false, "downgrade memory-safety diagnostics to warnings")
	flags.BoolVar(&opts.clangVersion, "clang-version", false, "print the resolved Clang version")
	flags.BoolVar(&opts.clangVersion, "cv", false, "print the resolved Clang version")
	flags.StringVar(&opts.target, "target", "", "target triple or architecture")
	flags.StringVar(&opts.stdRoot, "std", "", "standard-library directory")
	flags.BoolVar(&opts.lsp, "lsp", false, "run the language server over stdio")
	if err := flags.Parse(args); err != nil {
		return options{}, err
	}

	if opts.version || opts.clangVersion || opts.lsp {
		if flags.NArg() != 0 {
			return options{}, fmt.Errorf("information commands do not accept an input file")
		}
		return opts, nil
	}
	if flags.NArg() != 1 {
		return options{}, fmt.Errorf("expected exactly one input file, got %d", flags.NArg())
	}
	opts.emit = strings.ToLower(opts.emit)
	switch opts.emit {
	case "llvm", "ll":
		opts.emit = "llvm"
	case "object", "obj", "o":
		opts.emit = "object"
	case "exe", "executable", "binary", "bin":
		opts.emit = "exe"
	default:
		return options{}, fmt.Errorf("invalid --emit value %q (expected llvm, object, or exe)", opts.emit)
	}
	if opts.opt < 0 || opts.opt > 3 {
		return options{}, fmt.Errorf("invalid --opt value %d (expected 0 through 3)", opts.opt)
	}
	if opts.errorTraceSlots == 0 || opts.errorTraceSlots > 1024 || opts.errorTraceSlots&(opts.errorTraceSlots-1) != 0 {
		return options{}, fmt.Errorf("invalid --error-trace-slots value %d (expected a power of two from 1 through 1024)", opts.errorTraceSlots)
	}
	opts.inputFile = flags.Arg(0)
	return opts, nil
}

func normalizeArgs(args []string) []string {
	normalized := make([]string, 0, len(args)+1)
	for _, arg := range args {
		if len(arg) == 3 && strings.HasPrefix(arg, "-O") && arg[2] >= '0' && arg[2] <= '3' {
			normalized = append(normalized, "-O", arg[2:])
			continue
		}
		normalized = append(normalized, arg)
	}
	return normalized
}

func wrappedMain() error {
	opts, err := parseArgs(os.Args[1:])
	if err != nil {
		return err
	}
	debug.SetEnabled(opts.debug)
	if opts.stdRoot == "" {
		executable, err := os.Executable()
		if err != nil {
			return fmt.Errorf("locate compiler executable for standard-library discovery: %w", err)
		}
		opts.stdRoot = filepath.Join(filepath.Dir(executable), "std")
	}
	if opts.lsp {
		return lsp.ServeWithPolicy(os.Stdin, os.Stdout, opts.stdRoot, opts.safetyWarnings)
	}
	if opts.version {
		fmt.Printf("Magma %s\n", compilerVersion())
		return nil
	}
	if opts.clangVersion {
		path, version, err := clangresolver.Resolve("")
		if err != nil {
			return err
		}
		fmt.Printf("Clang %s (%s)\n", version, path)
		return nil
	}
	clangPath, _, err := clangresolver.Resolve("")
	if err != nil {
		return err
	}
	target, err := magmatarget.Resolve(clangPath, opts.target)
	if err != nil {
		return err
	}
	if opts.out == "" {
		opts.out = defaultOutput(opts.emit, string(target.OS))
	}
	opts.target = target.Triple
	opts.targetOS = string(target.OS)
	debug.Printf("target: %s\n", target.Triple)
	filePathArg := opts.inputFile

	cwd, e := os.Getwd()
	if e != nil {
		return e
	}

	debug.Printf("input file: %s\n", filePathArg)
	debug.Printf("cwd: %s\n", cwd)

	// second arg of MakeAbs is expected to be file path
	absPath, e := makeabs.MakeAbs(filePathArg, cwd+"/a.b")
	if e != nil {
		return e
	}

	s, e := shared.MakeShared(cwd, opts.stdRoot)
	if e != nil {
		return e
	}
	s.ErrorTraceSlots = opts.errorTraceSlots
	s.Target = target

	parsed, e := compilerpipeline.Parse(s, absPath)
	if e != nil {
		return e
	}
	if e = compilerpipeline.RequireMainModule(parsed, absPath); e != nil {
		return e
	}
	specialized, e := compilerpipeline.Specialize(parsed)
	if e != nil {
		return e
	}
	linked, e := compilerpipeline.Link(specialized)
	if e != nil {
		return e
	}
	typed, e := compilerpipeline.CheckTypes(linked)
	if e != nil {
		return e
	}
	validated, e := compilerpipeline.ValidateLowering(typed)
	if e != nil {
		return e
	}
	ready, e := compilerpipeline.CheckSafety(validated, opts.safetyWarnings)
	if e != nil {
		return e
	}
	for i := range s.Warnings {
		comp_err.FprintDiagnostic(os.Stderr, &s.Warnings[i])
	}

	irStr, e := compilerpipeline.Lower(ready)
	if e != nil {
		return e
	}

	//debug.Printf("LLVM IR:\n%s\n", irStr)
	debug.Printf("Successful lowering to LLVM\n")

	return emitOutput(opts, irStr, nativeLibraries(s), bundledFiles(s))
}

func nativeLibraries(s *types.SharedState) []string {
	seen := map[string]bool{}
	for _, file := range s.Files {
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

func bundledFiles(s *types.SharedState) []string {
	seen := map[string]bool{}
	for _, file := range s.Files {
		for _, bundle := range file.Bundles {
			seen[bundle] = true
		}
	}
	bundles := make([]string, 0, len(seen))
	for bundle := range seen {
		bundles = append(bundles, bundle)
	}
	sort.Strings(bundles)
	return bundles
}

func defaultOutput(emit, targetOS string) string {
	switch emit {
	case "object":
		if targetOS == "windows" {
			return "out.obj"
		}
		return "out.o"
	case "exe":
		if targetOS == "windows" {
			return "out.exe"
		}
		return "out"
	default:
		return "out.ll"
	}
}

func emitOutput(opts options, ir []byte, nativeLibraries, bundles []string) error {
	if opts.emit == "llvm" && opts.opt == 0 {
		return os.WriteFile(opts.out, []byte(ir), 0666)
	}

	clangPath, clangVersion, err := clangresolver.Resolve("")
	if err != nil {
		return err
	}
	debug.Printf("using Clang %s at %s\n", clangVersion, clangPath)

	temp, err := os.CreateTemp("", "magma-*.ll")
	if err != nil {
		return fmt.Errorf("create temporary LLVM file: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err = temp.Write(ir); err != nil {
		temp.Close()
		return fmt.Errorf("write temporary LLVM file: %w", err)
	}
	if err = temp.Close(); err != nil {
		return fmt.Errorf("close temporary LLVM file: %w", err)
	}

	args := []string{"-Wno-override-module", "-O" + strconv.Itoa(opts.opt), tempPath}
	if opts.target != "" {
		args = append([]string{"--target=" + opts.target}, args...)
	}
	switch opts.emit {
	case "llvm":
		args = append(args, "-S", "-emit-llvm")
	case "object":
		args = append(args, "-c")
	}
	if opts.emit == "exe" {
		for _, library := range nativeLibraries {
			if filepath.IsAbs(library) {
				args = append(args, library)
			} else {
				args = append(args, "-l"+library)
			}
		}
		args = append(args, runtimeLibraryArgs(opts.targetOS)...)
	}
	if dir := filepath.Dir(opts.out); dir != "." {
		if _, err := os.Stat(dir); err != nil {
			return fmt.Errorf("output directory %q: %w", dir, err)
		}
	}
	args = append(args, "-o", opts.out)
	debug.Printf("running: %s %s\n", clangPath, strings.Join(args, " "))
	cmd := exec.Command(clangPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Clang failed: %w", err)
	}
	if opts.emit == "exe" {
		if err := copyBundles(opts.out, bundles); err != nil {
			return err
		}
	}
	return nil
}

func runtimeLibraryArgs(targetOS string) []string {
	switch targetOS {
	case "linux", "freebsd", "netbsd", "openbsd":
		return []string{"-Wl,-rpath,$ORIGIN"}
	default:
		return nil
	}
}

func copyBundles(output string, bundles []string) error {
	outputDir := filepath.Dir(output)
	destinations := make(map[string]string, len(bundles))
	for _, source := range bundles {
		destination := filepath.Join(outputDir, filepath.Base(source))
		key := strings.ToLower(filepath.Clean(destination))
		if previous, exists := destinations[key]; exists && previous != source {
			return fmt.Errorf("bundle files %q and %q have the same output name", previous, source)
		}
		destinations[key] = source
	}

	for _, source := range bundles {
		destination := filepath.Join(outputDir, filepath.Base(source))
		sourceAbs, err := filepath.Abs(source)
		if err != nil {
			return fmt.Errorf("resolve bundle %q: %w", source, err)
		}
		destinationAbs, err := filepath.Abs(destination)
		if err != nil {
			return fmt.Errorf("resolve bundle destination %q: %w", destination, err)
		}
		if strings.EqualFold(sourceAbs, destinationAbs) {
			continue
		}

		input, err := os.Open(source)
		if err != nil {
			return fmt.Errorf("open bundle %q: %w", source, err)
		}
		info, err := input.Stat()
		if err != nil {
			input.Close()
			return fmt.Errorf("inspect bundle %q: %w", source, err)
		}
		if !info.Mode().IsRegular() {
			input.Close()
			return fmt.Errorf("bundle %q is not a regular file", source)
		}
		out, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			input.Close()
			return fmt.Errorf("create bundled file %q: %w", destination, err)
		}
		_, copyErr := io.Copy(out, input)
		closeOutErr := out.Close()
		closeInputErr := input.Close()
		if copyErr != nil {
			return fmt.Errorf("copy bundle %q to %q: %w", source, destination, copyErr)
		}
		if closeOutErr != nil {
			return fmt.Errorf("close bundled file %q: %w", destination, closeOutErr)
		}
		if closeInputErr != nil {
			return fmt.Errorf("close bundle %q: %w", source, closeInputErr)
		}
	}
	return nil
}

func compilerVersion() string {
	return strings.TrimSpace(compilerVersionText)
}

func main() {
	err := wrappedMain()
	if err != nil {
		if err == flag.ErrHelp {
			fmt.Println(usage)
			return
		}
		comp_err.Print(err)
		os.Exit(1)
	}
}
