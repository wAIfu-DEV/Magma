mod main

use "std:fake_alloc" fake
use "std:strings" strings
use "std:io" io
use "std:heap" heap
use "std:thread_pool" tp
use "std:file" file
use "std:async" async

pub main() !void:
    a := heap.allocator()

    pool := try tp.newDefault(a)
    defer pool.close()
    as := async.new(pool, fake.allocator())

    f := try file.open(a, "main.go", file.mode().read())
    defer f.close()
    
    future := try as.read(f.reader(), try f.count())
    contents := try future.await()
    defer contents.free(a)

    io.writeLn(contents)
..
