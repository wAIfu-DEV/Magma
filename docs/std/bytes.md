# `std/bytes`

Utilities for byte slices. None allocate.

- `pub equal(a u8[], b u8[]) bool` reports equal length and byte-for-byte content.
- `pub indexByte(in u8[], value u8) !u64` returns the first matching index, or an error if absent.
- `pub contains(in u8[], value u8) bool` reports whether a value occurs.
- `pub startsWith(in u8[], prefix u8[]) bool` reports whether `prefix` is at the beginning.
- `pub endsWith(in u8[], suffix u8[]) bool` reports whether `suffix` is at the end.
- `pub reverse(in u8[]) void` reverses the slice in place.
