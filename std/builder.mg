mod builder

use "allocator.mg" alc
use "strings.mg" strings
use "memory.mg" mem
use "cast.mg" cast
use "errors.mg" errors

Segment(
    value str
    owned bool
)

Builder(
    allocator alc.Allocator
    segments ptr
    count u64
    capacity u64
    totalBytes u64
)

pub new(a alc.Allocator) !$Builder:
    sb Builder
    sb.allocator = a
    sb.capacity = 8
    sb.segments = try a.alloc(sb.capacity * sizeof Segment)
    ret sb
..

Builder.ensureCapacity() !void:
    if this.count < this.capacity:
        ret
    ..
    maxU64 u64 = 0 - 1
    if this.capacity > maxU64 / 2:
        throw errors.wouldOverflow("builder capacity overflow")
    ..
    newCapacity u64 = this.capacity * 2
    if sizeof Segment != 0 && newCapacity > maxU64 / sizeof Segment:
        throw errors.wouldOverflow("builder allocation size overflow")
    ..
    newSegments Segment* = try this.allocator.realloc(this.segments, newCapacity * sizeof Segment)
    this.segments = newSegments
    this.capacity = newCapacity
..

Builder.add(s str, owned bool) !void:
    byteCount u64 = strings.countBytes(s)
    maxU64 u64 = 0 - 1
    if byteCount > maxU64 - this.totalBytes:
        throw errors.wouldOverflow("builder byte count overflow")
    ..
    try this.ensureCapacity()
    segments Segment* = this.segments
    segment Segment
    segment.value = s
    segment.owned = owned
    segments[this.count] = segment
    this.count = this.count + 1
    this.totalBytes = this.totalBytes + byteCount
..

Builder.append(s str) !void:
    try this.add(s, false)
..

Builder.appendCopy(s str) !void:
    byteCount := strings.countBytes(s)
    if byteCount == 0:
        ret
    ..
    maxU64 u64 = 0 - 1
    if byteCount > maxU64 - this.totalBytes:
        throw errors.wouldOverflow("builder byte count overflow")
    ..
    # Reserve the segment first so allocation of the owned bytes is the final
    # fallible operation before committing the segment.
    try this.ensureCapacity()
    owned u8* = try this.allocator.alloc(byteCount)
    mem.copy(strings.toPtr(s), owned, byteCount)
    segments Segment* = this.segments
    segment Segment
    segment.value = strings.fromPtrNoCopy(owned, byteCount)
    segment.owned = true
    segments[this.count] = segment
    this.count = this.count + 1
    this.totalBytes = this.totalBytes + byteCount
..

Builder.build() !$str:
    if this.totalBytes == 0:
        ret strings.fromPtrNoCopy(cast.utop(0), 0)
    ..
    out u8* = try this.allocator.alloc(this.totalBytes)
    segments Segment* = this.segments
    offset u64 = 0
    i u64 = 0
    while i < this.count:
        s := segments[i].value
        byteCount := strings.countBytes(s)
        mem.copy(strings.toPtr(s), cast.utop(cast.ptou(out) + offset), byteCount)
        offset = offset + byteCount
        i = i + 1
    ..
    ret strings.fromPtrNoCopy(out, this.totalBytes)
..

Builder.byteCount() u64:
    ret this.totalBytes
..

Builder.isEmpty() bool:
    ret this.count == 0
..

Builder.releaseCopies() void:
    segments Segment* = this.segments
    i u64 = 0
    while i < this.count:
        if segments[i].owned:
            strings.free(this.allocator, segments[i].value)
        ..
        i = i + 1
    ..
..

Builder.reset() !void:
    this.releaseCopies()
    this.count = 0
    this.totalBytes = 0
..

Builder.free() void:
    this.releaseCopies()
    this.allocator.free(this.segments)
    this.count = 0
    this.capacity = 0
    this.totalBytes = 0
..
