mod io

use "strings.mg" strings
use "writer.mg"  writer
use "reader.mg"  reader

@platform("windows")
use "win/io_impl.mg" impl_io

@platform("linux", "android", "ios", "darwin", "freebsd", "netbsd", "openbsd")
use "unix/io_impl.mg" impl_io

ext ext_stdlib_puts puts(text ptr) i32

# printf extern declaration is still LLVM as we do not support variadic args yet
llvm "declare i32 @printf(ptr, ...)\n"

# Format strings
llvm "@.io.const.true  = private constant [5 x i8] c\"true\\00\"\n"
llvm "@.io.const.false = private constant [6 x i8] c\"false\\00\"\n\n"

llvm "@.io.fmt.uint = private constant [5 x i8] c\"%llu\\00\"\n"
llvm "@.io.fmt.int  = private constant [5 x i8] c\"%lld\\00\"\n"
llvm "@.io.fmt.flt  = private constant [4 x i8] c\"%lf\\00\"\n"
llvm "@.io.fmt.str  = private constant [3 x i8] c\"%s\\00\"\n"

pub stdout () writer.Writer:
    ret impl_io.stdout()
..

pub stderr () writer.Writer:
    ret impl_io.stderr()
..

pub stdin () reader.Reader:
    ret impl_io.stdin()
..
