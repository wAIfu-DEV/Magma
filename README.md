# Magma

Magma is a compact, statically typed systems programming language built around
explicit control, practical error handling, and native compilation through
LLVM. It combines low-level facilities—pointers, manual allocation, external
symbols, and inline LLVM—with modern conveniences such as generics, methods,
deferred cleanup, and typed error propagation.

The compiler is written in Go and can emit LLVM IR, object files, or native
executables. Magma is under active development, but already supports a
substantial standard library and sample applications ranging from command-line
tools to HTTP, threading, and graphics.

## Why Magma?

- **Native by default.** Magma lowers to LLVM and uses Clang to produce optimized
  object files and executables.
- **Errors are visible in the type system.** Throwing functions use `!T`, while
  `try` makes propagation concise and explicit.
- **Cleanup stays close to acquisition.** `defer` runs on normal returns, errors,
  and other scope exits.
- **Abstractions remain lightweight.** Generics are monomorphized, methods use
  value-oriented structs, and function pointers support vtable-style interfaces.
- **Systems programming is first-class.** Pointers, slices, fixed arrays, raw
  memory operations, external functions, and inline LLVM are directly available.
- **Portability is designed in.** Platform directives and Windows/Unix standard
  library backends keep platform-specific code behind shared APIs.
- **Ownership intent can be expressed.** Transfer annotations and a warning-only
  destroy checker catch common resource-management mistakes without hiding
  allocation or destruction.

## A First Program

```magma
mod main

use "../std/heap.mg" heap
use "../std/io.mg" io

pub main(args str[]) !void:
    a := heap.allocator()

    out := try io.stdout(a)
    defer out.close()

    try out.writeLn("Hello, World!")
..
```

Magma uses `:` to open a block and `..` to close it. Declarations place the name
before the type, `:=` provides local type inference, and `!void` declares that
`main` may return an error.

## Language Highlights

### Explicit, composable error handling

Failure is part of a function's signature:

```magma
readByte() !u8:
    ret 42
..

load() !void:
    value := try readByte()
    # use value
..
```

`try` unwraps a successful result and propagates an error. Callers that need to
recover can handle both results directly:

```magma
value u8, err error = readByte()
if errors.code(err) != 0:
    # recover or propagate with `throw err`
..
```

### Predictable resource management

`defer` registers cleanup at the point where a resource is acquired. Deferred
work runs in last-in, first-out order on every scope exit, including error
propagation:

```magma
f := try file.open(a, path, file.mode().read())
defer f.close()
```

The `$T` annotation marks ownership transfer in parameters and return values.
For destructible structs, Magma's flow analysis warns about leaks, double
consumption, consuming borrowed values, discarded owned results, and use after
transfer. See [Ownership and Destruction](docs/OWNERSHIP.md) for the model and
its current limits.

### Generics and methods

Magma supports generic structs, functions, and receiver methods:

```magma
Pair[A, B](
    left A,
    right B,
)

swap[A, B](p Pair[A, B]) Pair[B, A]:
    out Pair[B, A]
    out.left = p.right
    out.right = p.left
    ret out
..
```

Generics are monomorphized by the compiler, providing reusable abstractions
without requiring a runtime generic representation.

### Direct access to the machine

The language includes typed pointers, raw `ptr`, slices, fixed-size arrays,
`sizeof`, address and dereference operations, and external function declarations.
When a primitive cannot yet be expressed in Magma itself, inline LLVM provides a
deliberate escape hatch.

## Standard Library

The standard library in [`std/`](std/) demonstrates that Magma's small core can
support practical, reusable components:

- allocators, platform heaps, memory operations, and explicit casts;
- files, paths, readers, writers, buffered streams, and duplex streams;
- arrays, lists, queues, maps, builders, sorting, and searching;
- strings, byte utilities, UTF-8 helpers, and numeric conversion;
- JSON values and serialization;
- CPU core discovery, native threads, mutexes, wake primitives, and worker pools;
- Windows streaming HTTP through WinHTTP;
- Windows raylib 5.5 bindings for windows, drawing, and input.

Platform implementations live under `std/win/` and `std/unix/` and are selected
with `@platform(...)` directives.

## Build and Run

### Requirements

- [Go](https://go.dev/) 1.24.6 or later
- [Clang/LLVM](https://llvm.org/) available on `PATH`, or configured through the
  `MAGMA_CLANG` environment variable

Build the compiler, then compile and run the hello-world sample:

```powershell
go build
.\Magma.exe --emit exe --out hello.exe samples\hello_world.mg
.\hello.exe
```

The compiler can also stop at LLVM IR or an object file:

```powershell
.\Magma.exe --emit llvm --out hello.ll samples\hello_world.mg
.\Magma.exe --emit object --out hello.obj samples\hello_world.mg
```

On Windows, `BUILD&RUN_TESTS.bat` builds the compiler, compiles the language test
suite, and runs it. `BUILD_SAMPLE.bat` builds and runs the raylib sample.

### Compiler options

```text
usage: magma [options] <input-file>

  --debug                 print compiler diagnostics
  --version, -v           print the compiler version
  --out, -o <path>        choose the output path
  --emit, -e <kind>       emit llvm, object, or exe
  --opt, -O <0-3>         select the LLVM optimization level
  --error-trace-slots <n> trace slots per runtime shard (default 1024)
  --clang-version, -cv    print the resolved Clang version and path
```

Executable output and optimization level 3 are the current defaults. Magma
searches `MAGMA_CLANG`, `PATH`, common LLVM installations, and Visual Studio LLVM
directories when resolving Clang.

Error traces use 64 fixed runtime shards. `--error-trace-slots` accepts a power
of two from 1 through 65536 and controls the slots in each shard. Increasing it
retains diagnostics longer at the cost of proportional static storage.

## Explore the Project

- [`docs/SYNTAX.md`](docs/SYNTAX.md) — complete syntax guide and language details
- [`docs/OWNERSHIP.md`](docs/OWNERSHIP.md) — ownership, destruction, and analysis
- [`docs/std/`](docs/std/) — standard library reference
- [`samples/`](samples/) — focused examples and complete console applications
- [`src/`](src/) — compiler implementation

The sample suite includes FizzBuzz, a calculator, file readers, a contact book,
a to-do list, an expense tracker, Tic-Tac-Toe, HTTP, thread-pool benchmarks, and
a raylib program.

## Compiler Architecture

The frontend tokenizes and parses source files concurrently, gathers scope
information, performs link and type checking, checks ownership and destruction,
monomorphizes generics, and lowers the result to LLVM IR. An IR cleanup pass
prepares the output before Clang performs optimization and native code
generation.

Key packages include:

- `src/tokenizer` and `src/parser` for syntax;
- `src/checker` and `src/destroy_checker` for static analysis;
- `src/monomorph` for generic specialization;
- `src/llvm_ir` and `src/ir_cleaner` for LLVM output;
- `src/clang` for toolchain discovery and native compilation.

## Project Status

Magma is an early-stage language and compiler. It is suitable for exploration,
language development, and experimental native programs, but it does not yet
promise production-grade safety or stability. In particular:

- ownership diagnostics are warning-only and currently track direct locals, not
  aliases, fields, indexed storage, pointers, or partial moves;
- bounds checks, null checks, and general memory safety are not provided;
- static checking is still being expanded, and some invalid programs may reach
  LLVM before being rejected;
- `while` is currently the primary loop construct; `for` loops are planned;
- inline LLVM is intentionally powerful and lies outside normal type checking.

Current implementation work is tracked in [`TODO.md`](TODO.md). Contributions,
experiments, and feedback are welcome as the language evolves.
