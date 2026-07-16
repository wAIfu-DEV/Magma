# `std/allocator`

## Example

```magma
a := heap.allocator()
block := try a.alloc(16)
block = try a.realloc(block, 32)
a.free(block)
```

Defines the allocator interface used by allocating standard-library APIs.

## Type

### `Allocator`

```magma
AllocatorVTable(
    fn_alloc (ptr, u64) !u8*,
    fn_realloc (ptr, u8*, u64) !u8*,
    fn_free (ptr, u8*) void,
)

Allocator(
    impl ptr,
    vtable AllocatorVTable*,
)
```

`impl` is adapter-specific state. `vtable` points to a shared immutable table
whose three function pointers allocate, resize, and free memory. The allocator
handle is 16 bytes on 64-bit targets. Blocks must be released or resized with
the same allocator that created them.

## Methods

- `Allocator.alloc(byteCount u64) !$u8*` allocates `byteCount` uninitialized bytes.
- `Allocator.allocT[T](count u64) !$T*` allocates space for `count` values of `T`; it fails if the byte-size calculation overflows.
- `Allocator.realloc(block u8*, byteCount u64) !$u8*` resizes an allocation.
- `Allocator.reallocT[T](block T*, count u64) !$T*` resizes a typed allocation; it fails if the byte-size calculation overflows.
- `Allocator.free(block u8*) void` releases a block made by this allocator.

`$` marks returned allocations as owned. Allocation methods can fail with the error supplied by the underlying adapter.
