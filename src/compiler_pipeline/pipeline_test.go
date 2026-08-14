package compilerpipeline

import (
	"Magma/src/comp_err"
	"Magma/src/shared"
	"Magma/src/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validateTestProgram(t *testing.T, source string) ValidatedProgram {
	t.Helper()
	parsed, _ := testProgram(t, source)
	specialized, err := Specialize(*parsed)
	if err != nil {
		t.Fatal(err)
	}
	linked, err := Link(specialized)
	if err != nil {
		t.Fatal(err)
	}
	typed, err := CheckTypes(linked)
	if err != nil {
		t.Fatal(err)
	}
	validated, err := ValidateLowering(typed)
	if err != nil {
		t.Fatal(err)
	}
	return validated
}

const ownershipProgramPrefix = `mod main
Resource(value u64)
destr Resource.close() void:
..
makeResource() $Resource:
    ret Resource(value=1)
..
consume(value $Resource) void:
    value.close()
..
`

func TestExplicitMovePassesFatalOwnershipStageAndLowers(t *testing.T) {
	validated := validateTestProgram(t, ownershipProgramPrefix+`main() void:
    value $Resource = makeResource()
    consume(move value)
..
`)
	ready, err := CheckSafety(validated, false)
	if err != nil {
		t.Fatalf("explicit move failed: %v", err)
	}
	if _, err := Lower(ready); err != nil {
		t.Fatalf("lower explicit move: %v", err)
	}
}

func TestInferredCallResultResolvesGenericMember(t *testing.T) {
	validateTestProgram(t, `mod main
use "std:heap" heap
main() !void:
    a := heap.allocator()
    block := try a.allocT[u64](1)
    a.free(block)
..
`)
}

func TestNamedOwnershipTransferRequiresMove(t *testing.T) {
	validated := validateTestProgram(t, ownershipProgramPrefix+`main() void:
    value $Resource = makeResource()
    consume(value)
..
`)
	_, err := CheckSafety(validated, false)
	if err == nil || !strings.Contains(err.Error(), "requires 'move'") {
		t.Fatalf("error = %v, want explicit-move diagnostic", err)
	}
	if diagnostics := comp_err.Diagnostics(err); len(diagnostics) != 1 || diagnostics[0].Severity != types.SeverityError || diagnostics[0].Code != "missing-move" {
		t.Fatalf("fatal diagnostics = %#v", diagnostics)
	}
}

func TestSafetyWarningsDowngradeButPreserveMoveSemantics(t *testing.T) {
	validated := validateTestProgram(t, ownershipProgramPrefix+`main() void:
    value $Resource = makeResource()
    consume(value)
    value.close()
..
`)
	if _, err := CheckSafety(validated, true); err != nil {
		t.Fatalf("warning mode returned fatal error: %v", err)
	}
	warnings := validated.State().Warnings
	if len(warnings) < 2 {
		t.Fatalf("warnings = %#v, want missing-move and use-after-transfer", warnings)
	}
	for _, warning := range warnings {
		if warning.Severity != types.SeverityWarning {
			t.Fatalf("warning mode retained fatal diagnostic: %#v", warning)
		}
	}
	if warnings[0].Code == "" {
		t.Fatalf("warning-mode safety diagnostic has no stable code: %#v", warnings[0])
	}
}

func TestCleanupLeakRemainsWarningInFatalMode(t *testing.T) {
	validated := validateTestProgram(t, ownershipProgramPrefix+`main() void:
    value $Resource = makeResource()
..
`)
	if _, err := CheckSafety(validated, false); err != nil {
		t.Fatalf("cleanup leak became fatal: %v", err)
	}
	if warnings := validated.State().Warnings; len(warnings) != 1 || !strings.Contains(warnings[0].Message, "not consumed") {
		t.Fatalf("cleanup warnings = %#v", warnings)
	}
}

func TestUnprovenOrdinarySubscriptIsFatal(t *testing.T) {
	validated := validateTestProgram(t, `mod main
read(values u64[], i u64) u64:
    ret values[i]
..
`)
	_, err := CheckSafety(validated, false)
	if err == nil || !strings.Contains(err.Error(), "not proven in range") {
		t.Fatalf("error = %v, want range-proof diagnostic", err)
	}
}

func TestUnsafeBlockIsRequiredForUnknownPointerDereference(t *testing.T) {
	validated := validateTestProgram(t, `mod main
read(address ptr) u64:
    value u64* = address
    ret *value
..
`)
	_, err := CheckSafety(validated, false)
	if err == nil || !strings.Contains(err.Error(), "requires an unsafe block") {
		t.Fatalf("error = %v, want unsafe dereference diagnostic", err)
	}
}

func TestUnsafeBlockPermitsUnknownDereferenceAndInlineLLVM(t *testing.T) {
	validated := validateTestProgram(t, `mod main
read(address ptr) u64:
    unsafe:
        value u64* = address
        ret *value
    ..
..
raw(value u64) u64:
    unsafe:
        llvm "  ret i64 %value\n"
    ..
..
`)
	ready, err := CheckSafety(validated, false)
	if err != nil {
		t.Fatalf("unsafe operation was rejected: %v", err)
	}
	if _, err := Lower(ready); err != nil {
		t.Fatalf("lower unsafe block: %v", err)
	}
}

func TestUnsafeBlockDoesNotSuppressKnownOwnershipErrors(t *testing.T) {
	validated := validateTestProgram(t, ownershipProgramPrefix+`main() void:
    value $Resource = makeResource()
    unsafe:
        value.close()
        value.close()
    ..
..
`)
	_, err := CheckSafety(validated, false)
	if err == nil || !strings.Contains(err.Error(), "more than once") {
		t.Fatalf("error = %v, want ownership error inside unsafe", err)
	}
}

func TestInlineLLVMOutsideUnsafeIsRejected(t *testing.T) {
	validated := validateTestProgram(t, `mod main
raw(value u64) u64:
    llvm "  ret i64 %value\n"
..
`)
	_, err := CheckSafety(validated, false)
	if err == nil || !strings.Contains(err.Error(), "inline LLVM requires an unsafe block") {
		t.Fatalf("error = %v, want inline-LLVM unsafe diagnostic", err)
	}
}

func TestPointerSubscriptRequiresUnsafe(t *testing.T) {
	validated := validateTestProgram(t, `mod main
read(values u64*, i u64) u64:
    ret values[i]
..
`)
	_, err := CheckSafety(validated, false)
	if err == nil || !strings.Contains(err.Error(), "pointer subscript") {
		t.Fatalf("error = %v, want pointer-subscript unsafe diagnostic", err)
	}
}

func TestExternalPointerArgumentsDefaultToCallOnlyBorrow(t *testing.T) {
	validated := validateTestProgram(t, `mod main
ext nativeWrite native_write(value u64*) void
main() void:
    value u64 = 1
    nativeWrite(addrof value)
    value = 2
..
`)
	ready, err := CheckSafety(validated, false)
	if err != nil {
		t.Fatalf("call-only external pointer argument failed: %v", err)
	}
	if _, err := Lower(ready); err != nil {
		t.Fatalf("lower call-only external: %v", err)
	}
}

func TestExternalOwnedParameterDoesNotImplyConsumption(t *testing.T) {
	validated := validateTestProgram(t, ownershipProgramPrefix+`ext inspectNative inspect_native(value $Resource) void
main() void:
    value $Resource = makeResource()
    inspectNative(value)
    value.close()
..
`)
	if _, err := CheckSafety(validated, false); err != nil {
		t.Fatalf("external annotation exceeded non-consuming default: %v", err)
	}
}

func TestExternalPointerResultHasOpaqueProvenance(t *testing.T) {
	validated := validateTestProgram(t, `mod main
ext nativePointer native_pointer() u64*
read() u64:
    value u64* = nativePointer()
    ret *value
..
`)
	_, err := CheckSafety(validated, false)
	if err == nil || !strings.Contains(err.Error(), "unknown pointer provenance") {
		t.Fatalf("error = %v, want opaque external-result diagnostic", err)
	}
}

func TestExternalOwnedPointerBackedResultRequiresWrapper(t *testing.T) {
	validated := validateTestProgram(t, `mod main
Handle(value u64)
destr Handle.join() void:
..
ext nativeStart native_start(context u64*) $Handle
start(context u64*) $Handle:
    unsafe:
        handle $Handle = nativeStart(context)
        ret move handle
    ..
..
main() void:
    context u64 = 1
    handle $Handle = start(addrof context)
    handle.join()
..
`)
	ready, err := CheckSafety(validated, false)
	if err != nil {
		t.Fatalf("audited native wrapper failed: %v", err)
	}
	if _, err := Lower(ready); err != nil {
		t.Fatalf("lower native wrapper: %v", err)
	}

	direct := validateTestProgram(t, `mod main
Handle(value u64)
destr Handle.join() void:
..
ext nativeStart native_start(context u64*) $Handle
main() void:
    context u64 = 1
    handle $Handle = nativeStart(addrof context)
    handle.join()
..
`)
	_, err = CheckSafety(direct, false)
	if err == nil || !strings.Contains(err.Error(), "requires an audited unsafe wrapper") {
		t.Fatalf("error = %v, want exceptional-FFI wrapper diagnostic", err)
	}
}

func TestBoundedSubscriptsShareOneEntryGuard(t *testing.T) {
	validated := validateTestProgram(t, `mod main
read(values u64[], i u64) u64:
    bounded i < values.count():
        first := values[i]
        ret first + values[i]
    ..
    ret 0
..
`)
	ready, err := CheckSafety(validated, false)
	if err != nil {
		t.Fatalf("bounded range proof failed: %v", err)
	}
	ir, err := Lower(ready)
	if err != nil {
		t.Fatalf("lower bounded block: %v", err)
	}
	text := string(ir)
	start := strings.Index(text, ".read(")
	end := -1
	if start >= 0 {
		end = strings.Index(text[start:], "\n}\n")
	}
	if start < 0 || end < 0 {
		t.Fatalf("read function not found in IR")
	}
	functionIR := text[start : start+end]
	// The body has one cleanup conditional; the other is the single bounded
	// entry guard, independent of the number of authorized accesses.
	if got := strings.Count(functionIR, "br i1"); got != 2 {
		t.Fatalf("bounded function emitted %d conditional branches, want one entry guard plus cleanup\n%s", got, functionIR)
	}
}

func TestForLoopAndEarlyExitEstablishRangeFacts(t *testing.T) {
	for _, source := range []string{
		`mod main
clear(values u64[]) void:
    for i u64 = 0 to values.count():
        values[i] = 0
    ..
..
`,
		`mod main
read(values u64[], i u64) u64:
    if i >= values.count():
        ret 0
    ..
    ret values[i]
..
`,
		`mod main
sum(values u64[]) u64:
    i u64 = 0
    total u64 = 0
    loop i < values.count():
        total = total + values[i]
        i = i + 1
    ..
    ret total
..
`,
	} {
		validated := validateTestProgram(t, source)
		if _, err := CheckSafety(validated, false); err != nil {
			t.Fatalf("implicit range proof failed: %v", err)
		}
	}
}

func TestRangeFactsInvalidateOnlyParticipatingVariables(t *testing.T) {
	validated := validateTestProgram(t, `mod main
read(values u64[], i u64) u64:
    unrelated u64 = 0
    bounded i < values.count():
        unrelated = 1
        ret values[i]
    ..
    ret unrelated
..
`)
	if _, err := CheckSafety(validated, false); err != nil {
		t.Fatalf("unrelated assignment invalidated proof: %v", err)
	}

	validated = validateTestProgram(t, `mod main
read(values u64[], i u64) u64:
    bounded i < values.count():
        i = i + 1
        ret values[i]
    ..
    ret 0
..
`)
	if _, err := CheckSafety(validated, false); err == nil || !strings.Contains(err.Error(), "not proven in range") {
		t.Fatalf("participating assignment did not invalidate proof: %v", err)
	}
}

func TestPointerProvenanceRejectsDirectAndHelperLocalEscape(t *testing.T) {
	for _, source := range []string{
		`mod main
dangling() u64*:
    value u64 = 10
    ret addrof value
..
`,
		`mod main
identity(value u64*) u64*:
    ret value
..
dangling() u64*:
    value u64 = 10
    ret identity(addrof value)
..
`,
	} {
		validated := validateTestProgram(t, source)
		if _, err := CheckSafety(validated, false); err == nil || !strings.Contains(err.Error(), "cannot escape its source frame") {
			t.Fatalf("local pointer escape error = %v", err)
		}
	}
}

func TestPointerLoanEndsAtLastReachableUseAndIsFieldSensitive(t *testing.T) {
	validated := validateTestProgram(t, `mod main
State(counter u64, status u64)
inspect(value u64*) void:
    ignored u64 = *value
..
main() void:
    state State = State(counter=1, status=0)
    pointer u64* = addrof state.counter
    state.status = 1
    inspect(pointer)
    state.counter = 2
..
`)
	if _, err := CheckSafety(validated, false); err != nil {
		t.Fatalf("last-use or disjoint-field loan rejected: %v", err)
	}

	validated = validateTestProgram(t, `mod main
main() u64:
    value u64 = 1
    pointer u64* = addrof value
    value = 2
    ret *pointer
..
`)
	if _, err := CheckSafety(validated, false); err == nil || !strings.Contains(err.Error(), "while a pointer to it remains live") {
		t.Fatalf("live pointer loan did not restrict mutation: %v", err)
	}
}

func TestMutationThroughOriginatingPointerDoesNotConflictWithItsLoan(t *testing.T) {
	validated := validateTestProgram(t, `mod main
State(counter u64, status u64)
main() u64:
    state State = State(counter=1, status=0)
    pointer State* = addrof state
    pointer.counter = 2
    ret pointer.counter
..
`)
	if _, err := CheckSafety(validated, false); err != nil {
		t.Fatalf("mutation through originating pointer rejected: %v", err)
	}
}

func TestConstantStaticArrayIndexCarriesIntrinsicRangeProof(t *testing.T) {
	validated := validateTestProgram(t, `mod main
main() u64:
    values := array u64[2]
    values[0] = 7
    values[1] = 9
    ret values[0] + values[1]
..
`)
	if _, err := CheckSafety(validated, false); err != nil {
		t.Fatalf("constant in-range static array index rejected: %v", err)
	}
}

func TestCanonicalLoopOverSavedSliceCountCarriesRangeProof(t *testing.T) {
	validated := validateTestProgram(t, `mod main
use "std:slices" slices
sum(values u64[]) u64:
    count := slices.count(values)
    total u64 = 0
    for i u64 = 0 to count:
        total = total + values[i]
    ..
    ret total
..
`)
	if _, err := CheckSafety(validated, false); err != nil {
		t.Fatalf("canonical saved-count loop rejected: %v", err)
	}
}

func TestStaticArrayExtentMatchesNamedConstantLoopBound(t *testing.T) {
	validated := validateTestProgram(t, `mod main
const count u64 = 4
main() u64:
    values := array u64[4]
    i u64 = 0
    loop i < count:
        values[i] = i
        i = i + 1
    ..
    ret values[3]
..
`)
	if _, err := CheckSafety(validated, false); err != nil {
		t.Fatalf("named constant matching a static extent rejected: %v", err)
	}
}

func TestBorrowedDestructibleStaticArrayElementsAreNotOwnershipStorage(t *testing.T) {
	validated := validateTestProgram(t, `mod main
main() str:
    values := array str[2]
    values[0] = "first"
    values[1] = "second"
    ret values[0]
..
`)
	if _, err := CheckSafety(validated, false); err != nil {
		t.Fatalf("borrowed string array initialization rejected: %v", err)
	}
}

func TestSafeContainerViewRetainsReceiverRangeIdentity(t *testing.T) {
	validated := validateTestProgram(t, `mod main
use "std:allocator" allocator
use "std:array" array
Item(value u64)
readLast(a allocator.Allocator, items array.Array[Item]) !u64:
    count := items.count()
    if count == 0:
        ret 0
    ..
    ret items.view()[count - 1].value
..
`)
	if _, err := CheckSafety(validated, false); err != nil {
		t.Fatalf("safe container view lost receiver range identity: %v", err)
	}
}

func TestShortCircuitExactCountGuardsRightHandSubscript(t *testing.T) {
	validated := validateTestProgram(t, `mod main
use "std:slices" slices
valid(values u8[]) bool:
    ret slices.count(values) == 3 && values[2] == 7
..
invalid(values u8[]) bool:
    ret slices.count(values) != 3 || values[2] != 7
..
`)
	if _, err := CheckSafety(validated, false); err != nil {
		t.Fatalf("short-circuit count guard did not refine right operand: %v", err)
	}
}

func TestUnsafeSubscriptAuthorizationSurvivesLowering(t *testing.T) {
	validated := validateTestProgram(t, `mod main
readUnchecked(values u8[], index u64) u8:
    unsafe:
        ret values[index]
    ..
..
`)
	ready, err := CheckSafety(validated, false)
	if err != nil {
		t.Fatalf("unsafe subscript was not authorized: %v", err)
	}
	if _, err := Lower(ready); err != nil {
		t.Fatalf("unsafe subscript authorization was lost before lowering: %v", err)
	}
}

func TestThrowErrorRetainsOkContinuationForSafetyAnalysis(t *testing.T) {
	validated := validateTestProgram(t, `mod main
use "std:errors" errors
writeAfterPropagation(value u64*, failure error) !void:
    throw failure
    *value = 7
..
`)
	ready, err := CheckSafety(validated, false)
	if err != nil {
		t.Fatalf("conditional error throw lost its OK continuation: %v", err)
	}
	if _, err := Lower(ready); err != nil {
		t.Fatalf("post-throw continuation was not safety-validated: %v", err)
	}
}

func TestVisibleIdentityHelperPreservesPointerProvenance(t *testing.T) {
	validated := validateTestProgram(t, `mod main
identity(value u64*) u64*:
    ret value
..
main() u64:
    value u64 = 42
    pointer := identity(addrof value)
    observed u64 = *pointer
    value = 7
    ret observed + value
..
`)
	ready, err := CheckSafety(validated, false)
	if err != nil {
		t.Fatalf("visible pointer identity lost provenance: %v", err)
	}
	if _, err := Lower(ready); err != nil {
		t.Fatalf("lower provenance-checked identity helper: %v", err)
	}
}

func TestStackBackedSliceCannotEscapeFrame(t *testing.T) {
	validated := validateTestProgram(t, `mod main
dangling() u8[]:
    values := array u8[4]
    ret values
..
`)
	if _, err := CheckSafety(validated, false); err == nil || !strings.Contains(err.Error(), "stack-backed slice") {
		t.Fatalf("stack slice escape error = %v", err)
	}
}

func TestCheckedSubslicePreservesSourceProvenance(t *testing.T) {
	validated := validateTestProgram(t, `mod main
use "std:slices" slices
dangling() !u8[]:
    values := array u8[4]
    ret try slices.subslice[u8](values, 0, 2)
..
`)
	if _, err := CheckSafety(validated, false); err == nil || !strings.Contains(err.Error(), "stack-backed slice") {
		t.Fatalf("subslice provenance escape error = %v", err)
	}

	validated = validateTestProgram(t, `mod main
use "std:slices" slices
prefix(values u8[]) !u8[]:
    ret try slices.subslice[u8](values, 0, values.count())
..
`)
	ready, err := CheckSafety(validated, false)
	if err != nil {
		t.Fatalf("checked subslice rejected: %v", err)
	}
	if _, err := Lower(ready); err != nil {
		t.Fatalf("lower checked subslice: %v", err)
	}
}

const completionRetentionProgram = `mod main
Handle(value u64)
destr Handle.join() void:
..
start(context u64*) $Handle:
    ret Handle(value=0)
..
`

func TestCompletionBearingHandleAllowsJoinedLocalContext(t *testing.T) {
	validated := validateTestProgram(t, completionRetentionProgram+`main() void:
    context u64 = 0
    handle $Handle = start(addrof context)
    handle.join()
    context = 42
..
`)
	ready, err := CheckSafety(validated, false)
	if err != nil {
		t.Fatalf("joined local context failed: %v", err)
	}
	if _, err := Lower(ready); err != nil {
		t.Fatalf("lower joined local context: %v", err)
	}
}

func TestCompletionBearingHandleMustFinishBeforeContextScopeExit(t *testing.T) {
	validated := validateTestProgram(t, completionRetentionProgram+`main() void:
    context u64 = 0
    handle $Handle = start(addrof context)
..
`)
	if _, err := CheckSafety(validated, false); err == nil || !strings.Contains(err.Error(), "completion-bearing handle must be consumed") {
		t.Fatalf("unfinished completion error = %v", err)
	}
}

func TestCompletionBearingHandleCannotEscapeRetainedLocal(t *testing.T) {
	validated := validateTestProgram(t, completionRetentionProgram+`bad() $Handle:
    context u64 = 0
    ret start(addrof context)
..
`)
	if _, err := CheckSafety(validated, false); err == nil || !strings.Contains(err.Error(), "completion-bearing handle retaining local place") {
		t.Fatalf("escaping completion handle error = %v", err)
	}
}

func TestCompletionRetentionFollowsHandleMove(t *testing.T) {
	validated := validateTestProgram(t, completionRetentionProgram+`main() void:
    context u64 = 0
    first $Handle = start(addrof context)
    second $Handle = move first
    second.join()
..
`)
	if _, err := CheckSafety(validated, false); err != nil {
		t.Fatalf("moved completion handle failed: %v", err)
	}
}

func TestCompletionRetentionFollowsConsumingMemberResult(t *testing.T) {
	validated := validateTestProgram(t, `mod main
Handle(value u64)
destr Handle.transfer() $Handle:
    ret Handle(value=this.value)
..
destr Handle.join() void:
..
start(context u64*) $Handle:
    ret Handle(value=0)
..
main() void:
    context u64 = 0
    first $Handle = start(addrof context)
    second := first.transfer()
    second.join()
..
`)
	if _, err := CheckSafety(validated, false); err != nil {
		t.Fatalf("consuming member result lost completion retention: %v", err)
	}
}

func TestNoRetainOwnedResultDoesNotCapturePointerArgument(t *testing.T) {
	validated := validateTestProgram(t, `mod main
Handle(value u64)
destr Handle.close() void:
..
@no_retain
build(source u64*) $Handle:
    ret Handle(value=*source)
..
make() $Handle:
    source u64 = 7
    ret build(addrof source)
..
`)
	ready, err := CheckSafety(validated, false)
	if err != nil {
		t.Fatalf("no_retain result incorrectly retained pointer argument: %v", err)
	}
	if _, err := Lower(ready); err != nil {
		t.Fatalf("lower no_retain program: %v", err)
	}
}

func TestCompletionBearingHandleCannotBeDiscarded(t *testing.T) {
	validated := validateTestProgram(t, completionRetentionProgram+`main() void:
    context u64 = 0
    start(addrof context)
..
`)
	if _, err := CheckSafety(validated, false); err == nil || !strings.Contains(err.Error(), "completion-bearing handle retaining source storage cannot be discarded") {
		t.Fatalf("discarded completion handle error = %v", err)
	}
}

func TestCompletionBearingHandleCannotBeOverwrittenBeforeCompletion(t *testing.T) {
	validated := validateTestProgram(t, completionRetentionProgram+`main() void:
    firstContext u64 = 0
    secondContext u64 = 0
    handle $Handle = start(addrof firstContext)
    handle = start(addrof secondContext)
    handle.join()
..
`)
	if _, err := CheckSafety(validated, false); err == nil || !strings.Contains(err.Error(), "assignment discards completion-bearing handle") {
		t.Fatalf("overwritten completion handle error = %v", err)
	}
}

func TestSignedAndTransitiveRangeProofs(t *testing.T) {
	validated := validateTestProgram(t, `mod main
read(values u64[], i i64) u64:
    bounded 0 <= i, i < values.count():
        ret values[i]
    ..
    ret 0
..
clear(values u64[], limit u64) void:
    bounded limit <= values.count():
        for i u64 = 0 to limit:
            values[i] = 0
        ..
    ..
..
`)
	if _, err := CheckSafety(validated, false); err != nil {
		t.Fatalf("signed/transitive range proof failed: %v", err)
	}

	validated = validateTestProgram(t, `mod main
read(values u64[], i i64) u64:
    bounded i < values.count():
        ret values[i]
    ..
    ret 0
..
`)
	if _, err := CheckSafety(validated, false); err == nil || !strings.Contains(err.Error(), "non-negative") {
		t.Fatalf("signed index without lower bound was accepted: %v", err)
	}
}

func TestMoveAfterDeferredDestructorIsFatal(t *testing.T) {
	validated := validateTestProgram(t, ownershipProgramPrefix+`main() void:
    value $Resource = makeResource()
    defer value.close()
    consume(move value)
..
`)
	_, err := CheckSafety(validated, false)
	if err == nil || !strings.Contains(err.Error(), "deferred destructor is pending") || !strings.Contains(err.Error(), "defer scheduled at line") {
		t.Fatalf("error = %v, want deferred-transfer diagnostic with origin", err)
	}
}

const aggregateOwnershipProgram = ownershipProgramPrefix + `Container(left Resource, right Resource, count u64)
destr Container.close() void:
..
`

func TestFieldMoveReinitializationAndCompleteExtractionLower(t *testing.T) {
	validated := validateTestProgram(t, aggregateOwnershipProgram+`main() void:
    value $Container = Container(left=makeResource(), right=makeResource(), count=2)
    consume(move value.left)
    value.left = makeResource()
    consume(move value.left)
    consume(move value.right)
..
`)
	ready, err := CheckSafety(validated, false)
	if err != nil {
		t.Fatalf("field ownership flow failed: %v", err)
	}
	if _, err := Lower(ready); err != nil {
		t.Fatalf("lower field ownership flow: %v", err)
	}
}

func TestDestructorAfterFieldMoveIsFatal(t *testing.T) {
	validated := validateTestProgram(t, aggregateOwnershipProgram+`main() void:
    value $Container = Container(left=makeResource(), right=makeResource(), count=2)
    consume(move value.left)
    value.close()
..
`)
	_, err := CheckSafety(validated, false)
	if err == nil || !strings.Contains(err.Error(), "after it was moved") {
		t.Fatalf("error = %v, want partially-moved destructor rejection", err)
	}
}

func TestOwnershipWarningsJoinSharedDiagnostics(t *testing.T) {
	parsed, _ := testProgram(t, "mod main\nleak(value $str) void:\n..\n")
	specialized, err := Specialize(*parsed)
	if err != nil {
		t.Fatal(err)
	}
	linked, err := Link(specialized)
	if err != nil {
		t.Fatal(err)
	}
	typed, err := CheckTypes(linked)
	if err != nil {
		t.Fatal(err)
	}
	validated, err := ValidateLowering(typed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CheckSafety(validated, true); err != nil {
		t.Fatal(err)
	}
	warnings := parsed.State().Warnings
	if len(warnings) != 1 || warnings[0].Severity != types.SeverityWarning ||
		warnings[0].Stage != "ownership checking" || !strings.Contains(warnings[0].Message, "not consumed") {
		t.Fatalf("ownership warnings were not collected: %#v", warnings)
	}
}

func testProgram(t *testing.T, source string) (*ParsedProgram, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "main.mg")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	stdRoot, err := filepath.Abs(filepath.Join("..", "..", "std"))
	if err != nil {
		t.Fatal(err)
	}
	state, err := shared.MakeShared(dir, stdRoot)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(state, path)
	if err != nil {
		t.Fatalf("parse pipeline: %v", err)
	}
	return &parsed, path
}

func TestStagesPreserveProgramIdentityAndLower(t *testing.T) {
	parsed, path := testProgram(t, "mod main\nmain() void:\n..\n")
	if err := RequireMainModule(*parsed, path); err != nil {
		t.Fatal(err)
	}
	specialized, err := Specialize(*parsed)
	if err != nil {
		t.Fatal(err)
	}
	linked, err := Link(specialized)
	if err != nil {
		t.Fatal(err)
	}
	typed, err := CheckTypes(linked)
	if err != nil {
		t.Fatal(err)
	}
	validated, err := ValidateLowering(typed)
	if err != nil {
		t.Fatal(err)
	}
	ready, err := CheckSafety(validated, true)
	if err != nil {
		t.Fatal(err)
	}
	if ready.State() != parsed.State() {
		t.Fatal("compiler stage replaced the shared program state")
	}
	ir, err := Lower(ready)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ir), ".main()") {
		t.Fatal("lowered IR does not contain main definition")
	}
}

func TestPublicUseReexportLowers(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"heap.mg":    "mod heap\npub allocator() u64:\n    ret 7\n..\n",
		"library.mg": "mod library\npub use \"heap.mg\" heap\n",
		"main.mg":    "mod main\nuse \"library.mg\" lib\nmain() void:\n    value u64 = lib.heap.allocator()\n..\n",
	}
	for name, source := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	stdRoot, err := filepath.Abs(filepath.Join("..", "..", "std"))
	if err != nil {
		t.Fatal(err)
	}
	state, err := shared.MakeShared(dir, stdRoot)
	if err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(dir, "main.mg")
	parsed, err := Parse(state, mainPath)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	specialized, err := Specialize(parsed)
	if err != nil {
		t.Fatalf("specialize: %v", err)
	}
	linked, err := Link(specialized)
	if err != nil {
		t.Fatalf("link: %v", err)
	}
	typed, err := CheckTypes(linked)
	if err != nil {
		t.Fatalf("types: %v", err)
	}
	validated, err := ValidateLowering(typed)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	ready, err := CheckSafety(validated, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Lower(ready); err != nil {
		t.Fatalf("lower: %v", err)
	}
}

func TestParseReturnsPartialProgramOnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.mg")
	if err := os.WriteFile(path, []byte("mod main\nmain() void:\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdRoot, err := filepath.Abs(filepath.Join("..", "..", "std"))
	if err != nil {
		t.Fatal(err)
	}
	state, err := shared.MakeShared(dir, stdRoot)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(state, path)
	if err == nil {
		t.Fatal("expected malformed source to fail")
	}
	if parsed.State() != state || state.Files[path] == nil {
		t.Fatal("parse error discarded the partial program")
	}
}

func TestRequireMainModuleIsAnExplicitStage(t *testing.T) {
	parsed, path := testProgram(t, "mod library\n")
	if err := RequireMainModule(*parsed, path); err == nil {
		t.Fatal("expected non-main root module to be rejected")
	}
}
