package csmith

import "testing"

func TestMakeRandomStmtErrorGuardNoRepick(t *testing.T) {
	// Statement.cpp:309 — ERROR_GUARD after factory; sticky error skips re-pick
	opts := Defaults()
	opts.MaxBlockSize = 1
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	cg := WithFunc(f, EmptyEffect())
	SetError(ErrCompatibleCheck)
	st := makeRandomStmt(NewRng(1), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg, blk)
	if stmtOK(st) {
		t.Fatal("ERROR_GUARD must yield null stmt")
	}
	if !HasError() {
		t.Fatal("error must remain sticky")
	}
	ClearError()
	// Statement.cpp always has RNG; no invent MAX-kind shell without it
	st2 := makeRandomStmt(nil, opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg, blk)
	if stmtOK(st2) || st2.Kind != 0 {
		t.Fatalf("nil RNG soft invent %#v", st2)
	}
}

func TestMakeRandomBlockRequiresCurrentFunc(t *testing.T) {
	// Block.cpp:120 — assert(curr_func); no soft invent parentless block
	opts := Defaults()
	cg := EmptyCGContext()
	if MakeRandomBlock(NewRng(1), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), NewStatementThresholdTable(opts), &cg, false) != nil {
		t.Fatal("nil CurrentFunc must fail closed")
	}
}

func TestMakeRandomBlockAbortsOnStickyError(t *testing.T) {
	// Block.cpp:157–161 — error after stmts → delete block (nil)
	opts := Defaults()
	opts.MaxBlockSize = 0
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	cg := WithFunc(f, EmptyEffect())
	SetError(ErrGeneric)
	b := MakeRandomBlock(NewRng(1), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg, false)
	if b != nil {
		t.Fatal("want nil block on sticky error")
	}
	if len(f.Stack) != 0 {
		t.Fatal("stack must be popped")
	}
	ClearError()
}

func TestMakeRandomBlockClearsErrorOnSuccess(t *testing.T) {
	// Block.cpp:187 — set_error(SUCCESS) on success
	opts := Defaults()
	opts.MaxBlockSize = 1
	vs := NewVariableSelector(opts)
	_ = vs.GenerateNewGlobal(AccessWrite, EmptyCGContext(), GetIntType(), nil, NewRng(1))
	f := &Function{Name: "f", ReturnType: GetIntType()}
	cg := WithFunc(f, EmptyEffect())
	cg.Types = &TypeEnv{}
	tab := &ThresholdTable{}
	tab.Add(100, int(StmtAssign))
	ClearError()
	b := MakeRandomBlock(NewRng(2), opts, NewProbabilities(opts), vs, NewExprTables(opts), tab, &cg, false)
	if b == nil {
		t.Fatal("nil block")
	}
	if HasError() {
		t.Fatal("success must ClearError")
	}
}

func TestMakeRandomBlockIncompleteFailClosed(t *testing.T) {
	// incomplete EffectAccum / GlobalFacts must not invent block success
	ClearError()
	opts := Defaults()
	opts.MaxBlockSize = 0
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetSimpleType(EVoid)}
	fm := NewFactMgr(f)
	inc := IncompleteEffect()
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &inc
	cg.Types = &TypeEnv{}
	if MakeRandomBlock(NewRng(1), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg, false) != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomBlock")
	}
	if !HasError() {
		t.Fatal("must SetError")
	}
	if len(f.Blocks) != 0 || len(f.Stack) != 0 {
		t.Fatal("must abortBlockMake registration")
	}
	ClearError()
	f2 := &Function{Name: "f2", ReturnType: GetSimpleType(EVoid)}
	fm2 := NewFactMgr(f2)
	fm2.GlobalFacts = IncompleteFactSlice()
	cg2 := WithFunc(f2, EmptyEffect()).WithFactMgr(fm2)
	cg2.Types = &TypeEnv{}
	if MakeRandomBlock(NewRng(2), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg2, false) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed MakeRandomBlock")
	}
	if !HasError() {
		t.Fatal("must SetError GlobalFacts")
	}
	ClearError()
}
