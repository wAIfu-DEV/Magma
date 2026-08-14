mod main

use "std:fake_alloc" fake
use "std:strings" strings
use "std:io" io
use "std:heap" heap
use "std:thread_pool" tp
use "std:file" file
use "std:context_default" context

pub main() !void:
    ctx := try context.newDefault()

    f := try file.open(ctx.alloc, "main.go", file.mode().read())
    defer f.close()

    reader := try f.reader()

    future := try reader.readAsync(ctx, try f.count())
    contents := try future.await()
    defer contents.free(ctx.alloc)

    io.printLn(contents)
..
