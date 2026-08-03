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
