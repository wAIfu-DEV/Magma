# Experimental LLVM object backend

This package is the opt-in foundation for lowering Magma into LLVM's in-memory
object model. It deliberately does not participate in `compiler_pipeline.Lower`;
the textual backend remains the production backend until feature parity and
output validation are complete.

## Supported LLVM version

The experimental backend currently supports **LLVM 22 only**, using the pinned
`tinygo.org/x/go-llvm` revision in `go.mod`. LLVM's C API is not treated as
cross-major compatible: changing the supported major requires updating the Go
binding, CI image, build tag, and object-backend tests together.

Build and test with:

```sh
go test -tags='llvm_object llvm22' ./src/llvm_object
```

`llvm_object` enables this package and `llvm22` selects its supported C API and
linker flags. Other LLVM-major tags are unsupported even if go-llvm happens to
compile with them.

LLVM headers and shared libraries are development dependencies supplied by the
host's signed system package repository. They are deliberately not bundled in
the ordinary Magma compiler. Distribution therefore remains split:

- normal builds have no LLVM development dependency and use textual lowering;
- experimental object-backend builds require the system LLVM 22 development
  package and both build tags above.

CI uses Arch Linux's signed `llvm`, `llvm-libs`, and `clang` packages. A future
binary distribution may add a separately packaged LLVM-enabled compiler, but
must not make the normal compiler dynamically depend on LLVM.

The API intentionally owns LLVM contexts and hides raw go-llvm handles. This
gives later lowering code stable `Module`, `Builder`, `Type`, `Value`,
`Function`, and `Block` concepts while keeping library-specific lifetime and
target-machine details at the boundary.

Every handle is bound to the module that created it. Cross-module handles,
invalid widths/address spaces, zero handles, and use after module closure are
rejected before go-llvm is called. A module is deliberately single-writer:
parallel lowering must create independent modules and merge them at a
deterministic synchronization point.

`MergeModules` implements that synchronization point. It sorts compilation
units by their stable module name, rejects duplicate names and target metadata
disagreements, transfers each unit through bitcode into one destination
context, links in canonical order, and verifies the whole program. Source
modules remain independently owned and usable after the merge.

Backend failures use `BackendError`, which retains the source token, function,
block, lowering operation, LLVM diagnostic, and target whenever those values
are known. Verification callers should use `VerifyAt` at source-aware
finalization boundaries.

`ExperimentalIR` is the build-tagged direct entry point for tests and
embedders. It accepts only the backend-neutral interface, verifies the result,
and returns printable LLVM IR without routing through or changing the
production textual compiler pipeline.

The neutral CFG rejects cross-function branches, instructions after a
terminator, incomplete phi inputs, and non-`i1` branch conditions before LLVM
verification. Instructions can carry stable `magma.source` metadata without
exposing LLVM metadata handles through the neutral interface.
