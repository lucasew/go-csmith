package csmith

import "testing"

// Production path for seed-2 l_233: InitExpr=&g then ArrayOp-style merge assign.
// FactMgr.cpp:376–388 merge for array; Fact.cpp:85–112 var init abstract.
func TestPointerArrayInitThenArrayOpMerge(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	SetProcessOptionsSess(testAmbientSession, opts)
	SetProcessProbabilitiesSess(testAmbientSession, NewProbabilities(opts))
	r := NewRngSess(testAmbientSession, 42)
	SetProcessRngSess(testAmbientSession, r)
	g := CreateVariableScalarsSess(testAmbientSession, "g_127", PointerToSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EShort)), false, false)
	elem := PointerToSess(testAmbientSession, PointerToSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EShort)))
	ie := &Expression{Term: TermVariable, Var: g, ExprType: elem}
	q := NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false})
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
	fp := FindRelatedPointToSess(testAmbientSession, fm.GlobalFacts, &av.Variable)
	if fp == nil || fp.IsNullSess(testAmbientSession) || fp.IsDeadSess(testAmbientSession) {
		t.Fatalf("after InitExpr=&g want pure live: %+v", fp)
	}
	rhs := &Expression{Term: TermVariable, Var: g, ExprType: elem}
	if !fm.UpdateFactForAssign(&av.Variable, 0, rhs) {
		t.Fatal("arrayop")
	}
	fp2 := FindRelatedPointToSess(testAmbientSession, fm.GlobalFacts, &av.Variable)
	if fp2 == nil || fp2.IsNullSess(testAmbientSession) || fp2.IsDeadSess(testAmbientSession) {
		t.Fatalf("after arrayop want pure live: %+v", fp2)
	}
	fm2 := NewFactMgrSess(testAmbientSession, &Function{Name: "f2"})
	fm2.GlobalFacts = []*FactPointTo{MakeFactPointToSess(testAmbientSession, &av.Variable, NullPtr)}
	if !fm2.UpdateFactForAssign(&av.Variable, 0, rhs) {
		t.Fatal("merge null")
	}
	fp3 := FindRelatedPointToSess(testAmbientSession, fm2.GlobalFacts, &av.Variable)
	if fp3 == nil || !fp3.IsNullSess(testAmbientSession) {
		t.Fatalf("from null entry must keep may-null: %+v", fp3)
	}
	ClearErrorSess(testAmbientSession)
}
