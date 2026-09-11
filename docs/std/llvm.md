# `std/llvm`

`std:llvm` is the low-level, audited bridge to frequently needed LLVM IR
operations. Import it with a non-keyword alias:

```magma
use "std:llvm" as ll
bits := ll.populationCount64(value)
```

It includes:

- generic pointer reinterpretation, byte offsets, address conversion, loads,
  and stores;
- ABI-level extraction and construction of raw string and slice views;
- volatile `u8`, `u32`, and `u64` loads and stores;
- raw acquire, release, relaxed, sequentially consistent, exchange,
  fetch-add/subtract, and compare-exchange operations used by synchronization
  and platform modules;
- same-width `f32`/`u32` and `f64`/`u64` bit reinterpretation;
- integer extension and truncation, signed/unsigned bit preservation, and
  integer/`f64` conversions corresponding to `std:cast`;
- byte swapping, bit reversal, population counts, and leading/trailing-zero
  counts for common integer widths;
- branch expectation hints, assumptions, a sequentially consistent fence,
  a platform-selected processor pause/yield hint, and an unconditional trap.

Memory functions require valid, correctly aligned storage. Volatile operations
are intended for device memory and externally observable accesses; they are not
atomic. Passing false to `assume` is undefined behavior. `trap` does not return.

Prefer normal Magma expressions, `std:cast`, `std:memory`, or `std:atomic` when
they describe the operation: this module exists for semantics those APIs do not
expose.

`pause` uses `pause` on x86 and `yield` on ARM. Other supported architectures
receive an LLVM side-effect barrier, because LLVM has no target-independent
processor-pause intrinsic.
