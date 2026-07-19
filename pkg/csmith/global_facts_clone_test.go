package csmith

import "testing"

func TestMakeRandomForIncompleteGlobalFactsFailClosed(t *testing.T) {
	// StatementFor.cpp:299–300 pre_facts snapshot; incomplete must not invent cleaned
	ClearError()
	opts := Defaults()
	opts.MaxBlockSize = 1
	fm := NewFactMgr(nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	vs := NewVariableSelector(opts)
	if MakeRandomFor(NewRng(1), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed MakeRandomFor")
	}
	ClearError()
}

func TestAppendReturnIncompleteGlobalFactsFailClosed(t *testing.T) {
	opts := Defaults()
	f := &Function{Name: "f", ReturnType: GetIntType()}
	f.RV = CreateVariableScalars("f_rv", GetIntType(), false, false)
	b := &Block{StmID: 1, Func: f}
	fm := NewFactMgr(f)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	vs := NewVariableSelector(opts)
	if b.AppendReturnStmt(NewRng(2), opts, vs, &cg) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed AppendReturnStmt")
	}
}

func TestVisitFactsBinaryOrderedIncompleteGlobalFactsFailClosed(t *testing.T) {
	// after-left snapshot incomplete fails closed (no invent cleaned merge)
	fm := NewFactMgr(nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	cg := EmptyCGContext().WithFactMgr(fm)
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

func TestMakeRandomIfFunc1IncompleteGlobalFactsFailClosed(t *testing.T) {
	// func_1 pre_facts snapshot must fail closed on incomplete GlobalFacts
	ClearError()
	opts := Defaults()
	opts.MaxBlockSize = 1
	opts.MaxBlockDepth = 1
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	fm := NewFactMgr(f)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	vs := NewVariableSelector(opts)
	// seed a global for expression selection
	_ = vs.GenerateNewGlobal(AccessRead, cg, GetIntType(), nil, NewRng(3))
	if MakeRandomIf(NewRng(4), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed MakeRandomIf func_1 path")
	}
	ClearError()
}

func TestMakeRandomExprStmtIncompleteGlobalFactsFailClosed(t *testing.T) {
	// StatementExpr.cpp:58–59 snapshot; incomplete must not invent cleaned rollback
	ClearError()
	opts := Defaults()
	fm := NewFactMgr(nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	vs := NewVariableSelector(opts)
	st := MakeRandomExprStmt(NewRng(5), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg)
	if st.Kind != 0 || st.StmID != 0 {
		t.Fatal("incomplete GlobalFacts must fail closed MakeRandomExprStmt")
	}
	ClearError()
}

func TestFindFixedPointIncompleteInputsFailClosed(t *testing.T) {
	// incomplete inputs fail closed (no invent cleaned fixed-point)
	ClearError()
	b := &Block{StmID: 1, Stmts: nil}
	fm := NewFactMgr(nil)
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	in := []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	_, _, ok := FindFixedPointBlock(b, in, &cg, Defaults(), false)
	if ok {
		t.Fatal("incomplete inputs must fail closed FindFixedPointBlock")
	}
	ClearError()
}

func TestVisitUnorderedParamsIncompleteFailClosed(t *testing.T) {
	fi := &Invocation{
		Args: []*Expression{
			{Term: TermConstant, Con: MakeInt(1)},
			{Term: TermConstant, Con: MakeInt(2)},
		},
	}
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	facts := []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	cg := EmptyCGContext()
	if fi.VisitUnorderedParams(&facts, &cg, Defaults()) {
		t.Fatal("incomplete facts must fail closed VisitUnorderedParams")
	}
}

func TestPostCreationAnalysisIncompleteFailClosed(t *testing.T) {
	ClearError()
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 3, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignSimple,
	}
	fm := NewFactMgr(nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	pre := []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	PostCreationAnalysis(st, pre, EmptyEffect(), &cg, Defaults())
	// incomplete pre: fail closed nil GlobalFacts (no invent cleaned assign update)
	if fm.GlobalFacts != nil {
		t.Fatal("incomplete pre must clear GlobalFacts, not invent post-creation")
	}
	// incomplete GlobalFacts seed
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	pre2 := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	PostCreationAnalysis(st, pre2, EmptyEffect(), &cg, Defaults())
	if fm.GlobalFacts != nil {
		t.Fatal("incomplete GlobalFacts must fail closed nil")
	}
	ClearError()
}
