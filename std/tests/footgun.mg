mod main

use "../footgun.mg" footgun

Owned(value u64)
destr Owned.free() void: this.value = 0 ..

makeOwned() !$Owned:
    ret Owned(value=42)
..

pub main() !void:
    value := try makeOwned()
    footgun.drop[Owned](value)
..
