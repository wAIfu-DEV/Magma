# `std/fs`

Whole-file convenience operations.

- `pub readFile(a alc.Allocator, path str) !$str` opens and reads the complete file into an owned string, then closes the file. The caller frees the result with the same allocator.
- `pub writeFile(a alc.Allocator, path str, contents str) !void` creates or truncates a file, writes all `contents`, and closes it.

Both functions propagate allocation, open, I/O, and close errors.
