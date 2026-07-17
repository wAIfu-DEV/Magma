mod errors

# A cursor over an error's bounded propagation trace. The newest propagation
# site is returned first. Ring reuse can truncate an old cursor; accessors stay
# safe and isTruncated reports that condition.
Trace(
    handle u64
)

# Returns the error code of an error.
# A code of 0 indicates a successful operation.
# O(1).
# @param e input error
# @returns error code
pub code(e error) u32:
	llvm "  %e0 = extractvalue %type.error %e, 2\n"
    llvm "  ret i32 %e0\n"
..

# Returns the message from an error.
# O(1).
# @param e input error
# @returns error message
pub message(e error) str:
	llvm "  %ep = extractvalue %type.error %e, 0\n"
	llvm "  %el = extractvalue %type.error %e, 3\n"
	llvm "  %el64 = zext i32 %el to i64\n"
	llvm "  %s0 = insertvalue %type.str zeroinitializer, ptr %ep, 0\n"
	llvm "  %s1 = insertvalue %type.str %s0, i64 %el64, 1\n"
	llvm "  ret %type.str %s1\n"
..

# Internal bridge from the built-in error representation.
traceHandle(e error) u64:
    llvm "  %t = call i64 @magma.error.trace(%type.error %e)\n"
    llvm "  ret i64 %t\n"
..

# Returns an allocation-free cursor over the propagation trace.
pub trace(e error) Trace:
    ret Trace(handle=traceHandle(e))
..

traceStatus(handle u64) u32:
    llvm "  %status = call i32 @magma.error.trace.status(i64 %handle)\n"
    llvm "  ret i32 %status\n"
..

pub Trace.isEmpty() bool:
    ret traceStatus(this.handle) != 0
..

# Returns true when part of the trace was overwritten in bounded diagnostic
# storage. Check the terminal cursor after iteration.
pub Trace.isTruncated() bool:
    ret traceStatus(this.handle) == 2
..

traceNext(handle u64) u64:
    llvm "  %next = call i64 @magma.error.trace.next(i64 %handle)\n"
    llvm "  ret i64 %next\n"
..

# Advances toward the error's origin. Calling this on an empty cursor is invalid.
pub Trace.next() Trace:
    ret Trace(handle=traceNext(this.handle))
..

# The following accessors are valid only for a non-empty cursor.
traceFunction(handle u64) str:
    llvm "  %value = call %type.str @magma.error.trace.function(i64 %handle)\n"
    llvm "  ret %type.str %value\n"
..

pub Trace.function() str:
    ret traceFunction(this.handle)
..

traceFile(handle u64) str:
    llvm "  %value = call %type.str @magma.error.trace.file(i64 %handle)\n"
    llvm "  ret %type.str %value\n"
..

pub Trace.file() str:
    ret traceFile(this.handle)
..

traceLine(handle u64) u32:
    llvm "  %value = call i32 @magma.error.trace.line(i64 %handle)\n"
    llvm "  ret i32 %value\n"
..

pub Trace.line() u32:
    ret traceLine(this.handle)
..

traceColumn(handle u64) u32:
    llvm "  %value = call i32 @magma.error.trace.column(i64 %handle)\n"
    llvm "  ret i32 %value\n"
..

pub Trace.column() u32:
    ret traceColumn(this.handle)
..

# Prints all recorded propagation sites without allocating.
pub printTrace(e error) void:
    llvm "  call void @magma.error.printTrace(%type.error %e)\n"
    llvm "  ret void\n"
..

pub is(a error, b error) bool:
    ret code(a) == code(b)
..

pub isOk(e error) bool:
    ret code(e) == 0
..

pub isError(e error) bool:
    ret code(e) != 0
..

# Returns whether an error belongs to a numeric category. Error equality is
# category-based; messages and platform details are not compared.
pub hasCode(e error, expected u32) bool:
    ret code(e) == expected
..

# Returns the error type as a string.
# For example: 0 => "ok", 1 => "unexpected"
# O(1).
# @param e input error
# @returns error type as string
pub toStr(e error) str:
    c u32 = code(e)

    if c == 0:
        ret "ok"
    elif c == 1:
        ret "unexpected"
    elif c == 2:
        ret "invalid argument"
    elif c == 3:
        ret "out of memory"
    elif c == 4:
        ret "end of file"
    elif c == 5:
        ret "would overflow"
    elif c == 6:
        ret "invalid type"
    elif c == 7:
        ret "out of bounds"
    ..
    ret "unknown error"
..

# Creates an error value from a code and message.
# O(1).
makeErr(errorCode u32, msg str) error:
	llvm "  %mp = extractvalue %type.str %msg, 0\n"
	llvm "  %ml64 = extractvalue %type.str %msg, 1\n"
	llvm "  %ml = trunc i64 %ml64 to i32\n"
	llvm "  %e0 = insertvalue %type.error zeroinitializer, ptr %mp, 0\n"
	llvm "  %e1 = insertvalue %type.error %e0, i32 %errorCode, 2\n"
	llvm "  %e2 = insertvalue %type.error %e1, i32 %ml, 3\n"
	llvm "  ret %type.error %e2\n"
..

# Wraps a platform error code without allocating a formatted message. The high
# bit distinguishes native codes from standard-library categories.
pub native(errorCode u32, msg str) error:
    ret makeErr(0x80000000 | errorCode, msg)
..

pub isNative(e error) bool:
    ret (code(e) & 0x80000000) != 0
..

pub nativeCode(e error) u32:
    ret code(e) & 0x7FFFFFFF
..

# Returns an error with code 0 indicating success.
# There isn't any reason to use this function, unless a client code must return 
# an error no matter what.
# O(1).
# @returns error
pub ok() error:
    llvm "  ret %type.error zeroinitializer\n"
..

# Returns an error with code 1 indicating an opaque error.
# O(1).
# @returns error
pub failure(msg str) error:
    ret makeErr(1, msg)
..

# Returns an error with code 2 indicating that the client provided an invalid
# argument to a function or protocol.
# O(1).
# @returns error
pub invalidArgument(msg str) error:
    ret makeErr(2, msg)
..

# Returns an error with code 3 indicating that the system is out of memory.
# O(1).
# @returns error
pub outOfMemory(msg str) error:
    ret makeErr(3, msg)
..

# Returns an error with code 4 indicating the operation hitting the end of a file.
# This may or may not be an error condition, so good documentation is warranted
# if this error is thrown and should be handled by the consumer.
# O(1).
# @returns error
pub endOfFile(msg str) error:
    ret makeErr(4, msg)
..

# Returns an error with code 5 indicating a would-overflow condition.
# O(1).
# @returns error
pub wouldOverflow(msg str) error:
    ret makeErr(5, msg)
..

# Returns an error with code 6 indicating an invalid type.
# O(1).
# @returns error
pub invalidType(msg str) error:
    ret makeErr(6, msg)
..

# Returns an error with code 7 indicating an index being out of bounds of a container.
# O(1).
# @returns error
pub outOfBounds(msg str) error:
    ret makeErr(7, msg)
..
