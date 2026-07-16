mod main
use "../allocator.mg" allocator
use "../errors.mg" errors
use "../heap.mg" heap
use "../slices.mg" slices
use "../strings.mg" strings
use "../utf8.mg" utf8
pub main() !void:
    a allocator.Allocator = heap.allocator()
    count := try utf8.countCodepoints("hé")
    if count != 2:
        throw errors.failure("utf8 count changed")
    ..
    wide := try utf8.utf8To16(a, "hé")
    defer slices.free(a, wide)
    if slices.count(wide) != 2:
        throw errors.failure("utf8 conversion changed")
    ..
    roundTrip := try utf8.utf16to8(a, wide)
    defer strings.free(a, roundTrip)
    roundTripPtr u8* = strings.toPtr(roundTrip)
    if roundTripPtr[strings.countBytes(roundTrip)] != 0:
        throw errors.failure("UTF-8 result is not null terminated")
    ..
..
