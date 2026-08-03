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

## Types and scalar operations

- `Codepoint(value u32, width u8)` contains a Unicode scalar and its UTF-16
  width in code units.
- `Utf16Iterator(data u16[], index u64)` is a borrowed cursor over UTF-16 code
  units. `iterator(units)` constructs one; `hasData`, `peek`, and `next`
  inspect or advance it.
- `decode(units) !Codepoint` decodes the first scalar.
- `validate(units) bool` validates a complete UTF-16 slice.
- `encodedSize(cp) !u64` returns one or two code units for a valid scalar.
- `encode(cp, output) !u64` writes one scalar to caller-provided storage.

`toUtf8Size(units)` and `fromUtf8Size(text)` validate their input and return
the capacity required by `toUtf8` and `fromUtf8` respectively.

## Byte order and incremental decoding

`ENDIAN_LITTLE`, `ENDIAN_BIG`, and `ENDIAN_BOM` select byte decoding policy.
`BOM_NONE`, `BOM_LITTLE`, and `BOM_BIG` are the possible results from
`detectBom(bytes)`. The BOM constants intentionally share values with their
corresponding explicit endian constants.

`Decoder.acceptPair(first, second)` is the low-level state operation used by
incremental decoding to accept one byte pair. Most callers should use
`Decoder.push`, which performs buffering, output accounting, and validation.
