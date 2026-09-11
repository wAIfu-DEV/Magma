package loweringast

import (
	lb "Magma/src/lowering_backend"
	mt "Magma/src/types"
	"testing"
)

func TestErrorPredicateBranchWeights(t *testing.T) {
	for _, test := range []struct {
		predicate mt.ErrorPredicateKind
		want      lb.BranchWeights
	}{
		{predicate: mt.ErrorPredicateNok, want: lb.UnlikelyThen},
		{predicate: mt.ErrorPredicateOk, want: lb.UnlikelyElse},
	} {
		call := &mt.NodeExprCall{AssociatedFnDef: &mt.NodeFuncDef{ErrorPredicate: test.predicate}}
		got, ok := errorPredicateBranchWeights(call)
		if !ok || got != test.want {
			t.Fatalf("predicate %d: got (%+v, %v), want (%+v, true)", test.predicate, got, ok, test.want)
		}
	}
	if _, ok := errorPredicateBranchWeights(&mt.NodeExprCall{}); ok {
		t.Fatal("ordinary call unexpectedly received branch weights")
	}
}
