# Magma
## Yet another programming language

Magma is a small, low-level, compiled language that currently lowers to LLVM IR.
It is trying to be practical enough to write real programs, sharp enough to cut
your desk in half, and unfinished enough that you should not point it at payroll
software unless payroll has personally wronged you.

The compiler frontend is written in Go. It tokenizes, parses, gathers scope
information, checks links and types, monomorphizes generics, emits LLVM IR, then
lets Clang turn that IR into something your operating system can be blamed for.

For the full language tour, read [docs/SYNTAX.md](docs/SYNTAX.md). This README is the version for people that can't read too good.

## A Taste

```magma
mod main

use "../std/allocator.mg" alc
use "../std/heap.mg" heap
use "../std/io.mg" io

main(args str[]) !void:
    a alc.Allocator = heap.allocator()

    stdout := try io.stdout(a)
    defer stdout.close()

    out := stdout.writer()
    try out.writeLn("Hello, World!")
..
```

Yes, `main` can take `args str[]`. Yes, it can throw. No, this does not make it
enterprise-grade. Please stop asking.

## What Works Today

Magma currently has:

1. modules declared with `mod`
2. imports with `use "path" alias`
3. functions, throwing functions, and `void`
4. structs and methods with implicit `this`
5. generic structs, generic functions, and generic receiver methods
6. local variables with explicit types or `:=` inference
7. zero-initialized locals and globals
8. assignments to names, fields, nested fields, and indexed values
9. pointers, slices, fixed-size arrays, `sizeof`, `addrof`, `&`, `*`, and indexing
10. conditionals with `if`, `elif`, and `else`
11. `while`, `break`, and `continue`
12. `defer` statements and `defer:` blocks
13. a first-class `error` type with `try`, `throw`, and result destructuring
14. external function declarations with `ext`
15. inline LLVM strings with `llvm`
16. `@platform(...)` directives for platform-specific imports/declarations
17. ownership-transfer annotations, destructor declarations, and a warning-only
    destroy/borrow checker
18. a standard library covering allocation, heap, files, buffered IO, readers,
    writers, strings, slices, memory, UTF-8 helpers, lists, queues, maps, casts,
    and errors

Here is a less tiny example, because apparently some people will judge a language beyond just "hello world":

```magma
mod main

use "../std/allocator.mg" alc
use "../std/errors.mg" errors
use "../std/heap.mg" heap
use "../std/io.mg" io
use "../std/list.mg" list

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

mightThrow(fail bool) !u8:
    if fail:
        throw errors.failure("asked to fail")
    ..
    ret 42
..

main(args str[]) !void:
    a := heap.allocator()

    stdout := try io.stdout(a)
    defer stdout.close()

    out := stdout.writer()

    value u8, err error = mightThrow(false)
    if errors.code(err) != 0:
        throw err
    ..

    xs := try list.new[u8](a)
    try xs.pushRight(a, value)

    got u8 = try xs.popLeft(a)
    if got != 42:
        throw errors.failure("math has abandoned us")
    ..

    try out.writeLn("Passed the tiny trial.")
..
```

## Error Handling

Throwing return types use `!`:

```magma
readByte() !u8:
    ret 42
..
```

`try` unwraps a throwing call or returns the error from the current throwing
function:

```magma
byte u8 = try readByte()
```

`throw` conditionally returns an error. Throwing `errors.ok()` is a no-op,
which is weirdly elegant if you squint:

```magma
throw errors.invalidArgument("bad argument")
throw errors.ok()
```

You can also destructure a throwing call when you want to handle the error
yourself:

```magma
value u8, err error = readByte()
if errors.code(err) != 0:
    throw err
..
```

## Blocks

Blocks start with `:` and end with `..`. Indentation is for humans, and humans
are on probation.

```magma
while i < n:
    if shouldStop:
        break
    ..
    i = i + 1
..
```

The same block shape is used for functions, methods, conditionals, loops, and
multi-line `defer`.

## Ownership and Destruction

`$T` marks an ownership-transfer position. An owned return gives the result to
the caller, while an owned parameter consumes its argument. The same `T`
without `$` is borrowed:

```magma
Resource(data ptr)

destr Resource.free() void:
    # release this.data
..

make() $Resource:
    value Resource
    ret value
..

consume(value $Resource) void:
    value.free()
..
```

The `destr` modifier marks a struct member as a destructor. Destructors may have
arguments and may throw, but their result type must be `void`/`!void`. They are
not called automatically: call one explicitly or register it with `defer`.

The compiler's destroy checker follows owned destructible locals across direct
assignments, calls, returns, branches, loops, and scope exits. It warns about
leaks, consuming borrowed values, double consumption, use after transfer,
overwrites, and discarded owned results. Warnings do not stop compilation, and
the analysis intentionally does not model fields, indexed storage, pointers,
aliases, or partial moves. See [Ownership and Destruction](docs/OWNERSHIP.md).

## Standard Library

The `std/` directory is already doing useful low-level work:

- `allocator`, `heap`: allocation interfaces and platform heap implementations
- `io`, `file`, `reader`, `writer`, `buffered`: file and stream IO
- `errors`: error constructors, codes, messages, comparison helpers
- `strings`, `slices`, `utf8`: string/slice utilities and UTF-8/UTF-16 helpers
- `memory`, `cast`: memory operations and explicit casts
- `array`, `list`, `queue`, `linear_map`, `hash_map`, `builder`: containers and builders
- `http`: Windows streaming HTTP through WinHTTP
- `raylib`: initial Windows raylib 5.5 window, drawing, and input bindings

Platform-specific pieces live under `std/win/` and `std/unix/`, selected with
`@platform(...)`.

## Compiler Layout

The interesting bits live here:

- `main.go`: command-line entry point
- `src/tokenizer`: tokenization
- `src/parser`: AST construction
- `src/scope_info`: scope maps
- `src/checker`: link and type checking
- `src/monomorph`: generic monomorphization
- `src/llvm_ir`: LLVM IR lowering
- `src/ir_cleaner`: cleanup pass for emitted IR
- `std/`: Magma standard library
- `samples/`: sample programs and smoke tests
- `docs/SYNTAX.md`: syntax reference and current edge cases
- `docs/OWNERSHIP.md`: ownership, borrowing, destructors, and checker limits

The compiler accepts one input `.mg` file and emits LLVM IR by default.

```powershell
go build
.\Magma.exe samples\minimal.mg
clang.exe out.ll -o out.exe
.\out.exe
```

Compiler options:

```text
--debug                     print compiler diagnostics
--version, -v               print the compiler version
--out, -o <path>            choose the output path
--emit, -e <llvm|object|exe>
--opt, -O <0|1|2|3>         choose the LLVM optimization level
--clang-path                print the resolved Clang executable path
--clang-version, -cv        print the resolved Clang version and path
```

Object, executable, and optimized LLVM output require Clang. Magma searches
`MAGMA_CLANG` first, followed by `PATH`, `LLVM_HOME`/`LLVM_PATH`, standard LLVM
install locations, Visual Studio's LLVM directories on Windows, and common
LLVM locations on Unix and macOS.

## Get Started

You need:

1. Go matching the module requirement in [go.mod](go.mod), currently `1.24.6`
2. Clang/LLVM, because Magma emits LLVM IR and Clang does the final lowering

On Windows, the batch scripts are the paved road:

- `BUILD&RUN_TESTS.bat`: build the compiler, lower `samples/tests.mg`, compile
  `out.ll`, and run the result
- `BUILD&LOWER_SAMPLE.bat`: build and lower `samples/impl.mg`, also emitting
  optimized LLVM snapshots
- `FULL_COMP_O1&RUN_SAMPLE.bat`: build, lower `samples/minimal.mg`, compile
  with `-O1`, and run it
- `FULL_COMP_O3&RUN_SAMPLE.bat`: same idea, with `-O3`
- `COMP&RUN.bat`: compile an existing `out.ll` and run `out.exe`

If `clang.exe` is not on `PATH`, either put it there or edit the scripts. This
is not a Magma feature. This is your machine expressing itself.

Unix support exists in the standard library via `std/unix/`, but the checked-in
automation is Windows batch files. Bring your own shell commands and emotional
stability.

## Current Caveats

Magma is still a compiler project, not a lifestyle brand. The useful caveats:

1. `for` loops are planned; use `while` for now.
2. Global variables are zero-initialized; top-level initializers are still WIP.
3. Implicit struct constructors are planned, not something to rely on today.
4. Ownership checking is warning-only and limited to direct local variables;
   raw pointers, aliases, fields, indexed values, and partial moves remain unchecked.
5. The checker is incomplete. Some invalid return values, casts, throwing calls,
   and pointer adventures may get farther than they deserve.
6. There are no bounds checks, null checks, or read-only string mutation guards.
7. Inline LLVM is accepted as text. If it is nonsense, LLVM will have opinions.

See [TODO.md](TODO.md) for the current pile of sharp edges.

## Philosophy

Magma is deliberately direct: explicit blocks, explicit allocation, explicit
errors, explicit casts, explicit enough that the compiler sometimes hands you
the screwdriver and walks away.

The goal is not to become the language equivalent of a padded meeting room. The
goal is a small systems language with readable syntax, first-class errors,
generics, a usable standard library, and enough low-level escape hatches to make
bad decisions efficiently.

That may sound irresponsible, but at least it is honest.

## The End

Read [docs/SYNTAX.md](docs/SYNTAX.md), run `BUILD&RUN_TESTS.bat`, and remember: if the
generated program segfaults, that still counts as native performance.
