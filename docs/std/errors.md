# `std/errors`

Creates and inspects Magma `error` values. Standard-library errors use category codes; native platform errors set a distinguishing high bit.

## Inspection

- `pub code(e error) u32` returns the stored code; zero means success.
- `pub message(e error) str` returns the stored message.
- `pub is(a error, b error) bool` compares error categories, not messages.
- `pub hasCode(e error, expected u32) bool` tests a numeric category.
- `pub toStr(e error) str` returns the category name.
- `pub isNative(e error) bool` reports whether the native-code marker is set.
- `pub nativeCode(e error) u32` returns the platform code without the marker.

## Construction

- `pub native(code u32, message str) error` wraps a platform error code.
- `pub ok() error` creates code 0.
- `pub failure(message str) error` creates code 1 (opaque failure).
- `pub invalidArgument(message str) error` creates code 2.
- `pub outOfMemory(message str) error` creates code 3.
- `pub endOfFile(message str) error` creates code 4.
- `pub wouldOverflow(message str) error` creates code 5.
- `pub invalidType(message str) error` creates code 6.
- `makeErr(code u32, msg str) error` is the internal common constructor.
