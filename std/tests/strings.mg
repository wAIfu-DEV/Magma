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
    if strings.byteAt(filled, 0) != 65 || filledPtr[3] != 0:
        throw errors.failure("filled string is not null terminated")
    ..
    cstr := try strings.toCstr(a, "magma")
    defer a.free(cstr)
    if cstr[5] != 0:
        throw errors.failure("C string is not null terminated")
    ..

    noCopy := strings.toCstrNoCopy(copy)
    if noCopy != copyPtr || strings.cStrLen(noCopy) != 5:
        throw errors.failure("toCstrNoCopy rejected a terminated owned string")
    ..

    unterminated u8* = try a.alloc(5)
    defer a.free(unterminated)
    i u64 = 0
    while i < 5:
        unterminated[i] = 65
        i = i + 1
    ..
    borrowed str = strings.fromPtrNoCopy(unterminated, 5)
    borrowedPtr := strings.toCstrNoCopy(borrowed)
    if borrowedPtr != unterminated:
        throw errors.failure("toCstrNoCopy did not return the borrowed pointer")
    ..
    copiedFromPtr := try strings.fromPtr(a, unterminated, 5)
    defer strings.free(a, copiedFromPtr)
    if strings.countBytes(copiedFromPtr) != 5 || strings.toPtr(copiedFromPtr) == unterminated:
        throw errors.failure("fromPtr did not copy its input")
    ..
    borrowedCstr := strings.fromCstrNoCopy(cstr)
    ownedCstr := try strings.fromCstr(a, cstr)
    defer strings.free(a, ownedCstr)
    if strings.compare(borrowedCstr, "magma") == false || strings.compare(ownedCstr, "magma") == false:
        throw errors.failure("C string conversion changed")
    ..
..
