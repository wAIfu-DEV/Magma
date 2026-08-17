# `std/context` and implicit `ctx`

Every ordinary Magma function has an implicit local `ctx context.Ctx`. The
caller passes its context by hidden pointer and the callee copies the value at
entry, so rebinding `ctx` affects only descendants of that invocation:

```magma
use "std:context" context

ctx = context.new(arena.allocator(), scratch.allocator(), pool.executor())
result := try operation() # receives addrof this function's local ctx
```

`Ctx(procAlloc, tempAlloc, exec)` separates storage returned to the caller from
temporary implementation workspace. Both allocator fields have the ordinary
`allocator.Allocator` runtime type and preserve the lifetime of their concrete
implementation owners. A context is a borrowed value and owns no resources.

Functions declared `noctx` omit the hidden ABI argument. They may use `ctx` or
call an ordinary function only after every reachable path has assigned `ctx`.
A `noctx` function can be converted to an ordinary function pointer; the
compiler emits an adapter that discards the incoming context. Native functions
and external declarations are contextless.

Executable startup initializes a retained per-thread default context with the
process heap, retained scratch storage, and retained executor. Native export
thunks do the same on foreign-created threads. `--null-context` instead selects
valid null allocator and executor adapters; their operations return ordinary
unsupported/unavailable errors rather than dereferencing null pointers.

The default bootstrap is itself `noctx`: it first installs a minimal heap/null-
executor context and then constructs the retained thread pool. Initialization
failure is printed and terminates the executable or native thunk.
