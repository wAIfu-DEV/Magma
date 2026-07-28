# `std/slices`

## Example

```magma
a := heap.allocator()
block := try a.alloc(4)
bytes := slices.reinterpret[u8, u8](slices.fromPtr(block, 4))
count := slices.count(bytes) # 4
slices.free(a, bytes)
```

Low-level generic slice representation and ownership helpers.

- `pub count(s slice) u64` returns a slice's element count.
- `pub fromPtr(p ptr, elemCount u64) slice` creates a borrowed slice descriptor over existing memory; it does not allocate or validate the region.
- `pub toPtr(s slice) ptr` returns the underlying data pointer.
- `pub reinterpret[T, R](in T[]) R[]` returns a view over the same bytes with
  element type `R`; its count is `sourceBytes / sizeof R`, rounded down. It does
  not validate alignment or representation.
- `pub alloc[T](a alc.Allocator, elemCount u64) !$T[]` allocates an owned typed
  slice with checked byte-size multiplication.
- `pub free(a alc.Allocator, s slice) void` releases an owned slice allocation. Use only for a `$T[]` created with the same allocator; it does not recursively free elements.
