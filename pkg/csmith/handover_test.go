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
	// FactMgr.cpp:108–114 — nil arg → abstract nullptr rhs → garbage for pointer param
	// (no invent NewFactPointTo outside abstract path)
	if FindRelatedPointTo(facts, p) == nil {
		t.Fatal("nil-arg param must get abstract garbage fact", facts)
	}
	if !FindRelatedPointTo(facts, p).IsDead() {
		t.Fatal("nil arg → garbage, not invent other pointee")
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

func TestCallerToCalleeHandoverNilHole(t *testing.T) {
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgr(f)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	facts := []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	fm.CallerToCalleeHandover(nil, &facts)
	if FactsComplete(facts) {
		t.Fatal("nil fact hole must fail closed", facts)
	}
}

func TestCallerToCalleeHandoverParamHoleFailClosed(t *testing.T) {
	// soft invent: Param hole → IsVariableInSet false → drop param from keep
	// fair: VariablesComplete Param fails closed nil inputs
	callee := &Function{Name: "c", ReturnType: GetIntType()}
	p := CreateVariableScalars("p_1", PointerTo(GetIntType()), false, false)
	callee.Param = []*Variable{p, nil}
	fm := NewFactMgr(callee)
	g := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	facts := []*FactPointTo{MakeFactPointTo(g, NullPtr), MakeFactPointTo(p, NullPtr)}
	fm.CallerToCalleeHandover(nil, &facts)
	if FactsComplete(facts) {
		t.Fatal("incomplete Param must fail closed nil inputs, not invent drop param", facts)
	}
}

func TestVariablesCompleteAndIsVariableInSet(t *testing.T) {
	a := CreateVariableScalars("g_a", GetIntType(), true, false)
	b := CreateVariableScalars("g_b", GetIntType(), true, false)
	if !VariablesComplete([]*Variable{a, b}) || VariablesComplete([]*Variable{a, nil, b}) {
		t.Fatal("VariablesComplete")
	}
	if !IsVariableInSet([]*Variable{a, b}, a) {
		t.Fatal("complete membership")
	}
	// incomplete: membership false (no invent skip hole to later match)
	if IsVariableInSet([]*Variable{nil, a}, a) {
		t.Fatal("incomplete set must not invent membership past hole")
	}
}

func TestRemoveRVFactsNilHole(t *testing.T) {
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgr(f)
	facts := []*FactPointTo{nil}
	fm.RemoveRVFacts(&facts)
	if FactsComplete(facts) {
		t.Fatal("nil fact hole must fail closed", facts)
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
	// FactMgr.cpp:108–114 — update_fact_for_assign; null const → null fact
	// (not invent NewFactPointTo garbage when abstract succeeds)
	callee := &Function{Name: "c"}
	p := CreateVariableScalars("p_1", PointerTo(GetIntType()), false, false)
	callee.Param = []*Variable{p}
	fm := NewFactMgr(callee)
	facts := []*FactPointTo{}
	arg := &Expression{Term: TermConstant, Con: &Constant{Type: PointerTo(GetIntType()), Value: "0"}, ExprType: PointerTo(GetIntType())}
	fm.AddParamFacts([]*Expression{arg}, &facts)
	got := FindRelatedPointTo(facts, p)
	if got == nil {
		t.Fatal("param fact", facts)
	}
	if got.IsDead() {
		t.Fatal("null arg must not invent garbage")
	}
	if !got.IsNull() {
		t.Fatalf("want null, got %+v", got.PointTo)
	}
	// missing arg → nullptr rhs → garbage via abstract
	facts2 := []*FactPointTo{}
	fm.AddParamFacts(nil, &facts2)
	got2 := FindRelatedPointTo(facts2, p)
	if got2 == nil || !got2.IsDead() {
		t.Fatal("nil args → garbage via abstract", facts2)
	}
}
