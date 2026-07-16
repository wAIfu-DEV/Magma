mod main
use "../allocator.mg" allocator
use "../errors.mg" errors
use "../heap.mg" heap
use "../strconv.mg" strconv
use "../strings.mg" strings
pub main() !void:
    a allocator.Allocator = heap.allocator()
    number := try strconv.parseUint("42")
    boolean := try strconv.parseBool("true")
    if number != 42 || boolean == false:
        throw errors.failure("strconv parse changed")
    ..
    formatted := try strconv.formatUint(a, 42)
    defer strings.free(a, formatted)
    if strings.compare(formatted, "42") == false:
        throw errors.failure("strconv format changed")
    ..
    formattedPtr u8* = strings.toPtr(formatted)
    if formattedPtr[strings.countBytes(formatted)] != 0:
        throw errors.failure("formatted string is not null terminated")
    ..
..
