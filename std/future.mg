mod future
# Owned asynchronous results that can be awaited and released safely.

use "std:allocator" alc
use "std:cast" cast
use "std:errors" errors
use "std:thread_pool" thread_pool
use "std:time" time

@platform("windows")
use "std:win/address_wait" address_wait

@platform("linux", "android", "ios", "darwin", "freebsd", "netbsd", "openbsd")
use "std:unix/address_wait" address_wait

State[T](
    allocator alc.Allocator
    value T
    failure error
    status u32
    references u32
    waiter address_wait.Wait
)

Work[T, Context](
    state State[T]
    entry (Context*) !T
    context Context
)

# Single-consumer asynchronous result backed by worker-pool state.
# @mustcall await
pub Future[T](
    state State[T]*
)

atomicAdd(target u64*, value u64) void:
    llvm "  %previous = atomicrmw add ptr %target, i64 %value monotonic, align 8\n"
    llvm "  ret void\n"
..

atomicLoad(target u64*) u64:
    llvm "  %value = load atomic i64, ptr %target monotonic, align 8\n"
    llvm "  ret i64 %value\n"
..

atomicStore(target u64*, value u64) void:
    llvm "  store atomic i64 %value, ptr %target monotonic, align 8\n"
    llvm "  ret void\n"
..

publishDone(status u32*) void:
    llvm "  store atomic i32 1, ptr %status release, align 4\n"
    llvm "  ret void\n"
..

loadStatus(status u32*) u32:
    llvm "  %value = load atomic i32, ptr %status acquire, align 4\n"
    llvm "  ret i32 %value\n"
..

releaseReference(references u32*) u32:
    llvm "  %previous = atomicrmw sub ptr %references, i32 1 acq_rel, align 4\n"
    llvm "  ret i32 %previous\n"
..

releaseState[T](state State[T]*) void:
    if releaseReference(addrof state.references) == 1:
        address_wait.free(addrof state.waiter)
        state.allocator.free(state)
    ..
..

taskMain[T, Context](raw ptr) u64:
    work Work[T, Context]* = cast.reinterpret[Work[T, Context]](raw)
    state State[T]* = addrof work.state
    value T, failure error = work.entry(addrof work.context)
    if errors.code(failure) == 0:
        state.value = value
    else:
        state.failure = failure
    ..
    publishDone(addrof state.status)
    address_wait.wake(addrof state.waiter, addrof state.status)
    releaseState[T](state)
    ret 0
..

submitWork[T, Context](pool thread_pool.ThreadPool, work Work[T, Context]*) !bool:
    try pool.submit(taskMain[T, Context], work)
    ret true
..

# Future backend using atomic publication and a platform completion wait.
# @complexity O(1) to allocate and submit
# @param a allocator for task and result state
# @param pool pool that executes entry
# @param entry function producing the result
# @param context context copied into task storage
# @returns owned active future
# @ownership pool and a must remain valid until await completes.
# @example
#   pending := try future.new[u64, Work](a, pool, run, work)
pub new[T, Context](a alc.Allocator, pool thread_pool.ThreadPool, entry (Context*) !T, context Context) !$Future[T]:
    work Work[T, Context]* = try a.allocT[Work[T, Context]](1)
    state State[T]* = addrof work.state
    state.allocator = a
    state.failure = errors.ok()
    state.status = 0
    state.references = 2
    work.entry = entry
    work.context = context

    waiter address_wait.Wait, waiterErr error = address_wait.new()
    if errors.code(waiterErr) != 0:
        a.free(work)
        throw waiterErr
    ..
    state.waiter = waiter

    submitted bool, submitErr error = submitWork[T, Context](pool, work)
    if errors.code(submitErr) != 0:
        address_wait.free(addrof state.waiter)
        a.free(work)
        throw submitErr
    ..
    ret Future[T](state=state)
..

# Reports whether the worker has published a result without consuming it.
# @complexity O(1)
# @throws invalidArgument after the future has been consumed
# @example
#   complete := try pending.isDone()
Future[T].isDone() !bool:
    if this.state == none:
        throw errors.invalidArgument("future is not active")
    ..
    state State[T]* = this.state
    ret loadStatus(addrof state.status) != 0
..

# Blocks until completion, consumes the future, and returns ownership of its result.
# @complexity O(1) when complete; otherwise blocks without busy-waiting
# @throws the error returned by the worker entry function
# @throws invalidArgument when the future was already consumed
# @example
#   value := try pending.await()
destr Future[T].await() !$T:
    if this.state == none:
        throw errors.invalidArgument("future is not active")
    ..
    state State[T]* = this.state
    while loadStatus(addrof state.status) == 0:
        try address_wait.wait(addrof state.waiter, addrof state.status)
    ..

    if errors.code(state.failure) != 0:
        failure error = state.failure
        this.state = none
        releaseState[T](state)
        throw failure
    ..
    value $T = state.value
    this.state = none
    releaseState[T](state)
    ret value
..
