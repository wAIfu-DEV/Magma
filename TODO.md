# TODO

## To check

* UTF8 parsing/iterator

## To implement

* fix issue when assigning function from another module to func ptr
* intercept windows argv and make UTF8 before magma.argsToSlice()
* error on using try with non-fallible function
* "panic: interface conversion: types.NodeTypeKind is \*types.NodeTypeAbsolute, not \*types.NodeTypeFunc" when calling inexistant method on struct

## Bugs

* extern func decl without ret type silently fails
* return doesn't check for type of value against type of function ret

