mod builder
# Efficiently constructs owned strings from incrementally appended values.

use "std:allocator" alc
use "std:strings" strings
use "std:slices" slices
use "std:memory" mem
use "std:cast" cast
use "std:errors" errors
use "std:footgun" footgun
use "std:checked" checked

const FLAG_OWNED u8 = 1
const FLAG_BYTE u8 = 2

Segment(
    value str
    flags u8
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
    newCapacity := try checked.uMul(this.capacity, 2)
    segmentPtr Segment* = cast.reinterpret[Segment](this.segments)
    newSegments Segment* = try this.allocator.reallocT[Segment](segmentPtr, newCapacity)
    this.segments = newSegments
    this.capacity = newCapacity
..

Builder.add(s str, owned bool) !void:
    byteCount u64 = s.countBytes()
    newTotal := try checked.uAdd(this.totalBytes, byteCount)
    try this.ensureCapacity()
    segments Segment* = this.segments
    ownedBits u8 = 0

    if owned:
        ownedBits = FLAG_OWNED
    ..
    segment := Segment(value=s, flags=ownedBits)
    segments[this.count] = segment
    this.count = this.count + 1
    this.totalBytes = newTotal
..

Builder.addByte(b u8) !void:
    try this.ensureCapacity()
    segments Segment* = this.segments

    # write the byte directly within the pointer
    val ptr = none
    p u8* = cast.reinterpret[u8](addrof val)
    *p = b

    segment := Segment(value=strings.fromPtrNoCopy(val, 0), flags=FLAG_BYTE)
    segments[this.count] = segment
    this.count = this.count + 1
    this.totalBytes = this.totalBytes + 1
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
    byteCount := s.countBytes()
    if byteCount == 0:
        ret
    ..
    newTotal := try checked.uAdd(this.totalBytes, byteCount)
    # Reserve the segment first so allocation of the owned bytes is the final
    # fallible operation before committing the segment.
    try this.ensureCapacity()
    owned str = try strings.copy(this.allocator, s)
    segments Segment* = this.segments
    segment := Segment(value=owned, flags=FLAG_OWNED)
    segments[this.count] = segment
    this.count = this.count + 1
    this.totalBytes = newTotal
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

    byteBuff := array u8[2]
    byteBuff[1] = 0

    while i < this.count:
        seg Segment = segments[i]
        s := seg.value

        if (seg.flags & FLAG_BYTE) != 0:
            # unpack byte
            p := strings.toPtr(s)
            byteBuff[0] = *(cast.reinterpret[u8](addrof p))
            s = strings.fromPtrNoCopy(slices.toPtr(byteBuff), 1)
        ..

        byteCount := s.countBytes()
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
        if (segments[i].flags & FLAG_OWNED) != 0:
            segments[i].value.free(this.allocator)
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
