# Pre-lowering validation

This package defines the read-only contract between semantic checking and LLVM
lowering. It must reject incomplete semantic AST state with source diagnostics;
it must never repair, normalize, or otherwise mutate the AST.

Covered contracts currently include:

- recursive lowering type shapes for function signatures, struct fields,
  globals, constants, locals, and covered expression metadata;
- resolved intrinsic and absolute types, including nested pointer, reference,
  slice, and function types;
- direct, member, function-pointer, `try`, and destructuring calls;
- function member identity and compiler-inserted receiver arguments;
- struct declaration identity and its resolved LLVM-visible symbol;
- resolved value names and their composite member paths, including global
  symbols, owner/result continuity, and pointer transitions;
- standalone member-access metadata with matching target and result types; and
- slice, pointer, and reference subscripts;
- literals, unary and binary operations, `sizeof`, address-of, and assignments;
- array element/result metadata and struct-initializer field metadata; and
- return expressions and their owning-function return types;
- destructuring bindings and throwing-call result metadata; and
- boolean branch conditions, valid `if` chains, loop placement for `break` and
  `continue`, and unambiguous expression/body forms for `defer`.

Compiler-known placeholders, unresolved named types, incomplete nested types,
and residual generic arguments are rejected before LLVM emission. The LLVM type
emitter independently returns errors for these shapes when called directly.
Deprecated compiler-inserted destructor expressions are also rejected: their
producers and LLVM lowering paths are no longer live and must not silently
re-enter the pipeline.

Add further invariants here in small expression-family tranches as unchecked
lowering assumptions are removed.

`main` runs validation immediately after type checking. `llvm_ir.IrWrite` also
runs it because tests and embedders may call the lowering package directly.
