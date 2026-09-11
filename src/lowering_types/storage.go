package loweringtypes

import (
	"fmt"

	lb "Magma/src/lowering_backend"
	t "Magma/src/types"
)

// CreateLocal mirrors textual irVarDef: reserve fixed-size storage in the
// function entry and zero-initialize it at the declaration point. No lifetime
// markers or dynamic checks are added because textual lowering emits neither.
func (l *Lowerer) CreateLocal(function lb.FunctionID, declarationBlock lb.BlockID, semanticType *t.NodeType) (lb.ValueID, error) {
	typeID, err := l.Lower(semanticType)
	if err != nil {
		return 0, err
	}
	storage, err := l.backend.StaticAlloca(function, typeID, 0)
	if err != nil {
		return 0, err
	}
	zero, err := l.backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantZero, Type: typeID})
	if err != nil {
		return 0, err
	}
	value, err := l.backend.ConstantValue(zero)
	if err != nil {
		return 0, err
	}
	if _, err := l.backend.Store(declarationBlock, value, storage, 0, false); err != nil {
		return 0, fmt.Errorf("initialize local: %w", err)
	}
	return storage, nil
}

// MaterializeArgument mirrors textual function prologues, which copy each
// ordinary parameter into an entry-block stack slot before body lowering.
func (l *Lowerer) MaterializeArgument(function lb.FunctionID, entry lb.BlockID, parameter int, semanticType *t.NodeType) (lb.ValueID, error) {
	typeID, err := l.Lower(semanticType)
	if err != nil {
		return 0, err
	}
	value, err := l.backend.Parameter(function, parameter)
	if err != nil {
		return 0, err
	}
	storage, err := l.backend.StaticAlloca(function, typeID, 0)
	if err != nil {
		return 0, err
	}
	if _, err := l.backend.Store(entry, value, storage, 0, false); err != nil {
		return 0, fmt.Errorf("materialize argument %d: %w", parameter, err)
	}
	return storage, nil
}
