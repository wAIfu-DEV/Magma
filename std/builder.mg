mod builder
# Efficiently constructs owned strings from incrementally appended values.

use "std:allocator" alc
use "std:strings" strings
use "std:memory" mem
use "std:cast" cast
use "std:errors" errors
use "std:footgun" footgun

Segment(
    value str
    owned bool
)

# Accumulates borrowed and owned string segments before producing one owned string.
pub Builder(
    allocator alc.Allocator
    segments ptr
    count u64
    capacity u64
    totalBytes u64
)

# Creates an empty builder using a for segment storage and copied strings.
# @complexity O(1), excluding allocator cost
# @ownership Release with Builder.free.
# @example
#   output := try builder.new(a)
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

# Appends a borrowed segment without copying it.
# @complexity Amortized O(1)
# @ownership s must remain valid until build or reset completes.
# @example
#   try output.appendBorrowed("prefix: ")
Builder.appendBorrowed(s str) !void:
    try this.add(s, false)
..

# Transfers an owned string into the builder.
# @complexity Amortized O(1)
# @ownership The builder releases s during reset or free.
# @example
#   try output.appendOwned(ownedText)
Builder.appendOwned(s $str) !void:
    try this.add(s, true)
    footgun.drop[str](s)
..

# Copies and appends a string, making the segment independent of s.
# @complexity O(N), where N is the string byte length
# @example
#   try output.appendCopy(temporaryText)
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

# Concatenates all segments into a newly allocated owned string.
# Building does not clear the builder or release copied segments.
# @complexity O(N), where N is the total output byte length
# @ownership Release the returned string with the builder's allocator.
# @example
#   text := try output.build()
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

# Returns the byte length of the string that build would produce.
# @complexity O(1)
Builder.byteCount() u64:
    ret this.totalBytes
..

# Reports whether no segments have been appended.
# @complexity O(1)
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

# Releases copied and transferred segments while retaining segment capacity.
# @complexity O(S), where S is the number of segments
# @example
#   try output.reset()
Builder.reset() !void:
    this.releaseCopies()
    this.count = 0
    this.totalBytes = 0
..

# Releases all owned segments and the builder's segment storage.
# @complexity O(S), where S is the number of segments
# @example
#   output.free()
destr Builder.free() void:
    this.releaseCopies()
    this.allocator.free(this.segments)
    this.count = 0
    this.capacity = 0
    this.totalBytes = 0
..
