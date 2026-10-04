# External-call benchmarks

These samples compare the previous call path with the optimized path in the
same executable. Build them with `-O3`; run several times and compare medians.

```sh
Magma --std ./std --emit exe -O 3 --out /tmp/cpu-cache samples/cpu_cache_benchmark.mg
Magma --std ./std --emit exe -O 3 --out /tmp/env-lookup samples/env_lookup_benchmark.mg
Magma --std ./std --emit exe -O 3 --out /tmp/socket-open samples/nonblocking_socket_benchmark.mg
Magma --std ./std --emit exe -O 3 --out /tmp/fs-copy samples/fs_copy_benchmark.mg
Magma --std ./std --emit exe -O 3 --out /tmp/fs-range samples/fs_read_range_benchmark.mg
Magma --std ./std --emit exe -O 3 --out /tmp/poll-wake samples/poll_wake_benchmark.mg
Magma --std ./std --emit exe -O 3 --out /tmp/wake-token samples/wake_fastpath_benchmark.mg
Magma --std ./std --emit exe -O 3 --out /tmp/mutex-candidates samples/mutex_candidates_benchmark.mg
Magma --std ./std --emit exe -O 3 --out /tmp/lock-pathologies samples/lock_pathologies_benchmark.mg
```

The file benchmarks expect these inputs:

```sh
truncate -s 67108864 /tmp/magma-copy-source
truncate -s 4096 /tmp/magma-range-source
```

## Linux reference results

Five-run medians on the development machine, September 2026:

| Feature | Previous | Optimized | Change |
|---|---:|---:|---:|
| 100,000 processor-count reads | 478.94 ms | 0.024 ms | -99.99% |
| 100,000 environment lookups | 4.44 ms | 2.58 ms | -41.8% |
| 1,000 nonblocking socket lifetimes | 4.51 ms | 3.96 ms | -12.2% |
| 64 MiB file copy | 30.88 ms | 27.52 ms | -10.9% |
| 10,000 path-based range reads | 23.82 ms | 19.82 ms | -16.8% |
| 100,000 pending poll interrupts | 27.53 ms | 0.421 ms | -98.5% |
| 1,000,000 retained wake-token pairs | 18.43 ms | 12.52 ms | -32.0% |

The environment baseline contains an optimizer memory barrier so LLVM cannot
hoist `getenv` out of the loop. Socket benchmarks may require a sandbox that
allows local socket creation.

## Linux mutex candidates

Three Linux/x86-64 implementations were prototyped entirely in Magma and
compared with the current `std:mutex` pthread backend. The creation benchmark
measures one million complete lifecycles. For pthread this includes both
`pthread_mutex_init` and `pthread_mutex_destroy`; candidates have observable
state initialization but require no teardown. The lock benchmarks exclude
creation. Values below are five-run medians.

| Implementation | Lifecycle (1M) | Uncontended (5M pairs) | 4 threads (400k increments) |
|---|---:|---:|---:|
| pthread baseline | 13.289 ms | 46.646 ms | 16.499 ms |
| three-state futex | 0.551 ms | 42.356 ms | 13.036 ms |
| adaptive, 128 spins | 0.488 ms | 42.296 ms | 20.483 ms |
| ticket + futex bitset | 0.523 ms | 42.136 ms | 982.210 ms |

Contention scaling, using the same 100,000 protected increments per thread:

| Threads | pthread | Three-state | Adaptive | Ticket/futex |
|---:|---:|---:|---:|---:|
| 2 | 6.717 ms | 6.048 ms | 7.205 ms | 3.266 ms |
| 4 | 17.147 ms | 13.182 ms | 28.932 ms | 1,009.402 ms |
| 8 | 31.338 ms | 26.721 ms | 63.132 ms | 2,704.167 ms |
| 24 | 98.473 ms | 79.679 ms | 198.751 ms | 8,026.241 ms |

The three-state mutex is the only candidate that improves the median at every
tested thread count (10.0%, 23.1%, 14.7%, and 19.1%, respectively). Fixed
spinning wastes CPU under sustained contention. Strict ticket handoff is fast
with two threads, but targeted futex wakeups become prohibitively expensive as
the queue grows. The Linux prototypes remain benchmark-only; the pthread
implementation is unchanged.

## Windows mutex comparison

The Windows standard-library mutex uses SRWLOCK. The comparison sample measures
it against `std:spinlock`: one million lock lifecycles, five million
uncontended lock/unlock pairs, and contention with 2, 4, 8, and 24 threads.

Build it from a Windows command prompt and run at least five times:

```bat
Magma.exe --std .\std --emit exe -O 3 --out mutex-windows.exe samples\mutex_windows_benchmark.mg
FOR /L %i IN (1,1,5) DO @mutex-windows.exe
```

Both Windows implementations have zero-resource creation and destruction.
The lifecycle measurement nevertheless keeps every initialized mutex
observable so LLVM cannot remove the work.

Seven-run medians on the development machine, September 2026:

| Workload | SRW mutex | Spinlock | Spinlock change |
|---|---:|---:|---:|
| Creation, 1M | 0.494 ms | 0.251 ms | -49.2% |
| Uncontended, 5M pairs | 49.913 ms | 42.763 ms | -14.3% |
| 2 threads, 100k increments each | 3.181 ms | 4.024 ms | +26.5% |
| 4 threads, 100k increments each | 14.383 ms | 19.286 ms | +34.1% |
| 8 threads, 100k increments each | 52.479 ms | 49.054 ms | -6.5% |
| 24 threads, 100k increments each | 180.618 ms | 53.911 ms | -70.2% |

These contention cases deliberately protect only one increment, making them a
throughput stress test for the lock itself. The spinlock currently yields with
`SwitchToThread` after failed acquisition; it is not a pure `pause` loop.
Results can change substantially with critical-section duration, scheduling,
worker placement, and CPU count. SRWLOCK remains the general-purpose choice
because it can park waiters instead of consuming runnable CPU while a lock is
held.

Five additional low-contention candidates were tested with the same protected
increment workload. Seven-run medians were:

| Workload | SRW | TAS | TTAS | Backoff TTAS | Adaptive TTAS | Spin/park |
|---|---:|---:|---:|---:|---:|---:|
| Uncontended, 5M | 49.388 ms | 22.053 ms | 21.585 ms | 21.443 ms | 21.412 ms | 41.804 ms |
| 2 threads | 3.297 ms | 2.308 ms | 2.523 ms | 2.611 ms | 2.557 ms | 5.728 ms |
| 4 threads | 15.378 ms | 5.751 ms | 6.107 ms | 6.473 ms | 6.222 ms | 18.902 ms |
| 8 threads | 56.830 ms | 28.849 ms | 20.511 ms | 17.854 ms | 19.108 ms | 48.731 ms |
| 24 threads | 217.803 ms | 345.373 ms | 174.707 ms | 179.289 ms | 67.095 ms | 170.131 ms |

The spin/park implementation was removed because it lost to SRWLOCK at both
low-contention thread counts. TAS, TTAS, backoff TTAS, and adaptive TTAS remain
as benchmark candidates. TAS also degrades beyond SRWLOCK under heavy
contention, consistent with cache-line invalidation from repeated exchanges.

The adaptive candidate was then split into a single-CAS happy path and a
separate spin/yield slow path, and compared directly with SRWLOCK and the
standard spinlock. Seven-run medians:

| Workload | SRW | Adaptive TTAS | Spinlock |
|---|---:|---:|---:|
| Creation, 1M | 0.492 ms | 0.244 ms | 0.246 ms |
| Uncontended, 5M | 49.256 ms | 22.440 ms | 43.608 ms |
| 1 thread | 1.275 ms | 0.606 ms | 1.015 ms |
| 2 threads | 3.382 ms | 2.441 ms | 4.644 ms |
| 4 threads | 12.374 ms | 5.264 ms | 13.830 ms |
| 8 threads | 51.676 ms | 16.281 ms | 41.592 ms |
| 24 threads | 177.649 ms | 60.616 ms | 58.620 ms |

The adaptive lock retains the best low-contention path and approaches the
yielding spinlock under oversubscription. It is intended only for short,
non-blocking critical sections; SRWLOCK remains appropriate when owners may
block or hold the lock for an unpredictable duration.

The same executable was built and run under Ubuntu on WSL1
(`4.4.0-28000-Microsoft`). Seven-run medians were:

| Workload | pthread mutex | Adaptive TTAS | Spinlock |
|---|---:|---:|---:|
| Creation, 1M | 15.285 ms | 0.254 ms | 0.245 ms |
| Uncontended, 5M | 49.258 ms | 22.735 ms | 43.765 ms |
| 1 thread | 1.209 ms | 0.580 ms | 0.945 ms |
| 2 threads | 4.692 ms | 4.731 ms | 3.202 ms |
| 4 threads | 14.391 ms | 15.352 ms | 12.078 ms |
| 8 threads | 25.934 ms | 38.418 ms | 31.900 ms |
| 24 threads | 80.323 ms | 138.356 ms | 49.983 ms |

Adaptive results were bimodal once multiple workers contended, indicating
strong sensitivity to WSL1's scheduler. The implementation is correct on this
target, but these WSL1 scheduling results should not be generalized to native
Linux or WSL2.

### Native Linux adaptive-lock comparison

Seven-run medians on a 12-logical-CPU Ryzen 5 3600 system. Each contention
worker performs 100,000 lock-protected increments; the 24-worker case is
deliberately oversubscribed.

| Workload | pthread mutex | Adaptive TTAS | TAS | TTAS | Backoff TTAS | Spinlock |
|---|---:|---:|---:|---:|---:|---:|
| Uncontended, 5M | 46.828 ms | 21.784 ms | 22.021 ms | 21.691 ms | 21.429 ms | 42.054 ms |
| 1 thread | 1.034 ms | 0.590 ms | 0.613 ms | 0.480 ms | 0.604 ms | 0.895 ms |
| 2 threads | 4.947 ms | 2.462 ms | 2.143 ms | 1.919 ms | 2.122 ms | 2.146 ms |
| 4 threads | 14.833 ms | 4.695 ms | 4.833 ms | 6.318 ms | 6.036 ms | 7.371 ms |
| 8 threads | 30.825 ms | 15.884 ms | 26.902 ms | 17.781 ms | 17.755 ms | 28.814 ms |
| 24 threads | 84.937 ms | 62.649 ms | 384.860 ms | 138.740 ms | 129.233 ms | 45.663 ms |

For one million observable constructions, pthread mutex initialization plus
destruction took 13.511 ms, adaptive initialization took 0.243 ms, and
spinlock initialization took 0.246 ms. TAS, TTAS, and backoff use the same
single-zero-word representation as adaptive and therefore have equivalent
construction cost.

Adaptive TTAS is the strongest general short-critical-section candidate here:
it wins among the custom candidates at 4 and 8 workers and remains much faster
than pthread through 24 workers. Plain TTAS wins at 1 and 2 workers, while the
yielding spinlock wins the oversubscribed 24-worker case. TAS collapses under
heavy contention because every failed exchange invalidates the shared cache
line.

### Lock pathology comparison

`lock_pathologies_benchmark.mg` compares the three public lock choices through
their common failure modes. Five-run medians on the same 12-logical-CPU system:

| Workload | Mutex wall / CPU | Adaptive wall / CPU | Yielding spin wall / CPU |
|---|---:|---:|---:|
| 64 pauses, 8 threads | 263.945 / 636.860 ms | 167.838 / 774.649 ms | 171.272 / 1,022.330 ms |
| 512 pauses, 8 threads | 454.939 / 528.525 ms | 333.915 / 1,548.679 ms | 336.923 / 2,027.795 ms |
| 64 pauses, 24 threads | 198.373 / 522.839 ms | 131.853 / 1,012.724 ms | 133.523 / 1,300.857 ms |
| Owner yields, 8 threads | 37.025 / 169.880 ms | 19.417 / 89.686 ms | 20.659 / 106.004 ms |
| Owner sleeps 1 ms, 4 threads | 105.662 / 0.783 ms | 105.433 / 217.928 ms | 105.451 / 245.896 ms |

All fixed-work cases give every worker the same acquisition count. In the
500,000-acquisition fairness race with 12 workers, the median per-thread ranges
were 37,053–46,433 for mutex, 25,671–52,736 for adaptive, and 15,337–82,975 for
yielding spinlock. Thus adaptive reduces the extreme starvation/barging seen in
the yielding spinlock, but remains materially less fair than pthread mutex.

The blocking-owner case is the decisive limitation: wall time is necessarily
about 106 ms for every lock, but adaptive consumes 278 times the mutex CPU and
yielding spinlock consumes 314 times the mutex CPU. Adaptive remains suitable
for short, non-blocking ownership; mutex remains the safe default when a holder
may sleep, perform I/O, or be descheduled for a significant interval.

Windows-only changes are compile-checked here but must be latency-benchmarked
on Windows. They include process-wide Winsock initialization, process-global
heap/QPC/standard-handle caches, `GetFileAttributesExW` metadata, and the
private overlapped implementation of `fs.readRange`.
