mod thread_pool

use "allocator.mg" alc
use "cast.mg" cast
use "errors.mg" errors
use "mutex.mg" mutex
use "thread.mg" thread
use "wake.mg" wake
use "memory.mg" mem

Task(
    entry (ptr) u64
    context ptr
)

State(
    allocator alc.Allocator
    workers thread.Thread*
    workerCount u64
    tasks Task*
    capacity u64
    head u64
    tail u64
    count u64
    pending u64
    idleWaiters u64
    stopping bool
    fatalError error
    lock mutex.Mutex
    work wake.Wake
    idle wake.Wake
)

ThreadPool(
    state State*
)

taskAt(state State*, index u64) Task*:
    ret cast.utop(cast.ptou(state.tasks) + (index * sizeof Task))
..

workerAt(state State*, index u64) thread.Thread*:
    ret cast.utop(cast.ptou(state.workers) + (index * sizeof thread.Thread))
..

lockState(state State*) !void:
    try state.lock.lock()
..

unlockState(state State*) !void:
    try state.lock.unlock()
..

waitForWork(state State*) !void:
    try state.work.wait()
..

notifyWork(state State*) !void:
    try state.work.notify()
..

waitForIdle(state State*) !void:
    try state.idle.wait()
..

notifyIdle(state State*) !void:
    try state.idle.notify()
..

# Result adapters make fallible void operations inspectable by worker entry
# functions, whose native signature cannot return a Magma error.
waitForWorkResult(state State*) !bool:
    try waitForWork(state)
    ret true
..

lockStateResult(state State*) !bool:
    try lockState(state)
    ret true
..

unlockStateResult(state State*) !bool:
    try unlockState(state)
    ret true
..

notifyIdleResult(state State*) !bool:
    try notifyIdle(state)
    ret true
..

notifyWorkResult(state State*) !bool:
    try notifyWork(state)
    ret true
..

joinWorkerResult(state State*, index u64) !bool:
    try workerAt(state, index).join()
    ret true
..

freeIdleResult(state State*) !bool:
    try state.idle.free()
    ret true
..

freeWorkResult(state State*) !bool:
    try state.work.free()
    ret true
..

freeLockResult(state State*) !bool:
    try state.lock.free()
    ret true
..

waitStateResult(state State*) !bool:
    try waitState(state)
    ret true
..

# Records the first infrastructure failure. If taking the state lock itself
# fails, storing the error is best-effort because useful synchronization is no
# longer possible. Waking idle waiters ensures wait() can observe the failure.
recordFatal(state State*, failure error) void:
    waiterCount u64 = 1
    locked bool, lockErr error = lockStateResult(state)
    if errors.code(lockErr) == 0:
        if errors.code(state.fatalError) == 0:
            state.fatalError = failure
        ..
        state.stopping = true
        if state.idleWaiters > waiterCount:
            waiterCount = state.idleWaiters
        ..
        unlockState(state)
    elif errors.code(state.fatalError) == 0:
        state.fatalError = failure
        state.stopping = true
    ..
    i u64 = 0
    while i < waiterCount:
        notifyIdle(state)
        i = i + 1
    ..
..

# Used only after the state mutex itself fails. At that point synchronized
# recovery is impossible, so publish the failure directly and wake waiters.
recordFatalUnsafe(state State*, failure error) void:
    if errors.code(state.fatalError) == 0:
        state.fatalError = failure
    ..
    state.stopping = true
    notifyIdle(state)
..

initializeState(state State*, a alc.Allocator, workers thread.Thread*, workerCount u64, tasks Task*, capacity u64, lock $mutex.Mutex, work $wake.Wake, idle $wake.Wake) void:
    state.allocator = a
    state.workers = workers
    state.workerCount = workerCount
    state.tasks = tasks
    state.capacity = capacity
    state.lock = lock
    state.work = work
    state.idle = idle
..

workerMain(state State*) u64:
    running bool = true
    while running:
        waited bool, waitErr error = waitForWorkResult(state)
        if errors.code(waitErr) != 0:
            recordFatal(state, waitErr)
            ret 1
        ..
        locked bool, lockErr error = lockStateResult(state)
        if errors.code(lockErr) != 0:
            recordFatalUnsafe(state, lockErr)
            ret 1
        ..

        hasTask bool = state.count != 0
        stopping bool = state.stopping
        task Task
        if hasTask:
            task = *taskAt(state, state.head)
            state.head = (state.head + 1) % state.capacity
            state.count = state.count - 1
        ..
        unlocked bool, unlockErr error = unlockStateResult(state)
        if errors.code(unlockErr) != 0:
            recordFatalUnsafe(state, unlockErr)
            ret 1
        ..

        if hasTask:
            task.entry(task.context)
            completionLocked bool, completionLockErr error = lockStateResult(state)
            if errors.code(completionLockErr) != 0:
                recordFatalUnsafe(state, completionLockErr)
                ret 1
            ..
            state.pending = state.pending - 1
            becameIdle bool = state.pending == 0
            idleWaiters u64 = state.idleWaiters
            completionUnlocked bool, completionUnlockErr error = unlockStateResult(state)
            if errors.code(completionUnlockErr) != 0:
                recordFatalUnsafe(state, completionUnlockErr)
                ret 1
            ..
            if becameIdle:
                waiterIndex u64 = 0
                while waiterIndex < idleWaiters:
                    notified bool, notifyErr error = notifyIdleResult(state)
                    if errors.code(notifyErr) != 0:
                        recordFatal(state, notifyErr)
                        ret 1
                    ..
                    waiterIndex = waiterIndex + 1
                ..
            ..
        elif stopping:
            running = false
        ..
    ..
    ret 0
..

# Creates a fixed-size pool. queueCapacity bounds outstanding queued tasks.
pub new(a alc.Allocator, workerCount u64, queueCapacity u64) !$ThreadPool:
    if workerCount == 0 || queueCapacity == 0:
        throw errors.invalidArgument("thread pool sizes must be greater than zero")
    ..

    maxU64 u64 = 0 - 1
    if queueCapacity > maxU64 / sizeof Task || workerCount > maxU64 / sizeof thread.Thread:
        throw errors.wouldOverflow("thread pool allocation size overflow")
    ..

    state State* = try a.allocT[State](1)
    mem.zero(state, sizeof State)

    tasks Task*, tasksErr error = a.allocT[Task](queueCapacity)
    if errors.code(tasksErr) != 0:
        a.free(state)
        throw tasksErr
    ..
    workers thread.Thread*, workersErr error = a.allocT[thread.Thread](workerCount)
    if errors.code(workersErr) != 0:
        a.free(tasks)
        a.free(state)
        throw workersErr
    ..

    lock mutex.Mutex, lockErr error = mutex.new()
    if errors.code(lockErr) != 0:
        a.free(workers)
        a.free(tasks)
        a.free(state)
        throw lockErr
    ..
    work wake.Wake, workErr error = wake.new(wake.condition())
    if errors.code(workErr) != 0:
        lock.free()
        a.free(workers)
        a.free(tasks)
        a.free(state)
        throw workErr
    ..
    idle wake.Wake, idleErr error = wake.new(wake.condition())
    if errors.code(idleErr) != 0:
        work.free()
        lock.free()
        a.free(workers)
        a.free(tasks)
        a.free(state)
        throw idleErr
    ..

    initializeState(state, a, workers, workerCount, tasks, queueCapacity, lock, work, idle)

    i u64 = 0
    while i < workerCount:
        worker thread.Thread, spawnErr error = thread.new[State](workerMain, state)
        if errors.code(spawnErr) != 0:
            locked bool, stopLockErr error = lockStateResult(state)
            if errors.code(stopLockErr) == 0:
                state.stopping = true
                unlockState(state)
            else:
                state.stopping = true
            ..
            wakeIndex u64 = 0
            while wakeIndex < i:
                notifyWork(state)
                wakeIndex = wakeIndex + 1
            ..
            joinIndex u64 = 0
            while joinIndex < i:
                workerAt(state, joinIndex).join()
                joinIndex = joinIndex + 1
            ..
            idle.free()
            work.free()
            lock.free()
            a.free(workers)
            a.free(tasks)
            a.free(state)
            throw spawnErr
        ..
        workerSlot thread.Thread* = workerAt(state, i)
        *workerSlot = worker
        i = i + 1
    ..
    ret ThreadPool(state=state)
..

submitState(state State*, task Task) !void:
    try lockState(state)
    if errors.code(state.fatalError) != 0:
        failure error = state.fatalError
        try unlockState(state)
        throw failure
    ..
    if state.stopping:
        try unlockState(state)
        throw errors.failure("thread pool is stopping")
    ..
    if state.count == state.capacity:
        try unlockState(state)
        throw errors.wouldOverflow("thread pool queue is full")
    ..
    slot Task* = taskAt(state, state.tail)
    *slot = task
    state.tail = (state.tail + 1) % state.capacity
    state.count = state.count + 1
    state.pending = state.pending + 1
    try unlockState(state)
    notified bool, notifyErr error = notifyWorkResult(state)
    if errors.code(notifyErr) != 0:
        recordFatal(state, notifyErr)
        throw notifyErr
    ..
..

# Submits borrowed context. It must remain valid until wait() or shutdown().
# This bounded first implementation reports a full queue instead of blocking.
ThreadPool.submit(entry (ptr) u64, context ptr) !void:
    if this.state == none || entry == none:
        throw errors.invalidArgument("invalid thread pool submission")
    ..
    task := Task(entry=entry, context=context)
    try submitState(this.state, task)
..

waitState(state State*) !void:
    waiting bool = true
    while waiting:
        try lockState(state)
        if errors.code(state.fatalError) != 0:
            failure error = state.fatalError
            try unlockState(state)
            throw failure
        ..
        waiting = state.pending != 0
        if waiting:
            state.idleWaiters = state.idleWaiters + 1
        ..
        try unlockState(state)
        if waiting:
            waited bool, waitErr error = waitForIdleResult(state)
            try lockState(state)
            state.idleWaiters = state.idleWaiters - 1
            try unlockState(state)
            if errors.code(waitErr) != 0:
                recordFatal(state, waitErr)
                throw waitErr
            ..
        ..
    ..
..

# Waits until all tasks submitted before the observed idle point are complete.
ThreadPool.wait() !void:
    try waitState(this.state)
..

shutdownState(state State*) !void:
    firstError error = errors.ok()
    waited bool, waitErr error = waitStateResult(state)
    if errors.code(waitErr) != 0:
        firstError = waitErr
    ..
    locked bool, lockErr error = lockStateResult(state)
    if errors.code(lockErr) == 0:
        state.stopping = true
        unlocked bool, unlockErr error = unlockStateResult(state)
        if errors.code(unlockErr) != 0 && errors.code(firstError) == 0:
            firstError = unlockErr
        ..
    elif errors.code(firstError) == 0:
        firstError = lockErr
    ..

    i u64 = 0
    while i < state.workerCount:
        notified bool, notifyErr error = notifyWorkResult(state)
        if errors.code(notifyErr) != 0 && errors.code(firstError) == 0:
            firstError = notifyErr
        ..
        i = i + 1
    ..
    i = 0
    while i < state.workerCount:
        joined bool, joinErr error = joinWorkerResult(state, i)
        if errors.code(joinErr) != 0 && errors.code(firstError) == 0:
            firstError = joinErr
        ..
        i = i + 1
    ..

    freed bool, freeErr error = freeIdleResult(state)
    if errors.code(freeErr) != 0 && errors.code(firstError) == 0:
        firstError = freeErr
    ..
    workFreed bool, workFreeErr error = freeWorkResult(state)
    if errors.code(workFreeErr) != 0 && errors.code(firstError) == 0:
        firstError = workFreeErr
    ..
    lockFreed bool, lockFreeErr error = freeLockResult(state)
    if errors.code(lockFreeErr) != 0 && errors.code(firstError) == 0:
        firstError = lockFreeErr
    ..
    state.allocator.free(state.workers)
    state.allocator.free(state.tasks)
    if errors.code(firstError) != 0:
        throw firstError
    ..
..

# Completes queued work, stops every worker, joins them, and releases the pool.
destr ThreadPool.shutdown() !void:
    if this.state == none:
        throw errors.invalidArgument("thread pool is not active")
    ..
    state State* = this.state
    completed bool, shutdownErr error = shutdownStateResult(state)
    state.allocator.free(state)
    this.state = none
    if errors.code(shutdownErr) != 0:
        throw shutdownErr
    ..
..

shutdownStateResult(state State*) !bool:
    try shutdownState(state)
    ret true
..

waitForIdleResult(state State*) !bool:
    try waitForIdle(state)
    ret true
..
