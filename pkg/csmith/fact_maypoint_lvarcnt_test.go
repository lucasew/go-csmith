package csmith

import "testing"

// FactMgr.cpp:376–388 + FactPointTo.cpp:266–278 —
// abstract_fact_for_assign returns lvars.size() including specials that
// make_facts skips. lvar_cnt==2 → merge_fact, not renew_fact.
// Soft invent re-computed/forced lvarCnt==1 and wiped may-null (seed-363 g_73).
func TestApplyPointToAssignLvarCnt2MergesMayNull(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	g4 := CreateVariableScalarsSess(testAmbientSession, "g_4", GetIntTypeSess(testAmbientSession), true, false)
	g73 := CreateVariableScalarsSess(testAmbientSession, "g_73", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)

	// g_73 may-null before definitive-looking transfer fact {g_4}
	facts := []*FactPointTo{
		MakeFactPointToSet(g73, []*Variable{NullPtr, g4}),
	}
	// facts_out for g_73 only (make_facts skipped null special)
	// lvar_cnt==2 because lvars were {null, g_73} (may-point-to)
	newFacts := []*FactPointTo{MakeFactPointToSet(g73, []*Variable{g4})}
	_, ok := applyPointToAssignFacts(&facts, g73, 0, newFacts, 2)
	if !ok {
		t.Fatalf("apply failed err=%v", HasErrorSess(testAmbientSession))
	}
	fp := FindRelatedPointTo(facts, g73)
	if fp == nil || !fp.IsNull() {
		names := []string{}
		if fp != nil {
			for _, p := range fp.PointTo {
				if p != nil {
					names = append(names, p.Name)
				}
			}
		}
		t.Fatalf("lvar_cnt==2 must merge keep may-null, pts=%v", names)
	}

	// Control: same newFacts with lvar_cnt==1 renews and drops null
	facts2 := []*FactPointTo{
		MakeFactPointToSet(g73, []*Variable{NullPtr, g4}),
	}
	_, ok = applyPointToAssignFacts(&facts2, g73, 0, newFacts, 1)
	if !ok {
		t.Fatalf("renew apply failed err=%v", HasErrorSess(testAmbientSession))
	}
	fp2 := FindRelatedPointTo(facts2, g73)
	if fp2 == nil || fp2.IsNull() {
		t.Fatal("lvar_cnt==1 renew must replace with {g_4} only (control)")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMergePointeesIncludesNullSpecial(t *testing.T) {
	// FactPointTo.cpp:756–784 — specials skipped as *ptrs*; pointees may be null
	ClearErrorSess(testAmbientSession)
	g73 := CreateVariableScalarsSess(testAmbientSession, "g_73c", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	g72 := CreateVariableScalarsSess(testAmbientSession, "g_72c", PointerToSess(testAmbientSession, PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))), true, false)
	factsIn := []*FactPointTo{
		MakeFactPointToSet(g72, []*Variable{NullPtr, g73}),
	}
	lvars := MergePointeesOfPointer(g72, 1, factsIn)
	if !VariablesComplete(lvars) {
		t.Fatalf("incomplete lvars err=%v", HasErrorSess(testAmbientSession))
	}
	if len(lvars) != 2 {
		t.Fatalf("want {null,g_73}, got %d", len(lvars))
	}
	hasNull, has73 := false, false
	for _, v := range lvars {
		if v == NullPtr {
			hasNull = true
		}
		if v == g73 {
			has73 = true
		}
	}
	if !hasNull || !has73 {
		t.Fatalf("want null+g_73 in merge_pointees, got %v", lvars)
	}
	ClearErrorSess(testAmbientSession)
}
