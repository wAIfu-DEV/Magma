# `std/strconv`

## Example

```magma
a := heap.allocator()
answer := try strconv.parseUint("42")
enabled := try strconv.parseBool("true")
text := try strconv.formatUint(a, answer)
defer strings.free(a, text)
```

Basic string conversions.

- `pub parseUint(s str) !u64` parses a nonempty ASCII decimal unsigned integer. Non-digits and overflow produce an error.
- `pub parseBool(s str) !bool` accepts exactly `"true"` or `"false"`; other input produces an error.
- `pub formatUint(a alc.Allocator, value u64) !$str` returns an owned decimal representation. Free it with the same allocator.
