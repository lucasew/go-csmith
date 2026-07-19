package csmith

import "testing"

func TestStatementProbabilitySeed2(t *testing.T) {
	ClearError()
	tab := NewStatementThresholdTable(Defaults())
	r := NewRng(2)
	// first RndUpto(100) seed2 = 1959434203 % 100 = 3 → IfElse
	st := StatementProbability(r, tab)
	if st != StmtIfElse {
		t.Fatalf("got %v want IfElse", st)
	}
	// nil table sticky MAX
	ClearError()
	if StatementProbability(NewRng(1), nil) != MaxStatementType {
		t.Fatal("nil table must fail closed MAX")
	}
	if !HasError() {
		t.Fatal("nil table StatementProbability must SetError sticky")
	}
	ClearError()
}

func TestStatementProbabilityFilterRejectCompound(t *testing.T) {
	// Simulate max block depth: reject compound types via custom filter on the U100 value.
	ClearError()
	tab := NewStatementThresholdTable(Defaults())
	// Filter that rejects values mapping to compound statements
	f := filterFunc(func(v uint32) bool {
		return IsCompound(NumberToType(tab, v))
	})
	r := NewRng(2)
	for i := 0; i < 30; i++ {
		st := StatementProbabilityFilter(r, tab, f)
		if IsCompound(st) {
			t.Fatalf("compound slipped through: %v", st)
		}
	}
	ClearError()
}

func TestIsCompound(t *testing.T) {
	if !IsCompound(StmtFor) || IsCompound(StmtAssign) {
		t.Fatal("is_compound")
	}
}

func TestMakeRandomStmtKindUnknownFailClosed(t *testing.T) {
	// Statement.cpp:275–277 — assert(!"unknown Statement type"); no invent shell
	ClearError()
	defer ClearError()
	opts := Defaults()
	f := &Function{Name: "f", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(NewFactMgr(f))
	st := makeRandomStmtKind(NewRng(1), opts, NewProbabilities(opts), NewVariableSelector(opts),
		NewExprTables(opts), NewStatementThresholdTable(opts), &cg, blk, MaxStatementType)
	if stmtOK(st) {
		t.Fatal("unknown kind must not invent usable stmt")
	}
	if !HasError() {
		t.Fatal("unknown kind must set sticky error like assert")
	}
	ClearError()
	// nil CGContext sticky — no invent Kind-only shell
	st2 := makeRandomStmtKind(NewRng(1), opts, nil, nil, nil, nil, nil, nil, StmtAssign)
	if stmtOK(st2) || st2.Kind != 0 {
		t.Fatalf("nil cg soft invent %#v", st2)
	}
	if !HasError() {
		t.Fatal("nil cg makeRandomStmtKind must SetError sticky")
	}
	ClearError()
	// nil RNG sticky — no invent Kind-only shell
	st3 := makeRandomStmtKind(nil, opts, NewProbabilities(opts), NewVariableSelector(opts),
		NewExprTables(opts), NewStatementThresholdTable(opts), &cg, blk, StmtAssign)
	if stmtOK(st3) || st3.Kind != 0 {
		t.Fatalf("nil RNG soft invent %#v", st3)
	}
	if !HasError() {
		t.Fatal("nil RNG makeRandomStmtKind must SetError sticky")
	}
	ClearError()
}

func TestStmtOKBlockRequiresThen(t *testing.T) {
	if stmtOK(Stmt{Kind: StmtBlock}) {
		t.Fatal("StmtBlock without Then must fail stmtOK")
	}
	if !stmtOK(Stmt{Kind: StmtBlock, Then: &Block{}}) {
		t.Fatal("StmtBlock with Then is usable")
	}
}

func TestStmtOKIncompleteForIfAssignFailClosed(t *testing.T) {
	// no invent usable shells from partial IR
	iv := CreateVariableScalars("i", GetIntType(), false, false)
	if stmtOK(Stmt{Kind: StmtFor, Loop: &LoopControl{IV: iv}}) {
		t.Fatal("for IV-only must fail stmtOK")
	}
	if stmtOK(Stmt{Kind: StmtFor, Loop: &LoopControl{
		IV: iv, InitStmt: &Stmt{Kind: StmtAssign}, TestExpr: &Expression{Term: TermConstant, Con: MakeInt(1)},
	}, Then: &Block{}}) {
		t.Fatal("for missing IncrStmt must fail stmtOK")
	}
	// complete for shape
	init := &Stmt{Kind: StmtAssign, LhsVar: iv, Expr: &Expression{Term: TermConstant, Con: MakeInt(0)}}
	incr := &Stmt{Kind: StmtAssign, LhsVar: iv, Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}}
	test := &Expression{Term: TermConstant, Con: MakeInt(1)}
	if !stmtOK(Stmt{Kind: StmtFor, Loop: &LoopControl{
		IV: iv, InitStmt: init, TestExpr: test, IncrStmt: incr,
	}, Then: &Block{}}) {
		t.Fatal("complete for must pass stmtOK")
	}
	// if missing Else/test
	if stmtOK(Stmt{Kind: StmtIfElse, Then: &Block{}}) {
		t.Fatal("if without Expr/Else must fail stmtOK")
	}
	if stmtOK(Stmt{Kind: StmtIfElse, Expr: test, Then: &Block{}}) {
		t.Fatal("if without Else must fail stmtOK")
	}
	if !stmtOK(Stmt{Kind: StmtIfElse, Expr: test, Then: &Block{}, Else: &Block{}}) {
		t.Fatal("complete if must pass")
	}
	// assign without RHS
	if stmtOK(Stmt{Kind: StmtAssign, LhsVar: iv}) {
		t.Fatal("assign without Expr must fail stmtOK")
	}
	if !stmtOK(Stmt{Kind: StmtAssign, LhsVar: iv, Expr: test}) {
		t.Fatal("assign with lhs+rhs must pass")
	}
	// goto without test
	if stmtOK(Stmt{Kind: StmtGoto, Label: "lbl"}) {
		t.Fatal("goto without Expr must fail stmtOK")
	}
	if !stmtOK(Stmt{Kind: StmtGoto, Label: "lbl", Expr: test}) {
		t.Fatal("goto with label+test must pass")
	}
}
