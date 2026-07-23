mod main

Destructible(
    someValue u64
)

destr Destructible.free() void:
    # destr keyword modifier marks a function as being able to satisfy the checker when called.
    # a single struct can have multiple destructor functions.
    # a struct with no destructor will not be checked.
    this.someValue = 0
..

new() $Destructible: # $ marks ownership transfer
    d Destructible
    d.someValue = 42
    ret d # destructible value is returned, implicit ownership transfer
..

consume(x $Destructible) void: # $ before arg type marks ownership transfer (non borrowing)
    defer x.free() # since destructor function is called, checker is satisfied
    # more code
..

assign(x $Destructible) void:
    someSlice := array Destructible[1]
    someSlice[0] = x # ownership is transferred, checker is satisfied

    # WRONG
    x.free() # ownership was already transferred, checker warning!
..

borrow(x Destructible) void:
    # Do stuff with x, checker won't care.

    # WRONG
    x.free() # unless you call a destructor on a borrowed value, now the checker will scream at you.
..

pub main() !void:
    myVal := new() # ownership transferred to myVal
    condition bool = false

    if condition:
        consume(myVal) # functions takes ownership of value, checker is satisfied with this branch
        ret
    ..

    if condition:
        assign(myVal) # same here
        ret 
    ..

    # WRONG
    # Destructor function is not called within this branch,
    # checker will raise a warning!
    ret
..
