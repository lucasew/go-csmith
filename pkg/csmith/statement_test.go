package csmith

import "testing"

func TestStatementProbabilitySeed2(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	tab := NewStatementThresholdTable(Defaults())
	r := NewRngSess(testAmbientSession, 2)
	// first RndUpto(100) seed2 = 1959434203 % 100 = 3 → IfElse
	st := StatementProbability(r, tab)
	if st != StmtIfElse {
		t.Fatalf("got %v want IfElse", st)
	}
	// nil table sticky MAX
	ClearErrorSess(testAmbientSession)
	if StatementProbability(NewRngSess(testAmbientSession, 1), nil) != MaxStatementType {
		t.Fatal("nil table must fail closed MAX")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil table StatementProbability must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestStatementProbabilityFilterRejectCompound(t *testing.T) {
	// Simulate max block depth: reject compound types via custom filter on the U100 value.
	ClearErrorSess(testAmbientSession)
	tab := NewStatementThresholdTable(Defaults())
	// Filter that rejects values mapping to compound statements
	f := filterFunc(func(v uint32) bool {
		return IsCompound(NumberToType(tab, v))
	})
	r := NewRngSess(testAmbientSession, 2)
	for i := 0; i < 30; i++ {
		st := StatementProbabilityFilter(r, tab, f)
		if IsCompound(st) {
			t.Fatalf("compound slipped through: %v", st)
		}
	}
	ClearErrorSess(testAmbientSession)
}

func TestIsCompound(t *testing.T) {
	if !IsCompound(StmtFor) || IsCompound(StmtAssign) {
		t.Fatal("is_compound")
	}
}

func TestMakeRandomStmtKindUnknownFailClosed(t *testing.T) {
	// Statement.cpp:275–277 — assert(!"unknown Statement type"); no invent shell
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	opts := Defaults()
	f := &Function{Name: "f", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	st := makeRandomStmtKind(NewRngSess(testAmbientSession, 1), opts, NewProbabilities(opts), NewVariableSelector(testAmbientSession, opts),
		NewExprTables(opts), NewStatementThresholdTable(opts), &cg, blk, MaxStatementType)
	if stmtOK(st) {
		t.Fatal("unknown kind must not invent usable stmt")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("unknown kind must set sticky error like assert")
	}
	ClearErrorSess(testAmbientSession)
	// nil CGContext sticky — no invent Kind-only shell
	st2 := makeRandomStmtKind(NewRngSess(testAmbientSession, 1), opts, nil, nil, nil, nil, nil, nil, StmtAssign)
	if stmtOK(st2) || st2.Kind != 0 {
		t.Fatalf("nil cg soft invent %#v", st2)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil cg makeRandomStmtKind must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nil RNG sticky — no invent Kind-only shell
	st3 := makeRandomStmtKind(nil, opts, NewProbabilities(opts), NewVariableSelector(testAmbientSession, opts),
		NewExprTables(opts), NewStatementThresholdTable(opts), &cg, blk, StmtAssign)
	if stmtOK(st3) || st3.Kind != 0 {
		t.Fatalf("nil RNG soft invent %#v", st3)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG makeRandomStmtKind must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
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
	iv := CreateVariableScalarsSess(testAmbientSession, "i", GetIntType(), false, false)
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
	// StatementArrayOp.cpp make_random_array_init: numeric LoopControl + IV + body
	// (no InitStmt/TestExpr/IncrStmt — those are StatementFor only).
	// Rejecting this shape soft-fails ArrayOp and re-picks the statement kind
	// (seed-2 e33136: first_div after unfair ArrayOp drop).
	if stmtOK(Stmt{Kind: StmtArrayOp, Loop: &LoopControl{IV: iv}}) {
		t.Fatal("array-op IV-only without body must fail stmtOK")
	}
	if !stmtOK(Stmt{Kind: StmtArrayOp, Loop: &LoopControl{
		IV: iv, InitN: 0, LimitN: 4, IncrN: 1, TestOp: BinCmpLt, IncrOp: AssignAdd,
	}, Then: &Block{}, ArrayAccess: "a[i]"}) {
		t.Fatal("array-init ArrayOp (numeric loop + body) must pass stmtOK")
	}
	if stmtOK(Stmt{Kind: StmtArrayOp, Then: &Block{}}) {
		t.Fatal("array-op body without Loop/ArrayAccess must fail stmtOK")
	}
}
