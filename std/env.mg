mod env
# Portable access to the process environment.

use "std:allocator" allocator
use "std:list" list

@platform("windows")
use "std:win/env_impl" impl

@platform("linux", "android", "ios", "darwin", "freebsd", "netbsd", "openbsd")
use "std:unix/env_impl" impl

pub get(name str) !$str:
    a := ctx.procAlloc
    ret try impl.get(name)
..

pub has(name str) bool:
    ret impl.has(name)
..

pub set(name str, value str) !void:
    try impl.set(name, value)
..

pub unset(name str) !void:
    try impl.unset(name)
..

pub list() !$list.List[str]:
    a := ctx.procAlloc
    ret try impl.list()
..
