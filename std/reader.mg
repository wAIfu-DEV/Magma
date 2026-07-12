mod reader

use "allocator.mg" alc
use "slices.mg"    slices
use "strings.mg"   strings
use "errors.mg"    errors
use "cast.mg"      cast

# Reader interface for pulling bytes into strings or buffers.
# O(1) wrapper calls; underlying reader decides cost.
Reader(
    impl ptr,
    fn_read (ptr, u8[], u64) !u64,
)

pub new(impl ptr, readFunc (ptr, u8[], u64) !u64) Reader:
    r Reader
    r.impl = impl
    r.fn_read = readFunc
    ret r
..

# Reads up to nBytes and returns a string containing the bytes read.
# Warning: returned string is backed by allocator-owned memory.
# O(N) for nBytes.
# @param a allocator to use
# @param nBytes maximum bytes to read
# @returns string with read bytes
Reader.read(a alc.Allocator, nBytes u64) !$str:
    if nBytes == 0:
        ret strings.fromPtrNoCopy(cast.utop(0), 0)
    ..
    buffPtr u8* = try a.alloc(nBytes)
    buff u8[] = slices.fromPtr(buffPtr, nBytes)
    readCnt u64, readErr error = this.readToBuff(buff, nBytes)
    if errors.code(readErr) != 0:
        a.free(buffPtr)
        throw readErr
    ..
    ret strings.fromPtrNoCopy(buffPtr, readCnt)
..

# Reads into the provided buffer up to nBytes bytes.
# O(N) for nBytes.
# @param buff destination buffer
# @param nBytes number of bytes to read
# @returns number of bytes read
Reader.readToBuff(buff u8[], nBytes u64) !u64:
    if slices.count(buff) < nBytes:
        throw errors.invalidArgument("would overflow")
    ..
    readCnt u64 = try this.fn_read(this.impl, buff, nBytes)
    if readCnt > nBytes:
        throw errors.failure("reader returned more bytes than requested")
    ..
    ret readCnt
..
