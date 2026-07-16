# `std/heap`

## Example

```magma
block := try heap.allocZero(32)
block = try heap.reallocZero(block, 64, 32)
heap.free(block)
```

Portable access to the process heap.

- `pub allocator() a.Allocator` returns an allocator adapter backed by this module.
- `pub alloc(nBytes u64) !$u8*` allocates uninitialized owned memory.
- `pub allocZero(nBytes u64) !$u8*` allocates zero-filled owned memory.
- `pub realloc(in u8*, nBytes u64) !$u8*` resizes a heap block; newly added bytes are uninitialized.
- `pub reallocZero(in u8*, nBytes u64, prevNbytes u64) !$u8*` resizes and zeros bytes added beyond `prevNbytes`.
- `pub free(in u8*) void` releases a heap block.

Allocation size must be greater than zero. Do not mix this module's blocks with another allocator.
