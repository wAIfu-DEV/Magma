mod dialog_impl_unix
# XDG Desktop Portal backend boundary. The D-Bus implementation is pending;
# keeping it behind this module preserves the portable API.

use "std:allocator" allocator
use "std:errors" errors

pub openFile(a allocator.Allocator, filters ptr, filterCount u64, defaultPath str, title str, parent ptr) !$str:
    throw errors.failure("XDG file dialog backend is not implemented")
..

pub saveFile(a allocator.Allocator, filters ptr, filterCount u64, defaultPath str, defaultName str, title str, parent ptr) !$str:
    throw errors.failure("XDG file dialog backend is not implemented")
..

pub openDir(a allocator.Allocator, defaultPath str, title str, parent ptr) !$str:
    throw errors.failure("XDG file dialog backend is not implemented")
..
