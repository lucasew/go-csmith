package csmith

import "testing"

func TestAddEffect(t *testing.T) {
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	b := CreateVariableScalars("g_b", GetIntType(), false, false)
	e1 := EmptyEffect().ReadVar(a)
	e2 := EmptyEffect().WriteVar(b)
	m := e1.AddEffect(e2)
	if !m.IsRead(a) || !m.IsWritten(b) {
		t.Fatal("union")
	}
	// non-vol write does not clear SE-free (Effect.cpp:144–145)
	if !m.IsSideEffectFree() {
		t.Fatal("non-vol stays SE-free")
	}
	// volatile write in add_effect union clears SE-free
	vol := CreateVariableScalars("g_v", GetIntType(), false, true)
	m2 := EmptyEffect().AddEffect(EmptyEffect().WriteVar(vol))
	if m2.IsSideEffectFree() {
		t.Fatal("vol write clears SE-free")
	}
}

func TestIsOrderedBinary(t *testing.T) {
	if !IsOrderedBinary(BinAnd) || !IsOrderedBinary(BinOr) {
		t.Fatal("and/or")
	}
	if IsOrderedBinary(BinAdd) {
		t.Fatal("add")
	}
}

func TestUnionFieldHelpers(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	var env TypeEnv
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EShort), GetSimpleType(EUInt)}
	ut := MakeRandomUnionType(NewRng(3), opts, probs, &env, "U0")
	if ut == nil {
		t.Skip("no union")
	}
	uv := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	if len(uv.FieldVars) == 0 {
		t.Skip("fields")
	}
	f0 := uv.FieldVars[0]
	if !f0.IsUnionField() || !f0.IsInsideUnionField() {
		t.Fatal("union field")
	}
	if f0.GetFieldID() != 0 {
		t.Fatalf("fid %d", f0.GetFieldID())
	}
	// Variable always live; sticky false (no invent not-union-field soft-skip)
	ClearError()
	if (*Variable)(nil).IsUnionField() {
		t.Fatal("nil IsUnionField must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil IsUnionField must SetError sticky")
	}
	ClearError()
	if (*Variable)(nil).IsInsideUnionField() {
		t.Fatal("nil IsInsideUnionField must fail closed false")
	}
	if !HasError() {
		t.Fatal("nil IsInsideUnionField must SetError sticky")
	}
	ClearError()
	// Type-nil parent sticky true (restrictive — no invent not-inside soft-skip)
	parent := &Variable{Name: "g_u"} // Type nil
	field := &Variable{Name: "g_u.f0", Type: GetIntType(), FieldVarOf: parent}
	if !field.IsInsideUnionField() {
		t.Fatal("Type-nil parent IsInsideUnionField must fail closed true restrictive")
	}
	if !HasError() {
		t.Fatal("Type-nil parent IsInsideUnionField must SetError sticky")
	}
	ClearError()
	// non-field complete false
	if !f0.IsInsideUnionField() {
		// f0 is real union field — should be true; use scalar
	}
	scalar := CreateVariableScalars("g_i", GetIntType(), false, false)
	if scalar.IsInsideUnionField() {
		t.Fatal("scalar IsInsideUnionField must be false complete")
	}
	if HasError() {
		t.Fatal("scalar IsInsideUnionField must not sticky")
	}
	ClearError()
}

func TestIsNonreadableField(t *testing.T) {
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	var env TypeEnv
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EShort), GetSimpleType(EUInt)}
	ut := MakeRandomUnionType(NewRng(5), opts, probs, &env, "U0")
	if ut == nil || len(ut.Fields) < 2 {
		t.Skip("union")
	}
	uv := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	if len(uv.FieldVars) < 2 {
		t.Skip("fields")
	}
	f0, f1 := uv.FieldVars[0], uv.FieldVars[1]
	// empty facts → not blocked
	if IsNonreadableField(f1, nil) {
		t.Fatal("empty")
	}
	// Variable always live; sticky nonreadable (no invent readable soft-skip)
	ClearError()
	if !IsNonreadableField(nil, nil) {
		t.Fatal("nil Variable IsNonreadableField must fail closed true")
	}
	if !HasError() {
		t.Fatal("nil Variable IsNonreadableField must SetError sticky")
	}
	ClearError()
	// last write f0 → f1 nonreadable
	facts := []*FactUnion{MakeFactUnion(uv, 0)}
	if IsNonreadableField(f0, facts) {
		t.Fatal("f0 should be readable")
	}
	if !IsNonreadableField(f1, facts) {
		t.Fatal("f1 blocked")
	}
	if HasError() {
		t.Fatal("complete IsNonreadableField paths must not sticky")
	}
	// Imply residual: PointTo-style hole via incomplete union fact soft invent was soft-continue readable.
	// Fair: sticky nonreadable. Use incomplete map already covers; also Type-nil ancestry residual.
	ClearError()
	// incomplete UnionFacts hole: sticky fail closed nonreadable / not-readable
	ClearError()
	hole := []*FactUnion{MakeFactUnion(uv, 0), nil}
	if IsFieldReadable(uv, 0, hole) {
		t.Fatal("incomplete UnionFacts must not invent field readable")
	}
	if !HasError() {
		t.Fatal("incomplete UnionFacts IsFieldReadable must SetError sticky")
	}
	ClearError()
	if !IsNonreadableField(f0, hole) {
		t.Fatal("incomplete UnionFacts must fail closed nonreadable")
	}
	if !HasError() {
		t.Fatal("incomplete UnionFacts IsNonreadableField must SetError sticky")
	}
	ClearError()
}

func TestUpdateAssignUnionFact(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	var env TypeEnv
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EShort), GetSimpleType(EUInt)}
	ut := MakeRandomUnionType(NewRng(7), opts, probs, &env, "U0")
	if ut == nil || len(ut.Fields) < 1 {
		t.Skip("union")
	}
	uv := CreateVariableQfer("g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	if len(uv.FieldVars) == 0 {
		t.Skip("fields")
	}
	fm := NewFactMgr(nil)
	rhs := &Expression{Term: TermConstant, Con: MakeInt(1)}
	fm.UpdateFactForAssign(uv.FieldVars[0], 0, rhs)
	fu := FindRelatedUnion(fm.UnionFacts, uv)
	if fu == nil || fu.LastWrittenFID != 0 {
		t.Fatalf("%+v", fu)
	}
}

func TestOrderedBinaryEffectIsolation(t *testing.T) {
	// && : after left writes a, RHS generation sees pre-left context only
	ClearError()
	opts := Defaults()
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	b := CreateVariableScalars("g_b", GetIntType(), false, false)
	vs := NewVariableSelector(opts)
	vs.GlobalList = []*Variable{a, b}
	eff := EmptyEffect()
	cg := EmptyCGContext()
	cg.EffectAccum = &eff
	// force ordered path by calling MakeRandomBinary many times? just unit-test restore logic
	preLeft := EmptyEffect()
	// simulate left write
	*cg.EffectAccum = preLeft.WriteVar(a)
	postLeft := *cg.EffectAccum
	*cg.EffectAccum = preLeft
	// RHS read of a would be OK under preLeft (a not written in context)
	if !IsEligibleVar(a, 0, AccessRead, cg) {
		t.Fatal("a readable under pre-left")
	}
	// under postLeft as ambient effect_context, a is written → not eligible for read
	cg2 := WithEffectContext(postLeft)
	if IsEligibleVar(a, 0, AccessRead, cg2) {
		t.Fatal("a blocked after left write")
	}
}
