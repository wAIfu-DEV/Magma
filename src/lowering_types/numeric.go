package loweringtypes

import (
	"fmt"
	"math/big"

	lb "Magma/src/lowering_backend"
	magmatypes "Magma/src/magma_types"
	t "Magma/src/types"
)

func numericDescription(node *t.NodeType) (magmatypes.NumberType, error) {
	if node != nil {
		if named, ok := node.KindNode.(*t.NodeTypeNamed); ok {
			if name, ok := named.NameNode.(*t.NodeNameSingle); ok {
				if description, ok := magmatypes.NumberTypes[name.Name]; ok {
					return description, nil
				}
			}
		}
	}
	return magmatypes.NumberType{}, fmt.Errorf("numeric lowering requires a resolved numeric type")
}

func isBool(node *t.NodeType) bool {
	if node == nil {
		return false
	}
	named, ok := node.KindNode.(*t.NodeTypeNamed)
	if !ok {
		return false
	}
	name, ok := named.NameNode.(*t.NodeNameSingle)
	return ok && name.Name == "bool"
}

func isPointer(node *t.NodeType) bool {
	if node == nil {
		return false
	}
	switch value := node.KindNode.(type) {
	case *t.NodeTypePointer, *t.NodeTypeRfc, *t.NodeTypeFunc:
		return true
	case *t.NodeTypeNamed:
		name, ok := value.NameNode.(*t.NodeNameSingle)
		return ok && name.Name == "ptr"
	default:
		return false
	}
}

// CoerceNumeric mirrors textual numeric promotion. It emits only the LLVM cast
// selected by the checked operand types and deliberately adds no range checks.
func (l *Lowerer) CoerceNumeric(block lb.BlockID, value lb.ValueID, from, to *t.NodeType) (lb.ValueID, error) {
	fromDescription, err := numericDescription(from)
	if err != nil {
		return 0, err
	}
	toDescription, err := numericDescription(to)
	if err != nil {
		return 0, err
	}
	if fromDescription == toDescription {
		return value, nil
	}
	toID, err := l.Lower(to)
	if err != nil {
		return 0, err
	}
	var operation lb.CastOp
	switch {
	case fromDescription.IsFloat && toDescription.IsFloat && fromDescription.ByteSize > toDescription.ByteSize:
		operation = lb.CastFloatTruncate
	case fromDescription.IsFloat && toDescription.IsFloat:
		operation = lb.CastFloatExtend
	case fromDescription.IsFloat && !toDescription.IsFloat && toDescription.IsSigned:
		operation = lb.CastFloatToSignedInt
	case fromDescription.IsFloat && !toDescription.IsFloat:
		operation = lb.CastFloatToUnsignedInt
	case !fromDescription.IsFloat && toDescription.IsFloat && fromDescription.IsSigned:
		operation = lb.CastSignedIntToFloat
	case !fromDescription.IsFloat && toDescription.IsFloat:
		operation = lb.CastUnsignedIntToFloat
	case fromDescription.ByteSize > toDescription.ByteSize:
		operation = lb.CastTruncate
	case fromDescription.ByteSize < toDescription.ByteSize && fromDescription.IsSigned:
		operation = lb.CastSignExtend
	case fromDescription.ByteSize < toDescription.ByteSize:
		operation = lb.CastZeroExtend
	default:
		// LLVM integers do not encode signedness, so an equal-width signedness
		// change has no instruction in either lowering path.
		return value, nil
	}
	return l.backend.Cast(block, operation, value, toID)
}

// NumericBinary lowers the already-promoted arithmetic operation. Division,
// remainder, shifts, and overflow retain LLVM's behavior from textual IR; this
// helper does not synthesize guards.
func (l *Lowerer) NumericBinary(block lb.BlockID, operator t.KwType, left, right lb.ValueID, operandType *t.NodeType) (lb.ValueID, error) {
	description, err := numericDescription(operandType)
	if err != nil {
		return 0, err
	}
	operations := map[t.KwType]lb.BinaryOp{
		t.KwPlus: lb.BinaryAdd, t.KwMinus: lb.BinarySub, t.KwAsterisk: lb.BinaryMul,
		t.KwAmpersand: lb.BinaryAnd, t.KwPipe: lb.BinaryOr, t.KwCaret: lb.BinaryXor,
		t.KwShiftLeft: lb.BinaryShiftLeft,
	}
	operation, ok := operations[operator]
	if !ok {
		switch operator {
		case t.KwSlash:
			if description.IsFloat {
				operation = lb.BinaryFloatDiv
			} else if description.IsSigned {
				operation = lb.BinarySignedDiv
			} else {
				operation = lb.BinaryUnsignedDiv
			}
		case t.KwPercent:
			if description.IsFloat {
				operation = lb.BinaryFloatRem
			} else if description.IsSigned {
				operation = lb.BinarySignedRem
			} else {
				operation = lb.BinaryUnsignedRem
			}
		case t.KwShiftRight:
			if description.IsSigned {
				operation = lb.BinaryArithmeticShiftRight
			} else {
				operation = lb.BinaryLogicalShiftRight
			}
		default:
			return 0, fmt.Errorf("unsupported numeric binary operator %q", t.KwTypeToRepr[operator])
		}
	}
	if description.IsFloat && (operator == t.KwAmpersand || operator == t.KwPipe || operator == t.KwCaret || operator == t.KwShiftLeft || operator == t.KwShiftRight) {
		return 0, fmt.Errorf("operator %q requires integer operands", t.KwTypeToRepr[operator])
	}
	return l.backend.Binary(block, operation, left, right)
}

// Compare lowers the predicates used by textual IR. In particular, float !=
// remains unordered-not-equal, while all other float predicates are ordered.
func (l *Lowerer) Compare(block lb.BlockID, operator t.KwType, left, right lb.ValueID, operandType *t.NodeType) (lb.ValueID, error) {
	if isPointer(operandType) || isBool(operandType) {
		if operator == t.KwCmpEq {
			return l.backend.Compare(block, lb.CompareEqual, left, right)
		}
		if operator == t.KwCmpNeq {
			return l.backend.Compare(block, lb.CompareNotEqual, left, right)
		}
		return 0, fmt.Errorf("type only supports equality comparisons")
	}
	description, err := numericDescription(operandType)
	if err != nil {
		return 0, err
	}
	var operation lb.CompareOp
	if description.IsFloat {
		operation = map[t.KwType]lb.CompareOp{t.KwCmpEq: lb.CompareFloatOrderedEqual, t.KwCmpNeq: lb.CompareFloatUnorderedNotEqual, t.KwCmpGt: lb.CompareFloatOrderedGreater, t.KwCmpGtEq: lb.CompareFloatOrderedGreaterEqual, t.KwCmpLt: lb.CompareFloatOrderedLess, t.KwCmpLtEq: lb.CompareFloatOrderedLessEqual}[operator]
	} else if description.IsSigned {
		operation = map[t.KwType]lb.CompareOp{t.KwCmpEq: lb.CompareEqual, t.KwCmpNeq: lb.CompareNotEqual, t.KwCmpGt: lb.CompareSignedGreater, t.KwCmpGtEq: lb.CompareSignedGreaterEqual, t.KwCmpLt: lb.CompareSignedLess, t.KwCmpLtEq: lb.CompareSignedLessEqual}[operator]
	} else {
		operation = map[t.KwType]lb.CompareOp{t.KwCmpEq: lb.CompareEqual, t.KwCmpNeq: lb.CompareNotEqual, t.KwCmpGt: lb.CompareUnsignedGreater, t.KwCmpGtEq: lb.CompareUnsignedGreaterEqual, t.KwCmpLt: lb.CompareUnsignedLess, t.KwCmpLtEq: lb.CompareUnsignedLessEqual}[operator]
	}
	if operation == 0 {
		return 0, fmt.Errorf("unsupported comparison operator %q", t.KwTypeToRepr[operator])
	}
	return l.backend.Compare(block, operation, left, right)
}

func (l *Lowerer) UnaryNot(block lb.BlockID, value lb.ValueID, operandType *t.NodeType) (lb.ValueID, error) {
	bits := 1
	if !isBool(operandType) {
		description, err := numericDescription(operandType)
		if err != nil || description.IsFloat {
			return 0, fmt.Errorf("bitwise not requires an integer operand")
		}
		bits = description.ByteSize
	}
	typeID, err := l.Lower(operandType)
	if err != nil {
		return 0, err
	}
	ones := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(bits)), big.NewInt(1))
	constant, err := l.backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: typeID, Integer: ones.String()})
	if err != nil {
		return 0, err
	}
	mask, err := l.backend.ConstantValue(constant)
	if err != nil {
		return 0, err
	}
	return l.backend.Binary(block, lb.BinaryXor, value, mask)
}
