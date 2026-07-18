package csmith

import "testing"

func TestOpportunisticValidateNoDeref(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	if OpportunisticValidate(NewRng(1), v, GetIntType(), nil, 0, 0) != 1 {
		t.Fatal("same level")
	}
}

func TestOpportunisticValidateNullDead(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	// no fact → 0
	if OpportunisticValidate(NewRng(1), p, GetIntType(), nil, 0, 0) != 0 {
		t.Fatal("no fact")
	}
	// null, prob 0 → 0
	facts := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	if OpportunisticValidate(NewRng(1), p, GetIntType(), facts, 0, 0) != 0 {
		t.Fatal("null blocked")
	}
	// live target → 1
	tgt := CreateVariableScalars("g_t", GetIntType(), false, false)
	facts = []*FactPointTo{MakeFactPointTo(p, tgt)}
	if OpportunisticValidate(NewRng(1), p, GetIntType(), facts, 0, 0) != 1 {
		t.Fatal("live")
	}
	// garbage, prob 0 → 0
	facts = []*FactPointTo{NewFactPointTo(p)}
	if OpportunisticValidate(NewRng(1), p, GetIntType(), facts, 0, 0) != 0 {
		t.Fatal("dead blocked")
	}
}

func TestVariableCompatible(t *testing.T) {
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	b := CreateVariableScalars("g_b", GetIntType(), false, false)
	if !a.Compatible(a, false) {
		t.Fatal("self")
	}
	if a.Compatible(b, false) {
		t.Fatal("other no expand")
	}
	if !a.Compatible(b, true) {
		t.Fatal("expand non-field")
	}
	vol := CreateVariableScalars("g_v", GetIntType(), false, true)
	if vol.Compatible(vol, false) {
		t.Fatal("vol self")
	}
}

func TestCompatibleCheckerDisabled(t *testing.T) {
	opts := Defaults()
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	e := &Expression{Term: TermVariable, Var: a}
	if CompatibleCheckExprVar(opts, a, e) {
		t.Fatal("disabled")
	}
	opts.CompatibleCheck = true
	if !CompatibleCheckExprVar(opts, a, e) {
		t.Fatal("self assign rejected")
	}
}

func TestHasDereferenceableVar(t *testing.T) {
	opts := Defaults()
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	tgt := CreateVariableScalars("g_t", GetIntType(), false, false)
	fm := NewFactMgr(nil)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, tgt)}
	cg := EmptyCGContext().WithFactMgr(fm)
	if !HasDereferenceableVar([]*Variable{p}, GetIntType(), cg, opts) {
		t.Fatal("valid ptr")
	}
	// garbage not valid
	fm.GlobalFacts = []*FactPointTo{NewFactPointTo(p)}
	if HasDereferenceableVar([]*Variable{p}, GetIntType(), cg, opts) {
		t.Fatal("dead")
	}
}

func TestIsPartialVolatileAfterDeref(t *testing.T) {
	// pointer to volatile struct/union field type
	opts := Defaults()
	probs := NewProbabilities(opts)
	var env TypeEnv
	st := MakeRandomStructType(NewRng(2), opts, probs, &env, "S0")
	// force a volatile field if possible — check method on non-vol struct pointer
	pt := PointerTo(st)
	v := CreateVariableQfer("g_p", pt, NewCVQualifiers([]bool{false}, []bool{false}))
	// if struct has no vol fields, partial is false
	_ = v.IsPartialVolatileAfterDeref(1)
	// fully volatile pointer qfer → not partial
	vv := CreateVariableQfer("g_pv", pt, NewCVQualifiers([]bool{false, false}, []bool{true, false}))
	// IsVolatileAfterDeref(1) depends on qfer layout — just ensure no panic
	_ = vv.IsPartialVolatileAfterDeref(1)
}

func TestMakeRandomAssignCompatibleRegen(t *testing.T) {
	opts := Defaults()
	opts.CompatibleCheck = true
	vs := NewVariableSelector(opts)
	q := NewCVQualifiers([]bool{false}, []bool{false})
	g := vs.GenerateNewGlobal(AccessWrite, EmptyCGContext(), GetIntType(), &q, NewRng(2))
	if g == nil {
		t.Fatal("g")
	}
	// generate assigns — should not panic
	for seed := uint64(1); seed < 10; seed++ {
		st := MakeRandomAssign(NewRng(seed), opts, NewProbabilities(opts), vs, NewExprTables(opts), EmptyCGContext(), GetIntType())
		if st.Kind != StmtAssign {
			t.Fatal(st.Kind)
		}
	}
}
