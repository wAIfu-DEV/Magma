mod context

use "std:allocator" alc
use "std:executor" exec

pub Ctx(
    allocator alc.Allocator
    executor exec.Executor
)

