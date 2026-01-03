# TODO

- add labels in-between deferred statements and jump to the correct ones depending on the return statement
- move error context out of return value, only keep errcode or fat pointer
- comptime conditional for platform-specific code
- file impl for windows
- file impl for unix
- modify io.* funcs to make use of std* file writes
- UTF8 parsing/iterator
- codepoint counter in str (replace `count()` ?)
- intercept windows argv and make UTF8 before magma.argsToSlice()
- struct method call impl
- destructor methods impl
