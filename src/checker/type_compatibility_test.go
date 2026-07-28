package checker

import (
	types "Magma/src/types"
	"testing"
)

func TestTypeCompatibilityCharacterization(t *testing.T) {
	i32 := makeNamedType("i32")
	i64 := makeNamedType("i64")
	f64 := makeNamedType("f64")
	str := makeNamedType("str")

	fn := func(arg, ret *types.NodeType) *types.NodeType {
		return &types.NodeType{KindNode: &types.NodeTypeFunc{
			Args:    []*types.NodeType{arg},
			RetType: ret,
		}}
	}

	tests := []struct {
		name     string
		expected *types.NodeType
		actual   *types.NodeType
		want     bool
	}{
		{name: "identical named types", expected: i32, actual: makeNamedType("i32"), want: true},
		{name: "different numeric types", expected: i32, actual: f64, want: true},
		{name: "unrelated named types", expected: i32, actual: str, want: false},
		{name: "matching function types", expected: fn(i32, i64), actual: fn(makeNamedType("i32"), makeNamedType("i64")), want: true},
		{name: "different function arity", expected: fn(i32, i64), actual: &types.NodeType{KindNode: &types.NodeTypeFunc{RetType: i64}}, want: false},
		{name: "throwing mismatch", expected: &types.NodeType{Throws: true, KindNode: i32.KindNode}, actual: i32, want: false},
		{name: "nil actual", expected: i32, actual: nil, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := compatibleTypes(test.expected, test.actual); got != test.want {
				t.Fatalf("compatibleTypes() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestConstArrayIndexCharacterization(t *testing.T) {
	tests := []struct {
		value string
		want  uint64
		ok    bool
	}{
		{value: "12", want: 12, ok: true},
		{value: "1_024", want: 1024, ok: true},
		{value: "0x10", want: 16, ok: true},
		{value: "0b101", want: 5, ok: true},
		{value: "-1", ok: false},
		{value: "nope", ok: false},
	}

	for _, test := range tests {
		expr := &types.NodeExprLit{LitType: types.TokLitNum, Value: test.value}
		got, ok := constArrayIndex(expr)
		if ok != test.ok || got != test.want {
			t.Errorf("constArrayIndex(%q) = (%d, %v), want (%d, %v)", test.value, got, ok, test.want, test.ok)
		}
	}
}
