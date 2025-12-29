mod main
use "io.mg"     io
use "errors.mg" errors

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

pub main(args str[]) !void:
    myStr str = "test"
    myErr error
    myStrt MyStruct

    firstArg str = args[0]
    io.printLn(firstArg)

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
