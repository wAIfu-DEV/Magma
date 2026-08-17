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
pub proto Allocator(
    alloc(byteCount u64) !$u8*
    realloc(block u8*, byteCount u64) !$u8*
    free(block u8*) void
)
```

Concrete allocators use `impl Allocator`; calling `proto()` on stable named
implementation storage creates the borrowed two-word interface view. Blocks
must be released or resized with the same allocator that created them.

The compiler associates allocations with the concrete implementation owner,
not the interface variable. Moving the implementation preserves that identity;
copying or losing an interface value does not extend it. Storage from local
scratch, arena, or custom allocator implementations therefore cannot escape
their owners. The same rule applies when an allocator flows through implicit
`ctx.procAlloc` or `ctx.tempAlloc`.

## Methods

- `Allocator.alloc(byteCount u64) !$u8*` allocates `byteCount` uninitialized bytes.
- `Allocator.allocT[T](count u64) !$T*` allocates space for `count` values of `T`; it fails if the byte-size calculation overflows.
- `Allocator.realloc(block u8*, byteCount u64) !$u8*` resizes an allocation.
- `Allocator.reallocT[T](block T*, count u64) !$T*` resizes a typed allocation; it fails if the byte-size calculation overflows.
- `Allocator.free(block u8*) void` releases a block made by this allocator.
- `null() Allocator` returns a stable placeholder whose allocation and resize
  operations fail. Containers use it when caller-owned backing storage needs no
  allocator cleanup.

`$` marks returned allocations as owned. Allocation methods can fail with the error supplied by the underlying adapter.
