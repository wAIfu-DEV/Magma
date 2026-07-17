# `std/cpu`

`coreCount()` returns the number of online logical CPU cores available to the
process. It always returns at least 1, including when the operating system
cannot provide a count, so it can be used directly as a default worker count.

```magma
use "std/cpu.mg" cpu

workers u64 = cpu.coreCount()
```

The value describes logical cores, not physical processor packages or physical
cores, and may change between program runs when the machine or process CPU set
changes.
