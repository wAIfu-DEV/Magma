mod bytes

use "slices.mg"   slc
use "errors.mg"   errors
use "iterator.mg" iter

pub equal(a u8[], b u8[]) bool:
    n := slc.count(a)
    if n != slc.count(b):
        ret false
    ..
    i u64 = 0
    while i < n:
        if a[i] != b[i]:
            ret false
        ..
        i = i + 1
    ..
    ret true
..

pub indexByte(in u8[], value u8) !u64:
    i u64 = 0
    while i < slc.count(in):
        if in[i] == value:
            ret i
        ..
        i = i + 1
    ..
    throw errors.failure("byte not found")
..

pub contains(in u8[], value u8) bool:
    i u64, e error = indexByte(in, value)
    ret errors.code(e) == 0
..

pub startsWith(in u8[], prefix u8[]) bool:
    if slc.count(prefix) > slc.count(in):
        ret false
    ..
    i u64 = 0
    while i < slc.count(prefix):
        if in[i] != prefix[i]:
            ret false
        ..
        i = i + 1
    ..
    ret true
..

pub endsWith(in u8[], suffix u8[]) bool:
    if slc.count(suffix) > slc.count(in):
        ret false
    ..
    start := slc.count(in) - slc.count(suffix)
    i u64 = 0
    while i < slc.count(suffix):
        idx := start + i
        if in[idx] != suffix[i]:
            ret false
        ..
        i = i + 1
    ..
    ret true
..

pub reverse(in u8[]) void:
    n := slc.count(in)
    i u64 = 0
    while i < n / 2:
        right := n - i - 1
        tmp := in[i]
        in[i] = in[right]
        in[right] = tmp
        i = i + 1
    ..
..

iterHasData(impl ptr, index u64*) bool:
    bytesPtr u8[]* = impl
    bytes u8[] = *bytesPtr
    count := slc.count(bytes)
    ret *index < count
..

iterNext(impl ptr, index u64*) u8:
    bytesPtr u8[]* = impl
    bytes u8[] = *bytesPtr
    count := slc.count(bytes)
    idx := *index
    item := bytes[idx]
    *index = idx + 1
    ret item
..

pub iterator(bytes u8[]*) iter.Iterator[u8]:
    ret iter.new[u8](bytes, iterHasData, iterNext)
..
