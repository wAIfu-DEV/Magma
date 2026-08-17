# Compiler design

This document describes the compiler present in `main.go` and `src/`. It is an
implementation map, not a proposed architecture. The command-line driver owns
Clang discovery and native emission; packages under `src/` load a whole Magma
program, validate it, and produce LLVM IR.

## Pipeline

The authoritative order is encoded by the result types in
`src/compiler_pipeline`:

```text
source files
  -> parse and build scopes
  -> specialize generics
  -> link names and types
  -> type-check
  -> validate lowering contracts
  -> check ownership, provenance, and bounds safety
  -> lower and clean LLVM IR
  -> Clang optimization and native emission
```

`compilerpipeline.Parse` loads the root and all transitive imports. Each file
is tokenized, parsed, and given a scope tree. Imports may be processed
concurrently, but a compilation unit is published in `SharedState.Files` only
once. The command-line compiler then calls `RequireMainModule`; editor analysis
omits that check so an arbitrary module can be opened in the LSP.

`monomorph.Run` specializes reachable generic structs, functions, and methods
before semantic linking. It mutates module ASTs, rebuilds their scope trees,
and removes generic templates after concrete work has been exhausted.

The checker has two ordered passes. `CheckLinks` resolves declarations, source
types, calls, and member paths. `TypeChecker` validates expressions and
statements using those links and attaches the semantic types consumed by later
stages.

`lowering_validate.Validate` is a read-only boundary check. It rejects
unresolved or inconsistent AST metadata, residual generic types, malformed
control flow, and expression shapes the backend cannot lower. `llvm_ir.IrWrite`
also runs it defensively for direct package callers.

`destroy_checker.Run` is the mandatory safety gate. It analyzes ownership
places, explicit moves, destructors, pointer provenance, local escapes,
completion-bearing handles, and subscript range proofs. Definite safety
diagnostics stop compilation unless warning mode was explicitly selected;
resource cleanup and leak diagnostics remain warnings.

`llvm_ir.IrWrite` emits a whole LLVM module, and `ir_cleaner.CleanIr` removes
redundant textual fragments. The driver passes the result to Clang. LLVM output
stops before native compilation; object output stops before linking;
executable output also links declared libraries and copies declared bundles.

## Program representation

`types.SharedState` is the compilation-wide store. It contains the target,
standard-library root, parsed `FileCtx` objects, import synchronization, LLVM
declarations, exported native symbols, LSP source overrides, and diagnostics.
Stage wrapper types constrain orchestration, but passes currently mutate and
annotate this shared representation in place.

Each `FileCtx` contains source bytes, line indexes, tokens, its AST and scope
tree, module identity, import aliases, native libraries, and bundles. The
parser assigns every loaded module a unique internal package name, preventing
backend symbol collisions while source keeps using declared names and aliases.

AST nodes acquire information through the pipeline. Parsing records source
shape, linking attaches declaration identity, type checking supplies semantic
types and member transitions, safety analysis adds proofs, and lowering
consumes them. Diagnostics retain file and token locations rather than relying
on generated LLVM names.

## Modules and imports

Every source file starts with `mod <name>`. `pipeline.DoMain` also imports
`std/core.mg` into the root module as the private `__core` namespace. Normal
paths resolve relative to the importing file; `std:` paths resolve from
`SharedState.StdRoot`. Source overrides let the LSP substitute an unsaved
buffer while other imports continue to come from disk.

An import alias names the imported namespace. Nested module traversal is
allowed through `pub use` re-exports, and `types.ResolveModulePrefix` requires
every intermediate namespace to be public. Declaration visibility is checked
separately by the linker.

## Targets and native emission

`src/target` asks Clang to canonicalize the requested target and emit an empty
LLVM module. Its triple and data layout are retained verbatim. Clang
preprocessor macros determine target C integer widths for the compiler-known
aliases in `std:c`. An architecture-only request such as `i386` replaces the
architecture in Clang's native triple while retaining its OS and ABI.

The backend emits that target triple and data layout. The driver invokes Clang
again to optimize LLVM or produce an object or executable. Top-level `link`
declarations affect executable emission only. Ordinary library names become
`-l<name>`, `:filename` retains Clang's exact-library form, and
`framework:<name>` becomes `-framework <name>`. `bundle` declarations are also
executable-only and are copied after a successful link.

## LLVM backend

The backend is divided by lowering responsibility. `type_emission.go` renders
LLVM types; `values.go` handles constants, globals, locals, and aggregates;
`expressions.go` and `numeric.go` handle expressions; `calls.go` handles direct,
member, function-pointer, and destructor calls; `control_flow.go` emits
statements and branches; and `functions.go` emits bodies and entry/export
wrappers. `c_abi.go` contains foreign-ABI classification.

Magma globals lower as thread-local LLVM globals. Throwing functions return a
value/error pair in the internal convention. Failed `try` and `throw` edges
append static propagation sites to a bounded sharded trace runtime. External
declarations and `@export_name` wrappers instead use the backend's C ABI rules.

## Language server

`src/lsp` reuses parsing and semantic analysis rather than maintaining another
language model. It installs source overrides for open documents and can retain
a partial parsed program when a later import or semantic stage fails. It
provides diagnostics, completion, hover, definition lookup, import-path
completion, safety quick fixes, and semantic highlighting.

Safety warning mode changes diagnostic severity, not analysis or diagnostic
codes. `--safety-warnings` supplies the initial policy; LSP initialization
options and configuration changes can update it.

## Package map

- `tokenizer`, `parser`, and `scope_info` build per-file syntax and scopes.
- `pipeline`, `pipeline_async`, and `join` load the import graph.
- `monomorph` creates the concrete generic program.
- `checker` performs name linking and type checking.
- `lowering_validate` checks the semantic-to-backend contract.
- `destroy_checker` and `safety/place` implement safety analysis.
- `llvm_ir`, `llvm_fragments`, and `ir_cleaner` produce LLVM text.
- `target` resolves the target and compiler-known C types.
- `clang` locates and identifies Clang.
- `comp_err` and `line_idx` support source diagnostics.
- `compiler_pipeline` is the ordered orchestration API.
- `lsp` is the stdio language-server adapter.

Package-local README files describe internal file splits and refactoring
invariants for the larger compiler packages.
