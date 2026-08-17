mod main

ordinary(value u64) u64:
    ret value
..

noctx bootstrap(callback noctx (u64) u64) void:
    callback = ordinary
..
