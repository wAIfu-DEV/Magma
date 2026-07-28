package llvmir

import (
	magmatypes "Magma/src/magma_types"
	t "Magma/src/types"
	"bytes"
	"fmt"
	"strings"
)

type cABIClass uint8

const (
	cABIDirect cABIClass = iota
	cABIIndirect
	cABICoerce
)

type cABIPart struct {
	llvmType string
	offset   int
}

type cABIValue struct {
	kind       cABIClass
	logical    string
	size       int
	align      int
	parts      []cABIPart
	byVal      bool
	returnSRet bool
}

func cABITypeString(ctx *IrCtx, typ *t.NodeType) (string, error) {
	b := &bytes.Buffer{}
	cpy := *ctx
	cpy.bld = ScopeBuilder{Struct: b, Global: b, Head: b, Body: b, Tail: b, StructM: ctx.bld.StructM}
	if err := irType(&cpy, typ); err != nil {
		return "", err
	}
	return strings.TrimSpace(b.String()), nil
}

func cABIStructDef(ctx *IrCtx, absolute string) *t.StructDef {
	for _, file := range ctx.Shared.Files {
		if file.GlNode == nil {
			continue
		}
		for _, def := range file.GlNode.StructDefs {
			if def.Module+"."+def.Name == absolute {
				return def
			}
		}
	}
	return nil
}

func cABIAlignUp(value, alignment int) int {
	if alignment <= 1 {
		return value
	}
	return (value + alignment - 1) &^ (alignment - 1)
}

type cABILayout struct {
	size, align int
	aggregate   bool
	leaves      []cABILeaf
}

type cABILeaf struct {
	offset  int
	size    int
	isFloat bool
}

func cABITypeLayout(ctx *IrCtx, typ *t.NodeType) (cABILayout, error) {
	if typ == nil {
		return cABILayout{}, fmt.Errorf("C ABI: missing type")
	}
	switch n := typ.KindNode.(type) {
	case *t.NodeTypePointer, *t.NodeTypeRfc, *t.NodeTypeFunc:
		bytes := ctx.Shared.Target.PointerBits / 8
		return cABILayout{size: bytes, align: bytes}, nil
	case *t.NodeTypeSlice:
		bytes := ctx.Shared.Target.PointerBits / 8
		return cABILayout{size: bytes * 2, align: bytes, aggregate: true,
			leaves: []cABILeaf{{offset: 0, size: bytes}, {offset: bytes, size: bytes}}}, nil
	case *t.NodeTypeNamed:
		if single, ok := n.NameNode.(*t.NodeNameSingle); ok {
			if single.Name == "void" {
				return cABILayout{}, nil
			}
			if single.Name == "bool" {
				return cABILayout{size: 1, align: 1, leaves: []cABILeaf{{size: 1}}}, nil
			}
			if desc, ok := magmatypes.NumberTypes[single.Name]; ok {
				bytes := desc.ByteSize / 8
				return cABILayout{size: bytes, align: bytes, leaves: []cABILeaf{{size: bytes, isFloat: desc.IsFloat}}}, nil
			}
			if single.Name == "ptr" {
				bytes := ctx.Shared.Target.PointerBits / 8
				return cABILayout{size: bytes, align: bytes, leaves: []cABILeaf{{size: bytes}}}, nil
			}
		}
	case *t.NodeTypeAbsolute:
		def := cABIStructDef(ctx, n.AbsoluteName)
		if def == nil {
			return cABILayout{}, fmt.Errorf("C ABI: cannot resolve struct %s", t.DisplayType(&t.NodeType{KindNode: n}))
		}
		result := cABILayout{aggregate: true, align: 1}
		for _, name := range def.FieldOrder {
			field, err := cABITypeLayout(ctx, def.Fields[name])
			if err != nil {
				return cABILayout{}, err
			}
			result.size = cABIAlignUp(result.size, field.align)
			base := result.size
			for _, leaf := range field.leaves {
				leaf.offset += base
				result.leaves = append(result.leaves, leaf)
			}
			result.size += field.size
			if field.align > result.align {
				result.align = field.align
			}
		}
		result.size = cABIAlignUp(result.size, result.align)
		return result, nil
	}
	return cABILayout{}, fmt.Errorf("C ABI: unsupported type %T", typ.KindNode)
}

func cABIIntegerType(size int) string {
	if size < 1 {
		size = 1
	}
	return fmt.Sprintf("i%d", size*8)
}

func cABISysVParts(layout cABILayout) []cABIPart {
	count := (layout.size + 7) / 8
	parts := make([]cABIPart, 0, count)
	for chunk := 0; chunk < count; chunk++ {
		start, end := chunk*8, (chunk+1)*8
		if end > layout.size {
			end = layout.size
		}
		allFloat := true
		floatBytes := 0
		for _, leaf := range layout.leaves {
			if leaf.offset >= end || leaf.offset+leaf.size <= start {
				continue
			}
			if !leaf.isFloat {
				allFloat = false
				break
			}
			floatBytes += leaf.size
		}
		typ := cABIIntegerType(end - start)
		if allFloat && floatBytes == end-start {
			switch floatBytes {
			case 4:
				typ = "float"
			case 8:
				// Two f32 fields form one SSE register; one f64 remains double.
				isDouble := false
				for _, leaf := range layout.leaves {
					if leaf.offset >= start && leaf.offset < end && leaf.size == 8 {
						isDouble = true
					}
				}
				if isDouble {
					typ = "double"
				} else {
					typ = "<2 x float>"
				}
			}
		}
		parts = append(parts, cABIPart{llvmType: typ, offset: start})
	}
	return parts
}

func cABIIsAArch64HFA(layout cABILayout) bool {
	if len(layout.leaves) == 0 || len(layout.leaves) > 4 {
		return false
	}
	elementSize := layout.leaves[0].size
	if !layout.leaves[0].isFloat || (elementSize != 2 && elementSize != 4 && elementSize != 8 && elementSize != 16) {
		return false
	}
	for _, leaf := range layout.leaves[1:] {
		if !leaf.isFloat || leaf.size != elementSize {
			return false
		}
	}
	return true
}

func cABIClassify(ctx *IrCtx, typ *t.NodeType, isReturn bool) (cABIValue, error) {
	logical, err := cABITypeString(ctx, typ)
	if err != nil {
		return cABIValue{}, err
	}
	layout, err := cABITypeLayout(ctx, typ)
	if err != nil {
		return cABIValue{}, err
	}
	value := cABIValue{kind: cABIDirect, logical: logical, size: layout.size, align: layout.align}
	if !layout.aggregate {
		return value, nil
	}

	arch, os := string(ctx.Shared.Target.Arch), string(ctx.Shared.Target.OS)
	if arch == "x86_64" && os == "windows" {
		if layout.size == 1 || layout.size == 2 || layout.size == 4 || layout.size == 8 {
			value.kind = cABICoerce
			value.parts = []cABIPart{{llvmType: cABIIntegerType(layout.size)}}
			return value, nil
		}
		value.kind = cABIIndirect
		value.returnSRet = isReturn
		return value, nil
	}
	if arch == "x86_64" {
		if layout.size > 16 {
			value.kind = cABIIndirect
			value.byVal = !isReturn
			value.returnSRet = isReturn
			return value, nil
		}
		value.kind = cABICoerce
		value.parts = cABISysVParts(layout)
		return value, nil
	}
	if arch == "aarch64" || arch == "arm64" {
		if layout.size > 16 && !cABIIsAArch64HFA(layout) {
			value.kind = cABIIndirect
			value.returnSRet = isReturn
		}
		// AArch64 passes small aggregates, including HFAs, in their logical form.
		return value, nil
	}
	return value, nil
}

func cABIReturnType(plan cABIValue) string {
	if plan.kind == cABIIndirect {
		return "void"
	}
	if plan.kind != cABICoerce {
		return plan.logical
	}
	if len(plan.parts) == 1 {
		return plan.parts[0].llvmType
	}
	parts := make([]string, len(plan.parts))
	for i := range plan.parts {
		parts[i] = plan.parts[i].llvmType
	}
	return "{ " + strings.Join(parts, ", ") + " }"
}

func cABIIndirectSpelling(plan cABIValue, ssa string, isReturn bool) string {
	if isReturn {
		return fmt.Sprintf("ptr sret(%s) align %d %s", plan.logical, plan.align, ssa)
	}
	if plan.byVal {
		return fmt.Sprintf("ptr byval(%s) align %d %s", plan.logical, plan.align, ssa)
	}
	return "ptr " + ssa
}

func irCABIExternalDeclaration(ctx *IrCtx, fn *t.NodeFuncDef) error {
	ret, err := cABIClassify(ctx, fn.ReturnType, true)
	if err != nil {
		return err
	}
	args := make([]cABIValue, len(fn.Class.ArgsNode.Args))
	for i, arg := range fn.Class.ArgsNode.Args {
		args[i], err = cABIClassify(ctx, arg.TypeNode, false)
		if err != nil {
			return err
		}
	}

	irWritef(ctx, "declare %s @%s(", cABIReturnType(ret), fn.NoAliasName)
	items := []string{}
	if ret.kind == cABIIndirect {
		items = append(items, cABIIndirectSpelling(ret, "", true))
	}
	for _, arg := range args {
		switch arg.kind {
		case cABIIndirect:
			items = append(items, strings.TrimSpace(cABIIndirectSpelling(arg, "", false)))
		case cABICoerce:
			for _, part := range arg.parts {
				items = append(items, part.llvmType)
			}
		default:
			items = append(items, arg.logical)
		}
	}
	irWrite(ctx, strings.Join(items, ", "))
	irWrite(ctx, ")\n")
	return nil
}

func irCABIStoreLogical(ctx *IrCtx, plan cABIValue, value SsaName) SsaName {
	tmp := irSsaLocal(ctx)
	irWritef(ctx, "  %s = alloca %s, align %d\n", tmp.Repr, plan.logical, plan.align)
	irWritef(ctx, "  store %s ", plan.logical)
	irPossibleLitSsa(ctx, value)
	irWritef(ctx, ", ptr %s, align %d\n", tmp.Repr, plan.align)
	return tmp
}

func irCABILoadParts(ctx *IrCtx, plan cABIValue, source SsaName) []SsaName {
	result := make([]SsaName, len(plan.parts))
	for i, part := range plan.parts {
		address := source
		if part.offset != 0 {
			address = irSsaLocal(ctx)
			irWritef(ctx, "  %s = getelementptr i8, ptr %s, i64 %d\n", address.Repr, source.Repr, part.offset)
		}
		result[i] = irSsaLocal(ctx)
		irWritef(ctx, "  %s = load %s, ptr %s, align 1\n", result[i].Repr, part.llvmType, address.Repr)
	}
	return result
}

func irCABIExternalCall(ctx *IrCtx, fnCall *t.NodeExprCall, argsSsa []SsaName, topLevel bool) (SsaName, error) {
	fn := fnCall.AssociatedFnDef
	ret, err := cABIClassify(ctx, fn.ReturnType, true)
	if err != nil {
		return SsaName{}, err
	}
	argPlans := make([]cABIValue, len(fn.Class.ArgsNode.Args))
	for i, arg := range fn.Class.ArgsNode.Args {
		argPlans[i], err = cABIClassify(ctx, arg.TypeNode, false)
		if err != nil {
			return SsaName{}, err
		}
	}

	type callArg struct{ spelling, value string }
	callArgs := []callArg{}
	var resultStorage SsaName
	if ret.kind == cABIIndirect {
		resultStorage = irSsaLocal(ctx)
		irWritef(ctx, "  %s = alloca %s, align %d\n", resultStorage.Repr, ret.logical, ret.align)
		callArgs = append(callArgs, callArg{
			spelling: fmt.Sprintf("ptr sret(%s) align %d", ret.logical, ret.align),
			value:    resultStorage.Repr,
		})
	}

	for i, plan := range argPlans {
		switch plan.kind {
		case cABIIndirect:
			tmp := irCABIStoreLogical(ctx, plan, argsSsa[i])
			spelling := "ptr"
			if plan.byVal {
				spelling = fmt.Sprintf("ptr byval(%s) align %d", plan.logical, plan.align)
			}
			callArgs = append(callArgs, callArg{spelling: spelling, value: tmp.Repr})
		case cABICoerce:
			tmp := irCABIStoreLogical(ctx, plan, argsSsa[i])
			parts := irCABILoadParts(ctx, plan, tmp)
			for j, part := range plan.parts {
				callArgs = append(callArgs, callArg{spelling: part.llvmType, value: parts[j].Repr})
			}
		default:
			value := argsSsa[i].Repr
			if argsSsa[i].IsLiteral {
				value = argsSsa[i].Repr
			}
			callArgs = append(callArgs, callArg{spelling: plan.logical, value: value})
		}
	}

	rawResult := SsaName{}
	physicalReturn := cABIReturnType(ret)
	if ret.kind != cABIIndirect && physicalReturn != "void" && !topLevel {
		rawResult = irSsaLocal(ctx)
		irWritef(ctx, "  %s = ", rawResult.Repr)
	} else {
		irWrite(ctx, "  ")
	}
	irWritef(ctx, "call %s @%s(", physicalReturn, fn.NoAliasName)
	for i, arg := range callArgs {
		if i != 0 {
			irWrite(ctx, ", ")
		}
		irWritef(ctx, "%s %s", arg.spelling, arg.value)
	}
	irWrite(ctx, ")\n")

	if topLevel || physicalReturn == "void" && ret.kind != cABIIndirect {
		return SsaName{}, nil
	}
	if ret.kind == cABIIndirect {
		result := irSsaLocal(ctx)
		irWritef(ctx, "  %s = load %s, ptr %s, align %d\n", result.Repr, ret.logical, resultStorage.Repr, ret.align)
		return result, nil
	}
	if ret.kind != cABICoerce {
		return rawResult, nil
	}

	tmp := irSsaLocal(ctx)
	irWritef(ctx, "  %s = alloca %s, align %d\n", tmp.Repr, ret.logical, ret.align)
	irWritef(ctx, "  store %s %s, ptr %s, align %d\n", physicalReturn, rawResult.Repr, tmp.Repr, ret.align)
	result := irSsaLocal(ctx)
	irWritef(ctx, "  %s = load %s, ptr %s, align %d\n", result.Repr, ret.logical, tmp.Repr, ret.align)
	return result, nil
}

func irCABIExportWrapper(ctx *IrCtx, fn *t.NodeFuncDef) error {
	ret, err := cABIClassify(ctx, fn.ReturnType, true)
	if err != nil {
		return err
	}
	argPlans := make([]cABIValue, len(fn.Class.ArgsNode.Args))
	for i, arg := range fn.Class.ArgsNode.Args {
		argPlans[i], err = cABIClassify(ctx, arg.TypeNode, false)
		if err != nil {
			return err
		}
	}

	type parameter struct{ spelling, name string }
	params := []parameter{}
	if ret.kind == cABIIndirect {
		params = append(params, parameter{
			spelling: fmt.Sprintf("ptr sret(%s) align %d", ret.logical, ret.align),
			name:     "%cabi.sret",
		})
	}
	for i, plan := range argPlans {
		switch plan.kind {
		case cABIIndirect:
			spelling := "ptr"
			if plan.byVal {
				spelling = fmt.Sprintf("ptr byval(%s) align %d", plan.logical, plan.align)
			}
			params = append(params, parameter{spelling: spelling, name: fmt.Sprintf("%%cabi.arg.%d", i)})
		case cABICoerce:
			for j, part := range plan.parts {
				params = append(params, parameter{spelling: part.llvmType, name: fmt.Sprintf("%%cabi.arg.%d.%d", i, j)})
			}
		default:
			params = append(params, parameter{spelling: plan.logical, name: "%" + fn.Class.ArgsNode.Args[i].Name})
		}
	}

	irWritef(ctx, "define %s @%s(", cABIReturnType(ret), fn.ExportName)
	for i, param := range params {
		if i != 0 {
			irWrite(ctx, ", ")
		}
		irWritef(ctx, "%s %s", param.spelling, param.name)
	}
	irWrite(ctx, ") {\n")

	logicalArgs := make([]SsaName, len(argPlans))
	for i, plan := range argPlans {
		switch plan.kind {
		case cABIIndirect:
			logicalArgs[i] = irSsaLocal(ctx)
			irWritef(ctx, "  %s = load %s, ptr %%cabi.arg.%d, align %d\n", logicalArgs[i].Repr, plan.logical, i, plan.align)
		case cABICoerce:
			tmp := irSsaLocal(ctx)
			irWritef(ctx, "  %s = alloca %s, align %d\n", tmp.Repr, plan.logical, plan.align)
			for j, part := range plan.parts {
				address := tmp
				if part.offset != 0 {
					address = irSsaLocal(ctx)
					irWritef(ctx, "  %s = getelementptr i8, ptr %s, i64 %d\n", address.Repr, tmp.Repr, part.offset)
				}
				irWritef(ctx, "  store %s %%cabi.arg.%d.%d, ptr %s, align 1\n", part.llvmType, i, j, address.Repr)
			}
			logicalArgs[i] = irSsaLocal(ctx)
			irWritef(ctx, "  %s = load %s, ptr %s, align %d\n", logicalArgs[i].Repr, plan.logical, tmp.Repr, plan.align)
		default:
			logicalArgs[i] = SsaName{Repr: "%" + fn.Class.ArgsNode.Args[i].Name}
		}
	}

	returnsVoid := ret.logical == "void"
	logicalResult := SsaName{}
	if !returnsVoid {
		logicalResult = SsaName{Repr: "%export.ret"}
		irWritef(ctx, "  %s = ", logicalResult.Repr)
	} else {
		irWrite(ctx, "  ")
	}
	irWritef(ctx, "call %s @%s(", ret.logical, fn.AbsName)
	for i, arg := range logicalArgs {
		if i != 0 {
			irWrite(ctx, ", ")
		}
		irWritef(ctx, "%s %s", argPlans[i].logical, arg.Repr)
	}
	irWrite(ctx, ")\n")

	if returnsVoid {
		irWrite(ctx, "  ret void\n}\n")
		return nil
	}
	if ret.kind == cABIIndirect {
		irWritef(ctx, "  store %s %s, ptr %%cabi.sret, align %d\n", ret.logical, logicalResult.Repr, ret.align)
		irWrite(ctx, "  ret void\n}\n")
		return nil
	}
	if ret.kind != cABICoerce {
		irWritef(ctx, "  ret %s %s\n}\n", ret.logical, logicalResult.Repr)
		return nil
	}

	tmp := irCABIStoreLogical(ctx, ret, logicalResult)
	physical := cABIReturnType(ret)
	coerced := irSsaLocal(ctx)
	irWritef(ctx, "  %s = load %s, ptr %s, align %d\n", coerced.Repr, physical, tmp.Repr, ret.align)
	irWritef(ctx, "  ret %s %s\n}\n", physical, coerced.Repr)
	return nil
}
