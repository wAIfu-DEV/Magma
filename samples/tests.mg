mod main

use "../std/errors.mg" errors
use "../std/cast.mg"   cast
use "../std/io.mg"     io
use "../std/slices.mg" slices
use "../std/heap.mg"   heap
use "../std/utf8.mg"   utf8

main() !void:
    try testErrors()
    try testConds()
    try testVars()
    try testFuncs()
    try testThrowDestructure()
    try testSizeof()
    try testUtf8()

    io.printLn("Passed all tests successfully!")
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
    io.printLn("Testing: errors")

    v bool, e error = retErrBool()
    if errors.errCode(e) == 0:
        throw errors.failure("testErrors: retErrBool returned OK")
    ..

    try retOkBool()
    ret
..

testConds() !void:
    io.printLn("Testing: conditions")

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
    io.printLn("Testing: vars")

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

    v3 u8[3]
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
    io.printLn("Testing: functions")

    # test simple func call

    retZero()

    # test assignment of retval to var

    f0 i64 = retOne()

    if f0 != 1:
        throw errors.failure("from `if f0 != 1:` block")
    ..
..

testThrowDestructure() !void:
    io.printLn("Testing: err retval destructure")

    v bool, e error = retOkBool()

    if v == true:
    else:
        throw errors.failure("destructuring assignment failed: expected v == true")
    ..

    if errors.errCode(e) != 0:
        throw errors.failure("destructuring assignment failed: expected e.code == 1")
    ..

    v2 bool, e2 error = retErrBool()

    if v2 == true:
        throw errors.failure("destructuring assignment failed: expected v2 == false")
    ..

    if errors.errCode(e2) == 0:
        throw errors.failure("destructuring assignment failed: expected e2.code == 0")
    ..
..

testSizeof() !void:
    io.printLn("Testing: sizeof")

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
    io.printLn("Testing: utf8")

    #s str = "This is é test"
    #it utf8.Utf8Iterator = utf8.iterator(s)

    #codepoints utf8.Codepoint* = heap.alloc(strings.countBytes(s) * sizeof utf8.Codepoint)

    #i u64 = 0
    #while it.hasData():
    #    cp utf8.Codepoint = try it.next()
    #    codepoints[i] = cp
    #    i = i+1
    #..

    #if i != 14:
    #    throw errors.failure("from `if i != 14:` block") 
    #..
..
