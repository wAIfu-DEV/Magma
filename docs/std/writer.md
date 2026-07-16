# `std/writer`

## Example

```magma
output := writer.new(state, writeCallback)
try output.writeLn("ready")
try output.writeInt64(-42)
try output.writeFloat64(3.5, 2) # writes 3.50
```

A type-erased byte-output interface with formatting helpers.

## Type

`Writer(impl ptr, fn_write (ptr, str) !u64)` pairs adapter state with a write callback. The state and callback must remain valid for the writer's lifetime.

## API

- `pub new(impl ptr, writeFunc (ptr, str) !u64) Writer` constructs an interface.
- `Writer.write(bytes str) !u64` invokes the adapter once and returns its byte count.
- `Writer.writeAll(bytes str) !u64` continues through partial writes until all bytes are written; zero progress or a count larger than the remaining input is an error.
- `Writer.writeLn(bytes str) !u64` writes all bytes followed by `\n` and returns the combined count.
- `Writer.writeBool(b bool) !u64` writes `true` or `false`.
- `Writer.writeInt64(num i64) !u64` writes signed decimal, including the minimum `i64` value.
- `Writer.writeUint64(num u64) !u64` writes unsigned decimal.
- `Writer.writeFloat64(flt f64, precision u64) !u64` writes fixed-point decimal with `precision` fractional digits and handles NaN and infinities.

`digitToChar(i i16) u8` is an internal decimal formatting helper.
