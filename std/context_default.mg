mod context_default

use "std:context" context
use "std:allocator" alc
use "std:heap" heap
use "std:thread_pool" tp
use "std:atomic" atomic
use "std:executor" executor
use "std:errors" errors
use "std:scratch_alloc" scratch_alloc

initFlag atomic.U8
allocator alc.Allocator
thread_pool tp.ThreadPool
temporary scratch_alloc.Scratch

pub noctx newDefault() !context.Ctx:
    bootstrapAllocator := heap.allocator()
    ctx = context.new(bootstrapAllocator, bootstrapAllocator, executor.null())
    state := initFlag.load()
    if state == 1:
        throw errors.invalidArgument("default context initialization is reentrant")
    ..
    if state == 3:
        throw errors.invalidArgument("default context initialization previously failed")
    ..
    if state != 2:
        initFlag.store(1)
        onerror initFlag.store(3)
        allocator = bootstrapAllocator
        temporary = try scratch_alloc.newDefault()
        thread_pool = try tp.newDefault(allocator)
        initFlag.store(2)
    ..
    ret context.new(allocator, temporary.allocator(), thread_pool.executor())
..

pub noctx newNull() context.Ctx:
    ret context.new(alc.null(), alc.null(), executor.null())
..
