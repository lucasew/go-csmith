package csmith

import "testing"

// TestOrderedBinaryMergeMakeupUnionInitLast0 —
// FunctionInvocation.cpp:275–279 ordered &&/|| after RHS:
//
//	makeup_new_var_facts(facts_copy, global) then merge_facts(global, facts_copy).
//
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

// TestOrderedBinaryNilSnapshotStillMakeupMerge —
// FunctionInvocation.cpp:275–279 — ordered &&/|| always makeup+merge, even when
// the post-LHS snapshot is empty. NewFactMgr leaves GlobalFacts/UnionFacts nil;
// CloneFactSlice(nil)/CloneUnionFactSlice(nil) return nil. Soft invent guarded
// the make_random path with `factsCopy != nil`, which skipped the merge entirely
// for the first-program ordered binary (UP seed-199 seq=1: nCopy=0, live last=3
// → JOIN 0⊕3 BOTTOM; Go kept last=3 and later ChooseOKVar pool differed).
func TestOrderedBinaryNilSnapshotStillMakeupMerge(t *testing.T) {
	ClearError()
	SetProcessOptions(Defaults())
	ut := &Type{isUnion: true, StructName: "U_ord_nil", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: GetIntType(), BitWidth: -1},
		{Name: "f2", Type: GetIntType(), BitWidth: -1},
		{Name: "f3", Type: GetIntType(), BitWidth: -1},
	}}
	parent := CreateVariableScalars("g_u_nil", ut, false, false)
	parent.Init = MakeInt(0)
	parent.CreateFieldVars()
	liveU := MakeFactUnion(parent, 3)
	if liveU == nil {
		t.Fatal("live")
	}
	// nil snapshots == Clone*(nil) after NewFactMgr zero-value maps
	var factsCopy []*FactPointTo
	var unionCopy []*FactUnion
	fm := NewFactMgr(&Function{Name: "f", ReturnType: GetIntType()})
	// post-RHS live (g_u created during RHS of the outer &&)
	fm.UnionFacts = []*FactUnion{liveU}
	fm.GlobalFacts = []*FactPointTo{}

	// Same block as make_random ordered path (no factsCopy != nil guard).
	if !MakeupNewVarFacts(&factsCopy, fm.GlobalFacts) ||
		!makeupNewUnionFacts(&unionCopy, fm.UnionFacts) {
		t.Fatalf("makeup sticky=%v", GetError())
	}
	if !FactsComplete(factsCopy) || !UnionFactsComplete(unionCopy) {
		t.Fatal("makeup must leave complete snapshots")
	}
	got := FindRelatedUnion(unionCopy, parent)
	if got == nil || got.LastWrittenFID != 0 {
		t.Fatalf("nil snapshot makeup must add init last=0, got %#v", got)
	}
	_ = MergeFacts(&fm.GlobalFacts, factsCopy)
	for _, f := range unionCopy {
		fm.UnionFacts = MergeUnionFactInto(fm.UnionFacts, f)
	}
	if HasError() {
		t.Fatal(GetError())
	}
	merged := FindRelatedUnion(fm.UnionFacts, parent)
	if merged == nil || !merged.IsBottom() {
		t.Fatalf("nil post-LHS ⊕ live last=3 must BOTTOM, got %#v", merged)
	}
	ClearError()
}
