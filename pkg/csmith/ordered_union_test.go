package csmith

import "testing"

func TestAddEffect(t *testing.T) {
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntType(), false, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntType(), false, false)
	e1 := EmptyEffect().ReadVarSess(testAmbientSession, a)
	e2 := EmptyEffect().WriteVarSess(testAmbientSession, b)
	m := e1.AddEffectSess(testAmbientSession, e2)
	if !m.IsReadSess(testAmbientSession, a) || !m.IsWrittenSess(testAmbientSession, b) {
		t.Fatal("union")
	}
	// non-vol write does not clear SE-free (Effect.cpp:144–145)
	if !m.IsSideEffectFreeSess(testAmbientSession) {
		t.Fatal("non-vol stays SE-free")
	}
	// volatile write in add_effect union clears SE-free
	vol := CreateVariableScalarsSess(testAmbientSession, "g_v", GetIntType(), false, true)
	m2 := EmptyEffect().AddEffectSess(testAmbientSession, EmptyEffect().WriteVarSess(testAmbientSession, vol))
	if m2.IsSideEffectFreeSess(testAmbientSession) {
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
	env := TypeEnv{Sess: testAmbientSession}
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EShort), GetSimpleType(EUInt)}
	ut := MakeRandomUnionType(NewRng(3), opts, probs, &env, "U0")
	if ut == nil {
		t.Skip("no union")
	}
	uv := CreateVariableQferSess(testAmbientSession, "g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	if len(uv.FieldVars) == 0 {
		t.Skip("fields")
	}
	f0 := uv.FieldVars[0]
	if !f0.IsUnionFieldSess(testAmbientSession) || !f0.IsInsideUnionFieldSess(testAmbientSession) {
		t.Fatal("union field")
	}
	if f0.GetFieldIDSess(testAmbientSession) != 0 {
		t.Fatalf("fid %d", f0.GetFieldIDSess(testAmbientSession))
	}
	// Variable always live; sticky false (no invent not-union-field soft-skip)
	ClearErrorSess(testAmbientSession)
	if (*Variable)(nil).IsUnionFieldSess(testAmbientSession) {
		t.Fatal("nil IsUnionField must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil IsUnionField must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*Variable)(nil).IsInsideUnionFieldSess(testAmbientSession) {
		t.Fatal("nil IsInsideUnionField must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil IsInsideUnionField must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Type-nil parent sticky true (restrictive — no invent not-inside soft-skip)
	parent := &Variable{Name: "g_u"} // Type nil
	field := &Variable{Name: "g_u.f0", Type: GetIntType(), FieldVarOf: parent}
	if !field.IsInsideUnionFieldSess(testAmbientSession) {
		t.Fatal("Type-nil parent IsInsideUnionField must fail closed true restrictive")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Type-nil parent IsInsideUnionField must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// non-field complete false
	if !f0.IsInsideUnionFieldSess(testAmbientSession) {
		// f0 is real union field — should be true; use scalar
	}
	scalar := CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntType(), false, false)
	if scalar.IsInsideUnionFieldSess(testAmbientSession) {
		t.Fatal("scalar IsInsideUnionField must be false complete")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("scalar IsInsideUnionField must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIsNonreadableField(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := TypeEnv{Sess: testAmbientSession}
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EShort), GetSimpleType(EUInt)}
	ut := MakeRandomUnionType(NewRng(5), opts, probs, &env, "U0")
	if ut == nil || len(ut.Fields) < 2 {
		t.Skip("union")
	}
	uv := CreateVariableQferSess(testAmbientSession, "g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	if len(uv.FieldVars) < 2 {
		t.Skip("fields")
	}
	f0, f1 := uv.FieldVars[0], uv.FieldVars[1]
	// FactUnion.cpp:188–189 — fu == nullptr → nonreadable (empty complete map)
	if !IsNonreadableField(f1, nil) {
		t.Fatal("empty complete facts: no related FactUnion → nonreadable")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("empty complete path must not sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Variable always live; sticky nonreadable (no invent readable soft-skip)
	ClearErrorSess(testAmbientSession)
	if !IsNonreadableField(nil, nil) {
		t.Fatal("nil Variable IsNonreadableField must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Variable IsNonreadableField must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// last write f0 → f1 nonreadable
	facts := []*FactUnion{MakeFactUnion(uv, 0)}
	if IsNonreadableField(f0, facts) {
		t.Fatal("f0 should be readable")
	}
	if !IsNonreadableField(f1, facts) {
		t.Fatal("f1 blocked")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete IsNonreadableField paths must not sticky")
	}
	// Imply residual: PointTo-style hole via incomplete union fact soft invent was soft-continue readable.
	// Fair: sticky nonreadable. Use incomplete map already covers; also Type-nil ancestry residual.
	ClearErrorSess(testAmbientSession)
	// incomplete UnionFacts hole: sticky fail closed nonreadable / not-readable
	ClearErrorSess(testAmbientSession)
	hole := []*FactUnion{MakeFactUnion(uv, 0), nil}
	if IsFieldReadable(uv, 0, hole) {
		t.Fatal("incomplete UnionFacts must not invent field readable")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete UnionFacts IsFieldReadable must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !IsNonreadableField(f0, hole) {
		t.Fatal("incomplete UnionFacts must fail closed nonreadable")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete UnionFacts IsNonreadableField must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestUpdateAssignUnionFact(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := TypeEnv{Sess: testAmbientSession}
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EShort), GetSimpleType(EUInt)}
	ut := MakeRandomUnionType(NewRng(7), opts, probs, &env, "U0")
	if ut == nil || len(ut.Fields) < 1 {
		t.Skip("union")
	}
	uv := CreateVariableQferSess(testAmbientSession, "g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	if len(uv.FieldVars) == 0 {
		t.Skip("fields")
	}
	fm := NewFactMgrSess(testAmbientSession, nil)
	rhs := &Expression{Term: TermConstant, Con: MakeInt(1)}
	fm.UpdateFactForAssign(uv.FieldVars[0], 0, rhs)
	fu := FindRelatedUnion(fm.UnionFacts, uv)
	if fu == nil || fu.LastWrittenFID != 0 {
		t.Fatalf("%+v", fu)
	}
}

func TestOrderedBinaryEffectIsolation(t *testing.T) {
	// && : after left writes a, RHS generation sees pre-left context only
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntType(), false, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntType(), false, false)
	vs := NewVariableSelector(opts)
	vs.GlobalList = []*Variable{a, b}
	eff := EmptyEffect()
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.EffectAccum = &eff
	// force ordered path by calling MakeRandomBinary many times? just unit-test restore logic
	preLeft := EmptyEffect()
	// simulate left write
	*cg.EffectAccum = preLeft.WriteVarSess(testAmbientSession, a)
	postLeft := *cg.EffectAccum
	*cg.EffectAccum = preLeft
	// RHS read of a would be OK under preLeft (a not written in context)
	if !IsEligibleVar(a, 0, AccessRead, cg) {
		t.Fatal("a readable under pre-left")
	}
	// under postLeft as ambient effect_context, a is written → not eligible for read
	cg2 := WithEffectContext(postLeft).WithSession(testAmbientSession)
	if IsEligibleVar(a, 0, AccessRead, cg2) {
		t.Fatal("a blocked after left write")
	}
}
