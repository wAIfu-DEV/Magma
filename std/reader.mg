mod reader

use "allocator.mg" alc
use "slices.mg"    slices
use "strings.mg"   strings
use "errors.mg"    errors

Reader(
    impl ptr,

    fn_read (ptr, u8[], u64) !u64,
)

Reader.read(a alc.Allocator, nBytes u64) !$str:
    buffPtr u8* = try a.alloc(nBytes + 1)
    buff u8[] = slices.fromPtr(buffPtr, nBytes)
    readCnt u64 = try this.readToBuff(buff, nBytes)
    ret strings.fromPtrNoCopy(buffPtr, readCnt)
..

Reader.readToBuff(buff u8[], nBytes u64) !u64:
    if slices.count(buff) <= nBytes:
        throw errors.errInvalidArgument("would overflow")
    ..
    buff[0] = 0
    readCnt u64 = try this.fn_read(this.impl, buff, nBytes)
    buff[readCnt] = 0
    ret readCnt
..
