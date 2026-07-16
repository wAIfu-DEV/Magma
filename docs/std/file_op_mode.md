# `std/file_op_mode`

## Example

```magma
readOnly := file.mode().read()
readWrite := file.mode().read().write()
appendOnly := file.mode().write().append()
```

Defines composable file-open flags.

## Type

`OpenMode(r bool, w bool, a bool)` records read, write, and append permission.

## Methods

- `OpenMode.read() OpenMode` returns the mode with reading enabled.
- `OpenMode.write() OpenMode` returns the mode with writing enabled.
- `OpenMode.append() OpenMode` returns the mode with append enabled.

Methods return a modified value, allowing chaining from `file.mode()`.
