mod main

use "../std/heap.mg" heap
use "../std/io.mg"   io

pub main(args str[]) !void:
    a := heap.allocator()
    stdout := try io.stdout(a)
    out := stdout.writer()
    out.writeLn("Hello, World!")
..
