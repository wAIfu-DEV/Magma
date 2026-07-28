package llvmir

import (
	mt "Magma/src/types"
	"testing"
)

func TestValidateIrTypeKindRejectsIncompleteShapes(t *testing.T) {
	cases := []mt.NodeTypeKind{
		nil,
		&mt.NodeTypePointer{},
		&mt.NodeTypeSlice{},
		&mt.NodeTypeFunc{},
		&mt.NodeTypeNamed{NameNode: &mt.NodeNameSingle{Name: "unresolved"}},
		&mt.NodeTypeCompilerKnown{Name: "T"},
		&mt.NodeTypeAbsolute{},
	}
	for _, kind := range cases {
		if err := validateIrTypeKind(kind); err == nil {
			t.Fatalf("expected %T to be rejected", kind)
		}
	}
}

func TestValidateIrTypeKindAcceptsCompleteShapes(t *testing.T) {
	i64 := &mt.NodeType{KindNode: &mt.NodeTypeNamed{NameNode: &mt.NodeNameSingle{Name: "i64"}}}
	kind := &mt.NodeTypePointer{Kind: &mt.NodeTypeFunc{Args: []*mt.NodeType{i64}, RetType: i64}}
	if err := validateIrTypeKind(kind); err != nil {
		t.Fatalf("valid type rejected: %v", err)
	}
}
