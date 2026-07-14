mod main
use "../errors.mg" errors
use "../memory.mg" memory
use "../slices.mg" slices
pub main() !void:
    source u8[4]
    target u8[4]
    source[0] = 1
    source[1] = 2
    memory.copy(slices.toPtr(source), slices.toPtr(target), 4)
    if memory.compare(slices.toPtr(source), slices.toPtr(target), 4) == false:
        throw errors.failure("memory copy changed")
    ..
    memory.zero(slices.toPtr(target), 4)
    if target[0] != 0 || target[1] != 0:
        throw errors.failure("memory zero changed")
    ..
..
