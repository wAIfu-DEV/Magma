mod main

use "../std/heap.mg" heap
use "../std/io.mg"   io

pub main(args str[]) !void:
    a := heap.allocator()

    out := try io.stdout(a)
    defer out.close()

    out.writeLn("Hello, World!")
..
