package monomorph

import (
	"Magma/src/comp_err"
	t "Magma/src/types"
	"errors"
	"fmt"
)

func (m *monoCtx) fileCtxForGlobal(gl *t.NodeGlobal) *t.FileCtx {
	m.shared.FilesM.Lock()
	defer m.shared.FilesM.Unlock()
	for _, fileCtx := range m.shared.Files {
		if fileCtx.GlNode == gl {
			return fileCtx
		}
	}
	return nil
}

func (m *monoCtx) genericInstantiationError(gl *t.NodeGlobal, tk *t.Token, err error) error {
	var failure *genericInstantiationFailure
	if !errors.As(err, &failure) {
		return err
	}
	fileCtx := m.fileCtxForGlobal(gl)
	if fileCtx == nil {
		return err
	}
	if failure.unknown {
		return comp_err.CompilationErrorToken(
			fileCtx,
			tk,
			fmt.Sprintf("cannot instantiate generic %s '%s': no generic template was registered", failure.kind, failure.name),
			"use generic arguments only with a declaration that defines generic type parameters",
		)
	}
	return comp_err.CompilationErrorToken(
		fileCtx,
		tk,
		fmt.Sprintf("generic %s '%s' expects %d type arguments but got %d", failure.kind, failure.name, failure.expected, failure.got),
		fmt.Sprintf("provide exactly %d type argument(s)", failure.expected),
	)
}

func nameToken(name t.NodeName) *t.Token {
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

type monoCtx struct {
	shared *t.SharedState

	modules map[string]*t.NodeGlobal

	structTemplates map[string]*t.NodeStructDef
	funcTemplates   map[string]*t.NodeFuncDef
	memberTemplates map[string]*t.NodeFuncDef

	structInstances    map[string]string
	funcInstances      map[string]string
	memberInstances    map[string]string
	structDisplayNames map[string]string

	queuedStruct map[*t.NodeStructDef]bool
	queuedFunc   map[*t.NodeFuncDef]bool
	queuedVar    map[*t.NodeExprVarDef]bool

	structQueue []structWorkItem
	funcQueue   []*t.NodeFuncDef
	varQueue    []*t.NodeExprVarDef
}

type structWorkItem struct {
	module string
	st     *t.NodeStructDef
}
