# Checker package

The checker runs two ordered semantic passes over the parsed AST:

1. `CheckLinks` resolves names, declarations, calls, members, and source-level
   types.
2. `TypeChecker` validates expression and statement types using the links
   established by the first pass.

The type-checking implementation is divided by responsibility:

- `type_helpers.go` constructs, identifies, and renders semantic types.
- `type_diagnostics.go` derives source names and tokens for diagnostics.
- `type_compatibility.go` compares types and decodes constant array indexes.
- `type_expressions.go` validates expressions and their value/lvalue usage.
- `type_statements.go` validates control flow, throws, defers, and returns.
- `type_checker.go` validates functions and globals and contains the public
  `TypeChecker` entry point.

The link-checking implementation is divided similarly:

- `link_context.go` owns pass state and source-name helpers.
- `link_lookup.go` resolves structs, aliases, functions, fields, and methods.
- `link_names.go` resolves scoped names and member chains.
- `link_expressions.go` links calls and other expressions.
- `link_statements.go` traverses bodies and control-flow statements.
- `link_types.go` resolves source-level type syntax.
- `link_checker.go` links declarations and contains the public `CheckLinks`
  entry point.

## Invariants

- Link checking must complete before type checking starts.
- Resolved AST links and inferred types are shared with later compiler stages;
  the type checker must not replace them with independently reconstructed
  names or types.
- A source program rejected by either pass must not reach LLVM lowering.
- Refactors of this package should preserve diagnostic source locations and
  generated IR unless they explicitly implement a semantic language change.
