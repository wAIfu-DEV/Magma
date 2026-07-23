package llvmir

import (
	"bytes"
	"strings"
	"testing"

	mt "Magma/src/types"
)

func TestTraceStringPoolUsesCompactDeterministicNames(t *testing.T) {
	first := newTraceStringPool([]string{"veryLongFunctionName", "file.mg", "another"})
	second := newTraceStringPool([]string{"another", "veryLongFunctionName", "file.mg"})

	firstName := first.intern("veryLongFunctionName").Repr
	secondName := second.intern("veryLongFunctionName").Repr
	if firstName != secondName {
		t.Fatalf("trace name depends on collection order: %q != %q", firstName, secondName)
	}
	if !strings.HasPrefix(firstName, "@.mt") || len(firstName) > 8 {
		t.Fatalf("trace name is not compact: %q", firstName)
	}
	if first.intern("file.mg").Repr == firstName {
		t.Fatal("distinct trace strings received the same name")
	}

	var output bytes.Buffer
	first.writeTo(&output)
	if strings.Contains(output.String(), "766572794c6f6e6746756e6374696f6e4e616d65") {
		t.Fatal("trace constant name still embeds hex-encoded contents")
	}
}

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
