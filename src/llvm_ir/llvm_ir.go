package llvmir

import (
	"Magma/src/comp_err"
	llvmfragments "Magma/src/llvm_fragments"
	loweringvalidate "Magma/src/lowering_validate"
	magmatypes "Magma/src/magma_types"
	t "Magma/src/types"
	"bytes"
	"fmt"
	"maps"
	"sort"
	"sync"
)

func irProtoVtables(ctx *IrCtx, gl *t.NodeGlobal, reachable map[string]bool) error {
	names := make([]string, 0, len(gl.StructDefs))
	for name := range gl.StructDefs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		implementation := gl.StructDefs[name]
		for _, relation := range implementation.Implements {
			if relation.Proto == nil {
				return fmt.Errorf("unresolved prototype implementation on %s", implementation.Name)
			}
			proto := relation.Proto
			for _, borrowed := range []bool{false, true} {
				symbol := t.ProtoVtableSymbol(implementation, proto)
				if borrowed {
					symbol = t.ProtoBorrowVtableSymbol(implementation, proto)
				}
				if !reachable[symbol] {
					continue
				}
				irWriteGlf(ctx, "@%s = private constant %%struct.%s.%s { ", symbol, proto.Module, proto.VtableName)
				for i, method := range proto.Methods {
					concrete := implementation.Funcs[method.Name]
					if concrete == nil {
						return fmt.Errorf("missing resolved prototype method %s.%s", implementation.Name, method.Name)
					}
					if i != 0 {
						irWriteGl(ctx, ", ")
					}
					entry := concrete.AbsName
					if borrowed {
						entry = t.ProtoBorrowThunkSymbol(implementation, proto, method)
					}
					irWriteGlf(ctx, "ptr @%s", entry)
				}
				irWriteGl(ctx, " }\n")
				if borrowed {
					for _, method := range proto.Methods {
						concrete := implementation.Funcs[method.Name]
						irWrite(ctx, "define private ")
						if err := irThrowingType(ctx, method.Ret); err != nil {
							return err
						}
						irWritef(ctx, " @%s(", t.ProtoBorrowThunkSymbol(implementation, proto, method))
						if method.ContextABI == t.ContextABIContextful {
							irWrite(ctx, "ptr %ctx, ")
						}
						irWrite(ctx, "ptr %storage")
						for i, arg := range method.Args {
							irWrite(ctx, ", ")
							if err := irType(ctx, arg.TypeNode); err != nil {
								return err
							}
							irWritef(ctx, " %%arg%d", i)
						}
						irWrite(ctx, ") alwaysinline {\n  %receiver = load ptr, ptr %storage\n  ")
						returnsValue := !(isVoidType(method.Ret) && !method.Ret.Throws)
						if returnsValue {
							irWrite(ctx, "%result = ")
						}
						irWrite(ctx, "call ")
						if err := irThrowingType(ctx, method.Ret); err != nil {
							return err
						}
						irWritef(ctx, " @%s(", concrete.AbsName)
						if method.ContextABI == t.ContextABIContextful {
							irWrite(ctx, "ptr %ctx, ")
						}
						irWrite(ctx, "ptr %receiver")
						for i, arg := range method.Args {
							irWrite(ctx, ", ")
							if err := irType(ctx, arg.TypeNode); err != nil {
								return err
							}
							irWritef(ctx, " %%arg%d", i)
						}
						if returnsValue {
							irWrite(ctx, ")\n  ret ")
							if err := irThrowingType(ctx, method.Ret); err != nil {
								return err
							}
							irWrite(ctx, " %result\n}\n")
						} else {
							irWrite(ctx, ")\n  ret void\n}\n")
						}
					}
				}
			}
		}
	}
	return nil
}

func irDefineStruct(ctx *IrCtx, structNode *t.NodeStructDef) error {
	definition := ctx.fCtx.GlNode.StructDefs[structNode.Class.NameNode.(*t.NodeNameSingle).Name]
	if definition != nil && definition.IsProto && definition.Proto != nil {
		pointerSize := ctx.Shared.Target.PointerBits / 8
		maxSize, maxAlign, alignType := pointerSize, pointerSize, "ptr"
		for _, file := range ctx.Shared.Files {
			if file == nil || file.GlNode == nil {
				continue
			}
			for _, candidate := range file.GlNode.StructDefs {
				for _, implementation := range candidate.Implements {
					if implementation == nil || implementation.Proto == nil || implementation.Proto.Module != definition.Proto.Module || implementation.Proto.Name != definition.Proto.Name {
						continue
					}
					typ := &t.NodeType{KindNode: &t.NodeTypeAbsolute{AbsoluteName: candidate.Module + "." + candidate.Name}}
					layout, err := cABITypeLayout(ctx, typ)
					if err != nil {
						return err
					}
					if layout.size > maxSize {
						maxSize = layout.size
					}
					if layout.align > maxAlign {
						maxAlign, alignType = layout.align, "%struct."+candidate.Module+"."+candidate.Name
					}
				}
			}
		}
		irWriteGlf(ctx, "%%struct.%s = type { ptr, [0 x %s], [%d x i8] }\n", structNode.AbsName, alignType, maxSize)
		return nil
	}
	if definition != nil {
		if union := ctx.fCtx.GlNode.UnionDefs[definition.Name]; union != nil {
			var maxSize int
			var maxAlign int = 1
			var alignVariant *t.UnionVariant
			for _, variant := range union.Variants {
				typ := &t.NodeType{KindNode: &t.NodeTypeAbsolute{AbsoluteName: union.Module + ".__union_" + union.Name + "_" + variant.Name}}
				layout, err := cABITypeLayout(ctx, typ)
				if err != nil {
					return err
				}
				if layout.size > maxSize {
					maxSize = layout.size
				}
				if layout.align > maxAlign {
					maxAlign, alignVariant = layout.align, variant
				}
			}
			alignType := "i8"
			if alignVariant != nil {
				alignType = "%struct." + union.Module + ".__union_" + union.Name + "_" + alignVariant.Name
			}
			irWriteGlf(ctx, "%%struct.%s = type { i64, [0 x %s], [%d x i8] }\n", structNode.AbsName, alignType, maxSize)
			return nil
		}
	}
	if definition != nil && definition.CoreRole != t.CoreTypeNone {
		irWriteGlf(ctx, "%s = type { ", definition.CoreRole.LLVMName())
	} else {
		irWriteGlf(ctx, "%%struct.%s = type { ", structNode.AbsName)
	}

	// making dud ctx to redirect type IR to global writer
	cpy := *ctx
	cpy.bld = ScopeBuilder{
		Global: ctx.bld.Global,
		Head:   ctx.bld.Global,
		Tail:   ctx.bld.Global,
		Body:   ctx.bld.Global,
	}

	bound := len(structNode.Class.ArgsNode.Args)
	for i, field := range structNode.Class.ArgsNode.Args {
		e := irType(&cpy, field.TypeNode)
		if e != nil {
			return e
		}

		if i < bound-1 {
			irWriteGl(ctx, ", ")
		}
	}

	irWriteGl(ctx, " }\n")
	return nil
}

func irGlobalStructDefs(ctx *IrCtx, glNode *t.NodeGlobal) error {
	ctx.bld.StructM.Lock()
	defer ctx.bld.StructM.Unlock()

	for _, d := range glNode.Declarations {
		switch s := d.(type) {
		case *t.NodeStructDef:
			cpy := *ctx
			cpy.bld.Body = ctx.bld.Struct
			cpy.bld.Head = ctx.bld.Struct
			cpy.bld.Global = ctx.bld.Struct
			cpy.bld.Tail = ctx.bld.Struct

			e := irDefineStruct(&cpy, s)
			if e != nil {
				return e
			}
		default:
			continue
		}
	}
	return nil
}

func irGlobal(ctx *IrCtx, glNode *t.NodeGlobal, reachable map[*t.NodeFuncDef]bool) error {
	for _, d := range glNode.Declarations {
		if fn, ok := d.(*t.NodeFuncDef); ok && !reachable[fn] {
			continue
		}
		e := irGlobalDecl(ctx, d)
		if e != nil {
			if fn, ok := d.(*t.NodeFuncDef); ok {
				return fmt.Errorf("lower %s: %w", fn.AbsName, e)
			}
			return e
		}
	}
	return nil
}

func irLlvm(ctx *IrCtx, llvmNode *t.NodeLlvm) {
	irWrite(ctx, llvmNode.Text)
}

func irWriteModule(
	shared *t.SharedState,
	fCtx *t.FileCtx,
	builder *bytes.Buffer,
	glBld *bytes.Buffer,
	structBld *bytes.Buffer,
	strctBldM *sync.Mutex,
	traceStrings *traceStringPool,
	reachable map[*t.NodeFuncDef]bool,
	reachableVtables map[string]bool,
	i int,
) error {
	nextSsa := 0
	seenScopes := 0
	nestedLoop := 0

	ctx := &IrCtx{
		Shared: shared,
		fCtx:   fCtx,
		bld: ScopeBuilder{
			Struct:  structBld,
			StructM: strctBldM,
			Global:  glBld,
			Head:    &bytes.Buffer{},
			Tail:    &bytes.Buffer{},
			Body:    &bytes.Buffer{},
		},
		parentBld: ScopeBuilder{
			Struct:  structBld,
			StructM: strctBldM,
			Global:  glBld,
			Head:    &bytes.Buffer{},
			Tail:    &bytes.Buffer{},
			Body:    &bytes.Buffer{},
		},
		nextSsa:          &nextSsa,
		moduleIdx:        i,
		traceStrings:     traceStrings,
		constStrings:     map[*t.NodeExprLit]SsaName{},
		localSlots:       map[*t.NodeExprVarDef]SsaName{},
		SeenNestedScopes: &seenScopes,
		NestedLoopCnt:    &nestedLoop,
	}
	builder.Grow(512)

	irWriteGlf(ctx, "; File=\"%s\"\n", ctx.fCtx.FilePath)
	irWriteGlf(ctx, "; Module=\"%s\"\n\n", ctx.fCtx.PackageName)

	irWriteGl(ctx, "; Defined Types\n")
	e := irGlobalStructDefs(ctx, fCtx.GlNode)
	if e != nil {
		return e
	}
	if e = irProtoVtables(ctx, fCtx.GlNode, reachableVtables); e != nil {
		return e
	}

	irWriteGl(ctx, "\n; Global Defs\n")

	irWrite(ctx, "\n; Code\n")
	e = irGlobal(ctx, fCtx.GlNode, reachable)
	if e != nil {
		return e
	}

	builder.WriteString(ctx.bld.Head.String())
	builder.WriteString(ctx.bld.Body.String())
	builder.WriteString(ctx.bld.Tail.String())
	return nil
}

func IrWrite(shared *t.SharedState) ([]byte, error) {
	return irWriteProgram(shared, false)
}

// IrWriteReachable emits only function bodies reachable from executable and
// export roots. The command pipeline uses this after checking the full program;
// IrWrite retains its historical all-declarations behavior for embedders.
func IrWriteReachable(shared *t.SharedState) ([]byte, error) {
	return irWriteProgram(shared, true)
}

func irWriteProgram(shared *t.SharedState, pruneFunctions bool) ([]byte, error) {
	// IrWrite is also used directly by tests and embedders, so enforce the
	// checker-to-lowering contract when the command pipeline was bypassed.
	if err := loweringvalidate.Validate(shared); err != nil {
		return nil, err
	}

	// creates a shallow copy of shared.Files, will prevent any race condition
	// if it were ever to be modified, which it shouldn't.
	shared.FilesM.Lock()
	filesMap := maps.Clone(shared.Files)
	shared.FilesM.Unlock()

	// write header
	headBld := &bytes.Buffer{}
	headBld.WriteString("; Magma\n\n")
	if shared.Target.DataLayout != "" {
		fmt.Fprintf(headBld, "target datalayout = %q\n", shared.Target.DataLayout)
	}
	if shared.Target.Triple != "" {
		fmt.Fprintf(headBld, "target triple = %q\n\n", shared.Target.Triple)
	}
	headBld.WriteString("; Basic Types\n")
	magmatypes.WriteIrBasicTypes(headBld)
	// Context interfaces are represented as three two-word prototype views.
	// The root is retained per native thread; runtime bootstrap replaces these
	// valid zero adapters with the selected full or null implementations.
	headBld.WriteString("%type.context = type { ptr, ptr, ptr, ptr, ptr, ptr }\n")
	headBld.WriteString("@magma.context.root = internal thread_local global %type.context zeroinitializer, align 8\n")
	headBld.WriteString("declare void @abort() noreturn\n")

	headBld.WriteString("\n; Declarations\n")
	shared.LlvmDeclM.Lock()
	for llvm := range shared.LlvmDecl {
		headBld.WriteString(llvm)
	}
	shared.LlvmDeclM.Unlock()

	header := headBld.Bytes()

	traceSlots := shared.ErrorTraceSlots
	if traceSlots == 0 {
		traceSlots = 1024
	}
	utilsFragment, err := llvmfragments.RenderUtils(traceSlots)
	if err != nil {
		return nil, err
	}
	llvmFragments := [][]byte{
		header,
		{},
		utilsFragment,
	}
	fragLen := len(llvmFragments)

	// result receiver
	type resStr struct {
		S []byte
		E error
	}
	results := make([]resStr, len(filesMap)+fragLen)

	// insert llvm fragments
	for i := range fragLen {
		results[i] = resStr{S: llvmFragments[i]}
	}

	// multithreaded writing per-module

	wg := sync.WaitGroup{}
	wg.Add(len(filesMap))

	structDefBld := &bytes.Buffer{}
	structDefBldM := sync.Mutex{}
	reachable := allFunctions(filesMap)
	reachableVtables := allProtoVtables(filesMap)
	if pruneFunctions {
		extraRoots := make([]*t.NodeFuncDef, 0, len(shared.CoreMethods))
		for _, function := range shared.CoreMethods {
			if function != nil {
				extraRoots = append(extraRoots, function)
			}
		}
		for _, file := range filesMap {
			if file == nil || file.GlNode == nil {
				continue
			}
			for _, declaration := range file.GlNode.Declarations {
				if function, ok := declaration.(*t.NodeFuncDef); ok && function.NeedsContextAdapter {
					extraRoots = append(extraRoots, function)
				}
			}
		}
		reachable, reachableVtables = reachableFunctions(filesMap, shared.NullContext, true, extraRoots...)
	}
	traceStrings := newTraceStringPool(collectTraceStrings(filesMap, reachable))

	i := fragLen
	for _, v := range filesMap {

		localI := i
		go func(idx int) {
			defer wg.Done()

			// module local builder
			moduleBld := &bytes.Buffer{}
			glBld := &bytes.Buffer{}
			e := irWriteModule(shared, v, moduleBld, glBld, structDefBld, &structDefBldM, traceStrings, reachable, reachableVtables, idx)
			if e != nil {
				results[idx] = resStr{E: comp_err.EnsureDiagnostic(v, &t.Token{Pos: t.FilePos{Line: 1, Col: 1}}, e)}
				return
			}
			glBld.Write(moduleBld.Bytes())

			results[idx] = resStr{S: glBld.Bytes()}
		}(localI)

		i++
	}

	// join threads
	wg.Wait()

	results[1].S = structDefBld.Bytes()
	traceStringBld := &bytes.Buffer{}
	traceStrings.writeTo(traceStringBld)
	results[1].S = append(results[1].S, traceStringBld.Bytes()...)

	// process results
	irStrings := [][]byte{}
	for _, r := range results {
		if r.E != nil {
			return []byte{}, r.E
		}
		irStrings = append(irStrings, r.S)
	}
	return bytes.Join(irStrings, []byte("\n")), nil
}
