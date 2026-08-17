# `std/writer`

`Writer` is a type-erased byte-output `proto` whose implementation supplies
`write(bytes str) !u64`. Concrete writers expose a borrowed view with `proto()`
or a helper such as `File.writer()`.

- `writeAll(bytes) !u64` repeats writes until all bytes are accepted. Zero
  progress or a count larger than the remaining input is an error.
- `writeLn(bytes) !u64` writes all bytes followed by `\n`.
- `writeBool`, `writeInt64`, `writeUint64`, and `writeFloat64` format primitive
  values directly to the sink. Floating point uses fixed precision and handles
  NaN and infinities.

`ConstWriter` is a concrete `Writer` implementation for statically stored
sinks. It contains an implementation pointer and write function, provides
`write`, `writeAll`, and `writeLn`, and returns a borrowed general view through
`toWriter()`.

The implementation behind either interface must remain valid for every call.
A successful single `write` may be partial; use `writeAll` when completeness is
required.
