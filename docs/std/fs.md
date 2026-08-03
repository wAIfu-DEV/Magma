# `std/fs`

## Example

```magma
a := heap.allocator()
try fs.writeFile(a, "message.txt", "hello")
contents := try fs.readFile(a, "message.txt")
defer contents.free(a)
```

Whole-file, directory, metadata, traversal, and path-resolution operations.

- `pub readFile(a alc.Allocator, path str) !$str` opens and reads the complete file into an owned string, then closes the file. The caller frees the result with the same allocator.
- `pub writeFile(a alc.Allocator, path str, contents str) !void` creates or truncates a file, writes all `contents`, and closes it.
- `pub removeFile(a alc.Allocator, path str) !void` removes a file.
- `pub walk(a alc.Allocator, root str, visit (str, bool) !void) !void` walks a
  directory tree and calls `visit(path, isDirectory)` for each entry.

Both functions propagate allocation, open, I/O, and close errors.

## Directory iteration

```magma
directory := try fs.openDir(a, "assets")
defer directory.close()
entries := directory.iterator()
while entries.hasData():
    entry := try entries.next()
    name := entry.name()
..
```

`Entry` is a borrowed view whose name remains valid until the iterator advances
or the directory closes. `Dir` owns its native handle and current entry buffer
and must be closed. Its generic iterator borrows the `Dir`, which must remain
alive and open throughout iteration.

## Metadata and walking

- `metadata` follows symbolic links; `linkMetadata` inspects the link itself.
- `Metadata` exposes `kind`, `size`, `permissions`, and Unix modification seconds.
- `FileKind` wraps a kind value. `file()`, `directory()`, `symlink()`, and
  `other()` construct values corresponding to `KIND_FILE`, `KIND_DIR`,
  `KIND_SYMLINK`, and `KIND_OTHER`; `equal` compares kinds.
- `setPermissions` applies portable readable, writable, and executable bits.
- `walkDefault` performs a metadata-aware depth-first traversal without following links.
- `walkWithOptions` accepts `WalkOptions(followLinks=..., includeRoot=...)`.
- The original `walk(a, root, visit(str, bool))` remains as a compatibility API.

## Mutation and system paths

- `makeDir` creates one directory and `makeDirs` creates missing parents.
- `removeDir` removes an empty directory; `removeTree` recursively removes a tree without following directory links.
- `rename`, `replace`, and `copyFile` provide distinct move, atomic replacement, and copy operations.
- `currentDir`, `setCurrentDir`, `temporaryDir`, and `canonicalize` provide owned system paths.

```magma
try fs.makeDirs(a, "cache/objects")
try fs.copyFile(a, "input.bin", "cache/objects/input.bin")
resolved := try fs.canonicalize(a, "cache/objects")
defer resolved.free(a)
```
