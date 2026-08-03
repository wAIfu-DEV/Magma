# Standard Library Roadmap

This roadmap expands Magma's standard library in dependency order. It
prioritizes systems-programming foundations over domain-specific conveniences
and incorporates lessons from XSTD and Wade32's allocator implementations.

## Phase 1: Allocator ecosystem (implemented)

Add:

- `std/arena_alloc`
- `std/scratch_alloc`
- `std/debug_alloc`
- allocator statistics and documented behavioral conventions
- explicit allocator teardown where metadata is owned

The debug allocator improves on the XSTD and Wade32 implementations:

- validate a pointer before forwarding `free`
- reject and report double or foreign frees
- preserve realloc failure semantics
- distinguish allocation, reallocation, and free counters
- accurately track requested bytes, metadata bytes, and live allocations
- expose leak iteration and reporting
- support fixed-capacity tracking for kernels and embedded targets
- optionally grow its tracking table
- document thread-safety and require external locking for the initial
  implementation

Implemented API shape:

```magma
options := debug_alloc.Options(
    initialCapacity=256,
    canGrow=true,
    rejectUntrackedFree=true,
)

debug := try debug_alloc.new(heap.allocator(), options)
defer debug.free()

a := debug.allocator()
stats := debug.stats()
leak := try debug.leak(0)
```

Add allocator conformance tests reusable by every allocator implementation.

## Phase 2: Checked numerical operations (implemented)

Added `std/checked` with typed operations:

- addition, subtraction, multiplication, and division
- shifts and negation
- integer powers
- narrowing conversions
- alignment helpers
- byte-count and element-count calculations

Operations return Magma errors rather than silently wrapping:

```magma
size := try checked.uMul(count, sizeof T)
```

The default `u` and `i` operation families use `u64` and `i64`. Explicit
`u128` and `i128` families provide the same checked operations for wide
integers. Checked conversion, allocation-size, element-count, and alignment
helpers are included. Typed allocator operations and the first allocation-heavy
containers now use the shared helpers.

Add separate `wrapping` and `saturating` modules only if normal arithmetic
semantics need explicit alternatives.

## Phase 3: Unicode and text codecs (implemented)

Expanded beyond the existing UTF-8 support:

- `std/utf16`
- UTF-8 to UTF-16 and UTF-16 to UTF-8 conversion
- validated and lossy conversions
- code-point iterators
- surrogate handling
- endian-aware UTF-16 decoding
- byte-order-mark detection
- incremental decoding for readers

Then add small codec modules:

- hexadecimal
- Base64
- percent encoding
- UTF-32 conversions remain deferred; scalar `u32` APIs cover the immediate use cases

Implemented modules are `std/unicode`, `std/utf16`, `std/hex`, `std/base64`,
and `std/percent`. UTF-8 and UTF-16 expose strict scalar operations,
validation, lossy conversion, and bounded incremental decoders. UTF-16 raw-byte
decoding supports explicit little/big endian input and BOM-selected input.
Hexadecimal and Base64 provide allocating and caller-buffer APIs. Percent
encoding separates URI components, path segments, and form semantics.

UTF-16 should be completed before broadening Windows-native APIs.

## Phase 4: Environment and command lines (implemented)

Add:

- `std/env` for getting, setting, unsetting, and enumerating variables
- `std/args` for convenient access beyond raw `main` arguments
- `std/flag` for typed command-line parsing

`std/flag` should support booleans, integers, strings, repeated options,
positional arguments, `--`, generated usage, and errors for unknown or malformed
options.

Keep shell command construction out of this API. Process arguments should
remain structured arrays.

Implemented modules are `std/env`, `std/args`, and `std/flag`. Environment
strings intentionally follow native null-termination semantics without an
extra embedded-null validation traversal. Environment enumeration returns an
owned snapshot. Flags support typed scalar and repeated values, clustered short
booleans, positionals, `--`, writer-based and allocated usage text, and strict
unknown/malformed option handling. `process.spawnWithEnv` supplies a complete
replacement child environment without mutating the parent process.

## Phase 5: Complete filesystem support (implemented)

Extend `std/fs` and `std/path` with:

- directory iteration
- recursive walking
- file metadata
- permissions and file kinds
- directory creation and removal
- rename and atomic replacement
- copying
- symbolic-link inspection where supported
- current and temporary directories
- canonicalization
- platform-neutral path joining and normalization

Directory walking should use an iterator or callback instead of allocating an
entire tree.

Implemented with lexical `std/path` joining and normalization, owned directory
iteration through `openDir`, metadata and link inspection, metadata-aware
walking, `makeDir`/`makeDirs`, recursive removal, rename and atomic replacement,
file copying, permissions, current and temporary directory access, and
canonicalization. Directory entries own their names and are explicitly freed;
walkers do not allocate a complete tree. The original callback walker remains
available for compatibility.

## Phase 6: Networking foundations

Introduce networking in layers:

1. `std/net/address`
2. `std/net/dns`
3. `std/net/socket`
4. `std/net/tcp`
5. `std/net/udp`

Connections should integrate with `Reader`, `Writer`, `Duplex`, timeouts, and
ownership annotations. Once these layers are stable, refactor `std/http` onto
them and expand it beyond its current Windows-specific implementation.

TLS should be a separate package or binding, not an improvised cryptographic
implementation inside the standard library.

## Phase 7: Time and calendar support

Keep monotonic duration handling separate from civil time. Add:

- `Duration`
- `Instant` for monotonic measurement
- Unix timestamps
- UTC date and time conversion
- calendar fields
- ISO 8601 parsing and formatting
- checked duration arithmetic
- deadlines and timeouts

Timezone-database support is large enough to remain an external package
initially.

## Phase 8: Dynamic libraries and platform services

Add `std/dynlib` with operations to:

- open a library
- find a symbol
- close a library
- expose platform errors cleanly

Then consider:

- executable-path discovery
- host and user information
- terminal capability queries
- environment and operating-system identifiers

Avoid turning `std/c` into a dumping ground for native APIs.

## Phase 9: Testing, benchmarking, and diagnostics

Add a first-class testing library and compiler test-runner integration:

- assertions
- equality helpers
- expected-error assertions
- parameterized cases
- allocation-failure testing
- temporary files and directories
- deterministic random seeds
- benchmarks with warm-up and iteration control

The allocator conformance suite should test:

- zero-sized operations
- alignment
- realloc growth, shrinking, and movement
- exhaustion
- foreign pointers
- double frees
- metadata exhaustion
- arithmetic overflow
- cleanup after injected failures

Also add stack-trace or crash-reporting hooks where platform support permits.

## Phase 10: Math and geometry

Add these after the systems foundations:

- `std/math` scalar functions
- `Vec2[T]`, `Vec3[T]`, and possibly `Vec4[T]`
- rectangles and bounds
- clamp, minimum, maximum, and interpolation operations
- checked area and volume calculations

Geometry may fit better under `std/math/geometry` or in the raylib ecosystem
than in the default core library.

## Recommended implementation order

1. Allocator conformance framework
2. Arena allocator
3. Scratch allocator
4. Correct debug allocator
5. Checked arithmetic
6. UTF-16 and codecs
7. Environment and command-line parsing
8. Filesystem completion
9. Networking foundation and HTTP refactor
10. Duration and calendar APIs
11. Testing and benchmarking framework
12. Dynamic libraries
13. Math and geometry

Every new module should ship with:

- reference documentation
- unit and failure-path tests
- Windows and Unix implementations where applicable
- ownership and lifetime documentation
- at least one sample
- explicit thread-safety guarantees

The first release milestone should stop after checked arithmetic. A robust
allocator ecosystem plus reusable conformance tests provides the most value to
Magma's systems-programming identity and establishes infrastructure needed by
nearly every later feature.
