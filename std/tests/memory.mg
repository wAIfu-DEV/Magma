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
    memory.set(slices.toPtr(target), 4, 9)
    if target[0] != 9 || target[3] != 9:
        throw errors.failure("memory set changed")
    ..
    memory.swap(slices.toPtr(source), slices.toPtr(target), 4)
    if source[0] != 9 || target[0] != 1:
        throw errors.failure("memory swap changed")
    ..
    overlap u8[4]
    overlap[0] = 1
    overlap[1] = 2
    overlap[2] = 3
    memory.move(slices.toPtr(overlap), addrof overlap[1], 3)
    if overlap[1] != 1 || overlap[3] != 3:
        throw errors.failure("memory move changed")
    ..
    zero u64 = memory.zeroValue[u64]()
    if zero != 0:
        throw errors.failure("memory zeroValue changed")
    ..
..
