mod fs_impl_unix
# Unix filesystem backend used by the portable fs module.


use "std:c" c
use "std:allocator" allocator
use "std:builder" builder
use "std:cast" cast
use "std:errors" errors
use "std:strings" strings

ext ext_unlink unlink(path u8*) c.int
ext ext_opendir opendir(path u8*) ptr
ext ext_readdir readdir(directory ptr) ptr
ext ext_closedir closedir(directory ptr) c.int

pub removeFile(a allocator.Allocator, path str) !void:
    if ext_unlink(strings.toCstrNoCopy(path)) != 0:
        throw errors.failure("unlink failed")
    ..
..

join(a allocator.Allocator, left str, right str) !$str:
    out := try builder.new(a)
    defer out.free()
    try out.appendBorrowed(left)
    if strings.countBytes(left) > 0 && strings.byteAt(left, strings.countBytes(left) - 1) != 47:
        try out.appendBorrowed("/")
    ..
    try out.appendBorrowed(right)
    ret try out.build()
..

walkInner(a allocator.Allocator, root str, visit (str, bool) !void) !void:
    directory := ext_opendir(strings.toCstrNoCopy(root))
    if directory == none:
        throw errors.failure("opendir failed")
    ..
    defer ext_closedir(directory)

    entry := ext_readdir(directory)
    while entry != none:
        raw u8* = entry
        name u8* = cast.utop(cast.ptou(raw) + 19)
        borrowed := strings.fromCstrNoCopy(name)
        if strings.compare(borrowed, ".") == false && strings.compare(borrowed, "..") == false:
            child := try join(a, root, borrowed)
            isDirectory bool = raw[18] == 4
            try visit(child, isDirectory)
            if isDirectory:
                try walkInner(a, child, visit)
            ..
            strings.free(a, child)
        ..
        entry = ext_readdir(directory)
    ..
..

pub walk(a allocator.Allocator, root str, visit (str, bool) !void) !void:
    try walkInner(a, root, visit)
..
