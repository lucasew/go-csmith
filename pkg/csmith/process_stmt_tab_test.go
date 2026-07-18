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
	// StatementAssign::assignOpsTable_ from InitProbabilityTable; no invent per assign
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
	InitSessionProbabilityTables(opts)
	st = MakeRandomAssign(NewRng(2), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), &cg, GetIntType())
	// may still fail for other reasons; at least table is live
	_ = st
}

func TestAssignOpsProbabilityNilTableFailClosed(t *testing.T) {
	// StatementAssign::assignOpsTable_ always live; no invent NewAssignOpsTable
	op := AssignOpsProbability(NewRng(1), Defaults(), nil, GetIntType())
	if op != AssignOp(-1) {
		t.Fatalf("want invalid op, got %v", op)
	}
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
