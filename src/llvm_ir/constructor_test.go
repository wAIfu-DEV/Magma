package llvmir_test

import (
	"Magma/src/checker"
	"Magma/src/join"
	llvmir "Magma/src/llvm_ir"
	"Magma/src/monomorph"
	"Magma/src/pipeline"
	"Magma/src/shared"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func compileSource(t *testing.T, source string) (string, error) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.mg")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	state, err := shared.MakeShared(dir)
	if err != nil {
		t.Fatal(err)
	}
	err = pipeline.DoMain(state, path)
	if err = join.JoinCompilationUnits(state, err); err != nil {
		return "", err
	}
	if err = monomorph.Run(state); err != nil {
		return "", err
	}
	if err = checker.CheckLinks(state); err != nil {
		return "", err
	}
	if err = checker.TypeChecker(state); err != nil {
		return "", err
	}
	ir, err := llvmir.IrWrite(state)
	return string(ir), err
}

func TestAddrofSubscriptLowers(t *testing.T) {
	source := `mod main

elementAddress(items u64[], index u64) ptr:
    ret addrof items[index]
..

main() void:
    ret
..
`

	if _, err := compileSource(t, source); err != nil {
		t.Fatalf("addrof on a subscript failed to lower: %v", err)
	}
}

func TestPointerMemberLvalueLoadsPointerBeforeFieldAddress(t *testing.T) {
	source := `mod main

State(value u64)
Wrapper(state State*)

setValue(wrapper Wrapper*) void:
    wrapper.state.value = 42
..

main() void:
    ret
..
`

	ir, err := compileSource(t, source)
	if err != nil {
		t.Fatalf("pointer member lvalue failed to lower: %v", err)
	}
	namePos := strings.Index(ir, ".setValue(")
	if namePos < 0 {
		t.Fatal("setValue function was not emitted")
	}
	start := strings.LastIndex(ir[:namePos], "define ")
	endOffset := strings.Index(ir[namePos:], "\n}")
	if start < 0 || endOffset < 0 {
		t.Fatal("could not isolate emitted setValue function")
	}
	setValueIR := ir[start : namePos+endOffset]
	if !strings.Contains(setValueIR, "load ptr, ptr") {
		t.Fatal("pointer member lvalue did not load its pointer field")
	}
}

func TestMemberCallOnPointerFieldLoadsOwner(t *testing.T) {
	source := `mod main

State(value u64)
State.get() u64:
    ret this.value
..

Wrapper(state State*)

read(wrapper Wrapper*) u64:
    ret wrapper.state.get()
..

main() void:
    ret
..
`

	ir, err := compileSource(t, source)
	if err != nil {
		t.Fatalf("member call on pointer field failed to lower: %v", err)
	}
	namePos := strings.Index(ir, ".read(")
	if namePos < 0 {
		t.Fatal("read function was not emitted")
	}
	start := strings.LastIndex(ir[:namePos], "define ")
	endOffset := strings.Index(ir[namePos:], "\n}")
	if start < 0 || endOffset < 0 {
		t.Fatal("could not isolate emitted read function")
	}
	readIR := ir[start : namePos+endOffset]
	if !strings.Contains(readIR, "load ptr, ptr") {
		t.Fatal("member call did not load its pointer-valued owner field")
	}
}

func TestTypeErasedPointerConversions(t *testing.T) {
	source := `mod main

typedToErased(value u8*) ptr:
    erased ptr = value
    ret erased
..

erasedToTyped(value ptr) u8*:
    typed u8* = value
    ret typed
..

produceTyped() !u8*:
    ret none
..

produceErased() !ptr:
    ret none
..

throwingTypedToErased() !ptr:
    erased ptr = try produceTyped()
    ret erased
..

throwingErasedToTyped() !u8*:
    typed u8* = try produceErased()
    ret typed
..

main() void:
..
`

	if _, err := compileSource(t, source); err != nil {
		t.Fatalf("compile type-erased pointer conversions: %v", err)
	}
}

func TestPointerDereferenceAssignmentLowers(t *testing.T) {
	source := `mod test

write(p u64*, value u64) void:
    *p = value
..
`
	ir, err := compileSource(t, source)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ir, "store i64") {
		t.Fatalf("expected dereference assignment to lower to a store, got:\n%s", ir)
	}
}

func TestInferredThrowingCallDestructuringLowers(t *testing.T) {
	source := `mod test

produce() !u64:
    ret 7
..

consume(value u64, problem error) void:
..

main() void:
    value, problem := produce()
    consume(value, problem)
..
`
	ir, err := compileSource(t, source)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ir, "extractvalue") {
		t.Fatal("inferred destructuring did not lower the throwing result")
	}
}

func TestErrorTraceSitesLowerOnColdPropagationEdges(t *testing.T) {
	source := `mod test

fail() !u8:
    throw errorValue()
..

errorValue() error:
    llvm "  ret %type.error zeroinitializer\n"
..

main() !void:
    value := try fail()
    other := try fail()
..
`
	ir, err := compileSource(t, source)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ir, "%type.error = type { ptr, i64, i32, i32 }") {
		t.Fatal("error ABI is not the compact three-word representation")
	}
	if !strings.Contains(ir, "%type.error.trace.arena = type { [1024 x %type.error.trace.node], [0 x i8] }") ||
		!strings.Contains(ir, "@magma.error.trace.thread.shard") ||
		!strings.Contains(ir, "%type.error.trace.shard = type { i64, i8, [55 x i8] }") ||
		strings.Contains(ir, "@magma.error.trace.cursor =") {
		t.Fatal("error traces do not use the configured sharded ring")
	}
	// The implicit core imports allocator support, whose two throwing wrappers
	// also contain propagation sites. The test program itself contributes the
	// original throw and two try sites.
	if got := strings.Count(ir, "call %type.error @magma.error.push"); got != 5 {
		t.Fatalf("got %d trace pushes, want three program sites and two core dependency sites", got)
	}
	if got := strings.Count(ir, "!prof !9000"); got != 6 {
		t.Fatalf("got %d unlikely error branches, want four program branches and two core dependency branches", got)
	}
	if !strings.Contains(ir, "@magma.error.push(%type.error %error, ptr %site) cold noinline") {
		t.Fatal("trace recording helper is not kept out of hot callers")
	}
	if !strings.Contains(ir, "private constant %type.error.site") ||
		!strings.Contains(ir, "i32 4, i32 5") ||
		!strings.Contains(ir, "i32 12, i32 13") {
		t.Fatalf("generated trace sites do not retain throw/try source positions:\n%s", ir)
	}
	if !strings.Contains(ir, "private unnamed_addr constant [8 x i8] c\"test.mg\\00\"") {
		t.Fatal("trace metadata does not use the source basename")
	}
	if got := strings.Count(ir, "c\"test.mg\\00\""); got != 1 {
		t.Fatalf("trace filename emitted %d times, want one interned constant", got)
	}
	if got := strings.Count(ir, "c\"fail\\00\""); got != 1 {
		t.Fatalf("trace function name emitted %d times, want one interned constant", got)
	}
	if got := strings.Count(ir, "c\"main\\00\""); got != 1 {
		t.Fatalf("repeated trace function name emitted %d times, want one interned constant", got)
	}
	if !strings.Contains(ir, "call void @magma.error.print(%type.error %e)") {
		t.Fatal("throwing main does not print its uncaught error trace")
	}
}

func TestInferredDestructuringRejectsNonThrowingCall(t *testing.T) {
	source := `mod test

produce() u64:
    ret 7
..

main() void:
    value, problem := produce()
..
`
	_, err := compileSource(t, source)
	if err == nil || !strings.Contains(err.Error(), "cannot destructure non-throwing call") {
		t.Fatalf("error = %v, want throwing-call diagnostic", err)
	}
}

func TestInferredDestructuringRejectsThrowingVoidCall(t *testing.T) {
	source := `mod test

produce() !void:
..

main() void:
    value, problem := produce()
..
`
	_, err := compileSource(t, source)
	if err == nil || !strings.Contains(err.Error(), "cannot bind a result value from throwing void call") {
		t.Fatalf("error = %v, want !void diagnostic", err)
	}
}

func TestStructConstructorsAndConstantsLower(t *testing.T) {
	source := `mod test

VTable(
    fn_call (ptr) ptr
    context ptr
)

identity(p ptr) ptr:
    ret p
..

const table := VTable(
    context=none
    fn_call=identity
)

make() VTable:
    ret VTable(context=none, fn_call=identity)
..
`
	ir, err := compileSource(t, source)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ir, "private constant") || !strings.Contains(ir, "* @test_") {
		t.Fatalf("expected aggregate LLVM constant, got:\n%s", ir)
	}
	if !strings.Contains(ir, "insertvalue") {
		t.Fatalf("expected runtime constructor to use insertvalue, got:\n%s", ir)
	}
}

func TestStructConstructorNewlinesSeparateFields(t *testing.T) {
	source := `mod test

Pair(first u64, second u64)

make() Pair:
    ret Pair(
        first=1
        second=2
    )
..
`
	ir, err := compileSource(t, source)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ir, "insertvalue") {
		t.Fatalf("expected newline-separated constructor to lower, got:\n%s", ir)
	}
}

func TestGenericStructConstructorMonomorphizes(t *testing.T) {
	source := `mod test

Pair[T](value T)

make() Pair[u64]:
    ret Pair[u64](value=1)
..
`
	ir, err := compileSource(t, source)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ir, "Pair__g__N_u64") {
		t.Fatalf("expected specialized Pair type, got:\n%s", ir)
	}
}

func TestConstructorRejectsMissingField(t *testing.T) {
	source := `mod test

Pair(first u64, second u64)

make() Pair:
    ret Pair(first=1)
..
`
	_, err := compileSource(t, source)
	if err == nil || !strings.Contains(err.Error(), "missing field 'second'") {
		t.Fatalf("expected missing-field error, got %v", err)
	}
}

func TestConstructorFieldDiagnostics(t *testing.T) {
	tests := []struct {
		name   string
		fields string
		want   string
	}{
		{name: "duplicate", fields: "first=1, first=2", want: "duplicate field 'first'"},
		{name: "unknown", fields: "first=1, other=2", want: "has no field named 'other'"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := "mod test\n\nPair(first u64, second u64)\n\nmake() Pair:\n    ret Pair(" + test.fields + ")\n..\n"
			_, err := compileSource(t, source)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q, got %v", test.want, err)
			}
		})
	}
}

func TestExplicitConstAndUnsupportedInitializer(t *testing.T) {
	valid := `mod test

const answer u64 = 42
`
	ir, err := compileSource(t, valid)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ir, "private constant i64 42") {
		t.Fatalf("expected explicit numeric constant, got:\n%s", ir)
	}

	invalid := `mod test

value() u64:
    ret 42
..

const answer := value()
`
	_, err = compileSource(t, invalid)
	if err == nil || !strings.Contains(err.Error(), "constant initializer must be") {
		t.Fatalf("expected unsupported constant initializer error, got %v", err)
	}
}

func TestThrowStringBuildsGenericFailure(t *testing.T) {
	source := `mod main
fail() !void:
    throw "message"
..
main() void:
..
`
	ir, err := compileSource(t, source)
	if err != nil {
		t.Fatalf("compile string throw: %v", err)
	}
	if !strings.Contains(ir, "insertvalue %type.error") || !strings.Contains(ir, "i32 1, 2") {
		t.Fatalf("expected string throw to build error code 1, got:\n%s", ir)
	}
}

func TestTryBindsBeforeBinaryComparison(t *testing.T) {
	source := `mod test

number() !f64:
    ret 1.0
..

different() !bool:
    ret try number() != 2.0
..
`
	ir, err := compileSource(t, source)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ir, "fcmp une double") {
		t.Fatalf("expected a floating inequality after the tried call, got:\n%s", ir)
	}
}

func TestStringLiteralEscapesAreValidLLVM(t *testing.T) {
	source := `mod test

value() str:
    ret "quote:\" slash:\\ line:\n"
..
`
	ir, err := compileSource(t, source)
	if err != nil {
		t.Fatal(err)
	}
	for _, escaped := range []string{`\22`, `\5C`, `\0A`} {
		if !strings.Contains(ir, escaped) {
			t.Fatalf("expected LLVM string escape %q, got:\n%s", escaped, ir)
		}
	}
}

func TestAddrofFunctionArgumentMaterializesStorage(t *testing.T) {
	source := `mod test

read(value u64) u64:
    pointer u64* = addrof value
    ret *pointer
..
`
	ir, err := compileSource(t, source)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ir, "store i64 %value, ptr %value.addr") {
		t.Fatalf("expected addrof argument to materialize an address, got:\n%s", ir)
	}
}

func TestFunctionArgumentAssignmentUsesStableStorage(t *testing.T) {
	source := `mod test

change(value u64) u64:
    pointer u64* = addrof value
    value = 42
    ret *pointer
..
`
	ir, err := compileSource(t, source)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"i64 %value",
		"%value.addr = alloca i64",
		"store i64 %value, ptr %value.addr",
		"store i64 42, ptr %value.addr",
	} {
		if !strings.Contains(ir, expected) {
			t.Fatalf("expected mutable argument storage %q, got:\n%s", expected, ir)
		}
	}
}

func TestNarrowStructFieldKeepsNativeWidthAndLayout(t *testing.T) {
	source := `mod test

Native(prefix u32, port u16, next ptr)

read(value Native*) u16:
    ret value.port
..
`
	ir, err := compileSource(t, source)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ir, "= type { i32, i16, ptr }") {
		t.Fatalf("expected native-width u16 struct field, got:\n%s", ir)
	}
	if !strings.Contains(ir, "extractvalue %struct.") || !strings.Contains(ir, ", 1") {
		t.Fatalf("expected field 1 extraction for u16 field, got:\n%s", ir)
	}
}

func TestNestedVariableShadowingIsRejected(t *testing.T) {
	source := `mod test

invalid(value u64) u64:
    if value != 0:
        value u64 = 1
    ..
    ret value
..
`
	_, err := compileSource(t, source)
	if err == nil || !strings.Contains(err.Error(), "shadowing is not allowed") {
		t.Fatalf("expected nested shadowing error, got %v", err)
	}
}

func TestSameScopeDuplicateVariableIsRejected(t *testing.T) {
	source := `mod test

invalid() u64:
    value u64 = 1
    value u64 = 2
    ret value
..
`
	_, err := compileSource(t, source)
	if err == nil || !strings.Contains(err.Error(), "already declared") {
		t.Fatalf("expected duplicate declaration error, got %v", err)
	}
}

func TestVariableCannotShadowFunctionPointerName(t *testing.T) {
	source := `mod test

callback() u64:
    ret 1
..

invalid() u64:
    callback u64 = 2
    ret callback
..
`
	_, err := compileSource(t, source)
	if err == nil || !strings.Contains(err.Error(), "conflicts with a function") {
		t.Fatalf("expected function-name shadowing error, got %v", err)
	}
}

func TestFunctionCannotShadowGlobalVariable(t *testing.T) {
	source := `mod test

callback u64

callback() u64:
    ret 1
..
`
	_, err := compileSource(t, source)
	if err == nil || !strings.Contains(err.Error(), "conflicts with a variable") {
		t.Fatalf("expected variable-name shadowing error, got %v", err)
	}
}

func TestAddrofFunctionArgumentFieldMaterializesStorage(t *testing.T) {
	source := `mod test

Value(number u64)

address(value Value) ptr:
    ret addrof value.number
..
`
	ir, err := compileSource(t, source)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ir, "store %struct.") || !strings.Contains(ir, "getelementptr %struct.") {
		t.Fatalf("expected addrof argument field to materialize aggregate storage, got:\n%s", ir)
	}
}

func TestBranchLocalNamesReceiveUniqueLLVMNames(t *testing.T) {
	source := `mod test

choose(first bool) u64:
    if first:
        value u64 = 1
        ret value
    ..
    value u64 = 2
    ret value
..
`
	ir, err := compileSource(t, source)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(ir, "%value = alloca") {
		t.Fatalf("expected locals to use unique LLVM names, got:\n%s", ir)
	}
}

func TestSubscriptOnPointerValuedMember(t *testing.T) {
	source := `mod test

Item(value u64)
Box(items Item*)

Item.get() u64:
    ret this.value
..

read(box Box*) u64:
    ret box.items[0].get()
..
`
	if _, err := compileSource(t, source); err != nil {
		t.Fatal(err)
	}
}

func TestSubscriptOnMemberThroughPointerLocal(t *testing.T) {
	source := `mod test

Box(items u64*)

read(raw ptr) u64:
    box Box* = raw
    ret box.items[0]
..
`
	if _, err := compileSource(t, source); err != nil {
		t.Fatal(err)
	}
}

func TestMethodCallOnPointerArgumentLoadsImplicitOwner(t *testing.T) {
	source := `mod test

Item(value u64)

Item.get() u64:
    ret this.value
..

read(item Item*) u64:
    ret item.get()
..
`
	ir, err := compileSource(t, source)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ir, "load ptr, ptr %item.addr") {
		t.Fatalf("expected pointer argument to be loaded before use as implicit this, got:\n%s", ir)
	}
	if strings.Contains(ir, "call i64 @test_") && strings.Contains(ir, "(ptr %item.addr)") {
		t.Fatalf("method call passed the pointer's storage as implicit this:\n%s", ir)
	}
}

func TestMethodCallOnValueArgumentUsesImplicitOwnerAddress(t *testing.T) {
	source := `mod test

Item(value u64)

Item.get() u64:
    ret this.value
..

read(item Item) u64:
    ret item.get()
..
`
	ir, err := compileSource(t, source)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ir, "Item.get(ptr %item.addr)") {
		t.Fatalf("expected value argument storage to be used as implicit this, got:\n%s", ir)
	}
}

func TestTypedPointerFieldComparisonKeepsPointerType(t *testing.T) {
	source := `mod test

State(value u64)
Pool(state State*)

isEmpty(pool Pool*) bool:
    ret pool.state == none
..
`
	ir, err := compileSource(t, source)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ir, "icmp eq ptr") {
		t.Fatalf("expected typed pointer field comparison to use ptr, got:\n%s", ir)
	}
	if strings.Contains(ir, "icmp eq %struct.") {
		t.Fatalf("typed pointer field comparison used the pointee struct type:\n%s", ir)
	}
}
