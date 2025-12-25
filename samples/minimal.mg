mod main
use "io.mg" io

myStruct(
    field u32,
    other f32,
)

myStruct.member(first u32) !void:
..

myStruct.destructor() void:
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

pub main() !void:
    myStr str = "test"
    myErr error

    io.printLn("Running program...")

    io.print("Hello, ")
    io.print("World!\n")

    io.printInt(-45)
    io.printLn("")

    io.printUint(45)
    io.printLn("")

    throw myErr

    io.printLn("Did not throw.")

    val i32 = func2(func1())
    ret
..
