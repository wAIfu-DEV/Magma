# `std/checked`

Checked integer arithmetic and size calculations. Every arithmetic operation
returns `errors.wouldOverflow` instead of wrapping. Division by zero and an
invalid shift count return `errors.invalidArgument`.

The unsuffixed `u` and `i` families operate on Magma's default `u64` and `i64`
integers. Wide operations carry an explicit `u128` or `i128` prefix:

```magma
use "std:checked" checked

bytes := try checked.uMul(count, sizeof Item)
next := try checked.iAdd(current, delta)
wide := try checked.u128Pow(base, exponent)
```

## Arithmetic

The unsigned families provide `uAdd`, `uSub`, `uMul`, `uDiv`, `uShl`, and
`uPow`, with corresponding `u128Add` through `u128Pow` operations.

The signed families provide `iAdd`, `iSub`, `iMul`, `iDiv`, `iNeg`, `iShl`,
and `iPow`, with corresponding `i128Add` through `i128Pow` operations.

Shift counts must be below the operand width. Signed shifts are checked as
mathematical multiplication by a power of two, including for negative values.
Integer powers use exponentiation by squaring; `0^0` is `1`.

## Conversions

Checked conversions complement the deliberately truncating operations in
`std/cast`:

- `uToU8`, `uToU16`, and `uToU32`
- `iToI8`, `iToI16`, and `iToI32`
- `uToI` and `iToU`
- `u128ToU`, `i128ToI`, `u128ToI128`, and `i128ToU128`

The unsuffixed destination `U` and `I` denote `u64` and `i64`. Negative signed
values are rejected when converting to an unsigned representation.

## Sizes and alignment

```magma
bytes := try checked.byteCount[Item](count)
total := try checked.addByteCount(headerSize, count, sizeof Item)
aligned := try checked.alignUp(total, 16)
```

- `byteCount[T]` checks `count * sizeof T`.
- `addByteCount` checks a base size plus an element-size product.
- `elementCount` requires a nonzero element size and an exactly divisible byte
  count.
- `alignUp` and `alignDown` require a nonzero power-of-two alignment.
- `isAligned` returns false for invalid alignments instead of throwing.

All operations are allocation-free, deterministic, and thread-safe.
