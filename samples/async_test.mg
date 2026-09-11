mod main

use "std:io" as io
use "std:fmt" as fmt
use "std:file" as file
use "std:time" as time

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
    fmt.str(ctx.alloc, "\nTook (µs): ").uint(took).str("\n").print()

    contents := try future.await()
    defer contents.free()

    io.printLn(contents)
..
