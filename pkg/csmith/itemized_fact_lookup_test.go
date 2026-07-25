package csmith

import "testing"

// FactPointTo.cpp:415–426 is_valid_ptr exact var*; facts abstract onto get_collective
// (FactPointTo.cpp:276–277). During revisit, IsValidPtr falls back to collective
// without dual-registering itemized subjects into the FactVec (that inflates
// same_facts and breaks nested for shortcut reuse — seed-90).
func TestIsValidPtrItemizedFallsBackToCollectiveOnRevisit(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	SetProcessOptionsSess(testAmbientSession, Defaults())
	i32 := GetIntTypeSess(testAmbientSession)
	tgt := CreateVariableScalarsSess(testAmbientSession, "g_t", i32, false, false)
	l118 := CreateVariableScalarsSess(testAmbientSession, "l_118", PointerToSess(testAmbientSession, i32), false, false)
	elem := PointerToSess(testAmbientSession, PointerToSess(testAmbientSession, i32))
	addr := &Expression{Term: TermVariable, Var: l118, ExprType: elem}
	coll := &ArrayVariable{
		Variable: Variable{Name: "l_165", Type: elem, IsArray: true, InitExpr: addr},
		Sizes:    []int{9},
	}
	coll.AsArray = coll
	item := &ArrayVariable{
		Variable:   Variable{Name: "l_165", Type: elem, IsArray: true, InitExpr: addr},
		Sizes:      []int{9},
		Collective: coll,
		Indices:    []string{"0"},
	}
	item.AsArray = item

	// Only collective on lattice with live pointee (not dead/garbage).
	facts := []*FactPointTo{MakeFactPointToSess(testAmbientSession, &coll.Variable, tgt)}
	if FindRelatedPointToSess(testAmbientSession, facts, &item.Variable) != nil {
		t.Fatal("itemized must not be dual-keyed on lattice")
	}
	currentSession().InUserInvocationRevisit = false
	if IsValidPtrSess(testAmbientSession, &item.Variable, facts, 0, 0) {
		t.Fatal("gen IsValidPtrSess(testAmbientSession, itemized) must miss without dual-reg")
	}
	ClearErrorSess(testAmbientSession)
	currentSession().InUserInvocationRevisit = true
	defer func() { currentSession().InUserInvocationRevisit = false }()
	if !IsValidPtrSess(testAmbientSession, &item.Variable, facts, 0, 0) {
		t.Fatalf("revisit IsValidPtrSess(testAmbientSession, itemized) must fall back to collective err=%v", GetErrorSess(testAmbientSession))
	}
	if FindRelatedPointToSess(testAmbientSession, facts, &item.Variable) != nil {
		t.Fatal("fallback must not invent dual-reg entry")
	}
	ClearErrorSess(testAmbientSession)
}
