package csmith

import "testing"

// FactUnion.cpp:133–135 + FactMgr.cpp:370–381 — abstract_fact_for_assign uses
// Lhs::get_type() (desired type after deref). Soft invent used Variable.Type of
// the pointer: (*union U0 *p)= never took rhs_to_lhs_transfer, so eUnionWrite
// stayed BOTTOM while UP renewed last_write from whole-union star-assign
// (seed-177: UP g_88.f0 vs GO g_76 — ok pool n=30 with g_88.f0 vs n=29).
func TestStarAssignUnionPtrRenewsLastWrite(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	ut := &Type{isUnion: true, StructName: "U0", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f1", Type: GetIntType(), BitWidth: -1},
	}}
	g88 := CreateVariableQfer("g_88", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	g88.CreateFieldVars()
	// pointer to union
	l90 := CreateVariableQfer("l_90", PointerTo(ut), NewCVQualifiers([]bool{false}, []bool{false}))
	// source union with last=f0
	src := CreateVariableQfer("l_89", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	src.CreateFieldVars()

	f := &Function{Name: "func_t", ReturnType: GetIntType()}
	fm := NewFactMgr(f)
	// l_90 points to g_88
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(l90, g88)}
	// g_88 currently BOTTOM (as after conflicting path merges)
	fm.UnionFacts = []*FactUnion{MakeFactUnion(g88, FactUnionBottom), MakeFactUnion(src, 0)}

	// (*l_90) = src  — Lhs desired type is union, indir=1
	lhs := &Lhs{Var: l90, Type: ut}
	rhs := &Expression{Term: TermVariable, Var: src, ExprType: ut}
	indir := lhs.IndirectLevel()
	if indir != 1 {
		t.Fatalf("indir want 1 got %d err=%v", indir, GetErrorSess(testAmbientSession))
	}
	if !fm.UpdateFactForAssignWant(l90, indir, lhs.GetType(), rhs) {
		t.Fatalf("UpdateFactForAssignWant: %v", GetErrorSess(testAmbientSession))
	}
	got := FindRelatedUnion(fm.UnionFacts, g88)
	if got == nil {
		t.Fatal("missing g_88 after star-assign")
	}
	if got.LastWrittenFID != 0 {
		t.Fatalf("(*union*)= must renew last=0 (not keep BOTTOM), got %d", got.LastWrittenFID)
	}
	// Negative: Variable.Type-only path (old soft invent) must NOT transfer
	fm.UnionFacts = []*FactUnion{MakeFactUnion(g88, FactUnionBottom), MakeFactUnion(src, 0)}
	_ = fm.UpdateFactForAssign(l90, 1, rhs)
	got2 := FindRelatedUnion(fm.UnionFacts, g88)
	if got2 == nil {
		t.Fatal("missing g_88")
	}
	if got2.LastWrittenFID != FactUnionBottom {
		t.Fatalf("bare Variable.Type path must not invent union transfer; last=%d", got2.LastWrittenFID)
	}
}
