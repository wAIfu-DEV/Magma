# Monomorphization

This package specializes generic structs, functions, and member functions before
link and type checking. `Run` mutates the parsed module ASTs in place, rebuilds
their scope trees, and removes generic templates after all reachable concrete
instances have been processed.

The files are divided by responsibility:

- `context.go` owns shared state, work queues, and source-aware errors.
- `clone.go` provides the AST copies required before specialization.
- `names.go` defines stable type signatures and specialized symbol names.
- `substitution.go` applies generic substitutions to cloned AST nodes.
- `resolution.go` resolves templates, owners, and local type environments.
- `instantiation.go` creates concrete struct, function, and member instances.
- `rewrite.go` discovers and rewrites generic uses throughout types and bodies.
- `templates.go` registers, synchronizes, and prunes generic templates.
- `monomorph.go` contains the public pipeline entry point and work-loop order.

The specialized-name format, queue order, AST mutation order, and public `Run`
API are compiler contracts. Structural refactors must preserve them because
later checking and LLVM lowering consume the resulting concrete AST directly.
