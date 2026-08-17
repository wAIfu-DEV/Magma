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

Assignments between ownership places transfer tracked ownership. Direct locals
and statically identifiable struct fields are tracked independently. Moving a
field marks that projected place absent until it is replaced; direct ownership
moves through a dynamic index or pointer dereference are rejected in favor of a
checked container operation.

Moving a fresh owner into mutable module storage escapes the per-function
ownership flow because globals outlive the call. The checker does not prove
eventual cleanup of global owners or diagnose replacement of an owner retained
by an earlier call.

A struct constructor consumes owned values placed in ownership-bearing fields.
The resulting aggregate's statically identifiable fields remain tracked:

```magma
resource := try open()
holder := Holder(resource=move resource) # resource is consumed
```

Field reads borrow by default. An ownership transfer from a field requires
`move`, updates that field's tracked state, and is allowed only when the root is
owned. Indexed reads borrow; direct indexed ownership transfers are rejected
because a dynamic element is not a stable tracked place.

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

The checker runs after link, type checking, and lowering validation. Definite
safety violations are compilation errors by default; cleanup and leak findings
are warnings. It reports common cases including:

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

## Allocator implementation regions

Allocator results retain the identity and lifetime of the concrete
implementation behind the `Allocator` interface. Calling `proto()` on a global
implementation produces process-lifetime storage; calling it on a local,
field, or owned implementation bounds allocations by that owner's lifetime.
Copying or dropping the two-word interface does not change this provenance.

The checker propagates allocator regions through helpers, prototype dispatch,
branches, aggregates, context copies, and completion-bearing handles. It
rejects returning local-arena storage, storing it in longer-lived owners,
destroying an allocator implementation while derived storage remains usable,
and obvious `free`/`realloc` calls through a different allocator. Competing
branch origins use the shortest possible lifetime. Unsafe casts may erase
unknown provenance only inside `unsafe:`; known lifetime violations remain
errors.

`ctx.procAlloc` and `ctx.tempAlloc` use their actual implementation provenance;
the field names do not imply process or temporary lifetime. In version 1,
`Scratch.reset()` and `Arena.reset()` are not modeled as allocation epochs.
Reset still invalidates outstanding allocations at runtime, outside the
checker guarantee, so callers must ensure no derived value is used afterward.

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
