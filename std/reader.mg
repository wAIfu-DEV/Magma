mod reader
# Type-erased byte input with convenience methods for exact and allocated reads.

use "std:context"   context
use "std:allocator" alc
use "std:slices"    slices
use "std:strings"   strings
use "std:errors"    errors
use "std:cast"      cast
use "std:future"    future

# Reader interface for pulling bytes into strings or buffers.
# @complexity O(1) wrapper calls; underlying reader decides cost.
pub proto Reader(
    readRaw(buff u8[], nBytes u64) !u64
)

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
    onerror result.free(a)

    buffPtr u8* = strings.toPtr(result)
    buff u8[] = slices.fromPtr(buffPtr, nBytes)
    readCnt u64 = try this.readToBuff(buff, nBytes)
    # SAFETY: strings.alloc reserves the trailing terminator byte and readToBuff
    # returns no more than the supplied nBytes extent.
    unsafe:
        buffPtr[readCnt] = 0
    ..

    if strings.truncate(addrof result, readCnt) == false:
        throw errors.failure("reader produced an invalid byte count")
    ..
    ret move result
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
    readCnt u64 = try this.readRaw(buff, nBytes)
    if readCnt > nBytes:
        throw errors.failure("reader returned more bytes than requested")
    ..
    ret readCnt
..

ReaderReadTask(
    allocator alc.Allocator
    source Reader*
    count u64
)

runReadTask(ctx ReaderReadTask*) !str:
    ret try ctx.source.read(ctx.allocator, ctx.count)
..

Reader.readAsync(ctx context.Ctx, nBytes u64) !$future.Future[str]:
    task := ReaderReadTask(source=this, allocator=ctx.alloc, count=nBytes)
    ret try future.new[str, ReaderReadTask](ctx.alloc, ctx.exec, runReadTask, task)
..
