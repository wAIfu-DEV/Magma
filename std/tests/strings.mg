mod main
use "../allocator.mg" allocator
use "../errors.mg" errors
use "../heap.mg" heap
use "../strings.mg" strings
pub main() !void:
    a allocator.Allocator = heap.allocator()
    copy := try strings.copy(a, "magma")
    defer strings.free(a, copy)
    if strings.countBytes(copy) != 5 || strings.compare(copy, "magma") == false:
        throw errors.failure("strings behavior changed")
    ..
    copyPtr u8* = strings.toPtr(copy)
    if copyPtr[strings.countBytes(copy)] != 0:
        throw errors.failure("copied string is not null terminated")
    ..
    empty := try strings.alloc(a, 0)
    defer strings.free(a, empty)
    emptyPtr u8* = strings.toPtr(empty)
    if *emptyPtr != 0:
        throw errors.failure("empty allocated string is not null terminated")
    ..
    filled := try strings.allocFill(a, 3, 65)
    defer strings.free(a, filled)
    filledPtr u8* = strings.toPtr(filled)
    if filledPtr[3] != 0:
        throw errors.failure("filled string is not null terminated")
    ..
    cstr := try strings.toCstr(a, "magma")
    defer a.free(cstr)
    if cstr[5] != 0:
        throw errors.failure("C string is not null terminated")
    ..
..
