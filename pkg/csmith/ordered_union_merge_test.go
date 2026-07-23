package csmith

import "testing"

// TestOrderedBinaryMergeMakeupUnionInitLast0 —
// FunctionInvocation.cpp:275–279 ordered &&/|| after RHS:
//   makeup_new_var_facts(facts_copy, global) then merge_facts(global, facts_copy).
// When g_u is absent from facts_copy (not present after LHS) but live holds
// last_written=f3, makeup calls add_new_var_fact → abstract_fact_for_var_init
// (last=f0=0); merge_facts joins live f3 with copy f0 → BOTTOM (FactUnion::join).
// Observed UP seed-199: UP_ORDERED_MERGE live=3 copy=MISSING then JOIN 0⊕3.
func TestOrderedBinaryMergeMakeupUnionInitLast0(t *testing.T) {
	ClearError()
	SetProcessOptions(Defaults())
	ut := &Type{isUnion: true, StructName: "U_ord2", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: GetIntType(), BitWidth: -1},
		{Name: "f2", Type: GetIntType(), BitWidth: -1},
		{Name: "f3", Type: GetIntType(), BitWidth: -1},
	}}
	parent := CreateVariableScalars("g_u_ord", ut, false, false)
	parent.Init = MakeInt(0) // union init abstract → last=0 (f0)
	parent.CreateFieldVars()
	liveU := MakeFactUnion(parent, 3)
	if liveU == nil {
		t.Fatal("live")
	}
	var unionCopy []*FactUnion // MISSING subject after LHS
	fm := NewFactMgr(&Function{Name: "f", ReturnType: GetIntType()})
	fm.UnionFacts = []*FactUnion{liveU}
	fm.GlobalFacts = []*FactPointTo{}

	if !makeupNewUnionFacts(&unionCopy, fm.UnionFacts) {
		t.Fatalf("union makeup sticky=%v", GetError())
	}
	got := FindRelatedUnion(unionCopy, parent)
	if got == nil {
		t.Fatal("makeup must add init union fact")
	}
	if got.LastWrittenFID != 0 {
		t.Fatalf("makeup must use abstract_fact_for_var_init last=0, got %d", got.LastWrittenFID)
	}
	for _, f := range unionCopy {
		fm.UnionFacts = MergeUnionFactInto(fm.UnionFacts, f)
	}
	if HasError() {
		t.Fatal(GetError())
	}
	merged := FindRelatedUnion(fm.UnionFacts, parent)
	if merged == nil || !merged.IsBottom() {
		t.Fatalf("live last=3 ⊕ makeup last=0 must BOTTOM, got %#v", merged)
	}
	ClearError()
}
