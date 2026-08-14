// Package place constructs canonical ownership locations from linked and
// type-checked expressions. It contains no ownership policy.
package place

import (
	"Magma/src/types"
	"fmt"
	"strconv"
	"strings"
)

type ProjectionKind uint8

const (
	Field ProjectionKind = iota
	ConstantIndex
	DynamicIndex
	Dereference
)

// Projection identifies one step away from a root declaration. Token is
// diagnostic metadata and is intentionally ignored by Equal and Overlaps.
type Projection struct {
	Kind       ProjectionKind
	FieldOwner *types.StructDef
	FieldIndex int
	Index      uint64
	// DynamicExpr distinguishes exact construction sites for equality. All
	// dynamic indices still overlap regardless of this identity.
	DynamicExpr types.NodeExpr
	Token       types.Token
}

type Place struct {
	Root        *types.NodeExprVarDef
	Projections []Projection
	Token       types.Token
}

type BuildError struct {
	Token   types.Token
	Message string
}

func (e *BuildError) Error() string { return e.Message }

func (p Place) Equal(other Place) bool {
	if p.Root != other.Root || len(p.Projections) != len(other.Projections) {
		return false
	}
	for i := range p.Projections {
		if !projectionEqual(p.Projections[i], other.Projections[i]) {
			return false
		}
	}
	return true
}

func (p Place) IsPrefixOf(other Place) bool {
	if p.Root != other.Root || len(p.Projections) > len(other.Projections) {
		return false
	}
	for i := range p.Projections {
		if !projectionEqual(p.Projections[i], other.Projections[i]) {
			return false
		}
	}
	return true
}

func (p Place) Overlaps(other Place) bool {
	if p.Root != other.Root {
		// Until provenance analysis can relate pointer values to their origins,
		// dereferences through distinct pointer locals may still name the same
		// storage. Direct places with distinct declaration roots are disjoint.
		return p.hasDereference() || other.hasDereference()
	}
	limit := len(p.Projections)
	if len(other.Projections) < limit {
		limit = len(other.Projections)
	}
	for i := 0; i < limit; i++ {
		left, right := p.Projections[i], other.Projections[i]
		if projectionEqual(left, right) {
			continue
		}
		if left.Kind == Dereference || right.Kind == Dereference {
			return true
		}
		if isIndex(left.Kind) && isIndex(right.Kind) {
			return left.Kind == DynamicIndex || right.Kind == DynamicIndex
		}
		if left.Kind == Field && right.Kind == Field {
			return false
		}
		return true
	}
	return true
}

func (p Place) hasDereference() bool {
	for _, projection := range p.Projections {
		if projection.Kind == Dereference {
			return true
		}
	}
	return false
}

func projectionEqual(left, right Projection) bool {
	if left.Kind != right.Kind {
		return false
	}
	switch left.Kind {
	case Field:
		return left.FieldOwner == right.FieldOwner && left.FieldIndex == right.FieldIndex
	case ConstantIndex:
		return left.Index == right.Index
	case DynamicIndex:
		return left.DynamicExpr == right.DynamicExpr
	case Dereference:
		return true
	default:
		return false
	}
}

func isIndex(kind ProjectionKind) bool { return kind == ConstantIndex || kind == DynamicIndex }

// FromExpr builds a place after linking and type checking. Value expressions
// return a source-located BuildError rather than an internal failure.
func FromExpr(expr types.NodeExpr) (Place, error) { return build(expr) }

func build(expr types.NodeExpr) (Place, error) {
	switch node := expr.(type) {
	case *types.NodeExprName:
		root, ok := node.AssociatedNode.(*types.NodeExprVarDef)
		if !ok || root == nil {
			return Place{}, failure(node.Tk, "place root is not a resolved local or parameter")
		}
		out := Place{Root: root, Token: node.Tk}
		for _, access := range node.MemberAccesses {
			if err := appendField(&out, access, node.Tk); err != nil {
				return Place{}, err
			}
		}
		return out, nil
	case *types.NodeExprMemberAccess:
		out, err := build(node.Target)
		if err != nil {
			return Place{}, err
		}
		if err := appendField(&out, node.Access, node.Tk); err != nil {
			return Place{}, err
		}
		return out, nil
	case *types.NodeExprSubscript:
		out, err := build(node.Target)
		if err != nil {
			return Place{}, err
		}
		projection := Projection{Kind: DynamicIndex, DynamicExpr: node.Expr, Token: node.Tk}
		if index, ok := constantIndex(node.Expr); ok {
			projection.Kind, projection.Index = ConstantIndex, index
		}
		out.Projections = append(out.Projections, projection)
		return out, nil
	case *types.NodeExprUnary:
		if node.Operator != types.KwAsterisk {
			return Place{}, failure(node.Tk, "unary expression does not designate a place")
		}
		out, err := build(node.Operand)
		if err != nil {
			return Place{}, err
		}
		out.Projections = append(out.Projections, Projection{Kind: Dereference, Token: node.Tk})
		return out, nil
	default:
		return Place{}, failure(expressionToken(expr), fmt.Sprintf("%T does not designate a place", expr))
	}
}

func appendField(out *Place, access *types.MemberAccess, token types.Token) error {
	if access == nil || access.OwnerDef == nil || access.Type == nil {
		return failure(token, "field place has incomplete resolved member metadata")
	}
	if access.PtrDeref {
		out.Projections = append(out.Projections, Projection{Kind: Dereference, Token: token})
	}
	out.Projections = append(out.Projections, Projection{Kind: Field, FieldOwner: access.OwnerDef, FieldIndex: access.FieldNb, Token: token})
	return nil
}

func constantIndex(expr types.NodeExpr) (uint64, bool) {
	switch node := expr.(type) {
	case *types.NodeExprLit:
		if node.LitType != types.TokLitNum {
			return 0, false
		}
		repr := strings.ReplaceAll(node.Value, "_", "")
		base := 10
		if len(repr) > 2 && repr[0] == '0' {
			base = 0
		}
		value, err := strconv.ParseUint(repr, base, 64)
		return value, err == nil
	case *types.NodeExprName:
		variable, ok := node.AssociatedNode.(*types.NodeExprVarDef)
		if !ok || !variable.IsConst || variable.Initializer == nil {
			return 0, false
		}
		return constantIndex(variable.Initializer)
	default:
		return 0, false
	}
}

func expressionToken(expr types.NodeExpr) types.Token {
	switch node := expr.(type) {
	case *types.NodeExprName:
		return node.Tk
	case *types.NodeExprMemberAccess:
		return node.Tk
	case *types.NodeExprSubscript:
		return node.Tk
	case *types.NodeExprUnary:
		return node.Tk
	}
	return types.Token{}
}

func failure(token types.Token, message string) error {
	return &BuildError{Token: token, Message: message}
}
