mod hash

use "strings.mg" strings
use "slices.mg" slices

pub bytes(in u8[]) u64:
    h u64 = 14695981039346656037
    i u64 = 0
    while i < slices.count(in):
        h = h ^ in[i]
        h = h * 1099511628211
        i = i + 1
    ..
    ret h
..

pub string(in str) u64:
    data u8[] = slices.fromPtr(strings.toPtr(in), strings.countBytes(in))
    ret bytes(data)
..
