# `std/strings`

## Example

```magma
a := heap.allocator()
owned := try strings.copy(a, "magma")
defer owned.free(a)
bytes := owned.countBytes() # 5
same := strings.compare(owned, "magma")
```

Byte-level string, pointer, and C-string utilities. Magma `str` values are byte ranges and are not necessarily null-terminated.

## String access and ownership

- `pub alloc(a alc.Allocator, size u64) !$str` allocates an uninitialized owned
  string with a trailing null byte; `allocFill(a, size, fill)` initializes every
  logical byte.
- `pub countBytes(s str) u64` returns byte length in O(1), not Unicode codepoint count.
- `pub toPtr(s str) u8*` returns a borrowed pointer to string data.
- `pub byteAt(s str, idx u64) u8` returns one byte; it is not UTF-8-aware and requires a valid index.
- `pub copy(a alc.Allocator, s str) !$str` returns an owned copy.
- `pub toLower(a, s) !$str` and `toUpper(a, s) !$str` return owned ASCII-case
  conversions; non-ASCII bytes are unchanged.
- Owned strings are released with the intrinsic `s.free(a)` destructor.
- `pub compare(a str, b str) bool` tests byte-for-byte equality.
- `findByte(s, value) !u64` and `find(s, needle) !u64` return the first byte
  index or `outOfBounds`.
- `substring(a, s, start, end) !$str` copies the half-open byte range.
- `trim`, `trimPrefix`, and `trimSuffix` return allocated copies.

## Raw pointers

- `pub fromPtrNoCopy(p ptr, bytesCount u64) str` creates a borrowed string view. The pointed memory must remain valid.
- `pub fromPtr(a alc.Allocator, p ptr, byteCount u64) !$str` copies raw bytes into an owned string.

## C strings

- `pub toCstr(a alc.Allocator, s str) !$u8*` returns an owned null-terminated copy.
- `pub toCstrNoCopy(s str) u8*` returns the underlying pointer without checking
  or copying. The caller must guarantee a readable null byte immediately after
  the logical string; allocating string APIs satisfy this, arbitrary borrowed
  views may not.
- `pub cStrLen(cstr u8*) u64` scans to the null terminator.
- `pub fromCstrNoCopy(cstr u8*) str` returns a borrowed view after scanning its length.
- `pub fromCstr(a alc.Allocator, cstr u8*) !$str` returns an owned copy.

No function validates UTF-8. Use `std/utf8` for Unicode-aware traversal and conversion.

## Splitting

- `split(a, s, separator) !$Split` eagerly allocates a table and every part.
  `Split.count`, `get`, and `free` inspect and release it.
- `splitIter(a, s, separator) !$SplitIterator` copies the source and separator;
  `hasData` polls it, `next() !$str` returns each independently owned part, and
  `free` releases iterator state.
- `splitOnce(a, s, separator) !$pair.Pair[str, str]` allocates the two halves
  around the first match.

An empty separator is invalid. Free every owned result with the supplied allocator.
