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
	"time"
)

//go:embed VERSION.txt
var compilerVersionText string

const usage = `usage: magma [options] <input-file>

options:
  --debug                 print compiler diagnostics
  --timings               print compilation phase timings
  --version, -v           print the compiler version
  --out, -o <path>        output path (default depends on --emit)
  --emit, -e <kind>       llvm, object, or exe (default exe)
  --backend <kind>        object (default) or deprecated textual backend
  --incremental           cached bitcode compilation (default; use --incremental=false to disable)
  --cache-dir <path>      incremental cache directory (default: user cache)
  --incremental-explain   print per-module cache hit/miss reasons
  --opt, -O <0-3>         LLVM optimization level (default 3)
  --error-trace-slots <n> trace slots per runtime shard (default 1024)
  --compiler-arg <N=V>    provide a typed @compiler_known constant (repeatable)
  --safety-warnings       downgrade memory-safety diagnostics to warnings
  --null-context          use null allocator and executor adapters for roots
  --target <triple>       compilation target (default: Clang native target)
  --std <directory>       override the Magma standard-library directory
  --lsp                   run the Magma language server over stdio
  --clang-version, -cv    print the resolved Clang version and path`

type options struct {
	inputFile          string
	debug              bool
	timings            bool
	version            bool
	out                string
	emit               string
	backend            string
	incremental        bool
	cacheDir           string
	incrementalExplain bool
	opt                int
	errorTraceSlots    uint64
	safetyWarnings     bool
	nullContext        bool
	clangVersion       bool
	target             string
	targetOS           string
	stdRoot            string
	lsp                bool
	compilerArgs       compilerArgFlags
}

type compilerArgFlags map[string]string

func (values *compilerArgFlags) String() string { return "" }

func (values *compilerArgFlags) Set(value string) error {
	name, supplied, ok := strings.Cut(value, "=")
	if !ok || name == "" {
		return fmt.Errorf("compiler argument must use NAME=VALUE")
	}
	if *values == nil {
		*values = map[string]string{}
	}
	(*values)[name] = supplied
	return nil
}

func parseArgs(args []string) (options, error) {
	var opts options
	defaultBackendOptions(&opts)
	args = normalizeArgs(args)
	flags := flag.NewFlagSet("magma", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.BoolVar(&opts.debug, "debug", false, "print compiler diagnostics")
	flags.BoolVar(&opts.timings, "timings", false, "print compilation phase timings")
	flags.BoolVar(&opts.version, "version", false, "print compiler version")
	flags.BoolVar(&opts.version, "v", false, "print compiler version")
	flags.StringVar(&opts.out, "out", "", "output path")
	flags.StringVar(&opts.out, "o", "", "output path")
	flags.StringVar(&opts.emit, "emit", "exe", "output kind")
	flags.StringVar(&opts.emit, "e", "exe", "output kind")
	flags.StringVar(&opts.backend, "backend", opts.backend, "object or deprecated textual backend")
	flags.BoolVar(&opts.incremental, "incremental", opts.incremental, "cached bitcode compilation")
	flags.StringVar(&opts.cacheDir, "cache-dir", "", "incremental cache directory")
	flags.BoolVar(&opts.incrementalExplain, "incremental-explain", false, "explain incremental cache hits and misses")
	flags.IntVar(&opts.opt, "opt", 3, "optimization level")
	flags.IntVar(&opts.opt, "O", 3, "optimization level")
	flags.Uint64Var(&opts.errorTraceSlots, "error-trace-slots", 1024, "error trace slots per runtime shard")
	flags.BoolVar(&opts.safetyWarnings, "safety-warnings", false, "downgrade memory-safety diagnostics to warnings")
	flags.BoolVar(&opts.nullContext, "null-context", false, "use null root context")
	flags.BoolVar(&opts.clangVersion, "clang-version", false, "print the resolved Clang version")
	flags.BoolVar(&opts.clangVersion, "cv", false, "print the resolved Clang version")
	flags.StringVar(&opts.target, "target", "", "target triple or architecture")
	flags.StringVar(&opts.stdRoot, "std", "", "standard-library directory")
	flags.BoolVar(&opts.lsp, "lsp", false, "run the language server over stdio")
	flags.Var(&opts.compilerArgs, "compiler-arg", "compiler-known constant NAME=VALUE (repeatable)")
	if err := flags.Parse(args); err != nil {
		return options{}, err
	}
	incrementalExplicit := false
	flags.Visit(func(visited *flag.Flag) {
		if visited.Name == "incremental" {
			incrementalExplicit = true
		}
	})
	if opts.backend == "textual" && !incrementalExplicit {
		opts.incremental = false
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
	if opts.emit == "llvm" {
		if incrementalExplicit && opts.incremental {
			return options{}, fmt.Errorf("incremental compilation does not support textual LLVM output; use --emit object or --emit exe")
		}
		// LLVM text output is retained as a whole-program inspection path. It
		// cannot represent the linked cached-bitcode pipeline's native result.
		opts.incremental = false
	}
	if opts.opt < 0 || opts.opt > 3 {
		return options{}, fmt.Errorf("invalid --opt value %d (expected 0 through 3)", opts.opt)
	}
	if opts.backend != "" && opts.backend != "textual" && opts.backend != "object" {
		return options{}, fmt.Errorf("invalid --backend value %q (expected textual or object)", opts.backend)
	}
	if supplied, ok := opts.compilerArgs["ERROR_TRACE_SLOTS"]; ok {
		value, err := strconv.ParseUint(supplied, 0, 64)
		if err != nil {
			return options{}, fmt.Errorf("invalid compiler argument ERROR_TRACE_SLOTS=%q: %w", supplied, err)
		}
		opts.errorTraceSlots = value
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
	if err := validateIncrementalOptions(opts); err != nil {
		return err
	}
	debug.SetEnabled(opts.debug)
	timings := newCompilationTimings(opts.timings)
	defer timings.report(os.Stderr)
	stop := timings.start("Preparation", "standard library discovery")
	if opts.stdRoot == "" {
		executable, err := os.Executable()
		if err != nil {
			return fmt.Errorf("locate compiler executable for standard-library discovery: %w", err)
		}
		opts.stdRoot = filepath.Join(filepath.Dir(executable), "std")
	}
	stop()
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
	if opts.backend == "textual" {
		fmt.Fprintln(os.Stderr, "warning: the textual LLVM IR backend is deprecated and will be removed in a future release")
	}
	stop = timings.start("Preparation", "Clang and target resolution")
	clangPath, _, err := clangresolver.Resolve("")
	if err != nil {
		stop()
		return err
	}
	target, err := magmatarget.Resolve(clangPath, opts.target)
	stop()
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

	stop = timings.start("Preparation", "paths and shared state")
	cwd, e := os.Getwd()
	if e != nil {
		stop()
		return e
	}

	debug.Printf("input file: %s\n", filePathArg)
	debug.Printf("cwd: %s\n", cwd)

	// second arg of MakeAbs is expected to be file path
	absPath, e := makeabs.MakeAbs(filePathArg, cwd+"/a.b")
	if e != nil {
		stop()
		return e
	}

	s, e := shared.MakeShared(cwd, opts.stdRoot)
	if e != nil {
		stop()
		return e
	}
	s.ErrorTraceSlots = opts.errorTraceSlots
	s.CompilerArgs["ERROR_TRACE_SLOTS"] = strconv.FormatUint(opts.errorTraceSlots, 10)
	for name, value := range opts.compilerArgs {
		s.CompilerArgs[name] = value
	}
	s.NullContext = opts.nullContext
	s.Target = target
	stop()

	stop = timings.start("Front end", "parsing and imports")
	// Persistent object caching is safe independently of declaration-only
	// interface reuse. Interface materialization does not yet preserve every
	// semantic detail needed by generic bodies, prototype implementations, and
	// aggregate layouts, so normal builds parse source until it reaches parity.
	parsed, e := compilerpipeline.Parse(s, absPath)
	stop()
	if e != nil {
		return e
	}
	if opts.incremental {
		safetyMode := "strict"
		if opts.safetyWarnings {
			safetyMode = "warnings"
		}
		if e = compilerpipeline.UseCachedSpecializations(s, opts.cacheDir, compilerVersion(), safetyMode); e != nil {
			return e
		}
	}
	stop = timings.start("Front end", "main module validation")
	if e = compilerpipeline.RequireMainModule(parsed, absPath); e != nil {
		stop()
		return e
	}
	stop()
	stop = timings.start("Front end", "generic specialization")
	specialized, e := compilerpipeline.Specialize(parsed)
	stop()
	if e != nil {
		return e
	}
	stop = timings.start("Front end", "module linking")
	linked, e := compilerpipeline.Link(specialized)
	stop()
	if e != nil {
		return e
	}
	stop = timings.start("Checks", "type checking")
	typed, e := compilerpipeline.CheckTypes(linked)
	stop()
	if e != nil {
		return e
	}
	stop = timings.start("Checks", "lowering validation")
	validated, e := compilerpipeline.ValidateLowering(typed)
	stop()
	if e != nil {
		return e
	}
	stop = timings.start("Checks", "memory safety checking")
	ready, e := compilerpipeline.CheckSafety(validated, opts.safetyWarnings)
	stop()
	if e != nil {
		return e
	}
	for i := range s.Warnings {
		comp_err.FprintDiagnostic(os.Stderr, &s.Warnings[i])
	}

	stop = timings.start("Back end", backendLoweringLabel(opts))
	output, isObject, e := lowerBackend(ready, opts)
	stop()
	if e != nil {
		return e
	}

	//debug.Printf("LLVM IR:\n%s\n", irStr)
	debug.Printf("Successful lowering through %s\n", backendLoweringLabel(opts))

	stop = timings.start("Back end", "output and Clang")
	if isObject {
		e = emitObjectOutput(opts, output, nativeLibraries(s), bundledFiles(s), embeddedAssets(s))
	} else {
		e = emitOutput(opts, output, nativeLibraries(s), bundledFiles(s), embeddedAssets(s))
	}
	stop()
	return e
}

type embeddedAsset struct {
	Path   string
	Symbol string
	Size   uint64
}

func embeddedAssets(s *types.SharedState) []embeddedAsset {
	bySymbol := map[string]embeddedAsset{}
	for _, file := range s.Files {
		if file == nil || file.GlNode == nil {
			continue
		}
		for _, declaration := range file.GlNode.Declarations {
			constant, ok := declaration.(*types.NodeConstDef)
			if !ok {
				continue
			}
			embedded, ok := constant.Initializer.(*types.NodeExprEmbed)
			if ok {
				bySymbol[embedded.Symbol] = embeddedAsset{Path: embedded.Path, Symbol: embedded.Symbol, Size: embedded.Size}
			}
		}
	}
	assets := make([]embeddedAsset, 0, len(bySymbol))
	for _, asset := range bySymbol {
		assets = append(assets, asset)
	}
	sort.Slice(assets, func(i, j int) bool { return assets[i].Symbol < assets[j].Symbol })
	return assets
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

func emitOutput(opts options, ir []byte, nativeLibraries, bundles []string, assets []embeddedAsset) error {
	if len(assets) != 0 {
		return emitOutputWithAssets(opts, ir, nativeLibraries, bundles, assets)
	}
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
			args = append(args, nativeLibraryArgs(library)...)
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

func emitObjectOutput(opts options, object []byte, nativeLibraries, bundles []string, assets []embeddedAsset) error {
	if opts.emit == "llvm" {
		return fmt.Errorf("internal error: LLVM output was lowered as an object")
	}
	if dir := filepath.Dir(opts.out); dir != "." {
		if _, err := os.Stat(dir); err != nil {
			return fmt.Errorf("output directory %q: %w", dir, err)
		}
	}
	if opts.emit == "object" && len(assets) == 0 {
		return os.WriteFile(opts.out, object, 0666)
	}

	clangPath, clangVersion, err := clangresolver.Resolve("")
	if err != nil {
		return err
	}
	debug.Printf("using Clang %s at %s\n", clangVersion, clangPath)
	temporaryDir, err := os.MkdirTemp("", "magma-object-*")
	if err != nil {
		return fmt.Errorf("create object-link workspace: %w", err)
	}
	defer os.RemoveAll(temporaryDir)
	programObject := filepath.Join(temporaryDir, "program.o")
	if err := os.WriteFile(programObject, object, 0600); err != nil {
		return fmt.Errorf("write program object: %w", err)
	}

	inputs := []string{programObject}
	if len(assets) != 0 {
		assetSource := filepath.Join(temporaryDir, "assets.c")
		assetObject := filepath.Join(temporaryDir, "assets.o")
		if err := writeEmbeddedAssetSource(assetSource, assets); err != nil {
			return err
		}
		assetArgs := []string{"-std=c23", "-O0", "-c", assetSource, "-o", assetObject}
		if opts.target != "" {
			assetArgs = append([]string{"--target=" + opts.target}, assetArgs...)
		}
		debug.Printf("running: %s %s\n", clangPath, strings.Join(assetArgs, " "))
		if output, runErr := exec.Command(clangPath, assetArgs...).CombinedOutput(); runErr != nil {
			return fmt.Errorf("Clang failed compiling embedded assets: %w\n%s", runErr, output)
		}
		inputs = append(inputs, assetObject)
	}

	args := make([]string, 0, len(inputs)+len(nativeLibraries)+8)
	if opts.target != "" {
		args = append(args, "--target="+opts.target)
	}
	if opts.emit == "object" {
		args = append(args, "-r")
	}
	args = append(args, inputs...)
	if opts.emit == "exe" {
		for _, library := range nativeLibraries {
			args = append(args, nativeLibraryArgs(library)...)
		}
		args = append(args, runtimeLibraryArgs(opts.targetOS)...)
	}
	args = append(args, "-o", opts.out)
	debug.Printf("running: %s %s\n", clangPath, strings.Join(args, " "))
	command := exec.Command(clangPath, args...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	linkStart := time.Now()
	if err := command.Run(); err != nil {
		return fmt.Errorf("Clang failed linking object output: %w", err)
	}
	if os.Getenv("MAGMA_LLVM_TIMINGS") != "" {
		fmt.Fprintf(os.Stderr, "LLVM_TIMING clang_link_ns=%d\n", time.Since(linkStart).Nanoseconds())
	}
	if opts.emit == "exe" {
		return copyBundles(opts.out, bundles)
	}
	return nil
}

func emitOutputWithAssets(opts options, ir []byte, nativeLibraries, bundles []string, assets []embeddedAsset) error {
	if opts.emit == "llvm" {
		return fmt.Errorf("--emit llvm cannot represent @embed payloads; use --emit object or --emit exe")
	}
	clangPath, clangVersion, err := clangresolver.Resolve("")
	if err != nil {
		return err
	}
	debug.Printf("using Clang %s at %s\n", clangVersion, clangPath)
	if dir := filepath.Dir(opts.out); dir != "." {
		if _, err := os.Stat(dir); err != nil {
			return fmt.Errorf("output directory %q: %w", dir, err)
		}
	}

	temporaryDir, err := os.MkdirTemp("", "magma-embed-*")
	if err != nil {
		return fmt.Errorf("create embedding workspace: %w", err)
	}
	defer os.RemoveAll(temporaryDir)
	programIR := filepath.Join(temporaryDir, "program.ll")
	assetSource := filepath.Join(temporaryDir, "assets.c")
	programObject := filepath.Join(temporaryDir, "program.o")
	assetObject := filepath.Join(temporaryDir, "assets.o")
	if err := os.WriteFile(programIR, ir, 0600); err != nil {
		return fmt.Errorf("write temporary LLVM file: %w", err)
	}
	if err := writeEmbeddedAssetSource(assetSource, assets); err != nil {
		return err
	}

	targetArg := []string{}
	if opts.target != "" {
		targetArg = append(targetArg, "--target="+opts.target)
	}
	type compileResult struct {
		name   string
		output []byte
		err    error
	}
	results := make(chan compileResult, 2)
	programArgs := append(append([]string{}, targetArg...), "-Wno-override-module", "-O"+strconv.Itoa(opts.opt), "-c", programIR, "-o", programObject)
	assetArgs := append(append([]string{}, targetArg...), "-std=c23", "-O0", "-c", assetSource, "-o", assetObject)
	for name, args := range map[string][]string{"LLVM program": programArgs, "embedded assets": assetArgs} {
		go func(name string, args []string) {
			debug.Printf("running: %s %s\n", clangPath, strings.Join(args, " "))
			output, runErr := exec.Command(clangPath, args...).CombinedOutput()
			results <- compileResult{name: name, output: output, err: runErr}
		}(name, args)
	}
	var compileErr error
	for range 2 {
		result := <-results
		if result.err != nil && compileErr == nil {
			compileErr = fmt.Errorf("Clang failed compiling %s: %w\n%s", result.name, result.err, result.output)
		}
	}
	if compileErr != nil {
		return compileErr
	}

	args := append([]string{}, targetArg...)
	if opts.emit == "object" {
		args = append(args, "-r", programObject, assetObject)
	} else {
		args = append(args, programObject, assetObject)
		for _, library := range nativeLibraries {
			args = append(args, nativeLibraryArgs(library)...)
		}
		args = append(args, runtimeLibraryArgs(opts.targetOS)...)
	}
	args = append(args, "-o", opts.out)
	debug.Printf("running: %s %s\n", clangPath, strings.Join(args, " "))
	command := exec.Command(clangPath, args...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("Clang failed linking embedded assets: %w", err)
	}
	if opts.emit == "exe" {
		return copyBundles(opts.out, bundles)
	}
	return nil
}

func writeEmbeddedAssetSource(path string, assets []embeddedAsset) error {
	var source strings.Builder
	for _, asset := range assets {
		if asset.Size == 0 {
			fmt.Fprintf(&source, "const unsigned char %s[1] = {0};\n", asset.Symbol)
			continue
		}
		fmt.Fprintf(&source, "const unsigned char %s[] = {\n#embed %s\n};\n", asset.Symbol, strconv.Quote(filepath.ToSlash(asset.Path)))
	}
	if err := os.WriteFile(path, []byte(source.String()), 0600); err != nil {
		return fmt.Errorf("write embedded-asset source: %w", err)
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

func nativeLibraryArgs(library string) []string {
	if framework, found := strings.CutPrefix(library, "framework:"); found {
		return []string{"-framework", framework}
	}
	if filepath.IsAbs(library) {
		return []string{library}
	}
	return []string{"-l" + library}
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
