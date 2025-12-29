mod errors


pub ok() error:
    e error

    llvm "store %type.error zeroinitializer, ptr %e\n"

    llvm "%c = getelementptr %type.error, ptr %e, i32 0, i32 0\n"
    llvm "store i32 0, ptr %c\n"

    llvm "%m = getelementptr %type.error, ptr %e, i32 0, i32 1\n"
    llvm "store %type.str zeroinitializer, ptr %m\n"

    ret e
..

pub unexpected(message str) error:
    e error

    llvm "store %type.error zeroinitializer, ptr %e\n"

    llvm "%c = getelementptr %type.error, ptr %e, i32 0, i32 0\n"
    llvm "store i32 1, ptr %c\n"

    llvm "%m = getelementptr %type.error, ptr %e, i32 0, i32 1\n"
    llvm "store %type.str %message, ptr %m\n"

    ret e
..

pub invalidArgument(message str) error:
    e error

    llvm "store %type.error zeroinitializer, ptr %e\n"

    llvm "%c = getelementptr %type.error, ptr %e, i32 0, i32 0\n"
    llvm "store i32 2, ptr %c\n"

    llvm "%m = getelementptr %type.error, ptr %e, i32 0, i32 1\n"
    llvm "store %type.str %message, ptr %m\n"

    ret e
..
