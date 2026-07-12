mod path_impl

use "../strings.mg" strings

pub separator() u8:
    ret 47
..

pub isAbsolute(path str) bool:
    ret strings.byteAt(path, 0) == 47
..
