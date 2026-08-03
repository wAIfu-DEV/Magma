# `std/hex`

Strict hexadecimal encoding and decoding.

```magma
text := try hex.encode(a, bytes)       # lowercase
upper := try hex.encodeUpper(a, bytes)
decoded := try hex.decode(a, "CAFE")
```

`encodeTo`, `encodeUpperTo`, and `decodeTo` write into caller-provided storage.
Odd-length or non-hexadecimal input is rejected. Allocated strings and slices
must be released with the allocator passed to the operation.

`encodedSize(inputSize)` returns the required character count and detects size
overflow. `decodedSize(text)` validates the even input length and returns the
required byte count.
