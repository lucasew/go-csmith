package csmith

import (
	"strings"
	"testing"
)

func TestChooseVarFullInvalidVars(t *testing.T) {
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	b := CreateVariableScalars("g_b", GetIntType(), false, false)
	// invalid_vars contains a → only b
	got := ChooseVarFull(NewRng(2), []*Variable{a, b}, AccessRead, EmptyCGContext(),
		GetIntType(), nil, MatchFlexible, []*Variable{a}, false, false, false)
	if got != b {
		t.Fatalf("got %v want b", got)
	}
}

func TestChooseVarFullNoBitfield(t *testing.T) {
	bf := CreateVariableScalars("g_bf", GetIntType(), false, false)
	bf.IsBitfield = true
	ok := CreateVariableScalars("g_ok", GetIntType(), false, false)
	got := ChooseVarFull(NewRng(3), []*Variable{bf, ok}, AccessRead, EmptyCGContext(),
		GetIntType(), nil, MatchFlexible, nil, true, false, false)
	if got != ok {
		t.Fatalf("got %v want ok", got)
	}
}

func TestChooseVarFullNoUnion(t *testing.T) {
	ut := &Type{
		isUnion:    true,
		StructName: "U0",
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1},
			{Name: "f1", Type: GetIntType(), BitWidth: -1},
		},
	}
	uv := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	if len(uv.FieldVars) < 1 {
		t.Fatal("need fields")
	}
	plain := CreateVariableScalars("g_p", GetIntType(), false, false)
	// expand would surface union fields; noUnion must reject them
	got := ChooseVarFull(NewRng(5), []*Variable{uv, plain}, AccessRead, EmptyCGContext(),
		GetIntType(), nil, MatchFlexible, nil, false, false, true)
	if got != plain {
		t.Fatalf("got %v want plain", got)
	}
}

func TestChooseVarFullNoExpandKeepsStruct(t *testing.T) {
	st := &Type{
		isStruct:   true,
		StructName: "S0",
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1},
		},
	}
	sv := CreateVariableQfer("g_s", st, NewCVQualifiers([]bool{false}, []bool{false}))
	// no expand + want int → struct itself does not match int
	got := ChooseVarFull(NewRng(1), []*Variable{sv}, AccessRead, EmptyCGContext(),
		GetIntType(), nil, MatchFlexible, nil, false, true, false)
	if got != nil {
		t.Fatalf("want nil without expand, got %v", got)
	}
	// with expand (default) → field selected
	got2 := ChooseVarFull(NewRng(1), []*Variable{sv}, AccessRead, EmptyCGContext(),
		GetIntType(), nil, MatchFlexible, nil, false, false, false)
	if got2 == nil || got2 != sv.FieldVars[0] {
		t.Fatalf("want field, got %v", got2)
	}
}

func TestHashOutputWithUnionFactsSkipsUnread(t *testing.T) {
	ut := &Type{
		isUnion:    true,
		StructName: "U0",
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1},
			{Name: "f1", Type: GetIntType(), BitWidth: -1},
		},
	}
	uv := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	// nil facts → hash all (HashOutput path)
	all := uv.HashOutput()
	if !strings.Contains(all, "g_u.f0") || !strings.Contains(all, "g_u.f1") {
		t.Fatal(all)
	}
	// last write f0 only
	facts := []*FactUnion{MakeFactUnion(uv, 0)}
	out := uv.HashOutputWithUnionFacts(facts)
	if !strings.Contains(out, "g_u.f0") {
		t.Fatal("want f0", out)
	}
	if strings.Contains(out, "g_u.f1") {
		t.Fatal("must skip f1", out)
	}
}

func TestRecordPointerAvailForDeref(t *testing.T) {
	BookkeeperDoFinalization()
	before := pointerAvailForDeref
	RecordPointerAvailForDeref()
	if pointerAvailForDeref != before+1 {
		t.Fatalf("got %d want %d", pointerAvailForDeref, before+1)
	}
}

func TestChooseVarFromOKPreferDeref(t *testing.T) {
	// VariableSelector.cpp:459–471 — among ok with size>1, prefer higher indirection
	iv := CreateVariableScalars("g_i", GetIntType(), false, false)
	pv := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	opts := Defaults()
	// multiple seeds: always prefer pointer when both match want int
	for seed := uint64(1); seed < 20; seed++ {
		got := chooseVarFromOK(NewRng(seed), GetIntType(), []*Variable{iv, pv}, opts)
		if got != pv {
			t.Fatalf("seed %d: got %v want ptr", seed, got)
		}
	}
}

func TestChooseVarFromOKPreferAddressOf(t *testing.T) {
	// VariableSelector.cpp:484–514 — want pointer, prefer lower-indirection (take address)
	iv := CreateVariableScalars("g_i", GetIntType(), false, false)
	pv := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	want := PointerTo(GetIntType())
	opts := Defaults()
	for seed := uint64(1); seed < 20; seed++ {
		got := chooseVarFromOK(NewRng(seed), want, []*Variable{iv, pv}, opts)
		if got != iv {
			t.Fatalf("seed %d: got %v want addressable int", seed, got)
		}
	}
}

func TestChooseVarFromOKNoUnionFieldAddr(t *testing.T) {
	// take_union_field_addr off → skip union fields in addressable bias
	ut := &Type{
		isUnion:    true,
		StructName: "U0",
		Fields: []StructField{
			{Name: "f0", Type: GetIntType(), BitWidth: -1},
		},
	}
	uv := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	if len(uv.FieldVars) < 1 {
		t.Fatal("fields")
	}
	f0 := uv.FieldVars[0]
	pv := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	want := PointerTo(GetIntType())
	opts := Defaults()
	opts.TakeUnionFieldAddr = false
	// only union field is lower-indirection; bias empty → fall back to any ok
	got := chooseVarFromOK(NewRng(1), want, []*Variable{f0, pv}, opts)
	if got != f0 && got != pv {
		t.Fatalf("unexpected %v", got)
	}
	// with take_union_field_addr on, bias prefers f0 every time
	opts.TakeUnionFieldAddr = true
	for seed := uint64(1); seed < 20; seed++ {
		got = chooseVarFromOK(NewRng(seed), want, []*Variable{f0, pv}, opts)
		if got != f0 {
			t.Fatalf("seed %d: want union field, got %v", seed, got)
		}
	}
}

func TestChooseVarFromOKSingleNoBias(t *testing.T) {
	// size==1 skips bias paths
	iv := CreateVariableScalars("g_i", GetIntType(), false, false)
	got := chooseVarFromOK(NewRng(1), GetIntType(), []*Variable{iv}, Defaults())
	if got != iv {
		t.Fatal(got)
	}
}

func TestChooseVarFromOKIsInsideUnionFieldResidualSticky(t *testing.T) {
	// take_union_field_addr off + Type-nil parent: IsInsideUnionField stickies ERROR.
	// Soft invent was continue past residual then invent addressable bias / later pick.
	// Fair: sticky fail closed whole choose.
	ClearError()
	parent := &Variable{Name: "g_u"} // Type nil
	field := &Variable{Name: "g_u.f0", Type: GetIntType(), FieldVarOf: parent}
	pv := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	want := PointerTo(GetIntType())
	opts := Defaults()
	opts.TakeUnionFieldAddr = false
	got := chooseVarFromOK(NewRng(1), want, []*Variable{field, pv}, opts)
	if got != nil {
		t.Fatalf("Type-nil ancestry residual must fail closed nil, got %v", got)
	}
	if !HasError() {
		t.Fatal("Type-nil ancestry chooseVarFromOK must SetError sticky")
	}
	ClearError()
}
