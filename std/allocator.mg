mod allocator
# Allocator interfaces for allocating, resizing, and releasing owned memory.
# Allocations must be released through the same allocator that created them.

use "std:errors" as errors
use "std:checked" as checked
use "std:cast" as cast

# Generic allocator interface backed by a compiler-generated immutable vtable.
pub proto Allocator(
    alloc(byteCount u64) !$u8*
    realloc(block ptr, byteCount u64) !$u8*
    free(block ptr) void
)

# Allocates a new block of size count * sizeof T.
# @complexity O(1) wrapper call; allocator-dependent.
# @param count number of T elements to allocate
# @returns owned memory block
# @throws outOfMemory when the allocator cannot satisfy the request
# @ownership Release the block with the same allocator.
# @example
#   values := try a.allocT[u64](16)
#   a.free(values)
Allocator.allocT[T](count u64) !$T*:
    ret cast.reinterpret[T](try this.alloc(try checked.byteCount[T](count)))
..

# Reallocates a block of size count * sizeof T.
# @complexity O(1) wrapper call; allocator-dependent.
# @param block existing allocation
# @param count new number of T elements
# @returns owned memory block
# @throws outOfMemory when the block cannot be resized
# @ownership The returned pointer replaces block and remains owned by the caller.
Allocator.reallocT[T](block T*, count u64) !$T*:
    ret cast.reinterpret[T](try this.realloc(block, try checked.byteCount[T](count)))
..

# Internal bridge used by intrinsic values that embed an Allocator protocol.
# The pointer must address two fields laid out as { implementation, vtable }.
pub fromEmbedded(p ptr) Allocator:
    unsafe:
        value Allocator* = cast.reinterpret[Allocator](p)
        ret *value
    ..
..

# Reports whether this protocol has no dispatch table. Such a value cannot
# allocate or free and is used as the non-owning marker in intrinsic strings.
pub noctx Allocator.isNull() bool:
    ret this.vtable == none
..

# Raw protocol fields used when embedding an allocator in an intrinsic value.
pub noctx Allocator.implementation() ptr:
    ret this.impl
..

pub noctx Allocator.dispatchTable() ptr:
    ret this.vtable
..

# Stable placeholder used by containers whose optional backing allocator is
# inactive (for example arenas over caller-owned buffers).
NullAllocator impl Allocator(
    value u8
)

NullAllocator.alloc(byteCount u64) !$u8*:
    throw errors.invalidArgument("null allocator cannot allocate")
    ret none
..

NullAllocator.realloc(block ptr, byteCount u64) !$u8*:
    throw errors.invalidArgument("null allocator cannot reallocate")
    ret none
..

NullAllocator.free(block ptr) void:
..

gl_nullAllocator := NullAllocator(value=0)

pub noctx null() Allocator:
    ret gl_nullAllocator.proto()
..
