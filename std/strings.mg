mod strings

use "utf8.mg"   utf8
use "errors.mg" errors

# Returns size in bytes of string, for UTF8 strings codepoint (UTF8 character) count may be
# different from byte size.
# @param s input string
# @returns size in bytes of string

pub countBytes(s str) u64:
    llvm "  %l0 = extractvalue %type.str %s, 1\n"
    llvm "  ret i64 %l0\n"
..

pub countCodepoints(s str) !u64:
    cnt u64 = 0

    it utf8.Utf8Iterator = utf8.iterator(s)

    while it.hasData():
        try it.next()
        cnt = cnt + 1
    ..
    ret cnt
..

pub toPtr(s str) u8*:
    llvm "  %l0 = extractvalue %type.str %s, 0\n"
    llvm "  ret ptr %l0\n"
..

pub fromPtr(p ptr, bytesCount u64) str:
    llvm "  %s0 = insertvalue %type.str zeroinitializer, ptr %p, 0\n"
    llvm "  %s1 = insertvalue %type.str %s0, i64 %bytesCount, 1\n"
    llvm "  ret %type.str %s1\n"
..

pub byteAt(s str, idx u64) u8:
    llvm "  %l0 = extractvalue %type.str %s, 0\n"
    llvm "  %ptr = getelementptr inbounds i8, ptr %l0, i64 %idx\n"
    llvm "  %byte = load i8, ptr %ptr\n"
    llvm "  ret i8 %byte\n"
..
