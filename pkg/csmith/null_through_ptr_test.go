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
	ClearError()
	SetProcessOptionsSess(testAmbientSession, Defaults())
	i32 := GetIntType()
	pt := PointerTo(i32)
	ppt := PointerTo(pt)
	g77 := CreateVariableScalars("g_77", pt, false, false)
	g99 := CreateVariableScalars("g_99", ppt, false, false)
	f := &Function{Name: "f", ReturnType: i32}
	fm := NewFactMgr(f)
	fm.SetGlobalFacts([]*FactPointTo{MakeFactPointTo(g99, g77)}, "t")
	lhs := &Lhs{Var: g99, Type: pt}
	rhs := &Expression{Term: TermConstant, Con: &Constant{Type: pt, Value: "0"}}
	if !fm.UpdateFactForAssignWant(g99, lhs.IndirectLevel(), lhs.GetType(), rhs) {
		t.Fatal("UpdateFactForAssignWant returned false")
	}
	fg := FindRelatedPointTo(fm.GlobalFacts, g77)
	if fg == nil || !fg.IsNull() {
		t.Fatalf("g_77 must be null after *g_99=(void*)0, fact=%v", fg)
	}
	if IsValidPtr(g77, fm.GlobalFacts, 0, 0) {
		t.Fatal("IsValidPtr must fail for null g_77")
	}
	ClearError()
}

// Then *p=null / else leaves q live → merge must may-null q (IsNull).
// StatementIf.cpp: both arms from post-cond; merge outputs.
func TestIfThenNullMergeMakesPointeeInvalid(t *testing.T) {
	ClearError()
	SetProcessOptionsSess(testAmbientSession, Defaults())
	i32 := GetIntType()
	pt := PointerTo(i32)
	ppt := PointerTo(pt)
	g77 := CreateVariableScalars("g_77", pt, false, false)
	g99 := CreateVariableScalars("g_99", ppt, false, false)
	tgt := CreateVariableScalars("g_18", i32, false, false)
	pre := []*FactPointTo{
		MakeFactPointTo(g99, g77),
		MakeFactPointTo(g77, tgt),
	}
	thenWork := CloneFactSlice(pre)
	rhs := &Expression{Term: TermConstant, Con: &Constant{Type: pt, Value: "0"}}
	nf, n := AbstractFactForAssign(thenWork, g99, 1, rhs)
	if n != 1 || len(nf) != 1 {
		t.Fatalf("then abstract n=%d len=%d", n, len(nf))
	}
	_ = RenewFact(&thenWork, nf[0])
	elseWork := CloneFactSlice(pre)
	merged := CloneFactSlice(thenWork)
	_ = MergeFacts(&merged, elseWork)
	fg := FindRelatedPointTo(merged, g77)
	if fg == nil || !fg.IsNull() {
		t.Fatalf("merged g_77 must IsNull (may-null), got %v", fg)
	}
	if IsValidPtr(g77, merged, 0, 0) {
		t.Fatal("IsValidPtr must fail for may-null g_77")
	}
	ClearError()
}
