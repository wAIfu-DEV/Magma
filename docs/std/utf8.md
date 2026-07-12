# `std/utf8`

Validated UTF-8 iteration plus UTF-8/UTF-16 conversion.

## Types

- `Utf8Iterator(start ptr, end ptr)` is a borrowed cursor over a string. The source string must remain valid.
- `Codepoint(value u32, width u8)` contains a Unicode scalar value and its encoded UTF-8 width.

## Iteration

- `pub iterator(s str) Utf8Iterator` creates a cursor.
- `Utf8Iterator.hasData() bool` reports whether bytes remain.
- `Utf8Iterator.peek() !Codepoint` validates and returns the next codepoint without advancing.
- `Utf8Iterator.next() !Codepoint` validates, returns, and advances past the next codepoint.
- `pub countCodepoints(s str) !u64` counts validated codepoints.

Malformed, truncated, overlong, surrogate, or out-of-range UTF-8 produces an error.

## Conversion

- `pub utf8To16(a alc.Allocator, s str) !$u16[]` returns owned UTF-16 code units without a terminator.
- `pub utf8To16NT(a alc.Allocator, s str) !$u16[]` returns an owned UTF-16 slice with a trailing zero included.
- `pub utf16to8size(in u16[]) !u64` returns the required UTF-8 byte count after validating surrogate pairs.
- `pub utf16to8(a alc.Allocator, in u16[]) !$str` returns an owned UTF-8 string.

Free owned results with the same allocator. Internal conversion helpers are `u8to32`, `u32to8`, `u16to32`, `u32to16`, `decodeOnce`, `decodeFirst`, `utf8to16size`, `encodeUtf8`, `codepointUtf8Size`, and `utf16to8iter`.
