# TODO

## To check
- UTF8 parsing/iterator

## To implement
- call nested destructors from owner destructor (struct fields lifetime ended)
- keep signedness on implicit number cast
- implement rfc ref counting system
- prevent empty initialization of pointers and rfc
- intercept windows argv and make UTF8 before magma.argsToSlice()
- file impl for windows
- file impl for unix
- modify io.* funcs to make use of std* file writes
- error on using try with non-fallible function
