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
	fm := NewFactMgr(nil)
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
}

func TestChooseOKVarItemizesArray(t *testing.T) {
	opts := Defaults()
	av := CreateArrayVariable(NewRng(2), opts, NewProbabilities(opts), nil, "g_a", GetIntType(), MakeInt(0), NewCVQualifiers([]bool{false}, []bool{false}))
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
	env := &TypeEnv{}
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
	var env TypeEnv
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
