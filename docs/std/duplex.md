# `std/duplex`

`Duplex` is a type-erased bidirectional `proto` that also implements
`std/writer.Writer` and `std/reader.Reader`:

```magma
pub proto Duplex impl writer.Writer reader.Reader(
    write(bytes str) !u64
    readRaw(buff u8[], nBytes u64) !u64
)
```

A concrete type implements both required methods and produces a borrowed
`Duplex` view with `proto()`. `writer()` and `reader()` project that view into
the narrower interfaces without changing the underlying implementation.

`readToBuff(buff, nBytes) !u64` checks the destination extent, invokes
`readRaw`, and rejects a returned count larger than the request. Writer helper
methods are inherited through the implemented writer prototype. The concrete
object must remain stable and alive while any projected view is used.
