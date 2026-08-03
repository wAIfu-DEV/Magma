mod main

use "std:allocators" allocs
use "std:io" io

main(args str[]) !void:
    i u64 = 0

    while i < args.count(): defer i = i + 1
        io.printLn(args[i])
    ..
..
