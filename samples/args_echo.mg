mod main

use "std:allocators" as allocs
use "std:io" as io

main(args str[]) !void:
    i u64 = 0

    loop i < args.count(): defer i = i + 1
        io.printLn(args[i])
    ..
..
