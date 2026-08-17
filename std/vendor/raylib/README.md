# Raylib binary files

Place the raylib 5.5 static archives used by the standard-library binding in
the platform directories:

- `win/raylib.lib`
- `linux/libraylib.a`
- `mac/libraylib.a`

These binaries are intentionally ignored by Git. Shared raylib libraries may
also be kept alongside them, but the default binding does not use them.
