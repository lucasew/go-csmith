package csmith

import "testing"

// TestPostCreationStripsLoopBodyAfterIfNullElseDeref — Block.cpp:696–714 +
// StatementIf.cpp:182–196 merge + Lhs.cpp/ExpressionVariable.cpp is_valid_ptr.
//
// Looping body (no must_return): then *g99=(void*)0 renews g77→null; else
// (*g77)^=…; fall-through. If-merge may-nulls g77; self-back re-entry fails
// is_valid_ptr on else Lhs; post_creation FP strips from the if onward.
//
// Seed 17809409409875472624 wraps the same null/else-deref pattern inside a
// nested for whose body must_return (StatementFor does not override
// must_return), so that for is not is_loop_body and does not self-back-strip;
// outer strip is a separate residual. This test pins the unwrapped contract.
func TestPostCreationStripsLoopBodyAfterIfNullElseDeref(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	SetProcessOptionsSess(testAmbientSession, opts)
	SetProcessProbabilitiesSess(testAmbientSession, NewProbabilities(opts))
	SetProcessRngSess(testAmbientSession, NewRng(1))

	i32 := GetIntType()
	pt := PointerTo(i32)
	ppt := PointerTo(pt)
	g77 := CreateVariableScalars("g_77", pt, false, false)
	g99 := CreateVariableScalars("g_99", ppt, false, false)
	tgt := CreateVariableScalars("g_18", i32, false, false)

	f := &Function{Name: "f", ReturnType: i32}
	fm := NewFactMgrSess(testAmbientSession, f)

	// then: *g99 = (void*)0  — FactPointTo.cpp:275–278 + FactMgr renew
	thenNull := Stmt{
		Kind: StmtAssign, StmID: AllocStmID(),
		LhsVar: g99, Lhs: &Lhs{Var: g99, Type: pt},
		Expr:     &Expression{Term: TermConstant, Con: &Constant{Type: pt, Value: "0"}, ExprType: pt},
		AssignOp: AssignSimple,
	}
	// else: (*g77) = 1 — Lhs subject g77, desired type after one deref is i32
	elseDeref := Stmt{
		Kind: StmtAssign, StmID: AllocStmID(),
		LhsVar: g77, Lhs: &Lhs{Var: g77, Type: i32},
		Expr:     &Expression{Term: TermConstant, Con: MakeInt(1), ExprType: i32},
		AssignOp: AssignSimple,
	}
	ifSt := Stmt{
		Kind: StmtIfElse, StmID: AllocStmID(),
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1), ExprType: i32},
		Then: &Block{StmID: AllocStmID(), Func: f, Stmts: []Stmt{thenNull}},
		Else: &Block{StmID: AllocStmID(), Func: f, Stmts: []Stmt{elseDeref}},
	}
	for _, id := range []int{thenNull.StmID, elseDeref.StmID, ifSt.StmID, ifSt.Then.StmID, ifSt.Else.StmID} {
		fm.SetMapStmEffect(id, EmptyEffect())
	}

	body := &Block{
		Func: f, StmID: AllocStmID(), Looping: true,
		Stmts: []Stmt{ifSt},
	}
	fm.SetMapStmEffect(body.StmID, EmptyEffect())
	f.Blocks = []*Block{body, ifSt.Then, ifSt.Else}
	f.Stack = []*Block{body}

	entry := []*FactPointTo{
		MakeFactPointTo(g99, g77),
		MakeFactPointTo(g77, tgt),
	}
	fm.SetMapFactsIn(body.StmID, CloneFactSlice(entry))
	fm.GlobalFacts = CloneFactSlice(entry)

	cg := EmptyCGContext().WithFactMgr(fm)
	cg.CurrentFunc = f
	pre := EmptyEffect()
	cg.EffectAccum = &pre
	cg.EffectStm = EmptyEffect()

	// One gen-time walk leaves may-null on g77 (same as after if post_creation).
	if !VisitFactsStatementIf(&ifSt, &cg, opts) {
		t.Fatalf("gen VisitFacts if: err=%v", HasErrorSess(testAmbientSession))
	}
	if fg := FindRelatedPointTo(fm.GlobalFacts, g77); fg == nil || !fg.IsNull() {
		t.Fatalf("after if, g77 must may-null, got %v", fg)
	}
	// Reinstall complete map_stm_effect (visit may rewrite)
	for _, id := range []int{thenNull.StmID, elseDeref.StmID, ifSt.StmID, ifSt.Then.StmID, ifSt.Else.StmID, body.StmID} {
		fm.SetMapStmEffect(id, EmptyEffect())
	}

	nBefore := len(body.Stmts)
	body.PostCreationAnalysis(&cg, opts, pre, NewRng(1), nil)
	ClearErrorSess(testAmbientSession)
	if len(body.Stmts) != 0 {
		t.Fatalf("Block.cpp:709–714: self-back may-null re-entry must strip if; before=%d after=%d",
			nBefore, len(body.Stmts))
	}
}

// TestNestedMustReturnForDoesNotSelfBackStrip — StatementFor.cpp does not override
// must_return; body ending in return → must_break_or_return → !is_loop_body
// (Block.cpp:696–697). Null/else-deref inside such a body does not get
// post_creation self-back strip (seed-17809 shape).
func TestNestedMustReturnForDoesNotSelfBackStrip(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	SetProcessOptionsSess(testAmbientSession, opts)

	i32 := GetIntType()
	pt := PointerTo(i32)
	ppt := PointerTo(pt)
	g77 := CreateVariableScalars("g_77", pt, false, false)
	g99 := CreateVariableScalars("g_99", ppt, false, false)
	tgt := CreateVariableScalars("g_18", i32, false, false)

	f := &Function{Name: "f", ReturnType: i32, RV: CreateVariableScalars("rv", i32, false, false)}
	fm := NewFactMgrSess(testAmbientSession, f)

	thenNull := Stmt{
		Kind: StmtAssign, StmID: AllocStmID(),
		LhsVar: g99, Lhs: &Lhs{Var: g99, Type: pt},
		Expr:     &Expression{Term: TermConstant, Con: &Constant{Type: pt, Value: "0"}, ExprType: pt},
		AssignOp: AssignSimple,
	}
	elseDeref := Stmt{
		Kind: StmtAssign, StmID: AllocStmID(),
		LhsVar: g77, Lhs: &Lhs{Var: g77, Type: i32},
		Expr:     &Expression{Term: TermConstant, Con: MakeInt(1), ExprType: i32},
		AssignOp: AssignSimple,
	}
	ifSt := Stmt{
		Kind: StmtIfElse, StmID: AllocStmID(),
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1), ExprType: i32},
		Then: &Block{StmID: AllocStmID(), Func: f, Stmts: []Stmt{thenNull}},
		Else: &Block{StmID: AllocStmID(), Func: f, Stmts: []Stmt{elseDeref}},
	}
	ret := Stmt{
		Kind: StmtReturn, StmID: AllocStmID(),
		Expr: &Expression{Term: TermVariable, Var: f.RV, ExprType: i32},
	}
	body := &Block{
		Func: f, StmID: AllocStmID(), Looping: true,
		Stmts: []Stmt{ifSt, ret},
	}
	for _, id := range []int{thenNull.StmID, elseDeref.StmID, ifSt.StmID, ifSt.Then.StmID, ifSt.Else.StmID, ret.StmID, body.StmID} {
		fm.SetMapStmEffect(id, EmptyEffect())
	}
	f.Blocks = []*Block{body, ifSt.Then, ifSt.Else}
	f.Stack = []*Block{body}

	if !body.MustReturnWithFM(fm) {
		t.Fatal("body ending in return must MustReturn")
	}
	if body.MustBreakOrReturnFull(fm) {
		// must_break_or_return is true → is_loop_body false
	} else {
		t.Fatal("must_break_or_return should be true for last-return body")
	}
	// is_loop_body = !mustBR && looping → false; no FP strip expected
	entry := []*FactPointTo{
		MakeFactPointTo(g99, g77),
		MakeFactPointTo(g77, tgt),
	}
	fm.SetMapFactsIn(body.StmID, CloneFactSlice(entry))
	fm.GlobalFacts = CloneFactSlice(entry)
	cg := EmptyCGContext().WithFactMgr(fm)
	cg.CurrentFunc = f
	pre := EmptyEffect()
	cg.EffectAccum = &pre
	// gen walk
	_ = VisitFactsStatementIf(&ifSt, &cg, opts)
	for _, id := range []int{thenNull.StmID, elseDeref.StmID, ifSt.StmID, ifSt.Then.StmID, ifSt.Else.StmID, ret.StmID, body.StmID} {
		fm.SetMapStmEffect(id, EmptyEffect())
	}
	nBefore := len(body.Stmts)
	body.PostCreationAnalysis(&cg, opts, pre, NewRng(1), nil)
	ClearErrorSess(testAmbientSession)
	if len(body.Stmts) != nBefore {
		t.Fatalf("must_return loop body must not self-back-strip (is_loop_body false); before=%d after=%d",
			nBefore, len(body.Stmts))
	}
}
