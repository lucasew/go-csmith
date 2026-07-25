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
	SetErrorSess(testAmbientSession, ErrCompatibleCheck)
	st := makeRandomStmt(NewRng(1), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg, blk)
	if stmtOK(st) {
		t.Fatal("ERROR_GUARD must yield null stmt")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("error must remain sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Statement.cpp always has RNG sticky; no invent MAX-kind shell without it
	st2 := makeRandomStmt(nil, opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg, blk)
	if stmtOK(st2) || st2.Kind != 0 {
		t.Fatalf("nil RNG soft invent %#v", st2)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG makeRandomStmt must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomBlockRequiresCurrentFunc(t *testing.T) {
	// Block.cpp:120 — assert(curr_func) sticky; no soft invent parentless block
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	cg := EmptyCGContext()
	if MakeRandomBlock(NewRng(1), opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), NewStatementThresholdTable(opts), &cg, false) != nil {
		t.Fatal("nil CurrentFunc must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil CurrentFunc must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if MakeRandomBlock(nil, opts, NewProbabilities(opts), NewVariableSelector(opts), NewExprTables(opts), NewStatementThresholdTable(opts), &cg, false) != nil {
		t.Fatal("nil RNG must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil RNG MakeRandomBlock must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomBlockAbortsOnStickyError(t *testing.T) {
	// Block.cpp:157–161 — error after stmts → delete block (nil)
	opts := Defaults()
	opts.MaxBlockSize = 0
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	cg := WithFunc(f, EmptyEffect())
	SetErrorSess(testAmbientSession, ErrGeneric)
	b := MakeRandomBlock(NewRng(1), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg, false)
	if b != nil {
		t.Fatal("want nil block on sticky error")
	}
	if len(f.Stack) != 0 {
		t.Fatal("stack must be popped")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomBlockClearsErrorOnSuccess(t *testing.T) {
	// Block.cpp:187 — set_error(SUCCESS) on success
	opts := Defaults()
	opts.MaxBlockSize = 1
	vs := NewVariableSelector(opts)
	_ = vs.GenerateNewGlobal(AccessWrite, EmptyCGContext(), GetIntType(), nil, NewRng(1))
	f := &Function{Name: "f", ReturnType: GetIntType()}
	cg := WithFunc(f, EmptyEffect())
	cg.Types = &TypeEnv{Sess: testAmbientSession}
	tab := &ThresholdTable{}
	tab.Add(100, int(StmtAssign))
	ClearErrorSess(testAmbientSession)
	b := MakeRandomBlock(NewRng(2), opts, NewProbabilities(opts), vs, NewExprTables(opts), tab, &cg, false)
	if b == nil {
		t.Fatal("nil block")
	}
	if HasErrorSess(testAmbientSession) {
		t.Fatal("success must ClearError")
	}
}

func TestMakeRandomBlockIncompleteFailClosed(t *testing.T) {
	// incomplete EffectAccum / GlobalFacts / EffectContext must not invent block success
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.MaxBlockSize = 0
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetSimpleType(EVoid)}
	fm := NewFactMgrSess(testAmbientSession, f)
	inc := IncompleteEffect()
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &inc
	cg.Types = &TypeEnv{Sess: testAmbientSession}
	if MakeRandomBlock(NewRng(1), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg, false) != nil {
		t.Fatal("incomplete EffectAccum must fail closed MakeRandomBlock")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("must SetError")
	}
	if len(f.Blocks) != 0 || len(f.Stack) != 0 {
		t.Fatal("must not leave partial registration")
	}
	ClearErrorSess(testAmbientSession)
	f2 := &Function{Name: "f2", ReturnType: GetSimpleType(EVoid)}
	fm2 := NewFactMgrSess(testAmbientSession, f2)
	fm2.GlobalFacts = IncompleteFactSlice()
	cg2 := WithFunc(f2, EmptyEffect()).WithFactMgr(fm2)
	cg2.Types = &TypeEnv{Sess: testAmbientSession}
	if MakeRandomBlock(NewRng(2), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg2, false) != nil {
		t.Fatal("incomplete GlobalFacts must fail closed MakeRandomBlock")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("must SetError GlobalFacts")
	}
	ClearErrorSess(testAmbientSession)
	// Function + Block always live on make abort; sticky
	abortBlockMake(nil, &Block{})
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil func abortBlockMake must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	abortBlockMake(&Function{Name: "f"}, nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil block abortBlockMake must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	f3 := &Function{Name: "f3", ReturnType: GetSimpleType(EVoid)}
	cg3 := WithFunc(f3, IncompleteEffect()).WithFactMgr(NewFactMgrSess(testAmbientSession, f3))
	eff := EmptyEffect()
	cg3.EffectAccum = &eff
	cg3.Types = &TypeEnv{Sess: testAmbientSession}
	if MakeRandomBlock(NewRng(3), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg3, false) != nil {
		t.Fatal("incomplete EffectContext must fail closed MakeRandomBlock")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("must SetError EffectContext")
	}
	if len(f3.Blocks) != 0 || len(f3.Stack) != 0 {
		t.Fatal("must not leave partial registration on incomplete context")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeRandomStmtIncompletePreFailClosed(t *testing.T) {
	// incomplete GlobalFacts/accum/context pre-snapshot must sticky ERROR (no invent re-pick past holes)
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	opts.MaxBlockSize = 1
	vs := NewVariableSelector(opts)
	f := &Function{Name: "f", ReturnType: GetIntType()}
	blk := &Block{Func: f}
	f.Stack = []*Block{blk}
	fm := NewFactMgrSess(testAmbientSession, f)
	fm.GlobalFacts = IncompleteFactSlice()
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	st := makeRandomStmt(NewRng(1), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg, blk)
	if stmtOK(st) {
		t.Fatal("incomplete GlobalFacts must fail closed makeRandomStmt")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete GlobalFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	inc := IncompleteEffect()
	fm2 := NewFactMgrSess(testAmbientSession, f)
	cg2 := WithFunc(f, EmptyEffect()).WithFactMgr(fm2)
	cg2.EffectAccum = &inc
	st2 := makeRandomStmt(NewRng(2), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg2, blk)
	if stmtOK(st2) {
		t.Fatal("incomplete EffectAccum must fail closed makeRandomStmt")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectAccum must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete EffectContext fails closed sticky before re-pick loop
	cg3 := WithFunc(f, IncompleteEffect()).WithFactMgr(NewFactMgrSess(testAmbientSession, f))
	eff3 := EmptyEffect()
	cg3.EffectAccum = &eff3
	st3 := makeRandomStmt(NewRng(3), opts, NewProbabilities(opts), vs, NewExprTables(opts), NewStatementThresholdTable(opts), &cg3, blk)
	if stmtOK(st3) {
		t.Fatal("incomplete EffectContext must fail closed makeRandomStmt")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete EffectContext must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}
