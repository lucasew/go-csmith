package csmith

import (
	"testing"
)

// FactMgr.cpp:108–114 / 370–395 — add_param_facts uses Lhs* update_fact_for_assign,
// which must not set Function::fact_changed. StatementAssign overload alone does
// (FactMgr.cpp:397–403). Spurious FactChanged forced NeedsRevisit; for visit drops
// make_iteration IV read while static feffect path keeps it (seed 1048… func_53 g_283).
func TestAddParamFactsDoesNotSetFactChanged(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	SetProcessOptionsSess(testAmbientSession, opts)
	fn := &Function{Name: "func_param", ReturnType: GetIntType()}
	p := CreateVariableScalars("p_0", PointerTo(GetIntType()), false, false)
	fn.Param = []*Variable{p}
	fm := NewFactMgrSess(testAmbientSession, fn)
	g := CreateVariableScalars("g_t", GetIntType(), false, false)
	arg := &Expression{Term: TermVariable, Var: g}
	// address-of style: pointer arg pointing at g via variable expression is fine;
	// abstract for pointer param from non-ptr may yield garbage — still a lattice write
	facts := []*FactPointTo{}
	fm.AddParamFacts([]*Expression{arg}, &facts)
	if HasErrorSess(testAmbientSession) {
		t.Fatal("AddParamFacts sticky", HasErrorSess(testAmbientSession))
	}
	if fn.FactChanged {
		t.Fatal("add_param_facts must not set fact_changed (FactMgr.cpp:108–114 Lhs* path)")
	}
	// StatementAssign path still sets FactChanged
	ClearErrorSess(testAmbientSession)
	fn.FactChanged = false
	if fm.UpdateFactForAssign(p, 0, arg) || len(fm.GlobalFacts) > 0 {
		// if lattice changed, FactChanged must stick
		if len(fm.GlobalFacts) > 0 && !fn.FactChanged {
			t.Fatal("StatementAssign UpdateFactForAssign must set fact_changed when facts change")
		}
	}
	ClearErrorSess(testAmbientSession)
}

// Seed 10482453124604569829 — func_53 body is for(g_283=…); must use static feffect
// path (no FactChanged from params alone) so caller func_14 reads include g_283.
func TestSeed1048Func53NoSpuriousFactChanged(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.Seed = 10482453124604569829
	g := NewProgramGenerator(NewSession(opts))
	out := g.GoGenerator()
	if out == "" {
		t.Fatal("empty program", HasErrorSess(testAmbientSession))
	}
	var f53 *Function
	for _, f := range g.Funcs.Funcs {
		if f != nil && f.Name == "func_53" {
			f53 = f
			break
		}
	}
	if f53 == nil {
		t.Fatal("no func_53")
	}
	if f53.FactChanged {
		t.Fatal("func_53 FactChanged must stay false without body PT updates (params alone)")
	}
	if f53.NeedsRevisit() {
		t.Fatal("func_53 NeedsRevisit must be false so build_invocation uses static feffect")
	}
	has283 := false
	for _, v := range f53.FEffect.ReadVars() {
		if v != nil && v.Name == "g_283" {
			has283 = true
		}
	}
	if !has283 {
		t.Fatal("func_53 FEffect must read g_283 (make_iteration IV)")
	}
	var f14 *Function
	for _, f := range g.Funcs.Funcs {
		if f != nil && f.Name == "func_14" {
			f14 = f
			break
		}
	}
	if f14 == nil {
		t.Fatal("no func_14")
	}
	callerHas := false
	for _, v := range f14.FEffect.ReadVars() {
		if v != nil && v.Name == "g_283" {
			callerHas = true
		}
	}
	if !callerHas {
		t.Fatal("func_14 FEffect must include g_283 from static feffect fold of func_53")
	}
	ClearErrorSess(testAmbientSession)
}
