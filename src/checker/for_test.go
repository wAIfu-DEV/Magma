package checker_test

import (
	"strings"
	"testing"
)

func TestForLoopRejectsNonIntegerAndMismatchedBounds(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "floating index",
			body: "for i f64 = 0.0 to 2.0:\n    ..",
			want: "index must have an integer type",
		},
		{
			name: "floating bound",
			body: "for i u64 = 0 to 2.0:\n    ..",
			want: "bound must have an integer type",
		},
		{
			name: "mismatched bound",
			body: "max u32 = 2\n    for i u64 = 0 to max:\n    ..",
			want: "bound must have type 'u64'",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := "mod main\n\nmain() void:\n    " + test.body + "\n..\n"
			stage, err := compileMalformed(t, source)
			if err == nil {
				t.Fatal("invalid for loop was accepted")
			}
			if stage != "type" {
				t.Fatalf("stage = %q, want type: %v", stage, err)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("diagnostic = %q, want substring %q", err, test.want)
			}
		})
	}
}

func TestForLoopIndexDoesNotEscapeItsScope(t *testing.T) {
	stage, err := compileMalformed(t, `mod main

main() void:
    for i := 0 to 1:
    ..
    i = 2
..
`)
	if err == nil {
		t.Fatal("for-loop index escaped its scope")
	}
	if stage != "link" || !strings.Contains(err.Error(), "unknown variable 'i'") {
		t.Fatalf("stage = %q, diagnostic = %v", stage, err)
	}
}
