package csmith

import "testing"

// StatementIf.cpp:69 — pre_facts = fm->global_facts is a shallow Fact* vector copy.
// Mid-condition Join mutates the shared FactPointTo; pre_facts must observe it
// (deep CloneFactSlice freezes the lattice and diverges from C++ restore path).
func TestFunc1PreFactsSnapshotIsShallow(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	SetProcessOptionsSess(testAmbientSession, opts)
	g := CreateVariableScalars("g_ptr", PointerTo(GetIntType()), false, false)
	tgt := CreateVariableScalars("g_tgt", GetIntType(), false, false)
	if g == nil || tgt == nil {
		t.Fatal("vars")
	}
	// start: g → tgt only
	facts := []*FactPointTo{MakeFactPointTo(g, tgt)}
	// shallow snapshot like StatementIf.cpp:69
	pre := append([]*FactPointTo(nil), facts...)
	// mid-condition Join on the live fact object
	live := FindRelatedPointTo(facts, g)
	if live == nil {
		t.Fatal("missing live fact")
	}
	if !live.Join(MakeFactPointTo(g, NullPtr)) {
		t.Fatal("Join should add null", HasErrorSess(testAmbientSession))
	}
	// pre must observe may-null (shared Fact*)
	preF := FindRelatedPointTo(pre, g)
	if preF == nil || !preF.IsNull() {
		t.Fatalf("shallow pre must see mid-condition Join null")
	}
	// deep clone of the original lattice would freeze:
	deep := CloneFactSlice([]*FactPointTo{MakeFactPointTo(g, tgt)})
	deepF := FindRelatedPointTo(deep, g)
	if deepF == nil || deepF.IsNull() {
		t.Fatal("control: deep clone of pre-Join lattice must stay non-null")
	}
	ClearErrorSess(testAmbientSession)
}
