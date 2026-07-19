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
	ClearError()
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
	if !HasError() {
		t.Fatal("incomplete GlobalFacts must SetError")
	}
	ClearError()
	// incomplete EffectContext fails closed sticky before EffectStm clear
	cg2 := WithFunc(f, IncompleteEffect()).WithFactMgr(NewFactMgr(f))
	eff2 := EmptyEffect()
	cg2.EffectAccum = &eff2
	if b.AppendReturnStmt(NewRng(3), opts, vs, &cg2) != nil {
		t.Fatal("incomplete EffectContext must fail closed AppendReturnStmt")
	}
	if !HasError() {
		t.Fatal("incomplete EffectContext must SetError sticky")
	}
	ClearError()
}

func TestMakeRandomAssignIncompleteFailClosed(t *testing.T) {
	// incomplete ambient/facts must not invent assign shell
	ClearError()
	opts := Defaults()
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgr(f)
	inc := IncompleteEffect()
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &inc
	vs := NewVariableSelector(opts)
	if stmtOK(MakeRandomAssign(NewRng(1), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, GetIntType())) {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomAssign")
	}
	if !HasError() {
		t.Fatal("must SetError")
	}
	ClearError()
	fm2 := NewFactMgr(f)
	fm2.GlobalFacts = IncompleteFactSlice()
	cg2 := WithFunc(f, EmptyEffect()).WithFactMgr(fm2)
	if stmtOK(MakeRandomAssign(NewRng(2), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg2, GetIntType())) {
		t.Fatal("incomplete GlobalFacts must fail closed MakeRandomAssign")
	}
	if !HasError() {
		t.Fatal("must SetError GlobalFacts")
	}
	ClearError()
	// incomplete EffectStm must not invent assign under hole shell
	cg3 := WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgr(f))
	cg3.EffectStm = IncompleteEffect()
	if stmtOK(MakeRandomAssign(NewRng(3), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg3, GetIntType())) {
		t.Fatal("incomplete EffectStm must fail closed MakeRandomAssign")
	}
	if !HasError() {
		t.Fatal("incomplete EffectStm must SetError sticky")
	}
	ClearError()
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
	if !HasError() {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearError()
	// incomplete EffectAccum must not invent expr stmt under hole shell
	inc := IncompleteEffect()
	cg3 := EmptyCGContext()
	cg3.EffectAccum = &inc
	st3 := MakeRandomExprStmt(NewRng(6), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg3)
	if st3.Kind != 0 || st3.StmID != 0 {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomExprStmt")
	}
	if !HasError() {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearError()
	// incomplete EffectContext fails closed sticky
	cg4 := WithFunc(nil, IncompleteEffect())
	eff4 := EmptyEffect()
	cg4.EffectAccum = &eff4
	st4 := MakeRandomExprStmt(NewRng(7), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg4)
	if st4.Kind != 0 || st4.StmID != 0 {
		t.Fatal("incomplete EffectContext must fail closed MakeRandomExprStmt")
	}
	if !HasError() {
		t.Fatal("incomplete EffectContext must SetError sticky")
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
	ClearError()
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
	if !HasError() {
		t.Fatal("incomplete facts VisitUnorderedParams must SetError sticky")
	}
	ClearError()
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
	// incomplete pre: fail closed sticky wipe (no invent cleaned assign update)
	if FactsComplete(fm.GlobalFacts) {
		t.Fatal("incomplete pre must clear GlobalFacts, not invent post-creation")
	}
	if !HasError() {
		t.Fatal("incomplete pre must SetError sticky")
	}
	ClearError()
	// incomplete GlobalFacts seed
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	pre2 := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	PostCreationAnalysis(st, pre2, EmptyEffect(), &cg, Defaults())
	if FactsComplete(fm.GlobalFacts) {
		t.Fatal("incomplete GlobalFacts must fail closed nil")
	}
	if !HasError() {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearError()
	// StmID 0 — no invent post_creation success without maps
	st0 := &Stmt{
		Kind: StmtAssign, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignSimple,
	}
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	PostCreationAnalysis(st0, []*FactPointTo{MakeFactPointTo(p, NullPtr)}, EmptyEffect(), &cg, Defaults())
	if FactsComplete(fm.GlobalFacts) {
		t.Fatal("StmID 0 must fail closed nil GlobalFacts")
	}
	if !HasError() {
		t.Fatal("StmID 0 must SetError sticky")
	}
	ClearError()
}

func TestMakeRandomIfForIncompleteAmbientFailClosed(t *testing.T) {
	// incomplete ambient at entry must sticky ERROR before EffectStm clear
	ClearError()
	opts := Defaults()
	opts.MaxBlockSize = 1
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgr(f)
	inc := IncompleteEffect()
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &inc
	if MakeRandomIf(NewRng(1), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg) != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomIf")
	}
	if !HasError() {
		t.Fatal("MakeRandomIf must SetError sticky")
	}
	ClearError()
	cg2 := WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgr(f))
	cg2.EffectAccum = &inc
	if MakeRandomFor(NewRng(2), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg2) != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomFor")
	}
	if !HasError() {
		t.Fatal("MakeRandomFor must SetError sticky")
	}
	ClearError()
}
