mod main

use "../std/heap.mg"    heap
use "../std/json.mg"    json
use "../std/io.mg"      io

main() !void:
    a := heap.allocator()
    
    stdout := io.stdout(a)
    defer stdout.close()

    out := stdout.writer()

    obj := json.newObject(a)
    defer obj.free()

    try obj.set("test", json.numberInt(4))
    try obj.set("head", json.null())

    try obj.write(out, 5)
..
