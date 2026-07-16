# Magma Language Analysis

## Scope and method

This report analyzes the Magma language as it is actually used in `std/*.mg`,
`std/{win,unix}/*.mg`, and `samples/*.mg`. The compiler sources and the existing
syntax documentation were used only to resolve ambiguities such as operator
precedence and parser restrictions. Consequently, this is a description of the
current implemented language, not a proposal for a future version.

The corpus presents Magma as a small, statically typed systems language. Its
surface syntax combines name-before-type declarations, explicit error
propagation, generic containers, receiver methods, and deliberately exposed
pointer operations. It compiles through LLVM and permits inline LLVM when the
language itself cannot express an operation.

## 1. Overall character

Magma is expression-oriented enough for calls, arithmetic, member access, and
inference to compose naturally, but its control-flow constructs are statements.
It has no classes, traits, exceptions, pattern matching, lambdas, or `for` loop
in the observed corpus. Instead, abstraction is built from:

- modules and explicitly aliased imports;
- structs with receiver methods;
- monomorphized generic functions, structs, and methods;
- function pointers stored in structs;
- throwing function signatures and a first-class `error` value;
- pointers, slices, fixed arrays, raw memory routines, external symbols, and
  inline LLVM.

This produces a recognizable design: high-level conveniences are supplied where
they do not hide allocation or failure, while memory management and platform
interoperation remain explicit.

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

Declarations without `pub` are module-private. Methods in the standard library
are usually not marked `pub`; their availability follows from the public owner
type and the compiler's member resolution rules.

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

The explicit terminator makes nested control flow unambiguous without braces.
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
buffer u8[64]
```

`:=` infers a local type:

```magma
a := heap.allocator()
bound := strings.countBytes(value)
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

All observed data is mutable; there is no `const`, `let`, or read-only binding.
Globals use the same `name Type` form at module scope and are zero-initialized.
The current language does not reliably support initialized global declarations.

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

Destructors may take arguments, may be throwing, and must return `void` or
`!void`. A struct may expose multiple destructors. Calls are explicit; the
compiler does not automatically insert them.

## 4. Type system

### 4.1 Primitive and built-in types

The examples exercise:

- unsigned integers: `u8`, `u16`, `u32`, `u64`, `u128`;
- signed integers: `i8`, `i16`, `i32`, `i64`, `i128`;
- floating point: principally `f64`;
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
u8[64]    # fixed array of 64 u8 values
Object**  # pointer to pointer to Object
```

Fixed arrays own inline storage and their element count is part of the type.
Slices are a pointer-and-count view. Pointers and slices share indexing syntax,
and fixed arrays are accepted by generic slice utilities in the examples.

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
The unmarked form borrows. For structs with a `destr` member, a warning-only
flow checker tracks these transfers through direct locals, assignments, calls,
returns, control flow, and explicit destructor calls. It catches common leaks,
double consumption, consuming borrows, use after transfer, and discarded owned
results. It does not model aliases, pointers, fields, indexed values, aggregate
contents, or partial moves, so it is not a memory-safety guarantee. Detailed
rules are in [OWNERSHIP.md](OWNERSHIP.md).

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
This gives Magma dynamic dispatch and dependency inversion without
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

`sizeof Type` yields the byte size of a type and is fundamental to generic
allocation:

```magma
ret try this.vtable.fn_alloc(this.impl, count * sizeof T)
```

There is no general cast operator. Numeric conversions and pointer/integer
reinterpretations are named library functions, many implemented through inline
LLVM. This makes conversions easy to find but places some type-system work in
the standard library.

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
syntax, `switch`, or `match`, so indexed traversal is deliberately explicit.

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

This is one of the language's strongest safety conveniences: cleanup remains
explicit, but is colocated with acquisition and participates in error exits.

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

`throw expr` evaluates an `error`. A nonzero error code exits the current
function; an OK error continues. This conditional behavior permits `throw
errors.ok()` without failure, although normal code does not need that idiom.

### 7.3 Manual error handling

A throwing call can be destructured into a value and error:

```magma
count u64, countErr error = file.count()
if errors.code(countErr) != 0:
    throw countErr
..
```

This gives callers a choice between concise propagation and explicit recovery.
The destructuring form is narrow: it declares exactly a value and an `error`,
and the right side must be a throwing function call. A failed value result is
zero-initialized and must not be used before checking the error.

The standard error representation has a category code and message. Platform
errors are encoded by the standard library with the high bit of the code. The
language supplies the mechanism, while error categories and constructors live in
`std/errors.mg`.

## 8. Platform and low-level facilities

### 8.1 Conditional declarations

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

### 8.2 Foreign functions

`ext` binds a Magma alias to a native symbol:

```magma
ext ext_unix_read read(fd i32, buf ptr, count u64) i64
```

The first name is used by Magma code; the second is the linked external symbol.
Arguments and the return type remain explicit, and declarations have no body.
Windows and Unix implementations use this directly for OS and C-runtime APIs.

### 8.3 Inline LLVM

`llvm "..."` injects textual LLVM IR, most often inside a function:

```magma
pub ptou(x ptr) u64:
    llvm "%x0 = ptrtoint ptr %x to i64\n"
    llvm "ret i64 %x0\n"
..
```

It implements casts, descriptor construction, aggregate extraction, and memory
intrinsics that are absent from the core syntax. It is a powerful escape hatch,
but its strings are not type-checked against surrounding Magma code. Invalid or
mismatched IR is deferred to LLVM and can compromise optimizer assumptions.

### 8.4 Memory model in practice

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
warning-only ownership analysis is intentionally local and incomplete. Fixed
arrays are zeroed, but pointer validity remains the programmer's responsibility.
Magma should therefore be classified as memory-unsafe in its current form.

## 9. Standard-library feature picture

The language is small, but the corpus demonstrates that it can support a useful
systems library:

- allocation and platform heap wrappers;
- raw copy, move, compare, and fill operations;
- owned and borrowed strings and slices;
- UTF-8 decoding and UTF-8/UTF-16 conversion;
- files, readers, writers, buffered streams, and duplex streams;
- growable arrays, lists, queues, builders, linear maps, and hash maps;
- generic sorting and searching through comparison callbacks;
- numeric formatting and parsing;
- time and path operations with Windows/Unix backends;
- a manually tagged JSON value model and serializer.

The samples further exercise benchmarking, file reading, JSON output, error
destructuring, partial-write handling, overlapping memory moves, container
consistency, integer edge cases, and Unicode conversion. This is meaningful
evidence that function pointers, generics, explicit errors, and low-level memory
operations compose beyond toy programs.

## 10. Important implementation limitations

Several restrictions matter when reading or writing present-day Magma:

1. **No automatic memory safety.** `$` drives warning-only checking for direct
   destructible locals; pointers, aliases, fields, indexed values, aggregate
   contents, and partial moves are unchecked.
2. **Incomplete static checking.** Some invalid assignments, returns, casts, or
   throwing-call uses can survive farther into lowering than a mature language
   would permit.
3. **Restricted initialized globals.** Mutable module storage is zeroed; `const`
   supports LLVM-compatible literals, addresses, and struct aggregates rather
   than general compile-time evaluation.
4. **Restricted destructuring.** Only the two-result throwing-call form is
   supported.
5. **Restricted subscripting.** Named pointer-, slice-, and array-like targets
   are reliable; indexing arbitrary call or grouped expressions is not a safe
   assumption.
6. **No high-level sum types or interfaces.** Libraries manually encode tags,
   payload storage, and vtables.
7. **No `for`, `switch`, closures, or inference-heavy generic constraints.**
   The language favors a small set of explicit constructs.
9. **Inline LLVM crosses the type boundary.** It is essential to current library
   implementation but outside the normal safety and portability guarantees.
10. **Implicit fallthrough returns exist.** The backend can synthesize a zero
    result (and OK error for throwing functions) when execution reaches a
    function end. Explicit `ret` is clearer for value-returning functions.

These are not merely theoretical concerns: the standard library contains
workarounds such as local copies before `addrof`, explicit integer casts, manual
null tests via pointer-to-integer conversion, and careful error cleanup.

## 11. Design assessment

Magma's syntax is internally coherent. Name-before-type declarations work
uniformly for fields, parameters, locals, globals, and destructured results.
Square brackets consistently denote generic arguments or element shapes, while
postfix `*`, `[]`, and `[N]` keep low-level type composition compact. The
colon/`..` block form is unusual but easy to scan and independent of indentation.

The error system is the most distinctive high-level feature. `!T`, `try`,
`throw`, and explicit `(value, error)` handling make failure visible without the
verbosity of manually checking every call. Because throwing behavior is also
present in function-pointer types, it composes with the library's interface
pattern.

The central tradeoff is that a small core pushes substantial responsibility into
conventions and library code. Interfaces are vtable structs, variants are tagged
raw storage, and conversions are LLVM-backed functions. Ownership annotations
receive useful warning-only checking, but remain deliberately short of a general
lifetime or memory-safety proof. Correctness still depends heavily on disciplined
library implementation.

In summary, the observed language is already capable of practical low-level
programs and reusable generic libraries. It is best understood as an early,
LLVM-oriented systems language with explicit allocation and errors—not as a
memory-safe language or a feature-complete general-purpose platform. Its syntax
is compact and learnable; its current risks lie less in surface complexity than
in incomplete enforcement beneath that surface.
