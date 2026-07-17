mod address_wait_unix

use "../wake.mg" wake_mod

# Unix fallback. Wake's counted condition variable closes the race between the
# atomic status check and entering the native wait.
Wait(
    wake wake_mod.Wake
)

pub new() !$Wait:
    value wake_mod.Wake = try wake_mod.new(wake_mod.condition())
    ret Wait(wake=value)
..

pub wait(waiter Wait*, status u32*) !void:
    try waiter.wake.wait()
..

pub wake(waiter Wait*, status u32*) void:
    waiter.wake.notify()
..

pub free(waiter Wait*) void:
    waiter.wake.free()
..
