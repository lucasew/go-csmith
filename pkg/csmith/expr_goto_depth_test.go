package csmith

import "testing"

func TestExpressionNotEquals(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if !(&Expression{Term: TermConstant, Con: MakeInt(1)}).NotEquals(0) {
		t.Fatal("1 != 0")
	}
	if (&Expression{Term: TermConstant, Con: MakeInt(0)}).NotEquals(0) {
		t.Fatal("0 equals 0")
	}
	if (&Expression{Term: TermVariable}).NotEquals(0) {
		t.Fatal("var not_equals false")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete NotEquals paths must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestExpressionUseVar(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntType(), true, false)
	w := CreateVariableScalarsSess(testAmbientSession, "g_2", GetIntType(), true, false)
	e := &Expression{Term: TermVariable, Var: v}
	if !e.UseVar(v) || e.UseVar(w) {
		t.Fatal("var")
	}
	call := &Expression{
		Term: TermFunction,
		Invoke: &Invocation{Args: []*Expression{
			{Term: TermVariable, Var: v},
		}},
	}
	if !call.UseVar(v) || call.UseVar(w) {
		t.Fatal("call")
	}
	comma := &Expression{
		Term:     TermCommaExpr,
		CommaLHS: &Expression{Term: TermConstant, Con: MakeInt(0)},
		CommaRHS: e,
	}
	if !comma.UseVar(v) {
		t.Fatal("comma")
	}
	// incomplete IR fails closed sticky as uses (no invent conflict-free non-use)
	if !(&Expression{Term: TermFunction, Invoke: &Invocation{Args: []*Expression{nil}}}).UseVar(v) {
		t.Fatal("nil arg hole must fail closed as uses")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil arg hole UseVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !(&Expression{Term: TermCommaExpr, CommaLHS: nil, CommaRHS: e}).UseVar(v) {
		t.Fatal("nil comma side must fail closed as uses")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil comma side UseVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if !(&Expression{Term: TermVariable, Var: nil}).UseVar(v) {
		t.Fatal("nil TermVariable must fail closed as uses")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil TermVariable UseVar must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMustJumpUsesNotEquals(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	st := Stmt{Kind: StmtBreak, Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}}
	if !st.MustJump() {
		t.Fatal("true const")
	}
	st.Expr = &Expression{Term: TermConstant, Con: MakeInt(0)}
	if st.MustJump() {
		t.Fatal("false const")
	}
	// incomplete break without test sticky not-must-jump
	ClearErrorSess(testAmbientSession)
	st.Expr = nil
	if st.MustJump() {
		t.Fatal("nil Expr MustJump must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil Expr MustJump must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMinimalDepthTable(t *testing.T) {
	if MinimalDepth(DtConstant, 0) != 0 {
		t.Fatal("const")
	}
	if MinimalDepth(DtBlock, 0) != MinimalDepth(DtStatement, 0)+1 {
		t.Fatal("block")
	}
	if MinimalDepth(DtLoopControl, 0) != 3 {
		t.Fatal("loop")
	}
	if MinimalDepth(DtExpression, int(MaxTermTypes)) != 1 {
		t.Fatal("expr max term")
	}
}

func TestDepthGuardRandomAlwaysGood(t *testing.T) {
	opts := Defaults()
	opts.DFSExhaustive = false
	if DepthGuardByType(opts, DtBlock) != GoodDepth {
		t.Fatal("random")
	}
	opts.DFSExhaustive = true
	// DFS mode needs live DFS engine (DepthSpec::backtracking).
	// Fresh engine current_pos_=-1 → eager_backtracking returns false → GOOD.
	RandomNumberDoFinalizationSess(testAmbientSession)
	opts.MaxExhaustiveDepth = 8
	SetProcessOptionsSess(testAmbientSession, opts)
	CreateRandomNumberInstanceSess(testAmbientSession, RngKindDFS, 1)
	defer func() {
		RandomNumberDoFinalizationSess(testAmbientSession)
		ReinstallTestProcessSingletons()
		ClearErrorSess(testAmbientSession)
	}()
	ClearErrorSess(testAmbientSession)
	if DepthGuardByTypeFlag(opts, DtFunction, 0) != GoodDepth {
		t.Fatal("dfs fresh engine GOOD", GetErrorSess(testAmbientSession))
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete depth guard must not sticky")
	}
}

func TestVisitFactsGotoSubsetClearsDest(t *testing.T) {
	// StatementGoto.cpp:390–398 — subset outs clear map_facts_in/out[dest] full FactVec
	// (ePointTo + eUnionWrite). Soft invent was PT-only delete.
	ClearErrorSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerTo(GetIntType()), true, false)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntType(), true, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntType(), true, false)
	wide := MakeFactPointToSet(p, []*Variable{a, b})
	narrow := MakeFactPointTo(p, a)
	fm.SetMapFactsOut(5, []*FactPointTo{wide})
	fm.SetMapFactsIn(10, []*FactPointTo{wide})
	fm.SetMapFactsOut(10, []*FactPointTo{wide})
	ut := &Type{isUnion: true, StructName: "U_goto", Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
	}}
	gu := CreateVariableQferSess(testAmbientSession, "g_u", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	gu.Init = MakeInt(0)
	u := MakeFactUnion(gu, 0)
	// dest has union lattice; prev/cur outs empty union (size match via PT only)
	fm.MapUnionFactsIn = map[int][]*FactUnion{10: {u}}
	fm.MapUnionFactsOut = map[int][]*FactUnion{10: {u}}
	fm.UnionFacts = []*FactUnion{} // live empty pairs with empty prevOutU
	fm.MapVisited = map[int]bool{}
	fm.GlobalFacts = []*FactPointTo{narrow}
	eff := EmptyEffect()
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &eff
	st := &Stmt{
		Kind: StmtGoto, StmID: 5, GotoDestStmID: 10,
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)},
	}
	if !VisitFactsStatementGoto(st, &cg, Defaults()) {
		t.Fatal("visit", HasErrorSess(testAmbientSession))
	}
	if _, ok := fm.MapFactsIn[10]; ok {
		t.Fatal("dest PT in cleared")
	}
	if _, ok := fm.MapFactsOut[10]; ok {
		t.Fatal("dest PT out cleared")
	}
	if _, ok := fm.MapUnionFactsIn[10]; ok {
		t.Fatal("dest union in cleared (full FactVec clear)")
	}
	if _, ok := fm.MapUnionFactsOut[10]; ok {
		t.Fatal("dest union out cleared (full FactVec clear)")
	}
}

func TestVisitFactsGotoSubsetClearsDestStmID0(t *testing.T) {
	// fair sid: dest stm_id 0 is valid (StatementGoto.cpp no destID>0 invent)
	ClearErrorSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p0", PointerTo(GetIntType()), true, false)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a0", GetIntType(), true, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b0", GetIntType(), true, false)
	wide := MakeFactPointToSet(p, []*Variable{a, b})
	narrow := MakeFactPointTo(p, a)
	fm.SetMapFactsOut(1, []*FactPointTo{wide})
	fm.SetMapFactsIn(0, []*FactPointTo{wide})
	fm.SetMapFactsOut(0, []*FactPointTo{wide})
	fm.UnionFacts = []*FactUnion{}
	fm.MapVisited = map[int]bool{}
	fm.GlobalFacts = []*FactPointTo{narrow}
	eff := EmptyEffect()
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &eff
	st := &Stmt{
		Kind: StmtGoto, StmID: 1, GotoDestStmID: 0,
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)},
	}
	if !VisitFactsStatementGoto(st, &cg, Defaults()) {
		t.Fatal("visit", HasErrorSess(testAmbientSession))
	}
	if _, ok := fm.MapFactsIn[0]; ok {
		t.Fatal("dest id 0 in must clear")
	}
}

func TestExpressionToString(t *testing.T) {
	e := &Expression{Term: TermConstant, Con: MakeInt(42)}
	if e.ToString() != "42" && e.ToString() != e.Output() {
		t.Fatal(e.ToString())
	}
	// Expression always live; sticky empty via Output (no invent soft-skip past hole)
	ClearErrorSess(testAmbientSession)
	if (*Expression)(nil).ToString() != "" {
		t.Fatal("nil ToString must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil ToString must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}
