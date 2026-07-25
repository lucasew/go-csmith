package csmith

import "testing"

func TestPartialExpanderInactiveAllowsAll(t *testing.T) {
	ClearPartialExpander()
	if !ExpandCheck(StmtFor) || !ExpandCheck(StmtAssign) {
		t.Fatal("inactive allows all")
	}
}

func TestInitPartialExpanderAssignment(t *testing.T) {
	ClearPartialExpander()
	if !InitPartialExpander("assignment") {
		t.Fatal("init")
	}
	// partial mode active: only assignment (and invoke via assign alias) allowed
	if !ExpandCheck(StmtAssign) {
		t.Fatal("assign")
	}
	// first success clears MAX → subsequent all allowed
	if !ExpandCheck(StmtFor) {
		t.Fatal("after first, mode off")
	}
	ClearPartialExpander()
}

func TestInitPartialExpanderForOnly(t *testing.T) {
	ClearPartialExpander()
	if !InitPartialExpander("for") {
		t.Fatal("init")
	}
	// For allowed
	if !DirectExpandCheck(StmtFor) {
		t.Fatal("direct for")
	}
	if DirectExpandCheck(StmtAssign) {
		t.Fatal("assign not set")
	}
	// ExpandCheck(For) succeeds and disables partial mode
	if !ExpandCheck(StmtFor) {
		t.Fatal("expand for")
	}
	if ExpandCheck(StmtAssign) != true {
		// mode off
	}
	// restore backup
	RestorePartialExpanderInitValuesSess(testAmbientSession)
	if !currentSession().PartialExpands[MaxStatementType] {
		t.Fatal("restored MAX")
	}
	if !DirectExpandCheck(StmtFor) || DirectExpandCheck(StmtAssign) {
		t.Fatal("restored kinds")
	}
	ClearPartialExpander()
}

func TestInitPartialExpanderAll(t *testing.T) {
	ClearPartialExpander()
	if !InitPartialExpander("all") {
		t.Fatal("all")
	}
	// "all" sets every kind true including MAX from init then MAX true again
	if !DirectExpandCheck(StmtReturn) {
		t.Fatal("return")
	}
	ClearPartialExpander()
}

func TestInitPartialExpanderMulti(t *testing.T) {
	ClearPartialExpander()
	if !InitPartialExpander("if-else,return,invoke") {
		t.Fatal("multi")
	}
	if !DirectExpandCheck(StmtIfElse) || !DirectExpandCheck(StmtReturn) || !DirectExpandCheck(StmtInvoke) {
		t.Fatal("kinds")
	}
	// assign allowed via invoke alias while MAX set
	if !ExpandCheck(StmtAssign) {
		t.Fatal("assign via invoke")
	}
	ClearPartialExpander()
}

func TestInitPartialExpanderBad(t *testing.T) {
	ClearPartialExpander()
	if InitPartialExpander("nope") {
		t.Fatal("bad token")
	}
	ClearPartialExpander()
}

func TestInitFromOptions(t *testing.T) {
	opts := Defaults()
	opts.PartialExpand = "for,assignment"
	if !InitPartialExpanderFromOptions(opts) {
		t.Fatal("opts")
	}
	if !DirectExpandCheck(StmtFor) || !DirectExpandCheck(StmtAssign) {
		t.Fatal("from opts")
	}
	opts.PartialExpand = ""
	if !InitPartialExpanderFromOptions(opts) {
		t.Fatal("clear")
	}
	if currentSession().PartialExpands != nil {
		t.Fatal("cleared")
	}
}

func TestVisitFactsJumpStoresEffect(t *testing.T) {
	fm := NewFactMgr(nil)
	v := CreateVariableScalars("g_1", GetIntType(), true, false)
	eff := EmptyEffect()
	cg := EmptyCGContext().WithFactMgr(fm)
	cg.EffectAccum = &eff
	cg.EffectStm = EmptyEffect().ReadVar(v)
	st := &Stmt{
		Kind: StmtBreak, StmID: 3,
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)},
	}
	if !VisitFactsStatementJump(st, &cg, Defaults()) {
		t.Fatal("visit")
	}
	if !fm.GetMapStmEffect(3).IsRead(v) {
		t.Fatal("effect")
	}
}

func TestVisitFactsStatementExpr(t *testing.T) {
	fm := NewFactMgr(nil)
	eff := EmptyEffect()
	cg := EmptyCGContext().WithFactMgr(fm)
	cg.EffectAccum = &eff
	st := &Stmt{
		Kind: StmtInvoke, StmID: 4,
		Expr: &Expression{Term: TermConstant, Con: MakeInt(0)},
	}
	if !VisitFactsStatementExpr(st, &cg, Defaults()) {
		t.Fatal("expr")
	}
	if _, ok := fm.MapStmEffect[4]; !ok {
		t.Fatal("no effect map")
	}
}
