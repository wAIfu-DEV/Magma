mod builder

use "allocator.mg" alc
use "writer.mg"    writer
use "strings.mg"   strings
use "list.mg"      list
use "memory.mg"    mem
use "cast.mg"      cast
use "errors.mg"    errors

# A lazy string builder that accumulates str segments and concatenates
# them into a single allocation only when build() is called.
#
# Two append modes control memory safety:
#   - append: records the str as-is. O(1), zero allocation.
#     The caller guarantees the source string outlives the builder.
#   - appendCopy: immediately copies the string data into owned memory.
#     O(N) per call, but the segment is safe regardless of source lifetime.
#
# build() performs exactly one allocation sized to the total byte count,
# then writes all segments into it in order.
#
# Warning: call free() after build() to release the segment list and any
# copies made by appendCopy. The string returned by build() is separately
# owned by the caller and must be freed independently.
Builder(
    allocator  alc.Allocator,
    segments   list.List,
    totalBytes u64,
)

# Creates a new empty Builder.
# Allocates the internal segment list. No string data is allocated.
# O(1) aside from the segment list allocation.
# @param a allocator for segment list and any appendCopy data
# @returns new Builder
pub new(a alc.Allocator) !$Builder:
    sb Builder
    sb.allocator  = a
    sb.segments   = try list.new(a, sizeof str)
    sb.totalBytes = 0
    ret sb
..

# Appends a string by reference (borrow).
# The str is recorded as-is; no data is copied.
# O(1). Zero allocation.
# Warning: the caller must ensure the source string remains valid
# until build() is called. Use appendCopy if lifetime is uncertain.
# @param s string to borrow
Builder.append(s str) !void:
    try this.segments.pushRight(this.allocator, addrof s)
    this.totalBytes = this.totalBytes + strings.countBytes(s)
..

# Appends a string by copying its data into owned memory immediately.
# The copy is owned by the builder and freed on free().
# O(N) for byte count.
# @param s string to copy
Builder.appendCopy(s str) !void:
    byteCount u64 = strings.countBytes(s)

    if byteCount == 0:
        ret
    ..
    owned u8* = try this.allocator.alloc(byteCount)
    mem.copy(strings.toPtr(s), owned, byteCount)

    copy str = strings.fromPtrNoCopy(owned, byteCount)
    try this.segments.pushRight(this.allocator, addrof copy)
    this.totalBytes = this.totalBytes + byteCount
..

# Internal write function used to wire Builder as a Writer.
# Appends the segment by borrow into the Builder pointed to by impl.
# O(1).
# sbWrite(impl ptr, s str) !u64:
#     sb Builder* = impl
#     byteCount u64 = strings.countBytes(s)
#     tmp str = s # 2026-6: addrof of arg is invalidfvt_ ftr
#     try sb.segments.pushRight(sb.allocator, addrof tmp)
#     sb.totalBytes = sb.totalBytes + byteCount
#     ret byteCount
# ..

# TODO: writeInt etc write using temporary stack arrays
# this causes data corruption if we hold on to those strings instead of
# writing them directly (which is what this Builder does)
# 
# Returns a Writer that appends borrowed segments into this Builder.
# Use this to compose Builder with existing Writer-based APIs
# (e.g. writer.writeInt64, writer.writeFloat64).
# Warning: strings written through this Writer are borrowed — the data
# must remain valid until build() is called. For stack-allocated formatting
# buffers (as used by writeInt64 / writeFloat64) this is unsafe unless
# build() is called before those stack frames are released.
# Prefer calling build() immediately after any such writes.
# O(1).
# @returns writer interface backed by this Builder
# Builder.writer() writer.Writer:
#    w writer.Writer
#    w.impl = this
#    w.fn_write = sbWrite
#    ret w
#..

# Concatenates all segments into a single newly allocated string.
# O(N) for total byte count.
# @returns owned string containing all appended segments in order
Builder.build() !$str:
    if this.totalBytes == 0:
        ret strings.fromPtrNoCopy(cast.utop(0), 0)
    ..
    out u8* = try this.allocator.alloc(this.totalBytes)
    strs str[] = this.segments.view()
    segCount u64 = this.segments.count()

    writeOffset u64 = 0
    i u64 = 0
    while i < segCount:
        s str = strs[i]
        byteCount u64 = strings.countBytes(s)

        if byteCount > 0:
            dst ptr = cast.utop(cast.ptou(out) + writeOffset)
            mem.copy(strings.toPtr(s), dst, byteCount)
            writeOffset = writeOffset + byteCount
        ..
        i = i + 1
    ..
    ret strings.fromPtrNoCopy(out, this.totalBytes)
..

# Returns the total number of bytes that would be written by build().
# O(1).
Builder.byteCount() u64:
    ret this.totalBytes
..

# Returns true if no segments have been appended.
# O(1).
Builder.isEmpty() bool:
    ret this.totalBytes == 0
..

# Discards all segments and resets the byte count.
# Does not free the segment list allocation itself (kept for reuse).
# O(1).
Builder.reset() !void:
    try this.segments.clearShrink(this.allocator)
    this.totalBytes = 0
..

# Frees all resources owned by this Builder.
# Frees segment list memory. Does NOT free the string returned by build(),
# which is separately owned by the caller.
# O(1).
Builder.free() void:
    this.segments.free(this.allocator)
    this.totalBytes = 0
..
