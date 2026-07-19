mod strings

use "allocator.mg" alc
use "memory.mg"    mem
use "cast.mg"      cast
use "errors.mg"    err

const gl_nullTerm u8 = 0 

# Returns size in bytes of string, for UTF8 strings codepoint (UTF8 character) count may be
# different from byte size.
# O(1) regardless of size.
# @param s input string
# @returns size in bytes of string
pub countBytes(s str) u64:
    llvm "  %l0 = extractvalue %type.str %s, 1\n"
    llvm "  ret i64 %l0\n"
..

# Returns the pointer to the underlying data of the string (u8 array).
# Allocating string APIs provide a null byte at countBytes(s); borrowed views
# are not necessarily terminated at their logical end.
# @param s input string
pub toPtr(s str) u8*:
    llvm "  %l0 = extractvalue %type.str %s, 0\n"
    llvm "  ret ptr %l0\n"
..

pub alloc(a alc.Allocator, size u64) !$str:
    if size == 0 - 1:
        throw err.wouldOverflow("string allocation size overflow")
    ..
    p u8* = try a.alloc(size + 1) # Zero terminated
    p[size] = 0
    ret fromPtrNoCopy(p, size)
..

pub allocFill(a alc.Allocator, size u64, fill u8) !$str:
    if size == 0 - 1:
        throw err.wouldOverflow("string allocation size overflow")
    ..
    p u8* = try a.alloc(size + 1) # Zero terminated

    i u64 = 0
    while i < size:
        p[i] = fill
        i = i + 1
    ..

    p[size] = 0
    ret fromPtrNoCopy(p, size)
..

# Frees a allocated string using the provided allocator.
# Only use if the string is a owned $str from a function taking an Allocator
# as parameter.
# Allocator should be the exact same that the string was allocated with.
# @param a allocator
# @param s allocated slice
pub free(a alc.Allocator, s $str) void:
    s.free(a)
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
pub fromPtr(a alc.Allocator, p ptr, byteCount u64) !$str:
    if byteCount == 0:
        nt u8* = try a.alloc(1)
        *nt = 0
        ret fromPtrNoCopy(nt, 0)
    ..

    # cap size to 0 in case of impossibly large string size (9 exabytes in this case)
    # this should prevent classes of attacks using size overflow as vector
    max u64 = 0 - 1
    if byteCount > (max / 2):
        throw err.wouldOverflow("string too large")
    ..

    inData u8* = p
    strData u8* = try a.alloc(byteCount + 1) # Zero terminated

    i u64 = 0
    while i < byteCount:
        strData[i] = inData[i]
        i = i + 1
    ..

    strData[byteCount] = 0
    ret fromPtrNoCopy(strData, byteCount)
..

pub copy(a alc.Allocator, s str) !$str:
    byteCount u64 = countBytes(s)
    if byteCount == 0 - 1:
        throw err.wouldOverflow("string allocation size overflow")
    ..
    if byteCount == 0:
        nt u8* = try a.alloc(1)
        *nt = 0
        ret fromPtrNoCopy(nt, 0)
    ..
    inData u8* = toPtr(s)
    strData u8* = try a.alloc(byteCount + 1) # Zero terminated

    i u64 = 0
    while i < byteCount:
        strData[i] = inData[i]
        i = i + 1
    ..

    strData[byteCount] = 0
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

# Copies the provided string into a null-terminated C string.
# O(N) depending on string size
# @param a allocator
# @param s string to copy
# @returns a null-terminated c-style string
pub toCstr(a alc.Allocator, s str) !$u8*:
    size u64 = countBytes(s)
    if size == 0 - 1:
        throw err.wouldOverflow("C string allocation size overflow")
    ..

    if size == 0:
        nt u8* = try a.alloc(1)
        *nt = 0
        ret nt
    ..
    p u8* = toPtr(s)
    np u8* = try a.alloc(size + 1)

    i u64 = 0
    while i < size:
        np[i] = p[i]
        i = i + 1
    ..
    np[size] = 0
    ret np
..

# Returns the underlying string pointer without reading or copying its data.
# The caller must guarantee that the string has a null byte immediately after
# its logical data. Owned strings returned by allocating string APIs meet this
# precondition; arbitrary borrowed strings and substring views may not. This
# cannot be checked safely here because the byte after a borrowed view may not
# be readable memory.
# O(1)
# @param s string to copy
# @returns a null-terminated c-style string
pub toCstrNoCopy(s str) u8*:
    p u8* = toPtr(s)

    if p == none:
        ret addrof gl_nullTerm
    ..

    ret p
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
pub fromCstr(a alc.Allocator, cstr u8*) !$str:
    size u64 = cStrLen(cstr)
    if size == 0 - 1:
        throw err.wouldOverflow("string allocation size overflow")
    ..
    if size == 0:
        nt u8* = try a.alloc(1)
        *nt = 0
        ret fromPtrNoCopy(nt, 0)
    ..
    strData u8* = try a.alloc(size + 1)

    i u64 = 0
    while i < size:
        strData[i] = cstr[i]
        i = i + 1
    ..
    strData[size] = 0
    ret fromPtrNoCopy(strData, size)
..

# Compares both strings and returns true if they are strictly equal in size and
# content on the byte level.
# @param a first string
# @param b second string
# @returns true if both strings are equal
pub compare(a str, b str) bool:
    aLen u64 = countBytes(a)

    if aLen != countBytes(b):
        ret false
    ..
    ret mem.compare(toPtr(a), toPtr(b), aLen)
..
