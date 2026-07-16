# `std/duplex`

## Example

```magma
const streamVTable := duplex.DuplexVTable(
    fn_write=writeCallback,
    fn_read=readCallback,
)
stream := duplex.new(state, addrof streamVTable)
try stream.writer().writeAll("ping")
n := try stream.reader().readToBuff(buffer, 4)
```

Combines read and write callbacks over one implementation pointer.

## Type

`DuplexVTable(fn_write (ptr, str) !u64, fn_read (ptr, u8[], u64) !u64)` stores callbacks shared by implementations.

`Duplex(impl ptr, vtable DuplexVTable*)` stores adapter state and a pointer to that shared table. It occupies 16 bytes on 64-bit targets.

## API

- `pub new(impl ptr, vtable DuplexVTable*) Duplex` constructs the adapter.
- `Duplex.writer() wr.Writer` returns a writer interface using `fn_write`.
- `Duplex.reader() rd.Reader` returns a reader interface using `fn_read`.

The duplex object and `impl` state must remain valid while either returned interface is used.
