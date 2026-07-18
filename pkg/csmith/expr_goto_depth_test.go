package csmith

import "testing"

func TestExpressionNotEquals(t *testing.T) {
	if !(&Expression{Term: TermConstant, Con: MakeInt(1)}).NotEquals(0) {
		t.Fatal("1 != 0")
	}
	if (&Expression{Term: TermConstant, Con: MakeInt(0)}).NotEquals(0) {
		t.Fatal("0 equals 0")
	}
	if (&Expression{Term: TermVariable}).NotEquals(0) {
		t.Fatal("var not_equals false")
	}
}

func TestExpressionUseVar(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), true, false)
	w := CreateVariableScalars("g_2", GetIntType(), true, false)
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
}

func TestMustJumpUsesNotEquals(t *testing.T) {
	st := Stmt{Kind: StmtBreak, Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}}
	if !st.MustJump() {
		t.Fatal("true const")
	}
	st.Expr = &Expression{Term: TermConstant, Con: MakeInt(0)}
	if st.MustJump() {
		t.Fatal("false const")
	}
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
	// still GOOD without DFS engine
	if DepthGuardByTypeFlag(opts, DtFunction, 0) != GoodDepth {
		t.Fatal("dfs stub")
	}
}

func TestVisitFactsGotoSubsetClearsDest(t *testing.T) {
	fm := NewFactMgr(nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	a := CreateVariableScalars("g_a", GetIntType(), true, false)
	b := CreateVariableScalars("g_b", GetIntType(), true, false)
	// previous out was wide {a,b}; current inputs subset {a}
	wide := MakeFactPointToSet(p, []*Variable{a, b})
	narrow := MakeFactPointTo(p, a)
	fm.SetMapFactsOut(5, []*FactPointTo{wide})
	fm.SetMapFactsIn(10, []*FactPointTo{wide})
	fm.SetMapFactsOut(10, []*FactPointTo{wide})
	fm.MapVisited = map[int]bool{} // neither visited
	fm.GlobalFacts = []*FactPointTo{narrow}
	eff := EmptyEffect()
	cg := EmptyCGContext().WithFactMgr(fm)
	cg.EffectAccum = &eff
	st := &Stmt{
		Kind: StmtGoto, StmID: 5, GotoDestStmID: 10,
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)},
	}
	if !VisitFactsStatementGoto(st, &cg, Defaults()) {
		t.Fatal("visit")
	}
	if _, ok := fm.MapFactsIn[10]; ok {
		t.Fatal("dest in cleared")
	}
	if _, ok := fm.MapFactsOut[10]; ok {
		t.Fatal("dest out cleared")
	}
}

func TestExpressionToString(t *testing.T) {
	e := &Expression{Term: TermConstant, Con: MakeInt(42)}
	if e.ToString() != "42" && e.ToString() != e.Output() {
		t.Fatal(e.ToString())
	}
}
