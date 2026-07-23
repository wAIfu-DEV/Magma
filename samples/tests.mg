mod main

use "../std/allocator.mg" alc
use "../std/errors.mg" errors
use "../std/cast.mg"   cast
use "../std/io.mg"     io
use "../std/writer.mg" writer
use "../std/buffered.mg" buffered
use "../std/strings.mg" strings
use "../std/slices.mg" slices
use "../std/heap.mg"   heap
use "../std/utf8.mg"   utf8

out writer.Writer

main() !void:
    a alc.Allocator = heap.allocator()
    stdout buffered.Writer = try io.stdout(a)
    defer stdout.close()

    out = stdout.writer()

    try testErrors()
    try testConds()
    try testVars()
    try testFuncs()
    try testThrowDestructure()
    try testSizeof()
    try testUtf8()
    try testDeferredLoopControl()

    try out.writeLn("Passed all tests successfully!")
..

retZero() i64:
    ret 0
..

retOne() i64:
    ret 1
..

retOkBool() !bool:
    ret true
..

retErrBool() !bool:
    throw errors.failure("from retErrBool")
    ret true
..

retErr() !void:
    throw errors.failure("from retErr")
..

testErrors() !void:
    try out.writeLn("Testing: errors")

    v bool, e error = retErrBool()
    if errors.code(e) == 0:
        throw errors.failure("testErrors: retErrBool returned OK")
    ..

    try retOkBool()
    ret
..

testConds() !void:
    try out.writeLn("Testing: conditions")

    if true:
    else:
        throw errors.failure("from `if true: ... else:` block")
    ..

    if false:
        throw errors.failure("from `if false:` block")
    ..

    if true == true:
    else:
        throw errors.failure("from `if true == true: ... else:` block")
    ..

    if false == true:
        throw errors.failure("from `if false == true:` block")
    ..

    if false == true:
        throw errors.failure("from `if false == true:` block")
    ..

    if true != true:
        throw errors.failure("from `if true != true:` block")
    ..

    if true != false:
    else:
        throw errors.failure("from `if true != false: ... else:` block")
    ..

    if false != true:
    else:
        throw errors.failure("from `if false != true: ... else:` block")
    ..

    if true && true:
    else:
        throw errors.failure("from `if true && true: ... else:` block")
    ..

    if false && false:
        throw errors.failure("from `if false && false:` block")
    ..

    if true && false:
        throw errors.failure("from `if true && false:` block")
    ..

    if false && true:
        throw errors.failure("from `if true && false:` block")
    ..

    if true || true:
    else:
        throw errors.failure("from `if true || true: ... else:` block")
    ..

    if true || false:
    else:
        throw errors.failure("from `if true || false: ... else:` block")
    ..

    if false || true:
    else:
        throw errors.failure("from `if false || true: ... else:` block")
    ..

    if false || false:
        throw errors.failure("from `if false || false:` block")
    ..

    if 0 > 0:
        throw errors.failure("from `if 0 > 0:` block")
    ..

    if 0 < 0:
        throw errors.failure("from `if 0 < 0:` block")
    ..

    if 0 > 1:
        throw errors.failure("from `if 0 > 1:` block")
    ..

    if 0 < 1:
    else:
        throw errors.failure("from `if 0 < 1: ... else:` block")
    ..

    if 1 > 0:
    else:
        throw errors.failure("from `if 1 > 0: ... else:` block")
    ..

    if 1 < 0:
        throw errors.failure("from `if 1 < 0:` block")
    ..

    if 0 >= 0:
    else:
        throw errors.failure("from `if 0 >= 0: ... else:` block")
    ..

    if 0 <= 0:
    else:
        throw errors.failure("from `if 0 <= 0: ... else:` block")
    ..

    if 0 >= 1:
        throw errors.failure("from `if 0 >= 1:` block")
    ..

    if 0 <= 1:
    else:
        throw errors.failure("from `if 0 <= 1: ... else:` block")
    ..

    if 1 >= 0:
    else:
        throw errors.failure("from `if 1 >= 0: ... else:` block")
    ..

    if 1 <= 0:
        throw errors.failure("from `if 1 <= 0:` block")
    ..
..

testVars() !void:
    try out.writeLn("Testing: vars")

    # check if vars are correctly zero initialized
    
    v0 i64
    if v0 != 0:
        throw errors.failure("from `if v0 != 0:` block")
    ..

    # check if var assignment is correctly handled

    v1 i64 = 1
    if v1 != 1:
        throw errors.failure("from `if v1 != 1:` block")
    ..

    # check if assignment from retval is correct

    v2 i64 = retOne()
    if v2 != 1:
        throw errors.failure("from `if v2 != 1:` block")
    ..

    # test if array elements are correctly zero initialized

    v3 := array u8[3]
    if v3[0] != 0 || v3[1] != 0 || v3[2] != 0:
        throw errors.failure("from `if v3[n] != 0:` block")
    ..

    # test if array count reflects set size

    if slices.count(v3) != 3:
        throw errors.failure("from `if slices.count(v3) != 3:` block")
    ..

    # test if array lvalue assignment are correctly reflected

    v3[0] = 1
    v3[1] = 1
    v3[2] = 1

    if v3[0] != 1 || v3[1] != 1 || v3[2] != 1:
        throw errors.failure("from `if v3[n] != 1:` block")
    ..

    # test if pointers are zero initialized
    # TODO: prevent non-assignment vardef of pointers, effectively preventing accidental nullptr def

    v4 u8*

    if cast.ptou(v4) != 0:
        throw errors.failure("from `if v4 != 0:` block")
    ..
..

testFuncs() !void:
    try out.writeLn("Testing: functions")

    # test simple func call

    retZero()

    # test assignment of retval to var

    f0 i64 = retOne()

    if f0 != 1:
        throw errors.failure("from `if f0 != 1:` block")
    ..
..

testThrowDestructure() !void:
    try out.writeLn("Testing: err retval destructure")

    v, e := retOkBool()

    if v == true:
    else:
        throw errors.failure("destructuring assignment failed: expected v == true")
    ..

    if errors.code(e) != 0:
        throw errors.failure("destructuring assignment failed: expected e.code == 1")
    ..

    v2 bool, e2 error = retErrBool()

    if v2 == true:
        throw errors.failure("destructuring assignment failed: expected v2 == false")
    ..

    if errors.code(e2) == 0:
        throw errors.failure("destructuring assignment failed: expected e2.code == 0")
    ..
..

testSizeof() !void:
    try out.writeLn("Testing: sizeof")

    s0 u64 = sizeof u64
    if s0 != 8:
        throw errors.failure("from `if s0 != 8:` block")
    ..

    s1 u64 = sizeof u32
    if s1 != 4:
        throw errors.failure("from `if s1 != 4:` block")
    ..
    
    s2 u64 = sizeof u16
    if s2 != 2:
        throw errors.failure("from `if s2 != 2:` block")
    ..

    s3 u64 = sizeof u8
    if s3 != 1:
        throw errors.failure("from `if s3 != 1:` block")
    ..

    s4 u64 = sizeof ptr
    if s4 != 8:
        try out.write("Size of pointer: ")
        try out.writeUint64(s4)
        try out.writeLn("")
        throw errors.failure("from `if s4 != 8:` block")
    ..

    s5 u64 = sizeof u8*
    if s5 != 8:
        throw errors.failure("from `if s5 != 8:` block")
    ..

    s6 u64 = sizeof slice
    if s6 != 16:
        throw errors.failure("from `if s6 != 16:` block")
    ..

    s7 u64 = sizeof u8[]
    if s7 != 16:
        throw errors.failure("from `if s7 != 16:` block")
    ..
..

testUtf8() !void:
    try out.writeLn("Testing: utf8")
    a := heap.allocator()

    s0 str = "This is é test"
    nc u64 = try utf8.countCodepoints(s0)

    if nc != 14:
        try out.write("s0 codepoints: ")
        try out.writeUint64(nc)
        try out.writeLn("")
        throw errors.failure("from `if nc != 14:` block") 
    ..

    nb u64 = strings.countBytes(s0)

    if nb != 15:
        try out.write("s0 bytes: ")
        try out.writeUint64(nb)
        try out.writeLn("")
        throw errors.failure("from `if nb != 15:` block") 
    ..

    wide := try utf8.utf8To16(a, "ASCII")
    defer slices.free(a, wide)
    roundTrip := try utf8.utf16to8(a, wide)
    defer strings.free(a, roundTrip)
    if strings.compare(roundTrip, "ASCII") == false:
        throw errors.failure("UTF-16 to UTF-8 ASCII round trip failed")
    ..
..

increment(value u64*) void:
    value[0] = value[0] + 1
..

testDeferredLoopControl() !void:
    try out.writeLn("Testing: deferred loop control")

    sum u64 = 0
    i u64 = 0
    while i < 3:
        i = i + 1
        if i == 2:
            continue
        ..
        sum = sum + i
    ..
    if sum != 4:
        throw errors.failure("nested continue did not skip the loop body")
    ..

    count u64 = 0
    while count < 3:
        count = count + 1
        if count == 2:
            break
        ..
    ..
    if count != 2:
        throw errors.failure("nested break did not exit the loop")
    ..

    deferred u64 = 0
    iteration u64 = 0
    while iteration < 2:
        defer increment(addrof deferred)
        iteration = iteration + 1
        if iteration == 1:
            continue
        ..
        if iteration == 2:
            break
        ..
    ..
    if deferred != 2:
        throw errors.failure("loop control did not run deferred statements")
    ..
..
