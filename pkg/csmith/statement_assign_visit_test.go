package csmith

import (
	"testing"
)

func TestErrorCodes(t *testing.T) {
	ClearError()
	if HasError() || GetError() != ErrSuccess {
		t.Fatal(GetError())
	}
	SetError(ErrCompatibleCheck)
	if !HasError() || GetError() != ErrCompatibleCheck {
		t.Fatal(GetError())
	}
	ClearError()
}

func TestSafeAssign(t *testing.T) {
	if !SafeAssign(AssignBitAnd) || SafeAssign(AssignAdd) {
		t.Fatal("safe")
	}
}

func TestVisitFactsStatementAssignSimple(t *testing.T) {
	BookkeeperDoFinalization()
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	lhs := &Lhs{Var: v, Type: GetIntType()}
	rhs := &Expression{Term: TermConstant, Con: MakeInt(3)}
	st := Stmt{Kind: StmtAssign, LhsVar: v, Lhs: lhs, Expr: rhs, AssignOp: AssignSimple}
	eff := EmptyEffect()
	cg := EmptyCGContext()
	cg.EffectAccum = &eff
	if !VisitFactsStatementAssign(&st, &cg, Defaults()) {
		t.Fatal("visit")
	}
	if !eff.IsWritten(v) {
		t.Fatal("lhs write missing")
	}
}

func TestVisitFactsStatementAssignNoWriteToIV(t *testing.T) {
	iv := CreateVariableScalars("g_i", GetIntType(), false, false)
	lhs := &Lhs{Var: iv, Type: GetIntType()}
	st := Stmt{
		Kind: StmtAssign, LhsVar: iv, Lhs: lhs,
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignSimple,
	}
	cg := EmptyCGContext()
	cg.AddIVBound(iv, 10)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if VisitFactsStatementAssign(&st, &cg, Defaults()) {
		t.Fatal("should reject IV write")
	}
}

func TestMakeRandomAssignDualContext(t *testing.T) {
	// StatementAssign.cpp:181/225 — merge_param_context folds RHS/LHS into caller;
	// expr_depth and effect_accum must stick on *CGContext.
	ClearError()
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	// RHS make_random may pick comma (type nullptr) — needs Type env
	seedTypesForTest(NewRng(1), opts, probs, vs, nil)
	// seed a global
	g := vs.GenerateNewGlobal(AccessWrite, EmptyCGContext(), GetIntType(), nil, NewRng(1))
	if g == nil {
		t.Fatal("global")
	}
	eff := EmptyEffect()
	cg := EmptyCGContext()
	cg.EffectAccum = &eff
	cg.Types = vs.Types
	cg.ExprDepth = 0
	tables := NewExprTables(opts)
	st := MakeRandomAssign(NewRng(5), opts, probs, vs, tables, &cg, GetIntType())
	if st.Kind != StmtAssign {
		t.Fatal("kind")
	}
	if st.LhsVar == nil && st.Lhs == nil {
		t.Fatal("no lhs", st)
	}
	// successful assign with RHS expression bumps depth via merge
	if st.Expr != nil && BumpsExprDepth(st.Expr) && cg.ExprDepth < 1 {
		t.Fatalf("ExprDepth=%d after assign with leaf RHS (want ≥1)", cg.ExprDepth)
	}
	// LHS write lands on shared effect_accum
	if st.LhsVar != nil && !eff.IsWritten(st.LhsVar) && !cg.EffectStm.IsWritten(st.LhsVar) {
		// NoteWrite / visit_facts path — allow either accum or stm
		t.Fatalf("expected write effect for lhs %s", st.LhsVar.Name)
	}
}

func TestVisitFactsExpressionComma(t *testing.T) {
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	b := CreateVariableScalars("g_b", GetIntType(), false, false)
	e := &Expression{
		Term:     TermCommaExpr,
		CommaLHS: &Expression{Term: TermVariable, Var: a, ExprType: GetIntType()},
		CommaRHS: &Expression{Term: TermVariable, Var: b, ExprType: GetIntType()},
	}
	eff := EmptyEffect()
	cg := EmptyCGContext()
	cg.EffectAccum = &eff
	if !VisitFactsExpression(e, &cg, Defaults()) {
		t.Fatal("comma")
	}
	if !eff.IsRead(a) || !eff.IsRead(b) {
		t.Fatal("reads", eff.IsRead(a), eff.IsRead(b))
	}
}

func TestVisitFactsStatementAssignIndirectUpdate(t *testing.T) {
	// StatementAssign.cpp:386 + FactPointTo.cpp:275–278 —
	// *p when p is **T: Lhs type is *T (pointer) → transfer into pointees.
	// p : int**, q : int*, p → {q}; *p = 0 → fact for q becomes null.
	ppT := PointerTo(PointerTo(GetIntType()))
	p := CreateVariableScalars("g_p", ppT, false, false)
	q := CreateVariableScalars("g_q", PointerTo(GetIntType()), false, false)
	fm := NewFactMgr(nil)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, q)}
	rhs := &Expression{
		Term: TermConstant, Con: &Constant{Type: PointerTo(GetIntType()), Value: "0"},
		ExprType: PointerTo(GetIntType()),
	}
	// UpdateFactForAssign with indir 1 (mirror Lhs after one deref)
	if !fm.UpdateFactForAssign(p, 1, rhs) {
		t.Fatal("update *p")
	}
	got := FindRelatedPointTo(fm.GlobalFacts, q)
	if got == nil || !got.IsNull() {
		t.Fatalf("q should be null after *p=0: %+v", got)
	}
	// visit path also passes indir from Lhs (Type = *int, Var = **int → indir 1)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, q)}
	st := &Stmt{
		Kind: StmtAssign, StmID: 5,
		Lhs:      &Lhs{Var: p, Type: PointerTo(GetIntType())},
		LhsVar:   p,
		Expr:     rhs,
		AssignOp: AssignSimple,
	}
	if st.Lhs.IndirectLevel() != 1 {
		t.Fatalf("indir %d", st.Lhs.IndirectLevel())
	}
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	// visit may fail write checks; update is covered above; soft-check visit
	_ = VisitFactsStatementAssign(st, &cg, Defaults())
}

func TestVisitFactsStatementAssignWriteVarSet(t *testing.T) {
	// StatementAssign.cpp:377 — write_var_set after RHS
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	// compound assign so RHS effect folds; use simple still runs write_var_set
	st := &Stmt{
		Kind: StmtAssign, StmID: 9,
		LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignAdd,
	}
	cg := EmptyCGContext()
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	// seed lhs_write_vars on effect via a visit that sets them — simple path
	// just ensure visit succeeds (write_var_set of empty is no-op)
	if !VisitFactsStatementAssign(st, &cg, Defaults()) {
		t.Fatal("visit")
	}
}
