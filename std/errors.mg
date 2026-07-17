mod errors

# Returns the error code of an error.
# A code of 0 indicates a successful operation.
# O(1).
# @param e input error
# @returns error code
pub code(e error) u32:
    llvm "  %e0 = extractvalue %type.error %e, 0\n"
    llvm "  ret i32 %e0\n"
..

# Returns the message from an error.
# O(1).
# @param e input error
# @returns error message
pub message(e error) str:
    llvm "  %e0 = extractvalue %type.error %e, 1\n"
    llvm "  ret %type.str %e0\n"
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
    llvm "  %e0 = insertvalue %type.error zeroinitializer, i32 %errorCode, 0\n"
    llvm "  %e1 = insertvalue %type.error %e0, %type.str %msg, 1\n"
    llvm "  ret %type.error %e1\n"
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

