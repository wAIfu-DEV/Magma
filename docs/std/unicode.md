# `std/unicode`

Allocation-free Unicode scalar and UTF-16 surrogate primitives.

```magma
if unicode.isScalar(cp):
    pair := try unicode.splitSurrogate(cp)
..
```

- `isScalar(cp)` excludes surrogate values and values above `U+10FFFF`.
- `isHighSurrogate(unit)` and `isLowSurrogate(unit)` classify UTF-16 units.
- `combineSurrogates(high, low)` validates and combines a pair.
- `splitSurrogate(cp)` returns `SurrogatePair(high, low)` for a supplementary scalar.
- `REPLACEMENT` is `U+FFFD`; `MAX_SCALAR` is `U+10FFFF`.

All operations are allocation-free and thread-safe.
