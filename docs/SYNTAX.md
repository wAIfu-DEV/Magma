# Magma Syntax

This document describes the Magma syntax.

## Source Files

A Magma file is a module. The first meaningful line normally declares the module:

```magma
mod main
```

Module names are simple identifiers. Files in the standard library use names such
as `io`, `file` and `heap_impl_win`

Imports use `use`, a string path, and a local alias:

```magma
use "../std/io.mg" io
use "allocator.mg" alc
```

The alias is the name used from the importing file:

```magma
stdout := io.stdout(a)
```

Comments start with `#` and continue to the end of the line:

```magma
# This is a comment.
value u64 = 10 # This is also a comment.
```

Newlines separate top-level declarations and statements. Indentation is
conventional only; blocks are delimited explicitly with `:` and `..`.

## Identifiers and Literals

Identifiers contain letters, digits, and underscores. They are used for modules,
variables, functions, types, fields, aliases, and generic parameters:

```magma
my_var u64 = 0
OpenMode(read bool, write bool, append bool)
```

Boolean literals are:

```magma
true
false
```

String literals are written with double quotes:

```magma
"Hello, World!\n"
```

Supported string escapes include `\n`, `\r`, `\t`, `\\`, `\"`, `\'`, `\a`,
`\b`, `\f`, and `\v`.

Number literals may be decimal integers, decimal floating-point values, negative
numbers, or hexadecimal integers:

```magma
42
-1258
3.14
0xFFFFFFFF
0x7FF0000000000000
```

## Blocks

Blocks begin with `:` and end with `..`.

```magma
if condition:
    doSomething()
..
```

The same block syntax is used for functions, conditionals, loops, and multi-line
`defer` statements:

```magma
main() void:
    defer:
        cleanupA()
        cleanupB()
    ..
..
```

Empty blocks are valid:

```magma
if true:
else:
    fail()
..
```

## Types

Magma uses postfix type modifiers for pointers, slices, and arrays.

Basic built-in types include:

```magma
void
bool
error
ptr
str
slice
u8 u16 u32 u64
i8 i16 i32 i64
f32 f64
```

The compiler also recognizes wider and smaller numeric spellings used by the
backend type table:

```magma
u128 i128
f16 f128
```

Named user types are written by name:

```magma
File
Allocator
```

Imported type names use dotted names where the prefix is the import alias:

```magma
alc.Allocator
writer.Writer
ta.Pair[u8, u16]
```

Pointer types use postfix `*`:

```magma
u8*
File*
ta.Pair[u8, u16]*
```

Slices use postfix `[]`:

```magma
str[]
u8[]
T[]
```

Fixed-size array definition use postfix `[N]`:

```magma
u8[16]
str[3]
```

The same expression syntax is used for element access on pointers and slices:

```magma
buf[idx] = 1
first u8 = start[0]
```

Magma also accepts an ownership/reference marker `$` before or after types:

```magma
heap_ptr $MyStruct* = try heap.alloc(sizeof MyStruct)
Allocator.alloc(byteCount u64) !$u8*:
    ret try this.vtable.fn_alloc(this.impl, byteCount)
..
```

When `$` appears before the base type, it marks an ownership-transfer position
and does not change the backend type. `$T` returns produce ownership, `$T`
parameters consume it, and unmarked `T` positions borrow. The warning-only
destroy checker applies these rules to direct locals whose struct type declares
a destructor. When `$` appears after an already parsed type, it is the older
reference-like form and currently lowers as `ptr`; do not confuse it with the
prefix ownership annotation.

### Destructors and the destroy checker

`destr` is a modifier on a struct member function declared after its owner:

```magma
Resource(handle ptr)

destr Resource.close() !void:
    # release this.handle, or throw
..
```

A destructor may take arguments and may return `void` or `!void`. A struct may
have multiple destructor methods. Calling any marked destructor consumes its
receiver. Destructors are explicit rather than automatically inserted; use a
direct call or `defer value.close()` on every owning path.

The checker warns about unconsumed owners, consuming borrows, repeated
consumption, use after transfer, overwriting live owners, pending deferred
destructors during transfer, and discarded owned destructible results. It does
not reject compilation and only tracks direct locals. Fields, indexed values,
pointers, aliases, aggregate contents, and partial moves are outside its model.
See [OWNERSHIP.md](OWNERSHIP.md) for the complete behavior and examples.

Throwing return types use prefix `!` on a return type:

```magma
main() !void:
    ret
..

readLn(a alc.Allocator) !$str:
    ret try readString(a)
..
```

Only return types and function types may be throwing. Local variables cannot have
throwing types.

Function-pointer types are written as an argument type list followed by a return
type:

```magma
fn_alloc   (ptr, u64) !u8*
fn_realloc (ptr, u8*, u64) !u8*
fn_free    (ptr, u8*) void
```

They are commonly used inside interface-like structs:

```magma
AllocatorVTable(
    fn_alloc   (ptr, u64) !u8*,
    fn_realloc (ptr, u8*, u64) !u8*,
    fn_free    (ptr, u8*) void,
)

Allocator(impl ptr, vtable AllocatorVTable*)
```

Function type argument lists contain types only, not argument names:

```magma
(ptr, u8[], u64) !u64
```

## Variables and Assignment

Variable declarations use `name type`:

```magma
count u64
file File
```

Declarations without an initializer are zero-initialized:

```magma
v0 i64
buffer u8[16]
```

Declarations with initializers use `=`:

```magma
count u64 = 0
name str = "magma"
```

Type inference uses `:=`:

```magma
a := heap.allocator()
stdout := try io.stdout(a)
nested := tc.wrapPair[u8, u16](11, 22)
```

Assignments reuse `=`:

```magma
count = count + 1
f.open = false
array[0] = 42
```

Assignments may target names, fields, or indexed values:

```magma
myStruct.field = 45
myStruct.nested.field = 542
ptr.second_field = 50.0
bytes[i] = 0
```

Global variables use the same `name type` form at top level:

```magma
out writer.Writer
gl_heap ptr
```

Mutable globals remain zero-initialized. Immutable globals support explicit or
inferred types:

```magma
const count u64 = 4
const table := VTable(fn_call=callback)
```

Constant initializers currently support literals, function/global addresses,
and nested struct constructors. Mutation checks are not implemented yet.

## Functions

Function definitions are:

```magma
name(arg0 Type0, arg1 Type1) ReturnType:
    statements
..
```

Examples:

```magma
retOne() i64:
    ret 1
..

throwing(isThrowing bool) !i32:
    if isThrowing:
        throw errors.failure("from throwing func")
    ..
    ret 2
..
```

The return type is required. Use `void` for no value:

```magma
close() void:
    ret
..
```

Arguments use `name type`, separated by commas:

```magma
open(a alc.Allocator, path str, openMode fopm.OpenMode) !$File:
    ...
..
```

Trailing commas are accepted in argument lists, especially in multi-line struct
definitions.

Functions are called with parentheses:

```magma
retOne()
heap.alloc(8)
io.stdout(a)
writer.new(this, write)
```

Calls can be statements or expressions:

```magma
stdout.flush()
bytesLen u64 = strings.countBytes(bytes)
```

The `pub` modifier exports top-level declarations from a module:

```magma
pub open(a alc.Allocator, path str, openMode fopm.OpenMode) !$File:
    ...
..
```

## Return, Void, and Empty Values

`ret` returns from the current function:

```magma
ret value
```

In a `void` function, `ret` may appear without an expression:

```magma
main() void:
    ret
..
```

The expression `()` is parsed as a void expression:

```magma
ret ()
```

## Structs

Structs are declared with a name followed by a parenthesized field list:

```magma
File(
    handle ptr
    openMode fopm.OpenMode
    open bool
)
```

Fields use the same `name type` order as variables and arguments. Commas are
accepted and commonly used:

```magma
Pair[A, B](
    left A,
    right B,
)
```

Struct values are zero-initialized when declared:

```magma
f File
p Pair[u8, u16]
```

Fields are accessed with `.`:

```magma
f.open = true
p.left = left
```

Struct values can be constructed with a complete named-field list:

```magma
my_struct MyStruct = MyStruct(first_field=0, second_field=5.0)
```

Every field must be present exactly once. Field order is unrestricted. An empty
parenthesized list remains an ordinary function call.

## Methods

Methods are functions declared with `Owner.method` at top level:

```magma
File.close() void:
    if this.open:
        impl_file.closeFile(this.handle)
        this.open = false
    ..
..
```

Inside a method, `this` is an implicit pointer to the receiver. Assigning through
`this` mutates the receiver:

```magma
Writer.flush() !u64:
    this.position = 0
    ret totalWritten
..
```

Methods are called with dot syntax:

```magma
stdout.close()
reader.readLn(a)
p.swap()
```

Member methods must be defined after their owner struct in the same file.

Generic receiver methods include the receiver type parameters on the owner:

```magma
List[T].pushRight(a alc.Allocator, item T) !void:
    ...
..

Pair[A, B].swap() Pair[B, A]:
    ...
..
```

Methods may also have their own type parameters:

```magma
Box[T].map[U](out U) U:
    ret out
..
```

## Generics

Generic type parameters are declared in square brackets after a struct or
function name:

```magma
Pair[A, B](
    left A,
    right B,
)

pub pairOf[A, B](left A, right B) Pair[A, B]:
    ...
..
```

Generic arguments are also written in square brackets:

```magma
p Pair[u8, u16] = pairOf[u8, u16](7, 9)
l lst.List[u8] = try lst.new[u8](a)
```

Generic arguments can be nested:

```magma
tb.Bucket[ta.Pair[T, U]]
tb.Bucket[ta.Pair[u8, u16]]
```

Generic function calls can sometimes be inferred from assignment context:

```magma
myVar := tb.makeBucket[u8](33, 4)
nested := tc.wrapPair[u8, u16](11, 22)
```

When inference is insufficient or clarity matters, pass explicit type arguments:

```magma
ta.pairOf[T, U](left, right)
b.map[u16](2)
```

Generic parameter lists and argument lists cannot be empty:

```magma
# Invalid:
Box[](...)
make[](...)
```

## Expressions

Primary expressions include literals, names, grouped expressions, calls, indexes,
`try`, `sizeof`, and `addrof`.

```magma
42
"text"
myName
(a + b)
func(arg)
items[i]
try mightThrow()
sizeof u64
addrof value
```

Unary operators:

```magma
-x      # numeric negation
!x      # unary not
*x      # dereference-like unary operator
&x      # unary address/reference-like operator
~x      # bitwise not
```

Binary operators, from highest to lowest precedence:

```magma
* / %
+ -
<< >>
== != < > <= >=
&
^
|
&&
||
=
:=
```

Examples:

```magma
available u64 = this.filled - this.position
if readCount == 0:
    this.eof = true
..
if openMode.read && openMode.write == false:
    flags = O_RDONLY
..
outPtr[i] = u32to16((v >> 10) + 55296)
```

Use parentheses to make complex expressions explicit:

```magma
if (exp == EXP_MASK) && (frac != 0):
    ret try this.fn_write(this.impl, "nan")
..
```

## Control Flow

### Conditionals

Conditionals use `if`, optional `elif`, optional `else`, and block delimiters:

```magma
if condition:
    ...
elif otherCondition:
    ...
else:
    ...
..
```

The final `..` closes the whole chain. `elif` and `else` appear where a normal
statement would otherwise continue the current `if` body:

```magma
if myBoolFalse:
    out.writeLn("failure")
elif myBoolTrue:
    out.writeLn("success")
else:
    out.writeLn("fallback")
..
```

### While Loops

Loops use `while`:

```magma
while i < n:
    i = i + 1
..
```

`break` exits the nearest loop:

```magma
while true:
    if done:
        break
    ..
..
```

`continue` skips to the next iteration:

```magma
while i < n:
    i = i + 1
    if shouldSkip:
        continue
    ..
    process(i)
..
```

Indexed for loops are planned but currently WIP, prefer using this rather explicit syntax for the moment:

```
i u64 = 0
while i < n:
    defer i = i + 1

    statements
..
```

## Errors

Magma has a first-class `error` type and throwing return types.

A function whose return type is prefixed with `!` can throw:

```magma
retErr() !void:
    throw errors.failure("from retErr")
..
```

`throw expr` conditionally returns the error if it is non-OK. Throwing OK is a
no-op:

```magma
throw errors.ok()
throw errors.invalidArgument("invalid open mode")
```

`try expr` evaluates a throwing expression and automatically rethrows on error:

```magma
handle ptr = try impl_file.openFile(a, path, openMode)
line str = try stdin.readLn(a)
```

`try` can only be used inside a throwing function.

Throwing functions can return a value by `ret`:

```magma
alloc(nBytes u64) !$u8*:
    ret try heapAlloc(cast.utop(0), nBytes)
..
```

To handle errors manually, destructure a throwing call into a value and an
`error`:

```magma
myVal i32, myE error = throwing(true)
if errors.code(myE) != 0:
    handle(myE)
..
```

Destructuring currently supports a function call on the right-hand side:

```magma
value T, err error = someThrowingCall()
```

The value is valid when the error is OK. On error, the value is zero initialized, never rely on the value without checking for error status first.

## Defer

`defer` registers work to run when control leaves the current function or nested
body. Function-level defers run before the function returns; defers inside nested
bodies run when that body exits.

Single-expression defer:

```magma
defer stdout.close()
defer heap.free(allocOnHeap)
```

Block defer:

```magma
defer:
    stdout.close()
    stdin.close()
..
```

Deferred blocks cannot contain nested `defer` statements.

`defer` is useful for resource cleanup around throwing code:

```magma
path_cstr u8* = try strings.toCstr(a, path)
defer a.free(path_cstr)
fd i32 = ext_unix_open(path_cstr, flags, mode)
```

## Compiler Directives

Compiler directives begin with `@`. The only currently implemented directive is `platform`.

```magma
@platform("windows")
use "win/file_impl.mg" impl_file

@platform("linux", "android", "ios", "darwin", "freebsd", "netbsd", "openbsd")
use "unix/file_impl.mg" impl_file
```

`@platform(...)` applies to the next top-level declaration or import. If the
current compiler platform does not match one of the string arguments, that next
declaration is pruned.

Directive arguments must be literal constants: strings, numbers, or booleans.

## External Functions

External functions are declared with `ext`. They bind a Magma-visible alias to an
external symbol name:

```magma
ext ext_unix_open open(path u8*, flags i32, mode i32) i32
ext ext_stdlib_malloc malloc(size u64) ptr
```

The syntax is:

```magma
ext <alias> <external_symbol>(args...) ReturnType
```

The alias is the name used in code:

```magma
fd i32 = ext_unix_open(path_cstr, flags, mode)
```

External declarations have no body.

## Native Libraries

Native libraries required by external declarations are declared at top level:

```magma
link "./vendor/raylib/lib/raylib.lib"
link "winhttp"
```

`link` records a logical library name for executable emission. The compiler
deduplicates requirements across imported modules. Bare names are passed to
Clang as `-l<name>`. Values containing a path separator or file extension are
treated as library files, resolved relative to the declaring Magma file, and
passed directly to Clang. `link` does not affect LLVM or object emission and may
be selected with `@platform(...)`.

On Windows, linking an import library such as `raylib.lib` keeps the dependency
dynamic: `raylib.dll` must be beside the generated executable or otherwise on
the DLL search path at runtime. Selecting a static `.lib` later uses the same
declaration; the referenced library artifact determines the linkage kind.

Declaration of external functions from within Magma is planned but currently WIP.

## Inline LLVM

Inline LLVM is written with `llvm` followed by a string literal:

```magma
llvm "%x0 = ptrtoint ptr %x to i64\n"
llvm "ret i64 %x0\n"
```

`llvm` may appear at top level or inside function bodies. Inside functions it is
used to emit LLVM IR directly for operations not yet expressible in Magma:

```magma
pub ptou(x ptr) u64:
    llvm "%x0 = ptrtoint ptr %x to i64\n"
    llvm "ret i64 %x0\n"
..
```

The string contents are passed through as LLVM text; Magma syntax checking does
not validate LLVM correctness.

Inline LLVM is the platform-agnostic equivalent to C's inline assembly, it can be used for very low level operations but give no guarantee that generated executable will feature the written code as-is. Inline LLVM is not exempt from being modified or tampered with during optimization passes.

## Memory and Low-Level Idioms

Pointers are explicit:

```magma
p ptr = cast.utop(0)
data u8* = try a.alloc(64)
```

Address-like operations appear in two forms:

```magma
addrof value
&value
```

Current standard library code mostly uses `addrof`.

Indexing pointer-like values uses square brackets:

```magma
first u8 = start[0]
outPtr[i] = high
```

`sizeof` returns the byte size of a type:

```magma
bytes u64 = sizeof T
ptrSize u64 = sizeof ptr
```

Common allocation patterns combine `sizeof`, typed pointers, and casts:

```magma
out u8* = try a.alloc(count * sizeof T)
typed T* = out
```

## Expected Behavior and Edge Cases

This section documents behavior inferred from the current tokenizer, parser,
link checker, type checker, and standard library usage. Some items are language
rules; others describe current implementation limits.

### Tokenization and Lexical Edge Cases

Whitespace separates tokens. Newlines are significant as statement/declaration
separators, but spaces and tabs are otherwise not structural. Blocks are opened
and closed only by `:` and `..`.

Comments begin with `#` outside strings and continue until the next newline.
The newline after a comment is still emitted, so comments can safely appear after
statements:

```magma
count = count + 1 # comment
```

String escapes are interpreted by the tokenizer. Unknown escape sequences do not
currently produce a tokenizer error; the backslash is dropped and the escaped
character is kept. For example, `"\q"` is tokenized as `"q"`. Prefer the
documented escape set.

Decimal number tokenization is permissive before later lowering. A leading `-`
is part of the number literal only when it is immediately followed by a digit;
otherwise `-` is parsed as an operator. Hex literals are accepted after `0x` or
`0X`, but their internal representation is backend-oriented.

Keywords such as `ret`, `if`, `while`, and `true` are reserved by the tokenizer.
They cannot be used as ordinary identifiers.

### Parsing and Statement Shapes

Only a few expression forms are valid assignment targets:

```magma
name = value
name.field = value
items[i] = value
*pointer = value
```

The `:=` form declares a new inferred local and only accepts a simple name on
the left:

```magma
value := call()

# Invalid:
obj.field := value
module.name := value
value u64 := 1
```

Destructuring has one supported shape:

```magma
value T, err error = throwingCall()
```

Both bindings are declarations. The right-hand side must parse as a function
call, the call must return `!T`, the error binding must be exactly `error`, and
`!void` calls cannot be destructured.

Function calls accept trailing commas:

```magma
call(a, b,)
```

Argument lists for declarations and struct fields accept commas or newlines as
separators. Empty function calls and empty declaration argument lists are valid,
but empty generic parameter or argument lists are not.

Subscript expressions currently require a name-like target after link checking.
Simple forms such as `items[i]` are supported; subscripting an arbitrary call or
grouped expression is not expected to work reliably.

### Scope and Name Resolution

Functions and nested `if`, `elif`, `else`, and `while` bodies have lexical local
scopes. Magma deliberately forbids shadowing: a variable, parameter, global, or
function declaration cannot reuse a visible variable or function name.
Duplicate declarations in the same scope are also rejected. Separate sibling
scopes may reuse a local name because neither declaration is visible from the
other.

Imported members are accessed through the local alias from `use`. A file cannot
reuse the same alias for two active imports, and it cannot actively import the
same path twice. Imports pruned by `@platform` are ignored for these duplicate
checks.

Member methods must be declared after their owner struct in the same file. A
member function declaration has at most two name parts:

```magma
Owner.method() void:
    ret
..
```

Inside member methods, `this` is inserted as an implicit `Owner*` first
argument. Method calls subtract that implicit argument from the call-site arity.

### Type Checking Expectations

`if` and `while` conditions must infer to `bool`. Comparisons infer to `bool`;
`&&` and `||` require both operands to be `bool`.

Bitwise operators `&`, `|`, and `^` accept integer operands. They also accept
`bool` when both sides are `bool`. Shift operators require integer operands.

The current checker performs limited assignment compatibility checks. It infers
types and validates some operator families, but it does not yet enforce every
possible mismatch between declared variable types, return expressions, and
assignment expressions. Use explicit casts from `std/cast.mg` when narrowing,
widening, or converting pointer/integer/float values.

Numeric literals initially infer as `i64`; string literals infer as `str`; bool
literals infer as `bool`. Contextual lowering may still produce the declared
destination type in generated IR.

`sizeof` returns `u64`. In current samples and tests, primitive sizes use byte
counts, `ptr` is pointer-sized, and `str`/`slice` are two-word runtime structs.

### Returns, Errors, and Defer

If a function body reaches the end without an explicit `ret`, codegen emits a
zero value for the declared return type. For throwing return types this is an OK
error plus a zero value when the return type has a value. Prefer explicit
returns for clarity.

`throw err` checks `err.code`. If the code is zero, execution continues. If the
code is nonzero, the current function returns through the throwing return path.
Throwing from a non-throwing function is not a useful pattern; throwing functions
should have a `!` return type.

`try call()` only supports throwing function calls in practice. On non-OK error
it rethrows from the current function; on OK it yields the call value. `try` is
intended for use inside throwing functions.

Deferred statements run when control leaves their current function or nested
body through `ret`, `throw`, normal fallthrough, `break`, or `continue`.
Defers execute in last-in-first-out order for the scope where they were
registered. A `defer:` body cannot contain another `defer`.

`break` and `continue` parse anywhere, but code generation rejects them outside a
loop body.

### Globals and Initialization

Mutable global variables are emitted as zero-initialized storage. General
top-level initializer syntax is not supported; use restricted `const`
initializers for LLVM constants:

```magma
counter u64        # valid global, zero-initialized
counter u64 = 1    # not the supported global form
const counter_value u64 = 1
```

Mutable global state is used in parts of `std/` for low-level platform calls.
Because globals lower to storage, updates are visible across calls.

### Platform Directives

`@platform(...)` applies only to the next top-level declaration, import, external
declaration, or inline LLVM item. It does not apply to a whole file or to a
block of declarations.

Directive arguments must be literal strings, numbers, or booleans. The
implemented directive name is `platform`; other directive names are rejected.

### Low-Level and Runtime Caveats

Pointers, slices, `str`, and inline LLVM are low-level facilities. Bounds checks,
null checks, pointer lifetime/alias checks, and read-only memory protection are
not enforced. The destroy checker covers only direct destructible locals and is
warning-only. Incorrect pointer arithmetic, indexing, writes through invalid
pointers, or mutation of read-only string data can crash the generated program.

Function-pointer fields can be called, and function-pointer types lower to
backend function pointer signatures. Calls through function pointers still go
through the same basic argument-count checking as ordinary calls.

Inline LLVM text is injected into the generated IR stream. The Magma parser only
requires a string literal after `llvm`; it does not verify that the embedded IR
matches surrounding Magma types or remains intact after optimization.

## Namespaces and Member Access

Dotted names are used in three related ways:

```magma
module.member      # imported module access
value.field        # struct field access
value.method(...)  # method call
```

Examples:

```magma
errors.failure("failed")
this.underlying.write(toWrite)
this.payload.left
```

The link and type checker resolve whether a dotted expression refers to an
import, a field, a method, or a nested field.

## Formatting Conventions

The standard library generally follows these conventions:

```magma
mod module_name

use "path.mg" alias

StructName(
    field Type
)

pub functionName(arg Type) !ReturnType:
    if condition:
        ...
    ..
    ret value
..
```

Commas are common in struct field lists, especially for multi-line declarations:

```magma
Writer(
    impl ptr,
    fn_write (ptr, str) !u64,
)
```

Statements inside blocks are indented with spaces, but indentation is not the
source of block structure. The `:` and `..` tokens are authoritative.
