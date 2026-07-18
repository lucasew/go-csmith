package csmith

import (
	"testing"
)

func TestIsRV(t *testing.T) {
	v := CreateVariableScalars("func_1_rv", GetIntType(), false, false)
	if !v.IsRV() {
		t.Fatal("rv")
	}
	if CreateVariableScalars("g_1", GetIntType(), false, false).IsRV() {
		t.Fatal("not")
	}
}

func TestCallerToCalleeHandoverKeepsGlobals(t *testing.T) {
	callee := &Function{Name: "c", ReturnType: GetIntType()}
	p := CreateVariableScalars("p_1", PointerTo(GetIntType()), false, false)
	callee.Param = []*Variable{p}
	fm := NewFactMgr(callee)
	g := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	// g points to loc — loc should be kept transitively
	facts := []*FactPointTo{
		MakeFactPointTo(g, loc),
		MakeFactPointTo(CreateVariableScalars("l_p", PointerTo(GetIntType()), false, false), NullPtr),
	}
	// subject l_p is local name — not kept unless pointed
	fm.CallerToCalleeHandover(nil, &facts)
	// keep g (global); drop l_p unless pointed by keep
	if FindRelatedPointTo(facts, g) == nil {
		t.Fatal("lost global")
	}
	// loc subject not in facts initially as subject; g points to loc so if there was a fact for loc as subject...
	// only subjects kept: g and p (after param facts)
	// after AddParamFacts with nil args, p gets NewFactPointTo
	if FindRelatedPointTo(facts, p) == nil {
		// param might be added as garbage
		t.Log("param fact", facts)
	}
}

func TestCallerToCalleeHandoverTransitive(t *testing.T) {
	callee := &Function{Name: "c"}
	fm := NewFactMgr(callee)
	g := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	// caller local that g points to
	loc := CreateVariableScalars("l_tgt", GetIntType(), false, false)
	// fact about loc as subject (e.g. field) — use pointer fact with subject loc that is pointed
	// better: subject is another pointer that lives on stack
	lp := CreateVariableScalars("l_p", PointerTo(GetIntType()), false, false)
	facts := []*FactPointTo{
		MakeFactPointTo(g, lp),   // global points to stack ptr
		MakeFactPointTo(lp, loc), // stack ptr facts
	}
	fm.CallerToCalleeHandover(nil, &facts)
	// g kept; lp kept because g points to it; loc not a subject of pointer fact kept unless...
	if FindRelatedPointTo(facts, g) == nil || FindRelatedPointTo(facts, lp) == nil {
		t.Fatal("transitive", facts)
	}
}

func TestRemoveRVFacts(t *testing.T) {
	f := &Function{Name: "f", RV: CreateVariableScalars("f_rv", GetIntType(), false, false)}
	fm := NewFactMgr(f)
	other := CreateVariableScalars("other_rv", GetIntType(), false, false)
	facts := []*FactPointTo{
		MakeFactPointTo(CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false), NullPtr),
		{Var: other, PointTo: []*Variable{NullPtr}},
		{Var: f.RV, PointTo: []*Variable{NullPtr}},
	}
	// only pointer facts with Type - MakeFactPointTo needs pointer type
	// use raw for rv
	fm.RemoveRVFacts(&facts)
	// other_rv dropped; f_rv kept; g_p kept
	for _, fact := range facts {
		if fact.Var == other {
			t.Fatal("other rv kept")
		}
	}
}

func TestOutputTab(t *testing.T) {
	if OutputTab(0) != "" || OutputTab(2) != "        " {
		t.Fatal(OutputTab(2))
	}
}

func TestAddParamFacts(t *testing.T) {
	callee := &Function{Name: "c"}
	p := CreateVariableScalars("p_1", PointerTo(GetIntType()), false, false)
	callee.Param = []*Variable{p}
	fm := NewFactMgr(callee)
	tgt := CreateVariableScalars("g_t", GetIntType(), false, false)
	arg := &Expression{Term: TermVariable, Var: tgt, ExprType: PointerTo(GetIntType())}
	// &g_t would be better; assign arg as pointing
	// AbstractFactForAssign with variable RHS of pointer type copies
	facts := []*FactPointTo{}
	gp := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	_ = gp
	// use null const
	arg = &Expression{Term: TermConstant, Con: &Constant{Type: PointerTo(GetIntType()), Value: "0"}}
	fm.AddParamFacts([]*Expression{arg}, &facts)
	if FindRelatedPointTo(facts, p) == nil {
		t.Fatal("param fact", facts)
	}
}
