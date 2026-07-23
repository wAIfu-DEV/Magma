mod main
use "std:allocator" allocator
use "std:errors" errors
use "std:heap" heap
use "std:slices" slices
use "std:strings" strings
use "std:utf8" utf8
pub main() !void:
    a allocator.Allocator = heap.allocator()
    iterator := utf8.iterator("A")
    if iterator.hasData() == false:
        throw errors.failure("utf8 iterator lost input")
    ..
    codepoint := try iterator.peek()
    advanced := try iterator.next()
    if codepoint.value != 65 || advanced.value != 65 || iterator.hasData():
        throw errors.failure("utf8 iterator changed")
    ..
    count := try utf8.countCodepoints("hé")
    if count != 2:
        throw errors.failure("utf8 count changed")
    ..
    wide := try utf8.utf8To16(a, "hé")
    defer slices.free(a, wide)
    if slices.count(wide) != 2:
        throw errors.failure("utf8 conversion changed")
    ..
    wideNt := try utf8.utf8To16NT(a, "A")
    defer slices.free(a, wideNt)
    wideNtPtr u16* = slices.toPtr(wideNt)
    if slices.count(wideNt) != 1 || wideNtPtr[0] != 65 || wideNtPtr[1] != 0:
        throw errors.failure("null-terminated UTF-16 conversion changed")
    ..
    if try utf8.utf16to8size(wide) != strings.countBytes("hé"):
        throw errors.failure("UTF-16 size calculation changed")
    ..
    roundTrip := try utf8.utf16to8(a, wide)
    defer strings.free(a, roundTrip)
    roundTripPtr u8* = strings.toPtr(roundTrip)
    if roundTripPtr[strings.countBytes(roundTrip)] != 0:
        throw errors.failure("UTF-8 result is not null terminated")
    ..
..
