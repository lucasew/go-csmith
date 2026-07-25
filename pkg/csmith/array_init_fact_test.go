package csmith

import "testing"

// Production path for seed-2 l_233: InitExpr=&g then ArrayOp-style merge assign.
// FactMgr.cpp:376–388 merge for array; Fact.cpp:85–112 var init abstract.
func TestPointerArrayInitThenArrayOpMerge(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	SetProcessOptionsSess(testAmbientSession, opts)
	SetProcessProbabilitiesSess(testAmbientSession, NewProbabilities(opts))
	r := NewRng(42)
	SetProcessRngSess(testAmbientSession, r)
	g := CreateVariableScalarsSess(testAmbientSession, "g_127", PointerTo(GetSimpleType(EShort)), false, false)
	elem := PointerTo(PointerTo(GetSimpleType(EShort)))
	ie := &Expression{Term: TermVariable, Var: g, ExprType: elem}
	q := NewCVQualifiers([]bool{false}, []bool{false})
	av := CreateArrayVariable(r, opts, NewProbabilities(opts), nil, nil, nil, "l_233", elem, nil, q)
	if av == nil {
		t.Fatal("create")
	}
	av.InitExpr = ie
	fm := NewFactMgrSess(testAmbientSession, &Function{Name: "f"})
	fm.AddNewVarFact(&av.Variable)
	if HasErrorSess(testAmbientSession) {
		t.Fatalf("add %v", HasErrorSess(testAmbientSession))
	}
	fp := FindRelatedPointTo(fm.GlobalFacts, &av.Variable)
	if fp == nil || fp.IsNull() || fp.IsDead() {
		t.Fatalf("after InitExpr=&g want pure live: %+v", fp)
	}
	rhs := &Expression{Term: TermVariable, Var: g, ExprType: elem}
	if !fm.UpdateFactForAssign(&av.Variable, 0, rhs) {
		t.Fatal("arrayop")
	}
	fp2 := FindRelatedPointTo(fm.GlobalFacts, &av.Variable)
	if fp2 == nil || fp2.IsNull() || fp2.IsDead() {
		t.Fatalf("after arrayop want pure live: %+v", fp2)
	}
	fm2 := NewFactMgrSess(testAmbientSession, &Function{Name: "f2"})
	fm2.GlobalFacts = []*FactPointTo{MakeFactPointTo(&av.Variable, NullPtr)}
	if !fm2.UpdateFactForAssign(&av.Variable, 0, rhs) {
		t.Fatal("merge null")
	}
	fp3 := FindRelatedPointTo(fm2.GlobalFacts, &av.Variable)
	if fp3 == nil || !fp3.IsNull() {
		t.Fatalf("from null entry must keep may-null: %+v", fp3)
	}
	ClearErrorSess(testAmbientSession)
}
