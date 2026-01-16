mod file_impl_unix

use "../file.mg" file
use "../cast.mg" cast

pub write(handle FileHandle*, bytes str) !u64:
    ret 0
..

pub read(handle FileHandle*, n u64) !str:
    ret ""
..

pub closeFile(handle ptr) void:
   ret
..

pub openFile(path str, openMode i64) !ptr:
    ret cast.utop(0)
..
