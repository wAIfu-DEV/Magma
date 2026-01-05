mod main

use "../std/cast.mg"      cast
use "../std/io.mg"        io
use "../std/errors.mg"    errors
use "../std/slices.mg"    slices
use "../std/allocator.mg" alloc
use "../std/heap.mg"      heap

MyNestedStruct(
    field u32
)

MyStruct(
    field u32,
    other f32,
    nested MyNestedStruct,
)

MyStruct.member(first u32) !void:
..

MyStruct.destructor() void:
    io.printLn("CALLED DESTRUCTOR")
..

func2(arg i32) i32:
    ret arg
..

func1() i32:
    ret 0
..

func3() str:
    ret "lol"
..

throwing(isThrowing bool) !i32:
    if isThrowing:
        throw errors.failure("from throwing func")
    ..
    ret 2
..

pub main(args str[]) !void:
    defer io.printLn("DEFERRED!!")

    defer:
        io.printLn("DEFERRED IN BLOCK!!")
        io.printLn("YIPPEE!!")
    ..

    allocOnHeap u8* = try heap.alloc(8)

    byteSlice u8[] = slices.fromPtr(allocOnHeap, 8)

    defer heap.free(allocOnHeap)

    myVal i32, myE error = throwing(true)
    errCode u32 = errors.errCode(myE)

    if errCode != 0:
        io.printLn("err destructure worked!!")
        io.print("err code: ")
        io.printInt(errCode)
        io.printLn("")

        io.print("err msg: ")
        io.print(errors.errMsg(myE))
        io.printLn("")

        io.print("uninit val: ")
        io.printInt(myVal)
        io.printLn("")
    else:
        io.printLn("Something went wrong: throwing(true) returned error OK")
    ..

    if true:
        varWithDestructor MyStruct
        if true:
            anotherVarWithDestructor MyStruct
        ..
        # varWithDestructor destructor called here
    ..

    myStr str = "test"
    myErr error
    myStrt MyStruct

    myArr str[3]

    i i64 = 0
    while i != slices.count(args):
        io.print("arg")
        io.printInt(i)
        io.print(": ")
        io.printLn(args[i])
        i = i+1
    ..

    test i64 = -1258
    test2 i32 = cast.i64to32(test)

    io.print("value of (i64)-1258 after cast to i32: ")
    io.printInt(cast.i32to64(test2))
    io.printLn("")

    myOperand i32 = try throwing(false)

    io.print("throwing did not throw on first call\n")

    try throwing(true)

    myAdd f64 = cast.itof(0 + myOperand)

    io.print("0 + 2 = ")
    io.printFloat(myAdd)
    io.printLn("")

    myInt u32 = myStrt.nested.field
    io.printUint(myInt)
    io.printLn("")

    myInt = 0
    myStrt.nested.field = 542
    io.printUint(myStrt.nested.field)
    io.printLn("")

    myBoolTrue bool = true
    myBoolFalse bool = false

    io.printLn("Running program...")

    io.print("Hello, ")
    io.print("World!\n")

    io.print("myBoolTrue: ")
    io.printBool(myBoolTrue)
    io.printLn("")

    io.print("myBoolFalse: ")
    io.printBool(myBoolFalse)
    io.printLn("")

    io.printInt(-45)
    io.printLn("")

    io.printUint(45)
    io.printLn("")

    if false:
        io.printLn("false is true??")
    ..

    if true:
        io.printLn("true is true :)")
        someVar bool = false
    ..

    # simple conditional
    if myBoolTrue:
        io.printLn("cond1 success")
    ..

    # chained conditional
    if myBoolFalse:
        io.printLn("cond2 failure")
    elif myBoolTrue:
        io.printLn("cond2 success")
    ..

    # chained conditional with catch-all clause
    if myBoolFalse:
        io.printLn("cond3 failure")
    elif myBoolFalse:
        io.printLn("cond3 failure")
    else:
        io.printLn("cond3 success")
    ..

    # chained conditional with catch-all clause
    if myBoolFalse:
        io.printLn("cond4 failure")
    elif myBoolFalse:
        io.printLn("cond4 failure")
    elif myBoolTrue:
        io.printLn("cond4 success")
    else:
        io.printLn("cond4 failure")
    ..

    throw myErr

    io.printLn("Did not throw.")

    val i32 = func2(func1())

    throw errors.invalidArgument("hi from the main function")

    io.printLn("Did not throw.")
    ret
..
