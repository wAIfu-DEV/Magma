mod main
use "std:errors" errors
use "std:heap" heap
use "std:percent" percent
use "std:slices" slices
use "std:strings" strings

pub main() !void:
    a := heap.allocator()
    encoded := try percent.encode(a, "a b/c", percent.URI_COMPONENT)
    defer encoded.free(a)
    if strings.compare(encoded, "a%20b%2Fc") == false:
        throw errors.failure("URI-component percent encoding changed")
    ..
    form := try percent.encode(a, "a b+c", percent.FORM)
    defer form.free(a)
    if strings.compare(form, "a+b%2Bc") == false:
        throw errors.failure("form percent encoding changed")
    ..
    decoded := try percent.decodeForm(a, form)
    defer slices.free(a, decoded)
    if slices.count(decoded) != 5 || decoded[1] != 32 || decoded[3] != 43:
        throw errors.failure("form percent decoding changed")
    ..
    bad, badError := percent.decode(a, "%G0")
    if badError.ok():
        slices.free(a, bad)
        throw errors.failure("invalid percent escape accepted")
    ..
..
