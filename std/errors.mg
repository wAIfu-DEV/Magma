mod errors

# Returns the error code of an error.
# A code of 0 indicates a successful operation.
# @param e input error
# @returns error code

pub code(e error) u32:
    llvm "  %e0 = extractvalue %type.error %e, 0\n"
    llvm "  ret i32 %e0\n"
..

# Returns the error type as a string.
# For example: 0 => "ok", 1 => "unexpected"
# @param e input error
# @returns error type as string

pub toStr(e error) str:
    code u32

    llvm "  %e0 = extractvalue %type.error %e, 0\n"
    llvm "  store i32 %e0, ptr %code\n"

    if code == 0:
        ret "ok"
    elif code == 1:
        ret "unexpected"
    elif code == 2:
        ret "invalid_argument"
    ..

    ret ""
..

makeErr(code i32, msg str) error:
    llvm "  %e0 = insertvalue %type.error undef, i32 %code, 0\n"
    llvm "  %e1 = insertvalue %type.error %e0, %type.str %msg, 1\n"
    llvm "  ret %type.error %e1\n"
..

# Returns an error with code 0 indicating success.
# There isn't any reason to use this function, unless a client code must return 
# an error no matter what.
# @returns error

pub ok() error:
    llvm "  ret %type.error zeroinitializer\n"
..

# Returns an error with code 1 indicating an opaque error.
# @returns error

pub failure(message str) error:
    ret makeErr(1, message)
..

# Returns an error with code 2 indicating that the client provided an invalid
# argument to a function or protocol.
# @returns error

pub invalidArgument(message str) error:
    ret makeErr(2, message)
..
