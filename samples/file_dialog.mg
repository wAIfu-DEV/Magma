mod main
use "std:allocator" as allocator
use "std:dialog" as dialog
use "std:errors" as errors
use "std:heap" as heap
use "std:io" as io

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
    defer selected.free()
    try io.printLn(selected)
..
