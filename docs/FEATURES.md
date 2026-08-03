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

A source file must begin on its first line with one module declaration:

```magma
mod json
```

The command-line compiler additionally requires its root source file to declare
`mod main`. Imported files retain their own module names.

An import gives a module path and a mandatory local alias. Standard-library
imports canonically use `std:`; project imports are relative to the importing
file:

```magma
use "std:allocator" alc
use "std:io" io
use "models/user" user
```

Imported declarations are qualified through that alias, for example
`alc.Allocator` or `io.stdout(a)`. The `.mg` extension is optional. `std:x`
resolves from the compiler's standard-library root; other paths resolve from
the importing source file. There are no wildcard or selective imports, and
aliases provide the namespace visible to the importer.

`pub use` re-exports an imported module namespace. If a module imported as
`lib` contains `pub use "std:heap" heap`, its clients can access
`lib.heap.allocator()` without exposing private declarations from `std:heap`.

`pub` marks a top-level declaration as exported:

```magma
pub new[T](a alc.Allocator) !$Array[T]:
    # ...
..
```

Functions, structs, aliases, constants, globals, and imported namespaces can be
public. Declarations without `pub` are module-private and cannot be named
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

Identifiers contain Unicode letters, digits, and underscores; names cannot
start with a digit. Case carries convention rather than special semantics:
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

An uninitialized declaration is not undefined storage: all locals, arrays, structs, pointers, and globals are zero-initialized at declaration. This is
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
main() void:
    # ...
..

pub main(args str[]) !void:
    # ...
..
```

Magma does not overload functions by parameter type. Different operations use
distinct names (`writeUint64`, `writeInt64`, `numberFloat`, `numberInt`).

### 3.3 Structs

An identifier followed by a parenthesized field list and no return type defines
a struct. Capitalization is conventional, not syntactic:

```magma
Capture(
    data u8*
    count u64
    maxChunk u64
)
```

Commas between fields are optional when newlines separate them.  
Struct values may be constructed with a complete
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

These types include:

- unsigned integers: `u8`, `u16`, `u32`, `u64`, `u128`;
- signed integers: `i8`, `i16`, `i32`, `i64`, `i128`;
- floating point: `f16`, `f32`, `f64`, and `f128`;
- `bool`, `void`, `str`, and `error`.
- generic type-erased: `ptr`, `slice`, allowing `T*` and `T[]`  respectively

`str` and `slice` are runtime descriptor values (fat pointers) rather than raw pointers. They both occupy 16 bytes on a standard 64-bit target.

`error` is likewise a first-class aggregate containing:
- a error code (0 is OK, non zero is error)
- a message provided at construction
- a trace pointer (linked list-like built on error paths)
They have the LLVM shape `{ ptr, i32, i16, i16 }`, 16 bytes wide.

Aliases transparently name an existing type and create no distinct runtime
representation:

```magma
alias Handle = ptr
pub alias Count = u64
```

Public aliases are qualified through an import like other declarations.
Trusted standard-library code uses `@compiler_known_type("...")` to map the
target-dependent aliases in `std:c` to the selected C ABI.

There are no enums or tagged unions. `std:json` demonstrates how the library
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
frequently passed through explicit functions from `use "std:cast" cast`.

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
manually built interface or vtable. `Allocator`, `Duplex`, and `ConstWriter`
point to shared immutable vtables; `Reader` and `Writer` store their callback
directly. Calls through these function-pointer fields provide dynamic dispatch
without language-level interfaces. `Executor` applies the same pattern to
type-erased task scheduling.

### 4.5 Generics

Generic parameters and arguments use square brackets:

```magma
Array[T](data T*, state State*)
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

Generic type arguments are explicit. Magma does not infer them from call
arguments or assignment context, and it has no syntax for generic constraints.

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

The language's loop construct is `while`:

```magma
i u64 = 0
while i < n:
    i = i + 1
..
```

`break` and `continue` operate on the nearest loop. There is no `for`, range
syntax, `switch`, or `match`. Indexed traversal in the corpus uses `while`.

### 6.3 Deferred execution

`defer` schedules unconditional cleanup, either as one expression or as a body:

```magma
defer a.free(path_cstr)

defer:
    stdout.close()
    stdin.close()
..
```

The examples use it for allocations and file/stream handles. Deferred work is
scope-aware and runs on normal and abnormal exits, including returns, throws,
loop control, and body fallthrough. Defers execute last-in-first-out within their
scope. A deferred body cannot contain another `defer`.

`onerror` acts exactly the same as `defer` but only executes on abnormal exits, such as failing `try` and `throw`, but does not execute on normal exits such as `ret` or leaving a nested scope:

```magma
func() !$Struct:
    resource := acquireResource()
    onerror resource.free()

    try resource.doSomething() # may be freed here

    ret Struct(field=resource) # is not freed here
..
```

This is meant to fix the issue with having to duplicate resource freeing on each error paths, something not necessarily fixed by `defer` due to its unconditional nature.

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

The standard error representation is 16 bytes on 64-bit targets: a message
pointer, 32-bit category code, 16-bit trace handle, and 16-bit message length.
Messages retain at most 65,535 bytes. Failed `throw` and `try` edges append static
source metadata to a bounded, sharded ring; successful paths do not touch the
ring. `errors.trace` returns a cursor and
`errors.printTrace` formats it without allocating. Platform errors are encoded
by the standard library with the high bit of the code. The language supplies
the mechanism, while error categories and constructors live in `std:errors`.

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

Handled errors expose the same information through `std:errors`:

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
per shard by default. Handles retain recent sites on a best-effort basis;
iteration is bounded so reused parent links cannot cycle forever. Reaching that
bound makes `isTruncated()` true. The flag accepts powers of two from 1 through
1,024 and changes the slots per shard without changing the `error` ABI.

## 8. Asynchronous work with futures

Magma's asynchronous API is a standard-library composition of
`executor.Executor`, `thread_pool.ThreadPool`, and `future.Future[T]`; `async`
and `await` are not language keywords. A future submits a throwing function and
a copied context to a type-erased executor, publishes either its value or
error, and lets the caller wait for that result.

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

`pool.executor()` returns a borrowed `Executor` view with generic
`submit[Context]`. This is the scheduler interface accepted by futures. The pool
must remain alive until every task submitted through the view completes.

### 8.2 Creating and awaiting a future

`future.new[T, Context]` takes an allocator, executor, throwing task function,
and context value. The context is copied into private work storage. Several
task inputs can be packaged in one struct:

```magma
ReadTask(source reader.Reader, allocator allocator.Allocator, count u64)

runRead(task ReadTask*) !$str:
    ret try task.source.read(task.allocator, task.count)
..

task := ReadTask(source=r, allocator=a, count=n)
scheduler := pool.executor()
pending := try future.new[str, ReadTask](a, scheduler, runRead, task)

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
use "std:win/file_impl" impl_file

@platform("linux", "android", "ios", "darwin", "freebsd", "netbsd", "openbsd")
use "std:unix/file_impl" impl_file
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

### 9.3 Native libraries and bundled runtime files

Top-level `link` declarations record native inputs used when producing an
executable. Bare logical names are passed to Clang with `-l`; paths and
file-like names resolve relative to the declaring Magma module:

```magma
link "winhttp"
link "vendor/raylib/win/raylibdll.lib"
```

`bundle` records a runtime file to copy beside the linked executable:

```magma
@platform("windows")
bundle "vendor/raylib/win/raylib.dll"
```

Both declarations are deduplicated across modules and may be selected with
`@platform`. They are ignored for LLVM and object output; bundles are copied
only after successful executable linking.

### 9.4 Exported native functions

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

### 9.5 Inline LLVM

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

### 9.6 Memory model in practice

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

- allocator interfaces plus heap, arena, scratch, fake, and debug allocators;
- raw memory operations, explicit casts, checked arithmetic, and atomics;
- owned and borrowed strings and slices, string building, UTF-8 and UTF-16
  validation and conversion, Base64, hexadecimal, and percent encoding;
- files, directories, metadata and traversal, paths, environment variables,
  native dialogs, readers, writers, buffered streams, and duplex streams;
- processes and asynchronous process execution;
- mutexes, spin locks, lockers, platform wake primitives, native threads,
  type-erased executors, dynamically sized thread pools, typed futures, and
  asynchronous reader operations;
- HTTP clients with streaming request and response bodies, plus raylib bindings;
- growable arrays, lists, queues, heaps, builders, linear maps, hash maps,
  generic iterators, sorting, searching, and pseudorandom generation;
- numeric formatting and parsing, time, CPU information, command-line argument
  and flag parsing, and common error categories;
- a manually tagged JSON parser, value model, and serializer.

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
7. **No `for`, `switch`, closures, generic type inference, or generic
   constraints.** Generic arguments are explicit at specialization sites.
8. **Inline LLVM is not type-checked by Magma.** LLVM validates and transforms
   the injected IR.
9. **Implicit fallthrough returns exist.** The backend can synthesize a zero
    result (and OK error for throwing functions) when execution reaches a
    function end.
