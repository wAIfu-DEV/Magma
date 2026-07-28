mod path_impl
# Windows path rules used by the portable path module.

use "std:strings" strings

pub separator() u8:
    ret 92
..

# THIS IS DOGSHIT
pub isAbsolute(path str) bool:
    n := path.countBytes()
    ret strings.byteAt(path, 0) == 47 || strings.byteAt(path, 0) == 92 || (n > 2 && strings.byteAt(path, 1) == 58)
..
