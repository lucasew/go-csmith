package csmith

import (
	"strings"
	"testing"
)

func TestChooseVarFullInvalidVars(t *testing.T) {
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), false, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntTypeSess(testAmbientSession), false, false)
	// invalid_vars contains a → only b
	got := ChooseVarFull(NewRngSess(testAmbientSession, 2), []*Variable{a, b}, AccessRead, EmptyCGContext().WithSession(testAmbientSession),
		GetIntTypeSess(testAmbientSession), nil, MatchFlexible, []*Variable{a}, false, false, false)
	if got != b {
		t.Fatalf("got %v want b", got)
	}
}

func TestChooseVarFullNoBitfield(t *testing.T) {
	bf := CreateVariableScalarsSess(testAmbientSession, "g_bf", GetIntTypeSess(testAmbientSession), false, false)
	bf.IsBitfield = true
	ok := CreateVariableScalarsSess(testAmbientSession, "g_ok", GetIntTypeSess(testAmbientSession), false, false)
	got := ChooseVarFull(NewRngSess(testAmbientSession, 3), []*Variable{bf, ok}, AccessRead, EmptyCGContext().WithSession(testAmbientSession),
		GetIntTypeSess(testAmbientSession), nil, MatchFlexible, nil, true, false, false)
	if got != ok {
		t.Fatalf("got %v want ok", got)
	}
}

func TestChooseVarFullNoUnion(t *testing.T) {
	ut := &Type{
		isUnion:    true,
		StructName: "U0",
		Fields: []StructField{
			{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
			{Name: "f1", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
		},
	}
	uv := CreateVariableQferSess(testAmbientSession, "g_u", ut, NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false}))
	if len(uv.FieldVars) < 1 {
		t.Fatal("need fields")
	}
	plain := CreateVariableScalarsSess(testAmbientSession, "g_p", GetIntTypeSess(testAmbientSession), false, false)
	// expand would surface union fields; noUnion must reject them
	got := ChooseVarFull(NewRngSess(testAmbientSession, 5), []*Variable{uv, plain}, AccessRead, EmptyCGContext().WithSession(testAmbientSession),
		GetIntTypeSess(testAmbientSession), nil, MatchFlexible, nil, false, false, true)
	if got != plain {
		t.Fatalf("got %v want plain", got)
	}
}

func TestChooseVarFullNoExpandKeepsStruct(t *testing.T) {
	st := &Type{
		isStruct:   true,
		StructName: "S0",
		Fields: []StructField{
			{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
		},
	}
	sv := CreateVariableQferSess(testAmbientSession, "g_s", st, NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false}))
	// no expand + want int → struct itself does not match int
	got := ChooseVarFull(NewRngSess(testAmbientSession, 1), []*Variable{sv}, AccessRead, EmptyCGContext().WithSession(testAmbientSession),
		GetIntTypeSess(testAmbientSession), nil, MatchFlexible, nil, false, true, false)
	if got != nil {
		t.Fatalf("want nil without expand, got %v", got)
	}
	// with expand (default) → field selected
	got2 := ChooseVarFull(NewRngSess(testAmbientSession, 1), []*Variable{sv}, AccessRead, EmptyCGContext().WithSession(testAmbientSession),
		GetIntTypeSess(testAmbientSession), nil, MatchFlexible, nil, false, false, false)
	if got2 == nil || got2 != sv.FieldVars[0] {
		t.Fatalf("want field, got %v", got2)
	}
}

func TestChooseVarFullAmbientResidualSticky(t *testing.T) {
	// Ambient residual ERROR: HasEligibleVolatileVarQfer may soft-return false while residual sticks.
	// Soft invent was soft-continue choose then pick later good. Fair: sticky fail closed whole choose.
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), false, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntTypeSess(testAmbientSession), false, false)
	SetErrorSess(testAmbientSession, ErrGeneric)
	if ChooseVarFull(NewRngSess(testAmbientSession, 2), []*Variable{a, b}, AccessRead, EmptyCGContext().WithSession(testAmbientSession),
		GetIntTypeSess(testAmbientSession), nil, MatchFlexible, nil, false, false, false) != nil {
		t.Fatal("ambient residual must fail closed ChooseVarFull, not invent later pick")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("ambient residual ChooseVarFull must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestHashOutputWithUnionFactsSkipsUnread(t *testing.T) {
	ut := &Type{
		isUnion:    true,
		StructName: "U0",
		Fields: []StructField{
			{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
			{Name: "f1", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
		},
	}
	uv := CreateVariableQferSess(testAmbientSession, "g_u", ut, NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false}))
	// nil facts → hash all (HashOutput path)
	all := uv.HashOutputSess(testAmbientSession)
	if !strings.Contains(all, "g_u.f0") || !strings.Contains(all, "g_u.f1") {
		t.Fatal(all)
	}
	// last write f0 only
	facts := []*FactUnion{MakeFactUnionSess(testAmbientSession, uv, 0)}
	out := uv.HashOutputWithUnionFactsSess(testAmbientSession, facts)
	if !strings.Contains(out, "g_u.f0") {
		t.Fatal("want f0", out)
	}
	if strings.Contains(out, "g_u.f1") {
		t.Fatal("must skip f1", out)
	}
	// incomplete UnionFacts residual: soft invent was soft-skip unreadable then partial hash.
	// Fair: sticky fail closed empty whole hash.
	ClearErrorSess(testAmbientSession)
	if s := uv.HashOutputWithUnionFactsSess(testAmbientSession, IncompleteUnionFactSlice()); s != "" {
		t.Fatal("incomplete UnionFacts HashOutput must fail closed empty", s)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete UnionFacts HashOutput must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRecordPointerAvailForDeref(t *testing.T) {
	BookkeeperDoFinalizationSess(testAmbientSession)
	before := currentSession().BK.pointerAvailForDeref
	RecordPointerAvailForDerefSess(testAmbientSession)
	if currentSession().BK.pointerAvailForDeref != before+1 {
		t.Fatalf("got %d want %d", currentSession().BK.pointerAvailForDeref, before+1)
	}
}

func TestChooseVarFromOKPreferDeref(t *testing.T) {
	// VariableSelector.cpp:459–471 — among ok with size>1, prefer higher indirection
	iv := CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntTypeSess(testAmbientSession), false, false)
	pv := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	opts := Defaults()
	// multiple seeds: always prefer pointer when both match want int
	for seed := uint64(1); seed < 20; seed++ {
		got := chooseVarFromOKSess(testAmbientSession, NewRngSess(testAmbientSession, seed), GetIntTypeSess(testAmbientSession), []*Variable{iv, pv}, opts)
		if got != pv {
			t.Fatalf("seed %d: got %v want ptr", seed, got)
		}
	}
}

func TestChooseVarFromOKPreferAddressOf(t *testing.T) {
	// VariableSelector.cpp:484–514 — want pointer, prefer lower-indirection (take address)
	iv := CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntTypeSess(testAmbientSession), false, false)
	pv := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	want := PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))
	opts := Defaults()
	for seed := uint64(1); seed < 20; seed++ {
		got := chooseVarFromOKSess(testAmbientSession, NewRngSess(testAmbientSession, seed), want, []*Variable{iv, pv}, opts)
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
			{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
		},
	}
	uv := CreateVariableQferSess(testAmbientSession, "g_u", ut, NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false}))
	if len(uv.FieldVars) < 1 {
		t.Fatal("fields")
	}
	f0 := uv.FieldVars[0]
	pv := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	want := PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))
	opts := Defaults()
	opts.TakeUnionFieldAddr = false
	// only union field is lower-indirection; bias empty → fall back to any ok
	got := chooseVarFromOKSess(testAmbientSession, NewRngSess(testAmbientSession, 1), want, []*Variable{f0, pv}, opts)
	if got != f0 && got != pv {
		t.Fatalf("unexpected %v", got)
	}
	// with take_union_field_addr on, bias prefers f0 every time
	opts.TakeUnionFieldAddr = true
	for seed := uint64(1); seed < 20; seed++ {
		got = chooseVarFromOKSess(testAmbientSession, NewRngSess(testAmbientSession, seed), want, []*Variable{f0, pv}, opts)
		if got != f0 {
			t.Fatalf("seed %d: want union field, got %v", seed, got)
		}
	}
}

func TestChooseVarFromOKSingleNoBias(t *testing.T) {
	// size==1 skips bias paths
	iv := CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntTypeSess(testAmbientSession), false, false)
	got := chooseVarFromOKSess(testAmbientSession, NewRngSess(testAmbientSession, 1), GetIntTypeSess(testAmbientSession), []*Variable{iv}, Defaults())
	if got != iv {
		t.Fatal(got)
	}
}

func TestChooseVarFromOKIsInsideUnionFieldResidualSticky(t *testing.T) {
	// take_union_field_addr off + Type-nil parent: IsInsideUnionField stickies ERROR.
	// Soft invent was continue past residual then invent addressable bias / later pick.
	// Fair: sticky fail closed whole choose.
	ClearErrorSess(testAmbientSession)
	parent := &Variable{Name: "g_u"} // Type nil
	field := &Variable{Name: "g_u.f0", Type: GetIntTypeSess(testAmbientSession), FieldVarOf: parent}
	pv := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	want := PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))
	opts := Defaults()
	opts.TakeUnionFieldAddr = false
	got := chooseVarFromOKSess(testAmbientSession, NewRngSess(testAmbientSession, 1), want, []*Variable{field, pv}, opts)
	if got != nil {
		t.Fatalf("Type-nil ancestry residual must fail closed nil, got %v", got)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil ancestry chooseVarFromOK must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}
