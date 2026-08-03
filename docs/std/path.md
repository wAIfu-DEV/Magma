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
all returned strings are owned by the caller.

- `pub separator() u8` returns the platform's preferred path separator.
- `pub isSeparator(c u8) bool` recognizes both slash and backslash.
- `pub isAbsolute(path str) bool` reports whether a path is absolute under platform rules.
- `pub base(a allocator.Allocator, path str) !$str` returns an owned copy of the final path component.
- `pub extension(a allocator.Allocator, path str) !$str` returns an owned extension, including the leading dot, or an empty owned string.
- `pub join(a allocator.Allocator, parts str[]) !$str` joins and normalizes components.
- `pub normalize(a allocator.Allocator, value str) !$str` resolves lexical separators, `.` and resolvable `..` components without touching the filesystem.
- `pub parent(a allocator.Allocator, value str) !$str` returns the lexical parent.
- `pub stem(a allocator.Allocator, value str) !$str` returns the base name without its final extension.
- `pub changeExtension(a allocator.Allocator, value str, newExtension str) !$str` replaces or removes the final extension.

```magma
parts := array str[3]
parts[0] = "build"
parts[1] = "objects"
parts[2] = "main.o"
objectPath := try path.join(a, parts)
defer objectPath.free(a)
```
