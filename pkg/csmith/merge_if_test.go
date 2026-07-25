package csmith

import "testing"

func TestFactPointToImplyJoin(t *testing.T) {
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), false, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntTypeSess(testAmbientSession), false, false)
	wide := MakeFactPointToSetSess(testAmbientSession, p, []*Variable{a, b})
	narrow := MakeFactPointToSess(testAmbientSession, p, a)
	if !wide.ImplySess(testAmbientSession, narrow) {
		t.Fatal("wide implies narrow")
	}
	if narrow.ImplySess(testAmbientSession, wide) {
		t.Fatal("narrow not imply wide")
	}
	n2 := narrow.CloneSess(testAmbientSession)
	if !n2.JoinSess(testAmbientSession, MakeFactPointToSess(testAmbientSession, p, b)) {
		t.Fatal("join changed")
	}
	if !n2.ImplySess(testAmbientSession, wide) || !wide.ImplySess(testAmbientSession, n2) {
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
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), false, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntTypeSess(testAmbientSession), false, false)
	base := MakeFactPointToSess(testAmbientSession, p, a)
	hole := &FactPointTo{Var: p, PointTo: []*Variable{nil, b}}
	cp := base.CloneSess(testAmbientSession)
	if cp.JoinSess(testAmbientSession, hole) {
		t.Fatal("Join must fail closed on nil pointee hole, not soft-skip")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Join nil pointee must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if len(cp.PointTo) != 1 || cp.PointTo[0] != a {
		t.Fatal("Join must not partially absorb past hole", cp.PointTo)
	}
	if base.ImplySess(testAmbientSession, hole) {
		t.Fatal("Imply must fail closed on other nil pointee")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Imply other nil pointee must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if hole.ImplySess(testAmbientSession, base) {
		t.Fatal("Imply must fail closed on self nil pointee")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Imply self nil pointee must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if base.EqualSess(testAmbientSession, hole) || hole.EqualSess(testAmbientSession, base) {
		t.Fatal("equal must fail closed on nil pointee")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("equal nil pointee must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMergeFactLattice(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), false, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntTypeSess(testAmbientSession), false, false)
	facts := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, a)}
	facts = MergeFactIntoSess(testAmbientSession, facts, MakeFactPointToSess(testAmbientSession, p, b))
	fp := FindRelatedPointToSess(testAmbientSession, facts, p)
	if fp == nil || len(fp.PointTo) != 2 {
		t.Fatalf("%+v", fp)
	}
	// weaker fact no change
	n := len(facts)
	facts = MergeFactIntoSess(testAmbientSession, facts, MakeFactPointToSess(testAmbientSession, p, a))
	if len(facts) != n {
		t.Fatal("len")
	}
}

func TestMergeFactsNilAccumulatorSticky(t *testing.T) {
	// Fact merge always has live accumulator; sticky no invent soft-skip join
	ClearErrorSess(testAmbientSession)
	if MergeFactsSess(testAmbientSession, nil, nil) {
		t.Fatal("nil facts must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil facts MergeFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMergeFacts(t *testing.T) {
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	q := CreateVariableScalarsSess(testAmbientSession, "g_q", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), false, false)
	facts := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, a)}
	other := []*FactPointTo{MakeFactPointToSess(testAmbientSession, q, a), MakeFactPointToSess(testAmbientSession, p, CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntTypeSess(testAmbientSession), false, false))}
	MergeFactsSess(testAmbientSession, &facts, other)
	if FindRelatedPointToSess(testAmbientSession, facts, q) == nil {
		t.Fatal("q")
	}
	if len(FindRelatedPointToSess(testAmbientSession, facts, p).PointTo) < 2 {
		t.Fatal("p joined")
	}
	// nil hole fails closed sticky — no invent skip partial join / soft re-pick past wipe
	ClearErrorSess(testAmbientSession)
	hole := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, a), nil}
	base := []*FactPointTo{MakeFactPointToSess(testAmbientSession, q, a)}
	if MergeFactsSess(testAmbientSession, &base, hole) {
		t.Fatal("nil new hole must fail closed")
	}
	if FindRelatedPointToSess(testAmbientSession, base, p) != nil {
		t.Fatal("must not invent partial merge past hole")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete MergeFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// PointTo nil hole: Imply/Join residual soft invent was soft-continue merge later.
	// Fair: sticky wipe IncompleteFactSlice whole MergeFacts.
	brokenPT := &FactPointTo{Var: p, PointTo: []*Variable{a, nil}}
	base2 := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, a)}
	if MergeFactsSess(testAmbientSession, &base2, []*FactPointTo{brokenPT}) {
		t.Fatal("PointTo nil hole must fail closed MergeFacts")
	}
	if FactsComplete(base2) {
		t.Fatal("PointTo nil hole must wipe IncompleteFactSlice, not invent complete merge")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("PointTo nil hole MergeFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if FactsComplete(MergeFactIntoSess(testAmbientSession, facts, nil)) {
		t.Fatal("nil fact MergeFactInto must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil fact MergeFactInto must SetError sticky")
	}
	// MergeFactInto incomplete map marker stays non-sticky (soft re-pick); MergeFacts sticks.
	ClearErrorSess(testAmbientSession)
	if FactsComplete(MergeFactIntoSess(testAmbientSession, []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, a), nil}, MakeFactPointToSess(testAmbientSession, p, a))) {
		t.Fatal("incomplete map MergeFactInto must fail closed")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete map MergeFactInto must stay non-sticky soft re-pick")
	}
	ClearErrorSess(testAmbientSession)
}

func TestHasEligibleVolatileVar(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_v", GetIntTypeSess(testAmbientSession), false, true)
	nv := CreateVariableScalarsSess(testAmbientSession, "g_n", GetIntTypeSess(testAmbientSession), false, false)
	if !HasEligibleVolatileVar([]*Variable{v, nv}, GetIntTypeSess(testAmbientSession), AccessRead, EmptyCGContext().WithSession(testAmbientSession)) {
		t.Fatal("vol")
	}
	// non-SE-free blocks volatile
	if HasEligibleVolatileVar([]*Variable{v}, GetIntTypeSess(testAmbientSession), AccessRead, WithEffectContext(WithSideEffects()).WithSession(testAmbientSession)) {
		t.Fatal("se")
	}
	// nil hole fails closed sticky — no invent skip as absent non-vol
	if HasEligibleVolatileVar([]*Variable{nil, v}, GetIntTypeSess(testAmbientSession), AccessRead, EmptyCGContext().WithSession(testAmbientSession)) {
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
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	f.RV = CreateVariableScalarsSess(testAmbientSession, "f_rv", GetIntTypeSess(testAmbientSession), false, false)
	fm := NewFactMgrSess(testAmbientSession, f)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), false, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntTypeSess(testAmbientSession), false, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, a)}
	vs.GlobalList = []*Variable{p, a, b, CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntTypeSess(testAmbientSession), false, false)}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	f.Stack = []*Block{{Func: f}}
	// generate if — may or may not change facts; ensure no panic and FM restored/merged
	st := MakeRandomIf(NewRngSess(testAmbientSession, 11), opts, probs, vs, NewExprTablesSess(testAmbientSession, opts), NewStatementThresholdTable(opts), &cg)
	if st == nil || st.Kind != StmtIfElse {
		t.Fatal("if")
	}
	if FindRelatedPointToSess(testAmbientSession, fm.GlobalFacts, p) == nil {
		t.Fatal("p fact lost")
	}
}
