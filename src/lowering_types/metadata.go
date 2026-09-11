package loweringtypes

import (
	"fmt"

	t "Magma/src/types"
)

// TraceABI resolves the Magma-owned trace site layout and push function from
// checked program state. Callers consume semantic definitions and lower them
// normally; no LLVM spelling is hard-coded here.
func (l *Lowerer) TraceABI() (*t.StructDef, *t.NodeFuncDef, error) {
	l.state.FilesM.Lock()
	defer l.state.FilesM.Unlock()
	for _, file := range l.state.Files {
		if file == nil || file.ModuleName != "core" || file.GlNode == nil {
			continue
		}
		site := file.GlNode.StructDefs["ErrorTraceSite"]
		push := file.GlNode.FuncDefs["errorTracePush"]
		if site != nil && push != nil {
			return site, push, nil
		}
	}
	return nil, nil, fmt.Errorf("core trace implementation is incomplete")
}

// FunctionSource returns the source path owning a checked function identity.
func (l *Lowerer) FunctionSource(function *t.NodeFuncDef) (string, error) {
	if function == nil {
		return "", fmt.Errorf("cannot locate a missing function")
	}
	l.state.FilesM.Lock()
	defer l.state.FilesM.Unlock()
	for _, file := range l.state.Files {
		if file == nil || file.GlNode == nil {
			continue
		}
		for _, candidate := range file.GlNode.FuncDefs {
			if candidate == function {
				return file.FilePath, nil
			}
		}
		for _, definition := range file.GlNode.StructDefs {
			if definition == nil {
				continue
			}
			for _, candidate := range definition.Funcs {
				if candidate == function {
					return file.FilePath, nil
				}
			}
			if definition.Destructor == function {
				return file.FilePath, nil
			}
			for _, candidate := range definition.Destructors {
				if candidate == function {
					return file.FilePath, nil
				}
			}
		}
	}
	return "", fmt.Errorf("cannot locate source for function %q", function.AbsName)
}
