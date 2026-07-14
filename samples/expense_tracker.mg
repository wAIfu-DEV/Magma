mod main

use "../std/heap.mg" heap
use "../std/io.mg" io
use "../std/strconv.mg" strconv
use "../std/strings.mg" strings

main() !void:
    a := heap.allocator()

    stdout := try io.stdout(a)
    stdin := try io.stdin(a)

    defer stdout.close()
    defer stdin.close()

    out := stdout.writer()

    labels str[32]
    amounts u64[32]
    count u64 = 0
    running bool = true

    while running:
        try out.writeLn("\n1. Add expense  2. Show total  3. Quit")
        try out.write("Choice: ")
        try stdout.flush()

        choice := try stdin.readLn(a)

        if strings.compare(choice, "1") && count < 32:
            try out.write("Description: ")
            try stdout.flush()
            labels[count] = try stdin.readLn(a)
            try out.write("Amount in cents: ")
            try stdout.flush()
            amountText := try stdin.readLn(a)
            amounts[count] = try strconv.parseUint(amountText)
            strings.free(a, amountText)
            count = count + 1

        elif strings.compare(choice, "2"):
            total u64 = 0
            i u64 = 0

            while i < count:
                try out.write(labels[i])
                try out.write(": ")
                try out.writeUint64(amounts[i])
                try out.writeLn(" cents")
                total = total + amounts[i]
                i = i + 1
            ..

            try out.write("Total: ")
            try out.writeUint64(total)
            try out.writeLn(" cents")
        elif strings.compare(choice, "3"):
            running = false
        ..

        strings.free(a, choice)
    ..

    cleanupIndex u64 = 0

    while cleanupIndex < count:
        strings.free(a, labels[cleanupIndex])
        cleanupIndex = cleanupIndex + 1
    ..
..
