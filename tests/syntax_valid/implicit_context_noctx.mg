mod main

use "std:context_default" context_default

ordinary() u64:
    ret 42
..

noctx plusOne(value u64) u64:
    ret value + 1
..

noctx bootstrap() u64:
    ctx = context_default.newNull()
    callback (u64) u64 = plusOne
    ret callback(ordinary())
..

main() void:
..

