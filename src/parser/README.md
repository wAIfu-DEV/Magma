# Parser layout

The parser is split by grammar responsibility:

- `context.go` contains parser state, token helpers, modifiers, and module-level
  dependency declarations.
- `expressions.go` parses primary, postfix, unary, binary, assignment, array,
  and deferred expressions.
- `types.go` parses names, arguments, generic parameters, and types.
- `statements.go` parses function bodies and control-flow statements.
- `declarations.go` parses structs, aliases, functions, constants, and other
  global declarations.
- `directives.go` parses compiler and inline-LLVM directives.
- `parser.go` contains global traversal and the public `Parse` entry point.

The initial split is structural. Grammar behavior, AST construction, diagnostic
text, and token-consumption order should not be changed as part of file moves.
Semantic changes need focused tests and separate review.
