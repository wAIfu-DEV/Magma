mod env_impl_unix
# SAFETY: declares the platform-owned process environment global; no access occurs here.
llvm "@environ = external global ptr\n"
use "std:allocator" allocator
use "std:heap" heap
use "std:strings" strings
use "std:errors" errors
use "std:c" c
use "std:list" list

ext ext_getenv getenv(name u8*) u8*
ext ext_setenv setenv(name u8*, value u8*, overwrite c.int) c.int
ext ext_unsetenv unsetenv(name u8*) c.int

freeString(a allocator.Allocator, value $str) void:
    value.free(a)
..

environmentPointer() u8**:
    # SAFETY: this audited implementation injects the required low-level IR.
    unsafe:
        llvm "  %environment = load ptr, ptr @environ\n"
        llvm "  ret ptr %environment\n"
    ..
..

environmentAt(environment u8**, index u64) u8*:
    # SAFETY: environ is a platform-owned, null-terminated pointer array.
    unsafe:
        ret environment[index]
    ..
..

pub get(name str) !$str:
    a := ctx.procAlloc
    native := try strings.toCstr(name)
    defer heap.allocator().free(native)
    value := ext_getenv(native)
    if value == none:
        throw errors.notFound("environment variable was not found")
    ..
    ret try strings.fromCstr(value)
..

pub has(name str) bool:
    native u8*, e error = strings.toCstr(name)
    if e.nok(): ret false ..
    defer heap.allocator().free(native)
    ret ext_getenv(native) != none
..

pub set(name str, value str) !void:
    n := try strings.toCstr(name)
    defer heap.allocator().free(n)
    v := try strings.toCstr(value)
    defer heap.allocator().free(v)
    if ext_setenv(n, v, 1) != 0:
        throw errors.failure("setenv failed")
    ..
..

pub unset(name str) !void:
    n := try strings.toCstr(name)
    defer heap.allocator().free(n)
    if ext_unsetenv(n) != 0:
        throw errors.failure("unsetenv failed")
    ..
..

pub list() !$list.List[str]:
    a := ctx.procAlloc
    entries := try list.new[str](a, freeString)
    onerror entries.free()
    environment := environmentPointer()
    i u64 = 0
    loop environmentAt(environment, i) != none:
        value str = try strings.fromCstr(environmentAt(environment, i))
        try entries.pushRight(move value)
        i = i + 1
    ..
    ret move entries
..
