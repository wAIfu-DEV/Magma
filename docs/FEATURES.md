# Magma Language Analysis

## Scope and method

This report analyzes the Magma language as it is actually used in `std/*.mg`,
`std/{win,unix}/*.mg`, `std/tests/*.mg`, `tests/**/*.mg`, and `samples/*.mg`.
Compiler and runtime sources are also used to verify static-analysis, lowering,
error-trace, and concurrency behavior that cannot be established from examples
alone. Consequently, this is a description of the current implemented language,
not a proposal for a future version.

Magma is statically typed. Its surface syntax includes name-before-type
declarations, error propagation, generic containers, receiver methods, and
pointer operations. It compiles through LLVM and supports inline LLVM.

## 1. Feature overview

Calls, arithmetic, member access, and inference are expressions. Control-flow
constructs are statements.
It has no classes, traits, exceptions, pattern matching, lambdas, or `for` loop
in the observed corpus. Instead, abstraction is built from:

- modules and explicitly aliased imports;
- structs with receiver methods;
- monomorphized generic functions, structs, and methods;
- function pointers stored in structs;
- throwing function signatures and a first-class `error` value;
- pointers, slices, stack-backed arrays, raw memory routines, external symbols, and
  inline LLVM.

## 2. Lexical and structural syntax

### 2.1 Files, modules, and imports

A source file normally begins with one module declaration:

```magma
mod json
```

An import gives a relative path and a mandatory local alias:

```magma
use "allocator.mg" alc
use "../std/io.mg" io
```

Imported declarations are qualified through that alias, for example
`alc.Allocator` or `io.stdout(a)`. There is no evidence of wildcard imports,
selective imports, or hierarchical module declarations. Paths describe source
files; aliases provide the namespace visible to the importer.

`pub` marks a top-level declaration as exported:

```magma
pub new[T](a alc.Allocator) !$Array[T]:
    # ...
..
```

Functions and struct types without `pub` are module-private and cannot be named
through an import alias. Both public and private declarations remain available
inside their defining module. Methods are public by default when their owner
struct is public and do not require a separate `pub` modifier.

### 2.2 Comments, whitespace, and statement boundaries

`#` starts a line comment, including after code:

```magma
l.capacity = 8 # four slots of padding on either side
```

Newlines separate declarations and statements. Indentation is conventional but
not structural. A colon opens a body and `..` closes it:

```magma
while i < bound:
    if bytes[i] == 0:
        break
    ..
    i = i + 1
..
```

Nested control flow uses explicit terminators rather than braces.
Empty bodies are legal and occur in the tests, especially as an assertion idiom:

```magma
if true:
else:
    throw errors.failure("unreachable")
..
```

### 2.3 Identifiers and literals

The corpus uses ASCII-style identifiers containing letters, digits, and
underscores, with case carrying convention rather than special semantics:
`Value` and `Allocator` are types, while `writeValue` and `byteCount` are
functions or values.

Observed literals include:

- booleans: `true`, `false`;
- the null-like literal `none`, used for pointers and function pointers;
- decimal integers: `0`, `65535`, `-1`;
- hexadecimal integers: `0x80000000`, `0x7FF0000000000000`;
- decimal floating point: `1.75`, `0.0`;
- double-quoted strings with escapes such as `"`, `\\`, `\n`, `\r`, and
  `\t`.

There is no separate character literal in the examples; byte values are written
as integers. Negative minimum values may be expressed as arithmetic, as in
`-9223372036854775807 - 1`, avoiding a literal outside the positive signed range.

## 3. Declarations

### 3.1 Variables and initialization

Explicit declarations put the name before the type:

```magma
count u64
position u64 = 0
buffer := array u8[64]
```

`:=` infers a local type:

```magma
a := heap.allocator()
bound := value.countBytes()
```

An uninitialized declaration is not undefined storage: the examples depend on
locals, arrays, structs, pointers, and globals being zero-initialized. This is
used as a constructor substitute:

```magma
out Value
out.kind = 3
ret out
```

Ordinary assignment uses `=` and accepts names, fields, nested fields, and
indexed locations:

```magma
count = count + 1
this.position = 0
this.entries.values[i] = value
bytes[i] = 0
```

Ordinary variables are mutable; there is no `let` or read-only local binding.
Module-level immutable values use `const` with either an explicit or inferred
type:

```magma
const DEFAULT_BUFFER_SIZE u64 = 8192
const EMPTY := Value(kind=0, payload=0)
```

Constant initializers are restricted to LLVM-compatible literals, constant and
function references, global addresses, initialized arrays, and struct
aggregates; Magma does not perform general compile-time evaluation. Globals use
the same `name Type` form at module scope and accept the same restricted
initializers. They lower as thread-local storage: omitted initializers are zeroed,
and each thread has a separate instance.

`:=` is declaration syntax, not general assignment. Its left side is a simple
new name, so field or indexed inference such as `obj.field := x` is not part of
the language.

### 3.2 Functions

A function declaration has a name, named typed parameters, a mandatory return
type, and a body:

```magma
finite(value f64) bool:
    # ...
    ret result
..
```

`void` denotes no returned value, and `ret` may then omit its expression.
Calls use conventional parentheses and may be either statements or expressions.
Trailing commas are accepted in call and declaration lists.

The executable entry points show both supported shapes:

```magma
main() !void:
    # ...
..

pub main(args str[]) !void:
    # ...
..
```

There is no overloading visible in the corpus. Different operations use distinct
names (`writeUint64`, `writeInt64`, `numberFloat`, `numberInt`).

### 3.3 Structs

A capitalized name followed by a parenthesized field list defines a struct:

```magma
Capture(
    data u8*
    count u64
    maxChunk u64
)
```

Commas between fields are optional when newlines separate them; standard library
code uses both styles. Struct values may be constructed with a complete
`Type(field=value, ...)` named-field list.

Structs are value types in ordinary declarations and returns. A pointer suffix
is used when identity or mutation through a shared object is required.

### 3.4 Methods and `this`

A method is a top-level function whose name is qualified by an owner type:

```magma
File.close() !void:
    if this.open:
        try impl_file.closeFile(this.handle)
        this.open = false
    ..
..
```

`this` is implicit and pointer-like. The method declaration does not spell out a
receiver parameter, yet assignments through `this` mutate the caller. Calls use
ordinary member syntax: `file.close()`.

Methods must follow their owner struct in the same source file. Prefixing a
member with `destr` marks it as a destructor:

```magma
destr File.close() !void:
    # ...
..
```

Destructors may take arguments and return any ordinary or throwing type. A
struct may expose multiple destructors. Calls are explicit; the compiler does
not automatically insert them.

## 4. Type system

### 4.1 Primitive and built-in types

The examples exercise:

- unsigned integers: `u8`, `u16`, `u32`, `u64`, `u128`;
- signed integers: `i8`, `i16`, `i32`, `i64`, `i128`;
- floating point: `f16`, `f32`, `f64`, and `f128`, with `f32` and `f64`
  exercised by the current library and tests;
- `bool`, `void`, `ptr`, `str`, `slice`, and `error`.

`str` and `slice` are runtime descriptor values rather than raw pointers. Tests
expect both a typed slice such as `u8[]` and the raw `slice` type to occupy 16
bytes on a 64-bit target. Library LLVM snippets reveal a pointer-and-length
layout. `error` is likewise a first-class aggregate; library code extracts a
numeric code and message string from it.

There are no enums or tagged unions. `std/json.mg` demonstrates how the library
constructs one manually using a `u8` kind tag plus `u128` payload storage.

### 4.2 Postfix type constructors

Memory-related type syntax is postfix:

```magma
u8*       # pointer to u8
u8[]      # slice of u8
array u8[64] # zero-initialized local backing storage, returned as u8[]
Object**  # pointer to pointer to Object
```

Array expressions own local inline storage and produce a typed slice (`T[]`);
their element count is not part of type identity.
Slices are a pointer-and-count view. Pointers and slices share indexing syntax,
and stack-backed arrays are accepted by generic slice utilities in the examples.

The untyped `ptr` is the raw interoperation type. Typed and raw pointers are
frequently passed through the explicit functions in `std/cast.mg`.

### 4.3 Ownership marker `$`

`$` appears on return types such as `!$u8*`, `!$str`, and `!$T[]`:

```magma
pub utf8To16(a alc.Allocator, s str) !$u16[]:
    # ...
..
```

Prefix `$` marks ownership transfer without changing runtime layout. On a return
type it gives ownership to the caller; on a parameter it consumes the argument.
The unmarked form borrows. For structs with a `destr` member, and for primitives
with a registered destructor such as `str`, a warning-only flow checker tracks
these transfers through direct locals, assignments, calls,
returns, struct-constructor fields, control flow, and explicit destructor calls.
It catches common leaks, double consumption, consuming borrows, use after
transfer, and discarded owned results. A struct constructor consumes tracked
locals placed in its fields, but the checker does not subsequently model the
aggregate's contents. It also does not model aliases, pointers, field or indexed
state, or partial moves, so it is not a memory-safety guarantee. Detailed rules
are in [OWNERSHIP.md](OWNERSHIP.md).

### 4.4 Function types and interface-like structs

Function types are written as a parenthesized list of parameter types followed
by a return type:

```magma
(ptr, u64) !u8*
(ptr, str) !u64
(ptr, u8*) void
```

They appear as struct fields:

```magma
AllocatorVTable(
    fn_alloc (ptr, u64) !u8*,
    fn_free  (ptr, u8*) void,
)

Allocator(impl ptr, vtable AllocatorVTable*)
```

Together, an opaque implementation pointer and function-pointer fields form a
manually built interface or vtable. `Allocator` points to a shared immutable
`AllocatorVTable` and `DuplexVTable`; `Reader` and `Writer` embed their single function pointer.
Calls through these function-pointer fields provide dynamic dispatch without
language-level interfaces.

### 4.5 Generics

Generic parameters and arguments use square brackets:

```magma
Array[T](data T*, capacity u16)
new[T](a alc.Allocator) !$Array[T]
arr.new[Value](a)
```

Generic receiver methods repeat the owner parameters:

```magma
Array[T].pushRight(a alc.Allocator, item T) !void:
    # ...
..
```

Methods may also introduce their own type parameters, as demonstrated by
allocator methods such as `Allocator.allocT[T]`. Generic types can be nested and
import-qualified. The design is parametric rather than subtype-based: constraints
are not expressed, and generic algorithms receive operations explicitly as
function pointers when needed. For example, search and sort routines accept a
comparison function.

## 5. Expressions and operators

Primary expressions include names, literals, grouping, calls, member access,
generic calls, indexing, `try`, `sizeof`, and `addrof`. Postfix operations chain:

```magma
this.entries.valuesView()
this.entries.values[i]
```

Observed unary operations are:

- `-x`: numeric negation;
- `!x`: logical negation;
- `~x`: bitwise complement;
- `*p`: pointer dereference;
- `&x` and `addrof x`: address-like operations.

The compiler precedence, from tightest binary group to loosest, is:

1. `*`, `/`, `%`
2. `+`, `-`
3. `<<`, `>>`
4. `==`, `!=`, `<`, `>`, `<=`, `>=`
5. `&`
6. `^`
7. `|`
8. `&&`
9. `||`
10. `=`
11. `:=`

Assignment is right-associative; ordinary arithmetic and logical operators are
left-associative. Parentheses are used extensively where bit-level expressions
mix shifts, masks, and comparisons.

`sizeof Type` yields the byte size of a type and is used in generic allocation:

```magma
ret try this.vtable.fn_alloc(this.impl, count * sizeof T)
```

There is no general cast operator. Numeric conversions and pointer/integer
reinterpretations are named library functions, many implemented through inline
LLVM.

## 6. Control flow

### 6.1 Conditions

Magma uses a single explicitly terminated `if`/`elif`/`else` chain:

```magma
if c == 0:
    ret "ok"
elif c == 1:
    ret "unexpected"
else:
    ret "unknown"
..
```

Conditions are boolean. The standard library commonly writes explicit tests
such as `flag == false`, although `!flag` is syntactically available.

### 6.2 Loops

The only demonstrated loop is `while`:

```magma
i u64 = 0
while i < n:
    i = i + 1
..
```

`break` and `continue` operate on the nearest loop. There is no `for`, range
syntax, `switch`, or `match`. Indexed traversal in the corpus uses `while`.

### 6.3 Deferred execution

`defer` schedules cleanup, either as one expression or as a body:

```magma
defer a.free(path_cstr)

defer:
    stdout.close()
    stdin.close()
..
```

The examples use it for allocations and file/stream handles. Deferred work is
scope-aware and runs on normal or abnormal exits, including returns, throws,
loop control, and body fallthrough. Defers execute last-in-first-out within their
scope. A deferred body cannot contain another `defer`.

## 7. Error model

### 7.1 Throwing signatures

Prefixing a return type with `!` declares a function that may fail:

```magma
read(a alc.Allocator, n u64) !$str:
    # ...
..
```

This is part of the static function type, including function-pointer fields.
Failure is not an untyped exception; it is an additional `error` result carried
by a throwing call.

### 7.2 Propagation with `try` and `throw`

`try` unwraps a successful throwing result and returns the error from the current
throwing function on failure:

```magma
line str = try reader.readLn(a)
try writer.writeAll(line)
```

`throw expr` accepts an `error` or string. A string becomes a failure with that
message. A nonzero error code exits the current function; an OK error continues.
This permits `throw errors.ok()` without failure, although normal code does not
need that idiom. The checker rejects `throw` in a non-throwing function.

### 7.3 Manual error handling

A throwing call can be destructured into a value and error:

```magma
count u64, countErr error = file.count()
if countErr.nok():
    throw countErr
..
```

Both binding types can instead be inferred from the throwing call:

```magma
count, countErr := file.count()
```

Callers can use propagation or explicit recovery.
The destructuring form is narrow: it declares exactly a value and an `error`,
and the right side must be a throwing function call. A failed value result is
zero-initialized and must not be used before checking the error.

The standard error representation has a category code, message, and an
allocation-free propagation trace. Failed `throw` and `try` edges append static
source metadata to a bounded, sharded ring; successful paths do not touch the
ring. `errors.trace` returns a cursor and
`errors.printTrace` formats it without allocating. Platform errors are encoded
by the standard library with the high bit of the code. The language supplies
the mechanism, while error categories and constructors live in `std/errors.mg`.

### 7.4 Propagation stack traces

Every failed `throw` and `try` edge records the current function, source-file
basename, line, and column. If an error escapes `main`, the runtime prints the
error followed by its propagation sites, newest first:

```text
Uncaught Error: 1 'fake alloc'
  at main (async_test.mg:21:14)
  at Async.read (async.mg:36:9)
  at new[str, async.ReaderReadTask] (future.mg:92:9)
  at Allocator.allocT[future.Work[str, async.ReaderReadTask]] (allocator.mg:37:9)
  at fakeAlloc (fake_alloc.mg:9:5)
```

Trace names use source-level names rather than LLVM linker symbols. Generic
frames retain their readable type arguments, including nested generic and
qualified types. The trace describes error propagation rather than every
native call frame: functions which neither throw nor propagate the error do not
appear.

Handled errors expose the same information through `std/errors.mg`:

```magma
value, failure := mayFail()
if failure.nok():
    errors.printTrace(failure)

    cursor := errors.trace(failure)
    while cursor.isEmpty() == false:
        # cursor.function(), cursor.file(), cursor.line(), cursor.column()
        cursor = cursor.next()
    ..
    if cursor.isTruncated():
        # Some older frames were overwritten by later errors.
    ..
..
```

A `Trace` requires no cleanup, and its accessors are valid only while it is
non-empty. Trace nodes live in a thread-safe ring with 64 shards and 1,024 slots
per shard by default. A reused node ends iteration safely and makes
`isTruncated()` true; formatted traces print the configured capacity and suggest
increasing `--error-trace-slots`. The flag accepts powers of two from 1 through
65,536 and changes the slots per shard without changing the `error` ABI.

## 8. Asynchronous work with futures

Magma's asynchronous API is a standard-library composition of
`thread_pool.ThreadPool` and `future.Future[T]`; `async` and `await` are not
language keywords. A future submits a throwing function and a copied context to
a pool, publishes either its value or error, and lets the caller wait for that
result.

### 8.1 Creating a pool

Pools require an allocator and own their worker, queue, and synchronization
storage. They start with their configured minimum worker count, grow toward the
configured maximum when the available workers are occupied, and shrink to the
minimum after bursts drain. `new` exposes both worker limits, the initial queue
capacity, and the worker spin count. `newDefault` starts one worker per available
CPU core, uses an initial queue capacity of 256, derives its spin count from the
core count, and requests the largest `u64` worker limit. The current
implementation initially allocates bookkeeping for only the minimum workers and
doubles that storage on demand, capped by the configured maximum. Worker
contexts have stable individual allocations, so growing the bookkeeping arrays
does not invalidate running workers.

```magma
pool := try thread_pool.newDefault(a)
defer pool.close()

# Or configure minimum and maximum workers, queue capacity, and spin count:
pool := try thread_pool.new(a, 2, 8, 256, 1024)
defer pool.close()
```

`ThreadPool.submit(entry, context)` is the low-level `(ptr) u64` task API. The
queue grows through the pool allocator when full. `pool.wait()` blocks until all
submitted work completes. `pool.close()` first waits for pending work, stops and
joins every worker, and releases the pool; it must not race with new submissions.

### 8.2 Creating and awaiting a future

`future.new[T, Context]` takes an allocator, pool, throwing task function, and
context value. The context is copied into private work storage. Several task
inputs can be packaged in one struct:

```magma
ReadTask(source reader.Reader, allocator allocator.Allocator, count u64)

runRead(task ReadTask*) !$str:
    ret try task.source.read(task.allocator, task.count)
..

task := ReadTask(source=r, allocator=a, count=n)
pending := try future.new[str, ReadTask](a, pool, runRead, task)

if try pending.isDone():
    # Polling is optional; await also works before completion.
..

contents := try pending.await()
```

`isDone()` is a non-blocking status check. `await()` blocks using the platform
address-wait primitive rather than busy-waiting, then returns the task value or
rethrows its error. It is a destructor and consumes the future: a future can be
awaited only once, and calling either method after consumption is an error.
Future state is reference-counted so it remains alive if the future is awaited
before or after the worker finishes.

The allocator and every resource referenced by the copied context must remain
valid until the task completes. Copying an interface-like value such as
`Reader` copies its pointers, not the storage to which they refer. Always await
or otherwise finish outstanding work before closing the pool or destroying
those resources. Tasks which block awaiting other tasks on the same fully
occupied pool can deadlock through worker starvation.

### 8.3 Async execution context

`async.Async` bundles a borrowed pool and allocator. Its `read` operation
packages `Reader.read` as a future without making `std:reader` depend on the
thread-pool stack:

```magma
pool := try thread_pool.newDefault(a)
defer pool.close()
as := async.new(pool, a)

f := try file.open(a, "main.go", file.mode().read())
defer f.close()

pending := try as.read(f.reader(), f.count())
contents := try pending.await()
```

The returned string is allocator-owned, just as with synchronous `Reader.read`.
If work fails on a worker, its existing propagation trace is stored with the
error. `await()` rethrows that same error and adds the awaiting path, so an
uncaught trace connects the caller to the asynchronous task's failure origin.

## 9. Platform and low-level facilities

### 9.1 Conditional declarations

`@platform(...)` applies to the next top-level item:

```magma
@platform("windows")
use "win/file_impl.mg" impl_file

@platform("linux", "android", "ios", "darwin", "freebsd", "netbsd", "openbsd")
use "unix/file_impl.mg" impl_file
```

This permits both branches to use the same alias and present one portable module
API. It is item-level conditional compilation, not a general compile-time
expression system.

### 9.2 Foreign functions

`ext` binds a Magma alias to a native symbol:

```magma
ext ext_unix_read read(fd i32, buf ptr, count u64) i64
```

The first name is used by Magma code; the second is the linked external symbol.
Arguments and the return type remain explicit, and declarations have no body.
Windows and Unix implementations use this directly for OS and C-runtime APIs.

### 9.3 Exported native functions

`@export_name` exposes a top-level Magma function under a stable native symbol:

```magma
@export_name("magma_add")
add(a i32, b i32) i32:
    ret a + b
..
```

Magma continues to emit and use the ordinary module-mangled implementation. A
second function named `magma_add` forwards its arguments and result, allowing C
code to declare and call it normally:

```c
#include <stdint.h>

extern int32_t magma_add(int32_t a, int32_t b);
```

The optional second argument selects the ABI. It defaults to C, and C is the
only ABI currently supported:

```magma
@export_name("magma_add", "C")
```

Native export visibility and Magma module visibility are separate. Add `pub`
only when other Magma modules must also access the function. Symbol names must
be valid C identifiers and unique across every module in the compilation.

Exported functions must be top-level, concrete, and non-throwing. Generic and
member functions have no single stable native signature. Throwing results use
Magma's internal error aggregate, which does not have a supported C ABI. Expose
a throwing operation through an explicit non-throwing adapter instead, for
example by returning an integer status and writing the successful value through
a pointer argument.

### 9.4 Inline LLVM

`llvm "..."` injects textual LLVM IR, most often inside a function:

```magma
pub ptou(x ptr) u64:
    llvm "%x0 = ptrtoint ptr %x to i64\n"
    llvm "ret i64 %x0\n"
..
```

It implements casts, descriptor construction, aggregate extraction, and memory
intrinsics that are absent from the core syntax. Its strings are not type-checked
against surrounding Magma code. Invalid or
mismatched IR is deferred to LLVM and can compromise optimizer assumptions.

### 9.5 Memory model in practice

Allocation is explicit and allocator-driven. Resource-owning structs expose
explicit `destr` methods such as `free` or `close`; callers invoke or defer them.
The destroy checker warns if a tracked owner is not consumed on every path.
Pointer arithmetic is normally performed by converting pointers to
`u64`, doing byte arithmetic, and converting back:

```magma
next ptr = cast.utop(cast.ptou(base) + offset)
```

There are no bounds checks, null checks, general lifetime or alias checks,
data-race rules, or protection against writing string literal storage. The
warning-only ownership analysis is local and does not cover all values. Fixed
arrays are zeroed, but pointer validity is not checked.

## 10. Standard-library features

The corpus contains the following standard-library functionality:

- allocation and platform heap wrappers;
- raw copy, move, compare, and fill operations;
- owned and borrowed strings and slices;
- UTF-8 decoding and UTF-8/UTF-16 conversion;
- files, readers, writers, buffered streams, and duplex streams;
- worker thread pools, typed futures, and asynchronous reader operations;
- processes and asynchronous process execution;
- atomics, mutexes, spin locks, lockers, and platform wake primitives;
- HTTP clients and streaming request and response bodies;
- generic iterators and pseudorandom number generation;
- growable arrays, lists, queues, builders, linear maps, and hash maps;
- generic sorting and searching through comparison callbacks;
- numeric formatting and parsing;
- time and path operations with Windows/Unix backends;
- a manually tagged JSON value model and serializer.

The samples exercise benchmarking, file reading, JSON output, error
destructuring, partial-write handling, overlapping memory moves, container
consistency, integer edge cases, and Unicode conversion.

## 11. Implementation limits

The current implementation has the following restrictions:

1. **No automatic memory safety.** `$` drives warning-only checking for direct
   destructible locals and transfers into struct constructors; pointers,
   aliases, field and indexed state, aggregate contents, and partial moves
   remain unchecked.
2. **Permissive compatibility rules.** Initializers, assignments, arguments,
   returns, and operator families are checked, but numeric types are broadly
   compatible and pointer compatibility is permissive. Some
   narrowing or representation-changing conversions are warnings rather than
   errors.
3. **Restricted thread-local globals.** Mutable module storage is per-thread and
   accepts only LLVM-compatible initializers. `const` supports literals,
   references, initialized arrays, addresses, and struct aggregates rather than
   general compile-time evaluation.
4. **Restricted destructuring.** Only the two-result throwing-call form is
   supported.
5. **Type-directed subscripting.** Postfix indexing can target general
   expressions, including calls and grouped expressions, but the resulting
   target must have a pointer, slice, or fixed-array type.
6. **No built-in sum types or interfaces.** Libraries manually encode tags,
   payload storage, and vtables.
7. **No `for`, `switch`, closures, or inference-heavy generic constraints.**
8. **Inline LLVM is not type-checked by Magma.** LLVM validates and transforms
   the injected IR.
9. **Implicit fallthrough returns exist.** The backend can synthesize a zero
    result (and OK error for throwing functions) when execution reaches a
    function end.
