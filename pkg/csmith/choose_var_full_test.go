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
