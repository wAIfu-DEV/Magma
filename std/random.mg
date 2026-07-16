mod random

Random(
    state u64
)

pub new(seed u64) Random:
    actualSeed := seed
    if actualSeed == 0:
        actualSeed = 11400714819323198485
    ..
    ret Random(state=actualSeed)
..

Random.next() u64:
    x := this.state
    x = x ^ (x >> 12)
    x = x ^ (x << 25)
    x = x ^ (x >> 27)
    this.state = x
    ret x * 2685821657736338717
..

Random.bounded(bound u64) u64:
    if bound == 0:
        ret 0
    ..
    ret this.next() % bound
..

Random.boolean() bool:
    ret (this.next() & 1) == 1
..
