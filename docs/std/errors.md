# `std/errors`

## Example

```magma
problem := errors.invalidArgument("expected a positive count")
if errors.hasCode(problem, 2):
    message := problem.message()
..
```

Creates and inspects Magma `error` values. Standard-library errors use category codes; native platform errors set a distinguishing high bit.

## Inspection

- `pub trace(e error) Trace` returns a cursor over propagation frames.
- `Trace.isEmpty`, `isTruncated`, `next`, `function`, `file`, `line`, and
  `column` inspect that cursor; `printTrace(e)` prints it.
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
- `pub outOfBounds(message str) error` creates code 7.
- `pub notFound(message str) error` creates code 8.
- `pub cancelled(message str) error` creates code 9.
- `makeErr(code u32, msg str) error` is the internal common constructor.

The corresponding public constants are `ERR_OK`, `ERR_FAIL`, `ERR_INVALID_ARG`,
`ERR_OUT_OF_MEMORY`, `ERR_END_OF_FILE`, `ERR_WOULD_OVERFLOW`, `ERR_INVALID_TYPE`,
`ERR_OUT_OF_BOUNDS`, `ERR_NOT_FOUND`, and `ERR_CANCELLED`.
