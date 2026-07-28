package types

import "testing"

func TestDisplayTypeHidesInternalNamesAndPreservesGenerics(t *testing.T) {
	tests := []struct {
		name     string
		typeNode *NodeType
		want     string
	}{
		{
			name:     "qualified pointer",
			typeNode: &NodeType{KindNode: &NodeTypePointer{Kind: &NodeTypeAbsolute{AbsoluteName: "html_fFpyoMhsyZ.Scanner"}}},
			want:     "html.Scanner*",
		},
		{
			name: "specialized generic",
			typeNode: &NodeType{KindNode: &NodeTypeAbsolute{
				AbsoluteName: "collections_a1b2c3d4e5.Box__g__N_u64",
				DisplayName:  "collections.Box[u64]",
			}},
			want: "collections.Box[u64]",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := DisplayType(test.typeNode); got != test.want {
				t.Fatalf("DisplayType() = %q, want %q", got, test.want)
			}
		})
	}
}
