# Milestone 0 provisional baseline

Recorded on 2026-09-11 on Linux/amd64 with LLVM 22.1.8 and an AMD Ryzen 5 3600.
These figures establish that the harness works; they are not rollout thresholds.

The compiler-library benchmark performs fresh shared-state creation, parsing,
specialization, semantic and ownership checks, and lowering. It excludes final
native linking. Five one-iteration samples produced:

| Pipeline | Median | Observed range | Bytes allocated/op | Allocations/op |
| --- | ---: | ---: | ---: | ---: |
| Textual | 52.8 ms | 49.9–66.9 ms | about 29.5 MB | about 765,000 |
| Object, LLVM `-O0` | 131.5 ms | 120.1–134.4 ms | about 24.1 MB | about 759,000 |

Command:

```sh
GOCACHE=/tmp/magma-go-build go test -run='^$' \
  -bench='BenchmarkWholeProgramBaseline' -benchtime=1x -count=5 \
  -tags='llvm_object llvm22' ./src/compiler_pipeline
```

The end-to-end harness successfully compiled and executed textual and object
outputs at `-O0` and `-O3`. This container did not provide GNU `time`, so that
validation recorded wall time but not peak RSS or user/system CPU time. The
harness records all four values when `/usr/bin/time` or `/bin/time` is present.
The Milestone 0 stability gate remains open until repeated end-to-end runs,
including RSS and CPU measurements, are captured on the designated benchmark
machine.

## Incremental Milestone 5 result

Five Linux/amd64 end-to-end `-O3` runs on 2026-09-11, after seeding the cache,
measured a median 200 ms for the final interface-backed incremental path versus 371 ms for current
whole-program object lowering and 399 ms for textual lowering. Observed wall
ranges were 195–218 ms incremental, 364–384 ms object, and 388–406 ms textual.
The warm incremental result is about 46% faster than object and 50% faster than
textual on the representative fixture. Cold and warm executables both ran
successfully; repeated warm output objects were byte-identical through the
final-artifact cache.

## Larger incremental benchmark

`incremental_baseline.sh` intentionally uses a small correctness fixture and
is not representative of optimizer-heavy programs. The larger benchmark
generates eight modules containing 640 nontrivial functions. Runtime input
keeps the optimizer from folding the workload into a constant.

It separately records cold, unchanged-graph warm, and leaf-change warm object
builds. Run it with:

```sh
benchmarks/incremental_large.sh /path/to/magma 5
```

Object emission is deliberate: it isolates frontend, LLVM optimization, and
machine-code generation costs from the native linker. Before introducing
pre-optimized bitcode, this benchmark should establish how much of a partial
warm build remains in LLVM lowering and code generation.

The harness also compares output bytes. An unchanged warm object must match the
cold object, while changing executable code in one leaf module must produce a
different object. Timing results are rejected if either invariant fails.

After correcting implementation-source invalidation, a three-run measurement
found median wall times of approximately 901 ms cold, 170 ms unchanged-warm,
and 687 ms after changing one leaf implementation. Median LLVM lowering was
744 ms cold and 567 ms after the leaf change. Every run passed the output-byte
invariants.

`tests/incremental_large` separately preserves a stdlib-heavy real-program
fixture derived from the filesystem-copy benchmark. It covers compatible
cross-unit declarations and private backing layouts in public types, while the
generated fixture remains the stable source of cache performance numbers.

### Cross-module runtime comparison

`cross_module_runtime.sh` builds three branch/multiply workloads through the
native whole-program and ThinLTO pipelines. Every
workload performs 10 million iterations with respectively 2, 8, or 16
cross-module calls per iteration. A stable runtime-only `getppid()` seed keeps
LLVM from precomputing the loop, and the harness rejects result disagreement.
Twenty-one paired, order-alternating Linux/amd64 runs produced these medians:

| Cross-module workload | Whole-program `-O3` | Module objects `-O3` | Module delta |
| --- | ---: | ---: | ---: |
| Low: 2 calls/iteration | 29.840 ms | 31.341 ms | +5.0% |
| Medium: 8 calls/iteration | 121.033 ms | 120.074 ms | -0.8% |
| Heavy: 16 calls/iteration | 244.433 ms | 240.496 ms | -1.6% |

Process launch is included but is small relative to these loop durations. The
medium/heavy differences are close enough to treat as parity on this workload;
the low-density result shows a small repeatable module-boundary penalty.

The following module-object figures are retained as historical evidence for
why that experimental pipeline was removed. Whole-program executables were
47,424 bytes. Module-object executables ranged
from 142,496 to 143,664 bytes, roughly three times larger because the native
linker cannot perform LLVM whole-program inlining and dead-code elimination.
An inline-sensitive control replaces each branch/multiply helper with a single
addition. Eleven paired runs show the module-object route's worst case:

| Trivial cross-module workload | Whole-program `-O3` | Module objects `-O3` | Slowdown |
| --- | ---: | ---: | ---: |
| Low: 2 calls/iteration | 1.696 ms | 24.089 ms | 14.2x |
| Medium: 8 calls/iteration | 2.202 ms | 84.801 ms | 38.5x |
| Heavy: 16 calls/iteration | 1.778 ms | 163.835 ms | 92.1x |

Whole-program LLVM inlines these helpers and collapses most of the fixed loop;
native object boundaries preserve every runtime call. Reproduce this control
by setting `MAGMA_CROSS_MODULE_KERNEL=trivial` when invoking the harness.
Run the benchmark with:

```sh
benchmarks/cross_module_runtime.sh /path/to/magma 21
```

The shell's timing builtin supplied wall/user/system figures because GNU
`time` is unavailable in this container; peak RSS remains unavailable here.

### Experimental ThinLTO result

The driver-based ThinLTO route uses cached summarized module bitcode and LLD's
persistent ThinLTO backend cache. On a 531-unit target, isolated
`-O3` builds measured 6.16 s for a cold cached whole-program build, 4.62 s for
ThinLTO, and 4.08 s for independent module objects. Seven unchanged warm runs
had medians of 0.97 s, 1.02 s, and 1.03 s respectively. After changing one
private implementation, the same caches rebuilt in 3.65 s, 1.12 s, and 1.06 s:
ThinLTO was 3.26x faster than whole-program optimization and within 6% of the
unsafe independent-object route.

Eleven order-rotated runtime runs found ThinLTO within launch noise of the
whole-program pipeline on the inline-sensitive control, eliminating the
module-object route's 14–92x slowdown. On the branch/multiply workload its
medians were approximately 31.4 ms, 136.0 ms, and 268.4 ms for low, medium,
and heavy, versus 30.3 ms, 121.5 ms, and 245.8 ms whole-program: a roughly
4–12% runtime cost. ThinLTO executables were about twice the whole-program
size but substantially smaller than independent module-object executables.
This is a meaningful edit-build/runtime compromise, so ThinLTO is now the
default pipeline. The independent module-object route was removed because its
cross-module runtime cost was unacceptable.

### Preset selection

On the same target, the initial proposed `fast-comp` configuration of
ThinLTO `-O0` took 4.76 s cold and 1.02 s unchanged-warm. Whole-program `-O0`
took 5.03 s cold and 0.98 s warm. Textual `-O0` took 0.89 s and 0.88 s, so the
measured preset uses textual `-O0` rather than ThinLTO. The default ThinLTO
`-O3` preset measured 4.68 s cold, 1.01 s warm, and 1.12 s after a private
implementation edit. The `fast-runtime` whole-program `-O3` preset measured
6.18 s cold, 0.96 s warm, and 3.55 s after the same edit, while retaining the
best arithmetic-heavy runtime and smallest output.
