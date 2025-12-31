# Magma
## Yet another programming language

Magma is a [enter list of all the paradigms that are hip and cool in this day and age] language.
It appeals to [what makes money currently]

That's cool and all but it doesn't really give you an idea of what the language is about, does it?
Here's some snippets so we can start to boil the frog:

### 1. The classic

```
mod main
use io

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
```


## Warning

As you get more into this language, you'll be glad to notice that we do not have
any of the nicities that makes languages nowadays usable by even the most brain
dead humonculi.

We are proud of that fact, and we are glad that YOU also agree with us.

And if you do not agree, do not worry, we have plenty of time to make multiple very,
very bad decisions that will stick with this project for centuries because we WILL
be too p*ssy to simply do a full rewrite of the entire language.

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

The unpaid intern ran away. The README will stay like this until we fetch a new one from the nearest orphanage.

### Thank you for your patience.
