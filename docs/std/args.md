# `std/args`

`Args` is an allocation-free borrowed view over the `str[]` passed to
`main`. `executable()` returns element zero, while `values()`, `count()`,
`get()`, and `iterator()` address only the arguments after the executable.

```magma
pub main(raw str[]) !void:
    command := args.new(raw)
    executable := try command.executable()
    values := command.values()
..
```

The wrapper and all strings returned from it are invalidated when the original
argument slice is invalidated.
