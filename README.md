# Magma

Magma is a statically typed systems programming language. Its compiler is
written in Go, lowers whole programs to LLVM IR, and invokes Clang to optimize
that IR and produce LLVM, object, or executable output.

The language includes pointers, manual allocation, external symbols, inline
LLVM, generics, methods, deferred statements, and typed error propagation. The
standard library is implemented in Magma under [`std/`](std/).

For a released compiler setup, follow the detailed
[Installation and First Use guide](docs/INSTALLATION.md). Download a release
archive rather than cloning the repository, then install LLVM Clang and add
Magma to `PATH` with the included script.

## Hello World

```magma
mod main

use "std:io" io

pub main() void:
    io.printLn("Hello, World!")
..
```

Magma uses `:` to open a block and `..` to close it. Declarations place the name
before the type, `:=` infers the type of a local variable, and `!void` indicates
that a function can return an error. `use "std:io" io` is the canonical form
for standard-library imports: `std:` resolves from the standard library shipped
beside the compiler, the `.mg` extension is optional, and the final `io` is the
local namespace alias.

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
value, err := readByte()
if err.nok():
    throw err
..
```

### Deferred statements and ownership annotations

`defer` registers a statement to run when its scope exits. Multiple deferred
statements in a scope run in last-in, first-out order.

```magma
use "std:file" file

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
pub Pair[A, B](
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

Modules may also re-export an imported namespace with `pub use`. For example,
`std:allocators` groups the allocator modules as `allocators.heap`,
`allocators.arena`, `allocators.debug`, and `allocators.scratch`.

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

### Platform and C interoperability

`@platform(...)` conditionally includes the next top-level declaration or
import. Target-dependent C integer aliases are available from `std:c`; `ext`
declares imported native functions, `link` records libraries required for
executable emission, and `bundle` copies runtime files beside a completed
executable. See [Magma Syntax](docs/SYNTAX.md) for their exact forms and ABI
restrictions.

## Standard Library

The [`std/`](std/) directory contains modules for:

- heap, arena, scratch, debug, and interface-based allocation;
- raw memory operations, explicit casts, and checked arithmetic;
- files, directories, paths, processes, environment variables, dialogs,
  readers, writers, and buffered or duplex streams;
- arrays, lists, queues, heaps, maps, iterators, sorting, and searching;
- strings, byte slices, builders, UTF-8, UTF-16, Base64, hexadecimal and
  percent encoding, formatting, and numeric conversion;
- JSON parsing and serialization, HTTP clients, and raylib bindings;
- atomics, mutexes, spin locks, wake primitives, threads, type-erased
  executors, dynamically sized thread pools, futures, and asynchronous I/O;
- command-line arguments and flags, time, randomness, CPU information, and
  common error categories.

Platform implementations are selected with `@platform(...)` directives and are
stored under [`std/win/`](std/win/) and [`std/unix/`](std/unix/). Individual
module documentation records any narrower platform support.

Import installed modules with their canonical `std:` paths:

```magma
use "std:heap" heap
use "std:array" array
use "std:fmt" fmt
```

## Install and Run

Install Magma from a release archive, keep its `std` directory beside
`Magma.exe`, install LLVM Clang, and run the included
`ADD_MAGMA_TO_PATH.bat`. Repository clones and GitHub source archives are not
release packages and may omit executable or dynamic-library artifacts. The
complete procedure is in [Installing Magma](docs/INSTALLATION.md).

After installation, compile a program from any working directory:

```powershell
Magma.exe --out hello.exe hello.mg
.\hello.exe
```

Executable output and optimization level 3 are the defaults. The compiler finds
its adjacent standard library automatically; normal release use does not need
`--std`. It can also emit LLVM IR or an object file:

```powershell
Magma.exe --emit llvm --out hello.ll hello.mg
Magma.exe --emit object --out hello.obj hello.mg
```

Use `Magma.exe --clang-version` to inspect the Clang installation selected by
the compiler. Clang discovery honors `MAGMA_CLANG`, `PATH`, `LLVM_HOME`, and
`LLVM_PATH`, then checks conventional platform locations.

### Compiler options

```text
usage: magma [options] <input-file>

  --debug                 print compiler diagnostics
  --version, -v           print the compiler version
  --out, -o <path>        output path (default depends on --emit)
  --emit, -e <kind>       llvm, object, or exe (default exe)
  --opt, -O <0-3>         LLVM optimization level (default 3)
  --error-trace-slots <n> trace slots per runtime shard (default 1024)
  --safety-warnings       downgrade memory-safety diagnostics to warnings
  --target <triple>       compilation target (default: Clang native target)
  --std <directory>       override the Magma standard-library directory
  --lsp                   run the Magma language server over stdio
  --clang-version, -cv    print the resolved Clang version and path
```

Executable output and optimization level 3 are the defaults. The compiler
accepts `llvm`, `object`, and `exe` as output kinds. If `--out` is omitted, the
output name depends on the selected kind and target platform.

By default, the compiler uses the `std` directory beside the compiler
executable. `--std` is primarily useful while developing the compiler or
standard library:

```powershell
.\Magma.exe --std C:\path\to\Magma\std --lsp
```

`--error-trace-slots` accepts a power of two from 1 through 1024. The compiler
uses 64 runtime trace shards, each with the configured number of slots.

`--lsp` runs the built-in language server over standard input and output. It
provides diagnostics, completion, hover documentation, definition lookup,
import-path completion, safety quick fixes, and semantic highlighting. Use
`--safety-warnings --lsp` for migration-mode diagnostics in the editor.

## Building from Source

Source development requires [Go](https://go.dev/) 1.24.6 or later and a usable
LLVM Clang installation. From a complete development checkout:

```powershell
go build
.\Magma.exe --std .\std --out hello.exe samples\hello_world.mg
.\hello.exe
```

The explicit `--std .\std` selects the checkout's standard library instead of
the directory associated with some other installed compiler. On Windows,
`RUN_TESTS.bat` runs the Go compiler tests and compiles and runs the
standard-library tests. `BUILD_SAMPLE.bat` builds the compiler and runs the
argument-echo sample.

## Documentation and Source

- [`docs/INSTALLATION.md`](docs/INSTALLATION.md) - release installation, LLVM
  Clang setup, PATH configuration, and first use
- [`docs/COMPILER.md`](docs/COMPILER.md) - compiler CLI and language server
- [`docs/SYNTAX.md`](docs/SYNTAX.md) - language syntax
- [`docs/FEATURES.md`](docs/FEATURES.md) - language and library features
- [`docs/OWNERSHIP.md`](docs/OWNERSHIP.md) - ownership and destruction analysis
- [`docs/std/`](docs/std/) - standard-library reference
- [`samples/`](samples/) - example programs
- [`src/`](src/) - compiler implementation

The compiler pipeline loads and parses the root module and its imports, builds
scope information, specializes generics, links names, checks types, validates
that the program can be lowered, runs warning-only ownership analysis, emits
and cleans LLVM IR, and finally invokes Clang for the selected output kind.

Relevant packages include:

- `src/tokenizer` and `src/parser` for syntax;
- `src/checker` and `src/destroy_checker` for static analysis;
- `src/monomorph` for generic specialization;
- `src/lowering_validate`, `src/llvm_ir`, and `src/ir_cleaner` for validated
  LLVM output;
- `src/target` and `src/clang` for target and Clang discovery;
- `src/lsp` for editor diagnostics and language features.
