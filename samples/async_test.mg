mod main

use "../std/fake_alloc.mg" fake
use "../std/strings.mg" strings
use "../std/io.mg" io
use "../std/heap.mg" heap
use "../std/thread_pool.mg" tp
use "../std/file.mg" file

pub main() !void:
    a := heap.allocator()
    out := try io.stdout(a)
    defer out.close()

    pool := try tp.newDefault(a)
    defer pool.close()

    f := try file.open(a, "main.go", file.mode().read())
    defer f.close()
    
    reader := f.reader()

    future := try reader.readAsync(pool, fake.allocator(), f.count())
    contents := try future.await()
    defer strings.free(a, contents)

    out.writeLn(contents)
..
