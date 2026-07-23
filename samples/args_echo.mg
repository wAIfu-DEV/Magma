mod main

use "std:io"   io

main(args str[]) !void:
    i u64 = 0
    while i < args.count(): defer i = i + 1
        io.writeLn(args[i])
    ..
..
