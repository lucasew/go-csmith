package csmith

import "testing"

func TestFactPointToImplyJoin(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	b := CreateVariableScalars("g_b", GetIntType(), false, false)
	wide := MakeFactPointToSet(p, []*Variable{a, b})
	narrow := MakeFactPointTo(p, a)
	if !wide.Imply(narrow) {
		t.Fatal("wide implies narrow")
	}
	if narrow.Imply(wide) {
		t.Fatal("narrow not imply wide")
	}
	n2 := narrow.Clone()
	if !n2.Join(MakeFactPointTo(p, b)) {
		t.Fatal("join changed")
	}
	if !n2.Imply(wide) || !wide.Imply(n2) {
		// same set
		if len(n2.PointTo) != 2 {
			t.Fatal(len(n2.PointTo))
		}
	}
}

func TestMergeFactLattice(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	b := CreateVariableScalars("g_b", GetIntType(), false, false)
	facts := []*FactPointTo{MakeFactPointTo(p, a)}
	facts = MergeFactInto(facts, MakeFactPointTo(p, b))
	fp := FindRelatedPointTo(facts, p)
	if fp == nil || len(fp.PointTo) != 2 {
		t.Fatalf("%+v", fp)
	}
	// weaker fact no change
	n := len(facts)
	facts = MergeFactInto(facts, MakeFactPointTo(p, a))
	if len(facts) != n {
		t.Fatal("len")
	}
}

func TestMergeFacts(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	q := CreateVariableScalars("g_q", PointerTo(GetIntType()), false, false)
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	facts := []*FactPointTo{MakeFactPointTo(p, a)}
	other := []*FactPointTo{MakeFactPointTo(q, a), MakeFactPointTo(p, CreateVariableScalars("g_b", GetIntType(), false, false))}
	MergeFacts(&facts, other)
	if FindRelatedPointTo(facts, q) == nil {
		t.Fatal("q")
	}
	if len(FindRelatedPointTo(facts, p).PointTo) < 2 {
		t.Fatal("p joined")
	}
	// nil hole fails closed — no invent skip partial join
	hole := []*FactPointTo{MakeFactPointTo(p, a), nil}
	base := []*FactPointTo{MakeFactPointTo(q, a)}
	if MergeFacts(&base, hole) {
		t.Fatal("nil new hole must fail closed")
	}
	if FindRelatedPointTo(base, p) != nil {
		t.Fatal("must not invent partial merge past hole")
	}
	if MergeFactInto(facts, nil) != nil {
		t.Fatal("nil fact MergeFactInto must fail closed")
	}
}

func TestHasEligibleVolatileVar(t *testing.T) {
	v := CreateVariableScalars("g_v", GetIntType(), false, true)
	nv := CreateVariableScalars("g_n", GetIntType(), false, false)
	if !HasEligibleVolatileVar([]*Variable{v, nv}, GetIntType(), AccessRead, EmptyCGContext()) {
		t.Fatal("vol")
	}
	// non-SE-free blocks volatile
	if HasEligibleVolatileVar([]*Variable{v}, GetIntType(), AccessRead, WithEffectContext(WithSideEffects())) {
		t.Fatal("se")
	}
	// nil hole fails closed — no invent skip as absent non-vol
	if HasEligibleVolatileVar([]*Variable{nil, v}, GetIntType(), AccessRead, EmptyCGContext()) {
		t.Fatal("nil hole must fail closed")
	}
}

func TestIfMergesFacts(t *testing.T) {
	opts := Defaults()
	opts.MaxBlockSize = 1
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	f.RV = CreateVariableScalars("f_rv", GetIntType(), false, false)
	fm := NewFactMgr(f)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	b := CreateVariableScalars("g_b", GetIntType(), false, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, a)}
	vs.GlobalList = []*Variable{p, a, b, CreateVariableScalars("g_i", GetIntType(), false, false)}
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	f.Stack = []*Block{{Func: f}}
	// generate if — may or may not change facts; ensure no panic and FM restored/merged
	st := MakeRandomIf(NewRng(11), opts, probs, vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg)
	if st == nil || st.Kind != StmtIfElse {
		t.Fatal("if")
	}
	if FindRelatedPointTo(fm.GlobalFacts, p) == nil {
		t.Fatal("p fact lost")
	}
}
