package llvmir

import (
	t "Magma/src/types"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

func irGlVarDef(ctx *IrCtx, vd *t.NodeExprVarDef) error {
	cpy := *ctx
	cpy.bld.Body = ctx.bld.Head
	if vd.Initializer != nil {
		irPrepareConstStrings(&cpy, vd.Initializer, map[t.NodeExpr]bool{})
	}

	//irWritef(&cpy, "@%s = internal global ", vd.AbsName)
	irWritef(&cpy, "@%s = private thread_local global ", vd.AbsName)
	//irWritef(&cpy, "@%s = private static thread_local global ", vd.AbsName)

	e := irType(&cpy, vd.Type)
	if e != nil {
		return e
	}

	irWrite(&cpy, " ")
	if vd.Initializer == nil {
		irWrite(&cpy, "zeroinitializer")
	} else if e := irConstValue(&cpy, vd.Type, vd.Initializer); e != nil {
		return e
	}
	irWrite(&cpy, "\n")
	return nil
}

func irConstValue(ctx *IrCtx, expected *t.NodeType, expr t.NodeExpr) error {
	switch n := expr.(type) {
	case *t.NodeExprLit:
		switch n.LitType {
		case t.TokLitNum, t.TokLitBool:
			irWrite(ctx, n.Value)
			return nil
		case t.TokLitStr:
			stringGlobal, ok := ctx.constStrings[n]
			if !ok {
				return fmt.Errorf("global string constant was not prepared")
			}
			irWritef(ctx, "{ ptr %s, i64 %d }", stringGlobal.Repr, len(n.Value))
			return nil
		case t.TokLitNone:
			irWrite(ctx, "null")
			return nil
		default:
			return fmt.Errorf("string literals are not supported in global constants")
		}
	case *t.NodeExprName:
		switch associated := n.AssociatedNode.(type) {
		case *t.NodeFuncDef:
			name := associated.AbsName
			if associated.NoAliasName != "" {
				name = associated.NoAliasName
			}
			irWritef(ctx, "@%s", name)
			return nil
		case *t.NodeExprVarDef:
			if associated.IsConst && associated.Initializer != nil {
				return irConstValue(ctx, expected, associated.Initializer)
			}
		}
		return fmt.Errorf("constant name '%s' is not a constant or function symbol", flattenName(n.Name))
	case *t.NodeExprAddrof:
		name, ok := n.Expr.(*t.NodeExprName)
		if !ok {
			return fmt.Errorf("constant addrof requires a global name")
		}
		varDef, ok := name.AssociatedNode.(*t.NodeExprVarDef)
		if !ok || !varDef.IsGlobal {
			return fmt.Errorf("constant addrof requires a global name")
		}
		irWritef(ctx, "@%s", varDef.AbsName)
		return nil
	case *t.NodeExprStructInit:
		fields := slices.Clone(n.Fields)
		slices.SortFunc(fields, func(a, b t.NodeStructFieldInit) int { return a.FieldIndex - b.FieldIndex })
		irWrite(ctx, "{ ")
		for i, field := range fields {
			if e := irType(ctx, field.FieldType); e != nil {
				return e
			}
			irWrite(ctx, " ")
			if e := irConstValue(ctx, field.FieldType, field.Expression); e != nil {
				return e
			}
			if i+1 < len(fields) {
				irWrite(ctx, ", ")
			}
		}
		irWrite(ctx, " }")
		return nil
	case *t.NodeExprArray:
		return fmt.Errorf("array constants require backing storage")
	default:
		return fmt.Errorf("expression %T is not supported in a global constant", expr)
	}
}

func irPrepareConstStrings(ctx *IrCtx, expr t.NodeExpr, seen map[t.NodeExpr]bool) {
	if expr == nil || seen[expr] {
		return
	}
	seen[expr] = true
	switch n := expr.(type) {
	case *t.NodeExprLit:
		if n.LitType == t.TokLitStr {
			if ctx.constStrings == nil {
				ctx.constStrings = map[*t.NodeExprLit]SsaName{}
			}
			if _, exists := ctx.constStrings[n]; !exists {
				ctx.constStrings[n] = irCStringGlobal(ctx, n.Value)
			}
		}
	case *t.NodeExprName:
		if variable, ok := n.AssociatedNode.(*t.NodeExprVarDef); ok && variable.IsConst {
			irPrepareConstStrings(ctx, variable.Initializer, seen)
		}
	case *t.NodeExprStructInit:
		for _, field := range n.Fields {
			irPrepareConstStrings(ctx, field.Expression, seen)
		}
	case *t.NodeExprArray:
		for _, entry := range n.Entries {
			irPrepareConstStrings(ctx, entry.Value, seen)
		}
	}
}

func irConstUint(expr t.NodeExpr) (uint64, bool) {
	switch n := expr.(type) {
	case *t.NodeExprLit:
		if n.LitType != t.TokLitNum {
			return 0, false
		}
		repr := strings.ReplaceAll(n.Value, "_", "")
		base := 10
		if strings.HasPrefix(repr, "0x") || strings.HasPrefix(repr, "0X") || strings.HasPrefix(repr, "0b") || strings.HasPrefix(repr, "0B") || strings.HasPrefix(repr, "0o") || strings.HasPrefix(repr, "0O") {
			base = 0
		}
		value, err := strconv.ParseUint(repr, base, 64)
		return value, err == nil
	case *t.NodeExprName:
		variable, ok := n.AssociatedNode.(*t.NodeExprVarDef)
		if !ok || !variable.IsConst || variable.Initializer == nil {
			return 0, false
		}
		return irConstUint(variable.Initializer)
	default:
		return 0, false
	}
}

func irConstArrayDef(ctx *IrCtx, def *t.NodeConstDef, array *t.NodeExprArray) error {
	length, ok := irConstUint(array.Length)
	if !ok {
		return fmt.Errorf("constant array length is not an integer constant")
	}
	values := make(map[uint64]t.NodeExpr, len(array.Entries))
	for _, entry := range array.Entries {
		values[entry.ResolvedIndex] = entry.Value
	}
	c := *ctx
	c.bld.Body, c.bld.Head, c.bld.Tail = ctx.bld.Global, ctx.bld.Global, ctx.bld.Global
	irWriteGlf(ctx, "@%s.data = private global [%d x ", def.VarDef.AbsName, length)
	if err := irType(&c, array.ElemType); err != nil {
		return err
	}
	irWriteGl(ctx, "] [")
	for i := uint64(0); i < length; i++ {
		if i != 0 {
			irWriteGl(ctx, ", ")
		}
		if err := irType(&c, array.ElemType); err != nil {
			return err
		}
		irWriteGl(ctx, " ")
		if value, exists := values[i]; exists {
			if err := irConstValue(&c, array.ElemType, value); err != nil {
				return err
			}
		} else {
			irWriteGl(ctx, "zeroinitializer")
		}
	}
	irWriteGl(ctx, "]\n")
	irWriteGlf(ctx, "@%s = private constant %%type.slice { ptr @%s.data, i64 %d }\n", def.VarDef.AbsName, def.VarDef.AbsName, length)
	return nil
}

func irConstArrayInitializer(expr t.NodeExpr) (*t.NodeExprArray, bool) {
	switch n := expr.(type) {
	case *t.NodeExprArray:
		return n, true
	case *t.NodeExprName:
		variable, ok := n.AssociatedNode.(*t.NodeExprVarDef)
		if ok && variable.IsConst && variable.Initializer != nil {
			return irConstArrayInitializer(variable.Initializer)
		}
	}
	return nil, false
}

func irConstDef(ctx *IrCtx, def *t.NodeConstDef) error {
	irPrepareConstStrings(ctx, def.Initializer, map[t.NodeExpr]bool{})
	if array, ok := irConstArrayInitializer(def.Initializer); ok {
		return irConstArrayDef(ctx, def, array)
	}
	irWriteGlf(ctx, "@%s = private constant ", def.VarDef.AbsName)
	cpy := *ctx
	cpy.bld.Body = ctx.bld.Global
	cpy.bld.Head = ctx.bld.Global
	cpy.bld.Tail = ctx.bld.Global
	if e := irType(&cpy, def.VarDef.Type); e != nil {
		return e
	}
	irWriteGl(ctx, " ")
	if e := irConstValue(&cpy, def.VarDef.Type, def.Initializer); e != nil {
		return e
	}
	irWriteGl(ctx, "\n")
	return nil
}

func irVarDef(ctx *IrCtx, vd *t.NodeExprVarDef) (SsaName, error) {
	if vd == nil || vd.Storage != t.VariableStorageLocal {
		return SsaName{}, fmt.Errorf("cannot lower non-local variable definition as a local stack slot")
	}
	assignLocalIrName(ctx, vd)
	allocSsa := ctx.localSlots[vd]

	/* DEPRECATED
	if vd.Type.Destructor != nil {
		irWrite(ctx, "  ; has destructor\n")
	}*/

	// Fixed-size local slots must be allocated in the function entry block.
	// Emitting an alloca into a nested scope makes it execute every time that
	// scope is entered; for a loop body, those allocations accumulate until the
	// function returns and can exhaust the stack. Keep initialization below at
	// the declaration point so each scope entry still resets the variable.
	cpy := *ctx
	cpy.bld.Body = ctx.parentBld.Head
	irWritef(&cpy, "  %s = alloca ", allocSsa.Repr)

	e := irType(&cpy, vd.Type)
	if e != nil {
		return ssaName(""), e
	}
	irWrite(&cpy, "\n")

	irWrite(ctx, "  store ")
	e = irType(ctx, vd.Type)
	if e != nil {
		return ssaName(""), e
	}
	irWritef(ctx, " zeroinitializer, ptr %s\n", allocSsa.Repr)

	return allocSsa, nil
}

func irPossibleLitSsa(ctx *IrCtx, ssa SsaName) {
	if ssa.IsLiteral {
		irWrite(ctx, ssa.Repr)
	} else {
		irWritef(ctx, "%s", ssa.Repr)
	}
}

func irVarDefAssign(ctx *IrCtx, vda *t.NodeExprVarDefAssign) (SsaName, error) {
	assignSsa, e := irExpression(ctx, vda.VarDef.Type, vda.AssignExpr, false)
	if e != nil {
		return ssaName(""), e
	}
	assignSsa, e = irCoerceNumeric(ctx, vda.VarDef.Type, vda.AssignExpr, assignSsa)
	if e != nil {
		return ssaName(""), e
	}

	allocSsa, e := irVarDef(ctx, vda.VarDef)
	if e != nil {
		return ssaName(""), e
	}

	// TODO: this assumes we correctly infer the expression type during type checking,
	// but we don't, we need to make sure the inference rules mirror number promotion
	/*
		if isNumberType(vda.GetInferredType()) {
			if !isSameNumType(vda.GetInferredType(), vda.AssignExpr.GetInferredType()) {
				if !assignSsa.IsLiteral {
					return SsaName{}, comp_err.CompilationErrorToken(
						ctx.fCtx,
						&vda.Tk,
						"implicit number cast is forbidden on assignment",
						fmt.Sprintf("left side type is: %s, right side type is: %s", t.DisplayType(vda.GetInferredType()), t.DisplayType(vda.AssignExpr.GetInferredType())),
					)
				}
			}
		}*/

	irWrite(ctx, "  store ")
	e = irType(ctx, vda.VarDef.Type)
	if e != nil {
		return ssaName(""), e
	}

	irWrite(ctx, " ")
	irPossibleLitSsa(ctx, assignSsa)

	irWritef(ctx, ", ptr %s\n", allocSsa.Repr)
	return allocSsa, nil
}

func irExprStructInit(ctx *IrCtx, init *t.NodeExprStructInit) (SsaName, error) {
	current := SsaName{Repr: "zeroinitializer", IsLiteral: true}
	for _, field := range init.Fields {
		value, e := irExpression(ctx, field.FieldType, field.Expression, false)
		if e != nil {
			return SsaName{}, e
		}
		value, e = irCoerceNumeric(ctx, field.FieldType, field.Expression, value)
		if e != nil {
			return SsaName{}, e
		}
		next := irSsaLocal(ctx)
		irWritef(ctx, "  %s = insertvalue ", next.Repr)
		if e := irType(ctx, init.Type); e != nil {
			return SsaName{}, e
		}
		irWrite(ctx, " ")
		irPossibleLitSsa(ctx, current)
		irWrite(ctx, ", ")
		if e := irType(ctx, field.FieldType); e != nil {
			return SsaName{}, e
		}
		irWrite(ctx, " ")
		irPossibleLitSsa(ctx, value)
		irWritef(ctx, ", %d\n", field.FieldIndex)
		current = next
	}
	return current, nil
}

func irExprProtoView(ctx *IrCtx, view *t.NodeExprProtoView) (SsaName, error) {
	if view.Implementation == nil || view.Implementation.Owner == nil || view.Implementation.Proto == nil {
		return SsaName{}, fmt.Errorf("cannot lower unresolved prototype view")
	}
	var target SsaName
	var err error
	if view.TargetIsPointer {
		target, err = irExpression(ctx, view.Target.GetInferredType(), view.Target, false)
	} else {
		target, err = irExpressionLvalue(ctx, view.Target)
	}
	if err != nil {
		return SsaName{}, err
	}
	current := SsaName{Repr: "zeroinitializer", IsLiteral: true}
	withImpl := irSsaLocal(ctx)
	irWritef(ctx, "  %s = insertvalue ", withImpl.Repr)
	if err := irType(ctx, view.ProtoType); err != nil {
		return SsaName{}, err
	}
	irWritef(ctx, " %s, ptr %s, 0\n", current.Repr, target.Repr)
	withTable := irSsaLocal(ctx)
	irWritef(ctx, "  %s = insertvalue ", withTable.Repr)
	if err := irType(ctx, view.ProtoType); err != nil {
		return SsaName{}, err
	}
	irWritef(ctx, " %s, ptr @%s, 1\n", withImpl.Repr, t.ProtoVtableSymbol(view.Implementation.Owner, view.Implementation.Proto))
	return withTable, nil
}
