package llvmir

import (
	"testing"

	mt "Magma/src/types"
)

func TestTraceDisplayNameHidesGenericMangling(t *testing.T) {
	tests := []struct {
		name mt.NodeName
		want string
	}{
		{&mt.NodeNameSingle{Name: "fakeAlloc"}, "fakeAlloc"},
		{&mt.NodeNameSingle{Name: "new__g__N_str__A_reader_hash__ReaderReadTask"}, "new"},
		{&mt.NodeNameComposite{Parts: []string{"Allocator", "allocT__g__A_future_hash__Work"}}, "Allocator.allocT"},
		{&mt.NodeNameComposite{Parts: []string{"Owner__g__N_str", "read__g__N_u8"}}, "Owner.read"},
	}

	for _, test := range tests {
		if got := traceDisplayName(test.name); got != test.want {
			t.Errorf("traceDisplayName(%q) = %q, want %q", flattenName(test.name), got, test.want)
		}
	}
}
