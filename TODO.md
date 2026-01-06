# TODO

## To check
- UTF8 parsing/iterator
- codepoint counter in str (replace `count()` ?)

## To implement
- implement rfc ref counting system
- prevent empty initialization of pointers
- prevent empty initialization of rfc
- fix suspicious bitcasts in IR for HeapAllocator.allocator()
- bitwise operators
- UTF16 parsing/iterator
- intercept windows argv and make UTF8 before magma.argsToSlice()
- comptime conditional for platform-specific code
- file impl for windows
- file impl for unix
- modify io.* funcs to make use of std* file writes
