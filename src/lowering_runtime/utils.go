// Package loweringruntime contains canonical backend-neutral object recipes for
// Magma's compiler-provided runtime helpers. These recipes are the source of
// truth for both object linking and generated textual compatibility artifacts.
package loweringruntime

import (
	"fmt"

	lb "Magma/src/lowering_backend"
)

// Utils identifies the runtime entities constructed by BuildUtils.
type Utils struct {
	String      lb.TypeID
	Slice       lb.TypeID
	ArgsToSlice lb.FunctionID
}

// BuildUtils constructs the bootstrap helpers formerly maintained in
// llvm_fragments/utils.ll. It intentionally adds no checks beyond that textual
// implementation.
func BuildUtils(backend lb.Backend) (Utils, error) {
	if backend == nil {
		return Utils{}, fmt.Errorf("runtime utilities require a lowering backend")
	}
	i1, err := backend.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 1})
	if err != nil {
		return Utils{}, err
	}
	i32, err := backend.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 32})
	if err != nil {
		return Utils{}, err
	}
	i64, err := backend.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 64})
	if err != nil {
		return Utils{}, err
	}
	pointer, err := backend.InternType(lb.TypeSpec{Kind: lb.TypePointer})
	if err != nil {
		return Utils{}, err
	}
	stringType, err := backend.InternStruct(lb.StructSpec{Name: "type.str", Elements: []lb.TypeID{pointer, i64, pointer, pointer}})
	if err != nil {
		return Utils{}, err
	}
	sliceType, err := backend.InternStruct(lb.StructSpec{Name: "type.slice", Elements: []lb.TypeID{pointer, i64}})
	if err != nil {
		return Utils{}, err
	}
	voidType, err := backend.InternType(lb.TypeSpec{Kind: lb.TypeVoid})
	if err != nil {
		return Utils{}, err
	}
	i8, err := backend.InternType(lb.TypeSpec{Kind: lb.TypeInteger, Bits: 8})
	if err != nil {
		return Utils{}, err
	}
	if _, err := backend.DeclareFunction(lb.FunctionSpec{
		Symbol: "llvm.memset.p0.i64", Result: voidType,
		Parameters: []lb.TypeID{pointer, i8, i64, i1}, Linkage: lb.LinkageExternal,
		CallingConvention: lb.CallingConventionC,
	}); err != nil {
		return Utils{}, err
	}
	strlen, err := backend.DeclareFunction(lb.FunctionSpec{
		Symbol: "strlen", Result: i64, Parameters: []lb.TypeID{pointer},
		Linkage: lb.LinkageExternal, CallingConvention: lb.CallingConventionC,
		Attributes: []lb.AttributeSpec{
			{Kind: lb.AttributeNoUnwind, Placement: lb.AttributeFunction},
		},
	})
	if err != nil {
		return Utils{}, err
	}
	function, err := backend.DeclareFunction(lb.FunctionSpec{
		Symbol: "magma.argsToSlice", Result: sliceType,
		Parameters: []lb.TypeID{i32, pointer, pointer}, Linkage: lb.LinkageInternal,
		CallingConvention: lb.CallingConventionC, Definition: true,
	})
	if err != nil {
		return Utils{}, err
	}
	enter, err := backend.AppendBlock(function, "enter")
	if err != nil {
		return Utils{}, err
	}
	loop, err := backend.AppendBlock(function, "loop")
	if err != nil {
		return Utils{}, err
	}
	body, err := backend.AppendBlock(function, "loop.body")
	if err != nil {
		return Utils{}, err
	}
	finish, err := backend.AppendBlock(function, "finish")
	if err != nil {
		return Utils{}, err
	}
	argc, err := backend.Parameter(function, 0)
	if err != nil {
		return Utils{}, err
	}
	argv, err := backend.Parameter(function, 1)
	if err != nil {
		return Utils{}, err
	}
	buffer, err := backend.Parameter(function, 2)
	if err != nil {
		return Utils{}, err
	}
	argc64, err := backend.Cast(enter, lb.CastSignExtend, argc, i64)
	if err != nil {
		return Utils{}, err
	}
	zeroID, err := backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: i64, Integer: "0"})
	if err != nil {
		return Utils{}, err
	}
	oneID, err := backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantInteger, Type: i64, Integer: "1"})
	if err != nil {
		return Utils{}, err
	}
	zero, err := backend.ConstantValue(zeroID)
	if err != nil {
		return Utils{}, err
	}
	one, err := backend.ConstantValue(oneID)
	if err != nil {
		return Utils{}, err
	}
	nullID, err := backend.InternConstant(lb.ConstantSpec{Kind: lb.ConstantNull, Type: pointer})
	if err != nil {
		return Utils{}, err
	}
	nullPointer, err := backend.ConstantValue(nullID)
	if err != nil {
		return Utils{}, err
	}
	if err := backend.Branch(enter, loop); err != nil {
		return Utils{}, err
	}
	index, err := backend.Phi(loop, i64)
	if err != nil {
		return Utils{}, err
	}
	done, err := backend.Compare(loop, lb.CompareEqual, index, argc64)
	if err != nil {
		return Utils{}, err
	}
	if err := backend.CondBranch(loop, done, finish, body); err != nil {
		return Utils{}, err
	}
	argumentAddress, err := backend.GEP(body, pointer, argv, []lb.ValueID{index}, false)
	if err != nil {
		return Utils{}, err
	}
	cString, err := backend.Load(body, pointer, argumentAddress, 0, false)
	if err != nil {
		return Utils{}, err
	}
	length, err := backend.Call(body, strlen, []lb.ValueID{cString})
	if err != nil {
		return Utils{}, err
	}
	elementAddress, err := backend.GEP(body, stringType, buffer, []lb.ValueID{index}, false)
	if err != nil {
		return Utils{}, err
	}
	stringValue, err := backend.BuildAggregate(body, stringType, []lb.ValueID{cString, length, nullPointer, nullPointer})
	if err != nil {
		return Utils{}, err
	}
	if _, err := backend.Store(body, stringValue, elementAddress, 0, false); err != nil {
		return Utils{}, err
	}
	next, err := backend.Binary(body, lb.BinaryAdd, index, one)
	if err != nil {
		return Utils{}, err
	}
	if err := backend.Branch(body, loop); err != nil {
		return Utils{}, err
	}
	if err := backend.AddPhiIncoming(index, []lb.ValueID{zero, next}, []lb.BlockID{enter, body}); err != nil {
		return Utils{}, err
	}
	result, err := backend.BuildAggregate(finish, sliceType, []lb.ValueID{buffer, argc64})
	if err != nil {
		return Utils{}, err
	}
	if err := backend.Return(finish, result); err != nil {
		return Utils{}, err
	}
	if err := backend.FinalizeFunction(function); err != nil {
		return Utils{}, err
	}
	return Utils{String: stringType, Slice: sliceType, ArgsToSlice: function}, nil
}
