mod main
use "../errors.mg" errors
use "../heap.mg" heap
pub main() !void:
    a := heap.allocator()
    viaAllocator := try a.alloc(8)
    a.free(viaAllocator)
    raw := try heap.alloc(8)
    raw[0] = 7
    raw = try heap.realloc(raw, 16)
    if raw[0] != 7:
        heap.free(raw)
        throw errors.failure("heap realloc did not preserve bytes")
    ..
    heap.free(raw)
    block := try heap.allocZero(32)
    if block[0] != 0 || block[31] != 0:
        heap.free(block)
        throw errors.failure("heap allocZero did not clear bytes")
    ..
    block = try heap.reallocZero(block, 64, 32)
    if block[32] != 0 || block[63] != 0:
        heap.free(block)
        throw errors.failure("heap reallocZero did not clear growth")
    ..
    heap.free(block)
..
