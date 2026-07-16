mod memory

use "cast.mg" cast

# Copies n bytes from 'from' to 'to'.
# Warning: prefer move() for possibly overlapping regions
# O(N) for n bytes.
# @param from source pointer
# @param to destination pointer
# @param n number of bytes to copy
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

# Copies n bytes from possibly overlapping 'from' to 'to'.
# O(N) for n bytes.
# @param from source pointer
# @param to destination pointer
# @param n number of bytes to copy
pub move(from ptr, to ptr, n u64) void:
    reg0 u64 = cast.ptou(from)
    reg1 u64 = cast.ptou(to)

    # Subtraction is used instead of reg0 + n so an address-range check cannot
    # wrap at U64_MAX.
    if reg1 > reg0 && (reg1 - reg0) < n:
        au u8* = from
        bu u8* = to

        bound u64 = 0 - 1 # U64_MAX
        i u64 = n - 1
        while i != bound: # stops after 0
            bu[i] = au[i]
            i = i - 1
        ..
    else:
        # safe to copy left-to-right
        copy(from, to, n)
    ..
..

# Swaps n bytes between non-overlapping x and y.
# Warning: Using with overlapping x and y may cause loss of data
# O(N) for n bytes, zero allocation.
# @param x first pointer
# @param y second pointer
# @param n number of bytes to swap
pub swap(x ptr, y ptr, n u64) void:
    ax u8* = x
    ay u8* = y

    i u64 = 0
    while i < n:
        tmp u8 = ax[i]
        ax[i] = ay[i]
        ay[i] = tmp
        i = i + 1
    ..
..

# Compares two byte ranges and returns true if all n bytes match.
# O(N) for n bytes.
# @param a first pointer
# @param b second pointer
# @param n number of bytes to compare
# @returns true if all bytes are equal
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

# Fills n bytes starting at in with the provided byte value.
# O(N) for n bytes.
# @param in destination pointer
# @param n number of bytes to write
# @param with byte value to set
pub set(in ptr, n u64, with u8) void:
    # will lower to @llvm.memset.p0i8.i64

    inu u8* = in

    i u64 = 0
    while i < n:
        inu[i] = with
        i = i + 1
    ..
..

# Zeros n bytes starting at in.
# O(N) for n bytes.
# @param in destination pointer
# @param n number of bytes to zero
pub zero(in ptr, n u64) void:
    set(in, n, 0)
..

# Returns a zero initialized value of type T 
# O(1)
pub zeroValue[T]() T:
    x T
    ret x
..

