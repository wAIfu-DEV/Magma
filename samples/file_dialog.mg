mod main
use "std:allocator" allocator
use "std:dialog" dialog
use "std:errors" errors
use "std:heap" heap
use "std:io" io

pub main() !void:
    a allocator.Allocator = heap.allocator()
    configuration := dialog.defaultOptions()
    selected str, dialogError error = dialog.openFile(configuration)
    if dialogError.nok():
        if errors.hasCode(dialogError, errors.ERR_CANCELLED):
            try io.printLn("cancelled")
            ret
        ..
        throw dialogError
    ..
    defer selected.free(a)
    try io.printLn(selected)
..
