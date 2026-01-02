mod main
use "../std/errors.mg" errors

main() !void:
    try testConds()
    try testVars()
    try testFuncs()
    try testThrowDestructure()
..

retZero() i64:
    ret 0
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

testConds() !void:
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
..

testVars() !void:
    v0 i64
    v1 i64 = 0
    v2 i64 = retZero()
    v3 u8[3]
    v4 u8*
..

testFuncs() !void:
    retZero()
    f0 = retZero()
..

testThrowDestructure() !void:
    v bool, e error = retOkBool()
    throw e

    if v == true:
    else:
        throw errors.failure("destructuring assignment failed: expected v == true")
    ..

    v2 bool, e2 error = retOkBool()
    throw e2

    if v2 == true:
        throw errors.failure("destructuring assignment failed: expected v2 == false")
    ..

    if errors.errCode(e2) == 0:
        throw errors.failure("destructuring assignment failed: expected e2.code == 1")
    ..
..
