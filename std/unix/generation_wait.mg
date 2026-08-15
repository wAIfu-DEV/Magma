mod generation_wait_unix
# Unix generation-counter wait backend used by thread synchronization APIs.

use "std:wake" wake_mod

pub Wait(
    wake wake_mod.Wake
)

pub new() !$Wait:
    value wake_mod.Wake = try wake_mod.new(wake_mod.condition())
    ret Wait(wake=move value)
..

pub observe(generation u32*) u32:
    # SAFETY: this audited implementation injects the required low-level IR.
    unsafe:
        llvm "  %value = load atomic i32, ptr %generation acquire, align 4\n"
        llvm "  ret i32 %value\n"
    ..
..

advance(generation u32*) void:
    # SAFETY: this audited implementation injects the required low-level IR.
    unsafe:
        llvm "  %previous = atomicrmw add ptr %generation, i32 1 release, align 4\n"
        llvm "  ret void\n"
    ..
..

pub signal(generation u32*) void:
    advance(generation)
..

pub wait(waiter Wait*, generation u32*, observed u32) !void:
    if observe(generation) != observed:
        ret
    ..
    try waiter.wake.wait()
..

pub wakeOne(waiter Wait*, generation u32*) void:
    advance(generation)
    waiter.wake.notify()
..

pub wakeAll(waiter Wait*, generation u32*, count u64) void:
    advance(generation)
    for i u64 = 0 to count:
        waiter.wake.notify()
    ..
..

pub free(waiter Wait*) void:
    # SAFETY: free consumes the embedded wake object through its owning Wait pointer.
    unsafe:
        waiter.wake.free()
    ..
..
