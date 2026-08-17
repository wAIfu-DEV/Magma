# `std/reader`

`Reader` is a type-erased byte-input `proto`:

```magma
pub proto Reader(
    readRaw(buff u8[], nBytes u64) !u64
)
```

Concrete reader types implement `readRaw` and expose a borrowed view with
`proto()` or a helper such as `File.reader()`. The implementation and its
storage must outlive the view.

- `read(a, nBytes) !$str` allocates space with `a`, requests up to `nBytes`,
  validates the adapter's returned count, and returns an owned string truncated
  to the bytes obtained.
- `readToBuff(buff, nBytes) !u64` rejects a request larger than `buff`, invokes
  `readRaw`, and rejects an adapter count larger than the request.
- `readAsync(nBytes) !$future.Future[str]` copies the interface into future
  work storage and runs `read` through `ctx.exec`, allocating through
  `ctx.procAlloc`. The implementation and all context components must remain valid
  until the future is awaited.

Read semantics, EOF signaling, and other errors come from the concrete
adapter. A zero count is a valid successful result; higher-level adapters such
as `std/buffered` define their own EOF behavior.
