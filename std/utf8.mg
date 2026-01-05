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

decode(it Utf8Iterator*) !Codepoint:
    if cast.ptou(it.start) == 0:
        throw errors.invalidArgument("Utf8Iterator was not correctly initialized, use utf8.iterateStr")
    ..

    cp Codepoint

    start ptr = it.start
    end   ptr = it.end

    # utf8.decode defined in llvm fragment utf8.ll

    llvm "  %i0 = load ptr, ptr %start\n"
    llvm "  %i1 = load ptr, ptr %end\n"
    llvm "  %c = call i64 @utf8.decode(ptr %i0, ptr %i1)\n"
    llvm "  store i64 %c, ptr %cp\n"
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
    ret try decode(this)
..

Utf8Iterator.next() !Codepoint:
    cp Codepoint = try this.peek()
    this.start = cast.utop(cast.ptou(this.start) + cast.u8to64(cp.width))
    ret cp
..

Utf8Iterator.hasData() bool:
    if cast.ptou(this.start) == 0:
        ret false
    ..
    ret cast.ptou(this.start) < cast.ptou(this.end)
..
