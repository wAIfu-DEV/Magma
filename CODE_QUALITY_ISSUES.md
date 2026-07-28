# Code Quality Issues

Status updated after the contained-fixes pass. `go test ./...` and `go vet ./...`
both pass.

## Completed contained fixes

- Added an optional `--std <directory>` compiler argument and propagated it
  through compilation and LSP analysis. When omitted, the compiler uses the
  `std` directory beside `Magma.exe`.
- Updated test helpers and repository scripts to pass the standard-library path
  explicitly where appropriate, while preserving executable-relative defaults
  for normal compiler and LSP launches.
- Protected `rand_id`'s global random source with a mutex.
- Replaced two unsafe byte-to-string reinterpretations with normal conversions.
  The old conversions avoided allocation by making strings share byte-array
  storage, bypassing Go's type safety and string immutability guarantees.
- Preserved the underlying source-file error and path when reading fails.
- Made malformed LSP JSON and LSP error-response write failures observable.
- Added defensive argument-count and unresolved-call validation before LLVM
  call lowering.
- Rejected void-returning calls used as values.
- Rejected named function arguments with a structured diagnostic instead of
  silently passing them positionally.
- Added duplicate and cross-kind declaration checks for structs in `scope_info`.
- Separated diagnostic printing from compilation-unit error aggregation.
- Added a CI workflow for tests, vet, and CGO-enabled race testing.

`Magma.exe` remains part of the local LSP workflow and is not considered a code
quality issue. Historical reports and other repository artifacts are likewise
outside this code-quality inventory.

## Completed substantial refactors

- Split the former 114 KB `llvm_ir.go`, 84 KB `parser.go`, 47 KB
  `monomorph.go`, 38 KB `type_checker.go`, and 43 KB `link_checker.go` into
  responsibility-focused files with package documentation and characterization
  tests.
- Added a read-only checker-to-LLVM validation boundary for direct, member,
  function-pointer, `try`, and destructuring calls, then extended it to resolved
  names, member paths, subscripts, and recursive lowering type shapes. Function
  signatures, struct fields, variables, constants, and covered expression
  metadata now reject incomplete nested types, unresolved placeholders, and
  residual generic arguments. Literal, operator, `sizeof`, address-of, array,
  struct-initializer, assignment, and return metadata are also covered.
  Incomplete covered nodes produce structured
  diagnostics before ownership analysis or LLVM generation, and the covered
  LLVM assumptions no longer panic or emit placeholder type text.
- Corrected member-access typing so every resolved hop records its actual owner
  and field-result types, including pointer-valued intermediate fields. The
  type checker and pre-lowering contract now verify complete path continuity
  and pointer transitions instead of relying on LLVM-oriented type rewriting.
- Corrected global field-access lowering for reads, writes, pointer-valued
  intermediate fields, and address-of expressions. Rvalue and lvalue lowering
  now share global/local/SSA root-storage resolution and distinguish qualified
  symbols from field paths using resolved member metadata rather than source
  name shape. The pre-lowering contract also rejects globals without a resolved
  LLVM symbol.
- Added explicit function member and entry-point metadata, preserved it through
  generic specialization, and made scope construction and LLVM signature/body
  lowering consume it instead of inferring receiver or entry-point semantics
  from source-name shape. The pre-lowering contract validates member/receiver
  consistency.
- Added explicit resolved struct symbols, preserved and updated them through
  generic specialization, and made LLVM struct emission consume them instead
  of reconstructing names from source syntax and package context. The
  pre-lowering contract rejects missing or inconsistent struct identities.
- Replaced four duplicated reflection-based LSP AST walkers with one tested
  traversal API that centralizes cycle detection, semantic-backlink exclusion,
  pointer/value visitation, and early termination. Completion, documentation
  indexing, value indexing, and hover lookup now supply only their node-specific
  visitor logic.
- Replaced the variable `IsSsa`/`IrName` mixture with explicit resolved storage
  classes for globals, arguments, locals, and direct SSA values. Numeric local
  slot names now live in a per-function LLVM map rather than mutating the shared
  AST; argument slots retain their stable `%name.addr` convention. The
  pre-lowering contract rejects unresolved or contradictory storage metadata.
- Completed cross-module variable resolution for nested member paths. Imported
  globals and constants now resolve an exported root declaration before field
  validation, retain global symbol/storage identity through lowering, preserve
  source-accurate member diagnostics, and enforce constant immutability.
- Audited return-expression lowering. Active returns already use the shared
  expression lowerer for every live expression kind; the apparent unsupported
  path was unreachable legacy code and has been removed.
- Completed function-pointer semantic ownership. Function declarations now
  consistently infer complete function-value signatures (including specialized
  generic functions), declarations are immutable lvalues, calls consume the
  checker-resolved signature, and the lowering contract verifies that function
  values still match their resolved declarations. LLVM no longer reconstructs
  function types from declarations.
- Completed context-sensitive intrinsic-type validation. `void` is now limited
  to function and function-pointer returns; `ptr` is the sole opaque-pointer
  spelling, so the redundant `void*` form is rejected. Parameters, variables, constants, fields,
  slices, arrays, and `sizeof` operands reject direct `void` with source-level
  diagnostics, and the pre-lowering contract independently enforces the same
  invariant without changing valid type trees.
- Resolved generic function-value/subscript ambiguity after module joining.
  Imported specializations such as `lib.identity[u64]` now work as values,
  while type-like runtime indices retain ordinary subscript semantics.
- Completed generic declaration validation. Duplicate parameters, mismatched
  generic member owners, owner/member parameter collisions, and parameterized
  primitive owners now produce source-level diagnostics. Generic struct,
  function, and member-function arity failures are structured diagnostics, and
  the unused incomplete templated-name parser has been removed.
- Moved numeric promotion out of LLVM reconstruction. Type checking now records
  the common operand representation, mixed-width integer and floating-point
  arithmetic widens to the largest required representation, and lowering only
  emits the resolved conversions. Potentially lossy storage, argument, field,
  array, and return conversions produce non-fatal source warnings.
- Removed LLVM's remaining non-numeric semantic type reconstruction. Array
  lengths and subscript indices now carry checker-resolved lowering
  representations through specialization, while conditions, throws, slices,
  and member addresses consume their already-resolved types. The pre-lowering
  contract rejects missing or inconsistent metadata.
- Defined an explicit whole-program compiler pipeline with typed outputs for
  parsing, specialization, linking, type checking, lowering validation,
  ownership analysis, and LLVM generation. CLI and LSP semantic analysis now
  share that orchestration, while parsing still exposes partial programs for
  editor recovery and the established pass order and IR lowering remain
  unchanged.
- Unified source errors and warnings under a transport-neutral diagnostic
  record with severity, compiler stage, source context, message, hint, and an
  optional underlying cause. Pipeline boundaries now attach their stage,
  parallel import failures are aggregated in deterministic path order, and the
  CLI renders source and internal failures without losing either. The LSP uses
  the same diagnostic extraction and has a tested, side-effect-free protocol
  conversion for source ranges and severities. Compatibility constructors keep
  existing compiler passes unchanged while they migrate incrementally.
- Replaced the remaining user-reachable ad-hoc checker errors with structured,
  token-localized diagnostics. Function lookup now distinguishes an unknown
  module alias from a missing imported symbol, invalid chained member access
  identifies the failing member, and bitwise/shift operand failures report both
  resolved types. A contradictory dead guard was removed from LLVM lowering;
  the pre-lowering contract remains responsible for validating member-access
  metadata before IR generation.
- Removed function-level defer scratch state from the semantic AST. Function
  and nested-scope lowering now use the same lowering-local collection model,
  preserving the existing reverse execution order, labels, and emitted control
  flow while making LLVM lowering read-only with respect to defer metadata.

## Remaining issues requiring substantial refactoring

No remaining substantial code-quality issues from the current audit.

Named function arguments are now explicitly unsupported and rejected. Adding
them later would be a language feature, not a code-quality fix.

The pre-lowering contract now covers live specialized control-flow metadata:
destructuring bindings and throwing calls, boolean conditions, `if` chain
shapes, loop-only `break`/`continue`, return ownership metadata, and defer
expression/body consistency. Deprecated compiler-inserted destructor residuals
are rejected rather than reviving their dead LLVM path.
