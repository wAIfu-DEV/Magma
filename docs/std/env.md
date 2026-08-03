# `std/env`

`get(a, name)` returns an allocator-owned snapshot, `has(name)` checks for a
variable, and `set(name, value)` and `unset(name)` mutate the process-wide
environment. Environment mutation requires external synchronization.

`list(a)` returns an owned snapshot of native `name=value` entries. Release it
with `snapshot.free()`; `snapshot.view()` and its strings borrow the snapshot.

Strings cross the native API boundary using the platform's ordinary
null-terminated representation. Magma does not perform an additional scan for
embedded null bytes; the first null therefore has the native terminator
semantics. Windows values are converted through UTF-16.
