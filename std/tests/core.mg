mod main
use "std:errors" errors
use "std:heap" heap
use "std:strings" strings

pub main() !void:
    a := heap.allocator()
    owned := try strings.copy(a, "core")
    if strings.countBytes(owned) != 4:
        owned.free(a)
        throw errors.failure("primitive string method behavior changed")
    ..
    owned.free(a)

    # Borrowed literals are ordinary `str` values and carry no destroy duty.
    borrowed str = "literal"
    if strings.countBytes(borrowed) != 7:
        throw errors.failure("borrowed string behavior changed")
    ..
..
