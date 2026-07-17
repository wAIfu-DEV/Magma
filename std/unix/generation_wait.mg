mod generation_wait_unix

use "../wake.mg" wake_mod

Wait(
    wake wake_mod.Wake
)

pub new() !$Wait:
    value wake_mod.Wake = try wake_mod.new(wake_mod.condition())
    ret Wait(wake=value)
..

pub observe(generation u32*) u32:
    llvm "  %value = load atomic i32, ptr %generation acquire, align 4\n"
    llvm "  ret i32 %value\n"
..

advance(generation u32*) void:
    llvm "  %previous = atomicrmw add ptr %generation, i32 1 release, align 4\n"
    llvm "  ret void\n"
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
    i u64 = 0
    while i < count:
        waiter.wake.notify()
        i = i + 1
    ..
..

pub free(waiter Wait*) void:
    waiter.wake.free()
..
