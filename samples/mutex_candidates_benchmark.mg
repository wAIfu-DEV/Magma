mod main
# Linux/x86-64 mutex candidate benchmark. All hot paths are Magma code; the
# only native boundary added by candidates is the futex syscall when parking.

use "std:atomic" as atomic
use "std:c" as c
use "std:errors" as errors
use "std:io" as io
use "std:mutex" as mutex
use "std:slices" as slices
use "std:thread" as thread
use "std:time" as time
use "std:writer" as writer

ext ext_syscall syscall(number c.long, address ptr, operation c.int, expected u32, timeout ptr, secondAddress ptr, value u32) c.long

const FUTEX_WAIT_PRIVATE i32 = 128
const FUTEX_WAKE_PRIVATE i32 = 129
const FUTEX_WAIT_BITSET_PRIVATE i32 = 137
const FUTEX_WAKE_BITSET_PRIVATE i32 = 138
const CREATION_ITERATIONS u64 = 1000000
const UNCONTENDED_ITERATIONS u64 = 5000000
const WORKERS u64 = 4
const ITERATIONS u64 = 100000

StateMutex(
    state atomic.U32
    waits atomic.U64
    wakes atomic.U64
)

TicketMutex(
    next atomic.U32
    serving atomic.U32
    waits atomic.U64
    wakes atomic.U64
)

stateNew() StateMutex:
    ret StateMutex(state=atomic.newU32(0), waits=atomic.newU64(0), wakes=atomic.newU64(0))
..

ticketNew() TicketMutex:
    ret TicketMutex(next=atomic.newU32(0), serving=atomic.newU32(0), waits=atomic.newU64(0), wakes=atomic.newU64(0))
..

compareExchange(value atomic.U32*, expected u32, desired u32) u32:
    unsafe:
        ret @llvm("cmpxchg_old", value, expected, desired, "acquire", "monotonic", 4, u32)
    ..
..

cpuRelax() void:
    unsafe:
        @llvm("asm_sideeffect", "pause", void)
    ..
..

# Keeps construction observable without adding a function call or resource to
# candidates whose complete initialization is just zeroing their state.
consumeMutex(value ptr) void:
    unsafe:
        @llvm("sideeffect", void)
    ..
..

futexWait(address atomic.U32*, expected u32) void:
    ext_syscall(202, address, FUTEX_WAIT_PRIVATE, expected, none, none, 0)
..

futexWakeOne(address atomic.U32*) void:
    ext_syscall(202, address, FUTEX_WAKE_PRIVATE, 1, none, none, 0)
..

ticketMask(ticket u32) u32:
    ret 1 << (ticket & 31)
..

futexWaitTicket(address atomic.U32*, expected u32, ticket u32) void:
    ext_syscall(202, address, FUTEX_WAIT_BITSET_PRIVATE, expected, none, none, ticketMask(ticket))
..

futexWakeTicket(address atomic.U32*, ticket u32) void:
    ext_syscall(202, address, FUTEX_WAKE_BITSET_PRIVATE, 0x7FFFFFFF, none, none, ticketMask(ticket))
..

# Candidate 1: conventional unlocked/locked/contended futex state machine.
threeStateLock(value StateMutex*) void:
    observed := compareExchange(addrof value.state, 0, 1)
    if observed == 0:
        ret
    ..
    if observed != 2:
        observed = value.state.exchange(2)
    ..
    loop observed != 0:
        value.waits.fetchAdd(1)
        futexWait(addrof value.state, 2)
        observed = value.state.exchange(2)
    ..
..

threeStateUnlock(value StateMutex*) void:
    previous := value.state.fetchSub(1)
    if previous == 1:
        ret
    ..
    value.state.storeRelease(0)
    value.wakes.fetchAdd(1)
    futexWakeOne(addrof value.state)
..

# Candidate 2: bounded spinning before the same futex parking path.
adaptiveLock(value StateMutex*) void:
    observed := compareExchange(addrof value.state, 0, 1)
    if observed == 0:
        ret
    ..
    for spin u64 = 0 to 128:
        if value.state.load() == 0:
            observed = compareExchange(addrof value.state, 0, 1)
            if observed == 0:
                ret
            ..
        ..
        cpuRelax()
    ..
    observed = value.state.exchange(2)
    loop observed != 0:
        value.waits.fetchAdd(1)
        futexWait(addrof value.state, 2)
        observed = value.state.exchange(2)
    ..
..

adaptiveUnlock(value StateMutex*) void:
    threeStateUnlock(value)
..

# Candidate 3: FIFO ticket assignment, parking on the serving counter.
ticketLock(value TicketMutex*) void:
    ticket := value.next.fetchAdd(1)
    spins u64 = 0
    observed := value.serving.loadAcquire()
    loop observed != ticket:
        if spins < 64:
            spins = spins + 1
            cpuRelax()
        else:
            value.waits.fetchAdd(1)
            futexWaitTicket(addrof value.serving, observed, ticket)
        ..
        observed = value.serving.loadAcquire()
    ..
..

ticketUnlock(value TicketMutex*) void:
    nextServing := value.serving.fetchAdd(1) + 1
    if value.next.loadAcquire() != nextServing:
        value.wakes.fetchAdd(1)
        futexWakeTicket(addrof value.serving, nextServing)
    ..
..

BaselineContext(lock mutex.Mutex*, counter u64*, ready atomic.U64*, start atomic.U64*)
StateContext(lock StateMutex*, counter u64*, ready atomic.U64*, start atomic.U64*)
TicketContext(lock TicketMutex*, counter u64*, ready atomic.U64*, start atomic.U64*)

baselineWorker(context BaselineContext*) u64:
    context.ready.fetchAdd(1)
    loop context.start.loadAcquire() == 0: thread.yield() ..
    for i u64 = 0 to ITERATIONS:
        locked bool, lockError error = baselineLock(context.lock)
        if lockError.nok(): ret 1 ..
        *context.counter = *context.counter + 1
        unlocked bool, unlockError error = baselineUnlock(context.lock)
        if unlockError.nok(): ret 1 ..
    ..
    ret 0
..

threeStateWorker(context StateContext*) u64:
    context.ready.fetchAdd(1)
    loop context.start.loadAcquire() == 0: thread.yield() ..
    for i u64 = 0 to ITERATIONS:
        threeStateLock(context.lock)
        *context.counter = *context.counter + 1
        threeStateUnlock(context.lock)
    ..
    ret 0
..

adaptiveWorker(context StateContext*) u64:
    context.ready.fetchAdd(1)
    loop context.start.loadAcquire() == 0: thread.yield() ..
    for i u64 = 0 to ITERATIONS:
        adaptiveLock(context.lock)
        *context.counter = *context.counter + 1
        adaptiveUnlock(context.lock)
    ..
    ret 0
..

ticketWorker(context TicketContext*) u64:
    context.ready.fetchAdd(1)
    loop context.start.loadAcquire() == 0: thread.yield() ..
    for i u64 = 0 to ITERATIONS:
        ticketLock(context.lock)
        *context.counter = *context.counter + 1
        ticketUnlock(context.lock)
    ..
    ret 0
..

baselineLock(value mutex.Mutex*) !bool:
    try value.lock()
    ret true
..

baselineUnlock(value mutex.Mutex*) !bool:
    try value.unlock()
    ret true
..

baselineContended() !u64:
    lock := try mutex.new()
    counter u64 = 0
    ready := atomic.newU64(0)
    start := atomic.newU64(0)
    contexts := array BaselineContext[24]
    workers := array thread.Thread[24]
    for i u64 = 0 to WORKERS:
        contexts[i] = BaselineContext(lock=addrof lock, counter=addrof counter, ready=addrof ready, start=addrof start)
        workers[i] = try thread.new[BaselineContext](baselineWorker, addrof contexts[i])
    ..
    loop ready.loadAcquire() != WORKERS: thread.yield() ..
    start.storeRelease(1)
    try thread.joinAll(slices.fromPtr(slices.toPtr(workers), WORKERS))
    try lock.free()
    ret counter
..

threeStateContended() !u64:
    lock := stateNew()
    counter u64 = 0
    ready := atomic.newU64(0)
    start := atomic.newU64(0)
    contexts := array StateContext[24]
    workers := array thread.Thread[24]
    for i u64 = 0 to WORKERS:
        contexts[i] = StateContext(lock=addrof lock, counter=addrof counter, ready=addrof ready, start=addrof start)
        workers[i] = try thread.new[StateContext](threeStateWorker, addrof contexts[i])
    ..
    loop ready.loadAcquire() != WORKERS: thread.yield() ..
    start.storeRelease(1)
    try thread.joinAll(slices.fromPtr(slices.toPtr(workers), WORKERS))
    ret counter
..

adaptiveContended() !u64:
    lock := stateNew()
    counter u64 = 0
    ready := atomic.newU64(0)
    start := atomic.newU64(0)
    contexts := array StateContext[24]
    workers := array thread.Thread[24]
    for i u64 = 0 to WORKERS:
        contexts[i] = StateContext(lock=addrof lock, counter=addrof counter, ready=addrof ready, start=addrof start)
        workers[i] = try thread.new[StateContext](adaptiveWorker, addrof contexts[i])
    ..
    loop ready.loadAcquire() != WORKERS: thread.yield() ..
    start.storeRelease(1)
    try thread.joinAll(slices.fromPtr(slices.toPtr(workers), WORKERS))
    ret counter
..

ticketContended() !u64:
    lock := ticketNew()
    counter u64 = 0
    ready := atomic.newU64(0)
    start := atomic.newU64(0)
    contexts := array TicketContext[24]
    workers := array thread.Thread[24]
    for i u64 = 0 to WORKERS:
        contexts[i] = TicketContext(lock=addrof lock, counter=addrof counter, ready=addrof ready, start=addrof start)
        workers[i] = try thread.new[TicketContext](ticketWorker, addrof contexts[i])
    ..
    loop ready.loadAcquire() != WORKERS: thread.yield() ..
    start.storeRelease(1)
    try thread.joinAll(slices.fromPtr(slices.toPtr(workers), WORKERS))
    ret counter
..

baselineUncontended() !void:
    lock := try mutex.new()
    for i u64 = 0 to UNCONTENDED_ITERATIONS:
        try lock.lock()
        try lock.unlock()
    ..
    try lock.free()
..

threeStateUncontended() void:
    lock := stateNew()
    for i u64 = 0 to UNCONTENDED_ITERATIONS:
        threeStateLock(addrof lock)
        threeStateUnlock(addrof lock)
    ..
..

adaptiveUncontended() void:
    lock := stateNew()
    for i u64 = 0 to UNCONTENDED_ITERATIONS:
        adaptiveLock(addrof lock)
        adaptiveUnlock(addrof lock)
    ..
..

ticketUncontended() void:
    lock := ticketNew()
    for i u64 = 0 to UNCONTENDED_ITERATIONS:
        ticketLock(addrof lock)
        ticketUnlock(addrof lock)
    ..
..

baselineCreation() !void:
    for i u64 = 0 to CREATION_ITERATIONS:
        lock := try mutex.new()
        consumeMutex(addrof lock)
        try lock.free()
    ..
..

threeStateCreation() void:
    for i u64 = 0 to CREATION_ITERATIONS:
        lock := stateNew()
        consumeMutex(addrof lock)
    ..
..

adaptiveCreation() void:
    for i u64 = 0 to CREATION_ITERATIONS:
        lock := stateNew()
        consumeMutex(addrof lock)
    ..
..

ticketCreation() void:
    for i u64 = 0 to CREATION_ITERATIONS:
        lock := ticketNew()
        consumeMutex(addrof lock)
    ..
..

writeResult(out writer.Writer*, name str, creation u64, uncontended u64, contended u64) !void:
    try out.writeAll(name)
    try out.writeAll(" creation_ns=")
    try out.writeUint64(creation)
    try out.writeAll(" uncontended_ns=")
    try out.writeUint64(uncontended)
    try out.writeAll(" contended_ns=")
    try out.writeUint64(contended)
    try out.writeAll("\n")
..

pub main() !void:
    expected := WORKERS * ITERATIONS
    start := time.ticks()
    try baselineCreation()
    baselineCreate := time.ticksToNs(time.elapsedTicks(start))
    start = time.ticks()
    try baselineUncontended()
    baselineSingle := time.ticksToNs(time.elapsedTicks(start))
    start = time.ticks()
    baselineCount := try baselineContended()
    baselineMany := time.ticksToNs(time.elapsedTicks(start))

    start = time.ticks()
    threeStateCreation()
    threeCreate := time.ticksToNs(time.elapsedTicks(start))
    start = time.ticks()
    threeStateUncontended()
    threeSingle := time.ticksToNs(time.elapsedTicks(start))
    start = time.ticks()
    threeCount := try threeStateContended()
    threeMany := time.ticksToNs(time.elapsedTicks(start))

    start = time.ticks()
    adaptiveCreation()
    adaptiveCreate := time.ticksToNs(time.elapsedTicks(start))
    start = time.ticks()
    adaptiveUncontended()
    adaptiveSingle := time.ticksToNs(time.elapsedTicks(start))
    start = time.ticks()
    adaptiveCount := try adaptiveContended()
    adaptiveMany := time.ticksToNs(time.elapsedTicks(start))

    start = time.ticks()
    ticketCreation()
    ticketCreate := time.ticksToNs(time.elapsedTicks(start))
    start = time.ticks()
    ticketUncontended()
    ticketSingle := time.ticksToNs(time.elapsedTicks(start))
    start = time.ticks()
    ticketCount := try ticketContended()
    ticketMany := time.ticksToNs(time.elapsedTicks(start))

    if baselineCount != expected || threeCount != expected || adaptiveCount != expected || ticketCount != expected:
        throw errors.failure("mutex candidate lost protected increments")
    ..
    out := io.stdoutUnbuffered()
    try out.writeAll("creation_iterations=1000000 uncontended_iterations=5000000 workers=")
    try out.writeUint64(WORKERS)
    try out.writeAll(" increments_per_worker=100000\n")
    try writeResult(addrof out, "pthread", baselineCreate, baselineSingle, baselineMany)
    try writeResult(addrof out, "three_state", threeCreate, threeSingle, threeMany)
    try writeResult(addrof out, "adaptive_128", adaptiveCreate, adaptiveSingle, adaptiveMany)
    try writeResult(addrof out, "ticket_futex", ticketCreate, ticketSingle, ticketMany)
..
