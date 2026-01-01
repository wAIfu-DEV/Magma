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

Here is a sweet little example prepared by our unpaid intern to demonstrate:

```
mod main
use "std:io" io

MyStruct(
    whatever i32
)

main() void:
    my_var MyStruct
    my_var.whatever = 420

    io.printInt(cast.i32to64(my_var.whatever)) # this prints 420 to console, INSANE

    my_array str[3]
    my_array[0] = "bruh"
    my_array[1] = "sigma"
    my_array[2] = "skibidi"
..
```

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
