# `std/random`

## Example

```magma
rng := random.new(123)
die := rng.bounded(6) + 1 # 1 through 6
coin := rng.bool()
```

A small deterministic pseudorandom generator. It is not cryptographically secure.

## Type

`Random(state u64)` stores generator state.

## API

- `pub new(seed u64) Random` initializes a generator. A zero seed is replaced with a nonzero default state.
- `Random.next() u64` advances the state and returns a pseudorandom 64-bit value.
- `Random.bounded(bound u64) u64` returns a value in `[0, bound)`. A zero bound returns zero.
- `Random.bool() bool` returns a pseudorandom boolean.
