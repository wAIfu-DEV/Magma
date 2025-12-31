mod allocator

Allocator(
    impl ptr # opaque pointer, syntax not fixed yet
    
    fn_alloc   (ptr, u64) !u8*,
    fn_realloc (ptr, u8*, u64) !u8*,
    fn_free    (ptr, u8*) !void,
)

Allocator.alloc(n u64) !u8*:
    ret try this.fn_alloc(this.impl, n)
..

Allocator.realloc(p u8*, n u64) !u8*:
    ret try this.fn_realloc(this.impl, p, n)
..

Allocator.free(p u8*) !void:
    try this.fn_free(this.impl, p)
..
