package checker

import (
	"Magma/src/comp_err"
	t "Magma/src/types"
	"fmt"
)

type sh *t.SharedState
type ctx struct {
	Shared           sh
	ScopeTree        *t.Scope
	GlobalNode       *t.NodeGlobal
	ModuleBundle     *t.ModuleBundle
	LastFuncDef      *t.NodeFuncDef
	CurrentTypeFunc  *t.NodeFuncDef
	FileCtx          *t.FileCtx
	LoopDepth        int
	ErrorBoundary    int
	PrimitiveMethods map[string]primitiveMethod

	CurrScope  *t.Scope
	AliasStack map[string]bool
}

type primitiveMethod struct {
	Function *t.NodeFuncDef
	Module   string
}

type entryType int

type parsedName struct {
	First    string
	Parts    []string
	HasParts bool
}

type privateSymbolError struct {
	kind   string
	module string
	name   string
}

func resolveModuleName(c *ctx, name parsedName) (string, int, error) {
	parts := append([]string{name.First}, name.Parts...)
	return t.ResolveModulePrefix(c.ModuleBundle.Modules, c.GlobalNode, parts)
}

func (e *privateSymbolError) Error() string {
	return fmt.Sprintf("%s '%s.%s' is private and cannot be used from another module", e.kind, e.module, e.name)
}

func privateSymbolDiagnostic(c *ctx, token *t.Token, err error) error {
	private, ok := err.(*privateSymbolError)
	if !ok {
		return err
	}
	return comp_err.CompilationErrorToken(c.FileCtx, token, private.Error(), "add 'pub' to the declaration to export it")
}

const (
	enumEntAll entryType = iota
	enumEntVar
	enumEntFunc
	enumEntStruct
	enumEntFuncAndVar
)

func enterScope(c *ctx, scope *t.Scope) {
	c.CurrScope = scope
}

func leaveScope(c *ctx) {
	if c.CurrScope.Parent != nil {
		c.CurrScope = c.CurrScope.Parent
	}
}

func flattenName(name t.NodeName) string {
	s := ""

	parsed := parseName(name)

	s += parsed.First
	if parsed.HasParts {
		for _, x := range parsed.Parts {
			s += "." + x
		}
	}
	return s
}

func parseName(name t.NodeName) parsedName {
	switch n := name.(type) {
	case *t.NodeNameSingle:
		return parsedName{
			First:    n.Name,
			HasParts: false,
		}
	case *t.NodeNameComposite:
		return parsedName{
			First:    n.Parts[0],
			HasParts: true,
			Parts:    n.Parts[1:],
		}
	}
	return parsedName{}
}

func lastNameToken(name t.NodeName) *t.Token {
	switch n := name.(type) {
	case *t.NodeNameSingle:
		return &n.Tk
	case *t.NodeNameComposite:
		if len(n.Tokens) > 0 {
			return &n.Tokens[len(n.Tokens)-1]
		}
	}
	return &t.Token{}
}

func ptrTypeFromStructDef(c *ctx, strct *t.StructDef) *t.NodeType {
	tp := typeFromStructDef(c, strct)

	return &t.NodeType{
		Throws: false,
		KindNode: &t.NodeTypePointer{
			Kind: tp.KindNode,
		},
	}
}

func typeFromStructDef(c *ctx, strct *t.StructDef) *t.NodeType {
	/*
		name := &t.NodeNameComposite{
			Parts: []string{
				strct.Module,
				strct.Name,
			},
		}

		if !strings.HasPrefix(strct.Name, strct.Module) {
			name.Parts = []string{strct.Name}
		}*/

	return &t.NodeType{
		Throws: false,
		KindNode: &t.NodeTypeAbsolute{
			AbsoluteName: strct.Module + "." + strct.Name,
		},
	}
}
