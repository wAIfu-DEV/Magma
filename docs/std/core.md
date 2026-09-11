# `std/core`

This module is imported implicitly and defines methods on intrinsic types.
Programs normally do not import it directly.

- `slice.count()` returns a slice's element count.
- `error.ok()`, `nok()`, `code()`, and `message()` inspect an error.
- `str.countBytes()` returns UTF-8 byte length, not code-point count.
- `str.free()` releases an owned string through its embedded allocator.
  Literals, borrowed strings, and the zero string have no allocator and are
  harmless no-ops.
