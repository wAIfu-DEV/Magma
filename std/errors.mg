mod errors
# Error construction, classification, and bounded propagation-trace inspection.

pub const ERR_OK u32 = 0
pub const ERR_FAIL u32 = 1
pub const ERR_INVALID_ARG u32 = 2
pub const ERR_OUT_OF_MEMORY u32 = 3
pub const ERR_END_OF_FILE u32 = 4
pub const ERR_WOULD_OVERFLOW u32 = 5
pub const ERR_INVALID_TYPE u32 = 6
pub const ERR_OUT_OF_BOUNDS u32 = 7
pub const ERR_NOT_FOUND u32 = 8
pub const ERR_CANCELLED u32 = 9
pub const ERR_WOULD_BLOCK u32 = 10
pub const ERR_TIMED_OUT u32 = 11
pub const ERR_CONNECTION_RESET u32 = 12
pub const ERR_CONNECTION_REFUSED u32 = 13
pub const ERR_ADDRESS_IN_USE u32 = 14

# A cursor over an error's bounded propagation trace. The newest propagation
# site is returned first. Traversal is bounded because ring reuse may replace
# old parent links; accessors stay safe and isTruncated reports that bound.
pub Trace(
    handle u64
)

# Internal bridge from the built-in error representation.
traceHandle(e error) u64:
    # SAFETY: this audited implementation injects the required low-level IR.
    unsafe:
        llvm "  %t = call i64 @magma.error.trace(%type.error %e)\n"
        llvm "  ret i64 %t\n"
    ..
..

# Returns an allocation-free cursor over the propagation trace.
# @complexity O(1)
# @example
#   cursor := errors.trace(failure)
pub trace(e error) Trace:
    ret Trace(handle=traceHandle(e))
..

traceStatus(handle u64) u32:
    # SAFETY: this audited implementation injects the required low-level IR.
    unsafe:
        llvm "  %status = call i32 @magma.error.trace.status(i64 %handle)\n"
        llvm "  ret i32 %status\n"
    ..
..

# Reports whether the cursor has no current propagation site.
# @complexity O(1)
pub Trace.isEmpty() bool:
    ret traceStatus(this.handle) != 0
..

# Returns true when traversal reached its safety bound or observed a node being
# replaced concurrently. Check the terminal cursor after iteration.
# @complexity O(1)
pub Trace.isTruncated() bool:
    ret traceStatus(this.handle) == 2
..

traceNext(handle u64) u64:
    # SAFETY: this audited implementation injects the required low-level IR.
    unsafe:
        llvm "  %next = call i64 @magma.error.trace.next(i64 %handle)\n"
        llvm "  ret i64 %next\n"
    ..
..

# Advances toward the error's origin. Calling this on an empty cursor is invalid.
# @complexity O(1)
pub Trace.next() Trace:
    ret Trace(handle=traceNext(this.handle))
..

# The following accessors are valid only for a non-empty cursor.
traceFunction(handle u64) str:
    # SAFETY: this audited implementation injects the required low-level IR.
    unsafe:
        llvm "  %value = call %type.str @magma.error.trace.function(i64 %handle)\n"
        llvm "  ret %type.str %value\n"
    ..
..

# Returns the function name at the current trace site.
# @complexity O(1)
# @warning The cursor must not be empty.
pub Trace.function() str:
    ret traceFunction(this.handle)
..

traceFile(handle u64) str:
    # SAFETY: this audited implementation injects the required low-level IR.
    unsafe:
        llvm "  %value = call %type.str @magma.error.trace.file(i64 %handle)\n"
        llvm "  ret %type.str %value\n"
    ..
..

# Returns the source file at the current trace site.
# @complexity O(1)
# @warning The cursor must not be empty.
pub Trace.file() str:
    ret traceFile(this.handle)
..

traceLine(handle u64) u32:
    # SAFETY: this audited implementation injects the required low-level IR.
    unsafe:
        llvm "  %value = call i32 @magma.error.trace.line(i64 %handle)\n"
        llvm "  ret i32 %value\n"
    ..
..

# Returns the one-based source line at the current trace site.
# @complexity O(1)
# @warning The cursor must not be empty.
pub Trace.line() u32:
    ret traceLine(this.handle)
..

traceColumn(handle u64) u32:
    # SAFETY: this audited implementation injects the required low-level IR.
    unsafe:
        llvm "  %value = call i32 @magma.error.trace.column(i64 %handle)\n"
        llvm "  ret i32 %value\n"
    ..
..

# Returns the one-based source column at the current trace site.
# @complexity O(1)
# @warning The cursor must not be empty.
pub Trace.column() u32:
    ret traceColumn(this.handle)
..

# Prints all recorded propagation sites without allocating.
# @complexity O(N), where N is the retained trace length
# @example
#   errors.printTrace(failure)
pub printTrace(e error) void:
    # SAFETY: this audited implementation injects the required low-level IR.
    unsafe:
        llvm "  call void @magma.error.printTrace(%type.error %e)\n"
        llvm "  ret void\n"
    ..
..

# Reports whether two errors belong to the same numeric category.
# Messages and platform-specific details are ignored.
# @complexity O(1)
# @example
#   sameKind := errors.is(actual, errors.outOfBounds(""))
pub is(a error, b error) bool:
    ret a.code() == b.code()
..

# Returns whether an error belongs to a numeric category. Error equality is
# category-based; messages and platform details are not compared.
# @complexity O(1)
pub hasCode(e error, expected u32) bool:
    ret e.code() == expected
..

# Returns the error type as a string.
# For example: 0 => "ok", 1 => "unexpected"
# @complexity O(1).
# @param e input error
# @returns error type as string
# @example
#   label := errors.toStr(failure)
pub toStr(e error) str:
    c u32 = e.code()

    if c == ERR_OK:
        ret "ok"
    elif c == ERR_FAIL:
        ret "unexpected"
    elif c == ERR_INVALID_ARG:
        ret "invalid argument"
    elif c == ERR_OUT_OF_MEMORY:
        ret "out of memory"
    elif c == ERR_END_OF_FILE:
        ret "end of file"
    elif c == ERR_WOULD_OVERFLOW:
        ret "would overflow"
    elif c == ERR_INVALID_TYPE:
        ret "invalid type"
    elif c == ERR_OUT_OF_BOUNDS:
        ret "out of bounds"
    elif c == ERR_NOT_FOUND:
        ret "not found"
    elif c == ERR_CANCELLED:
        ret "cancelled"
    elif c == ERR_WOULD_BLOCK:
        ret "would block"
    elif c == ERR_TIMED_OUT:
        ret "timed out"
    elif c == ERR_CONNECTION_RESET:
        ret "connection reset"
    elif c == ERR_CONNECTION_REFUSED:
        ret "connection refused"
    elif c == ERR_ADDRESS_IN_USE:
        ret "address in use"
    ..
    ret "unknown error"
..

# Creates an error value from a code and message. Messages longer than 65,535
# bytes retain their first 65,535 bytes.
# @complexity O(1).
makeErr(errorCode u32, msg str) error:
	# SAFETY: this audited implementation injects the required low-level IR.
	unsafe:
        llvm "  %mp = extractvalue %type.str %msg, 0\n"
        llvm "  %ml64 = extractvalue %type.str %msg, 1\n"
        llvm "  %too.long = icmp ugt i64 %ml64, 65535\n"
        llvm "  %bounded = select i1 %too.long, i64 65535, i64 %ml64\n"
        llvm "  %ml = trunc i64 %bounded to i16\n"
        llvm "  %e0 = insertvalue %type.error zeroinitializer, ptr %mp, 0\n"
        llvm "  %e1 = insertvalue %type.error %e0, i32 %errorCode, 1\n"
        llvm "  %e2 = insertvalue %type.error %e1, i16 %ml, 3\n"
        llvm "  ret %type.error %e2\n"
	..
..

# Wraps a platform error code without allocating a formatted message. The high
# bit distinguishes native codes from standard-library categories.
# @complexity O(1)
# @example
#   failure := errors.native(platformCode, "open failed")
pub native(errorCode u32, msg str) error:
    ret makeErr(0x80000000 | errorCode, msg)
..

# Reports whether an error wraps a native platform code.
# @complexity O(1)
pub isNative(e error) bool:
    ret (e.code() & 0x80000000) != 0
..

# Returns the wrapped platform code, or zero for a non-native error.
# @complexity O(1)
pub nativeCode(e error) u32:
    ret e.code() & 0x7FFFFFFF
..

# Returns an error with code 0 indicating success.
# There isn't any reason to use this function, unless a client code must return 
# an error no matter what.
# @complexity O(1).
# @returns error
# @example
#   ret errors.ok()
pub ok() error:
    # SAFETY: this audited implementation injects the required low-level IR.
    unsafe:
        llvm "  ret %type.error zeroinitializer\n"
    ..
..

# Returns an error with code 1 indicating an opaque error.
# @complexity O(1).
# @param msg user-facing context for the failure
# @returns error
# @example
#   ret errors.failure("operation failed")
pub failure(msg str) error:
    ret makeErr(ERR_FAIL, msg)
..

# Returns an error with code 2 indicating that the client provided an invalid
# argument to a function or protocol.
# @complexity O(1).
# @param msg explanation of the rejected argument
# @returns error
# @example
#   ret errors.invalidArgument("count must be positive")
pub invalidArgument(msg str) error:
    ret makeErr(ERR_INVALID_ARG, msg)
..

# Returns an error with code 3 indicating that the system is out of memory.
# @complexity O(1).
# @param msg context about the allocation that failed
# @returns error
# @example
#   ret errors.outOfMemory("could not grow buffer")
pub outOfMemory(msg str) error:
    ret makeErr(ERR_OUT_OF_MEMORY, msg)
..

# Returns an error with code 4 indicating the operation hitting the end of a file.
# This may or may not be an error condition, so good documentation is warranted
# if this error is thrown and should be handled by the consumer.
# @complexity O(1).
# @param msg context about the exhausted input
# @returns error
# @example
#   ret errors.endOfFile("record is incomplete")
pub endOfFile(msg str) error:
    ret makeErr(ERR_END_OF_FILE, msg)
..

# Returns an error with code 5 indicating a would-overflow condition.
# @complexity O(1).
# @param msg explanation of the overflowing operation
# @returns error
# @example
#   ret errors.wouldOverflow("capacity exceeds u64")
pub wouldOverflow(msg str) error:
    ret makeErr(ERR_WOULD_OVERFLOW, msg)
..

# Returns an error with code 6 indicating an invalid type.
# @complexity O(1).
# @param msg explanation of the expected and received types
# @returns error
# @example
#   ret errors.invalidType("expected integer")
pub invalidType(msg str) error:
    ret makeErr(ERR_INVALID_TYPE, msg)
..

# Returns an error with code 7 indicating an index being out of bounds of a container.
# @complexity O(1).
# @param msg context about the invalid index or range
# @returns error
# @example
#   ret errors.outOfBounds("index exceeds length")
pub outOfBounds(msg str) error:
    ret makeErr(ERR_OUT_OF_BOUNDS, msg)
..

pub notFound(msg str) error:
    ret makeErr(ERR_NOT_FOUND, msg)
..

# Returns an error indicating that an operation was cancelled before producing
# a result. Cancellation is distinct from native or unexpected failure.
pub cancelled(msg str) error:
    ret makeErr(ERR_CANCELLED, msg)
..

pub wouldBlock(msg str) error:
    ret makeErr(ERR_WOULD_BLOCK, msg)
..

pub timedOut(msg str) error:
    ret makeErr(ERR_TIMED_OUT, msg)
..

pub connectionReset(msg str) error:
    ret makeErr(ERR_CONNECTION_RESET, msg)
..

pub connectionRefused(msg str) error:
    ret makeErr(ERR_CONNECTION_REFUSED, msg)
..

pub addressInUse(msg str) error:
    ret makeErr(ERR_ADDRESS_IN_USE, msg)
..
