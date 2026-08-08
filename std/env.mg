mod env
# Portable access to the process environment.

use "std:allocator" allocator
use "std:list" list

@platform("windows")
use "std:win/env_impl" impl

@platform("linux", "android", "ios", "darwin", "freebsd", "netbsd", "openbsd")
use "std:unix/env_impl" impl

pub get(a allocator.Allocator, name str) !$str:
    ret try impl.get(a, name)
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

pub list(a allocator.Allocator) !$list.List[str]:
    ret try impl.list(a)
..
