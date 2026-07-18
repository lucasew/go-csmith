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
	// non-convertible pointer won't match int under Flexible
	b := CreateVariableQfer("g_2", PointerTo(GetSimpleType(EInt)), q)
	vs.GlobalList = []*Variable{a, b}
	r := NewRng(2)
	got := vs.SelectGlobal(AccessRead, EmptyCGContext(), GetSimpleType(EInt), &q, r)
	// eFlexible: int matches a; *int is not convertible to int without deref path
	// is_derivable: ptr_type==int for *int? this is int*, ptr_type is int, is_derivable(int)
	// from *int: this==t false; is_convertable(*int) false; is_dereferenced_from(*int) true (int from *int)
	// Wait — match is want.Match(var.Type): int.Match(*int, Flexible) = int.is_derivable(*int)
	// is_derivable(*int): this==t no; is_convertable no; is_dereferenced_from(*int) yes (int is deref of *int)
	// So Flexible actually matches *int as source for int! That's eDereference-like via is_derivable.
	// Upstream may then emit *g_2 via ExpressionVariable. Our SelectGlobal returns the var.
	if got != a && got != b {
		t.Fatalf("should pick existing, got %v", got)
	}
	// only matching exact non-pointer when we use two ints
	c := CreateVariableQfer("g_3", GetSimpleType(EInt), q)
	vs.GlobalList = []*Variable{a, c}
	got = vs.SelectGlobal(AccessRead, EmptyCGContext(), GetSimpleType(EInt), &q, r)
	if got != a && got != c {
		t.Fatalf("want one of int globals, got %v", got)
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
	// nil qfer → random_qualifiers (2 flips) + Constant::make_random (more draws).
	opts := Defaults()
	vs := NewVariableSelector(opts)
	r := NewRng(2)
	v := vs.GenerateNewGlobal(AccessRead, EmptyCGContext(), GetSimpleType(EInt), nil, r)
	if v == nil || v.Init == nil || v.Init.Value == "" {
		t.Fatalf("init missing: %+v", v)
	}
	if r.RandDepth() < 2 {
		t.Fatalf("expected qfer+const RNG, depth=%d", r.RandDepth())
	}
}

func TestGenerateNewGlobalFixedQferHasInit(t *testing.T) {
	// Scalar create_and_initialize path (not array itemize): make_init_value applied.
	opts := Defaults()
	opts.Arrays = false
	vs := NewVariableSelector(opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	r := NewRng(2)
	v := vs.GenerateNewGlobal(AccessRead, EmptyCGContext(), GetSimpleType(EInt), &q, r)
	if v == nil || (v.Init == nil && v.InitExpr == nil) {
		t.Fatal("MakeInitValue init")
	}
	// pointer qfer must be depth 2 (indirect_level+1); make_init asserts sanity_check
	qPtr := NewCVQualifiers([]bool{false, false}, []bool{false, false})
	// pointer: make_init_value → Constant "0" (20%) or &visible Expression
	vp := vs.GenerateNewGlobal(AccessRead, EmptyCGContext(), PointerTo(GetSimpleType(EInt)), &qPtr, r)
	if vp == nil || (vp.Init == nil && vp.InitExpr == nil) {
		t.Fatalf("pointer init: Init=%+v InitExpr=%+v", vp.Init, vp.InitExpr)
	}
	if vp.Init != nil && vp.Init.Value != "0" {
		t.Fatalf("pointer constant init want 0 got %+v", vp.Init)
	}
}
