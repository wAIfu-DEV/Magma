mod strings
# String allocation, conversion, comparison, searching, splitting, and iteration.

use "std:allocator" alc
use "std:memory"    mem
use "std:cast"      cast
use "std:errors"    err
use "std:footgun"   footgun
use "std:pair"      pair

const gl_nullTerm u8 = 0 

# Returns the pointer to the underlying data of the string (u8 array).
# Allocating string APIs provide a null byte at s.countBytes(); borrowed views
# are not necessarily terminated at their logical end.
# @param s input string
# @complexity O(1)
# @returns borrowed pointer to the first byte
# @example
#   pointer := strings.toPtr(text)
pub toPtr(s str) u8*:
    llvm "  %l0 = extractvalue %type.str %s, 0\n"
    llvm "  ret ptr %l0\n"
..

# Allocates an owned, uninitialized string with size bytes plus a null terminator.
# @complexity O(1), excluding allocator cost
# @ownership Release with str.free using the same allocator.
# @example
#   text := try strings.alloc(a, 32)
pub alloc(a alc.Allocator, size u64) !$str:
    if size == 0 - 1:
        throw err.wouldOverflow("string allocation size overflow")
    ..
    p u8* = try a.alloc(size + 1) # Zero terminated
    p[size] = 0
    ret fromPtrNoCopy(p, size)
..

# Allocates an owned string and initializes every byte to fill.
# @complexity O(N)
# @ownership Release with str.free using the same allocator.
# @example
#   padding := try strings.allocFill(a, 8, 32)
pub allocFill(a alc.Allocator, size u64, fill u8) !$str:
    if size == 0 - 1:
        throw err.wouldOverflow("string allocation size overflow")
    ..
    p u8* = try a.alloc(size + 1) # Zero terminated

    for i u64 = 0 to size:
        p[i] = fill
    ..

    p[size] = 0
    ret fromPtrNoCopy(p, size)
..

# Returns a str from a pointer and a length in bytes.
# @warning This ties the lifetime of the input pointer to the output magma str,
# if the input pointer is deallocated or falls out of scope, it WILL result in
# invalid and unsafe reads from memory when using the output str, leading to
# SEGFAULTs in the best cases or security vulnerabilities in the worst.
# Prefer using fromPtr when unsure about lifetimes.
# @complexity O(1)
# @param p pointer to the first byte
# @param bytesCount number of bytes in the view
# @returns borrowed string view over p
# @example
#   view := strings.fromPtrNoCopy(pointer, byteCount)
pub fromPtrNoCopy(p ptr, bytesCount u64) str:
    llvm "  %s0 = insertvalue %type.str zeroinitializer, ptr %p, 0\n"
    llvm "  %s1 = insertvalue %type.str %s0, i64 %bytesCount, 1\n"
    llvm "  ret %type.str %s1\n"
..

# Returns a str from a pointer and a length in bytes.
# This copies the contents of p into a newly allocated str
# @complexity O(N) depending on byte count
# @param a allocator used for the copy
# @param p pointer to the first byte
# @param byteCount number of bytes to copy
# @returns independently owned copy of byteCount bytes
# @example
#   text := try strings.fromPtr(a, pointer, byteCount)
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

    for i u64 = 0 to byteCount:
        strData[i] = inData[i]
    ..

    strData[byteCount] = 0
    ret fromPtrNoCopy(strData, byteCount)
..

# Allocates an independent copy of a string.
# @complexity O(N)
# @ownership Release the returned string with the same allocator.
# @example
#   owned := try strings.copy(a, borrowed)
pub copy(a alc.Allocator, s str) !$str:
    byteCount u64 = s.countBytes()
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

    for i u64 = 0 to byteCount:
        strData[i] = inData[i]
    ..

    strData[byteCount] = 0
    ret fromPtrNoCopy(strData, byteCount)
..

# Returns an owned ASCII-lowercase copy of s. Bytes outside A-Z are unchanged.
# @complexity O(N)
# @ownership Release the returned string with the same allocator.
# @example
#   lower := try strings.toLower(a, "Hello")
pub toLower(a alc.Allocator, s str) !$str:
    result $str = try copy(a, s)
    data u8* = toPtr(result)
    for i u64 = 0 to result.countBytes():
        if data[i] >= 65 && data[i] <= 90:
            data[i] = data[i] + 32
        ..
    ..
    ret result
..

# Returns an owned ASCII-uppercase copy of s. Bytes outside a-z are unchanged.
# @complexity O(N)
# @ownership Release the returned string with the same allocator.
# @example
#   upper := try strings.toUpper(a, "Hello")
pub toUpper(a alc.Allocator, s str) !$str:
    result $str = try copy(a, s)
    data u8* = toPtr(result)
    for i u64 = 0 to result.countBytes():
        if data[i] >= 97 && data[i] <= 122:
            data[i] = data[i] - 32
        ..
    ..
    ret result
..

# Returns the byte at position idx in string, prefer utf8.Utf8Iter for UTF8-aware
# string traversal.
# @complexity O(1) regardless of size.
# @param s input string
# @param idx zero-based byte index
# @returns byte at idx
# @example
#   firstByte := strings.byteAt(text, 0)
pub byteAt(s str, idx u64) u8:
    llvm "  %l0 = extractvalue %type.str %s, 0\n"
    llvm "  %ptr = getelementptr inbounds i8, ptr %l0, i64 %idx\n"
    llvm "  %byte = load i8, ptr %ptr\n"
    llvm "  ret i8 %byte\n"
..

# Copies the provided string into a null-terminated C string.
# @complexity O(N) depending on string size
# @param a allocator
# @param s string to copy
# @returns a null-terminated c-style string
# @example
#   cText := try strings.toCstr(a, text)
pub toCstr(a alc.Allocator, s str) !$u8*:
    size u64 = s.countBytes()
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

    for i u64 = 0 to size:
        np[i] = p[i]
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
# @complexity O(1)
# @param s string to copy
# @returns a null-terminated c-style string
# @example
#   cText := strings.toCstrNoCopy(ownedText)
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
# @complexity O(N) depending on string size.
# @param cstr null-terminated C-style string
# @example
#   byteCount := strings.cStrLen(cText)
pub cStrLen(cstr u8*) u64:
    len u64 = 0
    loop cstr[len] != 0:
        len = len + 1
    ..
    ret len
..

# Creates a magma-style str from a null-terminated C-string.
# This function does not copy the input C-string, instead it borrows it.
# @warning This ties the lifetime of the C-string to the output magma str,
# if the C-string is deallocated or falls out of scope, it WILL result in
# invalid and unsafe reads from memory when using the output str, leading to
# SEGFAULTs in the best cases or security vulnerabilities in the worst.
# Prefer using fromCstr when unsure about the input C-string lifetime.
# @complexity O(N) depending on string size. (due to length traversal)
# Prefer using fromPtr which is O(1) if you already know the length of the C-string.
# @param cstr null-terminated C-string
# @returns magma-style str
# @example
#   view := strings.fromCstrNoCopy(cText)
pub fromCstrNoCopy(cstr u8*) str:
    len u64 = cStrLen(cstr)
    ret fromPtrNoCopy(cstr, len)
..

# Creates a magma-style str from a null-terminated C-string.
# This function copies the C-string to the new str.
# @param cstr null-terminated C-string
# @returns magma-style str
# @complexity O(N)
# @ownership Release the returned string with the supplied allocator.
# @example
#   text := try strings.fromCstr(a, cText)
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

    for i u64 = 0 to size:
        strData[i] = cstr[i]
    ..
    strData[size] = 0
    ret fromPtrNoCopy(strData, size)
..

# Compares both strings and returns true if they are strictly equal in size and
# content on the byte level.
# @param a first string
# @param b second string
# @returns true if both strings are equal
# @complexity O(N)
# @example
#   same := strings.compare("magma", candidate)
pub compare(a str, b str) bool:
    aLen u64 = a.countBytes()

    if aLen != b.countBytes():
        ret false
    ..
    ret mem.compare(toPtr(a), toPtr(b), aLen)
..

# Returns the first byte index containing value.
# @complexity O(N). Returns outOfBounds when value is absent.
# @example
#   index := try strings.findByte(text, 10)
pub findByte(s str, value u8) !u64:
    size := s.countBytes()
    data := toPtr(s)
    for index u64 = 0 to size:
        if data[index] == value:
            ret index
        ..
    ..
    throw err.outOfBounds("byte was not found in string")
..

matchesAt(source str, needle str, offset u64) bool:
    sourceSize := source.countBytes()
    needleSize := needle.countBytes()
    if offset > sourceSize || needleSize > sourceSize - offset:
        ret false
    ..
    sourceData := toPtr(source)
    needleData := toPtr(needle)
    for index u64 = 0 to needleSize:
        if sourceData[offset + index] != needleData[index]:
            ret false
        ..
    ..
    ret true
..

# Returns the first byte index at which needle occurs.
# @complexity O(N*M). An empty needle is found at index zero.
# @example
#   index := try strings.find(text, "needle")
pub find(s str, needle str) !u64:
    sourceSize := s.countBytes()
    needleSize := needle.countBytes()
    if needleSize == 0:
        ret 0
    ..
    if needleSize > sourceSize:
        throw err.outOfBounds("string was not found")
    ..
    limit := sourceSize - needleSize
    index u64 = 0
    loop index <= limit:
        if matchesAt(s, needle, index):
            ret index
        ..
        index = index + 1
    ..
    throw err.outOfBounds("string was not found")
..

# Allocates an independent substring spanning [start, end).
# @complexity O(end - start)
# @throws outOfBounds when the requested range is invalid
# @example
#   part := try strings.substring(a, text, 0, 5)
pub substring(a alc.Allocator, s str, start u64, end u64) !$str:
    size := s.countBytes()
    if start > end || end > size:
        throw err.outOfBounds("substring bounds are invalid")
    ..
    data := toPtr(s)
    ret try fromPtr(a, cast.utop(cast.ptou(data) + start), end - start)
..

isTrimByte(value u8) bool:
    ret value == 32 || value == 9 || value == 10 || value == 11 || value == 12 || value == 13
..

# Allocates a copy without leading or trailing ASCII whitespace.
# @complexity O(N)
# @example
#   clean := try strings.trim(a, "  magma  ")
pub trim(a alc.Allocator, s str) !$str:
    start u64 = 0
    end := s.countBytes()
    data := toPtr(s)
    loop start < end && isTrimByte(data[start]):
        start = start + 1
    ..
    loop end > start && isTrimByte(data[end - 1]):
        end = end - 1
    ..
    ret try substring(a, s, start, end)
..

# Allocates s without prefix when it starts with prefix, otherwise copies s.
# @complexity O(N)
# @example
#   value := try strings.trimPrefix(a, text, "prefix-")
pub trimPrefix(a alc.Allocator, s str, prefix str) !$str:
    prefixSize := prefix.countBytes()
    if matchesAt(s, prefix, 0):
        ret try substring(a, s, prefixSize, s.countBytes())
    ..
    ret try copy(a, s)
..

# Allocates s without suffix when it ends with suffix, otherwise copies s.
# @complexity O(N)
# @example
#   value := try strings.trimSuffix(a, text, ".mg")
pub trimSuffix(a alc.Allocator, s str, suffix str) !$str:
    sourceSize := s.countBytes()
    suffixSize := suffix.countBytes()
    if suffixSize <= sourceSize && matchesAt(s, suffix, sourceSize - suffixSize):
        ret try substring(a, s, 0, sourceSize - suffixSize)
    ..
    ret try copy(a, s)
..

# Owning eager split result. Both the pointer table and every item are owned.
pub Split(
    items str*
    size u64
    allocator alc.Allocator
)

# Returns the number of split parts.
# @complexity O(1)
Split.count() u64:
    ret this.size
..

# Returns a borrowed view of a split part.
# @complexity O(1)
# @throws outOfBounds when index is not a valid part index
# @example
#   first := try parts.get(0)
Split.get(index u64) !str:
    if index >= this.size:
        throw err.outOfBounds("split index is out of bounds")
    ..
    ret this.items[index]
..

# Releases every owned part and the pointer table.
# @complexity O(N), where N is the number of parts
destr Split.free() void:
    for index u64 = 0 to this.size:
        this.items[index].free(this.allocator)
    ..
    if this.items != none:
        this.allocator.free(this.items)
    ..
    this.items = none
    this.size = 0
..

countParts(s str, separator str) !u64:
    separatorSize := separator.countBytes()
    if separatorSize == 0:
        throw err.invalidArgument("split separator cannot be empty")
    ..
    sourceSize := s.countBytes()
    count u64 = 1
    position u64 = 0
    loop position + separatorSize <= sourceSize:
        if matchesAt(s, separator, position):
            count = count + 1
            position = position + separatorSize
        else:
            position = position + 1
        ..
    ..
    ret count
..

# Eagerly splits s and allocates every item independently.
# @complexity O(N + K), where K is the number of produced parts
# @ownership The returned Split owns its table and every part.
# @example
#   parts := try strings.split(a, "a,b,c", ",")
pub split(a alc.Allocator, s str, separator str) !$Split:
    partCount := try countParts(s, separator)
    maxU64 u64 = 0 - 1
    if partCount > maxU64 / sizeof str:
        throw err.wouldOverflow("split result is too large")
    ..
    items str* = try a.allocT[str](partCount)
    separatorSize := separator.countBytes()
    sourceSize := s.countBytes()
    partStart u64 = 0
    position u64 = 0
    made u64 = 0
    onerror:
        for cleanup u64 = 0 to made:
            items[cleanup].free(a)
        ..
        a.free(items)
    ..

    loop position + separatorSize <= sourceSize:
        if matchesAt(s, separator, position):
            item $str = try substring(a, s, partStart, position)
            items[made] = item
            made = made + 1
            position = position + separatorSize
            partStart = position
        else:
            position = position + 1
        ..
    ..

    last $str = try substring(a, s, partStart, sourceSize)
    items[made] = last
    ret Split(items=items, size=partCount, allocator=a)
..

# Owning lazy splitter. Source and separator are copied at construction, and
# each next call returns a new independently owned string.
pub SplitIterator(
    source str
    separator str
    position u64
    finished bool
    allocator alc.Allocator
)

# Creates a lazy owning splitter that produces one allocated item at a time.
# @complexity O(N) to initialize; each next call scans to the following separator
# @ownership Free the iterator and every string returned by next.
# @example
#   iterator := try strings.splitIter(a, "a,b,c", ",")
pub splitIter(a alc.Allocator, s str, separator str) !$SplitIterator:
    if separator.countBytes() == 0:
        throw err.invalidArgument("split separator cannot be empty")
    ..
    sourceCopy := try copy(a, s)
    onerror sourceCopy.free(a)
    separatorCopy $str = try copy(a, separator)
    ret SplitIterator(source=sourceCopy, separator=separatorCopy, position=0, finished=false, allocator=a)
..

# Reports whether another part remains.
# @complexity O(1)
SplitIterator.hasData() bool:
    ret this.finished == false
..

# Allocates and returns the next split part.
# @complexity O(P), where P is the returned part length
# @throws outOfBounds when no part remains
# @ownership The caller owns the returned string.
SplitIterator.next() !$str:
    if this.finished:
        throw err.outOfBounds("split iterator is exhausted")
    ..
    sourceSize := this.source.countBytes()
    separatorSize := this.separator.countBytes()
    start := this.position
    position := start
    loop position + separatorSize <= sourceSize:
        if matchesAt(this.source, this.separator, position):
            this.position = position + separatorSize
            ret try substring(this.allocator, this.source, start, position)
        ..
        position = position + 1
    ..
    this.finished = true
    this.position = sourceSize
    ret try substring(this.allocator, this.source, start, sourceSize)
..

# Releases the iterator's copied source and separator strings.
# @complexity O(1), excluding allocator cost
destr SplitIterator.free() void:
    this.source.free(this.allocator)
    this.separator.free(this.allocator)
..

# Splits at the first separator and allocates both halves independently.
# @complexity O(N)
# @ownership Both returned strings are independently owned.
# @example
#   pair := try strings.splitOnce(a, "name=value", "=")
pub splitOnce(a alc.Allocator, s str, separator str) !$pair.Pair[str, str]:
    if separator.countBytes() == 0:
        throw err.invalidArgument("split separator cannot be empty")
    ..
    position := try find(s, separator)
    first := try substring(a, s, 0, position)
    onerror first.free(a)
    secondStart := position + separator.countBytes()
    second $str = try substring(a, s, secondStart, s.countBytes())
    result := pair.new[str, str](first, second)
    footgun.drop[str](first)
    footgun.drop[str](second)
    ret result
..
