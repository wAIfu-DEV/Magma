# `std/atomic`

Atomic numeric values for cross-thread coordination. Construct values before
sharing them and do not copy an atomic after publishing it to another thread.

- `newU8`, `newU32`, `newU64`, `newI64`, and `newF64` create initialized
  `U8`, `U32`, `U64`, `I64`, and `F64` values.
- Every type provides sequentially consistent `load`, `store`, and `exchange`.
- Integer types provide `fetchAdd` and `fetchSub`, returning the previous value.
- `U8` and `U32` provide acquire loads and release stores. `U32` also provides
  `fetchAddRelease` and `fetchSubAcqRel`.
- `U64` adds acquire/release and relaxed loads and stores plus
  `fetchAddRelaxed`.

The ordering-specific method names are `loadAcquire`, `storeRelease`,
`loadRelaxed`, `storeRelaxed`, `fetchAddRelease`, `fetchSubAcqRel`, and
`fetchAddRelaxed`; availability depends on the numeric type as listed above.

Relaxed operations guarantee atomicity but do not order unrelated memory. Pair
a release operation with an acquire operation when publishing data. Integer
arithmetic wraps. `F64.exchange` preserves the exact IEEE-754 bit pattern;
floating-point arithmetic operations are not provided.
