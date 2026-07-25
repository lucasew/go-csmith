package csmith

import "testing"

// StatementIf.cpp:69 — pre_facts = fm->global_facts is a shallow Fact* vector copy.
// Mid-condition Join mutates the shared FactPointTo; pre_facts must observe it
// (deep CloneFactSlice freezes the lattice and diverges from C++ restore path).
func TestFunc1PreFactsSnapshotIsShallow(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	SetProcessOptionsSess(testAmbientSession, opts)
	g := CreateVariableScalarsSess(testAmbientSession, "g_ptr", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	tgt := CreateVariableScalarsSess(testAmbientSession, "g_tgt", GetIntTypeSess(testAmbientSession), false, false)
	if g == nil || tgt == nil {
		t.Fatal("vars")
	}
	// start: g → tgt only
	facts := []*FactPointTo{MakeFactPointToSess(testAmbientSession, g, tgt)}
	// shallow snapshot like StatementIf.cpp:69
	pre := append([]*FactPointTo(nil), facts...)
	// mid-condition Join on the live fact object
	live := FindRelatedPointToSess(testAmbientSession, facts, g)
	if live == nil {
		t.Fatal("missing live fact")
	}
	if !live.JoinSess(testAmbientSession, MakeFactPointToSess(testAmbientSession, g, NullPtr)) {
		t.Fatal("Join should add null", HasErrorSess(testAmbientSession))
	}
	// pre must observe may-null (shared Fact*)
	preF := FindRelatedPointToSess(testAmbientSession, pre, g)
	if preF == nil || !preF.IsNullSess(testAmbientSession) {
		t.Fatalf("shallow pre must see mid-condition Join null")
	}
	// deep clone of the original lattice would freeze:
	deep := CloneFactSliceSess(testAmbientSession, []*FactPointTo{MakeFactPointToSess(testAmbientSession, g, tgt)})
	deepF := FindRelatedPointToSess(testAmbientSession, deep, g)
	if deepF == nil || deepF.IsNullSess(testAmbientSession) {
		t.Fatal("control: deep clone of pre-Join lattice must stay non-null")
	}
	ClearErrorSess(testAmbientSession)
}
