mod context

use "std:allocator" alc
use "std:executor" exe

pub Ctx(
    procAlloc alc.Allocator
    tempAlloc alc.Allocator
    exec exe.Executor
)

pub noctx new(procAllocator alc.Allocator, tempAllocator alc.Allocator, executor exe.Executor) Ctx:
    ret Ctx(
        procAlloc=procAllocator,
        tempAlloc=tempAllocator,
        exec=executor,
    )
..
