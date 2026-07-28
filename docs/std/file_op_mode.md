# `std/file_op_mode`

## Example

```magma
readOnly := file.mode().read()
readWrite := file.mode().read().write()
appendOnly := file.mode().write().append()
```

Defines composable file-open flags.

## Type

`OpenMode(flags u8)` stores the bitwise combination of `FLAG_READ`, `FLAG_WRITE`,
`FLAG_APPEND`, `FLAG_CREATE`, and `FLAG_TRUNCATE`.

## Methods

- `OpenMode.read() OpenMode` returns the mode with reading enabled.
- `OpenMode.write() OpenMode` returns the mode with writing enabled.
- `OpenMode.append() OpenMode` returns the mode with append enabled.
- `OpenMode.create() OpenMode` creates the file when it does not exist.
- `OpenMode.truncate() OpenMode` truncates an existing file on open.

Methods return a modified value, allowing chaining from `file.mode()`.
