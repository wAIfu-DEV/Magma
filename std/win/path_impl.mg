mod path_impl

use "../strings.mg" strings

pub separator() u8:
    ret 92
..

# THIS IS DOGSHIT
pub isAbsolute(path str) bool:
    n := strings.countBytes(path)
    ret strings.byteAt(path, 0) == 47 || strings.byteAt(path, 0) == 92 || (n > 2 && strings.byteAt(path, 1) == 58)
..
