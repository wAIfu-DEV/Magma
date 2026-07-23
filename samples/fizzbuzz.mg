mod main

use "../std/heap.mg" heap
use "../std/io.mg" io

main() !void:
    n u64 = 1

    while n <= 100:
        if n % 15 == 0:
            io.writeLn("FizzBuzz")
        elif n % 3 == 0:
            io.writeLn("Fizz")
        elif n % 5 == 0:
            io.writeLn("Buzz")
        else:
            io.writeUint64(n)
            io.writeLn("")
        ..

        n = n + 1
    ..
..
