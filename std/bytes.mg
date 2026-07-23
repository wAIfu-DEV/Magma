mod bytes
# Allocation-free algorithms and iterators for byte slices.

use "std:slices"   slc
use "std:errors"   errors
use "std:iterator" iter

# Compares two byte slices by length and contents.
# @complexity O(N), where N is the slice length.
# @param a first byte slice
# @param b second byte slice
# @returns true when both slices contain the same bytes in the same order
# @example
#   same := bytes.equal(left, right)
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

# Finds the first occurrence of a byte.
# @complexity O(N)
# @param in slice to search
# @param value byte to find
# @returns index of the first matching byte
# @throws outOfBounds when the byte is absent
# @example
#   index := try bytes.indexByte(data, 10)
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

# Reports whether a slice contains a byte.
# @complexity O(N)
# @param in slice to search
# @param value byte to find
# @example
#   found := bytes.contains(data, 0)
pub contains(in u8[], value u8) bool:
    i u64, e error = indexByte(in, value)
    ret errors.code(e) == 0
..

# Reports whether a slice begins with prefix.
# @complexity O(P), where P is the prefix length.
# @param in slice to inspect
# @param prefix expected leading bytes
# @example
#   matches := bytes.startsWith(data, prefix)
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

# Reports whether a slice ends with suffix.
# @complexity O(S), where S is the suffix length.
# @param in slice to inspect
# @param suffix expected trailing bytes
# @example
#   matches := bytes.endsWith(data, suffix)
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

# Reverses the bytes in a slice in place.
# @complexity O(N)
# @param in mutable slice to reverse
# @example
#   bytes.reverse(data)
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

iterNext(impl ptr, index u64*) !u8:
    bytesPtr u8[]* = impl
    bytes u8[] = *bytesPtr
    count := slc.count(bytes)
    idx := *index
    item := bytes[idx]
    *index = idx + 1
    ret item
..

# Creates a non-owning iterator over a byte slice.
# @complexity O(1) to create; O(1) per yielded byte
# @param bytes pointer to the slice to iterate
# @returns iterator that yields bytes in index order
# @ownership The slice and its backing storage must outlive the iterator.
# @example
#   it := bytes.iterator(addrof data)
pub iterator(bytes u8[]*) iter.Iterator[u8]:
    ret iter.new[u8](bytes, iterHasData, iterNext)
..
