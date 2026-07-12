# `std/allocator`

Defines the allocator interface used by allocating standard-library APIs.

## Type

### `Allocator`

```magma
Allocator(
    impl ptr,
    fn_alloc (ptr, u64) !u8*,
    fn_realloc (ptr, u8*, u64) !u8*,
    fn_free (ptr, u8*) void,
)
```

`impl` is adapter-specific state. The three function pointers allocate, resize, and free memory. Blocks must be released or resized with the same allocator that created them.

## Methods

- `Allocator.alloc(byteCount u64) !$u8*` allocates `byteCount` uninitialized bytes.
- `Allocator.allocT[T](count u64) !$T*` allocates space for `count` values of `T`; it fails if the byte-size calculation overflows.
- `Allocator.realloc(block u8*, byteCount u64) !$u8*` resizes an allocation.
- `Allocator.reallocT[T](block T*, count u64) !$T*` resizes a typed allocation; it fails if the byte-size calculation overflows.
- `Allocator.free(block u8*) void` releases a block made by this allocator.

`$` marks returned allocations as owned. Allocation methods can fail with the error supplied by the underlying adapter.
