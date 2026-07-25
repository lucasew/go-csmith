package csmith

import "testing"

// (*p)=(void*)0 when p→q must renew q to null.
// FactPointTo.cpp:275–278 (rhs_to_lhs_transfer pointer const 0 → null_ptr) +
// FactMgr.cpp:370–391 (lvar_cnt==1 non-array → renew_fact).
//
// Seed 17809409409875472624: func_61 for body has
//
//	if (...) { (*g_99)=(void*)0; ... } else { (*g_77)^=...; }
//
// with g_99→g_77. Then-arm nulls g_77; if-merge may-nulls g_77 so later
// is_valid_ptr fails. C++ post_creation FP strips the for; Go keeps it when
// this transfer/merge path is wrong or skipped (shortcut).
func TestAssignNullThroughPointerRenewsPointee(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	SetProcessOptionsSess(testAmbientSession, Defaults())
	i32 := GetIntTypeSess(testAmbientSession)
	pt := PointerToSess(testAmbientSession, i32)
	ppt := PointerToSess(testAmbientSession, pt)
	g77 := CreateVariableScalarsSess(testAmbientSession, "g_77", pt, false, false)
	g99 := CreateVariableScalarsSess(testAmbientSession, "g_99", ppt, false, false)
	f := &Function{Name: "f", ReturnType: i32}
	fm := NewFactMgrSess(testAmbientSession, f)
	fm.SetGlobalFacts([]*FactPointTo{MakeFactPointToSess(testAmbientSession, g99, g77)}, "t")
	lhs := &Lhs{Var: g99, Type: pt}
	rhs := &Expression{Term: TermConstant, Con: &Constant{Type: pt, Value: "0"}}
	if !fm.UpdateFactForAssignWant(g99, lhs.IndirectLevelSess(testAmbientSession), lhs.GetType(), rhs) {
		t.Fatal("UpdateFactForAssignWant returned false")
	}
	fg := FindRelatedPointToSess(testAmbientSession, fm.GlobalFacts, g77)
	if fg == nil || !fg.IsNullSess(testAmbientSession) {
		t.Fatalf("g_77 must be null after *g_99=(void*)0, fact=%v", fg)
	}
	if IsValidPtrSess(testAmbientSession, g77, fm.GlobalFacts, 0, 0) {
		t.Fatal("IsValidPtr must fail for null g_77")
	}
	ClearErrorSess(testAmbientSession)
}

// Then *p=null / else leaves q live → merge must may-null q (IsNull).
// StatementIf.cpp: both arms from post-cond; merge outputs.
func TestIfThenNullMergeMakesPointeeInvalid(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	SetProcessOptionsSess(testAmbientSession, Defaults())
	i32 := GetIntTypeSess(testAmbientSession)
	pt := PointerToSess(testAmbientSession, i32)
	ppt := PointerToSess(testAmbientSession, pt)
	g77 := CreateVariableScalarsSess(testAmbientSession, "g_77", pt, false, false)
	g99 := CreateVariableScalarsSess(testAmbientSession, "g_99", ppt, false, false)
	tgt := CreateVariableScalarsSess(testAmbientSession, "g_18", i32, false, false)
	pre := []*FactPointTo{
		MakeFactPointToSess(testAmbientSession, g99, g77),
		MakeFactPointToSess(testAmbientSession, g77, tgt),
	}
	thenWork := CloneFactSliceSess(testAmbientSession, pre)
	rhs := &Expression{Term: TermConstant, Con: &Constant{Type: pt, Value: "0"}}
	nf, n := AbstractFactForAssignSess(testAmbientSession, thenWork, g99, 1, rhs)
	if n != 1 || len(nf) != 1 {
		t.Fatalf("then abstract n=%d len=%d", n, len(nf))
	}
	_ = RenewFact(&thenWork, nf[0])
	elseWork := CloneFactSliceSess(testAmbientSession, pre)
	merged := CloneFactSliceSess(testAmbientSession, thenWork)
	_ = MergeFactsSess(testAmbientSession, &merged, elseWork)
	fg := FindRelatedPointToSess(testAmbientSession, merged, g77)
	if fg == nil || !fg.IsNullSess(testAmbientSession) {
		t.Fatalf("merged g_77 must IsNull (may-null), got %v", fg)
	}
	if IsValidPtrSess(testAmbientSession, g77, merged, 0, 0) {
		t.Fatal("IsValidPtr must fail for may-null g_77")
	}
	ClearErrorSess(testAmbientSession)
}
