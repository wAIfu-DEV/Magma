mod io

# Forward declarations
llvm "declare i32 @puts(ptr)\n"
llvm "declare i32 @printf(ptr, ...)\n"

# Format strings
llvm "@.io.const.true  = private constant [5 x i8] c\"true\\00\" \n"
llvm "@.io.const.false = private constant [6 x i8] c\"false\\00\"\n"

llvm "@.io.format.uint = private constant [5 x i8] c\"%llu\\00\" \n"
llvm "@.io.format.int  = private constant [5 x i8] c\"%lld\\00\" \n"
llvm "@.io.format.flt  = private constant [4 x i8] c\"%lf\\00\"  \n"
llvm "@.io.format.str  = private constant [3 x i8] c\"%s\\00\"   \n"
llvm "@.io.format.bool = private constant [3 x i8] c\"%t\\00\"   \n"

pub printLn (text str) void:
    llvm "  %s = extractvalue %type.str %text, 0\n"  # extract string pointer
    llvm "  call i32 @puts(ptr %s)\n"                # call stdlib puts
..

pub print (text str) void:
    llvm "  %s = extractvalue %type.str %text, 0\n"
    llvm "  call i32 (ptr, ...) @printf(ptr @.io.format.str, ptr %s)\n"
..

pub printUint (num u64) void:
    llvm "  call i32 (ptr, ...) @printf(ptr @.io.format.uint, i64 %num)\n"
..

pub printInt (num i64) void:
    llvm "  call i32 (ptr, ...) @printf(ptr @.io.format.int, i64 %num)\n"
..

pub printFloat (num f64) void:
    llvm "  call i32 (ptr, ...) @printf(ptr @.io.format.flt, double %num)\n"
..

pub printBool (boolean bool) void:
    llvm "  %b = select i1 %boolean, ptr @.io.const.true, ptr @.io.const.false\n"
    llvm "  call i32 (ptr, ...) @printf(ptr %b)\n"
..
