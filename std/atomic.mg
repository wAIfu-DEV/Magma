mod atomic
# Sequentially consistent atomic numeric values for cross-thread coordination.
# @warning Do not copy an atomic value after publishing it to other threads.

# Atomic operations use sequential consistency, matching the default ordering
# of C++ atomics. Do not copy an atomic value after publishing it to threads.

# Atomically accessed unsigned 8-bit value.
# @warning Initialize with newU8 before sharing its address between threads.
pub U8(
    value u8
)

# Atomically accessed unsigned 32-bit value.
# @warning Initialize with newU32 before sharing its address between threads.
pub U32(
    value u32
)

# Atomically accessed unsigned 64-bit value with sequential, acquire/release,
# and relaxed operations for counters and synchronization state.
pub U64(
    value u64
)

# Atomically accessed signed 64-bit value.
pub I64(
    value i64
)

# Atomically accessed f64 value supporting load, store, and exchange.
pub F64(
    value f64
)

# Creates an atomic u8 with the supplied initial value.
# @complexity O(1)
# @example
#   flag := atomic.newU8(0)
pub newU8(value u8) U8:
    ret U8(value=value)
..

# Creates an atomic u32 with the supplied initial value.
# @complexity O(1)
# @example
#   state := atomic.newU32(0)
pub newU32(value u32) U32:
    ret U32(value=value)
..

# Creates an atomic u64 with the supplied initial value.
# @complexity O(1)
# @example
#   requests := atomic.newU64(0)
pub newU64(value u64) U64:
    ret U64(value=value)
..

# Creates an atomic i64 with the supplied initial value.
# @complexity O(1)
# @example
#   balance := atomic.newI64(0)
pub newI64(value i64) I64:
    ret I64(value=value)
..

# Creates an atomic f64 with the supplied initial value.
# @complexity O(1)
# @example
#   latest := atomic.newF64(0.0)
pub newF64(value f64) F64:
    ret F64(value=value)
..

# Replaces the value with sequential consistency.
# @complexity O(1)
# @example
#   flag.store(1)
U8.store(value u8) void:
    llvm "  store atomic i8 %value, ptr %this seq_cst, align 1\n"
    llvm "  ret void\n"
..

# Reads the value with sequential consistency.
# @complexity O(1)
# @example
#   ready := flag.load() != 0
U8.load() u8:
    llvm "  %value = load atomic i8, ptr %this seq_cst, align 1\n"
    llvm "  ret i8 %value\n"
..

# Atomically replaces the value and returns its previous value.
# @complexity O(1)
# @example
#   wasSet := flag.exchange(1) != 0
U8.exchange(value u8) u8:
    llvm "  %previous = atomicrmw xchg ptr %this, i8 %value seq_cst, align 1\n"
    llvm "  ret i8 %previous\n"
..

# Reads with acquire ordering, observing writes published before a matching release.
# @complexity O(1)
# @example
#   ready := flag.loadAcquire() != 0
U8.loadAcquire() u8:
    llvm "  %value = load atomic i8, ptr %this acquire, align 1\n"
    llvm "  ret i8 %value\n"
..

# Stores with release ordering, publishing prior writes to acquiring threads.
# @complexity O(1)
# @example
#   flag.storeRelease(1)
U8.storeRelease(value u8) void:
    llvm "  store atomic i8 %value, ptr %this release, align 1\n"
    llvm "  ret void\n"
..

# Atomically adds value and returns the value from before the addition.
# @complexity O(1)
# @warning Arithmetic wraps at the u8 boundary.
# @example
#   previous := counter.fetchAdd(1)
U8.fetchAdd(value u8) u8:
    llvm "  %previous = atomicrmw add ptr %this, i8 %value seq_cst, align 1\n"
    llvm "  ret i8 %previous\n"
..

# Atomically subtracts value and returns the value from before subtraction.
# @complexity O(1)
# @warning Arithmetic wraps at the u8 boundary.
# @example
#   previous := counter.fetchSub(1)
U8.fetchSub(value u8) u8:
    llvm "  %previous = atomicrmw sub ptr %this, i8 %value seq_cst, align 1\n"
    llvm "  ret i8 %previous\n"
..

# Replaces the value with sequential consistency.
# @complexity O(1)
# @example
#   state.store(2)
U32.store(value u32) void:
    llvm "  store atomic i32 %value, ptr %this seq_cst, align 4\n"
    llvm "  ret void\n"
..

# Reads the value with sequential consistency.
# @complexity O(1)
# @example
#   current := state.load()
U32.load() u32:
    llvm "  %value = load atomic i32, ptr %this seq_cst, align 4\n"
    llvm "  ret i32 %value\n"
..

# Atomically replaces the value and returns its previous value.
# @complexity O(1)
# @example
#   previous := state.exchange(2)
U32.exchange(value u32) u32:
    llvm "  %previous = atomicrmw xchg ptr %this, i32 %value seq_cst, align 4\n"
    llvm "  ret i32 %previous\n"
..

# Reads with acquire ordering, observing writes published before a matching release.
# @complexity O(1)
# @example
#   current := state.loadAcquire()
U32.loadAcquire() u32:
    llvm "  %value = load atomic i32, ptr %this acquire, align 4\n"
    llvm "  ret i32 %value\n"
..

# Stores with release ordering, publishing prior writes to acquiring threads.
# @complexity O(1)
# @example
#   state.storeRelease(1)
U32.storeRelease(value u32) void:
    llvm "  store atomic i32 %value, ptr %this release, align 4\n"
    llvm "  ret void\n"
..

# Atomically adds value with sequential consistency and returns the previous value.
# @complexity O(1)
# @warning Arithmetic wraps at the u32 boundary.
# @example
#   ticket := counter.fetchAdd(1)
U32.fetchAdd(value u32) u32:
    llvm "  %previous = atomicrmw add ptr %this, i32 %value seq_cst, align 4\n"
    llvm "  ret i32 %previous\n"
..

# Adds with release ordering and returns the previous value, publishing prior writes.
# @complexity O(1)
# @warning Arithmetic wraps at the u32 boundary.
# @example
#   previous := counter.fetchAddRelease(1)
U32.fetchAddRelease(value u32) u32:
    llvm "  %previous = atomicrmw add ptr %this, i32 %value release, align 4\n"
    llvm "  ret i32 %previous\n"
..

# Atomically subtracts value with sequential consistency and returns the previous value.
# @complexity O(1)
# @warning Arithmetic wraps at the u32 boundary.
# @example
#   previous := counter.fetchSub(1)
U32.fetchSub(value u32) u32:
    llvm "  %previous = atomicrmw sub ptr %this, i32 %value seq_cst, align 4\n"
    llvm "  ret i32 %previous\n"
..

# Subtracts with acquire-release ordering and returns the previous value.
# @complexity O(1)
# @warning Arithmetic wraps at the u32 boundary.
# @example
#   wasLast := references.fetchSubAcqRel(1) == 1
U32.fetchSubAcqRel(value u32) u32:
    llvm "  %previous = atomicrmw sub ptr %this, i32 %value acq_rel, align 4\n"
    llvm "  ret i32 %previous\n"
..

# Replaces the value with sequential consistency.
# @complexity O(1)
# @example
#   counter.store(0)
U64.store(value u64) void:
    llvm "  store atomic i64 %value, ptr %this seq_cst, align 8\n"
    llvm "  ret void\n"
..

# Reads the value with sequential consistency.
# @complexity O(1)
# @example
#   total := counter.load()
U64.load() u64:
    llvm "  %value = load atomic i64, ptr %this seq_cst, align 8\n"
    llvm "  ret i64 %value\n"
..

# Atomically replaces the value and returns its previous value.
# @complexity O(1)
# @example
#   batch := counter.exchange(0)
U64.exchange(value u64) u64:
    llvm "  %previous = atomicrmw xchg ptr %this, i64 %value seq_cst, align 8\n"
    llvm "  ret i64 %previous\n"
..

# Reads atomically without synchronizing other memory accesses.
# @complexity O(1)
# @warning Use only when atomicity is needed but cross-variable ordering is not.
# @example
#   approximate := counter.loadRelaxed()
U64.loadRelaxed() u64:
    llvm "  %value = load atomic i64, ptr %this monotonic, align 8\n"
    llvm "  ret i64 %value\n"
..

# Reads with acquire ordering, observing writes published before a matching release.
# @complexity O(1)
# @example
#   published := state.loadAcquire()
U64.loadAcquire() u64:
    llvm "  %value = load atomic i64, ptr %this acquire, align 8\n"
    llvm "  ret i64 %value\n"
..

# Stores atomically without publishing preceding memory accesses.
# @complexity O(1)
# @warning Use only when cross-variable ordering is not required.
# @example
#   counter.storeRelaxed(0)
U64.storeRelaxed(value u64) void:
    llvm "  store atomic i64 %value, ptr %this monotonic, align 8\n"
    llvm "  ret void\n"
..

# Stores with release ordering, publishing prior writes to acquiring threads.
# @complexity O(1)
# @example
#   state.storeRelease(1)
U64.storeRelease(value u64) void:
    llvm "  store atomic i64 %value, ptr %this release, align 8\n"
    llvm "  ret void\n"
..

# Atomically adds value with sequential consistency and returns the previous value.
# @complexity O(1)
# @warning Arithmetic wraps at the u64 boundary.
# @example
#   id := counter.fetchAdd(1)
U64.fetchAdd(value u64) u64:
    llvm "  %previous = atomicrmw add ptr %this, i64 %value seq_cst, align 8\n"
    llvm "  ret i64 %previous\n"
..

# Atomically adds without synchronizing other memory and returns the previous value.
# @complexity O(1)
# @warning Arithmetic wraps; relaxed ordering cannot publish other data.
# @example
#   previous := metrics.fetchAddRelaxed(1)
U64.fetchAddRelaxed(value u64) u64:
    llvm "  %previous = atomicrmw add ptr %this, i64 %value monotonic, align 8\n"
    llvm "  ret i64 %previous\n"
..

# Atomically subtracts value with sequential consistency and returns the previous value.
# @complexity O(1)
# @warning Arithmetic wraps at the u64 boundary.
# @example
#   previous := counter.fetchSub(1)
U64.fetchSub(value u64) u64:
    llvm "  %previous = atomicrmw sub ptr %this, i64 %value seq_cst, align 8\n"
    llvm "  ret i64 %previous\n"
..

# Replaces the signed value with sequential consistency.
# @complexity O(1)
# @example
#   balance.store(0)
I64.store(value i64) void:
    llvm "  store atomic i64 %value, ptr %this seq_cst, align 8\n"
    llvm "  ret void\n"
..

# Reads the signed value with sequential consistency.
# @complexity O(1)
# @example
#   current := balance.load()
I64.load() i64:
    llvm "  %value = load atomic i64, ptr %this seq_cst, align 8\n"
    llvm "  ret i64 %value\n"
..

# Atomically replaces the signed value and returns its previous value.
# @complexity O(1)
# @example
#   previous := balance.exchange(0)
I64.exchange(value i64) i64:
    llvm "  %previous = atomicrmw xchg ptr %this, i64 %value seq_cst, align 8\n"
    llvm "  ret i64 %previous\n"
..

# Atomically adds value and returns the value from before the addition.
# @complexity O(1)
# @warning Signed overflow wraps according to the underlying integer operation.
# @example
#   previous := balance.fetchAdd(delta)
I64.fetchAdd(value i64) i64:
    llvm "  %previous = atomicrmw add ptr %this, i64 %value seq_cst, align 8\n"
    llvm "  ret i64 %previous\n"
..

# Atomically subtracts value and returns the value from before subtraction.
# @complexity O(1)
# @warning Signed overflow wraps according to the underlying integer operation.
# @example
#   previous := balance.fetchSub(cost)
I64.fetchSub(value i64) i64:
    llvm "  %previous = atomicrmw sub ptr %this, i64 %value seq_cst, align 8\n"
    llvm "  ret i64 %previous\n"
..

# Atomically replaces the floating-point value with sequential consistency.
# @complexity O(1)
# @example
#   latest.store(measurement)
F64.store(value f64) void:
    llvm "  %bits = bitcast double %value to i64\n"
    llvm "  store atomic i64 %bits, ptr %this seq_cst, align 8\n"
    llvm "  ret void\n"
..

# Reads the floating-point value with sequential consistency.
# @complexity O(1)
# @example
#   measurement := latest.load()
F64.load() f64:
    llvm "  %bits = load atomic i64, ptr %this seq_cst, align 8\n"
    llvm "  %value = bitcast i64 %bits to double\n"
    llvm "  ret double %value\n"
..

# Atomically replaces the floating-point value and returns its previous value.
# This exchanges the exact IEEE-754 bit pattern without numeric conversion.
# @complexity O(1)
# @example
#   previous := latest.exchange(measurement)
F64.exchange(value f64) f64:
    llvm "  %bits = bitcast double %value to i64\n"
    llvm "  %previous.bits = atomicrmw xchg ptr %this, i64 %bits seq_cst, align 8\n"
    llvm "  %previous = bitcast i64 %previous.bits to double\n"
    llvm "  ret double %previous\n"
..
