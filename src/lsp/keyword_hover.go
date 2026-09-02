package lsp

import "Magma/src/types"

var keywordHovers = map[string]string{
	"mod":      "`mod name` declares the source file's module. It must be the first line.",
	"use":      "`use \"path\" alias` imports a module under a mandatory local alias.",
	"pub":      "`pub` exports the following top-level declaration or imported namespace.",
	"ret":      "`ret expression` returns from the current function. The expression is optional for `void`.",
	"throw":    "`throw expression` returns a failing `error` from a throwing function. Throwing an OK error is a no-op.",
	"try":      "`try expression` evaluates a throwing call and propagates its error automatically.",
	"if":       "`if condition:` begins a conditional block, closed by `..`.",
	"elif":     "`elif condition:` adds a conditional branch to the current `if` chain.",
	"else":     "`else:` adds the fallback branch to the current `if` chain.",
	"loop":     "`loop condition:` repeats while its condition is true. A canonical bounds condition also establishes a range proof.",
	"for":      "`for index := start to bound:` iterates to an exclusive upper bound evaluated once.",
	"to":       "`to` separates a `for` loop's initial index from its exclusive upper bound.",
	"break":    "`break` exits the nearest enclosing loop.",
	"continue": "`continue` advances to the next iteration of the nearest enclosing loop.",
	"defer":    "`defer expression` registers cleanup for every exit from the current lexical scope. It also supports a block form.",
	"onerror":  "`onerror expression` registers cleanup that runs only when the scope exits by throwing or propagation.",
	"true":     "`true` is the boolean true literal.",
	"false":    "`false` is the boolean false literal.",
	"none":     "`none` is the null-like value for pointers and function pointers.",
	"sizeof":   "`sizeof Type` returns the target-dependent size of a type in bytes.",
	"addrof":   "`addrof value` returns the address of named storage.",
	"not":      "`not value` inverts a boolean value.",
	"const":    "`const` declares an immutable module-level value with an explicit or inferred type.",
	"alias":    "`alias Name = Type` declares a transparent type alias with no distinct runtime representation.",
	"destr":    "`destr` marks a receiver method as an explicit destructor. Calling it consumes the receiver.",
	"ext":      "`ext` declares a contextless external/native function.",
	"link":     "`link` adds a native library to the final linker invocation.",
	"bundle":   "`bundle` includes a native object or library artifact in the build.",
	"llvm":     "`llvm` inserts an inline LLVM fragment. Its validity remains the programmer's responsibility.",
	"noctx":    "`noctx` removes the hidden context argument from a function or function-pointer type.",
	"bounded":  "`bounded condition:` checks one or more range predicates on entry and establishes reusable proofs for its lexical block.",
	"unsafe":   "`unsafe:` localizes operations whose validity the compiler cannot prove, including explicit ownership claims with `move`. It does not disable unrelated type checks.",
	"move":     "`move value` transfers ownership from a named place and prevents subsequent use until reinitialization.",
	"this":     "`this` is the implicit pointer-like receiver available inside a method body.",
}

var directiveHovers = map[string]string{
	"platform":            "`@platform(...)` keeps the following top-level item only for the listed target operating systems.",
	"export_name":         "`@export_name(...)` exposes a non-generic Magma function through a native ABI wrapper.",
	"no_retain":           "`@no_retain` states that an owned result does not retain pointer or slice arguments.",
	"compiler_known_type": "`@compiler_known_type(...)` names a target-specific type supplied by the compiler for an internal alias.",
}

func (a *analysis) keywordHover(index int) string {
	if a == nil || a.file == nil || index < 0 || index >= len(a.file.Tokens) {
		return ""
	}
	tokens := a.file.Tokens
	token := tokens[index]
	previous := types.Token{}
	next := types.Token{}
	if index > 0 {
		previous = tokens[index-1]
	}
	if index+1 < len(tokens) {
		next = tokens[index+1]
	}

	if previous.KeywType == types.KwAt {
		return directiveHovers[token.Repr]
	}
	switch token.Repr {
	case "array":
		if !arrayExpressionAt(tokens, index) {
			return ""
		}
		return "`array Type[length](...)` creates zero-initialized, stack-backed storage and returns a typed slice. Initializers may be positional or designated by index."
	case "move":
		// `move` is contextual: calls, member access, declarations, and assignments
		// may still use that spelling as an ordinary name.
		if next.KeywType == types.KwParenOp || next.KeywType == types.KwDot || next.KeywType == types.KwEqual || next.KeywType == types.KwInfer {
			return ""
		}
	case "proto":
		if previous.KeywType != types.KwDot && next.Type != types.TokName {
			return ""
		}
		if previous.KeywType == types.KwDot {
			return "`.proto[Type]()` creates a borrowed prototype view of stable implementation storage."
		}
		return "`proto Name(...)` declares an interface-like prototype whose required methods form an immutable vtable."
	case "impl":
		if previous.Type != types.TokName || previous.KeywType == types.KwDot || next.KeywType == types.KwParenOp {
			return ""
		}
		return "`Type impl Prototype(...)` declares that a struct or prototype implements one or more prototypes."
	}
	if token.Type != types.TokKeyword && token.Repr != "move" && token.Repr != "this" {
		return ""
	}
	return keywordHovers[token.Repr]
}

func arrayExpressionAt(tokens []types.Token, index int) bool {
	if index < 0 || index+1 >= len(tokens) || tokens[index].Type != types.TokName || tokens[index].Repr != "array" {
		return false
	}
	next := tokens[index+1]
	if next.Type != types.TokName && next.KeywType != types.KwParenOp && next.KeywType != types.KwDollar {
		return false
	}
	for i := index + 1; i < len(tokens); i++ {
		switch tokens[i].KeywType {
		case types.KwEqual, types.KwNewline:
			return false
		case types.KwBrackOp:
			return true
		}
	}
	return false
}
