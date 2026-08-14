mod main
use "std:allocator" allocator
use "std:errors" errors
use "std:heap" heap
use "std:strings" strings
pub main() !void:
    a allocator.Allocator = heap.allocator()
    copy := try strings.copy(a, "magma")
    defer copy.free(a)
    if copy.countBytes() != 5 || strings.compare(copy, "magma") == false:
        throw errors.failure("strings behavior changed")
    ..
    copyPtr u8* = strings.toPtr(copy)
    # SAFETY: copy allocates a trailing terminator at index countBytes.
    unsafe:
        if copyPtr[copy.countBytes()] != 0:
            throw errors.failure("copied string is not null terminated")
        ..
    ..
    empty := try strings.alloc(a, 0)
    defer empty.free(a)
    emptyPtr u8* = strings.toPtr(empty)
    # SAFETY: strings.alloc always returns a terminated allocation.
    unsafe:
        if *emptyPtr != 0:
            throw errors.failure("empty allocated string is not null terminated")
        ..
    ..
    filled := try strings.allocFill(a, 3, 65)
    defer filled.free(a)
    filledPtr u8* = strings.toPtr(filled)
    # SAFETY: allocFill appends a terminator after the requested payload.
    unsafe:
        if strings.byteAt(filled, 0) != 65 || filledPtr[3] != 0:
            throw errors.failure("filled string is not null terminated")
        ..
    ..
    cstr := try strings.toCstr(a, "magma")
    defer a.free(cstr)
    # SAFETY: toCstr returns count+1 addressable bytes including the terminator.
    unsafe:
        if cstr[5] != 0:
            throw errors.failure("C string is not null terminated")
        ..
    ..

    noCopy := strings.toCstrNoCopy(copy)
    if noCopy != copyPtr || strings.cStrLen(noCopy) != 5:
        throw errors.failure("toCstrNoCopy rejected a terminated owned string")
    ..

    unterminated u8* = try a.alloc(5)
    defer a.free(unterminated)
    i u64 = 0
    # SAFETY: unterminated points to the five bytes allocated immediately above.
    unsafe:
        loop i < 5:
            unterminated[i] = 65
            i = i + 1
        ..
    ..
    borrowed str = strings.fromPtrNoCopy(unterminated, 5)
    borrowedPtr := strings.toCstrNoCopy(borrowed)
    if borrowedPtr != unterminated:
        throw errors.failure("toCstrNoCopy did not return the borrowed pointer")
    ..
    copiedFromPtr := try strings.fromPtr(a, unterminated, 5)
    defer copiedFromPtr.free(a)
    if copiedFromPtr.countBytes() != 5 || strings.toPtr(copiedFromPtr) == unterminated:
        throw errors.failure("fromPtr did not copy its input")
    ..
    borrowedCstr := strings.fromCstrNoCopy(cstr)
    ownedCstr := try strings.fromCstr(a, cstr)
    defer ownedCstr.free(a)
    if strings.compare(borrowedCstr, "magma") == false || strings.compare(ownedCstr, "magma") == false:
        throw errors.failure("C string conversion changed")
    ..

    if try strings.findByte("magma", 103) != 2 || try strings.find("one two", "two") != 4:
        throw errors.failure("string find changed")
    ..
    sub := try strings.substring(a, "magma", 1, 4)
    defer sub.free(a)
    if strings.compare(sub, "agm") == false:
        throw errors.failure("substring changed")
    ..
    trimmed := try strings.trim(a, " \t magma \r\n")
    defer trimmed.free(a)
    withoutPrefix := try strings.trimPrefix(a, "std:strings", "std:")
    defer withoutPrefix.free(a)
    withoutSuffix := try strings.trimSuffix(a, "file.mg", ".mg")
    defer withoutSuffix.free(a)
    if strings.compare(trimmed, "magma") == false || strings.compare(withoutPrefix, "strings") == false || strings.compare(withoutSuffix, "file") == false:
        throw errors.failure("string trimming changed")
    ..

    parts := try strings.split(a, "one::two::", "::")
    defer parts.free()
    if parts.count() != 3 || strings.compare(try parts.get(0), "one") == false || strings.compare(try parts.get(1), "two") == false || strings.compare(try parts.get(2), "") == false:
        throw errors.failure("eager split changed")
    ..

    splitPair := try strings.splitOnce(a, "left=right", "=")
    defer:
        # SAFETY: splitOnce returns two uniquely owned string fields.
        unsafe:
            splitPair.first.free(a)
            splitPair.second.free(a)
        ..
    ..
    if strings.compare(splitPair.first, "left") == false || strings.compare(splitPair.second, "right") == false:
        throw errors.failure("splitOnce changed")
    ..

    splitIterator := try strings.splitIter(a, "a,b,c", ",")
    defer splitIterator.free()
    iterFirst := try splitIterator.next()
    defer iterFirst.free(a)
    iterSecond := try splitIterator.next()
    defer iterSecond.free(a)
    iterThird := try splitIterator.next()
    defer iterThird.free(a)
    if splitIterator.hasData() || strings.compare(iterFirst, "a") == false || strings.compare(iterSecond, "b") == false || strings.compare(iterThird, "c") == false:
        throw errors.failure("split iterator changed")
    ..
..
