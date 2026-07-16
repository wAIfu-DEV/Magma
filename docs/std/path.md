# `std/path`

## Example

```magma
name := path.base("one/two.txt")  # "two.txt"
extension := path.extension(name) # ".txt"
absolute := path.isAbsolute("/tmp")
```

Platform-aware lexical path utilities. These functions do not access the filesystem or allocate.

- `pub separator() u8` returns the platform's preferred path separator.
- `pub isSeparator(c u8) bool` recognizes platform path separators.
- `pub isAbsolute(path str) bool` reports whether a path is absolute under platform rules.
- `pub base(path str) str` returns a borrowed view of the final path component.
- `pub extension(path str) str` returns a borrowed view of the final component's extension, including the leading dot, or an empty string.
