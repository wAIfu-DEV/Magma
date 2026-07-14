# Ownership and Destruction

Magma has a warning-only destroy/borrow checker for structs that declare at
least one destructor. It catches common ownership mistakes while preserving the
language's explicit, low-level memory model.

## Owned and borrowed values

`$` before a type marks an ownership-transfer position; it does not change the
value's runtime layout.

```magma
open() !$File                 # returns ownership
consume(file $File) void      # takes ownership
inspect(file File) void       # borrows for the call
```

An owned return initializes an owned local. Passing a direct local to a `$T`
parameter or returning it from a `$T` function transfers ownership and consumes
the source. A plain `T` parameter or return is borrowed. Function-pointer
parameters use the same rule. External function parameters are treated as
borrowing even if annotated.

Assignments between direct locals transfer tracked ownership. Assigning an
owned value into a field or indexed location is treated as an ownership escape:
the containing data structure becomes responsible for it, but its contents are
not tracked by this pass.

## Destructors

Prefix a struct member declaration with `destr`:

```magma
Buffer(data u8*)

destr Buffer.free(a alc.Allocator) void:
    a.free(this.data)
..
```

A struct may declare multiple destructor methods. Calling any of them consumes
the receiver for checker purposes. A destructor must be a member and must
return `void`; throwing `!void` destructors are also valid. Arguments are
allowed. The declaration must appear after its owner struct, like other methods.

Destructors are not inserted automatically. Call the appropriate destructor on
every exit path, commonly with `defer`:

```magma
buffer := try makeBuffer(a)
defer buffer.free(a)
```

`defer` runs during ordinary scope exit and on `ret`, `throw`, `break`,
`continue`, and failed `try` propagation. Do not transfer a value after
scheduling its destructor; the checker warns because the deferred call would
consume the old owner later.

## Checker diagnostics

The checker runs after link and type checking and emits non-fatal warnings. It
reports common cases including:

- an owned destructible value not consumed on every scope or function exit;
- consuming a borrowed value or consuming an owner more than once;
- using a local after its ownership may have been transferred;
- overwriting a still-live owned local;
- discarding an owned destructible call result;
- transferring a value while a deferred destructor is pending.

Control-flow joins are conservative, so a value consumed in only one branch is
reported when later used or when another exit can leave it live.

## Deliberate limits

This is not a full memory-safety or lifetime system. It tracks direct local
variables only. It does not prove alias validity, pointer lifetimes, bounds,
field or indexed ownership, aggregate contents, or partial moves. Address-taking
and raw pointer escapes are outside its model. Warnings do not fail the build.

The sample [destroy_checker.mg](../samples/destroy_checker.mg) demonstrates
transfers, borrows, destructors, branches, and expected warnings.
