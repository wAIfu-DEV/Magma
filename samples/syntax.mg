mod main

use errors
use io
use heap
use gc

# comment line
# second comment line

# global var defintion
my_var u64 = 0

# global const defintion
MY_CONST u64 : 0

# function defintion
myFunc (first_arg u64, second_arg f32) !bool:

    if first_arg == 0:
        io.printLn("first branch")
        throw errors.invalidArgument()
    elif second_arg < 0.0:
        io.printLn("second branch")
        throw errors.invalidArgument()
    else:
        io.printLn("third branch")
    ..

    for i u64 = 0..first_arg:
        io.printUint(i)
    ..
    ret true
..

# struct definition
MyStruct (
    first_field u64,
    second_field f32,
    third_field str,
)

# struct member func defintion
MyStruct.memberFunc () void:
    this.third_field = "some str value"
    this.first_field = this.third_field.count
..

allocs () !void:
    on_heap *MyStruct = try heap.alloc(sizeof(MyStruct))
    defer heap.free(on_heap)
..

pub main (args str[]) !void:

    # auto-throw on error
    ret_val bool = try myFunc(0, 0.0)

    # handle error, equivalent to previous
    ret_val2 bool, e err = myFunc(0, 0.0)
    if e.code != errors.ok().code:
        throw e
    ..

    my_struct MyStruct = MyStruct(first_field=0, second_field=5.0) # rest is 0-init

    # defered statements will be registered and executed on return
    defer my_struct.third_field = ""

    # defer multiple statements
    defer:
        my_struct.third_field = ""
        io.printLn("end of main")
    ..

    my_ptr *MyStruct = &my_struct
    my_ptr.second_field = 50.0

    # at this point my_struct.first_field == 50

    my_ptr.memberFunc()

    for i u64 = 0..args.count:
        io.printLn(args[i])
    ..
    ret
..
