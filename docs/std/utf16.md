# `std/utf16`

Validated UTF-16 iteration, scalar encoding, conversion, byte-order handling,
lossy recovery, and incremental decoding.

```magma
wide := try utf16.fromUtf8(a, "hello")
defer slices.free(a, wide)

text := try utf16.toUtf8(a, wide)
defer text.free(a)

it := utf16.iterator(wide)
while it.hasData():
    cp := try it.next()
..
```

Strict conversion rejects unpaired surrogates. `toUtf8Lossy` and
`fromUtf8Lossy` replace malformed input with `U+FFFD` while always advancing.

`decodeBytes(a, bytes, endian)` accepts `ENDIAN_LITTLE`, `ENDIAN_BIG`, or
`ENDIAN_BOM`. BOM-selected decoding requires and consumes a BOM; explicit byte
orders do not consume `U+FEFF`.

For chunked input, construct `newDecoder(endian)` and call
`Decoder.push(input, scalarOutput)`. `DecodeResult` reports consumed bytes,
written scalars, and whether more input or output space is needed. Call
`finish()` at end of input. Decoder state owns no allocation.
