mod allocator

Allocator(
    impl ptr, # opaque pointer, syntax not fixed yet
    
    fn_alloc   (ptr, u64) !u8*,
    fn_realloc (ptr, u8*, u64) !u8*,
    fn_free    (ptr, u8*) void,
)

Allocator.alloc(byteCount u64) !u8*:
    ret try this.fn_alloc(this.impl, byteCount)
..

Allocator.realloc(block u8*, byteCount u64) !u8*:
    ret try this.fn_realloc(this.impl, block, byteCount)
..

# cleanup functions should not throw in order to be useable in defer statements

Allocator.free(block u8*) void:
    this.fn_free(this.impl, block)
..
