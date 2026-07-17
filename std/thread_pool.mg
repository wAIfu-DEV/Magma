mod thread_pool

use "allocator.mg" alc
use "cast.mg" cast
use "errors.mg" errors
use "memory.mg" mem
use "mutex.mg" mutex
use "thread.mg" thread
use "wake.mg" wake
use "cpu.mg" cpu

@platform("windows")
use "win/generation_wait.mg" generation_wait

@platform("linux", "android", "ios", "darwin", "freebsd", "netbsd", "openbsd")
use "unix/generation_wait.mg" generation_wait

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
    sleepingWorkers u64
    wakeReservations u64
    workGeneration u32
    spinCount u64
    stopping bool
    fatalError error
    lock mutex.Mutex
    work generation_wait.Wait
    idle wake.Wake
)

cpuPause() void:
    llvm "  call void asm sideeffect \"pause\", \"~{memory}\"()\n"
    llvm "  ret void\n"
..

ThreadPool(
    state State*
)

# Makes ownership transfers into the heap-backed State explicit to the
# destructor checker. It cannot infer a move through a raw-pointer assignment.
claim[T](claimed $T) $T:
    ret claimed
..

abandon[T](value $T) void:
    abandoned T[1]
    abandoned[0] = value
..

releaseLock(value $mutex.Mutex) void:
    value.free()
..

releaseIdle(value $wake.Wake) void:
    value.free()
..

spawnWorkerInto(state State*, destination thread.Thread*) !bool:
    worker := try thread.new[State](workerMain, state)
    *destination = claim[thread.Thread](worker)
    ret true
..

taskAt(state State*, index u64) Task*:
    ret cast.utop(cast.ptou(state.tasks) + (index * sizeof Task))
..

# Doubles and linearizes a full queue. The caller holds state.lock. Allocation
# happens before any state is changed, so a failure leaves the queue intact.
growQueue(state State*) !bool:
    maxU64 u64 = 0 - 1
    if state.capacity > maxU64 / 2:
        throw errors.wouldOverflow("thread pool queue capacity overflow")
    ..
    newCapacity u64 = state.capacity * 2
    newTasks Task* = try state.allocator.allocT[Task](newCapacity)
    i u64 = 0
    while i < state.count:
        source u64 = (state.head + i) % state.capacity
        destination Task* = cast.utop(cast.ptou(newTasks) + (i * sizeof Task))
        *destination = *taskAt(state, source)
        i = i + 1
    ..
    state.allocator.free(state.tasks)
    state.tasks = newTasks
    state.capacity = newCapacity
    state.head = 0
    state.tail = state.count
    ret true
..

workerAt(state State*, index u64) thread.Thread*:
    ret cast.utop(cast.ptou(state.workers) + (index * sizeof thread.Thread))
..

lockResult(state State*) !bool:
    try state.lock.lock()
    ret true
..

unlockResult(state State*) !bool:
    try state.lock.unlock()
    ret true
..

waitIdleResult(state State*) !bool:
    try state.idle.wait()
    ret true
..

joinResult(state State*, index u64) !bool:
    try workerAt(state, index).join()
    ret true
..

recordFatal(state State*, failure error) void:
    locked bool, lockErr error = lockResult(state)
    if errors.code(lockErr) == 0:
        if errors.code(state.fatalError) == 0:
            state.fatalError = failure
        ..
        state.stopping = true
        sleepers u64 = state.sleepingWorkers
        state.sleepingWorkers = 0
        state.wakeReservations = 0
        state.lock.unlock()
        generation_wait.wakeAll(addrof state.work, addrof state.workGeneration, sleepers)
    else:
        state.fatalError = failure
        state.stopping = true
        generation_wait.wakeAll(addrof state.work, addrof state.workGeneration, state.workerCount)
    ..
    state.idle.notify()
..

workerMain(state State*) u64:
    running bool = true
    while running:
        locked bool, lockErr error = lockResult(state)
        if errors.code(lockErr) != 0:
            recordFatal(state, lockErr)
            ret 1
        ..

        if state.count != 0:
            task Task = *taskAt(state, state.head)
            state.head = (state.head + 1) % state.capacity
            state.count = state.count - 1
            unlocked bool, unlockErr error = unlockResult(state)
            if errors.code(unlockErr) != 0:
                recordFatal(state, unlockErr)
                ret 1
            ..

            task.entry(task.context)
            completionLocked bool, completionLockErr error = lockResult(state)
            if errors.code(completionLockErr) != 0:
                recordFatal(state, completionLockErr)
                ret 1
            ..
            state.pending = state.pending - 1
            becameIdle bool = state.pending == 0
            idleWaiters u64 = state.idleWaiters
            completionUnlocked bool, completionUnlockErr error = unlockResult(state)
            if errors.code(completionUnlockErr) != 0:
                recordFatal(state, completionUnlockErr)
                ret 1
            ..
            if becameIdle:
                i u64 = 0
                while i < idleWaiters:
                    state.idle.notify()
                    i = i + 1
                ..
            ..
        elif state.stopping:
            state.lock.unlock()
            running = false
        else:
            observed u32 = generation_wait.observe(addrof state.workGeneration)
            if state.spinCount != 0:
                unlocked bool, unlockErr error = unlockResult(state)
                if errors.code(unlockErr) != 0:
                    recordFatal(state, unlockErr)
                    ret 1
                ..
                spins u64 = 0
                while spins < state.spinCount && generation_wait.observe(addrof state.workGeneration) == observed:
                    cpuPause()
                    spins = spins + 1
                ..
                if generation_wait.observe(addrof state.workGeneration) != observed:
                    continue
                ..
                spinLocked bool, spinLockErr error = lockResult(state)
                if errors.code(spinLockErr) != 0:
                    recordFatal(state, spinLockErr)
                    ret 1
                ..
                if state.count != 0 || state.stopping || generation_wait.observe(addrof state.workGeneration) != observed:
                    spinUnlocked bool, spinUnlockErr error = unlockResult(state)
                    if errors.code(spinUnlockErr) != 0:
                        recordFatal(state, spinUnlockErr)
                        ret 1
                    ..
                    continue
                ..
            ..
            state.sleepingWorkers = state.sleepingWorkers + 1
            unlocked bool, unlockErr error = unlockResult(state)
            if errors.code(unlockErr) != 0:
                recordFatal(state, unlockErr)
                ret 1
            ..
            waited bool, waitErr error = waitWorkResult(state, observed)
            if errors.code(waitErr) != 0:
                recordFatal(state, waitErr)
                ret 1
            ..
            wakeLocked bool, wakeLockErr error = lockResult(state)
            if errors.code(wakeLockErr) != 0:
                recordFatal(state, wakeLockErr)
                ret 1
            ..
            if state.sleepingWorkers != 0:
                state.sleepingWorkers = state.sleepingWorkers - 1
            ..
            if state.wakeReservations != 0:
                state.wakeReservations = state.wakeReservations - 1
            ..
            wakeUnlocked bool, wakeUnlockErr error = unlockResult(state)
            if errors.code(wakeUnlockErr) != 0:
                recordFatal(state, wakeUnlockErr)
                ret 1
            ..
        ..
    ..
    ret 0
..

waitWorkResult(state State*, observed u32) !bool:
    try generation_wait.wait(addrof state.work, addrof state.workGeneration, observed)
    ret true
..

newConfigured(a alc.Allocator, workerCount u64, queueCapacity u64, spinCount u64) !$ThreadPool:
    if workerCount == 0 || queueCapacity == 0:
        throw errors.invalidArgument("thread pool sizes must be greater than zero")
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
        # A failed constructor does not produce an owned lock. Make that
        # conditional ownership explicit to the destructor checker.
        abandon[mutex.Mutex](lock)
        a.free(workers)
        a.free(tasks)
        a.free(state)
        throw lockErr
    ..
    work generation_wait.Wait, workErr error = generation_wait.new()
    if errors.code(workErr) != 0:
        releaseLock(lock)
        a.free(workers)
        a.free(tasks)
        a.free(state)
        throw workErr
    ..
    idle wake.Wake, idleErr error = wake.new(wake.condition())
    if errors.code(idleErr) != 0:
        # As above, the error result means idle contains no live resource.
        abandon[wake.Wake](idle)
        generation_wait.free(addrof work)
        releaseLock(lock)
        a.free(workers)
        a.free(tasks)
        a.free(state)
        throw idleErr
    ..
    state.allocator = a
    state.workers = workers
    state.workerCount = workerCount
    state.tasks = tasks
    state.capacity = queueCapacity
    state.spinCount = spinCount
    state.lock = claim[mutex.Mutex](lock)
    state.work = work
    state.idle = claim[wake.Wake](idle)

    i u64 = 0
    while i < workerCount:
        spawned bool, spawnErr error = spawnWorkerInto(state, workerAt(state, i))
        if errors.code(spawnErr) != 0:
            state.stopping = true
            generation_wait.wakeAll(addrof state.work, addrof state.workGeneration, i)
            j u64 = 0
            while j < i:
                workerAt(state, j).join()
                j = j + 1
            ..
            state.idle.free()
            generation_wait.free(addrof state.work)
            state.lock.free()
            a.free(workers)
            a.free(tasks)
            a.free(state)
            throw spawnErr
        ..
        i = i + 1
    ..
    ret ThreadPool(state=state)
..

pub new(a alc.Allocator, workerCount u64, queueCapacity u64, spinCount u64) !$ThreadPool:
    ret try newConfigured(a, workerCount, queueCapacity, spinCount)
..

pub newDefault(a alc.Allocator) !$ThreadPool:
    threadCount := cpu.coreCount()
    spinCount := threadCount / 3

    if spinCount < 1:
        spinCount = 1
    ..
    ret try newConfigured(a, threadCount, 256, spinCount)
..

ThreadPool.submit(entry (ptr) u64, context ptr) !void:
    if this.state == none || entry == none:
        throw errors.invalidArgument("invalid thread pool submission")
    ..
    state State* = this.state
    try state.lock.lock()
    if errors.code(state.fatalError) != 0:
        failure error = state.fatalError
        try state.lock.unlock()
        throw failure
    ..
    if state.stopping:
        try state.lock.unlock()
        throw errors.failure("thread pool is stopping")
    ..
    if state.count == state.capacity:
        grown bool, growErr error = growQueue(state)
        if errors.code(growErr) != 0:
            try state.lock.unlock()
            throw growErr
        ..
    ..
    *taskAt(state, state.tail) = Task(entry=entry, context=context)
    state.tail = (state.tail + 1) % state.capacity
    state.count = state.count + 1
    state.pending = state.pending + 1
    shouldWake bool = state.sleepingWorkers > state.wakeReservations
    if shouldWake:
        state.wakeReservations = state.wakeReservations + 1
    ..
    try state.lock.unlock()
    if shouldWake:
        generation_wait.wakeOne(addrof state.work, addrof state.workGeneration)
    elif state.spinCount != 0:
        generation_wait.signal(addrof state.workGeneration)
    ..
..

ThreadPool.wait() !void:
    state State* = this.state
    waiting bool = true
    while waiting:
        try state.lock.lock()
        if errors.code(state.fatalError) != 0:
            failure error = state.fatalError
            try state.lock.unlock()
            throw failure
        ..
        waiting = state.pending != 0
        if waiting:
            state.idleWaiters = state.idleWaiters + 1
        ..
        try state.lock.unlock()
        if waiting:
            waited bool, waitErr error = waitIdleResult(state)
            try state.lock.lock()
            state.idleWaiters = state.idleWaiters - 1
            try state.lock.unlock()
            if errors.code(waitErr) != 0:
                throw waitErr
            ..
        ..
    ..
..

destr ThreadPool.close() !void:
    if this.state == none:
        throw errors.invalidArgument("thread pool is not active")
    ..
    state State* = this.state
    try this.wait()
    try state.lock.lock()
    state.stopping = true
    sleepers u64 = state.sleepingWorkers
    state.sleepingWorkers = 0
    state.wakeReservations = 0
    try state.lock.unlock()
    generation_wait.wakeAll(addrof state.work, addrof state.workGeneration, sleepers)
    i u64 = 0
    while i < state.workerCount:
        try workerAt(state, i).join()
        i = i + 1
    ..
    try state.idle.free()
    generation_wait.free(addrof state.work)
    try state.lock.free()
    state.allocator.free(state.workers)
    state.allocator.free(state.tasks)
    state.allocator.free(state)
    this.state = none
..
