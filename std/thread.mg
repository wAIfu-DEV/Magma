mod thread

use "errors.mg" errors
use "slices.mg" slices
use "cast.mg" cast

@platform("windows")
use "win/thread_impl.mg" impl_thread

@platform("linux", "android", "ios", "darwin", "freebsd", "netbsd", "openbsd")
use "unix/thread_impl.mg" impl_thread

# A joinable native thread. The context is borrowed and must remain valid until
# the entry function has returned and the thread has been joined. The entry's
# integer result is reserved for the platform adapter and should normally be 0.
Thread(
    impl impl_thread.Thread
)

# Starts entry(context) on a new native thread.
# The returned Thread owns its native thread resource and must be joined once.
# Type parameter is used to ensure the correct context type is passed as argument.
pub new[Ctx](entry (Ctx*) u64, context Ctx*) !$Thread:
    impl impl_thread.Thread = try impl_thread.spawn(entry, context)
    ret Thread(impl=impl)
..

# Waits until the thread finishes and releases its native thread resource.
destr Thread.join() !void:
    try impl_thread.join(addrof this.impl)
..

# Returns true once the entry function has finished. This does not consume the
# thread or release its native resources; join must still be called afterwards.
Thread.isFinished() !bool:
    ret try impl_thread.isFinished(addrof this.impl)
..

# Joins every thread in the slice. All elements are consumed. If more than one
# join fails, the first error is returned after the remaining joins are tried.
pub joinAll(threads Thread[]) !void:
    firstError error = errors.ok()
    base Thread* = slices.toPtr(threads)
    i u64 = 0
    
    while i < slices.count(threads):
        implPtr ptr = cast.utop(cast.ptou(base) + (i * sizeof Thread))
        joined bool, joinError error = impl_thread.join(implPtr)

        if errors.code(joinError) != 0 && errors.code(firstError) == 0:
            firstError = joinError
        ..
        i = i + 1
    ..
    if errors.code(firstError) != 0:
        throw firstError
    ..
..

# Gives the operating system an opportunity to schedule another ready thread.
pub yield() void:
    impl_thread.yield()
..
