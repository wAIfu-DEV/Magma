# `std/builder`

Builds a string from borrowed or copied segments while avoiding repeated concatenation.

## Types

- `Segment(value str, owned bool)` is an internal segment record. `owned` says whether the builder must free the string.
- `Builder(allocator alc.Allocator, segments ptr, count u64, capacity u64, totalBytes u64)` stores segments and their combined byte count.

## API

- `pub new(a alc.Allocator) !$Builder` creates an empty builder.
- `Builder.append(s str) !void` appends a borrowed string. It must remain valid until the builder is built, reset, or freed.
- `Builder.appendCopy(s str) !void` copies and owns the appended string.
- `Builder.build() !$str` allocates and returns the concatenated string. The caller owns the result.
- `Builder.byteCount() u64` returns the combined byte count.
- `Builder.isEmpty() bool` reports whether there are no segments.
- `Builder.reset() !void` releases owned segment copies and returns to an empty initial state.
- `Builder.free() void` releases owned copies and segment storage.

`ensureCapacity() !void`, `add(s str, owned bool) !void`, and `releaseCopies() void` are internal storage and ownership helpers.
