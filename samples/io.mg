mod io

llvm "; forward decl of stdlib elems\n"
llvm "declare i32 @puts(ptr)\n"
llvm "declare i32 @printf(ptr, ...)\n"

llvm "\n; format strings\n"

llvm "@.io.const.nl = private constant [2 x i8] c\"\\0A\\00\"\n"
llvm "@.io.format.uint = private constant [5 x i8] c\"%llu\\00\"\n"
llvm "@.io.format.int = private constant [5 x i8] c\"%lld\\00\"\n"
llvm "@.io.format.float = private constant [4 x i8] c\"%lf\\00\"\n"
llvm "@.io.format.s = private constant [3 x i8] c\"%s\\00\"\n"

llvm "\n"

pub printLn (text str) void:
    llvm "  ; inline llvm\n"
    llvm "  %s = extractvalue %type.str %text, 1\n"  # extract string pointer
    llvm "  call i32 @puts(ptr %s)\n"                # call stdlib puts
..

pub print (text str) void:
    llvm "  ; inline llvm\n"
    llvm "  %s = extractvalue %type.str %text, 1\n" # extract string pointer
    llvm "  call i32 (ptr, ...) @printf(ptr @.io.format.s, ptr %s)\n"
..

pub printUint (num u64) void:
    llvm "  ; inline llvm\n"
    llvm "  call i32 (ptr, ...) @printf(ptr @.io.format.uint, i64 %num)\n" # call stdlib printf with %ull
..

pub printInt (num i64) void:
    llvm "  ; inline llvm\n"
    llvm "  call i32 (ptr, ...) @printf(ptr @.io.format.int, i64 %num)\n" # call stdlib printf with %ll
..

pub printFloat (num f64) void:
    llvm "  ; inline llvm\n"
    llvm "  call i32 (ptr, ...) @printf(ptr @.io.format.float, double %num)\n" # call stdlib printf with %ll
..
