# `std/strings`

## Example

```magma
a := ctx.alloc
owned := try strings.copy("magma")
defer owned.free()
bytes := owned.countBytes() # 5
same := strings.compare(owned, "magma")
```

Byte-level string, pointer, and C-string utilities. Magma `str` values are byte ranges and are not necessarily null-terminated.

## String access and ownership

- `pub alloc(size u64) !$str` allocates an uninitialized owned string through
  `ctx.alloc` with a trailing null byte; `allocFill(size, fill)` initializes every
  logical byte.
- `pub countBytes(s str) u64` returns byte length in O(1), not Unicode codepoint count.
- `pub toPtr(s str) u8*` returns a borrowed pointer to string data.
- `pub byteAt(s str, idx u64) u8` returns one byte; it is not UTF-8-aware and requires a valid index.
- `pub copy(s str) !$str` returns an owned copy.
- `pub truncate(value str*, byteCount u64) bool` shortens an existing string
  descriptor without changing its allocation; it returns false if the requested
  length exceeds the current length.
- `pub toLower(s) !$str` and `toUpper(s) !$str` return owned ASCII-case
  conversions; non-ASCII bytes are unchanged.
- Owned strings retain their originating allocator and are released with the
  intrinsic `s.free()` destructor.
- `pub compare(a str, b str) bool` tests byte-for-byte equality.
- `findByte(s, value) !u64` and `find(s, needle) !u64` return the first byte
  index or `outOfBounds`.
- `substring(s, start, end) !$str` copies the half-open byte range.
- `trim`, `trimPrefix`, and `trimSuffix` return allocated copies.

## Raw pointers

- `pub fromPtrNoCopy(p ptr, bytesCount u64) str` creates a borrowed string view. The pointed memory must remain valid.
- `pub fromPtr(p ptr, byteCount u64) !$str` copies raw bytes into an owned string.

## C strings

- `pub toCstr(s str) !$u8*` returns an owned null-terminated copy.
- `pub toCstrNoCopy(s str) u8*` returns the underlying pointer without checking
  or copying. The caller must guarantee a readable null byte immediately after
  the logical string; allocating string APIs satisfy this, arbitrary borrowed
  views may not.
- `pub cStrLen(cstr u8*) u64` scans to the null terminator.
- `pub fromCstrNoCopy(cstr u8*) str` returns a borrowed view after scanning its length.
- `pub fromCstr(cstr u8*) !$str` returns an owned copy.

No function validates UTF-8. Use `std/utf8` for Unicode-aware traversal and conversion.

## Splitting

- `split(s, separator) !$Split` eagerly allocates a table and every part.
  `Split.count`, `get`, and `free` inspect and release it.
- `splitIter(s, separator) !$SplitIterator` copies the source and separator;
  `hasData` polls it, `next() !$str` returns each independently owned part, and
  `free` releases iterator state.
- `splitOnce(s, separator) !$pair.Pair[str, str]` allocates the two halves
  around the first match.

An empty separator is invalid. Free every owned result with the `ctx.alloc`
value active when it was created.
