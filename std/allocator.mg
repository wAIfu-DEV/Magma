mod allocator
# Allocator interfaces for allocating, resizing, and releasing owned memory.
# Allocations must be released through the same allocator that created them.

use "std:errors" errors

# Function table shared by allocator handles whose implementation and vtable
# lifetimes are managed externally.
pub Vtable(
    fn_alloc   (ptr, u64) !u8*
    fn_realloc (ptr, u8*, u64) !u8*
    fn_free    (ptr, u8*) void
)

# Generic allocator interface backed by a shared, immutable vtable.
pub Allocator(
    impl ptr
    vtable Vtable*
)
# Byte size: 16B

# Allocates a new block of byteCount bytes.
# @complexity O(1) wrapper call; allocator-dependent.
# @param byteCount number of bytes to allocate
# @returns owned memory block
# @throws outOfMemory when the allocator cannot satisfy the request
# @ownership Release the block with the same allocator or transfer ownership.
# @example
#   block := try a.alloc(64)
#   a.free(block)
Allocator.alloc(byteCount u64) !$u8*:
    ret try this.vtable.fn_alloc(this.impl, byteCount)
..

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
    ret try this.vtable.fn_alloc(this.impl, count * sizeof T)
..

# Reallocates a block of byteCount bytes.
# @complexity O(1) wrapper call; allocator-dependent.
# @param block existing allocation
# @param byteCount new size in bytes
# @returns owned memory block
# @throws outOfMemory when the block cannot be resized
# @ownership The returned pointer replaces block and remains owned by the caller.
# @example
#   block = try a.realloc(block, 128)
Allocator.realloc(block u8*, byteCount u64) !$u8*:
    ret try this.vtable.fn_realloc(this.impl, block, byteCount)
..

# Reallocates a block of size count * sizeof T.
# @complexity O(1) wrapper call; allocator-dependent.
# @param block existing allocation
# @param count new number of T elements
# @returns owned memory block
# @throws outOfMemory when the block cannot be resized
# @ownership The returned pointer replaces block and remains owned by the caller.
Allocator.reallocT[T](block T*, count u64) !$T*:
    ret try this.vtable.fn_realloc(this.impl, block, count * sizeof T)
..

# Frees a previously allocated block.
# @complexity O(1) wrapper call; allocator-dependent.
# @param block allocation to free
# @warning block must have been allocated by this allocator and not already freed.
# @example
#   a.free(block)
Allocator.free(block u8*) void:
    this.vtable.fn_free(this.impl, block)
..
