# `std/duplex`

Combines read and write callbacks over one implementation pointer.

## Type

`Duplex(impl ptr, fn_write (ptr, str) !u64, fn_read (ptr, u8[], u64) !u64)` stores shared adapter state and its callbacks.

## API

- `pub new(impl ptr, writeFunc (ptr, str) !u64, readFunc (ptr, u8[], u64) !u64) Duplex` constructs the adapter.
- `Duplex.writer() wr.Writer` returns a writer interface using `fn_write`.
- `Duplex.reader() rd.Reader` returns a reader interface using `fn_read`.

The duplex object and `impl` state must remain valid while either returned interface is used.
