# `std/fmt`

## Example

```magma
message := fmt.str(a, "Value: ").uint(5).str(", active: ").bool(true)

try message.writeTo(output)
```

`Format` is a short-lived deferred sequence of typed formatting operations. It
starts with capacity for eight parts and grows when necessary. Strings added by
`fmt.str` and `Format.str` are borrowed and must remain valid until the format is
rendered or freed.

Construction and chained append operations do not throw. The first allocation
or capacity error is retained; subsequent appends are ignored. A terminal
operation reports that error before writing any output.

## API

- `pub str(a alc.Allocator, initial str) $Format` starts a format with a borrowed string.
- `Format.str(value str) $Format` appends a borrowed string.
- `Format.uint(value u64) $Format` appends an unsigned decimal integer.
- `Format.int(value i64) $Format` appends a signed decimal integer.
- `Format.bool(value bool) $Format` appends `true` or `false`.
- `Format.float(value f64, precision u64) $Format` appends a fixed-point value.
- `destr Format.writeTo(out writer.Writer) !u64` writes and consumes the format.
- `destr Format.print() !u64` writes to standard output and consumes the format.
- `destr Format.toStr(a alc.Allocator) !$str` returns an owned string and consumes the format.
- `destr Format.free() void` discards an unrendered format.

Output can be partial if the destination writer itself fails. Construction
errors never produce partial output.
