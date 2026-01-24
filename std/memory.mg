mod memory

use "cast.mg" cast

pub copy(from ptr, to ptr, n u64) void:
    # will lower to @llvm.memcpy.p0.p0.i64

    au u8* = from
    bu u8* = to

    i u64 = 0
    while i < n:
        bu[i] = au[i]
        i = i + 1
    ..
..

pub compare(a ptr, b ptr, n u64) bool:
    # fails to lower to llvm intrinsics, however code is tight so it should be good,
    # though it could use some optimization with variable length chunking.

    au u8* = a
    bu u8* = b
    
    i u64 = 0
    while i < n:
        if au[i] != bu[i]:
            ret false
        ..
        i = i + 1
    ..
    ret true
..

pub set(in ptr, n u64, with u8) void:
    # will lower to @llvm.memset.p0i8.i64

    inu u8* = in

    i u64 = 0
    while i < n:
        inu[i] = with
        i = i + 1
    ..
..

pub zero(in ptr, n u64) void:
    set(in, n, 0)
..
