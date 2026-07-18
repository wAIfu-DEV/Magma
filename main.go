package main

import (
	"Magma/src/checker"
	clangresolver "Magma/src/clang"
	"Magma/src/comp_err"
	"Magma/src/debug"
	destroychecker "Magma/src/destroy_checker"
	ircleaner "Magma/src/ir_cleaner"
	"Magma/src/join"
	llvmir "Magma/src/llvm_ir"
	"Magma/src/makeabs"
	"Magma/src/monomorph"
	"Magma/src/pipeline"
	"Magma/src/shared"
	"Magma/src/types"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	builddebug "runtime/debug"
	"sort"
	"strconv"
	"strings"
)

const usage = `usage: magma [options] <input-file>

options:
  --debug                 print compiler diagnostics
  --version, -v           print the compiler version
  --out, -o <path>        output path (default depends on --emit)
  --emit, -e <kind>       llvm, object, or exe (default llvm)
  --opt, -O <0-3>         LLVM optimization level (default 3)
  --error-trace-slots <n> trace slots per runtime shard (default 1024)
  --clang-version, -cv    print the resolved Clang version and path`

type options struct {
	inputFile       string
	debug           bool
	version         bool
	out             string
	emit            string
	opt             int
	errorTraceSlots uint64
	clangVersion    bool
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
	flags.BoolVar(&opts.clangVersion, "clang-version", false, "print the resolved Clang version")
	flags.BoolVar(&opts.clangVersion, "cv", false, "print the resolved Clang version")
	if err := flags.Parse(args); err != nil {
		return options{}, err
	}
	if opts.version || opts.clangVersion {
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
	if opts.errorTraceSlots == 0 || opts.errorTraceSlots > 65536 || opts.errorTraceSlots&(opts.errorTraceSlots-1) != 0 {
		return options{}, fmt.Errorf("invalid --error-trace-slots value %d (expected a power of two from 1 through 65536)", opts.errorTraceSlots)
	}
	opts.inputFile = flags.Arg(0)
	if opts.out == "" {
		opts.out = defaultOutput(opts.emit)
	}
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

	s, e := shared.MakeShared(cwd)
	if e != nil {
		return e
	}
	s.ErrorTraceSlots = opts.errorTraceSlots

	// actual meat of the program, multithreaded per file
	// 1. lexing/tokenization
	// 2. parsing to AST
	// 3. scope info gathering
	if e = pipeline.DoMain(s, absPath); e != nil {
		if !comp_err.Print(e) {
			fmt.Printf("fatal error in file '%s': %s\n", absPath, e.Error())
		}
	}

	// wait for other compilation unit goroutines
	if e = join.JoinCompilationUnits(s, e); e != nil {
		os.Exit(1)
	}
	mainFile := s.Files[absPath]
	if mainFile != nil && mainFile.ModuleName != "main" {
		return comp_err.CompilationErrorToken(
			mainFile,
			&types.Token{
				Pos:  types.FilePos{Line: 1, Col: 5},
				Repr: mainFile.ModuleName,
			},
			fmt.Sprintf("main file must declare module 'main', not '%s'", mainFile.ModuleName),
			"the root compilation unit must start with: `mod main`",
		)
	}

	if e = monomorph.Run(s); e != nil {
		return e
	}

	// check/resolve name->node
	if e = checker.CheckLinks(s); e != nil {
		return e
	}

	// check/resolve types
	if e = checker.TypeChecker(s); e != nil {
		return e
	}

	destroychecker.Run(s)

	// write LLVM intermediate repr
	irStr, e := llvmir.IrWrite(s)
	if e != nil {
		return e
	}

	irStr, e = ircleaner.CleanIr(irStr)
	if e != nil {
		return e
	}

	//debug.Printf("LLVM IR:\n%s\n", irStr)
	debug.Printf("Successful lowering to LLVM\n")

	return emitOutput(opts, irStr, nativeLibraries(s))
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

func defaultOutput(emit string) string {
	switch emit {
	case "object":
		if runtime.GOOS == "windows" {
			return "out.obj"
		}
		return "out.o"
	case "exe":
		if runtime.GOOS == "windows" {
			return "out.exe"
		}
		return "out"
	default:
		return "out.ll"
	}
}

func emitOutput(opts options, ir []byte, nativeLibraries []string) error {
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
	return nil
}

func compilerVersion() string {
	if info, ok := builddebug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "(devel)"
}

func main() {
	err := wrappedMain()
	if err != nil {
		if err == flag.ErrHelp {
			fmt.Println(usage)
			return
		}
		if !comp_err.Print(err) {
			fmt.Printf("uncaught fatal error: %s\n", err.Error())
		}
		os.Exit(1)
	}
}
