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
	// nil map key fails closed — leave base unchanged
	hole := EmptyEffect()
	hole.written = map[*Variable]bool{nil: true}
	got := base.ReadVar(v).AddEffectOpts(hole, false)
	if !got.IsRead(v) || got.IsWritten(v) {
		t.Fatal("nil effect key must not invent partial merge", got)
	}
	// HasRaceWith / HasGlobalEffect nil keys fail closed as conflict/global
	if !EmptyEffect().HasRaceWith(hole) {
		t.Fatal("nil write key must fail closed as race")
	}
	if !hole.HasGlobalEffect() {
		t.Fatal("nil write key must fail closed as global effect")
	}
	hole.read = map[*Variable]bool{nil: true}
	if !hole.UnionFieldIsRead() {
		t.Fatal("nil read key must fail closed as union field read")
	}
	if hole.CommentOutput() != "" {
		t.Fatal("nil key CommentOutput must fail closed empty")
	}
	if EmptyEffect().WriteVarSet([]*Variable{v, nil}).IsWritten(v) {
		t.Fatal("WriteVarSet nil hole must leave base unchanged")
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
	// FactMgr.cpp:191 — is_var_on_stack(v, stm) via Body as function-exit parent
	f := &Function{Name: "f"}
	loc := CreateVariableScalars("l_1", PointerTo(GetIntType()), false, false)
	g := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	body := &Block{Func: f, LocalVars: []*Variable{loc}}
	f.Blocks = []*Block{body}
	f.Body = body
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

func TestRemoveLoopLocalFactsMarksDeadPointee(t *testing.T) {
	// FactMgr.cpp:601–612 — update_facts_for_oos_vars marks pointees garbage
	loop := &Block{Looping: true}
	loc := CreateVariableScalars("l_t", GetIntType(), false, false)
	loop.LocalVars = []*Variable{loc}
	g := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	facts := []*FactPointTo{MakeFactPointTo(g, loc)}
	out := RemoveLoopLocalFacts(facts, loop)
	if len(out) != 1 || out[0].Var != g {
		t.Fatal(out)
	}
	if !out[0].IsDead() {
		t.Fatalf("pointee to loop-local must be garbage: %+v", out[0])
	}
}

func TestRemoveLoopLocalFactsForStmtUsesParent(t *testing.T) {
	loop := &Block{Looping: true, LocalVars: []*Variable{
		CreateVariableScalars("l_iv", GetIntType(), false, false),
	}}
	body := &Block{Parent: loop, LocalVars: []*Variable{
		CreateVariableScalars("l_tmp", GetIntType(), false, false),
	}}
	lp := CreateVariableScalars("l_p", PointerTo(GetIntType()), false, false)
	body.LocalVars = append(body.LocalVars, lp)
	g := CreateVariableScalars("g_q", PointerTo(GetIntType()), false, false)
	br := &Stmt{Kind: StmtBreak, StmID: 3}
	facts := []*FactPointTo{MakeFactPointTo(lp, NullPtr), MakeFactPointTo(g, NullPtr)}
	out := RemoveLoopLocalFactsForStmt(facts, br, body)
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
	if bareOut := GetDereferencedPtrs(bare); bareOut == nil || len(bareOut) != 0 {
		t.Fatal("no deref complete empty", bareOut)
	}
}

func TestGetDereferencedPtrsIncompleteFailClosed(t *testing.T) {
	// incomplete IR must not invent empty deref list as complete
	if GetDereferencedPtrs(&Expression{Term: TermVariable}) != nil {
		t.Fatal("nil Var must fail closed")
	}
	if GetDereferencedPtrs(&Expression{Term: TermCommaExpr}) != nil {
		t.Fatal("nil comma sides must fail closed")
	}
	if GetDereferencedPtrs(&Expression{Term: TermFunction}) != nil {
		t.Fatal("nil Invoke must fail closed")
	}
	if GetDereferencedPtrs(&Expression{Term: TermFunction, Invoke: &Invocation{
		Args: []*Expression{{Term: TermConstant, Con: MakeInt(1)}, nil},
	}}) != nil {
		t.Fatal("nil arg hole must fail closed")
	}
	if GetDereferencedPtrs(&Expression{Term: TermAssignment}) != nil {
		t.Fatal("nil Assign must fail closed")
	}
}
