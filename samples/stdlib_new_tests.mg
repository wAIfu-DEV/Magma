mod main

use "../std/bytes.mg" bytes
use "../std/hash.mg" hash
use "../std/random.mg" random
use "../std/search.mg" search
use "../std/sort.mg" sort
use "../std/strconv.mg" strconv
use "../std/path.mg" path
use "../std/fs.mg" fs
use "../std/hash_map.mg" hash_map
use "../std/heap.mg" heap
use "../std/builder.mg" builder
use "../std/strings.mg" strings
use "../std/errors.mg" errors

compareU64(a u64, b u64) i64:
    if a < b:
        ret -1
    elif a > b:
        ret 1
    ..
    ret 0
..

main() !void:
    xs u8[4]
    xs[0] = 1
    xs[1] = 2
    xs[2] = 3
    xs[3] = 4
    if bytes.contains(xs, 3) == false:
        throw errors.failure("bytes.contains failed")
    ..
    bytes.reverse(xs)
    if xs[0] != 4:
        throw errors.failure("bytes.reverse failed")
    ..

    v3 u64[4]
    v3[0] = 9
    v3[1] = 2
    v3[2] = 7
    v3[3] = 1
    sort.insertion[u64](v3, compareU64)
    idx := try search.binary[u64](v3, 7, compareU64)
    if idx != 2:
        throw errors.failure("sort/search failed")
    ..

    r := random.new(hash.string("seed"))
    if r.bounded(1) != 0:
        throw errors.failure("random bound failed")
    ..

    parsed := try strconv.parseUint("12345")
    if parsed != 12345:
        throw errors.failure("strconv failed")
    ..
    if path.isAbsolute("C:\\tmp") == false:
        throw errors.failure("path failed")
    ..
    a := heap.allocator()
    map := try hash_map.new[u64](a, 2)
    defer map.free()
    try map.set("answer", 42)
    try map.set("one", 1)
    try map.set("two", 2)
    try map.set("three", 3)
    try map.set("four", 4)
    answer := try map.get("answer")
    if answer != 42:
        throw errors.failure("hash map failed")
    ..
    three := try map.get("three")
    if three != 3 || map.count() != 5:
        throw errors.failure("hash map resize failed")
    ..
    ownedInput := try strings.copy(a, "copied-key")
    try map.set(ownedInput, 99)
    inputData := strings.toPtr(ownedInput)
    inputData[0] = 88
    strings.free(a, ownedInput)
    copied := try map.get("copied-key")
    if copied != 99:
        throw errors.failure("hash map key copy failed")
    ..
    try map.delete("copied-key")
    sb := try builder.new(a)
    try sb.appendCopy("standard")
    try sb.append(" library")
    built := try sb.build()
    defer strings.free(a, built)
    if strings.compare(built, "standard library") == false:
        throw errors.failure("builder failed")
    ..
    try sb.reset()
    sb.free()
..
