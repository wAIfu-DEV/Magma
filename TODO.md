# TODO

## To check
- UTF8 parsing/iterator

## To implement
- add "internal" IR attribute to private glvars, functions
- fix not being able to assign to args
- fix issue when assigning function from another module to func ptr
- remove destructor implementation
- call nested destructors from owner destructor (struct fields lifetime ended)
- keep signedness on implicit number cast
- implement rfc ref counting system
- prevent empty initialization of pointers and rfc
- intercept windows argv and make UTF8 before magma.argsToSlice()
- file impl for windows
- file impl for unix
- modify io.* funcs to make use of std* file writes
- error on using try with non-fallible function

## Bugs
- extern func decl without ret type silently fails
- return doesn't check for type of value against type of function ret
