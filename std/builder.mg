mod builder

use "allocator.mg" alc
use "strings.mg" strings
use "memory.mg" mem
use "cast.mg" cast
use "errors.mg" errors
use "footgun.mg" footgun

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
    ret Builder(
        allocator=a,
        segments=try a.allocT[Segment](8),
        count=0,
        capacity=8,
        totalBytes=0,
    )
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
    segmentPtr Segment* = cast.reinterpret[Segment](this.segments)
    newSegments Segment* = try this.allocator.reallocT[Segment](segmentPtr, newCapacity)
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
    segment := Segment(value=s, owned=owned)
    segments[this.count] = segment
    this.count = this.count + 1
    this.totalBytes = this.totalBytes + byteCount
..

Builder.appendBorrowed(s str) !void:
    try this.add(s, false)
..

Builder.appendOwned(s $str) !void:
    footgun.drop[str](s)
    try this.add(s, true)
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
    owned str = try strings.copy(this.allocator, s)
    segments Segment* = this.segments
    segment := Segment(value=owned, owned=true)
    segments[this.count] = segment
    this.count = this.count + 1
    this.totalBytes = this.totalBytes + byteCount
..

Builder.build() !$str:
    if this.totalBytes == 0:
        ret try strings.alloc(this.allocator, 0)
    ..
    result str = try strings.alloc(this.allocator, this.totalBytes)
    out u8* = strings.toPtr(result)
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
    ret result
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

destr Builder.free() void:
    this.releaseCopies()
    this.allocator.free(this.segments)
    this.count = 0
    this.capacity = 0
    this.totalBytes = 0
..
