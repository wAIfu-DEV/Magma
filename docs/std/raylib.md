# `std/raylib`

Initial Windows bindings for raylib 5.5 using its shared-library distribution.
Put the import library at `vendor/raylib/raylib.lib`, compile normally, and put
`raylib.dll` beside the resulting executable before running it.

The initial surface includes window management, timing, drawing basic 2D
shapes and text, keyboard input, mouse input, config flags, key constants, and
the standard raylib color palette. See `samples/raylib_window.mg`.

Functions ending in `C` take an existing null-terminated UTF-8 pointer and are
appropriate for drawing loops. The `str` wrappers allocate a temporary C string
and are intended for setup or occasional calls.

Texture, image, audio, model, and functions passing larger C structs are not
yet exposed. They require explicit C ABI lowering support in Magma; declaring
them directly as ordinary Magma struct calls would be unsafe on Windows x64.
