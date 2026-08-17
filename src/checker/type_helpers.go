package checker

import (
	magmatypes "Magma/src/magma_types"
	t "Magma/src/types"
	"fmt"
)

func makeNamedType(name string) *t.NodeType {
	return &t.NodeType{
		Throws: false,
		KindNode: &t.NodeTypeNamed{
			NameNode: &t.NodeNameSingle{Name: name},
		},
	}
}

func makeFuncPtrTypeFromDef(fnDef *t.NodeFuncDef) *t.NodeType {
	k := &t.NodeTypeFunc{
		Args:       []*t.NodeType{},
		RetType:    fnDef.ReturnType,
		ContextABI: fnDef.ContextABI,
	}

	for _, v := range fnDef.Class.ArgsNode.Args {
		k.Args = append(k.Args, v.TypeNode)
	}

	return &t.NodeType{
		Throws:   false,
		KindNode: k,
	}
}

func makePtrType(from *t.NodeType) *t.NodeType {
	cpy := *from

	var kind t.NodeTypeKind

	switch n := cpy.KindNode.(type) {
	case *t.NodeTypeNamed:
		kind = &t.NodeTypePointer{
			Kind: n,
		}
	}

	return &t.NodeType{
		Throws:   cpy.Throws,
		KindNode: kind,
	}
}

func isVoidType(node *t.NodeType) bool {
	if node == nil {
		return false
	}

	switch n := node.KindNode.(type) {
	case *t.NodeTypeNamed:
		switch nn := n.NameNode.(type) {
		case *t.NodeNameSingle:
			return nn.Name == "void"
		}
	}
	return false
}

func isErrType(node *t.NodeType) bool {
	if node == nil {
		return false
	}

	switch n := node.KindNode.(type) {
	case *t.NodeTypeNamed:
		switch nn := n.NameNode.(type) {
		case *t.NodeNameSingle:
			return nn.Name == "error"
		}
	}
	return false
}

func isStrType(node *t.NodeType) bool {
	if node == nil {
		return false
	}
	named, ok := node.KindNode.(*t.NodeTypeNamed)
	if !ok {
		return false
	}
	single, ok := named.NameNode.(*t.NodeNameSingle)
	return ok && single.Name == "str"
}

func isBoolType(node *t.NodeType) bool {
	if node == nil {
		return false
	}

	switch n := node.KindNode.(type) {
	case *t.NodeTypeNamed:
		switch nn := n.NameNode.(type) {
		case *t.NodeNameSingle:
			return nn.Name == "bool"
		}
	}
	return false
}

func isPointerType(node *t.NodeType) bool {
	if node == nil {
		return false
	}

	switch n := node.KindNode.(type) {
	case *t.NodeTypePointer:
		return true
	case *t.NodeTypeRfc:
		return true
	case *t.NodeTypeNamed:
		switch nn := n.NameNode.(type) {
		case *t.NodeNameSingle:
			return nn.Name == "ptr"
		}
	}
	return false
}

func isNumberType(node *t.NodeType) bool {
	if node == nil {
		return false
	}

	switch n := node.KindNode.(type) {
	case *t.NodeTypeNamed:
		switch nn := n.NameNode.(type) {
		case *t.NodeNameSingle:
			_, ok := magmatypes.NumberTypes[nn.Name]
			return ok
		}
	}
	return false
}

func isFloatType(node *t.NodeType) bool {
	if node == nil {
		return false
	}

	switch n := node.KindNode.(type) {
	case *t.NodeTypeNamed:
		switch nn := n.NameNode.(type) {
		case *t.NodeNameSingle:
			desc, ok := magmatypes.NumberTypes[nn.Name]
			return ok && desc.IsFloat
		}
	}
	return false
}

func isIntegerType(node *t.NodeType) bool {
	if node == nil || !isNumberType(node) {
		return false
	}
	if isFloatType(node) {
		return false
	}
	if isPointerType(node) {
		return false
	}
	return true
}

func numericDescriptor(node *t.NodeType) (magmatypes.NumberType, bool) {
	if node == nil {
		return magmatypes.NumberType{}, false
	}
	named, ok := node.KindNode.(*t.NodeTypeNamed)
	if !ok {
		return magmatypes.NumberType{}, false
	}
	single, ok := named.NameNode.(*t.NodeNameSingle)
	if !ok {
		return magmatypes.NumberType{}, false
	}
	desc, ok := magmatypes.NumberTypes[single.Name]
	return desc, ok
}

// numericPromotionType chooses the widest representation. Mixed integer/float
// operations use the floating operand's representation; equal-width operands
// retain the left type to preserve established signedness behavior.
func numericPromotionType(left, right *t.NodeType) *t.NodeType {
	leftDesc, leftOK := numericDescriptor(left)
	rightDesc, rightOK := numericDescriptor(right)
	if !leftOK || !rightOK {
		return nil
	}
	if leftDesc.IsFloat != rightDesc.IsFloat {
		width := leftDesc.ByteSize
		if rightDesc.ByteSize > width {
			width = rightDesc.ByteSize
		}
		return makeNamedType(fmt.Sprintf("f%d", width))
	}
	if rightDesc.ByteSize > leftDesc.ByteSize {
		return right
	}
	return left
}

func numericPromotionForExpressions(left, right t.NodeExpr) *t.NodeType {
	leftType := left.GetInferredType()
	rightType := right.GetInferredType()
	leftLiteral, leftIsLiteral := left.(*t.NodeExprLit)
	rightLiteral, rightIsLiteral := right.(*t.NodeExprLit)
	if leftIsLiteral && leftLiteral.LitType == t.TokLitNum && !(rightIsLiteral && rightLiteral.LitType == t.TokLitNum) {
		leftType = rightType
	}
	if rightIsLiteral && rightLiteral.LitType == t.TokLitNum && !(leftIsLiteral && leftLiteral.LitType == t.TokLitNum) {
		rightType = leftType
	}
	return numericPromotionType(leftType, rightType)
}

func isArrayType(node *t.NodeType) bool {
	if node == nil {
		return false
	}

	switch node.KindNode.(type) {
	case *t.NodeTypeNamed:
		return false
	case *t.NodeTypeRfc:
		return false
	case *t.NodeTypePointer:
		return true
	case *t.NodeTypeSlice:
		return true
	}
	return false
}

func isUntypedSlice(node *t.NodeType) bool {
	if node == nil {
		return false
	}
	named, ok := node.KindNode.(*t.NodeTypeNamed)
	if !ok {
		return false
	}
	single, ok := named.NameNode.(*t.NodeNameSingle)
	return ok && single.Name == "slice"
}

func isTypedSlice(node *t.NodeType) bool {
	if node == nil {
		return false
	}
	_, ok := node.KindNode.(*t.NodeTypeSlice)
	return ok
}

func getBoxedType(node *t.NodeType) *t.NodeType {
	if node == nil {
		return nil
	}

	switch n := node.KindNode.(type) {
	case *t.NodeTypeNamed:
		return nil
	case *t.NodeTypeRfc:
		return &t.NodeType{
			Throws:   node.Throws,
			KindNode: n.Kind,
		}
	case *t.NodeTypePointer:
		return &t.NodeType{
			Throws:   node.Throws,
			KindNode: n.Kind,
		}
	case *t.NodeTypeSlice:
		return &t.NodeType{
			Throws:   node.Throws,
			KindNode: n.ElemKind,
		}
	}
	return nil
}

func flattenType(node *t.NodeType) string {
	return t.DisplayType(node)
}
