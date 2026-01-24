mod main

use "../std/cast.mg"      cast
use "../std/io.mg"        io
use "../std/errors.mg"    errors
use "../std/slices.mg"    slices
use "../std/strings.mg"   strings
use "../std/allocator.mg" alloc
use "../std/heap.mg"      heap
use "../std/writer.mg"    writer
use "../std/memory.mg"    memory
# use "../std/utf8.mg"      utf8

use "../std/file.mg"      file

MyNestedStruct(
    field u32
)

MyStruct(
    field u32,
    other f32,
    nested MyNestedStruct,
)

MyStruct.member(out writer.Writer) !void:
    this.field = 45
    out.writeLn("CALLED METHOD")
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
        throw errors.errFailure("from throwing func")
    ..
    ret 2
..

pub main(args str[]) !void:
    out writer.Writer = io.stdout()

    defer out.writeLn("DEFERRED!!")

    defer:
        out.writeLn("DEFERRED IN BLOCK!!")
        out.writeLn("YIPPEE!!")
    ..

    firstArray u8[16]
    secondArray u8[16]
    
    iii u64 = 0
    while iii < 16:
        firstArray[iii] = cast.u64to8(iii) + 97
        iii = iii + 1
    ..

    memory.copy(slices.toPtr(firstArray), slices.toPtr(secondArray), 16)

    out.writeLn(strings.fromPtrNoCopy(slices.toPtr(secondArray), 16))

    allocOnHeap u8* = try heap.alloc(8)

    byteSlice u8[] = slices.fromPtr(allocOnHeap, 8)

    defer heap.free(allocOnHeap)

    myVal i32, myE error = throwing(true)
    errCode u32 = errors.code(myE)

    if errCode != 0:
        out.writeLn("err destructure worked!!")
        out.write("err code: ")
        out.writeInt64(errCode)
        out.writeLn("")

        out.write("err msg: ")
        out.write(errors.message(myE))
        out.writeLn("")

        out.write("uninit val: ")
        out.writeInt64(myVal)
        out.writeLn("")
    else:
        out.writeLn("Something went wrong: throwing(true) returned error OK")
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

    myStrt.member(out)

    if myStrt.field != 45:
        out.writeLn("member function failed to modify state of owner")
        ret
    ..

    myArr str[3]

    i i64 = 0
    while i != slices.count(args):
        out.write("arg")
        out.writeInt64(i)
        out.write(": ")
        out.writeLn(args[i])
        i = i+1
    ..

    test i64 = -1258
    test2 i32 = cast.i64to32(test)

    out.write("value of (i64)-1258 after cast to i32: ")
    out.writeInt64(cast.i32to64(test2))
    out.writeLn("")

    myOperand i32 = try throwing(false)

    out.write("throwing did not throw on first call, good\n")

    try throwing(true)

    myAdd f64 = cast.itof(0 + myOperand)

    out.write("0 + 2 = ")
    out.writeFloat64(myAdd, 3)
    out.writeLn("")

    myInt u32 = myStrt.nested.field
    out.writeUint64(myInt)
    out.writeLn("")

    myInt = 0
    myStrt.nested.field = 542
    out.writeUint64(myStrt.nested.field)
    out.writeLn("")

    myBoolTrue bool = true
    myBoolFalse bool = false

    out.writeLn("Running program...")

    out.write("Hello, ")
    out.write("World!\n")

    out.write("myBoolTrue: ")
    out.writeBool(myBoolTrue)
    out.writeLn("")

    out.write("myBoolFalse: ")
    out.writeBool(myBoolFalse)
    out.writeLn("")

    out.writeInt64(-45)
    out.writeLn("")

    out.writeUint64(45)
    out.writeLn("")

    if false:
        out.writeLn("false is true??")
    ..

    if true:
        out.writeLn("true is true :)")
        someVar bool = false
    ..

    # simple conditional
    if myBoolTrue:
        out.writeLn("cond1 success")
    ..

    # chained conditional
    if myBoolFalse:
        out.writeLn("cond2 failure")
    elif myBoolTrue:
        out.writeLn("cond2 success")
    ..

    # chained conditional with catch-all clause
    if myBoolFalse:
        out.writeLn("cond3 failure")
    elif myBoolFalse:
        out.writeLn("cond3 failure")
    else:
        out.writeLn("cond3 success")
    ..

    # chained conditional with catch-all clause
    if myBoolFalse:
        out.writeLn("cond4 failure")
    elif myBoolFalse:
        out.writeLn("cond4 failure")
    elif myBoolTrue:
        out.writeLn("cond4 success")
    else:
        out.writeLn("cond4 failure")
    ..

    throw myErr

    out.writeLn("Did not throw.")

    val i32 = func2(func1())

    throw errors.errInvalidArgument("hi from the main function")

    out.writeLn("Did not throw.")
    ret
..
