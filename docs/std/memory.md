# `std/memory`

Low-level operations on raw byte regions. Callers must provide valid pointers and sizes.

- `pub copy(from ptr, to ptr, n u64) void` copies `n` bytes between non-overlapping regions. Use `move` if overlap is possible.
- `pub move(from ptr, to ptr, n u64) void` safely copies possibly overlapping regions.
- `pub swap(x ptr, y ptr, n u64) void` swaps non-overlapping regions in place.
- `pub compare(a ptr, b ptr, n u64) bool` reports byte-for-byte equality.
- `pub set(in ptr, n u64, with u8) void` fills a region with one byte value.
- `pub zero(in ptr, n u64) void` fills a region with zero.
