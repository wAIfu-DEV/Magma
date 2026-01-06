mod utf8

use "errors.mg"  errors
use "strings.mg" strings
use "cast.mg"    cast

Utf8Iterator(
    start ptr,
    end   ptr,
)

Codepoint(
    value u32,
    width u8,
)

decodeOnce(it Utf8Iterator*) !Codepoint:
    if cast.ptou(it.start) == 0 || cast.ptou(it.end) == 0:
        throw errors.invalidArgument("Utf8Iterator was not correctly initialized, use utf8.iterator")
    ..
    cp Codepoint = decodeFirst(it.start, it.end)
    if cp.width == 0:
        throw errors.failure("failed to decode utf8 codepoint")
    ..
    ret cp
..

pub iterator(s str) Utf8Iterator:
    i Utf8Iterator

    p u8* = strings.toPtr(s)
    sLen u64 = strings.countBytes(s)

    i.start = p
    i.end = cast.utop(cast.ptou(p) + sLen)
    ret i
..

Utf8Iterator.peek() !Codepoint:
    ret try decodeOnce(this)
..

Utf8Iterator.next() !Codepoint:
    cp Codepoint = try this.peek()
    this.start = cast.utop(cast.ptou(this.start) + cast.u8to64(cp.width))
    ret cp
..

Utf8Iterator.hasData() bool:
    if cast.ptou(this.start) == 0 || cast.ptou(this.end) == 0:
        ret false
    ..
    ret cast.ptou(this.start) < cast.ptou(this.end)
..

decodeFirst(start u8*, end u8*) Codepoint:
    outCp Codepoint
    outCp.value = 0
    outCp.width = 0

    bytes ptr = cast.utop(cast.ptou(start))
    first u8 = start[0]
    width u8 = 0
    codepoint u32 = 0

    if (first & 128) == 0:
        width = 1
        codepoint = cast.u64to32(cast.u8to64(first))
    elif (first & 224) == 192:
        width = 2
        codepoint = cast.u64to32(cast.u8to64(first & 31))
    elif (first & 240) == 224:
        width = 3
        codepoint = cast.u64to32(cast.u8to64(first & 15))
    elif (first & 248) == 240:
        width = 4
        codepoint = cast.u64to32(cast.u8to64(first & 7))
    else:
        ret outCp
    ..

    if width > 1:
        ptdiff u64 = cast.ptou(end) - cast.ptou(start)
        if ptdiff < cast.u8to64(width):
            ret outCp
        ..
    ..

    cont u8 = 0
    i u64 = 1

    while i < width:
        cont = start[i]

        if (cont & 192) != 128:
            ret outCp
        ..
        codepoint = (codepoint << 6) | cast.u64to32(cast.u8to64(cont & 63))
        i = i + 1
    ..

    if width == 1:
    elif width == 2:
        if codepoint < 128:
            ret outCp
        ..
    elif width == 3:
        if codepoint < 2048:
            ret outCp
        ..
        if codepoint >= 55296 && codepoint <= 57343:
            ret outCp
        ..
    elif width == 4:
        if codepoint < 65536:
            ret outCp
        ..
        if codepoint > 1114111:
            ret outCp
        ..
    else:
        ret outCp
    ..

    outCp.value = codepoint
    outCp.width = width
    ret outCp
..
