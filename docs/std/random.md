# `std/random`

## Example

```magma
rng := random.new(123)
die := rng.below(6) + 1 # 1 through 6
coin := rng.bool()

nonce := try random.bytes(16)
defer slices.free(nonce)
secureDie := try random.below(6) + 1

secure := random.os()
secureCoin := try secure.bool()
```

The module provides a small deterministic pseudorandom generator and direct
access to the operating system's cryptographically secure random source.
`Random` is deterministic and is not cryptographically secure.

## Type

`Random(state u64)` stores generator state.

`OsRandom(reserved u8)` is a stateless generator backed by the operating-system random source.
Create it with `os()`; its reserved field carries no state.

## API

- `pub new(seed u64) Random` initializes a generator. A zero seed is replaced with a nonzero default state.
- `Random.next() u64` advances the state and returns a pseudorandom 64-bit value.
- `Random.below(bound u64) u64` returns a value in `[0, bound)`. A zero bound returns zero.
- `Random.bool() bool` returns a pseudorandom boolean.
- `pub os() OsRandom` creates an operating-system-backed generator.
- `OsRandom.next() !u64` returns an operating-system-backed random `u64`.
- `OsRandom.below(bound u64) !u64` returns an unbiased value in `[0, bound)`. A zero bound returns zero.
- `OsRandom.bool() !bool` returns an operating-system-backed random boolean.
- `pub bytes(count u64) !$u8[]` allocates and returns bytes from the operating-system random source.
- `pub bytesTo(output u8[]) !void` fills caller-provided storage from the operating-system random source.
- `pub next() !u64` returns an operating-system-backed random `u64`.
- `pub below(bound u64) !u64` returns an unbiased operating-system-backed value in `[0, bound)`. A zero bound returns zero.
- `pub bool() !bool` returns an operating-system-backed random boolean.
