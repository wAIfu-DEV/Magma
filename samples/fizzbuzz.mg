mod main

use "../std/heap.mg" heap
use "../std/io.mg" io

main() !void:
    a := heap.allocator()

    stdout := try io.stdout(a)
    defer stdout.close()

    out := stdout.writer()

    n u64 = 1

    while n <= 100:
        if n % 15 == 0:
            try out.writeLn("FizzBuzz")
        elif n % 3 == 0:
            try out.writeLn("Fizz")
        elif n % 5 == 0:
            try out.writeLn("Buzz")
        else:
            try out.writeUint64(n)
            try out.writeLn("")
        ..

        n = n + 1
    ..
..
