mod reader

use "allocator.mg" alc
use "slices.mg"    slices
use "strings.mg"   strings
use "errors.mg"    errors
use "cast.mg"      cast
use "future.mg"    future
use "thread_pool.mg" thread_pool
use "footgun.mg"   footgun

# Reader interface for pulling bytes into strings or buffers.
# O(1) wrapper calls; underlying reader decides cost.
Reader(
    impl ptr,
    fn_read (ptr, u8[], u64) !u64,
)

ReaderReadTask(
    source Reader
    allocator alc.Allocator
    count u64
)

pub new(impl ptr, readFunc (ptr, u8[], u64) !u64) Reader:
    ret Reader(impl=impl, fn_read=readFunc)
..

# Reads up to nBytes and returns a string containing the bytes read.
# Warning: returned string is backed by allocator-owned memory.
# O(N) for nBytes.
# @param a allocator to use
# @param nBytes maximum bytes to read
# @returns string with read bytes
Reader.read(a alc.Allocator, nBytes u64) !$str:
    if nBytes == 0:
        ret try strings.alloc(a, 0)
    ..
    result str = try strings.alloc(a, nBytes)

    buffPtr u8* = strings.toPtr(result)
    buff u8[] = slices.fromPtr(buffPtr, nBytes)
    readCnt u64, readErr error = this.readToBuff(buff, nBytes)
    if errors.code(readErr) != 0:
        strings.free(a, result)
        throw readErr
    ..
    buffPtr[readCnt] = 0

    # Compiler warning suppression
    footgun.drop[str](result)
    ret strings.fromPtrNoCopy(buffPtr, readCnt)
..

runReadTask(task ReaderReadTask*) !$str:
    ret try task.source.read(task.allocator, task.count)
..

# Runs read on the supplied pool. The receiver is copied into private task
# storage, while its underlying implementation must remain valid until await.
Reader.readAsync(pool thread_pool.ThreadPool, a alc.Allocator, nBytes u64) !$future.Future[str]:
    task := ReaderReadTask(source=*this, allocator=a, count=nBytes)
    ret try future.new[str, ReaderReadTask](a, pool, runReadTask, task)
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
