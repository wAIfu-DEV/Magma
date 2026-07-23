mod thread_impl_unix
# Unix native-thread backend used by the portable thread module.


use "std:c" c
@platform("linux", "freebsd", "netbsd", "openbsd")
link "pthread"

use "std:cast" cast
use "std:errors" errors
use "std:heap" heap

ext ext_pthread_create pthread_create(thread u64*, attributes ptr, startRoutine (ptr) u64, argument ptr) c.int
ext ext_pthread_join   pthread_join(thread u64, result ptr) c.int
ext ext_sched_yield    sched_yield() c.int

pub Thread(
    handle u64
    launch Launch*
)

Launch(
    entry (ptr) u64
    context ptr
    completed u8
)

storeCompleted(value u8*) void:
    llvm "  store atomic i8 1, ptr %value release, align 1\n"
    llvm "  ret void\n"
..

loadCompleted(value u8*) u8:
    llvm "  %done = load atomic i8, ptr %value acquire, align 1\n"
    llvm "  ret i8 %done\n"
..

threadMain(raw ptr) u64:
    launch Launch* = raw
    result u64 = launch.entry(launch.context)
    storeCompleted(addrof launch.completed)
    ret result
..

pub spawn(entry (ptr) u64, context ptr) !$Thread:
    if entry == none:
        throw errors.invalidArgument("thread entry is null")
    ..

    launch Launch* = try heap.alloc(sizeof Launch)
    launch.entry = entry
    launch.context = context
    launch.completed = 0

    handle u64 = 0
    code i32 = ext_pthread_create(addrof handle, none, threadMain, launch)
    if code != 0:
        heap.free(launch)
        throw errors.native(cast.u64to32(cast.itou(cast.i32to64(code))), "pthread_create failed")
    ..
    ret Thread(handle=handle, launch=launch)
..

pub isFinished(thread Thread*) !bool:
    if thread.handle == 0:
        throw errors.invalidArgument("thread is not joinable")
    ..
    ret loadCompleted(addrof thread.launch.completed) != 0
..

pub join(thread Thread*) !bool:
    if thread.handle == 0:
        throw errors.invalidArgument("thread is not joinable")
    ..
    code i32 = ext_pthread_join(thread.handle, none)
    if code != 0:
        throw errors.native(cast.u64to32(cast.itou(cast.i32to64(code))), "pthread_join failed")
    ..
    heap.free(thread.launch)
    thread.handle = 0
    thread.launch = none
    ret true
..

pub yield() void:
    ext_sched_yield()
..
