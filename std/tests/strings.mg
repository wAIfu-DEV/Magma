mod main
use "std:allocator" allocator
use "std:errors" errors
use "std:heap" heap
use "std:strings" strings
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

    if try strings.findByte("magma", 103) != 2 || try strings.find("one two", "two") != 4:
        throw errors.failure("string find changed")
    ..
    sub := try strings.substring(a, "magma", 1, 4)
    defer strings.free(a, sub)
    if strings.compare(sub, "agm") == false:
        throw errors.failure("substring changed")
    ..
    trimmed := try strings.trim(a, " \t magma \r\n")
    defer strings.free(a, trimmed)
    withoutPrefix := try strings.trimPrefix(a, "std:strings", "std:")
    defer strings.free(a, withoutPrefix)
    withoutSuffix := try strings.trimSuffix(a, "file.mg", ".mg")
    defer strings.free(a, withoutSuffix)
    if strings.compare(trimmed, "magma") == false || strings.compare(withoutPrefix, "strings") == false || strings.compare(withoutSuffix, "file") == false:
        throw errors.failure("string trimming changed")
    ..

    parts := try strings.split(a, "one::two::", "::")
    defer parts.free()
    if parts.count() != 3 || strings.compare(try parts.get(0), "one") == false || strings.compare(try parts.get(1), "two") == false || strings.compare(try parts.get(2), "") == false:
        throw errors.failure("eager split changed")
    ..

    splitPair := try strings.splitOnce(a, "left=right", "=")
    defer strings.free(a, splitPair.first)
    defer strings.free(a, splitPair.second)
    if strings.compare(splitPair.first, "left") == false || strings.compare(splitPair.second, "right") == false:
        throw errors.failure("splitOnce changed")
    ..

    splitIterator := try strings.splitIter(a, "a,b,c", ",")
    defer splitIterator.free()
    iterFirst := try splitIterator.next()
    defer strings.free(a, iterFirst)
    iterSecond := try splitIterator.next()
    defer strings.free(a, iterSecond)
    iterThird := try splitIterator.next()
    defer strings.free(a, iterThird)
    if splitIterator.hasData() || strings.compare(iterFirst, "a") == false || strings.compare(iterSecond, "b") == false || strings.compare(iterThird, "c") == false:
        throw errors.failure("split iterator changed")
    ..
..
