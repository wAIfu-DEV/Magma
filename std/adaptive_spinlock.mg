mod adaptive_spinlock
# Windows adaptive TTAS lock optimized for very short critical sections.

use "std:atomic" atomic
use "std:locker" locker
@platform("windows")
use "std:win/thread_impl" thread_impl

@platform("linux", "android", "ios", "darwin", "freebsd", "netbsd", "openbsd")
use "std:unix/thread_impl" thread_impl

pub AdaptiveSpinLock impl locker.Locker(
    state atomic.U32
)

compareExchange(value atomic.U32*, expected u32, desired u32) u32:
    unsafe:
        llvm "  %result = cmpxchg ptr %value, i32 %expected, i32 %desired acquire monotonic, align 4\n"
        llvm "  %observed = extractvalue { i32, i1 } %result, 0\n"
        llvm "  ret i32 %observed\n"
    ..
..

loadRelaxed(value atomic.U32*) u32:
    unsafe:
        llvm "  %observed = load atomic i32, ptr %value monotonic, align 4\n"
        llvm "  ret i32 %observed\n"
    ..
..

storeRelease(value atomic.U32*, desired u32) void:
    unsafe:
        llvm "  store atomic i32 %desired, ptr %value release, align 4\n"
        llvm "  ret void\n"
    ..
..

cpuRelax() void:
    unsafe:
        llvm "  call void asm sideeffect \"pause\", \"~{memory}\"()\n"
        llvm "  ret void\n"
    ..
..

lockSlow(value AdaptiveSpinLock*) void:
    loop true:
        for spin u64 = 0 to 64:
            if loadRelaxed(addrof value.state) == 0:
                if compareExchange(addrof value.state, 0, 1) == 0:
                    ret
                ..
            ..
            cpuRelax()
        ..
        thread_impl.yield()
    ..
..

pub new() AdaptiveSpinLock:
    ret AdaptiveSpinLock(state=atomic.newU32(0))
..

AdaptiveSpinLock.lock() void:
    if compareExchange(addrof this.state, 0, 1) == 0:
        ret
    ..
    lockSlow(this)
..

AdaptiveSpinLock.unlock() void:
    storeRelease(addrof this.state, 0)
..

AdaptiveSpinLock.lockRaw() !void:
    this.lock()
..

AdaptiveSpinLock.unlockRaw() !void:
    this.unlock()
..

AdaptiveSpinLock.releaseRaw() void:
..

AdaptiveSpinLock.locker() locker.Locker:
    ret this.proto()
..
