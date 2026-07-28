# `std/core`

This module is imported implicitly and defines methods on intrinsic types.
Programs normally do not import it directly.

- `slice.count()` returns a slice's element count.
- `error.ok()`, `nok()`, `code()`, and `message()` inspect an error.
- `str.countBytes()` returns UTF-8 byte length, not code-point count.
- `str.free(a)` releases an owned string with its original allocator. Passing a
  literal, borrowed string, or the wrong allocator is invalid.
