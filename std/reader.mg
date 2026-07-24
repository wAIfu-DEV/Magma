mod reader
# Type-erased byte input with convenience methods for exact and allocated reads.

use "std:allocator" alc
use "std:slices"    slices
use "std:strings"   strings
use "std:errors"    errors
use "std:cast"      cast
use "std:footgun"   footgun

# Reader interface for pulling bytes into strings or buffers.
# @complexity O(1) wrapper calls; underlying reader decides cost.
pub Reader(
    impl ptr,
    fn_read (ptr, u8[], u64) !u64,
)

# Creates a reader over caller-owned state.
# @complexity O(1)
# @ownership impl must remain valid while the Reader is used.
# @example
#   input := reader.new(state, readCallback)
pub new(impl ptr, readFunc (ptr, u8[], u64) !u64) Reader:
    ret Reader(impl=impl, fn_read=readFunc)
..

# Reads up to nBytes and returns a string containing the bytes read.
# @warning returned string is backed by allocator-owned memory.
# @complexity O(N) for nBytes.
# @param a allocator to use
# @param nBytes maximum bytes to read
# @returns string with read bytes
# @ownership Release the returned string with a.
# @example
#   chunk := try input.read(a, 4096)
Reader.read(a alc.Allocator, nBytes u64) !$str:
    if nBytes == 0:
        ret try strings.alloc(a, 0)
    ..
    result str = try strings.alloc(a, nBytes)

    buffPtr u8* = strings.toPtr(result)
    buff u8[] = slices.fromPtr(buffPtr, nBytes)
    readCnt u64, readErr error = this.readToBuff(buff, nBytes)
    if readErr.nok():
        strings.free(a, result)
        throw readErr
    ..
    buffPtr[readCnt] = 0

    # Compiler warning suppression
    footgun.drop[str](result)
    ret strings.fromPtrNoCopy(buffPtr, readCnt)
..

# Reads into the provided buffer up to nBytes bytes.
# @complexity O(N) for nBytes.
# @param buff destination buffer
# @param nBytes number of bytes to read
# @returns number of bytes read
# @throws invalidArgument when nBytes exceeds the destination length
# @example
#   count := try input.readToBuff(buffer, slices.count(buffer))
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
