package csmith

import "testing"

func TestStatementProbabilitySeed2(t *testing.T) {
	tab := NewStatementThresholdTable(Defaults())
	r := NewRng(2)
	// first RndUpto(100) seed2 = 1959434203 % 100 = 3 → IfElse
	st := StatementProbability(r, tab)
	if st != StmtIfElse {
		t.Fatalf("got %v want IfElse", st)
	}
}

func TestStatementProbabilityFilterRejectCompound(t *testing.T) {
	// Simulate max block depth: reject compound types via custom filter on the U100 value.
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
	// nil CGContext — no invent Kind-only shell
	st2 := makeRandomStmtKind(NewRng(1), opts, nil, nil, nil, nil, nil, nil, StmtAssign)
	if stmtOK(st2) || st2.Kind != 0 {
		t.Fatalf("nil cg soft invent %#v", st2)
	}
	// nil RNG — no invent Kind-only shell
	st3 := makeRandomStmtKind(nil, opts, NewProbabilities(opts), NewVariableSelector(opts),
		NewExprTables(opts), NewStatementThresholdTable(opts), &cg, blk, StmtAssign)
	if stmtOK(st3) || st3.Kind != 0 {
		t.Fatalf("nil RNG soft invent %#v", st3)
	}
}

func TestStmtOKBlockRequiresThen(t *testing.T) {
	if stmtOK(Stmt{Kind: StmtBlock}) {
		t.Fatal("StmtBlock without Then must fail stmtOK")
	}
	if !stmtOK(Stmt{Kind: StmtBlock, Then: &Block{}}) {
		t.Fatal("StmtBlock with Then is usable")
	}
}
