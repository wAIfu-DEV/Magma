mod io

# Forward declarations
llvm "declare i32 @puts(ptr)\n"
llvm "declare i32 @printf(ptr, ...)\n"
# compiler does a pass on the IR to remove duplicate declarations

llvm "\n"

# Format strings
llvm "@.io.const.true  = private constant [5 x i8] c\"true\\00\"\n"
llvm "@.io.const.false = private constant [6 x i8] c\"false\\00\"\n\n"

llvm "@.io.fmt.uint = private constant [5 x i8] c\"%llu\\00\"\n"
llvm "@.io.fmt.int  = private constant [5 x i8] c\"%lld\\00\"\n"
llvm "@.io.fmt.flt  = private constant [4 x i8] c\"%lf\\00\"\n"
llvm "@.io.fmt.str  = private constant [3 x i8] c\"%s\\00\"\n"

pub printLn (text str) void:
    llvm "%s = extractvalue %type.str %text, 0\n"  # extract string pointer
    llvm "call i32 @puts(ptr %s)\n"                # call stdlib puts
..

pub print (text str) void:
    llvm "%s = extractvalue %type.str %text, 0\n"
    llvm "call i32 (ptr, ...) @printf(ptr @.io.fmt.str, ptr %s)\n"
..

pub printUint (num u64) void:
    llvm "call i32 (ptr, ...) @printf(ptr @.io.fmt.uint, i64 %num)\n"
..

pub printInt (num i64) void:
    llvm "call i32 (ptr, ...) @printf(ptr @.io.fmt.int, i64 %num)\n"
..

pub printFloat (num f64) void:
    llvm "call i32 (ptr, ...) @printf(ptr @.io.fmt.flt, double %num)\n"
..

pub printBool (boolean bool) void:
    llvm "%b = select i1 %boolean, ptr @.io.const.true, ptr @.io.const.false\n"
    llvm "call i32 (ptr, ...) @printf(ptr @.io.fmt.str, ptr %b)\n"
..
