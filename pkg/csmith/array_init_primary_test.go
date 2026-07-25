package csmith

import "testing"

// Fact.cpp:89–111 — primary Variable::init first, then get_more_init_values.
// CreateArrayVariable puts alts in InitExprs; createAndInitialize sets InitExpr primary.
// Soft invent AbstractFactForVarInit with nil primary + InitExprs only:
// first AbstractFactForAssignSess(testAmbientSession, nil rhs) → GarbagePtr, then merge alts still IsDead
// (seed-10054 IsValidPtr fail on local pointer arrays during revisit).
func TestAbstractFactForVarInitPrimaryThenAltsNotDead(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	i32 := GetIntTypeSess(testAmbientSession)
	// pointee local: int32_t* shell
	l118 := CreateVariableScalarsSess(testAmbientSession, "l_118", PointerToSess(testAmbientSession, i32), false, false)
	// element type int32_t**
	elem := PointerToSess(testAmbientSession, PointerToSess(testAmbientSession, i32))
	// address-of l_118 as int32_t**
	addr := &Expression{Term: TermVariable, Var: l118, ExprType: elem}
	if n, ok := addr.IndirectLevelCompleteSess(testAmbientSession); !ok || n != -1 {
		t.Fatalf("addr-of level want -1 got %d ok=%v", n, ok)
	}
	q := NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false})
	av := &ArrayVariable{
		Variable: Variable{
			Name:    "l_165",
			Type:    elem,
			Qfer:    q.Clone(),
			IsArray: true,
			// primary only — fair with createAndInitialize InitExpr
			InitExpr: addr,
		},
		Sizes: []int{9},
	}
	av.AsArray = av
	// alts also &l_118
	av.InitExprs = []*Expression{addr, addr}

	pt, _ := AbstractFactForVarInitSess(testAmbientSession, &av.Variable)
	if HasErrorSess(testAmbientSession) {
		t.Fatalf("abstract sticky %v", HasErrorSess(testAmbientSession))
	}
	if !FactsComplete(pt) || len(pt) != 1 {
		t.Fatalf("pt incomplete/len %v n=%d", FactsComplete(pt), len(pt))
	}
	if pt[0].IsDeadSess(testAmbientSession) || pt[0].IsNullSess(testAmbientSession) {
		t.Fatalf("primary+alts must be pure live, dead=%v null=%v pts=%v",
			pt[0].IsDeadSess(testAmbientSession), pt[0].IsNullSess(testAmbientSession), pt[0].PointTo)
	}
	// must point at l_118 collective
	found := false
	for _, p := range pt[0].PointTo {
		if p == l118 || p == l118.GetCollectiveSess(testAmbientSession) {
			found = true
		}
	}
	if !found {
		t.Fatalf("want pointee l_118, got %+v", pt[0].PointTo)
	}
	ClearErrorSess(testAmbientSession)
}

// Nil InitExpr + InitExprs-only: promote InitExprs[0] as primary (no garbage-first).
func TestAbstractFactForVarInitNilPrimaryInitExprsOnly(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	i32 := GetIntTypeSess(testAmbientSession)
	l118 := CreateVariableScalarsSess(testAmbientSession, "l_118", PointerToSess(testAmbientSession, i32), false, false)
	elem := PointerToSess(testAmbientSession, PointerToSess(testAmbientSession, i32))
	addr := &Expression{Term: TermVariable, Var: l118, ExprType: elem}
	av := &ArrayVariable{
		Variable: Variable{
			Name: "l_165", Type: elem, IsArray: true,
			// NO InitExpr primary — only InitExprs alts
		},
		Sizes:     []int{9},
		InitExprs: []*Expression{addr, addr},
	}
	av.AsArray = av
	pt, _ := AbstractFactForVarInitSess(testAmbientSession, &av.Variable)
	if HasErrorSess(testAmbientSession) {
		t.Fatalf("sticky err %v", HasErrorSess(testAmbientSession))
	}
	if !FactsComplete(pt) || len(pt) != 1 {
		t.Fatalf("want complete fact, complete=%v n=%d", FactsComplete(pt), len(pt))
	}
	if pt[0].IsDeadSess(testAmbientSession) || pt[0].IsNullSess(testAmbientSession) {
		t.Fatalf("promoted primary must be pure live dead=%v null=%v", pt[0].IsDeadSess(testAmbientSession), pt[0].IsNullSess(testAmbientSession))
	}
	ClearErrorSess(testAmbientSession)
}
