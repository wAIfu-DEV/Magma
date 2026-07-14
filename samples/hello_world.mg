mod main

use "../std/heap.mg" heap
use "../std/io.mg"   io

pub main(args str[]) !void:
    a := heap.allocator()

    stdout := try io.stdout(a)
    defer stdout.close()

    out := stdout.writer()

    try out.writeLn("Hello, World!")
..
