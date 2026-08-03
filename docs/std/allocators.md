# `std/allocators`

Convenience namespace that groups the standard allocator modules under one
import:

```magma
use "std:allocators" allocators

a := allocators.heap.allocator()
arena := try allocators.arena.newDefault(a)
```

The public re-exports are:

- `allocator` for `std/allocator`
- `heap` for `std/heap`
- `arena` for `std/arena_alloc`
- `debug` for `std/debug_alloc`
- `scratch` for `std/scratch_alloc`

These are module namespaces, not allocator instances, and retain the APIs and
ownership rules documented by their underlying modules.
