# `std/strings`

## Example

```magma
a := heap.allocator()
owned := try strings.copy(a, "magma")
defer strings.free(a, owned)
bytes := strings.countBytes(owned) # 5
same := strings.compare(owned, "magma")
```

Byte-level string, pointer, and C-string utilities. Magma `str` values are byte ranges and are not necessarily null-terminated.

## String access and ownership

- `pub countBytes(s str) u64` returns byte length in O(1), not Unicode codepoint count.
- `pub toPtr(s str) u8*` returns a borrowed pointer to string data.
- `pub byteAt(s str, idx u64) u8` returns one byte; it is not UTF-8-aware and requires a valid index.
- `pub copy(a alc.Allocator, s str) !$str` returns an owned copy.
- `pub free(a alc.Allocator, s str) void` releases an owned string created with that allocator.
- `pub compare(a str, b str) bool` tests byte-for-byte equality.

## Raw pointers

- `pub fromPtrNoCopy(p ptr, bytesCount u64) str` creates a borrowed string view. The pointed memory must remain valid.
- `pub fromPtr(a alc.Allocator, p ptr, byteCount u64) !$str` copies raw bytes into an owned string.

## C strings

- `pub toCstr(a alc.Allocator, s str) !$u8*` returns an owned null-terminated copy.
- `pub cStrLen(cstr u8*) u64` scans to the null terminator.
- `pub fromCstrNoCopy(cstr u8*) str` returns a borrowed view after scanning its length.
- `pub fromCstr(a alc.Allocator, cstr u8*) !$str` returns an owned copy.

No function validates UTF-8. Use `std/utf8` for Unicode-aware traversal and conversion.
