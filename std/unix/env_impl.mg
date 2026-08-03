mod env_impl_unix
llvm "@environ = external global ptr\n"
use "std:allocator" allocator
use "std:heap" heap
use "std:strings" strings
use "std:errors" errors
use "std:c" c
use "std:array" array

ext ext_getenv getenv(name u8*) u8*
ext ext_setenv setenv(name u8*, value u8*, overwrite c.int) c.int
ext ext_unsetenv unsetenv(name u8*) c.int

pub Variables(
    allocator allocator.Allocator
    entries array.Array[str]
)
Variables.view() str[]: ret this.entries.view() ..
destr Variables.free() void:
    values := this.entries.view()
    i u64 = 0
    while i < this.entries.count():
        values[i].free(this.allocator)
        i = i + 1
    ..
    this.entries.free(this.allocator, none)
..

environmentPointer() u8**:
    llvm "  %environment = load ptr, ptr @environ\n"
    llvm "  ret ptr %environment\n"
..

pub get(a allocator.Allocator, name str) !$str:
    native := try strings.toCstr(heap.allocator(), name)
    defer heap.allocator().free(native)
    value := ext_getenv(native)
    if value == none:
        throw errors.notFound("environment variable was not found")
    ..
    ret try strings.fromCstr(a, value)
..

pub has(name str) bool:
    native u8*, e error = strings.toCstr(heap.allocator(), name)
    if e.nok(): ret false ..
    defer heap.allocator().free(native)
    ret ext_getenv(native) != none
..

pub set(name str, value str) !void:
    n := try strings.toCstr(heap.allocator(), name)
    defer heap.allocator().free(n)
    v := try strings.toCstr(heap.allocator(), value)
    defer heap.allocator().free(v)
    if ext_setenv(n, v, 1) != 0:
        throw errors.failure("setenv failed")
    ..
..

pub unset(name str) !void:
    n := try strings.toCstr(heap.allocator(), name)
    defer heap.allocator().free(n)
    if ext_unsetenv(n) != 0:
        throw errors.failure("unsetenv failed")
    ..
..

pub list(a allocator.Allocator) !$Variables:
    entries := try array.new[str](a)
    onerror:
        existing := entries.view()
        j u64 = 0
        while j < entries.count():
            existing[j].free(a)
            j = j + 1
        ..
        entries.free(a, none)
    ..
    environment := environmentPointer()
    i u64 = 0
    while environment[i] != none:
        value str = try strings.fromCstr(a, environment[i])
        try entries.pushRight(a, value)
        i = i + 1
    ..
    ret Variables(allocator=a, entries=entries)
..
