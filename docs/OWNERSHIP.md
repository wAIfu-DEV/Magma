# Ownership and Destruction

Magma has a warning-only destroy/borrow checker for structs that declare at
least one destructor and for primitives with a registered destructor, currently
including `str`. It emits warnings for the ownership conditions documented
below.

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

A struct constructor is also an ownership boundary. Direct owned locals used as
constructor fields are consumed, but individual fields in the resulting
aggregate are not subsequently tracked:

```magma
resource := try open()
holder := Holder(resource=resource) # resource is consumed
```

Field and indexed reads are treated as borrows because partial moves are not
modeled. An explicitly owned local can claim one (`item $T = values[i]`), but
the source aggregate is not updated and the checker does not verify that the
claimed value was owned.

## Destructors

Prefix a struct member declaration with `destr`:

```magma
Buffer(data u8*)

destr Buffer.free(a alc.Allocator) void:
    a.free(this.data)
..
```

A struct may declare multiple destructor methods. Calling any of them consumes
the receiver for checker purposes. A destructor must be a member, but may take
arguments and return any ordinary or throwing type. The declaration must appear
after its owner struct, like other methods.

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

For `value, err := throwingCall()`, an owned returned value is conditionally
live: it exists only on the success path. The built-in `err.ok()` and `err.nok()`
predicates refine that state, so cleanup is required on success but not on
failure. Equivalent-looking user predicates are not recognized.

## Analysis scope

This is not a full memory-safety or lifetime system. It tracks direct local
variables only. It does not prove alias validity, pointer lifetimes, bounds,
field or indexed ownership, aggregate contents, or partial moves. Address-taking
and raw pointer escapes are outside its model. Warnings do not fail the build.

External calls always borrow their arguments for this analysis, even if an
external declaration is annotated with `$`; ownership across a foreign API must
be handled explicitly. The checker also does not infer ownership through pointer
aliases or callbacks whose function type lacks an owned parameter.

The sample [destroy_checker.mg](../samples/destroy_checker.mg) demonstrates
transfers, borrows, destructors, branches, and expected warnings.
