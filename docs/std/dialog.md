# `std/dialog`

Native user-facing dialogs for choosing input files, output paths, and
directories. This is the current module name; `file_dialog.md` is retained as
an older documentation path.

```magma
use "std:dialog" dialog
use "std:heap" heap

options := dialog.defaultOptions()
path := try dialog.openFile(heap.allocator(), options)
defer path.free(heap.allocator())
```

## API

- `Filter(name, extensions)` describes a named group of comma-separated file
  extensions without leading dots, such as `"png,jpg"`.
- `Options(filters, defaultPath, defaultName, title, parent)` borrows all
  strings and filters for the duration of a call. `parent` is an optional
  native window handle.
- `defaultOptions()` supplies empty/native defaults.
- `openFile(a, options)`, `saveFile(a, options)`, and `openDir(a, options)`
  return an owned path that must be freed with `a`. `saveFile` only chooses a
  path; it does not create or write the file.

Cancellation is reported with the standard cancelled error category. Windows
uses native dialogs. The other platform entry points are currently reserved
and report unsupported operation.
