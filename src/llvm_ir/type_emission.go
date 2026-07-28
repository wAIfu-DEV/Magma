package llvmir

import (
	magmatypes "Magma/src/magma_types"
	t "Magma/src/types"
	"fmt"
)

func validateIrTypeKind(typeKind t.NodeTypeKind) error {
	switch kind := typeKind.(type) {
	case *t.NodeTypeSlice:
		return validateIrTypeKind(kind.ElemKind)
	case *t.NodeTypePointer:
		return validateIrTypeKind(kind.Kind)
	case *t.NodeTypeRfc:
		return validateIrTypeKind(kind.Kind)
	case *t.NodeTypeFunc:
		for i, argument := range kind.Args {
			if argument == nil || argument.KindNode == nil {
				return fmt.Errorf("LLVM type emission: function argument %d has no type", i+1)
			}
			if err := validateIrTypeKind(argument.KindNode); err != nil {
				return err
			}
		}
		if kind.RetType == nil || kind.RetType.KindNode == nil {
			return fmt.Errorf("LLVM type emission: function return has no type")
		}
		return validateIrTypeKind(kind.RetType.KindNode)
	case *t.NodeTypeNamed:
		name, ok := kind.NameNode.(*t.NodeNameSingle)
		if !ok || name.Name == "" {
			return fmt.Errorf("LLVM type emission: unresolved named type")
		}
		if _, ok := magmatypes.BasicTypes[name.Name]; !ok {
			return fmt.Errorf("LLVM type emission: named type %q was not resolved", name.Name)
		}
		if len(kind.GenericArgs) != 0 {
			return fmt.Errorf("LLVM type emission: type %q retains generic arguments", name.Name)
		}
		return nil
	case *t.NodeTypeAbsolute:
		if kind.AbsoluteName == "" {
			return fmt.Errorf("LLVM type emission: empty absolute type name")
		}
		return nil
	case *t.NodeTypeCompilerKnown:
		return fmt.Errorf("LLVM type emission: unresolved compiler-known type %q", kind.Name)
	case nil:
		return fmt.Errorf("LLVM type emission: missing type kind")
	default:
		return fmt.Errorf("LLVM type emission: unsupported type kind %T", typeKind)
	}
}

func irNameSingle(ctx *IrCtx, nameNode *t.NodeNameSingle, withPackage bool) error {
	if withPackage {
		irWrite(ctx, ctx.fCtx.PackageName)
		irWrite(ctx, ".")
	}
	irWrite(ctx, nameNode.Name)
	return nil
}

func irNameSingleSsa(ctx *IrCtx, nameNode *t.NodeNameSingle, withPackage bool) SsaName {
	ssa := "%"

	if withPackage {
		ssa += ctx.fCtx.PackageName + "."
	}
	ssa += nameNode.Name
	return ssaName(ssa)
}

func irNameComposite(ctx *IrCtx, nameNode *t.NodeNameComposite, withPackage bool) error {
	bound := len(nameNode.Parts)
	for i, n := range nameNode.Parts {

		if i == 0 {
			first := nameNode.Parts[0]

			// if not imported package, prepend with <thispackage>.
			alias, ok := ctx.fCtx.GlNode.ImportAlias[first]
			if !ok {
				if withPackage {
					irWrite(ctx, ctx.fCtx.PackageName)
					irWrite(ctx, ".")
				}
				irWrite(ctx, first)
			} else {
				irWrite(ctx, alias)
			}
			goto next
		}

		irWrite(ctx, n)

	next:
		if i < bound-1 {
			irWrite(ctx, ".")
		}
	}

	return nil
}

func irNameCompositeSsa(ctx *IrCtx, nameNode *t.NodeNameComposite, withPackage bool) SsaName {
	ssa := "%"

	if withPackage {
		first := nameNode.Parts[0]

		// if not imported package, prepend with <thispackage>.
		_, ok := ctx.fCtx.ImportAlias[first]
		if !ok {
			ssa += ctx.fCtx.PackageName + "."
		}
	}

	bound := len(nameNode.Parts)
	for i, n := range nameNode.Parts {
		ssa += n
		if i < bound-1 {
			ssa += "."
		}
	}

	return ssaName(ssa)
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

func irNameSsa(ctx *IrCtx, nameNode t.NodeName, withPackage bool) SsaName {
	switch n := nameNode.(type) {
	case *t.NodeNameComposite:
		return irNameCompositeSsa(ctx, n, withPackage)
	case *t.NodeNameSingle:
		return irNameSingleSsa(ctx, n, withPackage)
	}
	return ssaName("")
}

func irTypeKind(ctx *IrCtx, typeKind t.NodeTypeKind) error {
	if err := validateIrTypeKind(typeKind); err != nil {
		return err
	}
	switch tn := typeKind.(type) {
	case *t.NodeTypeSlice:
		irWrite(ctx, "%type.slice")
		return nil
	case *t.NodeTypePointer:
		irWrite(ctx, "ptr")
		return nil
	case *t.NodeTypeRfc:
		irWrite(ctx, "ptr")
		return nil
	case *t.NodeTypeFunc:
		e := irFuncPtrType(ctx, tn)
		if e != nil {
			return e
		}
		return nil
	case *t.NodeTypeNamed:
		switch n := tn.NameNode.(type) {
		case *t.NodeNameSingle:
			// TODO: check if intrinsic type
			_, ok := magmatypes.BasicTypes[n.Name]
			if ok {
				irWrite(ctx, magmatypes.BasicTypes[n.Name])
				return nil
			}
		}

		return fmt.Errorf("LLVM type emission: unresolved named type")
	case *t.NodeTypeAbsolute:
		irWritef(ctx, "%%struct.%s", tn.AbsoluteName)
		return nil
	}
	return fmt.Errorf("LLVM type emission: unsupported type kind %T", typeKind)
}

func irThrowingType(ctx *IrCtx, typeNode *t.NodeType) error {
	if typeNode == nil {
		return fmt.Errorf("LLVM type emission: missing throwing type")
	}

	if isVoidType(typeNode) && typeNode.Throws {
		irWrite(ctx, "{ %type.error }")
		return nil
	}

	if typeNode.Throws {
		irWrite(ctx, "{ %type.error, ")
	}

	e := irTypeKind(ctx, typeNode.KindNode)
	if e != nil {
		return e
	}

	if typeNode.Throws {
		irWrite(ctx, " }")
	}
	return nil
}

func irType(ctx *IrCtx, typeNode *t.NodeType) error {
	if typeNode == nil {
		return fmt.Errorf("LLVM type emission: missing type")
	}

	return irTypeKind(ctx, typeNode.KindNode)
}
