# Magma
## Yet another programming language

Magma is a [enter list of all the paradigms that are hip and cool in this day and age] language.
It appeals to [what jobs makes money currently], especially as a replacement to COBOL.

That's cool and all but it doesn't really give you an idea of what the language is about, does it?
Here's some snippets so we can start boiling the frog (you):

### 1. The classic

```
mod main
use "std:io" io

main() void:
    io.printLn("Hello, World!")
..

# wow, nearly less boilerplate than Java!!
```

### 2. The classic but more usable

```
mod main
use "std:io" io

main(args str[]) !void:
    io.printLn("Hello, World!")
..

# on the edge of unreasonable
```

## Features

Let's take a look at what Magma will allow you to do at the current and very present time.

### 1. The basics

1. define functions
2. define variables
3. call functions
4. define structs
5. assignment
6. nested member access
7. array indexed access
8. first class error system
9. while loops
10. WIP conditionals

Here is a sweet little example prepared by our unpaid intern to demonstrate:

```
mod main        # first line of file, must define a module name
use "std:io" io # import other modules and give it an alias `io`
use "std:errors" errors

# struct definition
MyStruct(
    whatever i32
)

# main function definition
main() !void:
    my_var MyStruct       # var definition `<name> <type>`, no assignment will result in zero initialization
    my_var.whatever = 420 # assignment to deeply nested member field

    # casting is required (on assignment), in expressions implicit casting may occur without loss of precision or truncations
    my_bigint i64 = cast.i32to64(my_var.whatever)

    io.printInt() # this prints 420 to console

    my_array str[3]         # defines a slice pointing to a stack allocated array of size 3 * sizeof(str)
    my_array[0] = "bruh"    # assignment to first index
    my_array[1] = "sigma"
    my_array[2] = "skibidi"

    my_retval i32 = try mightThrow() # try will result in automatic error re-throwing if the result of a function call is an error

    throw errors.outOfMemory("") # throw will conditionally return the given error, only if the error is non-ok
    throw errors.ok()            # this is a no-op and will cause no control flow changes
    # this keyword will reduce boilerplate for error handling (think go's err != nil { return err })

    # both `try` and `throw` require the current function to be throwing function, indicated by `!` before the return type

    # errors can be inspected using result destructuring (only available for error results (for now))
    my_reval i32, err error = mightThrow()
    code i32 = errors.code(err)
    
    if code == 1: # they really should make enums for error code
        # handle specific error type
    elif code != 0:
        # handle all other errors
    ..
..
```

## Get started

Building the compiler and compiling magma code will require:

1. Golang runtime: [https://go.dev/](https://go.dev/)

Required to build the compiler frontend.

2. Clang C compiler*: [https://releases.llvm.org/download.html](https://releases.llvm.org/download.html)

Currently required for going from LLVM IR -> binary, this will likely change in the future.

Run either of the FULL_COMP_*.bat scripts on windows to get the full compiled executable.
You might have to modify the script to change `clang.exe` to an absolute path depending
on where you installed it, and if the folder is in the system env variables.

If you are on linux, please install windows specifically for this project then proceed
to uninstall it (windows) after every use. This is crucial to the installation process.

## Politics

As you get more into this language, you'll be glad to notice that we do not have
any of the niceties that makes languages nowadays usable even by the most human-adjacent
homunculi.

Our philosophy is one of exclusivity, gatekeeping, excellence and irreverence.
We think that in an age of sanitized minds and speech, performative everything,
the new normal ought to be a little more human and rough.
We are proud to think that way, and we are glad that YOU also agree with us.

Now you may feel the need to ask why we feel the need to talk about philosophy and
politics in the README of a damn programming language, and that would be a QUITE
VALID QUESTION, but apparently every project needs to have politics involved now,
so take it or leave it, we won't really care that much.

---

## The end.

The unpaid intern ran away. The README will stay like this until we fetch a new one from the nearest orphanage.

### Thank you for your patience.
