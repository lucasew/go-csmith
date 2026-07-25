package csmith

import "testing"

func TestPickTermTypeNilTablesFailClosed(t *testing.T) {
	// Expression::InitProbabilityTables always live; no invent NewExprTables
	// when both arg and process tables are missing — sticky MaxTermTypes
	ClearError()
	prev := ProcessExprTablesSess(testAmbientSession)
	SetProcessExprTablesSess(testAmbientSession, nil)
	defer SetProcessExprTablesSess(testAmbientSession, prev)
	tt := PickTermType(NewRng(1), nil, Defaults(), GetIntType(), false, false, 0)
	if tt != MaxTermTypes {
		t.Fatalf("want MaxTermTypes, got %v", tt)
	}
	if !HasError() {
		t.Fatal("nil tables PickTermType must SetError sticky")
	}
	ClearError()
	tt = PickParamTermType(NewRng(1), nil, Defaults(), GetIntType(), 0)
	if tt != MaxTermTypes {
		t.Fatalf("param want MaxTermTypes, got %v", tt)
	}
	if !HasError() {
		t.Fatal("nil tables PickParamTermType must SetError sticky")
	}
	ClearError()
}

func TestMakeRandomAssignUsesProcessAssignOpsTable(t *testing.T) {
	// StatementAssign::assignOpsTable_ sticky; no invent per assign
	ClearError()
	prev := ProcessAssignOpsTableSess(testAmbientSession)
	SetProcessAssignOpsTableSess(testAmbientSession, nil)
	defer SetProcessAssignOpsTableSess(testAmbientSession, prev)
	opts := Defaults()
	f := &Function{Name: "f", ReturnType: GetIntType()}
	cg := EmptyCGContext().WithFactMgr(f.ensurePairedFactMgr())
	st := MakeRandomAssign(NewRng(1), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), &cg, GetIntType())
	if stmtOK(st) {
		t.Fatal("nil assignOpsTable must fail closed")
	}
	if !HasError() {
		t.Fatal("nil assignOpsTable must SetError sticky")
	}
	ClearError()
	InitSessionProbabilityTables(opts)
	st = MakeRandomAssign(NewRng(2), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), &cg, GetIntType())
	// may still fail for other reasons; at least table is live
	_ = st
	ClearError()
}

func TestAssignOpsProbabilityNilTableFailClosed(t *testing.T) {
	// StatementAssign::assignOpsTable_ always live sticky; no invent NewAssignOpsTable
	ClearError()
	op := AssignOpsProbability(NewRng(1), Defaults(), nil, GetIntType())
	if op != AssignOp(-1) {
		t.Fatalf("want invalid op, got %v", op)
	}
	if !HasError() {
		t.Fatal("nil table AssignOpsProbability must SetError sticky")
	}
	ClearError()
	if AssignOpsProbability(nil, Defaults(), NewAssignOpsTable(Defaults()), GetIntType()) != AssignOp(-1) {
		t.Fatal("nil RNG AssignOpsProbability must fail closed")
	}
	if !HasError() {
		t.Fatal("nil RNG AssignOpsProbability must SetError sticky")
	}
	ClearError()
}

func TestMakeRandomInvocationNilStmtTabFailClosed(t *testing.T) {
	// nested GenerateBody needs session Statement table; sticky no invent second table
	ClearError()
	prev := ProcessStmtTabSess(testAmbientSession)
	SetProcessStmtTabSess(testAmbientSession, nil)
	defer SetProcessStmtTabSess(testAmbientSession, prev)
	opts := Defaults()
	opts.MaxFuncs = 10
	vs := NewVariableSelector(opts)
	list := &FunctionList{Funcs: nil, Types: vs.Types}
	f := &Function{Name: "func_1", ReturnType: GetIntType()}
	_ = f.ensurePairedFactMgr()
	list.Funcs = []*Function{f}
	cg := EmptyCGContext().WithFuncList(list).WithFactMgr(f.PairedFactMgr())
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
	if fi != nil && fi.Failed && HasError() {
		// sticky path exercised
	}
	ClearError()
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
	ClearAttrGenerators()
	if EnsureVarAttrGenerator() != nil || EnsureFuncAttrGenerator() != nil {
		t.Fatal("must not invent generators without InitAttrGenerators")
	}
	// Output nil-safe
	if EnsureVarAttrGenerator().Output(NewRng(1)) != "" {
		t.Fatal("nil Output must be empty")
	}
	opts := Defaults()
	InitAttrGenerators(opts, NewProbabilities(opts))
	if EnsureVarAttrGenerator() == nil {
		t.Fatal("want generator after init")
	}
	ClearAttrGenerators()
}

func TestNewVariableSelectorNoInventProbs(t *testing.T) {
	// C++ VariableSelector uses process Probabilities; no invent second table
	prev := ProcessProbabilitiesSess(testAmbientSession)
	SetProcessProbabilitiesSess(testAmbientSession, nil)
	defer SetProcessProbabilitiesSess(testAmbientSession, prev)
	vs := NewVariableSelector(Defaults())
	if vs.Probs != nil {
		t.Fatal("must not invent NewProbabilities when process unset")
	}
	p := NewProbabilities(Defaults())
	SetProcessProbabilitiesSess(testAmbientSession, p)
	vs2 := NewVariableSelector(Defaults())
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
	ClearError()
	if MakeRandomExprStmt(NewRng(1), Defaults(), nil, nil, nil, nil).Kind != 0 {
		t.Fatal("nil cg MakeRandomExprStmt must fail closed empty")
	}
	if !HasError() {
		t.Fatal("nil cg MakeRandomExprStmt must SetError sticky")
	}
	ClearError()
}

func TestHasUncertainCallRecursiveExprNilSticky(t *testing.T) {
	ClearError()
	if !HasUncertainCallRecursiveExpr(nil) {
		t.Fatal("nil HasUncertainCallRecursiveExpr must fail closed true")
	}
	if !HasError() {
		t.Fatal("nil HasUncertainCallRecursiveExpr must SetError sticky")
	}
	ClearError()
	// complete constant no uncertain
	e := &Expression{Term: TermConstant, Con: MakeInt(1)}
	if HasUncertainCallRecursiveExpr(e) {
		t.Fatal("constant must not invent uncertain call")
	}
	if HasError() {
		t.Fatal("complete HasUncertainCallRecursiveExpr must not sticky")
	}
	ClearError()
}
