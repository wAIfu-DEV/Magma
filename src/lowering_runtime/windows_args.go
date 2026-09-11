package loweringruntime

import (
	"fmt"

	lb "Magma/src/lowering_backend"
	loweringtypes "Magma/src/lowering_types"
	t "Magma/src/types"
)

type WindowsArgs struct {
	FromUTF16 lb.FunctionID
	FreeUTF8  lb.FunctionID
}

func BuildWindowsArgs(backend lb.Backend, types *loweringtypes.Lowerer) (WindowsArgs, error) {
	if backend == nil || types == nil {
		return WindowsArgs{}, fmt.Errorf("Windows argument helpers require backend and type lowering")
	}
	i1, _ := backend.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 1})
	i32, _ := backend.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 32})
	i64, _ := backend.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 64})
	pointer, _ := backend.InternType(lb.TypeSpec{Kind: lb.TypePointer})
	void, _ := backend.InternType(lb.TypeSpec{Kind: lb.TypeVoid})
	stringType, err := types.Lower(&t.NodeType{KindNode: &t.NodeTypeAbsolute{CoreRole: t.CoreTypeString}})
	if err != nil {
		return WindowsArgs{}, err
	}
	getHeap, err := backend.DeclareFunction(lb.FunctionSpec{Symbol: "GetProcessHeap", Result: pointer, Linkage: lb.LinkageExternal, CallingConvention: lb.CallingConventionC})
	if err != nil {
		return WindowsArgs{}, err
	}
	heapAlloc, err := backend.DeclareFunction(lb.FunctionSpec{Symbol: "HeapAlloc", Result: pointer, Parameters: []lb.TypeID{pointer, i32, i64}, Linkage: lb.LinkageExternal, CallingConvention: lb.CallingConventionC})
	if err != nil {
		return WindowsArgs{}, err
	}
	heapFree, err := backend.DeclareFunction(lb.FunctionSpec{Symbol: "HeapFree", Result: i32, Parameters: []lb.TypeID{pointer, i32, pointer}, Linkage: lb.LinkageExternal, CallingConvention: lb.CallingConventionC})
	if err != nil {
		return WindowsArgs{}, err
	}
	wideToUTF8, err := backend.DeclareFunction(lb.FunctionSpec{Symbol: "WideCharToMultiByte", Result: i32, Parameters: []lb.TypeID{i32, i32, pointer, i32, pointer, i32, pointer, pointer}, Linkage: lb.LinkageExternal, CallingConvention: lb.CallingConventionC})
	if err != nil {
		return WindowsArgs{}, err
	}
	from, err := buildFromUTF16(backend, types, i1, i32, i64, pointer, stringType, getHeap, heapAlloc, heapFree, wideToUTF8)
	if err != nil {
		return WindowsArgs{}, err
	}
	free, err := buildFreeUTF8(backend, types, void, i32, i64, pointer, stringType, getHeap, heapFree)
	if err != nil {
		return WindowsArgs{}, err
	}
	return WindowsArgs{FromUTF16: from, FreeUTF8: free}, nil
}

func buildFromUTF16(backend lb.Backend, types *loweringtypes.Lowerer, i1, i32, i64, pointer, stringType lb.TypeID, getHeap, heapAlloc, heapFree, convert lb.FunctionID) (lb.FunctionID, error) {
	fn, err := backend.DeclareFunction(lb.FunctionSpec{Symbol: "magma.argsFromUtf16", Result: i1, Parameters: []lb.TypeID{i32, pointer, pointer}, Linkage: lb.LinkageInternal, CallingConvention: lb.CallingConventionC, Definition: true})
	if err != nil {
		return 0, err
	}
	entry, _ := backend.AppendBlock(fn, "entry")
	loop, _ := backend.AppendBlock(fn, "loop")
	conversion, _ := backend.AppendBlock(fn, "convert")
	allocate, _ := backend.AppendBlock(fn, "allocate")
	encode, _ := backend.AppendBlock(fn, "encode")
	store, _ := backend.AppendBlock(fn, "store")
	freeCurrent, _ := backend.AppendBlock(fn, "free.current")
	fail, _ := backend.AppendBlock(fn, "fail")
	cleanup, _ := backend.AppendBlock(fn, "cleanup")
	cleanupBody, _ := backend.AppendBlock(fn, "cleanup.body")
	failure, _ := backend.AppendBlock(fn, "failure")
	success, _ := backend.AppendBlock(fn, "success")
	argc, _ := backend.Parameter(fn, 0)
	argv, _ := backend.Parameter(fn, 1)
	buffer, _ := backend.Parameter(fn, 2)
	heap, err := backend.Call(entry, getHeap, nil)
	if err != nil {
		return 0, err
	}
	argc64, err := backend.Cast(entry, lb.CastSignExtend, argc, i64)
	if err != nil {
		return 0, err
	}
	zero32, err := constantValue(backend, i32, "0")
	if err != nil {
		return 0, err
	}
	zero64, err := constantValue(backend, i64, "0")
	if err != nil {
		return 0, err
	}
	one32, err := constantValue(backend, i32, "1")
	if err != nil {
		return 0, err
	}
	one64, err := constantValue(backend, i64, "1")
	if err != nil {
		return 0, err
	}
	codePage, _ := constantValue(backend, i32, "65001")
	flags, _ := constantValue(backend, i32, "128")
	minusOne, _ := constantValue(backend, i32, "4294967295")
	nullID, _ := backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantNull, Type: pointer})
	null, _ := backend.ConstantValue(nullID)
	falseValue, _ := constantValue(backend, i1, "0")
	trueValue, _ := constantValue(backend, i1, "1")
	if err := backend.Branch(entry, loop); err != nil {
		return 0, err
	}
	index, err := backend.Phi(loop, i64)
	if err != nil {
		return 0, err
	}
	done, err := backend.Compare(loop, lb.CompareEqual, index, argc64)
	if err != nil {
		return 0, err
	}
	if err := backend.CondBranch(loop, done, success, conversion); err != nil {
		return 0, err
	}
	argSlot, err := backend.GEP(conversion, pointer, argv, []lb.ValueID{index}, false)
	if err != nil {
		return 0, err
	}
	wide, err := backend.Load(conversion, pointer, argSlot, 0, false)
	if err != nil {
		return 0, err
	}
	size, err := backend.Call(conversion, convert, []lb.ValueID{codePage, flags, wide, minusOne, null, zero32, null, null})
	if err != nil {
		return 0, err
	}
	sizeOK, err := backend.Compare(conversion, lb.CompareSignedGreater, size, zero32)
	if err != nil {
		return 0, err
	}
	if err := backend.CondBranch(conversion, sizeOK, allocate, fail); err != nil {
		return 0, err
	}
	size64, err := backend.Cast(allocate, lb.CastZeroExtend, size, i64)
	if err != nil {
		return 0, err
	}
	bytes, err := backend.Call(allocate, heapAlloc, []lb.ValueID{heap, zero32, size64})
	if err != nil {
		return 0, err
	}
	allocated, err := backend.Compare(allocate, lb.CompareNotEqual, bytes, null)
	if err != nil {
		return 0, err
	}
	if err := backend.CondBranch(allocate, allocated, encode, fail); err != nil {
		return 0, err
	}
	written, err := backend.Call(encode, convert, []lb.ValueID{codePage, flags, wide, minusOne, bytes, size, null, null})
	if err != nil {
		return 0, err
	}
	encoded, err := backend.Compare(encode, lb.CompareEqual, written, size)
	if err != nil {
		return 0, err
	}
	if err := backend.CondBranch(encode, encoded, store, freeCurrent); err != nil {
		return 0, err
	}
	if _, err := backend.Call(freeCurrent, heapFree, []lb.ValueID{heap, zero32, bytes}); err != nil {
		return 0, err
	}
	if err := backend.Branch(freeCurrent, fail); err != nil {
		return 0, err
	}
	length32, err := backend.Binary(store, lb.BinarySub, size, one32)
	if err != nil {
		return 0, err
	}
	length, err := backend.Cast(store, lb.CastZeroExtend, length32, i64)
	if err != nil {
		return 0, err
	}
	element, err := backend.GEP(store, stringType, buffer, []lb.ValueID{index}, false)
	if err != nil {
		return 0, err
	}
	stringValue, err := types.BuildCoreValueWithDefaults(store, t.CoreTypeString, map[string]lb.ValueID{"__data": bytes, "__byteCount": length})
	if err != nil {
		return 0, err
	}
	if _, err := backend.Store(store, stringValue, element, 0, false); err != nil {
		return 0, err
	}
	next, err := backend.Binary(store, lb.BinaryAdd, index, one64)
	if err != nil {
		return 0, err
	}
	if err := backend.Branch(store, loop); err != nil {
		return 0, err
	}
	if err := backend.AddPhiIncoming(index, []lb.ValueID{zero64, next}, []lb.BlockID{entry, store}); err != nil {
		return 0, err
	}
	if err := backend.Branch(fail, cleanup); err != nil {
		return 0, err
	}
	cleanupIndex, err := backend.Phi(cleanup, i64)
	if err != nil {
		return 0, err
	}
	cleaned, err := backend.Compare(cleanup, lb.CompareEqual, cleanupIndex, index)
	if err != nil {
		return 0, err
	}
	if err := backend.CondBranch(cleanup, cleaned, failure, cleanupBody); err != nil {
		return 0, err
	}
	oldElement, err := backend.GEP(cleanupBody, stringType, buffer, []lb.ValueID{cleanupIndex}, false)
	if err != nil {
		return 0, err
	}
	old, err := backend.Load(cleanupBody, stringType, oldElement, 0, false)
	if err != nil {
		return 0, err
	}
	oldBytes, err := types.ExtractCoreField(cleanupBody, old, t.CoreTypeString, "__data")
	if err != nil {
		return 0, err
	}
	if _, err := backend.Call(cleanupBody, heapFree, []lb.ValueID{heap, zero32, oldBytes}); err != nil {
		return 0, err
	}
	cleanupNext, err := backend.Binary(cleanupBody, lb.BinaryAdd, cleanupIndex, one64)
	if err != nil {
		return 0, err
	}
	if err := backend.Branch(cleanupBody, cleanup); err != nil {
		return 0, err
	}
	if err := backend.AddPhiIncoming(cleanupIndex, []lb.ValueID{zero64, cleanupNext}, []lb.BlockID{fail, cleanupBody}); err != nil {
		return 0, err
	}
	if err := backend.Return(failure, falseValue); err != nil {
		return 0, err
	}
	if err := backend.Return(success, trueValue); err != nil {
		return 0, err
	}
	return fn, backend.FinalizeFunction(fn)
}

func buildFreeUTF8(backend lb.Backend, types *loweringtypes.Lowerer, void, i32, i64, pointer, stringType lb.TypeID, getHeap, heapFree lb.FunctionID) (lb.FunctionID, error) {
	fn, err := backend.DeclareFunction(lb.FunctionSpec{Symbol: "magma.freeUtf8Args", Result: void, Parameters: []lb.TypeID{i32, pointer}, Linkage: lb.LinkageInternal, CallingConvention: lb.CallingConventionC, Definition: true})
	if err != nil {
		return 0, err
	}
	entry, _ := backend.AppendBlock(fn, "entry")
	loop, _ := backend.AppendBlock(fn, "loop")
	body, _ := backend.AppendBlock(fn, "body")
	finish, _ := backend.AppendBlock(fn, "finish")
	argc, _ := backend.Parameter(fn, 0)
	buffer, _ := backend.Parameter(fn, 1)
	heap, err := backend.Call(entry, getHeap, nil)
	if err != nil {
		return 0, err
	}
	argc64, err := backend.Cast(entry, lb.CastSignExtend, argc, i64)
	if err != nil {
		return 0, err
	}
	zero32, _ := constantValue(backend, i32, "0")
	zero64, _ := constantValue(backend, i64, "0")
	one64, _ := constantValue(backend, i64, "1")
	if err := backend.Branch(entry, loop); err != nil {
		return 0, err
	}
	index, err := backend.Phi(loop, i64)
	if err != nil {
		return 0, err
	}
	done, err := backend.Compare(loop, lb.CompareEqual, index, argc64)
	if err != nil {
		return 0, err
	}
	if err := backend.CondBranch(loop, done, finish, body); err != nil {
		return 0, err
	}
	element, err := backend.GEP(body, stringType, buffer, []lb.ValueID{index}, false)
	if err != nil {
		return 0, err
	}
	value, err := backend.Load(body, stringType, element, 0, false)
	if err != nil {
		return 0, err
	}
	bytes, err := types.ExtractCoreField(body, value, t.CoreTypeString, "__data")
	if err != nil {
		return 0, err
	}
	if _, err := backend.Call(body, heapFree, []lb.ValueID{heap, zero32, bytes}); err != nil {
		return 0, err
	}
	next, err := backend.Binary(body, lb.BinaryAdd, index, one64)
	if err != nil {
		return 0, err
	}
	if err := backend.Branch(body, loop); err != nil {
		return 0, err
	}
	if err := backend.AddPhiIncoming(index, []lb.ValueID{zero64, next}, []lb.BlockID{entry, body}); err != nil {
		return 0, err
	}
	if err := backend.ReturnVoid(finish); err != nil {
		return 0, err
	}
	return fn, backend.FinalizeFunction(fn)
}

func constantValue(backend lb.Backend, typ lb.TypeID, value string) (lb.ValueID, error) {
	constant, err := backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: typ, Integer: value})
	if err != nil {
		return 0, err
	}
	return backend.ConstantValue(constant)
}
