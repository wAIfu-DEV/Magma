mod context_default

use "std:context" context
use "std:allocator" alc
use "std:heap" heap
use "std:thread_pool" tp
use "std:footgun" fg

allocator alc.Allocator
thread_pool tp.ThreadPool

pub newDefault() !context.Ctx:
    allocator = heap.allocator()
    thread_pool = try tp.newDefault(allocator)
    defer fg.drop[tp.ThreadPool](move thread_pool)

    ret context.new(allocator, thread_pool.executor())
..
