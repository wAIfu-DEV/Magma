# Magma
## Yet another programming language

Magma is a small, low-level, compiled language that currently lowers to LLVM IR.
It is trying to be practical enough to write real programs, sharp enough to cut
your desk in half, and unfinished enough that you should not point it at payroll
software unless payroll has personally wronged you.

The compiler frontend is written in Go. It tokenizes, parses, gathers scope
information, checks links and types, monomorphizes generics, emits LLVM IR, then
lets Clang turn that IR into something your operating system can be blamed for.

For the full language tour, read [SYNTAX.md](SYNTAX.md). This README is the version for people that can't read too good.

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
17. a standard library covering allocation, heap, files, buffered IO, readers,
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

## Standard Library

The `std/` directory is already doing useful low-level work:

- `allocator`, `heap`: allocation interfaces and platform heap implementations
- `io`, `file`, `reader`, `writer`, `buffered`: file and stream IO
- `errors`: error constructors, codes, messages, comparison helpers
- `strings`, `slices`, `utf8`: string/slice utilities and UTF-8/UTF-16 helpers
- `memory`, `cast`: memory operations and explicit casts
- `array`, `list`, `queue`, `linear_map`, `hash_map`, `builder`: containers and builders

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
- `SYNTAX.md`: syntax reference and current edge cases

The compiler currently accepts one input `.mg` file and writes `out.ll`.

```powershell
go build
.\Magma.exe samples\minimal.mg
clang.exe out.ll -o out.exe
.\out.exe
```

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
4. `$` is currently an ownership/reference cue, not enforced ownership.
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

Read [SYNTAX.md](SYNTAX.md), run `BUILD&RUN_TESTS.bat`, and remember: if the
generated program segfaults, that still counts as native performance.
