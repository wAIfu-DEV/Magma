mod executor

# Type-erased task scheduler. Executor is a borrowed view: its implementation
# must remain valid until every submitted task has completed.

pub proto Executor(
    submitRaw(entry ptr, context ptr) !void
    releaseRaw() void
)

# Schedules entry(context) for execution.
# @ownership context remains caller-owned and must stay valid until the task ends.
Executor.submit[Ctx](entry (Ctx*) u64, context Ctx*) !void:
    try this.submitRaw(entry, context)
..

# Releases resources owned by the view. Borrowed adapters may implement this as
# a no-op; it never releases the underlying scheduler.
destr Executor.free() void:
    this.releaseRaw()
..
