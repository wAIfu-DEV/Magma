# LLVM lowering layout

The LLVM backend is split by lowering responsibility:

- `llvm_ir.go` assembles modules, struct definitions, and the public `IrWrite`
  entry point.
- `context_types.go` owns lowering context, trace strings, shared name/type
  helpers, SSA allocation, and output writers.
- `type_emission.go` emits LLVM names and types.
- `values.go` lowers constants, globals, local definitions, and struct values.
- `expressions.go` lowers literals, addresses, member access, subscripting,
  assignment, arrays, and general expression dispatch.
- `numeric.go` lowers numeric conversions and unary/binary operations.
- `calls.go` lowers ordinary, member, function-pointer, and destructor calls.
- `control_flow.go` lowers returns, errors, branches, loops, and statements.
- `functions.go` lowers function bodies, entry-point wrappers, arguments, and
  exported wrappers.
- `c_abi.go` handles the external C ABI.

The initial split was deliberately mechanical: function bodies and emitted IR
text were left unchanged. Semantic cleanup should be performed separately and
covered by focused IR tests.

## Implicit context ABI

Ordinary Magma functions have a hidden leading `context.Ctx*` parameter.
Lowering copies the complete incoming context into an activation-local stack
slot and passes that slot's address to contextful descendants. This explicit
entry copy is the correctness-first representation: it preserves local `ctx`
rebinding without mutating the caller or a thread root. LLVM optimization may
remove redundant copies, but backend changes must preserve those semantics.

`noctx` and external functions have no hidden parameter. Function-pointer and
prototype-slot types carry the context ABI explicitly; native exports and
callbacks use wrappers so the hidden parameter never leaks into a public ABI.
