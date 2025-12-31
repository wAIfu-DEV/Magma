mod io

# Forward declarations
# - compiler does a pass on the IR to remove duplicate declarations
llvm "declare i32 @puts(ptr)\n"
llvm "declare i32 @printf(ptr, ...)\n"

llvm "\n"

# Format strings
llvm "@.io.const.true  = private constant [5 x i8] c\"true\\00\"\n"
llvm "@.io.const.false = private constant [6 x i8] c\"false\\00\"\n\n"

llvm "@.io.fmt.uint = private constant [5 x i8] c\"%llu\\00\"\n"
llvm "@.io.fmt.int  = private constant [5 x i8] c\"%lld\\00\"\n"
llvm "@.io.fmt.flt  = private constant [4 x i8] c\"%lf\\00\"\n"
llvm "@.io.fmt.str  = private constant [3 x i8] c\"%s\\00\"\n"

# Writes a string to stdout followed by a newline character.
# @param text input string

pub printLn (text str) void:
    llvm "  %s = extractvalue %type.str %text, 0\n"  # extract string pointer
    llvm "  call i32 @puts(ptr %s)\n"                # call stdlib puts
..

# Writes a string to stdout without suffix newline character.
# For the newline variant, use printLn
# @param text input string

pub print (text str) void:
    llvm "  %s = extractvalue %type.str %text, 0\n"
    llvm "  call i32 (ptr, ...) @printf(ptr @.io.fmt.str, ptr %s)\n"
..

# Writes a unsigned 64bit number (u64) to stdout.
# @param num input number

pub printUint (num u64) void:
    llvm "  call i32 (ptr, ...) @printf(ptr @.io.fmt.uint, i64 %num)\n"
..

# Writes a signed 64bit number (i64) to stdout.
# @param num input number

pub printInt (num i64) void:
    llvm "  call i32 (ptr, ...) @printf(ptr @.io.fmt.int, i64 %num)\n"
..

# Writes a floating point 64bit number (f64) to stdout.
# @param num input number

pub printFloat (num f64) void:
    llvm "  call i32 (ptr, ...) @printf(ptr @.io.fmt.flt, double %num)\n"
..

# Writes a boolean value ("true" / "false") to stdout.
# @param boolean input bool

pub printBool (boolean bool) void:
    llvm "  %b = select i1 %boolean, ptr @.io.const.true, ptr @.io.const.false\n"
    llvm "  call i32 (ptr, ...) @printf(ptr @.io.fmt.str, ptr %b)\n"
..
