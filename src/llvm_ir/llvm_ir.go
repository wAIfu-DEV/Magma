package llvmir

import (
	magmatypes "Magma/src/magma_types"
	t "Magma/src/types"
	"fmt"
	"slices"
	"strings"
)

type IrCtx struct {
	fCtx    *t.FileCtx
	builder strings.Builder
}

func irWritef(ctx *IrCtx, format string, a ...any) {
	ctx.builder.WriteString(fmt.Sprintf(format, a...))
}

func irFuncDef(ctx *IrCtx, fnDefNode *t.NodeFuncDef) error {
	ctx.builder.WriteString("define ")
	e := irType(ctx, &fnDefNode.ReturnType)
	if e != nil {
		return e
	}

	ctx.builder.WriteString(" @")
	e = irName(ctx, fnDefNode.Class.NameNode, true)
	if e != nil {
		return e
	}

	isMemberFunc := false
	switch fnDefNode.Class.NameNode.(type) {
	case *t.NodeNameComposite:
		isMemberFunc = true
	}

	e = irArgsList(ctx, &fnDefNode.Class.ArgsNode, isMemberFunc, fnDefNode.ReturnType.Throws)
	if e != nil {
		return e
	}

	ctx.builder.WriteString(" {")
	ctx.builder.WriteString("}\n")
	return nil
}

func irArg(ctx *IrCtx, argNode *t.NodeArg) error {
	e := irType(ctx, &argNode.TypeNode)
	if e != nil {
		return e
	}

	ctx.builder.WriteString(" %")
	ctx.builder.WriteString(argNode.Name)
	return nil
}

func irArgsList(ctx *IrCtx, argListNode *t.NodeArgList, thisArg bool, throwArg bool) error {
	ctx.builder.WriteString("(")
	bound := len(argListNode.Args)

	if thisArg {
		ctx.builder.WriteString("ptr %this")
		if bound > 0 || throwArg {
			ctx.builder.WriteString(", ")
		}
	}

	if throwArg {
		ctx.builder.WriteString("ptr %throw")
		if bound > 0 {
			ctx.builder.WriteString(", ")
		}
	}

	for i, a := range argListNode.Args {
		e := irArg(ctx, &a)
		if e != nil {
			return e
		}

		if i < bound-1 {
			ctx.builder.WriteString(", ")
		}
	}

	ctx.builder.WriteString(")")
	return nil
}

func irGlobalDecl(ctx *IrCtx, glDeclNode t.NodeGlobalDecl) error {
	switch g := glDeclNode.(type) {
	case *t.NodeFuncDef:
		e := irFuncDef(ctx, g)
		if e != nil {
			return e
		}
	}
	return nil
}

func irNameSingle(ctx *IrCtx, nameNode *t.NodeNameSingle, withPackage bool) error {
	if withPackage {
		ctx.builder.WriteString(ctx.fCtx.PackageName)
		ctx.builder.WriteString(".")
	}
	ctx.builder.WriteString(nameNode.Name)
	return nil
}

func irNameComposite(ctx *IrCtx, nameNode *t.NodeNameComposite, withPackage bool) error {
	if withPackage {
		first := nameNode.Parts[0]

		pack := ctx.fCtx.PackageName
		if slices.Contains(ctx.fCtx.Imports, first) {
			pack = first
		}

		ctx.builder.WriteString(pack)
		ctx.builder.WriteString(".")
	}

	bound := len(nameNode.Parts)
	for i, n := range nameNode.Parts {
		ctx.builder.WriteString(n)
		if i < bound-1 {
			ctx.builder.WriteString(".")
		}
	}

	return nil
}

func irName(ctx *IrCtx, nameNode t.NodeName, withPackage bool) error {
	switch n := nameNode.(type) {
	case *t.NodeNameComposite:
		e := irNameComposite(ctx, n, withPackage)
		if e != nil {
			return e
		}
	case *t.NodeNameSingle:
		e := irNameSingle(ctx, n, withPackage)
		if e != nil {
			return e
		}
	}
	return nil
}

func irType(ctx *IrCtx, typeNode *t.NodeType) error {
	switch tn := typeNode.KindNode.(type) {
	case *t.NodeTypeNamed:
		switch n := tn.NameNode.(type) {
		case *t.NodeNameSingle:
			// TODO: check if intrinsic type
			_, ok := magmatypes.BasicTypes[n.Name]
			if ok {
				ctx.builder.WriteString(magmatypes.BasicTypes[n.Name])
			}
			return nil
		}

		ctx.builder.WriteString("%struct.")
		e := irName(ctx, tn.NameNode, true)
		if e != nil {
			return e
		}
		return nil
	}
	ctx.builder.WriteString("<invalid type>")
	return nil
}

func irDefineStruct(ctx *IrCtx, structNode *t.NodeStructDef) error {
	ctx.builder.WriteString("%struct.")

	e := irName(ctx, structNode.Class.NameNode, true)
	if e != nil {
		return e
	}
	ctx.builder.WriteString(" = type { ")

	bound := len(structNode.Class.ArgsNode.Args)
	for i, field := range structNode.Class.ArgsNode.Args {
		e = irType(ctx, &field.TypeNode)
		if e != nil {
			return e
		}

		if i < bound-1 {
			ctx.builder.WriteString(", ")
		}
	}

	ctx.builder.WriteString(" }\n")
	return nil
}

func irGlobalStructDefs(ctx *IrCtx, glNode *t.NodeGlobal) error {
	for _, d := range glNode.Declarations {
		switch s := d.(type) {
		case *t.NodeStructDef:
			e := irDefineStruct(ctx, s)
			if e != nil {
				return e
			}
		default:
			continue
		}
	}
	return nil
}

func irGlobal(ctx *IrCtx, glNode *t.NodeGlobal) error {
	for _, d := range glNode.Declarations {
		e := irGlobalDecl(ctx, d)
		if e != nil {
			return e
		}
	}
	return nil
}

func IrWrite(fCtx *t.FileCtx, glNode *t.NodeGlobal) (string, error) {
	ctx := &IrCtx{
		fCtx:    fCtx,
		builder: strings.Builder{},
	}
	ctx.builder.Grow(128)

	irWritef(ctx, "; File=\"%s\"\n", ctx.fCtx.FilePath)
	irWritef(ctx, "; Module=\"%s\"\n\n", ctx.fCtx.PackageName)

	ctx.builder.WriteString("; Basic Types\n")
	magmatypes.WriteIrBasicTypes(&ctx.builder)

	ctx.builder.WriteString("\n; Defined Types\n")
	e := irGlobalStructDefs(ctx, glNode)
	if e != nil {
		return "", e
	}

	ctx.builder.WriteString("\n; Code\n")
	e = irGlobal(ctx, glNode)
	if e != nil {
		return "", e
	}

	return ctx.builder.String(), nil
}
