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

The Windows standard-library backend now uses the three-state algorithm with
`WaitOnAddress`. The comparison sample retains the former SRWLOCK behavior as
its baseline and separately measures one million mutex lifecycles, five million
uncontended lock/unlock pairs, and contention with 2, 4, 8, and 24 threads.

Build it from a Windows command prompt and run at least five times:

```bat
Magma.exe --std .\std --emit exe -O 3 --out mutex-windows.exe samples\mutex_windows_benchmark.mg
FOR /L %i IN (1,1,5) DO @mutex-windows.exe
```

Both Windows implementations have zero-resource creation and destruction.
The lifecycle measurement nevertheless keeps every initialized mutex
observable so LLVM cannot remove the work. Record medians independently for
creation, uncontended locking, and each contention level.

Windows-only changes are compile-checked here but must be latency-benchmarked
on Windows. They include process-wide Winsock initialization, process-global
heap/QPC/standard-handle caches, `GetFileAttributesExW` metadata, and the
private overlapped implementation of `fs.readRange`.
