package csmith

import (
	"testing"
)

func TestLhsWriteVarsFromWritten(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntType(), false, false)
	e := EmptyEffect().WriteVarSess(testAmbientSession, v)
	e = e.SetLhsWriteVarsFromWrittenSess(testAmbientSession)
	got := e.LhsWriteVarsSess(testAmbientSession)
	if len(got) != 1 || got[0] != v {
		t.Fatal(got)
	}
	// IncompleteEffect must not invent empty-complete lhs set sticky
	if VariablesComplete(IncompleteEffect().LhsWriteVarsSess(testAmbientSession)) {
		t.Fatal("IncompleteEffect LhsWriteVars must IncompleteVariables")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IncompleteEffect LhsWriteVars must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	hole := EmptyEffect()
	hole.lhsWrite = map[*Variable]bool{nil: true}
	if VariablesComplete(hole.LhsWriteVarsSess(testAmbientSession)) {
		t.Fatal("nil lhsWrite key must IncompleteVariables")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil lhsWrite key must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !IncompleteEffect().HasGlobalEffectSess(testAmbientSession) {
		t.Fatal("IncompleteEffect must fail closed HasGlobalEffect true")
	}
	if !IncompleteEffect().UnionFieldIsReadSess(testAmbientSession) {
		t.Fatal("IncompleteEffect must fail closed UnionFieldIsRead true")
	}
}

func TestWriteVarSet(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntType(), false, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntType(), false, false)
	e := EmptyEffect().WriteVarSetSess(testAmbientSession, []*Variable{a, b})
	if !e.IsWrittenSess(testAmbientSession, a) || !e.IsWrittenSess(testAmbientSession, b) {
		t.Fatal("writes")
	}
}

func TestAddEffectOptsIncludeLHS(t *testing.T) {
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntType(), false, false)
	other := EmptyEffect().WriteVarSess(testAmbientSession, v).SetLhsWriteVarsFromWrittenSess(testAmbientSession)
	base := EmptyEffect()
	merged := base.AddEffectOptsSess(testAmbientSession, other, true)
	if len(merged.LhsWriteVarsSess(testAmbientSession)) != 1 {
		t.Fatal("lhs not merged")
	}
	merged2 := base.AddEffectOptsSess(testAmbientSession, other, false)
	if len(merged2.LhsWriteVarsSess(testAmbientSession)) != 0 {
		t.Fatal("lhs should skip")
	}
	// nil map key fails closed IncompleteEffect (not invent leave-base complete)
	hole := EmptyEffect()
	hole.written = map[*Variable]bool{nil: true}
	got := base.ReadVarSess(testAmbientSession, v).AddEffectOptsSess(testAmbientSession, hole, false)
	if EffectComplete(got) {
		t.Fatal("nil effect key merge must fail closed incomplete", got)
	}
	if got.IsPureSess(testAmbientSession) || got.IsSideEffectFreeSess(testAmbientSession) || got.IsEmptySess(testAmbientSession) {
		t.Fatal("IncompleteEffect must not invent pure/SE-free/empty", got)
	}
	// HasRaceWith / HasGlobalEffect nil keys fail closed as conflict/global
	if !EmptyEffect().HasRaceWithSess(testAmbientSession, hole) {
		t.Fatal("nil write key must fail closed as race")
	}
	if !hole.HasGlobalEffectSess(testAmbientSession) {
		t.Fatal("nil write key must fail closed as global effect")
	}
	hole.read = map[*Variable]bool{nil: true}
	if !hole.UnionFieldIsReadSess(testAmbientSession) {
		t.Fatal("nil read key must fail closed as union field read")
	}
	ClearErrorSess(testAmbientSession)
	if hole.CommentOutputSess(testAmbientSession) != "" {
		t.Fatal("nil key CommentOutput must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil key CommentOutput must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if EffectComplete(EmptyEffect().WriteVarSetSess(testAmbientSession, []*Variable{v, nil})) {
		t.Fatal("WriteVarSet nil hole must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("WriteVarSet nil hole must SetError sticky")
	}
	// consolidate incomplete → sticky IncompleteEffect
	ClearErrorSess(testAmbientSession)
	c := EmptyEffect()
	c.read = map[*Variable]bool{nil: true}
	c.ConsolidateSess(testAmbientSession)
	if EffectComplete(c) {
		t.Fatal("Consolidate incomplete must yield IncompleteEffect")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Consolidate incomplete must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestVisitFactsLhsSetsLhsWrite(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntType(), false, false)
	lhs := &Lhs{Var: v, Type: GetIntType()}
	cg := EmptyCGContext().WithSession(testAmbientSession)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if !cg.VisitFactsLhs(lhs, Defaults()) {
		t.Fatal("visit")
	}
	if len(cg.EffectAccum.LhsWriteVarsSess(testAmbientSession)) != 1 {
		t.Fatal("lhs write not set", cg.EffectAccum.LhsWriteVarsSess(testAmbientSession))
	}
}

func TestRemoveFunctionLocalFactsIncompletePointToFailClosed(t *testing.T) {
	// soft invent: Clone incomplete PointTo appends nil / keeps partial out
	ClearErrorSess(testAmbientSession)
	fn := &Function{Name: "f", ReturnType: GetIntType()}
	body := &Block{Func: fn}
	fn.Body = body
	fn.Blocks = []*Block{body}
	gp := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerTo(GetIntType()), true, false)
	facts := []*FactPointTo{{Var: gp, PointTo: []*Variable{nil}}}
	if FactsComplete(RemoveFunctionLocalFacts(facts, fn)) {
		t.Fatal("incomplete PointTo must fail closed incomplete, not invent filter")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete PointTo RemoveFunctionLocalFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRemoveFunctionLocalFacts(t *testing.T) {
	// FactMgr.cpp:191 — is_var_on_stack(v, stm) via Body as function-exit parent
	f := &Function{Name: "f"}
	loc := CreateVariableScalarsSess(testAmbientSession, "l_1", PointerTo(GetIntType()), false, false)
	g := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerTo(GetIntType()), false, false)
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

func TestRemoveFunctionLocalFactsNilFuncNoResidualInvent(t *testing.T) {
	// Soft invent: MarkFuncEnd(nil) residual ERROR then SetMapFactsOut incomplete
	// while GlobalFacts complete invents visit success past residual.
	// Fair: nil Func is complete no-op mark; filter stays complete non-sticky.
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	g := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerTo(GetIntType()), false, false)
	facts := []*FactPointTo{MakeFactPointTo(g, NullPtr)}
	out := RemoveFunctionLocalFactsAt(facts, nil, nil)
	if !FactsComplete(out) || len(out) != 1 {
		t.Fatal("nil Func RemoveFunctionLocalFactsAt must stay complete", out)
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("nil Func RemoveFunctionLocalFactsAt must stay non-sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRemoveLoopLocalFacts(t *testing.T) {
	outer := &Block{Looping: true}
	inner := &Block{Parent: outer, LocalVars: []*Variable{
		CreateVariableScalarsSess(testAmbientSession, "l_i", GetIntType(), false, false),
	}}
	lp := CreateVariableScalarsSess(testAmbientSession, "l_p", PointerTo(GetIntType()), false, false)
	inner.LocalVars = append(inner.LocalVars, lp)
	g := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerTo(GetIntType()), false, false)
	facts := []*FactPointTo{MakeFactPointTo(lp, NullPtr), MakeFactPointTo(g, NullPtr)}
	out := RemoveLoopLocalFacts(facts, inner)
	if len(out) != 1 || out[0].Var != g {
		t.Fatal(out)
	}
}

func TestRemoveLoopLocalFactsMarksDeadPointee(t *testing.T) {
	// FactMgr.cpp:601–612 — update_facts_for_oos_vars marks pointees garbage
	loop := &Block{Looping: true}
	loc := CreateVariableScalarsSess(testAmbientSession, "l_t", GetIntType(), false, false)
	loop.LocalVars = []*Variable{loc}
	g := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerTo(GetIntType()), false, false)
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
		CreateVariableScalarsSess(testAmbientSession, "l_iv", GetIntType(), false, false),
	}}
	body := &Block{Parent: loop, LocalVars: []*Variable{
		CreateVariableScalarsSess(testAmbientSession, "l_tmp", GetIntType(), false, false),
	}}
	lp := CreateVariableScalarsSess(testAmbientSession, "l_p", PointerTo(GetIntType()), false, false)
	body.LocalVars = append(body.LocalVars, lp)
	g := CreateVariableScalarsSess(testAmbientSession, "g_q", PointerTo(GetIntType()), false, false)
	br := &Stmt{Kind: StmtBreak, StmID: 3}
	facts := []*FactPointTo{MakeFactPointTo(lp, NullPtr), MakeFactPointTo(g, NullPtr)}
	out := RemoveLoopLocalFactsForStmt(facts, br, body)
	if len(out) != 1 || out[0].Var != g {
		t.Fatal(out)
	}
}

func TestGetDereferencedPtrs(t *testing.T) {
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerTo(GetIntType()), false, false)
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
	// incomplete IR must IncompleteExpressions sticky (not bare nil invent empty-complete)
	ClearErrorSess(testAmbientSession)
	cases := []*Expression{
		{Term: TermVariable},
		{Term: TermCommaExpr},
		{Term: TermFunction},
		{Term: TermFunction, Invoke: &Invocation{
			Args: []*Expression{{Term: TermConstant, Con: MakeInt(1)}, nil},
		}},
		{Term: TermAssignment},
		nil,
	}
	for _, e := range cases {
		ClearErrorSess(testAmbientSession)
		if ExpressionsComplete(GetDereferencedPtrs(e)) {
			t.Fatalf("incomplete deref must IncompleteExpressions, got complete for %#v", e)
		}
		if !HasErrorSess(testAmbientSession) {
			t.Fatalf("incomplete deref must SetError sticky for %#v", e)
		}
	}
	ClearErrorSess(testAmbientSession)
}
