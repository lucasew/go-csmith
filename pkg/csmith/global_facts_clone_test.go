package csmith

import "testing"

func TestMakeRandomForIncompleteGlobalFactsFailClosed(t *testing.T) {
	// StatementFor.cpp:299–300 pre_facts snapshot; incomplete must not invent cleaned
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.MaxBlockSize = 1
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	vs := NewVariableSelector(testAmbientSession, opts)
	if MakeRandomFor(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed MakeRandomFor")
	}
	ClearErrorSess(testAmbientSession)
}

func TestAppendReturnIncompleteGlobalFactsFailClosed(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	f.RV = CreateVariableScalarsSess(testAmbientSession, "f_rv", GetIntTypeSess(testAmbientSession), false, false)
	b := &Block{StmID: 1, Func: f}
	fm := NewFactMgrSess(testAmbientSession, f)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	vs := NewVariableSelector(testAmbientSession, opts)
	if b.AppendReturnStmt(NewRngSess(testAmbientSession, 2), opts, vs, &cg) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed AppendReturnStmt")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts must SetError")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete EffectContext fails closed sticky before EffectStm clear
	cg2 := WithFunc(f, IncompleteEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	eff2 := EmptyEffect()
	cg2.EffectAccum = &eff2
	if b.AppendReturnStmt(NewRngSess(testAmbientSession, 3), opts, vs, &cg2) != nil {
		t.Fatal("incomplete EffectContext must fail closed AppendReturnStmt")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectContext must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomAssignIncompleteFailClosed(t *testing.T) {
	// incomplete ambient/facts must not invent assign shell
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	fm := NewFactMgrSess(testAmbientSession, f)
	inc := IncompleteEffect()
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &inc
	vs := NewVariableSelector(testAmbientSession, opts)
	if stmtOK(MakeRandomAssign(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, GetIntTypeSess(testAmbientSession))) {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomAssign")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("must SetError")
	}
	ClearErrorSess(testAmbientSession)
	fm2 := NewFactMgrSess(testAmbientSession, f)
	fm2.GlobalFacts = IncompleteFactSlice()
	cg2 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm2)
	if stmtOK(MakeRandomAssign(NewRngSess(testAmbientSession, 2), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg2, GetIntTypeSess(testAmbientSession))) {
		t.Fatal("incomplete GlobalFacts must fail closed MakeRandomAssign")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("must SetError GlobalFacts")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete EffectStm must not invent assign under hole shell
	cg3 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	cg3.EffectStm = IncompleteEffect()
	if stmtOK(MakeRandomAssign(NewRngSess(testAmbientSession, 3), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg3, GetIntTypeSess(testAmbientSession))) {
		t.Fatal("incomplete EffectStm must fail closed MakeRandomAssign")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectStm must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestVisitFactsBinaryOrderedIncompleteGlobalFactsFailClosed(t *testing.T) {
	// after-left snapshot incomplete fails closed (no invent cleaned merge)
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	fi := &Invocation{
		Args: []*Expression{
			{Term: TermConstant, Con: MakeInt(1)},
			{Term: TermConstant, Con: MakeInt(0)},
		},
		Binary: "&&",
	}
	if VisitFactsBinaryOrdered(fi, &cg, Defaults()) {
		t.Fatal("incomplete GlobalFacts must fail closed ordered binary visit")
	}
}

func TestVisitFactsBinaryOrderedVisitResidualSticky(t *testing.T) {
	// Visit residual soft invent was soft-continue right/merge invent visit success.
	ClearErrorSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, nil)
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	// incomplete Constant shell residuals VisitFactsExpression
	hole := &Expression{Term: TermConstant, Con: &Constant{Type: GetIntTypeSess(testAmbientSession)}} // empty Value
	good := &Expression{Term: TermConstant, Con: MakeInt(0)}
	fi := &Invocation{Args: []*Expression{hole, good}, Binary: "&&"}
	if VisitFactsBinaryOrdered(fi, &cg, Defaults()) {
		t.Fatal("visit residual must fail closed ordered binary, not invent success")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("visit residual VisitFactsBinaryOrdered must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// residual on right after left succeeds
	fi2 := &Invocation{Args: []*Expression{good, hole}, Binary: "||"}
	if VisitFactsBinaryOrdered(fi2, &cg, Defaults()) {
		t.Fatal("right visit residual must fail closed ordered binary")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("right visit residual must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomIfFunc1IncompleteGlobalFactsFailClosed(t *testing.T) {
	// func_1 pre_facts snapshot must fail closed on incomplete GlobalFacts
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.MaxBlockSize = 1
	opts.MaxBlockDepth = 1
	f := &Function{Name: "func_1", ReturnType: GetIntTypeSess(testAmbientSession)}
	fm := NewFactMgrSess(testAmbientSession, f)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	vs := NewVariableSelector(testAmbientSession, opts)
	// seed a global for expression selection
	_ = vs.GenerateNewGlobal(AccessRead, cg, GetIntTypeSess(testAmbientSession), nil, NewRngSess(testAmbientSession, 3))
	if MakeRandomIf(NewRngSess(testAmbientSession, 4), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed MakeRandomIf func_1 path")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomExprStmtIncompleteGlobalFactsFailClosed(t *testing.T) {
	// StatementExpr.cpp:58–59 snapshot; incomplete must not invent cleaned rollback
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	vs := NewVariableSelector(testAmbientSession, opts)
	st := MakeRandomExprStmt(NewRngSess(testAmbientSession, 5), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg)
	if st.Kind != 0 || st.StmID != 0 {
		t.Fatal("incomplete GlobalFacts must fail closed MakeRandomExprStmt")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete EffectAccum must not invent expr stmt under hole shell
	inc := IncompleteEffect()
	cg3 := EmptyCGContext().WithSession(testAmbientSession)
	cg3.EffectAccum = &inc
	st3 := MakeRandomExprStmt(NewRngSess(testAmbientSession, 6), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg3)
	if st3.Kind != 0 || st3.StmID != 0 {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomExprStmt")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete EffectContext fails closed sticky
	cg4 := WithFunc(nil, IncompleteEffect()).WithSession(testAmbientSession)
	eff4 := EmptyEffect()
	cg4.EffectAccum = &eff4
	st4 := MakeRandomExprStmt(NewRngSess(testAmbientSession, 7), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg4)
	if st4.Kind != 0 || st4.StmID != 0 {
		t.Fatal("incomplete EffectContext must fail closed MakeRandomExprStmt")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectContext must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestFindFixedPointIncompleteInputsFailClosed(t *testing.T) {
	// incomplete inputs fail closed (no invent cleaned fixed-point)
	ClearErrorSess(testAmbientSession)
	b := &Block{StmID: 1, Stmts: nil}
	fm := NewFactMgrSess(testAmbientSession, nil)
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	in := []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	_, _, _, ok := FindFixedPointBlock(b, in, &cg, Defaults(), false)
	if ok {
		t.Fatal("incomplete inputs must fail closed FindFixedPointBlock")
	}
	ClearErrorSess(testAmbientSession)
}

func TestVisitUnorderedParamsIncompleteFailClosed(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	fi := &Invocation{
		Args: []*Expression{
			{Term: TermConstant, Con: MakeInt(1)},
			{Term: TermConstant, Con: MakeInt(2)},
		},
	}
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	facts := []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	cg := EmptyCGContext().WithSession(testAmbientSession)
	if fi.VisitUnorderedParams(&facts, &cg, Defaults()) {
		t.Fatal("incomplete facts must fail closed VisitUnorderedParams")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete facts VisitUnorderedParams must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestPostCreationAnalysisIncompleteFailClosed(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 3, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntTypeSess(testAmbientSession)},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignSimple,
	}
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	pre := []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	PostCreationAnalysis(st, pre, nil, EmptyEffect(), &cg, Defaults())
	// incomplete pre: fail closed sticky wipe (no invent cleaned assign update)
	if FactsComplete(fm.GlobalFacts) {
		t.Fatal("incomplete pre must clear GlobalFacts, not invent post-creation")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete pre must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete GlobalFacts seed
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	pre2 := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	PostCreationAnalysis(st, pre2, nil, EmptyEffect(), &cg, Defaults())
	if FactsComplete(fm.GlobalFacts) {
		t.Fatal("incomplete GlobalFacts must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// StmID 0 — no invent post_creation success without maps
	st0 := &Stmt{
		Kind: StmtAssign, StmID: IncompleteStmID, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntTypeSess(testAmbientSession)},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignSimple,
	}
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	PostCreationAnalysis(st0, []*FactPointTo{MakeFactPointTo(p, NullPtr)}, nil, EmptyEffect(), &cg, Defaults())
	if FactsComplete(fm.GlobalFacts) {
		t.Fatal("IncompleteStmID must fail closed nil GlobalFacts")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("StmID 0 must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Statement + CGContext always live; sticky (no invent soft-skip past hole)
	// Nil FM is non-sticky soft re-pick
	PostCreationAnalysis(nil, []*FactPointTo{}, nil, EmptyEffect(), &cg, Defaults())
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil stmt PostCreationAnalysis must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	PostCreationAnalysis(st, []*FactPointTo{}, nil, EmptyEffect(), nil, Defaults())
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil cg PostCreationAnalysis must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	cgNoFM := EmptyCGContext().WithSession(testAmbientSession)
	PostCreationAnalysis(st, []*FactPointTo{}, nil, EmptyEffect(), &cgNoFM, Defaults())
	if HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM PostCreationAnalysis must stay non-sticky soft re-pick")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomIfForIncompleteAmbientFailClosed(t *testing.T) {
	// incomplete ambient at entry must sticky ERROR before EffectStm clear
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.MaxBlockSize = 1
	vs := NewVariableSelector(testAmbientSession, opts)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	fm := NewFactMgrSess(testAmbientSession, f)
	inc := IncompleteEffect()
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	cg.EffectAccum = &inc
	if MakeRandomIf(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg) != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomIf")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("MakeRandomIf must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	cg2 := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	cg2.EffectAccum = &inc
	if MakeRandomFor(NewRngSess(testAmbientSession, 2), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg2) != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomFor")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("MakeRandomFor must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}
