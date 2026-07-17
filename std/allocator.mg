mod allocator

use "errors.mg" errors

# Function table shared by allocator handles whose implementation and vtable
# lifetimes are managed externally.
Vtable(
    fn_alloc   (ptr, u64) !u8*
    fn_realloc (ptr, u8*, u64) !u8*
    fn_free    (ptr, u8*) void
)

# Generic allocator interface backed by a shared, immutable vtable.
Allocator(
    impl ptr
    vtable Vtable*
)
# Byte size: 16B

# Allocates a new block of byteCount bytes.
# O(1) wrapper call; allocator-dependent.
# @param byteCount number of bytes to allocate
# @returns owned memory block
Allocator.alloc(byteCount u64) !$u8*:
    ret try this.vtable.fn_alloc(this.impl, byteCount)
..

# Allocates a new block of size count * sizeof T.
# O(1) wrapper call; allocator-dependent.
# @param count number of T elements to allocate
# @returns owned memory block
Allocator.allocT[T](count u64) !$T*:
    ret try this.vtable.fn_alloc(this.impl, count * sizeof T)
..

# Reallocates a block of byteCount bytes.
# O(1) wrapper call; allocator-dependent.
# @param block existing allocation
# @param byteCount new size in bytes
# @returns owned memory block
Allocator.realloc(block u8*, byteCount u64) !$u8*:
    ret try this.vtable.fn_realloc(this.impl, block, byteCount)
..

# Reallocates a block of size count * sizeof T.
# O(1) wrapper call; allocator-dependent.
# @param block existing allocation
# @param count new number of T elements
# @returns owned memory block
Allocator.reallocT[T](block T*, count u64) !$T*:
    ret try this.vtable.fn_realloc(this.impl, block, count * sizeof T)
..

# Frees a previously allocated block.
# O(1) wrapper call; allocator-dependent.
# @param block allocation to free
Allocator.free(block u8*) void:
    this.vtable.fn_free(this.impl, block)
..
