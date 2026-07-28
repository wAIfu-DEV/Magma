# Magma

Magma is a statically typed systems programming language. Its compiler is
written in Go, emits LLVM IR, and invokes Clang to produce object files and
executables.

The language includes pointers, manual allocation, external symbols, inline
LLVM, generics, methods, deferred statements, and typed error propagation. The
standard library is implemented in Magma under [`std/`](std/).

## Hello World

```magma
mod main

use "../std/io.mg" io

pub main(args str[]) !void:
    io.printLn("Hello, World!")
..
```

Magma uses `:` to open a block and `..` to close it. Declarations place the name
before the type, `:=` infers the type of a local variable, and `!void` indicates
that a function can return an error.

## Language Features

### Error handling

Throwing functions prefix their return type with `!`. The `try` expression
returns a successful value or propagates the error to the caller:

```magma
readByte() !u8:
    ret 42
..

load() !void:
    value := try readByte()
    # use value
..
```

A caller can instead receive the value and error separately:

```magma
value u8, err error = readByte()
if err.nok():
    throw err
..
```

### Deferred statements and ownership annotations

`defer` registers a statement to run when its scope exits. Multiple deferred
statements in a scope run in last-in, first-out order.

```magma
f := try file.open(a, path, file.mode().read())
defer f.close()
```

The `$T` annotation marks ownership transfer in parameters and return values.
The compiler's destroy checker emits warnings for tracked ownership violations.
Its behavior and limits are documented in
[Ownership and Destruction](docs/OWNERSHIP.md).

### Generics and methods

Magma supports generic structs, functions, and receiver methods. Generic
definitions are specialized by the compiler for the types used by a program.

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

### Native interoperation

The language supports typed pointers, raw `ptr`, slices, stack-backed array
expressions, `sizeof`, address and dereference operations, external function
declarations, and inline LLVM.

The `@export_name` directive adds a native forwarding symbol for a top-level,
non-generic, non-throwing function. The default ABI is C.

```magma
@export_name("magma_add")
add(a i32, b i32) i32:
    ret a + b
..
```

### Futures

Asynchronous standard-library operations use `thread_pool.ThreadPool` and
`future.Future[T]`; the language does not define `async` or `await` keywords.
`Future.isDone()` polls a future, and `Future.await()` waits for and consumes it.
See [Asynchronous work with futures](docs/FEATURES.md#8-asynchronous-work-with-futures)
and the [`future` reference](docs/std/future.md).

## Standard Library

The [`std/`](std/) directory contains modules for:

- allocation, memory operations, casts, and atomics;
- files, paths, processes, readers, writers, and buffered streams;
- arrays, lists, queues, maps, sorting, and searching;
- strings, bytes, UTF-8, formatting, and numeric conversion;
- JSON serialization;
- threads, mutexes, wake primitives, worker pools, and futures;
- HTTP and raylib bindings.

Platform implementations are selected with `@platform(...)` directives and are
stored under [`std/win/`](std/win/) and [`std/unix/`](std/unix/). Individual
module documentation records any narrower platform support.

## Build and Run

### Requirements

- [Go](https://go.dev/) 1.24.6 or later
- [Clang/LLVM](https://llvm.org/) on `PATH`, or a Clang executable specified by
  the `MAGMA_CLANG` environment variable

Build the compiler and compile the included hello-world sample:

```powershell
go build
.\Magma.exe --std .\std --emit exe --out hello.exe samples\hello_world.mg
.\hello.exe
```

The compiler can also emit LLVM IR or an object file:

```powershell
.\Magma.exe --std .\std --emit llvm --out hello.ll samples\hello_world.mg
.\Magma.exe --std .\std --emit object --out hello.obj samples\hello_world.mg
```

On Windows, `RUN_TESTS.bat` builds the compiler, compiles the compiler test
suite, and compiles and runs the standard-library tests. `BUILD_SAMPLE.bat`
builds the compiler and compiles and runs `samples/args_echo.mg`.

### Compiler options

```text
usage: magma [options] <input-file>

  --debug                 print compiler diagnostics
  --version, -v           print the compiler version
  --out, -o <path>        choose the output path
  --emit, -e <kind>       emit llvm, object, or exe
  --opt, -O <0-3>         select the LLVM optimization level
  --error-trace-slots <n> trace slots per runtime shard (default 1024)
  --target <triple>       set the compilation target
  --std <directory>       override the Magma standard-library directory
  --lsp                   run the language server over standard I/O
  --clang-version, -cv    print the resolved Clang version and path
```

Executable output and optimization level 3 are the defaults. The compiler
accepts `llvm`, `object`, and `exe` as output kinds. If `--out` is omitted, the
output name depends on the selected kind and target platform.

By default, the compiler uses the `std` directory beside the compiler
executable. Pass `--std` to use another directory:

```powershell
.\Magma.exe --std C:\path\to\Magma\std --lsp
```

`--error-trace-slots` accepts a power of two from 1 through 65536. The compiler
uses 64 runtime trace shards, each with the configured number of slots.

## Documentation and Source

- [`docs/SYNTAX.md`](docs/SYNTAX.md) - language syntax
- [`docs/FEATURES.md`](docs/FEATURES.md) - language and library features
- [`docs/OWNERSHIP.md`](docs/OWNERSHIP.md) - ownership and destruction analysis
- [`docs/std/`](docs/std/) - standard-library reference
- [`samples/`](samples/) - example programs
- [`src/`](src/) - compiler implementation

The compiler pipeline parses source modules, gathers scope information, checks
links and types, runs ownership analysis, specializes generics, lowers the
program to LLVM IR, and cleans the generated IR before native compilation.

Relevant packages include:

- `src/tokenizer` and `src/parser` for syntax;
- `src/checker` and `src/destroy_checker` for static analysis;
- `src/monomorph` for generic specialization;
- `src/llvm_ir` and `src/ir_cleaner` for LLVM output;
- `src/clang` for Clang discovery and invocation.
