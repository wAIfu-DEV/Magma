mod strings

use "utf8.mg"      utf8
use "allocator.mg" alc
use "memory.mg"    mem

# Returns size in bytes of string, for UTF8 strings codepoint (UTF8 character) count may be
# different from byte size.
# O(1) regardless of size.
# @param s input string
# @returns size in bytes of string
pub countBytes(s str) u64:
    llvm "  %l0 = extractvalue %type.str %s, 1\n"
    llvm "  ret i64 %l0\n"
..

# Returns size in bytes of string, for UTF8 strings codepoint (UTF8 character) count may be
# different from byte size.
# O(N) depending on string size.
# @param s input string
# @returns size in bytes of string
pub countCodepoints(s str) !u64:
    cnt u64 = 0
    it utf8.Utf8Iterator = utf8.iterator(s)

    while it.hasData():
        try it.next()
        cnt = cnt + 1
    ..
    ret cnt
..

# Returns the pointer to the underlying data of the string (u8 array)
# Note that this array is not null-terminated, see toCstr for a null-terminated
# version.
# @param s input string
pub toPtr(s str) u8*:
    llvm "  %l0 = extractvalue %type.str %s, 0\n"
    llvm "  ret ptr %l0\n"
..

# Returns a str from a pointer and a length in bytes.
# Warning: This ties the lifetime of the input pointer to the output magma str,
# if the input pointer is deallocated or falls out of scope, it WILL result in
# invalid and unsafe reads from memory when using the output str, leading to
# SEGFAULTs in the best cases or security vulnerabilities in the worst.
# Prefer using fromPtr when unsure about lifetimes.
# O(1) complexity
# @param s input string
pub fromPtrNoCopy(p ptr, bytesCount u64) str:
    llvm "  %s0 = insertvalue %type.str zeroinitializer, ptr %p, 0\n"
    llvm "  %s1 = insertvalue %type.str %s0, i64 %bytesCount, 1\n"
    llvm "  ret %type.str %s1\n"
..

# Returns a str from a pointer and a length in bytes.
# This copies the contents of p into a newly allocated str
# O(N) depending on byte count
# @param s input string
pub fromPtr(a alc.Allocator, p ptr, byteCount u64) $str:
    inData u8* = p
    strData u8* = a.alloc(byteCount)

    i u64 = 0
    while i < byteCount:
        strData[i] = inData[i]
        i = i + 1
    ..
    ret fromPtrNoCopy(strData, byteCount)
..

# Returns the byte at position idx in string, prefer utf8.Utf8Iter for UTF8-aware
# string traversal.
# O(1) regardless of size.
# @param s input string
# @returns size in bytes of string
pub byteAt(s str, idx u64) u8:
    llvm "  %l0 = extractvalue %type.str %s, 0\n"
    llvm "  %ptr = getelementptr inbounds i8, ptr %l0, i64 %idx\n"
    llvm "  %byte = load i8, ptr %ptr\n"
    llvm "  ret i8 %byte\n"
..

# Makes a C string (pointer to null-terminated char array) from a provided str.
# This creates a allocated copy, do not use unless directly interfacing with APIs or
# protocols expecting C-style strings.
# Repeated use of this function may cause performance degradation, prefer using it
# once rather than many times.
# O(N) depending on string size.
# @param a allocator to use
# @param s string to copy
# @returns a null-terminated c-style string
pub toCstr(a alc.Allocator, s str) $u8*:
    size u64 = countBytes(s)
    cStr u8* = a.alloc(size + 1)

    i u64 = 0
    while i < size:
        cStr[i] = byteAt(s, i)
        i = i + 1
    ..
    cStr[size] = 0
    ret cStr
..

# Returns the length of a C-style string.
# Length is obtained through traversal of the string up to null-terminator,
# repeated use may cause performance degradation, especially with longer strings.
# O(N) depending on string size.
# @param cstr null-terminated C-style string
pub cStrLen(cstr u8*) u64:
    len u64 = 0
    while cstr[len] != 0:
        len = len + 1
    ..
    ret len
..

# Creates a magma-style str from a null-terminated C-string.
# This function does not copy the input C-string, instead it borrows it.
# Warning: This ties the lifetime of the C-string to the output magma str,
# if the C-string is deallocated or falls out of scope, it WILL result in
# invalid and unsafe reads from memory when using the output str, leading to
# SEGFAULTs in the best cases or security vulnerabilities in the worst.
# Prefer using fromCstr when unsure about the input C-string lifetime.
# O(N) depending on string size. (due to length traversal)
# Prefer using fromPtr which is O(1) if you already know the length of the C-string.
# @param cstr null-terminated C-string
# @returns magma-style str
pub fromCstrNoCopy(cstr u8*) str:
    len u64 = cStrLen(cstr)
    ret fromPtrNoCopy(cstr, len)
..

# Creates a magma-style str from a null-terminated C-string.
# This function copies the C-string to the new str.
# @param cstr null-terminated C-string
# @returns magma-style str
pub fromCstr(a alc.Allocator, cstr u8*) $str:
    size u64 = cStrLen(cstr)
    strData u8* = a.alloc(size)

    i u64 = 0
    while i < size:
        strData[i] = cstr[i]
        i = i + 1
    ..
    ret fromPtrNoCopy(strData, size)
..

# Compares both strings and returns true if they are strictly equal in size and
# content on the byte level.
# @param a first string
# @param b second string
# @returns true if both strings are equal
pub compare(a str, b str) bool:
    aLen u64 = countBytes(a)
    bLen u64 = countBytes(b)

    if aLen != bLen:
        ret false
    ..
    ret mem.compare( toPtr(a), toPtr(b), aLen)
..
