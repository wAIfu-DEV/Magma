mod lock
# Type-erased mutual-exclusion interface with explicit cleanup.

# Callback table implementing a Locker.
pub Vtable(
    lock (ptr) !void
    unlock (ptr) !void
    free (ptr) void
)

# Type-erased, non-owning handle to a mutual-exclusion implementation.
pub Locker(
    impl ptr
    vtable Vtable*
)

# Blocks until the caller acquires exclusive access.
# @complexity Implementation-dependent
# @example
#   try guard.lock()
Locker.lock() !void:
    try this.vtable.lock(this.impl)
..

# Releases exclusive access held by the caller.
# @complexity Implementation-dependent
# @warning Unlocking without a matching successful lock is invalid.
# @example
#   try guard.unlock()
Locker.unlock() !void:
    try this.vtable.unlock(this.impl)
..

# Invokes the implementation cleanup callback.
# @complexity Implementation-dependent
# @warning Do not use the Locker after free.
destr Locker.free() void:
    this.vtable.free(this.impl)
..
