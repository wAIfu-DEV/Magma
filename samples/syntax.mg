mod main

use "../std/errors.mg" errors
use "../std/io.mg"     io
use "../std/heap.mg"   heap
use "../std/slices"    slices

# comment line
# second comment line

# global var defintion
my_var u64 = 0

# global const/immutable defintion
MY_CONST u64 : 0

# function defintion
myFunc(first_arg u64, second_arg f32) !bool:

    if first_arg == 0:
        io.printLn("first branch")
        throw errors.invalidArgument()
    elif second_arg < 0.0:
        io.printLn("second branch")
        throw errors.invalidArgument()
    else:
        io.printLn("third branch")
    ..

    for i u64 = 0 -> first_arg:
        io.printUint(i)
    ..
    ret true
..

# struct definition
# implicit definition of constructor function of the same name
MyStruct(
    first_field u64,
    second_field f32,
    third_field str,
)

# struct member func defintion
MyStruct.memberFunc() void:
    this.third_field = "some str value"
    this.first_field = this.third_field.count
..

allocs() !void:
    heap_ptr MyStruct* = try heap.alloc(sizeof(MyStruct))
    defer heap.free(heap_ptr)

    rfcnt_ptr MyStruct$ = rfc MyStruct() # moves a copy of MyStruct to heap
    # ref counted '$' vars are freed once every references fall out of scope

    rfcnt_ptr2 MyStruct$ = rfcnt_ptr # adds another reference, until rfcnt_ptr2 falls out of scope
..

pub main(args str[]) !void:

    # auto-throw on error
    # only works from within functions with a return type of !T
    ret_val bool = try myFunc(0, 0.0)

    # handle error, equivalent to previous
    ret_val2 bool, e error = myFunc(0, 0.0)
    if errors.errCode(e) != 0:
        throw e # throw itself is conditional, if err == ok then control flow is resumed
    ..

    throw errors.ok() # is a no-op

    my_struct MyStruct = MyStruct(first_field=0, second_field=5.0) # rest is 0-init

    # defered statements will be registered and executed on return
    defer my_struct.third_field = ""

    # defer multiple statements
    defer:
        my_struct.third_field = ""
        io.printLn("end of main")
    ..

    my_ptr MyStruct* = &my_struct
    my_ptr.second_field = 50.0

    # at this point my_struct.first_field == 50

    my_ptr.memberFunc()

    for i u64 = 0 -> slices.count(args):
        io.printLn(args[i])
    ..
    ret
..
