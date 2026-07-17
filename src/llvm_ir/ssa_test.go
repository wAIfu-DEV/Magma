package llvmir

import "testing"

func TestGlobalSsaNamesSeparateModuleAndCounter(t *testing.T) {
	leftNext := 211
	rightNext := 11
	left := irSsaGlobal(&IrCtx{moduleIdx: 3, nextSsa: &leftNext})
	right := irSsaGlobal(&IrCtx{moduleIdx: 32, nextSsa: &rightNext})

	if left.Repr == right.Repr {
		t.Fatalf("ambiguous global names: both modules produced %q", left.Repr)
	}
	if left.Repr != "@.3.211" || right.Repr != "@.32.11" {
		t.Fatalf("unexpected global names: %q and %q", left.Repr, right.Repr)
	}
}
