package csmith

import "testing"

func TestFactPointToImplyJoin(t *testing.T) {
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerTo(GetIntType()), false, false)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntType(), false, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntType(), false, false)
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

func TestFactPointToJoinImplyNilPointeeHoleFailClosed(t *testing.T) {
	// soft invent: Join soft-skips nil PointTo and still absorbs later pointees
	// fair: incomplete PointTo sticky fail closed (no partial join / soft re-pick)
	ClearErrorSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerTo(GetIntType()), false, false)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntType(), false, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntType(), false, false)
	base := MakeFactPointTo(p, a)
	hole := &FactPointTo{Var: p, PointTo: []*Variable{nil, b}}
	cp := base.Clone()
	if cp.Join(hole) {
		t.Fatal("Join must fail closed on nil pointee hole, not soft-skip")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Join nil pointee must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if len(cp.PointTo) != 1 || cp.PointTo[0] != a {
		t.Fatal("Join must not partially absorb past hole", cp.PointTo)
	}
	if base.Imply(hole) {
		t.Fatal("Imply must fail closed on other nil pointee")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Imply other nil pointee must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if hole.Imply(base) {
		t.Fatal("Imply must fail closed on self nil pointee")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Imply self nil pointee must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if base.Equal(hole) || hole.Equal(base) {
		t.Fatal("equal must fail closed on nil pointee")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("equal nil pointee must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMergeFactLattice(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerTo(GetIntType()), false, false)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntType(), false, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntType(), false, false)
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

func TestMergeFactsNilAccumulatorSticky(t *testing.T) {
	// Fact merge always has live accumulator; sticky no invent soft-skip join
	ClearErrorSess(testAmbientSession)
	if MergeFacts(nil, nil) {
		t.Fatal("nil facts must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil facts MergeFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMergeFacts(t *testing.T) {
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerTo(GetIntType()), false, false)
	q := CreateVariableScalarsSess(testAmbientSession, "g_q", PointerTo(GetIntType()), false, false)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntType(), false, false)
	facts := []*FactPointTo{MakeFactPointTo(p, a)}
	other := []*FactPointTo{MakeFactPointTo(q, a), MakeFactPointTo(p, CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntType(), false, false))}
	MergeFacts(&facts, other)
	if FindRelatedPointTo(facts, q) == nil {
		t.Fatal("q")
	}
	if len(FindRelatedPointTo(facts, p).PointTo) < 2 {
		t.Fatal("p joined")
	}
	// nil hole fails closed sticky — no invent skip partial join / soft re-pick past wipe
	ClearErrorSess(testAmbientSession)
	hole := []*FactPointTo{MakeFactPointTo(p, a), nil}
	base := []*FactPointTo{MakeFactPointTo(q, a)}
	if MergeFacts(&base, hole) {
		t.Fatal("nil new hole must fail closed")
	}
	if FindRelatedPointTo(base, p) != nil {
		t.Fatal("must not invent partial merge past hole")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete MergeFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// PointTo nil hole: Imply/Join residual soft invent was soft-continue merge later.
	// Fair: sticky wipe IncompleteFactSlice whole MergeFacts.
	brokenPT := &FactPointTo{Var: p, PointTo: []*Variable{a, nil}}
	base2 := []*FactPointTo{MakeFactPointTo(p, a)}
	if MergeFacts(&base2, []*FactPointTo{brokenPT}) {
		t.Fatal("PointTo nil hole must fail closed MergeFacts")
	}
	if FactsComplete(base2) {
		t.Fatal("PointTo nil hole must wipe IncompleteFactSlice, not invent complete merge")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("PointTo nil hole MergeFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if FactsComplete(MergeFactInto(facts, nil)) {
		t.Fatal("nil fact MergeFactInto must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil fact MergeFactInto must SetError sticky")
	}
	// MergeFactInto incomplete map marker stays non-sticky (soft re-pick); MergeFacts sticks.
	ClearErrorSess(testAmbientSession)
	if FactsComplete(MergeFactInto([]*FactPointTo{MakeFactPointTo(p, a), nil}, MakeFactPointTo(p, a))) {
		t.Fatal("incomplete map MergeFactInto must fail closed")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete map MergeFactInto must stay non-sticky soft re-pick")
	}
	ClearErrorSess(testAmbientSession)
}

func TestHasEligibleVolatileVar(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_v", GetIntType(), false, true)
	nv := CreateVariableScalarsSess(testAmbientSession, "g_n", GetIntType(), false, false)
	if !HasEligibleVolatileVar([]*Variable{v, nv}, GetIntType(), AccessRead, EmptyCGContext().WithSession(testAmbientSession)) {
		t.Fatal("vol")
	}
	// non-SE-free blocks volatile
	if HasEligibleVolatileVar([]*Variable{v}, GetIntType(), AccessRead, WithEffectContext(WithSideEffects()).WithSession(testAmbientSession)) {
		t.Fatal("se")
	}
	// nil hole fails closed sticky — no invent skip as absent non-vol
	if HasEligibleVolatileVar([]*Variable{nil, v}, GetIntType(), AccessRead, EmptyCGContext().WithSession(testAmbientSession)) {
		t.Fatal("nil hole must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestIfMergesFacts(t *testing.T) {
	opts := Defaults()
	opts.MaxBlockSize = 1
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	f.RV = CreateVariableScalarsSess(testAmbientSession, "f_rv", GetIntType(), false, false)
	fm := NewFactMgrSess(testAmbientSession, f)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerTo(GetIntType()), false, false)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntType(), false, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntType(), false, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, a)}
	vs.GlobalList = []*Variable{p, a, b, CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntType(), false, false)}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	f.Stack = []*Block{{Func: f}}
	// generate if — may or may not change facts; ensure no panic and FM restored/merged
	st := MakeRandomIf(NewRngSess(testAmbientSession, 11), opts, probs, vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg)
	if st == nil || st.Kind != StmtIfElse {
		t.Fatal("if")
	}
	if FindRelatedPointTo(fm.GlobalFacts, p) == nil {
		t.Fatal("p fact lost")
	}
}
