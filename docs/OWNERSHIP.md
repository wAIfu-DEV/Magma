# Ownership and Destruction

Magma checks whole-local ownership for structs that declare at least one
destructor and for primitives with a registered destructor, currently including
`str`. Definite invalid use is a compilation error. `--safety-warnings`
downgrades safety diagnostics for migration; cleanup and leak findings remain
warnings in either mode.

## Owned and borrowed values

`$` before a type marks an ownership-transfer position; it does not change the
value's runtime layout.

```magma
open() !$File                 # returns ownership
consume(file $File) void      # takes ownership
inspect(file File) void       # borrows for the call
```

An owned return initializes an owned local. Transferring an already named owner
requires an explicit `move`, which consumes the source. Fresh owned temporaries
can flow directly. A plain `T` parameter or return is borrowed. Function-pointer
parameters use the same rule. External function parameters are treated as
borrowing even if annotated.

```magma
resource $Resource = openResource()
consume(move resource)
```

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

## Pointer provenance and local lifetimes

`addrof` records the source place of the resulting pointer as compiler metadata;
the runtime pointer remains one machine word. Provenance follows local
assignments and returns from visible helpers such as an identity function.
Returning a pointer to a body local, directly or through such a helper, is a
fatal safety error. Slices created by `array T[n]` are stack-backed and likewise
cannot be returned from their source frame.

Pointer loans are place-sensitive and end at the pointer's last reachable use.
A live pointer to `state.counter` prevents mutation, movement, or destruction of
that field, but does not freeze `state.status`. After the pointer's final use,
the source can be changed normally. Opaque pointer provenance is retained for
the lexical-unsafe stage rather than changing pointer layout or syntax.

## Completion-bearing handles

An owned destructible result constructed from a pointer or stack-backed view
carries that source dependency until its destructor consumes the handle. This
models joinable threads, futures, registrations, and similar wrappers without
source-level lifetime annotations. The dependency follows explicit handle
moves and control-flow joins.

A handle retaining local storage must be joined, awaited, removed, or otherwise
consumed before that storage leaves scope. It cannot be discarded or returned
from the source frame. A deferred destructor is also a valid completion boundary.
This checks storage lifetime only; it does not impose data-race rules.

The safety checker also proves ordinary slice bounds and tracks projected
aggregate ownership. Safety violations are fatal by default and are downgraded,
not suppressed, by `--safety-warnings`. Opaque native retention contracts and
exceptional opaque native retention contracts are handled by a later stage.

## Lexical unsafe blocks

Operations whose validity cannot be established from compiler provenance must
be placed in a lexical `unsafe:` body. This includes dereferencing an unknown
pointer, subscripting a pointer without a proven extent, and inline LLVM.
Unsafe permits the specific low-level operation; it does not disable known
ownership, bounds, escape, or control-flow errors in the body. A function that
contains an unsafe block remains callable from ordinary safe code.

External calls always borrow their arguments for this analysis, even if an
external declaration is annotated with `$`; ownership across a foreign API must
be handled explicitly. Pointer arguments use a call-only, non-retaining default,
and pointer results have opaque provenance rather than being inferred as aliases
of those arguments. A native operation that returns owned pointer-backed state
must be called inside an audited unsafe Magma wrapper; the wrapper exposes an
owning or completion-bearing value to ordinary safe callers. The checker also
does not infer ownership through callbacks whose function type lacks an owned
parameter.

The sample [destroy_checker.mg](../samples/destroy_checker.mg) demonstrates
transfers, borrows, destructors, branches, and expected warnings.
