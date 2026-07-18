package csmith

import (
	"testing"
)

func TestLhsWriteVarsFromWritten(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	e := EmptyEffect().WriteVar(v)
	e = e.SetLhsWriteVarsFromWritten()
	got := e.LhsWriteVars()
	if len(got) != 1 || got[0] != v {
		t.Fatal(got)
	}
}

func TestWriteVarSet(t *testing.T) {
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	b := CreateVariableScalars("g_b", GetIntType(), false, false)
	e := EmptyEffect().WriteVarSet([]*Variable{a, b})
	if !e.IsWritten(a) || !e.IsWritten(b) {
		t.Fatal("writes")
	}
}

func TestAddEffectOptsIncludeLHS(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	other := EmptyEffect().WriteVar(v).SetLhsWriteVarsFromWritten()
	base := EmptyEffect()
	merged := base.AddEffectOpts(other, true)
	if len(merged.LhsWriteVars()) != 1 {
		t.Fatal("lhs not merged")
	}
	merged2 := base.AddEffectOpts(other, false)
	if len(merged2.LhsWriteVars()) != 0 {
		t.Fatal("lhs should skip")
	}
}

func TestVisitFactsLhsSetsLhsWrite(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	lhs := &Lhs{Var: v, Type: GetIntType()}
	cg := EmptyCGContext()
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if !cg.VisitFactsLhs(lhs, Defaults()) {
		t.Fatal("visit")
	}
	if len(cg.EffectAccum.LhsWriteVars()) != 1 {
		t.Fatal("lhs write not set", cg.EffectAccum.LhsWriteVars())
	}
}

func TestRemoveFunctionLocalFacts(t *testing.T) {
	f := &Function{Name: "f"}
	loc := CreateVariableScalars("l_1", PointerTo(GetIntType()), false, false)
	g := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	f.Blocks = []*Block{{LocalVars: []*Variable{loc}}}
	facts := []*FactPointTo{
		MakeFactPointTo(loc, NullPtr),
		MakeFactPointTo(g, NullPtr),
	}
	out := RemoveFunctionLocalFacts(facts, f)
	if len(out) != 1 || out[0].Var != g {
		t.Fatal(out)
	}
}

func TestRemoveLoopLocalFacts(t *testing.T) {
	outer := &Block{Looping: true}
	inner := &Block{Parent: outer, LocalVars: []*Variable{
		CreateVariableScalars("l_i", GetIntType(), false, false),
	}}
	lp := CreateVariableScalars("l_p", PointerTo(GetIntType()), false, false)
	inner.LocalVars = append(inner.LocalVars, lp)
	g := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	facts := []*FactPointTo{MakeFactPointTo(lp, NullPtr), MakeFactPointTo(g, NullPtr)}
	out := RemoveLoopLocalFacts(facts, inner)
	if len(out) != 1 || out[0].Var != g {
		t.Fatal(out)
	}
}

func TestGetDereferencedPtrs(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	e := &Expression{Term: TermVariable, Var: p, ExprType: GetIntType()}
	d := GetDereferencedPtrs(e)
	if len(d) != 1 {
		t.Fatal(d)
	}
	bare := &Expression{Term: TermVariable, Var: p, ExprType: p.Type}
	if len(GetDereferencedPtrs(bare)) != 0 {
		t.Fatal("no deref")
	}
}
