# `std/dialog`

Native user-facing dialogs for choosing input files, output/download paths,
and directories.

```magma
a := heap.allocator()
filters := array dialog.Filter[1]
filters[0] = dialog.Filter(name="Images", extensions="png,jpg,jpeg")
configuration := dialog.Options(
    filters=filters,
    defaultPath="",
    defaultName="image.png",
    title="Save image",
    parent=none,
)
destination str, dialogError error = dialog.saveFile(a, configuration)
if dialogError.nok():
    if errors.is(dialogError, errors.cancelled("")):
        ret
    ..
    throw dialogError
..
defer destination.free(a)
try fs.writeFile(a, destination, encodedImage)
```

## API

- `defaultOptions()` returns a configuration with native defaults.
- `openFile(a, options)` selects one existing file.
- `saveFile(a, options)` selects a destination path; it does not write or download anything.
- `openDir(a, options)` selects one directory.
- `Filter` contains a friendly name and comma-separated extensions without dots.
- `Options` borrows its filters and strings for the duration of the call.
- Each dialog returns an owned path string that must be released with the allocator passed to the call.
- Cancellation throws the generic `errors.cancelled` category; it is distinct from native dialog failure.
- `parent` is an optional native window handle. Supplying it keeps the dialog modal to the application window.

The Windows backend uses the modern Vista-and-newer `IFileDialog` family. It
does not use the legacy `GetOpenFileName` or `SHBrowseForFolder` interfaces.

The Linux API is reserved for an XDG Desktop Portal implementation. XDG
portals communicate over the desktop session D-Bus and work on both X11 and
Wayland; they are not X11-specific.

## Credit

The Windows implementation is an altered, native Magma port derived from
Native File Dialog 1.1.6 by Michael Labbe. Native File Dialog is distributed
under the zlib license. The complete notice is retained in
`std/licenses/NATIVE_FILE_DIALOG.txt`.
