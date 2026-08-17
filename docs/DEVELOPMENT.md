# Development and testing

Compiler development requires Go 1.24.6 or later and a working Clang. Clang is
needed not only to link executables: the compiler queries it for the native or
requested target and its C ABI. Set `MAGMA_CLANG` to an executable path when
the desired Clang is not first on `PATH`.

## Build

From the repository root:

```sh
go build -o Magma .
```

On Windows, use `go build -o Magma.exe .`. A quick compiler-only verification
is:

```sh
go test ./...
```

The Go tests cover individual compiler packages and orchestration behavior.
They do not replace the Magma source suites described below.

## Run all source tests

Build the compiler at the repository root, then run:

```sh
./RUN_TESTS.sh
```

On Windows use `RUN_TESTS.bat`. The scripts optionally display each compiler
invocation's output and run two suites:

- Every `.mg` file below `tests/` is compiled. Files under a directory
  containing `.expect-failure` must be rejected without a compiler crash or
  backend failure; all other files must compile.
- Every `.mg` file in `std/tests/` is compiled as an executable and run. A zero
  exit status means its assertions passed.

The scripts also enforce a one-to-one test-file convention for top-level
`std/*.mg` modules, except `raylib.mg`: adding `std/widget.mg` requires a
matching `std/tests/widget.mg`, and orphaned standard-library tests fail too.
Temporary IR, logs, and test executables are created outside the repository.

## Add a compiler test

Put accepted programs under `tests/syntax_valid/`. Put rejected programs under
`tests/syntax_invalid/` or another directory with an inherited
`.expect-failure` marker. Keep each input focused on one language rule; package
unit tests are a better home when exact AST, diagnostic, or generated-IR
details must be asserted.

For a standard-library change, update the matching `std/tests/<module>.mg`.
Those tests are real executables, so they can validate runtime behavior as well
as compilation. Native or platform-specific behavior should be guarded using
the same conditional declarations as its implementation.

## Focused checks

Use normal Go test selection while iterating on a compiler package:

```sh
go test ./src/checker
go test ./src/llvm_ir -run TestName -v
```

To inspect generated IR for one Magma program:

```sh
./Magma --std ./std --emit llvm --out /tmp/example.ll samples/hello_world.mg
```

Use `--emit exe` when runtime behavior matters. The full CLI and output rules
are documented in [Magma Compiler](COMPILER.md).

## Change checklist

When a language feature changes, update its parser, semantic checks, lowering,
and ownership behavior as applicable, then cover both accepted and rejected
forms. Keep [Magma Syntax](SYNTAX.md) normative examples and
[Language Analysis](FEATURES.md) feature descriptions in sync.

When a public `std` declaration changes, update its source test and the matching
page under `docs/std/`; the [standard-library index](std/README.md) lists the
public modules. Platform implementation modules under `std/win`, `std/unix`,
and `std/linux` are internal and do not need separate public reference pages.

## Release scripts

`RELEASE.sh` and `RELEASE.bat` are maintainer workflows, not ordinary build
commands. They build the compiler, run the full source test suite, cross-build
Windows and Linux binaries, increment the patch version in `VERSION.txt`, and
create platform archives according to `RELEASE_IGNORE.txt`. Because they modify
the version and create release artifacts, run them only when preparing a
release.
