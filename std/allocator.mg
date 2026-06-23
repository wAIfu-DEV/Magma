mod allocator

# Generic allocator interface with function pointers.
# O(1) for wrapper calls; underlying allocator decides cost.
Allocator(
    impl ptr,
    
    fn_alloc   (ptr, u64) !u8*,
    fn_realloc (ptr, u8*, u64) !u8*,
    fn_free    (ptr, u8*) void,
)

# Allocates a new block of byteCount bytes.
# O(1) wrapper call; allocator-dependent.
# @param byteCount number of bytes to allocate
# @returns owned memory block
Allocator.alloc(byteCount u64) !$u8*:
    ret try this.fn_alloc(this.impl, byteCount)
..

# Reallocates a block to byteCount bytes.
# O(1) wrapper call; allocator-dependent.
# @param block existing allocation
# @param byteCount new size in bytes
# @returns owned memory block
Allocator.realloc(block u8*, byteCount u64) !$u8*:
    ret try this.fn_realloc(this.impl, block, byteCount)
..

# Frees a previously allocated block.
# O(1) wrapper call; allocator-dependent.
# @param block allocation to free
Allocator.free(block u8*) void:
    this.fn_free(this.impl, block)
..
