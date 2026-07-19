package csmith

import "testing"

func TestPickTermTypeNilTablesFailClosed(t *testing.T) {
	// Expression::InitProbabilityTables always live; no invent NewExprTables
	// when both arg and process tables are missing
	prev := ProcessExprTables()
	SetProcessExprTables(nil)
	defer SetProcessExprTables(prev)
	tt := PickTermType(NewRng(1), nil, Defaults(), GetIntType(), false, false, 0)
	if tt != MaxTermTypes {
		t.Fatalf("want MaxTermTypes, got %v", tt)
	}
	tt = PickParamTermType(NewRng(1), nil, Defaults(), GetIntType(), 0)
	if tt != MaxTermTypes {
		t.Fatalf("param want MaxTermTypes, got %v", tt)
	}
}

func TestMakeRandomAssignUsesProcessAssignOpsTable(t *testing.T) {
	// StatementAssign::assignOpsTable_ sticky; no invent per assign
	ClearError()
	prev := ProcessAssignOpsTable()
	SetProcessAssignOpsTable(nil)
	defer SetProcessAssignOpsTable(prev)
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
	// nested GenerateBody needs session Statement table; no invent second table
	prev := ProcessStmtTab()
	SetProcessStmtTab(nil)
	defer SetProcessStmtTab(prev)
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
	// without ProcessStmtTab, create-new path fails closed; may pick existing or fail
	if fi != nil && !fi.Failed && fi.User != nil && fi.User != f && fi.User.Body != nil {
		// if somehow built without stmt tab, bad
		if ProcessStmtTab() == nil && fi.User.BuildState == BuildBuilt {
			t.Fatal("must not invent body without session StmtTab")
		}
	}
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
	prev := ProcessProbabilities()
	SetProcessProbabilities(nil)
	defer SetProcessProbabilities(prev)
	vs := NewVariableSelector(Defaults())
	if vs.Probs != nil {
		t.Fatal("must not invent NewProbabilities when process unset")
	}
	p := NewProbabilities(Defaults())
	SetProcessProbabilities(p)
	vs2 := NewVariableSelector(Defaults())
	if vs2.Probs != p {
		t.Fatal("must share process Probabilities")
	}
}

func TestBinaryOpsFilterNoInventProbs(t *testing.T) {
	prev := ProcessProbabilities()
	SetProcessProbabilities(nil)
	defer SetProcessProbabilities(prev)
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
	prevP := ProcessProbabilities()
	prevS := ProcessStmtTab()
	defer func() {
		SetProcessProbabilities(prevP)
		SetProcessStmtTab(prevS)
	}()
	g := NewProgramGenerator(opts)
	if g.StmtTab != g.Probs.StatementThresholdTable() {
		t.Fatal("generator StmtTab must be probs statement table")
	}
	if ProcessStmtTab() != g.StmtTab {
		t.Fatal("ProcessStmtTab must share generator table")
	}
}
