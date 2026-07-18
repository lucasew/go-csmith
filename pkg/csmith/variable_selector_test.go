package csmith

import "testing"

func TestChooseOKVarSoleAndUpto(t *testing.T) {
	// VariableSelector::choose_ok_var
	a := CreateVariableScalars("g_1", GetSimpleType(EInt), false, false)
	b := CreateVariableScalars("g_2", GetSimpleType(EInt), false, false)
	c := CreateVariableScalars("g_3", GetSimpleType(EInt), false, false)

	if ChooseOKVar(NewRng(2), nil) != nil {
		t.Fatal("empty")
	}
	if ChooseOKVar(NewRng(2), []*Variable{a}) != a {
		t.Fatal("sole")
	}
	// seed2 first RndUpto(3) = 1959434203 % 3
	r := NewRng(2)
	wantIdx := int(r.RndUpto(3))
	r2 := NewRng(2)
	got := ChooseOKVar(r2, []*Variable{a, b, c})
	want := []*Variable{a, b, c}[wantIdx]
	if got != want {
		t.Fatalf("choose_ok_var: got %v want %v (idx %d)", got.Name, want.Name, wantIdx)
	}
}

func TestGenerateNewGlobalNamesAndList(t *testing.T) {
	// GenerateNewGlobal: gensym g_1, push GlobalList, random_qualifiers draws.
	opts := Defaults()
	vs := NewVariableSelector(opts)
	r := NewRng(2)
	tInt := GetSimpleType(EInt)
	// Fixed qfer — no RNG for quals
	q := NewCVQualifiers([]bool{false}, []bool{false})
	v := vs.GenerateNewGlobal(AccessRead, EmptyCGContext(), tInt, &q, r)
	if v == nil || v.Name != "g_1" {
		t.Fatalf("name: %+v", v)
	}
	if len(vs.GlobalList) != 1 || vs.GlobalList[0] != v {
		t.Fatal("GlobalList")
	}
	if !vs.VarCreated || vs.TmpCount != 1 {
		t.Fatal("flags")
	}
	v2 := vs.GenerateNewGlobal(AccessRead, EmptyCGContext(), tInt, &q, r)
	if v2.Name != "g_2" {
		t.Fatalf("second name %s", v2.Name)
	}
}

func TestSelectGlobalEmptyCreates(t *testing.T) {
	// SelectGlobal empty → GenerateNewGlobal
	opts := Defaults()
	vs := NewVariableSelector(opts)
	r := NewRng(2)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	v := vs.SelectGlobal(AccessRead, EmptyCGContext(), GetSimpleType(EInt), &q, r)
	if v == nil || v.Name != "g_1" || len(vs.GlobalList) != 1 {
		t.Fatalf("create on empty: %+v list=%d", v, len(vs.GlobalList))
	}
}

func TestSelectGlobalChoosesExisting(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	a := CreateVariableQfer("g_1", GetSimpleType(EInt), q)
	b := CreateVariableQfer("g_2", GetSimpleType(EShort), q)
	vs.GlobalList = []*Variable{a, b}
	// Want int → only a
	r := NewRng(2)
	got := vs.SelectGlobal(AccessRead, EmptyCGContext(), GetSimpleType(EInt), &q, r)
	if got != a {
		t.Fatalf("exact int: got %v", got)
	}
	// Want short → only b (sole, no upto)
	got = vs.SelectGlobal(AccessRead, EmptyCGContext(), GetSimpleType(EShort), &q, r)
	if got != b {
		t.Fatalf("exact short: got %v", got)
	}
}

func TestSelectGlobalMultiMatchUpto(t *testing.T) {
	opts := Defaults()
	vs := NewVariableSelector(opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	a := CreateVariableQfer("g_1", GetSimpleType(EInt), q)
	b := CreateVariableQfer("g_2", GetSimpleType(EInt), q)
	vs.GlobalList = []*Variable{a, b}
	r := NewRng(2)
	// First upto(2) = 1959434203 % 2
	rProbe := NewRng(2)
	idx := int(rProbe.RndUpto(2))
	got := vs.SelectGlobal(AccessRead, EmptyCGContext(), GetSimpleType(EInt), &q, r)
	want := []*Variable{a, b}[idx]
	if got != want {
		t.Fatalf("got %s want %s", got.Name, want.Name)
	}
}

func TestGenerateNewGlobalRandomQferConsumesRNG(t *testing.T) {
	// Wildcard/nil qfer → random_qualifiers for simple int: 2 flipcoins (vol, const) when SE-free READ.
	opts := Defaults()
	vs := NewVariableSelector(opts)
	r := NewRng(2)
	_ = vs.GenerateNewGlobal(AccessRead, EmptyCGContext(), GetSimpleType(EInt), nil, r)
	// After two flipcoins for storage, depth should be 2
	if r.RandDepth() != 2 {
		t.Fatalf("rand_depth after simple random_qualifiers: %d want 2", r.RandDepth())
	}
}
