# `std/fake_alloc`

`allocator()` returns a deterministic testing allocator. Every allocation and
reallocation throws an error, while freeing is a no-op. Use it to exercise
failure and cleanup paths; it is not a production allocator.
