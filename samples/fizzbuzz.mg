mod main

use "std:heap.mg" heap
use "std:io.mg" io
use "std:fmt" fmt

main() !void:
    a := heap.allocator()
    n u64 = 1

    loop n <= 100:
        if n % 15 == 0:
            io.printLn("FizzBuzz")
        elif n % 3 == 0:
            io.printLn("Fizz")
        elif n % 5 == 0:
            io.printLn("Buzz")
        else:
            fmt.printf(fmt.new(a).uint(n).str("\n"))
        ..

        n = n + 1
    ..
..
