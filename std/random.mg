mod random
# Deterministic pseudo-random values generated from an explicit seed.
# @warning Not suitable for cryptographic use.

# Stateful deterministic pseudo-random number generator.
pub Random(
    state u64
)

# Creates a generator from a seed; zero selects a fixed nonzero seed.
# Equal seeds produce equal sequences.
# @complexity O(1)
# @example
#   rng := random.new(42)
pub new(seed u64) Random:
    actualSeed := seed
    if actualSeed == 0:
        actualSeed = 11400714819323198485
    ..
    ret Random(state=actualSeed)
..

# Advances the generator and returns a value spanning the u64 range.
# @complexity O(1)
# @example
#   value := rng.next()
Random.next() u64:
    x := this.state
    x = x ^ (x >> 12)
    x = x ^ (x << 25)
    x = x ^ (x >> 27)
    this.state = x
    ret x * 2685821657736338717
..

# Returns a value in [0, bound), or zero when bound is zero.
# @complexity O(1)
# @warning Modulo reduction introduces bias unless bound divides the u64 range.
# @example
#   dieRoll := rng.bounded(6) + 1
Random.bounded(bound u64) u64:
    if bound == 0:
        ret 0
    ..
    ret this.next() % bound
..

# Returns a pseudo-random boolean with equal probability for both values.
# @complexity O(1)
# @example
#   heads := rng.bool()
Random.bool() bool:
    ret (this.next() & 1) == 1
..
