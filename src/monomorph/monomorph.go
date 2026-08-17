package monomorph

import (
	"Magma/src/comp_err"
	scopeinfo "Magma/src/scope_info"
	t "Magma/src/types"
	"fmt"
	"strings"
)

func Run(shared *t.SharedState) error {
	ctx := &monoCtx{
		shared: shared,

		modules:            map[string]*t.NodeGlobal{},
		structTemplates:    map[string]*t.NodeStructDef{},
		funcTemplates:      map[string]*t.NodeFuncDef{},
		memberTemplates:    map[string]*t.NodeFuncDef{},
		structInstances:    map[string]string{},
		funcInstances:      map[string]string{},
		memberInstances:    map[string]string{},
		structDisplayNames: map[string]string{},
		queuedStruct:       map[*t.NodeStructDef]bool{},
		queuedFunc:         map[*t.NodeFuncDef]bool{},
		queuedVar:          map[*t.NodeExprVarDef]bool{},
	}

	for _, f := range shared.Files {
		ctx.modules[f.PackageName] = f.GlNode
	}

	for module, gl := range ctx.modules {
		for name, st := range gl.StructDefs {
			if len(st.TypeParams) > 0 {
				if d, ok := func() (*t.NodeStructDef, bool) {
					for _, x := range gl.Declarations {
						if s, ok := x.(*t.NodeStructDef); ok && flattenName(s.Class.NameNode) == name {
							return s, true
						}
					}
					return nil, false
				}(); ok {
					ctx.structTemplates[makeTemplateKey(module, name)] = d
				}
			}
		}

		for name, fn := range gl.FuncDefs {
			ctx.registerFuncTemplate(module, name, fn)
		}
	}

	ctx.resolveGenericCandidateExpressions()

	for module, gl := range ctx.modules {
		for _, d := range gl.Declarations {
			switch n := d.(type) {
			case *t.NodeStructDef:
				if !isGenericStructDecl(n) {
					ctx.queueStruct(module, n)
				}
			case *t.NodeFuncDef:
				if !isGenericFuncDecl(n) {
					ctx.queueFunc(n)
				}
			case *t.NodeExprVarDef:
				ctx.queueVar(n)
			case *t.NodeConstDef:
				ctx.queueVar(n.VarDef)
				if e := ctx.rewriteExpr(module, gl, n.Initializer, map[string]*t.NodeType{}); e != nil {
					return e
				}
			}
		}
		_ = module
	}

	for len(ctx.structQueue) > 0 || len(ctx.funcQueue) > 0 || len(ctx.varQueue) > 0 {
		for len(ctx.structQueue) > 0 {
			item := ctx.structQueue[0]
			ctx.structQueue = ctx.structQueue[1:]
			module := item.module
			st := item.st
			gl := ctx.modules[module]
			if gl == nil {
				continue
			}
			for _, fld := range st.Class.ArgsNode.Args {
				if e := ctx.rewriteType(module, gl, fld.TypeNode); e != nil {
					return ctx.sourceError(gl, &fld.Tk, e)
				}
			}
			syncStructDefFields(gl, st)
		}

		for len(ctx.funcQueue) > 0 {
			fn := ctx.funcQueue[0]
			ctx.funcQueue = ctx.funcQueue[1:]
			module := strings.Split(fn.AbsName, ".")[0]
			gl := ctx.modules[module]
			if gl == nil {
				continue
			}

			for _, a := range fn.Class.ArgsNode.Args {
				if e := ctx.rewriteType(module, gl, a.TypeNode); e != nil {
					return ctx.sourceError(gl, &a.Tk, e)
				}
			}
			if e := ctx.rewriteType(module, gl, fn.ReturnType); e != nil {
				return ctx.sourceError(gl, nameToken(fn.Class.NameNode), e)
			}
			if fn.ImplicitContext != nil {
				if fn.ImplicitContext.Type == nil {
					fn.ImplicitContext.Type = t.ImplicitContextType(ctx.shared)
				}
				if fn.ImplicitContext.Type == nil {
					return ctx.sourceError(gl, nameToken(fn.ImplicitContext.Name), fmt.Errorf("canonical context.Ctx type is not loaded"))
				}
				if e := ctx.rewriteType(module, gl, fn.ImplicitContext.Type); e != nil {
					return ctx.sourceError(gl, nameToken(fn.ImplicitContext.Name), e)
				}
			}
			env := map[string]*t.NodeType{}
			for _, a := range fn.Class.ArgsNode.Args {
				env[a.Name] = cloneType(a.TypeNode)
			}
			if fn.ImplicitContext != nil {
				if name, ok := fn.ImplicitContext.Name.(*t.NodeNameSingle); ok {
					env[name.Name] = cloneType(fn.ImplicitContext.Type)
				}
			}
			for _, s := range fn.Body.Statements {
				if e := ctx.rewriteStmt(module, gl, s, env, fn.ReturnType); e != nil {
					return e
				}
			}
		}

		for len(ctx.varQueue) > 0 {
			v := ctx.varQueue[0]
			ctx.varQueue = ctx.varQueue[1:]
			module := strings.Split(v.AbsName, ".")[0]
			gl := ctx.modules[module]
			if gl == nil {
				continue
			}
			if e := ctx.rewriteType(module, gl, v.Type); e != nil {
				return ctx.sourceError(gl, nameToken(v.Name), e)
			}
		}
	}

	ctx.pruneTemplates()

	for _, f := range shared.Files {
		scope, e := scopeinfo.BuildScopeTree(f, f.GlNode)
		if e != nil {
			return comp_err.EnsureDiagnostic(f, &t.Token{Pos: t.FilePos{Line: 1, Col: 1}}, e)
		}
		f.ScopeTree = scope
	}

	return nil
}
