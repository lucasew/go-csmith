package csmith

import "testing"

// FactPointTo.cpp:415–426 is_valid_ptr exact var*; facts abstract onto get_collective
// (FactPointTo.cpp:276–277). local_vars holds itemized (create_array_and_itemize).
// AddNewVarFactInto(itemized) must leave a fact keyed on the itemized subject so
// IsValidPtr(itemized) succeeds (seed-10054 nested revisit).
func TestAddNewVarFactIntoItemizedRegistersSubject(t *testing.T) {
	ClearError()
	SetProcessOptions(Defaults())
	i32 := GetIntType()
	l118 := CreateVariableScalars("l_118", PointerTo(i32), false, false)
	elem := PointerTo(PointerTo(i32))
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

	var facts []*FactPointTo
	inUserInvocationRevisit = true
	defer func() { inUserInvocationRevisit = false }()
	AddNewVarFactInto(&item.Variable, &facts)
	if HasError() || !FactsComplete(facts) {
		t.Fatalf("add err=%v complete=%v", HasError(), FactsComplete(facts))
	}
	// collective fact present
	if FindRelatedPointTo(facts, &coll.Variable) == nil {
		t.Fatal("missing collective fact")
	}
	// itemized subject must also be keyed for is_valid_ptr
	if FindRelatedPointTo(facts, &item.Variable) == nil {
		t.Fatal("missing itemized subject fact")
	}
	if !IsValidPtr(&item.Variable, facts, 0, 0) {
		t.Fatal("IsValidPtr(itemized) must succeed after dual registration")
	}
	if IsValidPtr(&item.Variable, facts, 0, 0) && HasError() {
		t.Fatal("sticky after valid")
	}
	ClearError()
}
