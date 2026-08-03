# History

## Prelude

My story of writing programming languages started around 2014-2016 I believe,
at that time I got interested in how to tinker with program and game configuration files
which evolved into me trying to make my own game.

At first I used Unreal Engine 4's blueprint system, which provided a nice way to
get into programming, and while the blueprint system is limited, not knowing anything
about what I was doing was likely more limiting.

Some projects I made during this time included a top down 2D RPG platform similar
to dragon quest. I remember it being the time were I started looking for alternatives.
This led me to Godot, which had quite a lot of influence on Magma's design.

Between 2016 and 2020 I experimented with a lot of programming languages, always
trying to challenge myself with projects of absurd magnitude (that is if they were to be finished, all of them weren't).
Some curiously recurring projects were:
- Programming or scripting language transpilers
- Operating systems, desktop environments
- CLI Text editors

Some languages I toyed with during this period include:
- GDScript
- C++
- C# / VisualBasic .NET (for windows apps)
- C
- Python
- JavaScript/TypeScript

In 2020 I got a job that required me to use Excel, and I used my free time to explore
and optimize macros written in the oh so accursed VBA.
This experience is what led to my first attempt at writing my own programming language.

## Previous attempts

### ARES

The first language I ever designed was named ARES, and strong of my relationship with VBA,
it claimed a lot of very bad ideas from it.

Here is an example:
```
Fn main
    Print("Hello, World!")
    Input()
End
```

Which would successfully be transpiled to:
```
#include "ARES.h"
int main();
int main() {
ARES::Print( "Hello, World!");
};

```

The code on top would be transpiled, without checks into 1-to-1 representation as C++
as you can imagine, this leads to very unstable code.

The transpiler itself is an embrassement so I won't dwell any longer on it.

### ARES

Somehow ARES returned.
I present to you ARES, second of name:

```
// Global variables
arr<str> source_file_lines
arr<str> parsed_lines_first_pass

fn init
    // Cool graphical logo
    Print("            ___   ____  ____ ")
    Print("   -|- //| || \\\\ ||___ //__  ")
    Print("      //|| ||_// ||       \\\\ ")
    Print("     // || || \\\\ ||___ \\\\_// ")

    // Define default compiler variables
    str default_directory = "C:\\ARES"
    str compiler = "g++"
    str in_file = Str(default_directory, "\\out\\out.ares.cpp")
    str out_file = Str(default_directory, "\\out\\out.ares.exe")
    str compiler_command = Str("-g \"", in_file, "\" -o \"", out_file, "\" -static")

    // Get input from user
    Print("")
    Print("ARES source code (.ares) absolute path:")
    str source_file_path = Input()
    Print("")
    Print("Show parser/transpiler results? (debug purposes). [Y/n]")
    str show_translator_results = Input()
    bol show_results = false

    if show_translator_results == "Y"
        show_results = true
    ..if

    // Read source file lines
    file source_file
    source_file.Open(source_file_path)
    source_file_lines = source_file.ReadLines()

    // Start parsing
    Parser.FirstPass()
..fn

space ARES_CONST
    // Indicators
    str string_bound = "\""
    str arg_start = "("
    str arg_end = ")"
    str block_start = "{"
    str block_end = "}"
    str bracket_start = "["
    str bracket_end = "]"
    str arg_delim = ","
    str token_delim = " "
    // Keywords
    str kw_func = "Fn"
    str kw_end = "End"
    str kw_return = "Return"
    str kw_if = "If"
    str kw_else = "Else"
    str kw_loop = "Loop"
    str kw_obj = "Obj"
    str kw_space = "Space"
    str kw_let = "Let"
    str kw_call = "Call"
    // Map ARES types to CPP types
    map<str,str> types = {{"Str", "ARES::Types::String"},
                        {"Int", "long int"},
                        {"Sht", "int"},
                        {"Lng", "long long int"},
                        {"Dbl", "double"},
                        {"Flt", "float"},
                        {"Dci", "long double"},
                        {"File", "ARES::Types::File"},
                        {"Array", "ARES::Types::Array<#>"},
                        {"Map", "ARES::Types::Map<#,#>"}}
..space

space Parser
fn FirstPass

lng line_count = source_file_lines.Count() - 1
lng line_nb = -1
lng char_count
lng char_nb
str current_line
str current_char
str temp_line
bol is_string = False
bol discard_next = False
// Separate strings before line formatting
loop line_nb < line_count
    line_nb++
    is_string = False
    discard_next = False
    temp_line = ""
    current_char = ""
    current_line = source_file_lines[line_nb]
    char_count = current_line.Length() - 1
    char_nb = -1
    // Loop through every characters in line
    loop char_nb < char_count
        char_nb++
        current_char = current_line[char_nb]
        if char_nb != 0
            if current_line[char_nb + 1] == "/" And current_char == "/"
                discard_next = True
            end
        end
        if char_nb != char_count - 1
            if current_char == "\"" And (current_line[char_nb - 1] == "\\") == False
                is_string = True
            end
        end
        temp_line.Append(current_char)
    end
    Print(temp_line)
    end
end
```

Does it look good? No. Does it compile? Of course not.

But at this point some you may observe some similarities with Magma, notably the '..'
closing tag followed by the name of the closed scope.

Why does it switch from '..' to 'end'? Nobody knows at this point, but it is very likely
that I was experimenting to see which syntax would be best.

### Gravel

In 2023 I began to work on Gravel, a low, low level programming language.

```
name main;

// import directives
import "std";
import "asm";

module Program {
    set[readonly] name: *i8 = "Hello, World!\n";
    set comment_test: *i8 = "Hello, //World!\n";

    proc start: i16 {
        std.printf("Started %s.\n", Program.name);
    }
};

// entry point
proc main: i16 (set argc: i16, set argv: **i8) {
    set val: i16 = 0xb800;
    asm.mov(asm.eax, val);
    std.print(hello_world);

    for i: i8 = 0..100 {
        std.printf("  %d\n", i);
    }

    ret;
}
```

This came after I spent some time working with assembly, I believe for one of the
operating system projects I worked on.
Evidently I didn't suffer enough and this language can only be explained be me
having some sort of masochistic tendency.

This language was meant to be "C but lower level", but then I realized I never
wanted that to begin with.

Magma takes inspiration in only three ways:
- being low level
- borrowing 'ret' from LLVM IR
- having top-of-file module name declaration, likely stolen from Go

Ultimately I stopped working on the compiler early and moved on to the next
language, continuing the tradition of repurposing old names.

### ARES

Would you look at that, it's ARES again, anyone is surprised?

Another random file I found while digging was one I wrote in 2023:

```
// Most likely
// V No unnecessary keywords		Reduces bloat
// V Type after label				Makes code self-documenting and reduces importance of type
// V No unnecessary column			Reduces clutter
main(args str[]) i32 {
	variable_name i32 = 0;
	ret variable_name;
}

// C/C++ style
// V No unnecessary keywords
// X Type before label
// V No unnecessary column
i32 main(str[] args) {
	i32 variable_name = 0;
	ret variable_name;
}

// Modified Rust/Ts style
// V No unnecessary keywords
// V Type after label
// X Column after label
main(args: str[]): i32 {
	variable_name: i32 = 0;
	ret variable_name;
}

// Rust style
// X Keywords
// V Type after label
// X Column after label
fn main(args: str[]): i32 {
	variable_name: i32 = 0;
	ret variable_name;
}

// TypeScript style
// X Keywords
// V Type after label
// X Column after label
function main(args: str[]): i32 {
	variable_name: i32 = 0;
	return variable_name;
}

```

This file was also used as a platform to test ideas and compare syntaxes, it is
newer than the next examples, but the .ares extension means I still had the
intention to make this damn language happen.

You'll notice that the very first syntax example is quite similar to the current
syntax of Magma with the exception of curly brackets instead of the current : and ..

What would become the slice type is also present here with `str[]`.

Another newer syntax showcase file:

```
// Pointer Syntax

main() i32 {
    stack_int i32 = 0;                // Stack allocation
    heap_int i32 uadrs = alloc[i32]();// Heap allocation to unique address
    heap_flt f32 adrs = alloc[f32](); // Heap allocation to address
    (load heap_int) = 51;             // Deref and set value in heap
    ret 0;                            // heap_int freed, heap_flt leaked
}

// Most likely
// adrs keyword after type
// Easy to understand for new devs
// Deobfuscates the idea of a pointer
my_pointer i32 adrs
// unique pointer
my_pointer i32 uadrs

// Also likely
// ptr keyword after type
// Easy to understand for devs already familiar with pointers
my_pointer i32 ptr
// unique pointer
my_pointer i32 uptr

// ptr abstracted as templated type
// More consitent with the rest of types (good thing ?)
my_pointer ptr[i32]
my_pointer uptr[i32]

// C/C++ style variants
// Less easy to understand but more concise
my_pointer *i32
// unique ptr (lol)
my_pointer std::unique_ptr<i32>
```

This one introduces the 'ptr' type, but instead of being equivalent to void*, it
acts more like a keyword replacing the '\*' in C's T*

Also, generics make their big debut with the \[T\] syntax! Congrats to them, we're all so proud.


### GravelScript

Far from the previous low level language of the same etymology, GravelScript goes the complete opposite direction.

```
func Main;
    call Print, "Started";
    fetch startTime, UnixTime;

    array valueArr, 0, 25, 85, 152220, 0.0, 4850.0;
    fetch arrBound, Len, valueArr;
    call PrintItems, valueArr, 0, arrBound;

    call PrintTime, startTime;
    call Print, "Ended";
end;

func PrintItems, loopArray, i, bound;
    fetch loopItem, At, loopArray, i;
    call Print, "Item:", loopItem;

    fetch i, IncI, i;
    if Equals, i, bound;
        return;
    endif;
    call PrintItems, loopArray, i, bound;
end;

func PrintTime, startTime;
    fetch currTime, UnixTime;
    fetch execTime, SubI, currTime, startTime;
    call Print, "Exec(ms):", execTime;
end;
```

Created in 2024, this might be the most experimental language I ever made, and
basically no features from this made it to Magma.
The interpreter is functional and can be found [here](https://github.com/wAIfu-DEV/GravelScript)

### Lithium

Now this one's interesting as it is the closest relative
to Magma (2025), and shares a name relating to fire and rock.

Here's a sample:
```
struct option<T>:
    var T maybeValue
    var bool isNone
..

fn s32 main(var string[] args):
    printLine("Hello, World!")
    printLine("arg0: " + args.at(0))

    var string[] arr
    arr.pushBack("1")
    arr.pushBack("2")
    arr.pushFront("0")

    if true:
        printLine("Condition is true")
        printLine("second condition")
    elif true:
        printLine("Condition2 is true")
    else:
        printLine("Condition3 is true")
    ..

    var s8 i0 = 0
    while i0 < 5:
        printLine("Hello")
        i0 += 1
    ..

    var s64 bound = arr.length
    var s64 i1    = 0
    while i1 < bound:
        printLine(arr.at(i1))
        i1 += 1
    ..

    var u8[] bytes # type can also be array<s8>
    bytes.pushBack(10)

    var string str2
    str2.fromBytes(bytes)
    print(str2) # \n

    var option<string> front = arr.popFront()
    printLine(front.maybeValue)  # 0
    printLine(arr.at(0))         # 1

    var option<string> res = readFile("std.lh")

    if res.isNone:
        printLine("Failed to read file.")
        return 1
    ..

    printLine(res.maybeValue)

    var pair<s32, string> shellResult = shell("where python")
    print(shellResult.second)
    
    printLine("Success")
..

# Complex types examples

fn void test():
..

fn array<s64> test2():
..

fn s32[] test3():
..

fn array<string>[] test4():
..
```

Meant to transpile to both TypeScript and C++, this is
one of the more mature frontends I made.
Unlike previous attempts, this one generates a AST,
then lowers the AST to the target language.
This is in contrast to the very naive 1-to-1 translation used by the previous transpiled attempts.

While different from Magma's syntax, some elements are strikingly similar:
- : and .. scope delimiters
- T[] arrays (slices)

Ultimately this project was abandoned too. Not because it couldn't be made, but because I felt the self-imposed
humiliation of having to piggy-back on other, better languages.

## Not a language, but close

Right before starting to work on Magma, I had the sudden ambition
to write a better and safer C standard library.
The XSTD project ended up being pivotal for the future of Magma since
it essentially cemented several design decisions and
proved their effectiveness as I used the library in order
to make [Wade32](https://github.com/wAIfu-DEV/Wade32), a small QEMU floppy OS.

Some of the most important parts:
- result types (leading to fallible functions)
- error codes, largely inspired by Godot's model
- Vtable-based abstractions (Allocator, Writer)
- Ownership explicitness
- module-based OS abstraction

Here's a simple example:
```c
#include "../../xstd.h"

int main(void)
{
    Allocator *a = default_allocator();
    io_println("Echo started, type \"quit\" to exit.");

    while (true)
    {
        // Read line from stdin
        ResultOwnedStr inputRes = io_read_line(a);
        assert_ok(inputRes.error, "Could not read from stdin.");

        // Handle exit
        if (string_equals(inputRes.value, "quit"))
            return 0;

        io_println(inputRes.value);

        // Free owned memory
        a->free(a, inputRes.value);
    }
}
```

And another one because I feel generous:
```c
#include "../../xstd.h"

i32 main(i32 argc, String* argv)
{
    // CLI args, specifically on windows get ANSI encoded, which is retarded,
    // and break file paths which may have utf8 chars in them.
    // This functions ensures the args are UTF-8 encoded regardless of platform.
    String* args = io_args_utf8(argc, argv);

    if (argc != 2)
    {
        crash_print("More or less than 1 argument.", 1);
        return 1;
    }

    Allocator* a = default_allocator();

    String filePath = args[1];
    
    if (!file_exists(filePath))
    {
        ResultOwnedStr res = string_concat(a, "File does not exist: ", filePath);
        crash_print(res.error.code ? "File does not exist." : res.value, 1);
        return 1;
    }

    ResultFile fRes = file_open(filePath, EnumFileOpenMode.READ);
    if (fRes.error.code)
    {
        crash_print_error(fRes.error, "Cannot open file.", 1);
        return 1;
    }

    File* f = &fRes.value;

    while(true)
    {
        ResultOwnedStr chunkRes = file_read_str(a, f, 8);
        if (chunkRes.error.code)
        {
            crash_print_error(chunkRes.error, "Read error.", 1);
            return 1;
        }

        OwnedStr chunk = chunkRes.value;
        io_print(chunk);

        a->free(a, chunk);

        if (file_is_eof(f))
            break;
    }
    
    file_close(f);
    return 0;
}
```

## Magma

On December 9th 2025, I posted the first Magma commit to the repo.

Here is the syntax I proposed at the time:

```
# Language syntax for Magma lang

# first statement in file should be module name declaration
# other files in the project will be able to import it using "link main"
# mind you, main should not be imported by any files.
module main

# imports linked module and adds the module to the global scope of this file
link io
link error

# This is a line comment

SOME_CONST :: 5 # defines a compile-time constant

returnsFive() i64:
    return 5
..

# func definition
funcName(arg0: i64, arg1: i64) !i64:

    # indentation is not significant, but newlines are.

    val, err := failingFunc(arg = returnsFive())
    throw err # throw conditionally returns err if not OK
              # requires ! prefix on return type

    val, throw := failingFunc(arg0) # or throw the error directly (syntax sugar)
                                    # this is trying to fix the Go error model
                                    # while keeping the best of it.


    #val: io.SomeType = 0 # TODO: handle composite names in types
    io.printLn(val)

    some_math := (5 + 2 * 5) # should be 15
    return some_math
..

failingFunc(arg: i64) !i64:
    throw error.genericFailed()
    return 0
..

# struct definition
StructName(
    field0: f64,
    field1: u32,
)

# members can be defined only in the same file as struct def
StructName.doSomething() void:
    this.field1 = true # this refers to struct ptr
..

# underlying error type representation
# subject to change, but from testing this is the most sensible one,
# and bridges the gap with Exceptions.
Error(
    message: str,   # optional more descriptive error message
    trace: str[5],  # throw will "secretly" push call site data (func name, line) to this array.
                    # error printing or panics will print this trace.
                    # this means more overhead, but only on the error path, not the valid one.
    traceIdx: u8,
    code: u8,
)

wrongValue(field0: f64) bool:
    if field0 == 0.0:
        return true
    else:
        return false
    ..
..

# struct constructor (not intrinsic)
makeStruct(field0: f64) !StructName:
    if wrongValue(field0):
        throw error.invalidParameter()
    ..
    some_value: u32 = 0

    return StructName(
        field0 = field0,
        field1 = some_value,
    )
..

structConsumer() !void:
    my_struct, throw := makeStruct(field0=someValue)

    my_struct.field0 = 54
    my_struct.doSomething()
..

arrays() !void:
    my_arr := byte[15](fill=0) # stack array definition with utility arguments

    arrSize := my_arr.count # arrays have count field unlike C arrays which
                            # are basically pointers.
                            # I assume those arrays will be implemented using a fat pointer

    # unsafe/fast access
    val := my_arr[0]
    
    # safer bound checked access
    val2, ok := my_arr[arrSize+1]
    if !ok:
        throw error.outOfBounds() # this will be thrown
    ..

    throw error.ok() # this is useless, but with code of 0 (OK) error will not be propagated
                     # and control flow will move past this throw

    # throw is essentially syntax sugar for:
    # if err.code != 0:
    #     if err.traceIdx < err.trace.count:
    #         err.trace[err.traceIdx] = "file:XX.X ln:XX;col:XX fn:XXX()" # debug info added at compile time
    #         err.traceIdx += 1
    #     ..
    #     return (<0 initialized ret type>, err)
    # ..
..


# main function definition
main(args: str[]) !void:

    ret1: bool = wrongValue(0.0)
    #ret0: i64, err: Error = funcName(0, 0)

    val: i32 = 5
    val = 2 # reassignment

    my_ptr: *i32 = &val # similar to C pointers, however they cannot be 0 initialized,
                        # or else the compiler screams at you.
                        # This should make safe code faster since we won't have to check for null ptr.
    
    my_struct := StructName()
    struct_ptr := &my_struct # address of operator
    # struct_ptr := &StructName() # short hand for previous
                                  # this creates a pointer to a temporary stack alloc var
                                  # pointer is invalid once underlying temp falls out of scope
    
    struct_ptr.field0 = 54 # . is equivalent to -> operator in C
                           # this will mutate my_struct
    
    struct_cpy := *struct_ptr # ptr deref that is not assigned to will always return a copy
    *struct_ptr = struct_cpy  # ptr deref that is assigned to will mutate underlying memory

    for i := 0...args.count:
        io.printLn("arg:", args[i])
    ..

    return
..

```

Most of the features showcased here made it to the current
implementation of the language, with some having been implemented very recently at the time of writing this, such as:
- io.printLn(): you used to have to get a writer to write to console using io.stdout()
- Struct constructors
- constants, though they do not have the same syntax

If you want to learn more about the design decisions, go [here](https://github.com/wAIfu-DEV/Magma/tree/main/docs/DESIGN.md)
