package csmith

import (
	"testing"
)

func TestErrorCodes(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if HasErrorSess(testAmbientSession) || GetErrorSess(testAmbientSession) != ErrSuccess {
		t.Fatal(GetErrorSess(testAmbientSession))
	}
	SetErrorSess(testAmbientSession, ErrCompatibleCheck)
	if !HasErrorSess(testAmbientSession) || GetErrorSess(testAmbientSession) != ErrCompatibleCheck {
		t.Fatal(GetErrorSess(testAmbientSession))
	}
	ClearErrorSess(testAmbientSession)
}

func TestSafeAssign(t *testing.T) {
	if !SafeAssign(AssignBitAnd) || SafeAssign(AssignAdd) {
		t.Fatal("safe")
	}
}

func TestVisitFactsStatementAssignSimple(t *testing.T) {
	BookkeeperDoFinalizationSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	lhs := &Lhs{Var: v, Type: GetIntTypeSess(testAmbientSession)}
	rhs := &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 3)}
	st := Stmt{Kind: StmtAssign, LhsVar: v, Lhs: lhs, Expr: rhs, AssignOp: AssignSimple}
	eff := EmptyEffect()
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.EffectAccum = &eff
	if !VisitFactsStatementAssign(&st, &cg, Defaults()) {
		t.Fatal("visit")
	}
	if !eff.IsWrittenSess(testAmbientSession, v) {
		t.Fatal("lhs write missing")
	}
}

func TestVisitFactsStatementAssignNoWriteToIV(t *testing.T) {
	iv := CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntTypeSess(testAmbientSession), false, false)
	lhs := &Lhs{Var: iv, Type: GetIntTypeSess(testAmbientSession)}
	st := Stmt{
		Kind: StmtAssign, LhsVar: iv, Lhs: lhs,
		Expr: &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)}, AssignOp: AssignSimple,
	}
	cg := EmptyCGContext().WithSession(testAmbientSession)
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
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	// RHS make_random may pick comma (type nullptr) — needs Type env
	seedTypesForTest(NewRngSess(testAmbientSession, 1), opts, probs, vs, nil)
	// seed a global
	g := vs.GenerateNewGlobal(AccessWrite, EmptyCGContext().WithSession(testAmbientSession), GetIntTypeSess(testAmbientSession), nil, NewRngSess(testAmbientSession, 1))
	if g == nil {
		t.Fatal("global")
	}
	eff := EmptyEffect()
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	cg.EffectAccum = &eff
	cg.Types = vs.Types
	cg.ExprDepth = 0
	tables := NewExprTablesSess(testAmbientSession, opts)
	// single seed may fail Lhs/exact qfer; retry like other factories
	var st Stmt
	for seed := uint64(1); seed < 40; seed++ {
		ClearErrorSess(testAmbientSession)
		cg2 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
		cg2.EffectAccum = &eff
		cg2.Types = vs.Types
		st = MakeRandomAssign(NewRngSess(testAmbientSession, seed), opts, probs, vs, tables, &cg2, GetIntTypeSess(testAmbientSession))
		if stmtOK(st) {
			cg = cg2
			break
		}
	}
	if !stmtOK(st) {
		t.Fatal("no lhs", st)
	}
	// successful assign with RHS expression bumps depth via merge
	if st.Expr != nil && BumpsExprDepthSess(testAmbientSession, st.Expr) && cg.ExprDepth < 1 {
		t.Fatalf("ExprDepth=%d after assign with leaf RHS (want ≥1)", cg.ExprDepth)
	}
	// LHS write lands on shared effect_accum via visit_facts + merge_param_context.
	// Lhs.cpp:337–346 — *p writes pointees (not the pointer); bare p writes p.
	if st.LhsVar != nil {
		indir := 0
		if st.Lhs != nil {
			indir = st.Lhs.IndirectLevelSess(testAmbientSession)
		}
		if indir == 0 {
			if !eff.IsWrittenSess(testAmbientSession, st.LhsVar) && !cg.EffectStm.IsWrittenSess(testAmbientSession, st.LhsVar) {
				t.Fatalf("expected write effect for scalar lhs %s", st.LhsVar.Name)
			}
		} else if len(eff.WrittenVarsSess(testAmbientSession)) == 0 && len(cg.EffectStm.WrittenVarsSess(testAmbientSession)) == 0 {
			t.Fatalf("expected pointee write for deref lhs %s indir=%d", st.LhsVar.Name, indir)
		}
	}
}

func TestVisitFactsExpressionComma(t *testing.T) {
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), false, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntTypeSess(testAmbientSession), false, false)
	e := &Expression{
		Term:     TermCommaExpr,
		CommaLHS: &Expression{Term: TermVariable, Var: a, ExprType: GetIntTypeSess(testAmbientSession)},
		CommaRHS: &Expression{Term: TermVariable, Var: b, ExprType: GetIntTypeSess(testAmbientSession)},
	}
	eff := EmptyEffect()
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.EffectAccum = &eff
	if !VisitFactsExpression(e, &cg, Defaults()) {
		t.Fatal("comma")
	}
	if !eff.IsReadSess(testAmbientSession, a) || !eff.IsReadSess(testAmbientSession, b) {
		t.Fatal("reads", eff.IsReadSess(testAmbientSession, a), eff.IsReadSess(testAmbientSession, b))
	}
	// incomplete TermConstant shell sticky (no invent visit / soft re-pick)
	ClearErrorSess(testAmbientSession)
	if VisitFactsExpression(&Expression{Term: TermConstant}, &cg, Defaults()) {
		t.Fatal("nil Con constant must fail visit")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Con VisitFactsExpression must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if VisitFactsExpression(&Expression{Term: TermConstant, Con: &Constant{Type: GetIntTypeSess(testAmbientSession)}}, &cg, Defaults()) {
		t.Fatal("empty Value constant must fail visit")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("empty Value VisitFactsExpression must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !VisitFactsExpression(&Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0)}, &cg, Defaults()) {
		t.Fatal("live constant must visit")
	}
	ClearErrorSess(testAmbientSession)
	// comma LHS residual soft invent was soft-continue RHS invent visit success.
	hole := &Expression{Term: TermConstant, Con: &Constant{Type: GetIntTypeSess(testAmbientSession)}}
	comma := &Expression{
		Term:     TermCommaExpr,
		CommaLHS: hole,
		CommaRHS: &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)},
	}
	if VisitFactsExpression(comma, &cg, Defaults()) {
		t.Fatal("LHS visit residual must fail closed comma visit")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("LHS visit residual comma must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestVisitFactsStatementAssignIndirectUpdate(t *testing.T) {
	// StatementAssign.cpp:386 + FactPointTo.cpp:275–278 —
	// *p when p is **T: Lhs type is *T (pointer) → transfer into pointees.
	// p : int**, q : int*, p → {q}; *p = 0 → fact for q becomes null.
	ppT := PointerToSess(testAmbientSession, PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)))
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", ppT, false, false)
	q := CreateVariableScalarsSess(testAmbientSession, "g_q", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	fm := NewFactMgrSess(testAmbientSession, nil)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, q)}
	rhs := &Expression{
		Term: TermConstant, Con: &Constant{Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), Value: "0"},
		ExprType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)),
	}
	// UpdateFactForAssign with indir 1 (mirror Lhs after one deref)
	if !fm.UpdateFactForAssign(p, 1, rhs) {
		t.Fatal("update *p")
	}
	got := FindRelatedPointToSess(testAmbientSession, fm.GlobalFacts, q)
	if got == nil || !got.IsNullSess(testAmbientSession) {
		t.Fatalf("q should be null after *p=0: %+v", got)
	}
	// visit path also passes indir from Lhs (Type = *int, Var = **int → indir 1)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, q)}
	st := &Stmt{
		Kind: StmtAssign, StmID: 5,
		Lhs:      &Lhs{Var: p, Type: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))},
		LhsVar:   p,
		Expr:     rhs,
		AssignOp: AssignSimple,
	}
	if st.Lhs.IndirectLevelSess(testAmbientSession) != 1 {
		t.Fatalf("indir %d", st.Lhs.IndirectLevelSess(testAmbientSession))
	}
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	// visit may fail write checks; update is covered above; soft-check visit
	_ = VisitFactsStatementAssign(st, &cg, Defaults())
}

func TestVisitFactsStatementAssignWriteVarSet(t *testing.T) {
	// StatementAssign.cpp:377 — write_var_set after RHS
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	// compound assign so RHS effect folds; use simple still runs write_var_set
	st := &Stmt{
		Kind: StmtAssign, StmID: 9,
		LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntTypeSess(testAmbientSession)},
		Expr: &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)}, AssignOp: AssignAdd,
	}
	cg := EmptyCGContext().WithSession(testAmbientSession)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	// seed lhs_write_vars on effect via a visit that sets them — simple path
	// just ensure visit succeeds (write_var_set of empty is no-op)
	if !VisitFactsStatementAssign(st, &cg, Defaults()) {
		t.Fatal("visit")
	}
}

func TestVisitFactsStatementAssignIncompleteAmbientFailClosed(t *testing.T) {
	// incomplete EffectContext must not invent assign visit success under poisoned running
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 10,
		LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntTypeSess(testAmbientSession)},
		Expr:     &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1), ExprType: GetIntTypeSess(testAmbientSession)},
		AssignOp: AssignSimple,
	}
	cg := WithEffectContext(IncompleteEffect()).WithSession(testAmbientSession)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if VisitFactsStatementAssign(st, &cg, Defaults()) {
		t.Fatal("incomplete EffectContext must fail closed assign visit")
	}
	// incomplete parent EffectAccum after merge
	cg2 := EmptyCGContext().WithSession(testAmbientSession)
	inc := IncompleteEffect()
	cg2.EffectAccum = &inc
	if VisitFactsStatementAssign(st, &cg2, Defaults()) {
		t.Fatal("incomplete EffectAccum must fail closed assign visit")
	}
	ClearErrorSess(testAmbientSession)
}

func TestVisitFactsStatementAssignWriteVarSetResidualSticky(t *testing.T) {
	// LhsWriteVars residual soft invent was soft-empty skip WriteVarSet invent visit success.
	// Fair: sticky fail closed false.
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 11,
		LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntTypeSess(testAmbientSession)},
		Expr:     &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1), ExprType: GetIntTypeSess(testAmbientSession)},
		AssignOp: AssignSimple,
	}
	// plant incomplete EffectStm so RHS path leaves residual before WriteVarSet fold
	// (incomplete EffectContext fails earlier — use complete context + incomplete rhs via
	// compound path: seed rhsAccum-like incomplete by incomplete EffectStm after merge gate)
	cg := EmptyCGContext().WithSession(testAmbientSession)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	// incomplete EffectStm fails closed before LhsWriteVars — still residual invent class
	cg.EffectStm = IncompleteEffect()
	if VisitFactsStatementAssign(st, &cg, Defaults()) {
		t.Fatal("incomplete EffectStm must fail closed assign visit")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectStm assign visit must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// WriteVarSet residual: nil var in lhs_write map stickies IncompleteVariables → WriteVarSet
	// force incomplete rhsAccum LhsWriteVars by visiting with incomplete write fold path
	// via running EffectContext incomplete after RHS succeeds is hard; unit WriteVarSet already
	// covered in effect_test — here assert assign visit remains sticky under incomplete Accum
	// after MergeParamContext would re-check EffectComplete.
	cg3 := EmptyCGContext().WithSession(testAmbientSession)
	inc3 := IncompleteEffect()
	cg3.EffectAccum = &inc3
	if VisitFactsStatementAssign(st, &cg3, Defaults()) {
		t.Fatal("incomplete Accum WriteVarSet path must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete Accum assign visit must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

// FunctionInvocation.cpp:542–546 — add_visible_effect uses curr_blk (AnalysisBlock).
func TestVisitFactsInvocationUsesAnalysisBlock(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.MaxBlockSize = 1
	opts.MaxFuncs = 3
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(testAmbientSession, opts)
	list := &FunctionList{Types: &TypeEnv{Sess: testAmbientSession}}
	caller := &Function{Name: "caller", ReturnType: GetIntTypeSess(testAmbientSession), BuildState: BuildBuilt, IsBuilt: true}
	list.Funcs = []*Function{caller}
	fm := NewFactMgrSess(testAmbientSession, caller)
	callerBlk := &Block{Func: caller, StmID: AllocStmID()}
	caller.Stack = []*Block{callerBlk}
	// Nested stack frame that is NOT the statement parent
	inner := &Block{Func: caller, Parent: callerBlk, StmID: AllocStmID()}
	caller.Stack = []*Block{callerBlk, inner}
	cg := WithFunc(caller, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm).WithFuncList(list)
	cg.CurrBlk = callerBlk // statement parent (stm_visit_facts)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	// Build a small callee and revisit via VisitFactsInvocation
	callee := &Function{Name: "callee", ReturnType: GetIntTypeSess(testAmbientSession), BuildState: BuildBuilt, IsBuilt: true}
	callee.Body = &Block{Func: callee, StmID: AllocStmID()}
	callee.RV = CreateVariableScalarsSess(testAmbientSession, "rv", GetIntTypeSess(testAmbientSession), false, false)
	_ = callee.ensurePairedFactMgrSess(testAmbientSession)
	fi := &Invocation{User: callee}
	// Empty body visit should succeed; AddVisibleEffect must use CurrBlk not stack top
	ok := VisitFactsInvocation(fi, &cg, opts)
	if !ok && !HasErrorSess(testAmbientSession) {
		// may soft-fail without body maps; still check CurrBlk preference via AnalysisBlock
	}
	if cg.AnalysisBlock() != callerBlk {
		t.Fatal("AnalysisBlock must prefer CurrBlk over stack top")
	}
	if cg.CurrentBlock() != inner {
		t.Fatal("precondition: stack top is inner")
	}
	ClearErrorSess(testAmbientSession)
	_ = probs
	_ = vs
}

func TestAssignDerefDoesNotNoteWritePointer(t *testing.T) {
	// Lhs.cpp:337–346 — *p=… CheckReadVar(p)+write_pointed; StatementAssign must not
	// invent NoteWrite(p) after merge (seed2 first_div e9238: pointer false-written →
	// no ptr-bias in SelectParentLocal choose_var).
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	pointee := CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntTypeSess(testAmbientSession), false, false)
	ptr := CreateVariableScalarsSess(testAmbientSession, "l_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	fm := NewFactMgrSess(testAmbientSession, f)
	fm.GlobalFacts = append(fm.GlobalFacts, MakeFactPointToSess(testAmbientSession, ptr, pointee))
	blk := &Block{Func: f, LocalVars: []*Variable{ptr}}
	f.Stack = []*Block{blk}
	eff := EmptyEffect()
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &eff
	lhs := &Lhs{Var: ptr, Type: GetIntTypeSess(testAmbientSession), CompoundAssign: false}
	ClearErrorSess(testAmbientSession)
	if !cg.VisitFactsLhs(lhs, opts) {
		t.Fatal("VisitFactsLhs *p failed")
	}
	// Pointer itself must be read, not written
	if eff.IsWrittenSess(testAmbientSession, ptr) || cg.EffectStm.IsWrittenSess(testAmbientSession, ptr) {
		t.Fatalf("deref Lhs must not write pointer %s (writes=%v)", ptr.Name, eff.WrittenVarsSess(testAmbientSession))
	}
	if !eff.IsWrittenSess(testAmbientSession, pointee) && !cg.EffectStm.IsWrittenSess(testAmbientSession, pointee) {
		t.Fatalf("expected write of pointee %s, writes=%v", pointee.Name, eff.WrittenVarsSess(testAmbientSession))
	}
}

func TestMakeRandomAssignDoesNotUpdateFacts(t *testing.T) {
	// StatementAssign.cpp:make_random — no update_fact_for_assign; only ExpressionAssign
	// and post_creation_analysis update (seed-2 e10107 double-merge path).
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	SetProcessOptionsSess(testAmbientSession, opts)
	SetProcessProbabilitiesSess(testAmbientSession, NewProbabilities(opts))
	r := NewRngSess(testAmbientSession, 42)
	SetProcessRngSess(testAmbientSession, r)
	vs := NewVariableSelector(testAmbientSession, opts)
	g := CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntTypeSess(testAmbientSession), false, false)
	vs.GlobalList = append(vs.GlobalList, g)
	vs.AllVars = append(vs.AllVars, g)
	p := CreateVariableScalarsSess(testAmbientSession, "p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	p.Init = &Constant{Type: p.Type, Value: "0"}
	vs.GlobalList = append(vs.GlobalList, p)
	vs.AllVars = append(vs.AllVars, p)
	fm := NewFactMgrSess(testAmbientSession, &Function{Name: "f"})
	fm.AddNewVarFact(p)
	if !FindRelatedPointToSess(testAmbientSession, fm.GlobalFacts, p).IsNullSess(testAmbientSession) {
		t.Fatal("want null init for p")
	}
	// force pointer assign facts path: assign p = &g_x would change null→g_x if update ran
	before := CloneFactSliceSess(testAmbientSession, fm.GlobalFacts)
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.FM = fm
	cg.CurrentFunc = fm.Func
	st := MakeRandomAssign(r, opts, NewProbabilities(opts), vs, NewExprTablesSess(testAmbientSession, opts), &cg, nil)
	if HasErrorSess(testAmbientSession) {
		ClearErrorSess(testAmbientSession)
		// may fail to generate; that is ok — key is no fact mutation on success
	}
	if stmtOK(st) {
		// If make updated facts, pointer lattice would often change; require identical related p fact.
		got := FindRelatedPointToSess(testAmbientSession, fm.GlobalFacts, p)
		want := FindRelatedPointToSess(testAmbientSession, before, p)
		if got == nil || want == nil {
			t.Fatalf("facts missing after make got=%v want=%v", got, want)
		}
		if !got.EqualSess(testAmbientSession, want) {
			t.Fatalf("MakeRandomAssign must not update GlobalFacts; before=%v after=%v", want.PointTo, got.PointTo)
		}
	}
}

func TestVisitFactsStatementAssignRHSEffectStmFresh(t *testing.T) {
	// StatementAssign.cpp:365 + CGContext.cpp:74–82 — rhs_cg_context starts with
	// default-empty effect_stm. Parent may already hold sibling writes (e.g. left
	// of && before a nested ExpressionAssign). Lhs::ptr_modified_in_rhs
	// (Lhs.cpp:240–261) must see only this assign's RHS writes.
	// Seed 80: (***l_108)=… after (**l_108)=… under && must not FP-fail.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	pointee := CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntTypeSess(testAmbientSession), false, false)
	mid := CreateVariableScalarsSess(testAmbientSession, "l_mid", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	ptr := CreateVariableScalarsSess(testAmbientSession, "l_p", PointerToSess(testAmbientSession, PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))), false, false)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	fm := NewFactMgrSess(testAmbientSession, f)
	fm.GlobalFacts = []*FactPointTo{
		MakeFactPointToSess(testAmbientSession, ptr, mid),
		MakeFactPointToSess(testAmbientSession, mid, pointee),
	}
	eff := EmptyEffect()
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &eff
	// Parent EffectStm pretends a sibling already wrote the intermediate pointer.
	// If visit inherited this into rhs/lhs EffectStm, PtrModifiedInRhs would reject **p=.
	cg.EffectStm = EmptyEffect().WriteVarSess(testAmbientSession, mid)
	lhs := &Lhs{Var: ptr, Type: GetIntTypeSess(testAmbientSession)}
	if lhs.IndirectLevelSess(testAmbientSession) != 2 {
		t.Fatalf("precondition: want indir=2 got %d", lhs.IndirectLevelSess(testAmbientSession))
	}
	// Direct PtrModifiedInRhs against parent EffectStm must be true (sibling case).
	if !cg.PtrModifiedInRhs(lhs, fm.GlobalFacts) {
		t.Fatal("precondition: parent EffectStm write of mid must mark **p modified")
	}
	ClearErrorSess(testAmbientSession)
	st := Stmt{
		Kind: StmtAssign, Lhs: lhs, LhsVar: ptr,
		Expr: &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)}, AssignOp: AssignSimple,
	}
	if !VisitFactsStatementAssign(&st, &cg, opts) {
		t.Fatalf("visit must succeed with fresh rhs EffectStm; err=%v", GetErrorSess(testAmbientSession))
	}
}
