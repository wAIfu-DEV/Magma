# `std/path`

## Example

```magma
a := heap.allocator()
name := try path.base(a, "one/two.txt")
defer name.free(a)
extension := try path.extension(a, name)
defer extension.free(a)
absolute := path.isAbsolute("/tmp")
```

Platform-aware lexical path utilities. They do not access the filesystem;
`base` and `extension` allocate their results.

- `pub separator() u8` returns the platform's preferred path separator.
- `pub isSeparator(c u8) bool` recognizes both slash and backslash.
- `pub isAbsolute(path str) bool` reports whether a path is absolute under platform rules.
- `pub base(a allocator.Allocator, path str) !$str` returns an owned copy of the final path component.
- `pub extension(a allocator.Allocator, path str) !$str` returns an owned extension, including the leading dot, or an empty owned string.
