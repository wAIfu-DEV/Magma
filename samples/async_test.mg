mod main

use "std:io" io
use "std:fmt" fmt
use "std:file" file
use "std:time" time

pub main() !void:
    f := try file.open("main.go", file.mode().read())
    defer f.close()

    io.print("Waiting for file read: ")

    future := try f.reader().readAsync(f.count())

    start := time.ticks()
    loop try future.isDone() == false:
        io.print("#")
    ..

    took := time.elapsedUs(start)
    fmt.str(ctx.procAlloc, "\nTook (µs): ").uint(took).str("\n").print()

    contents := try future.await()
    defer contents.free(ctx.procAlloc)

    io.printLn(contents)
..
