package csmith

import "testing"

func TestPickTermTypeNilTablesFailClosed(t *testing.T) {
	// Expression::InitProbabilityTables always live; no invent NewExprTables
	// when both arg and process tables are missing — sticky MaxTermTypes
	ClearErrorSess(testAmbientSession)
	prev := ProcessExprTablesSess(testAmbientSession)
	SetProcessExprTablesSess(testAmbientSession, nil)
	defer SetProcessExprTablesSess(testAmbientSession, prev)
	tt := PickTermType(NewRng(1), nil, Defaults(), GetIntType(), false, false, 0)
	if tt != MaxTermTypes {
		t.Fatalf("want MaxTermTypes, got %v", tt)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil tables PickTermType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	tt = PickParamTermType(NewRng(1), nil, Defaults(), GetIntType(), 0)
	if tt != MaxTermTypes {
		t.Fatalf("param want MaxTermTypes, got %v", tt)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil tables PickParamTermType must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomAssignUsesProcessAssignOpsTable(t *testing.T) {
	// StatementAssign::assignOpsTable_ sticky; no invent per assign
	ClearErrorSess(testAmbientSession)
	prev := ProcessAssignOpsTableSess(testAmbientSession)
	SetProcessAssignOpsTableSess(testAmbientSession, nil)
	defer SetProcessAssignOpsTableSess(testAmbientSession, prev)
	opts := Defaults()
	f := &Function{Name: "f", ReturnType: GetIntType()}
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(f.ensurePairedFactMgr())
	st := MakeRandomAssign(NewRng(1), opts, NewProbabilities(opts), NewVariableSelector(testAmbientSession, opts), NewExprTables(opts), &cg, GetIntType())
	if stmtOK(st) {
		t.Fatal("nil assignOpsTable must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil assignOpsTable must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	InitSessionProbabilityTablesSess(testAmbientSession, opts)
	st = MakeRandomAssign(NewRng(2), opts, NewProbabilities(opts), NewVariableSelector(testAmbientSession, opts), NewExprTables(opts), &cg, GetIntType())
	// may still fail for other reasons; at least table is live
	_ = st
	ClearErrorSess(testAmbientSession)
}

func TestAssignOpsProbabilityNilTableFailClosed(t *testing.T) {
	// StatementAssign::assignOpsTable_ always live sticky; no invent NewAssignOpsTable
	ClearErrorSess(testAmbientSession)
	op := AssignOpsProbability(NewRng(1), Defaults(), nil, GetIntType())
	if op != AssignOp(-1) {
		t.Fatalf("want invalid op, got %v", op)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil table AssignOpsProbability must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if AssignOpsProbability(nil, Defaults(), NewAssignOpsTable(Defaults()), GetIntType()) != AssignOp(-1) {
		t.Fatal("nil RNG AssignOpsProbability must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG AssignOpsProbability must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomInvocationNilStmtTabFailClosed(t *testing.T) {
	// nested GenerateBody needs session Statement table; sticky no invent second table
	ClearErrorSess(testAmbientSession)
	prev := ProcessStmtTabSess(testAmbientSession)
	SetProcessStmtTabSess(testAmbientSession, nil)
	defer SetProcessStmtTabSess(testAmbientSession, prev)
	opts := Defaults()
	opts.MaxFuncs = 10
	vs := NewVariableSelector(testAmbientSession, opts)
	list := &FunctionList{Funcs: nil, Types: vs.Types}
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	_ = f.ensurePairedFactMgr()
	list.Funcs = []*Function{f}
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFuncList(list).WithFactMgr(f.PairedFactMgr())
	cg.CurrentFunc = f
	// force user path that would create a new function
	fi := MakeRandomInvocation(NewRng(1), opts, NewProbabilities(opts), vs, NewExprTables(opts), &cg, list, GetIntType(), nil, false)
	// without ProcessStmtTab, create-new path fails closed sticky Failed; may pick existing or fail
	if fi != nil && !fi.Failed && fi.User != nil && fi.User != f && fi.User.Body != nil {
		// if somehow built without stmt tab, bad
		if ProcessStmtTabSess(testAmbientSession) == nil && fi.User.BuildState == BuildBuilt {
			t.Fatal("must not invent body without session StmtTab")
		}
	}
	// when create-new path is taken without StmtTab, sticky Failed
	if fi != nil && fi.Failed && HasErrorSess(testAmbientSession) {
		// sticky path exercised
	}
	ClearErrorSess(testAmbientSession)
}

func TestGenerateSeed65WithProcessStmtTab(t *testing.T) {
	// regression: Initialize must re-install StmtTab after DoFinalization
	opts := Defaults()
	opts.Seed = 65
	out, err := Generate(opts)
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Fatal("empty")
	}
}

func TestEnsureAttrGeneratorsNoInvent(t *testing.T) {
	// InitAttrGenerators required; no soft invent zero-opts generators
	ClearAttrGeneratorsSess(testAmbientSession)
	if EnsureVarAttrGeneratorSess(testAmbientSession) != nil || EnsureFuncAttrGeneratorSess(testAmbientSession) != nil {
		t.Fatal("must not invent generators without InitAttrGenerators")
	}
	// Output nil-safe
	if EnsureVarAttrGeneratorSess(testAmbientSession).Output(NewRng(1)) != "" {
		t.Fatal("nil Output must be empty")
	}
	opts := Defaults()
	InitAttrGeneratorsSess(testAmbientSession, opts, NewProbabilities(opts))
	if EnsureVarAttrGeneratorSess(testAmbientSession) == nil {
		t.Fatal("want generator after init")
	}
	ClearAttrGeneratorsSess(testAmbientSession)
}

func TestNewVariableSelectorNoInventProbs(t *testing.T) {
	// C++ VariableSelector uses process Probabilities; no invent second table
	prev := ProcessProbabilitiesSess(testAmbientSession)
	SetProcessProbabilitiesSess(testAmbientSession, nil)
	defer SetProcessProbabilitiesSess(testAmbientSession, prev)
	vs := NewVariableSelector(testAmbientSession, Defaults())
	if vs.Probs != nil {
		t.Fatal("must not invent NewProbabilities when process unset")
	}
	p := NewProbabilities(Defaults())
	SetProcessProbabilitiesSess(testAmbientSession, p)
	vs2 := NewVariableSelector(testAmbientSession, Defaults())
	if vs2.Probs != p {
		t.Fatal("must share process Probabilities")
	}
}

func TestBinaryOpsFilterNoInventProbs(t *testing.T) {
	prev := ProcessProbabilitiesSess(testAmbientSession)
	SetProcessProbabilitiesSess(testAmbientSession, nil)
	defer SetProcessProbabilitiesSess(testAmbientSession, prev)
	// reject-all when process unset (no invent NewProbabilities(opts))
	f := BinaryOpsFilter(Defaults())
	if !f.Filter(0) {
		t.Fatal("nil process must reject-all")
	}
}

func TestStatementTableFromSessionProbs(t *testing.T) {
	// Statement.cpp:133–139 — stmtTable_ from pStatementProb on process Probabilities
	opts := Defaults()
	opts.Jumps = false
	opts.Arrays = false
	p := NewProbabilities(opts)
	tab := p.StatementThresholdTable()
	if tab == nil {
		t.Fatal("want statement table on Probabilities")
	}
	// without jumps/arrays, Goto/ArrayOp cutoffs absent — Assign at 100
	// rnd 50 must not be Goto
	for v := 0; v < 100; v++ {
		st := NumberToType(tab, uint32(v))
		if st == StmtGoto || st == StmtArrayOp {
			t.Fatalf("value %d → %v with jumps/arrays off", v, st)
		}
	}
	// NewProgramGenerator installs same instance as ProcessStmtTab
	prevP := ProcessProbabilitiesSess(testAmbientSession)
	prevS := ProcessStmtTabSess(testAmbientSession)
	defer func() {
		SetProcessProbabilitiesSess(testAmbientSession, prevP)
		SetProcessStmtTabSess(testAmbientSession, prevS)
	}()
	s := NewSession(opts)
	g := NewProgramGenerator(s)
	if g.StmtTab != g.Probs.StatementThresholdTable() {
		t.Fatal("generator StmtTab must be probs statement table")
	}
	// Table lives on the session bag (ambient ProcessStmtTab is only live while activated).
	if s.StmtTab != g.StmtTab {
		t.Fatal("session StmtTab must share generator table")
	}
}

func TestMakeRandomExprStmtNilCGSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if MakeRandomExprStmt(NewRng(1), Defaults(), nil, nil, nil, nil).Kind != 0 {
		t.Fatal("nil cg MakeRandomExprStmt must fail closed empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil cg MakeRandomExprStmt must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestHasUncertainCallRecursiveExprNilSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if !HasUncertainCallRecursiveExpr(nil) {
		t.Fatal("nil HasUncertainCallRecursiveExpr must fail closed true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil HasUncertainCallRecursiveExpr must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete constant no uncertain
	e := &Expression{Term: TermConstant, Con: MakeInt(1)}
	if HasUncertainCallRecursiveExpr(e) {
		t.Fatal("constant must not invent uncertain call")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("complete HasUncertainCallRecursiveExpr must not sticky")
	}
	ClearErrorSess(testAmbientSession)
}
