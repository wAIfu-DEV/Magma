mod context

use "std:allocator" alc
use "std:executor" exe

pub Ctx(
    alloc alc.Allocator
    exec exe.Executor
)

pub new(allocator alc.Allocator, executor exe.Executor) Ctx:
    ret Ctx(
        alloc=allocator,
        exec=executor,
    )
..

