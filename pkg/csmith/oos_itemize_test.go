package csmith

import "testing"

func TestMarkDeadVar(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	tgt := CreateVariableScalars("g_t", GetIntType(), false, false)
	f := MakeFactPointTo(p, tgt)
	nf := f.MarkDeadVar(tgt)
	if nf == nil || !nf.IsDead() {
		t.Fatal("dead")
	}
	// already garbage + another — remove
	f2 := MakeFactPointToSet(p, []*Variable{tgt, GarbagePtr})
	nf2 := f2.MarkDeadVar(tgt)
	if nf2 == nil || len(nf2.PointTo) != 1 || nf2.PointTo[0] != GarbagePtr {
		t.Fatalf("%+v", nf2)
	}
	if f.MarkDeadVar(CreateVariableScalars("g_x", GetIntType(), false, false)) != nil {
		t.Fatal("unrelated")
	}
}

func TestUpdateFactsForOOSVars(t *testing.T) {
	fm := NewFactMgrSess(testAmbientSession, nil)
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	// fact for local pointer removed when oos
	lp := CreateVariableScalars("l_p", PointerTo(GetIntType()), false, false)
	fm.GlobalFacts = []*FactPointTo{
		MakeFactPointTo(lp, loc),
		MakeFactPointTo(p, loc),
	}
	fm.UpdateFactsForOOSVars([]*Variable{lp, loc})
	// lp fact gone
	if FindRelatedPointTo(fm.GlobalFacts, lp) != nil {
		t.Fatal("lp fact should drop")
	}
	// p should now point garbage (loc dead)
	fp := FindRelatedPointTo(fm.GlobalFacts, p)
	if fp == nil || !fp.IsDead() {
		t.Fatalf("p fact %+v", fp)
	}
	// nil fact hole fails closed sticky
	ClearErrorSess(testAmbientSession)
	fm2 := NewFactMgrSess(testAmbientSession, nil)
	fm2.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	fm2.UpdateFactsForOOSVars([]*Variable{loc})
	if FactsComplete(fm2.GlobalFacts) {
		t.Fatal("nil fact hole must fail closed", fm2.GlobalFacts)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil fact hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestChooseOKVarItemizesArray(t *testing.T) {
	opts := Defaults()
	av := CreateArrayVariable(NewRng(2), opts, NewProbabilities(opts), nil, nil, nil, "g_a", GetIntType(), MakeInt(0), NewCVQualifiers([]bool{false}, []bool{false}))
	if av == nil || av.AsArray == nil {
		t.Fatal("av")
	}
	// sole array → itemize consumes per-dim RNG
	r := NewRng(5)
	got := ChooseOKVar(r, []*Variable{&av.Variable})
	if got == nil || got.AsArray == nil || got.AsArray.Collective != av {
		t.Fatalf("itemize %+v", got)
	}
	if len(got.AsArray.Indices) == 0 {
		t.Fatal("indices")
	}
}

func TestRandomTypeFromTypeNoVolatile(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := &TypeEnv{Sess: testAmbientSession}
	GenerateAllTypesEnv(NewRng(2), opts, probs, env)
	// should not panic; with noVolatile prefer nonvol path
	ty := RandomTypeFromType(NewRng(3), env, opts, probs, nil, true, false)
	if ty == nil {
		t.Fatal("nil")
	}
}

func TestVariableMatchAggregate(t *testing.T) {
	opts := Defaults()
	probs := NewProbabilities(opts)
	env := TypeEnv{Sess: testAmbientSession}
	env.AllTypes = []*Type{GetIntType(), GetSimpleType(EShort), GetSimpleType(EUInt)}
	st := MakeRandomStructType(NewRng(2), opts, probs, &env, "S0")
	sv := CreateVariableQfer("g_s", st, NewCVQualifiers([]bool{false}, []bool{false}))
	if len(sv.FieldVars) == 0 {
		t.Skip("no fields")
	}
	if !sv.Match(sv.FieldVars[0]) {
		t.Fatal("field match")
	}
}

func TestMarkDeadVarNilSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if (*FactPointTo)(nil).MarkDeadVar(CreateVariableScalars("g_x", GetIntType(), false, false)) != nil {
		t.Fatal("nil Fact MarkDeadVar must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Fact MarkDeadVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	tgt := CreateVariableScalars("g_t", GetIntType(), false, false)
	f := MakeFactPointTo(p, tgt)
	if f.MarkDeadVar(nil) != nil {
		t.Fatal("nil var MarkDeadVar must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil var MarkDeadVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMarkDeadVarStructFieldPointee(t *testing.T) {
	// FactPointTo.cpp:108–124 + find_field_variable_in_set: OOS aggregate marks
	// field pointees (l_531.f0) as garbage — seed-30 g_113 held l_531.f0 live.
	ClearErrorSess(testAmbientSession)
	st := &Type{isStruct: true, StructName: "S", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	agg := CreateVariableScalars("l_531", st, false, false)
	agg.CreateFieldVars()
	if len(agg.FieldVars) == 0 {
		t.Fatal("need field")
	}
	fld := agg.FieldVars[0]
	p := CreateVariableScalars("g_113", PointerTo(GetIntType()), false, false)
	f := MakeFactPointToSet(p, []*Variable{fld, NullPtr})
	nf := f.MarkDeadVar(agg)
	if nf == nil {
		t.Fatal("MarkDeadVar aggregate must hit field pointee")
	}
	if !nf.IsDead() {
		t.Fatalf("want garbage for field, got %+v", nf.PointTo)
	}
	// OOS path
	facts := []*FactPointTo{MakeFactPointToSet(p, []*Variable{fld, NullPtr})}
	UpdateFactsForOOSVars([]*Variable{agg}, &facts)
	fp := FindRelatedPointTo(facts, p)
	if fp == nil || !fp.IsDead() {
		t.Fatalf("OOS aggregate must garbage field pointee, got %+v", fp)
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete path must not sticky")
	}
}

func TestMarkFuncEndLocalsStructFieldPointee(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	st := &Type{isStruct: true, StructName: "S", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	agg := CreateVariableScalars("l_531", st, false, false)
	agg.CreateFieldVars()
	fld := agg.FieldVars[0]
	p := CreateVariableScalars("g_113", PointerTo(GetIntType()), false, false)
	facts := []*FactPointTo{MakeFactPointToSet(p, []*Variable{fld, NullPtr})}
	nf := facts[0].MarkFuncEndLocals([]*Variable{agg})
	if nf == nil || !nf.IsDead() {
		t.Fatalf("MarkFuncEndLocals must garbage field pointee via CollectExpandable, got %+v", nf)
	}
}
