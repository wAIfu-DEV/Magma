# `std/slices`

## Example

```magma
a := heap.allocator()
block := try a.alloc(4)
bytes := slices.fromPtr(block, 4)
count := slices.count(bytes) # 4
slices.free(a, bytes)
```

Low-level generic slice representation and ownership helpers.

- `pub count(s slice) u64` returns a slice's element count.
- `pub fromPtr(p ptr, elemCount u64) slice` creates a borrowed slice descriptor over existing memory; it does not allocate or validate the region.
- `pub toPtr(s slice) ptr` returns the underlying data pointer.
- `pub reinterpret[T](s slice) T[]` returns the same slice descriptor with element type `T`; it does not convert, allocate, or validate alignment.
- `pub free(a alc.Allocator, s slice) void` releases an owned slice allocation. Use only for a `$T[]` created with the same allocator; it does not recursively free elements.
