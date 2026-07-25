package csmith

import "testing"

func TestPartialExpanderInactiveAllowsAll(t *testing.T) {
	ClearPartialExpanderSess(testAmbientSession)
	if !ExpandCheck(StmtFor) || !ExpandCheck(StmtAssign) {
		t.Fatal("inactive allows all")
	}
}

func TestInitPartialExpanderAssignment(t *testing.T) {
	ClearPartialExpanderSess(testAmbientSession)
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
	ClearPartialExpanderSess(testAmbientSession)
}

func TestInitPartialExpanderForOnly(t *testing.T) {
	ClearPartialExpanderSess(testAmbientSession)
	if !InitPartialExpander("for") {
		t.Fatal("init")
	}
	// For allowed
	if !DirectExpandCheckSess(testAmbientSession, StmtFor) {
		t.Fatal("direct for")
	}
	if DirectExpandCheckSess(testAmbientSession, StmtAssign) {
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
	if !DirectExpandCheckSess(testAmbientSession, StmtFor) || DirectExpandCheckSess(testAmbientSession, StmtAssign) {
		t.Fatal("restored kinds")
	}
	ClearPartialExpanderSess(testAmbientSession)
}

func TestInitPartialExpanderAll(t *testing.T) {
	ClearPartialExpanderSess(testAmbientSession)
	if !InitPartialExpander("all") {
		t.Fatal("all")
	}
	// "all" sets every kind true including MAX from init then MAX true again
	if !DirectExpandCheckSess(testAmbientSession, StmtReturn) {
		t.Fatal("return")
	}
	ClearPartialExpanderSess(testAmbientSession)
}

func TestInitPartialExpanderMulti(t *testing.T) {
	ClearPartialExpanderSess(testAmbientSession)
	if !InitPartialExpander("if-else,return,invoke") {
		t.Fatal("multi")
	}
	if !DirectExpandCheckSess(testAmbientSession, StmtIfElse) || !DirectExpandCheckSess(testAmbientSession, StmtReturn) || !DirectExpandCheckSess(testAmbientSession, StmtInvoke) {
		t.Fatal("kinds")
	}
	// assign allowed via invoke alias while MAX set
	if !ExpandCheck(StmtAssign) {
		t.Fatal("assign via invoke")
	}
	ClearPartialExpanderSess(testAmbientSession)
}

func TestInitPartialExpanderBad(t *testing.T) {
	ClearPartialExpanderSess(testAmbientSession)
	if InitPartialExpander("nope") {
		t.Fatal("bad token")
	}
	ClearPartialExpanderSess(testAmbientSession)
}

func TestInitFromOptions(t *testing.T) {
	opts := Defaults()
	opts.PartialExpand = "for,assignment"
	if !InitPartialExpanderFromOptionsSess(testAmbientSession, opts) {
		t.Fatal("opts")
	}
	if !DirectExpandCheckSess(testAmbientSession, StmtFor) || !DirectExpandCheckSess(testAmbientSession, StmtAssign) {
		t.Fatal("from opts")
	}
	opts.PartialExpand = ""
	if !InitPartialExpanderFromOptionsSess(testAmbientSession, opts) {
		t.Fatal("clear")
	}
	if currentSession().PartialExpands != nil {
		t.Fatal("cleared")
	}
}

func TestVisitFactsJumpStoresEffect(t *testing.T) {
	fm := NewFactMgrSess(testAmbientSession, nil)
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntType(), true, false)
	eff := EmptyEffect()
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &eff
	cg.EffectStm = EmptyEffect().ReadVarSess(testAmbientSession, v)
	st := &Stmt{
		Kind: StmtBreak, StmID: 3,
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)},
	}
	if !VisitFactsStatementJump(st, &cg, Defaults()) {
		t.Fatal("visit")
	}
	if !fm.GetMapStmEffect(3).IsReadSess(testAmbientSession, v) {
		t.Fatal("effect")
	}
}

func TestVisitFactsStatementExpr(t *testing.T) {
	fm := NewFactMgrSess(testAmbientSession, nil)
	eff := EmptyEffect()
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
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
