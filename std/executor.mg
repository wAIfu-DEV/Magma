mod executor

# Type-erased task scheduler. Executor is a borrowed view: its implementation
# and vtable must remain valid until every submitted task has completed.

pub Vtable(
    submit (ptr, ptr, ptr) !void
    free (ptr) void
)

pub Executor(
    impl ptr
    vtable Vtable*
)

# Schedules entry(context) for execution.
# @ownership context remains caller-owned and must stay valid until the task ends.
Executor.submit[Ctx](entry (Ctx*) u64, context Ctx*) !void:
    try this.vtable.submit(this.impl, entry, context)
..

# Releases resources owned by the view. Borrowed adapters may implement this as
# a no-op; it never releases the underlying scheduler.
destr Executor.free() void:
    this.vtable.free(this.impl)
..
