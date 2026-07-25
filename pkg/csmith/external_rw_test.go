package csmith

import (
	"testing"
)

func TestGetExternalNoReadsWrites(t *testing.T) {
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	loc := CreateVariableScalarsSess(testAmbientSession, "l_1", GetIntTypeSess(testAmbientSession), false, false)
	cg := EmptyCGContext().WithSession(testAmbientSession).WithRW(&RWDirective{
		NoReadVars:  []*Variable{g, loc},
		NoWriteVars: []*Variable{g},
	})
	// without frame, only globals
	nr, nw := cg.GetExternalNoReadsWrites(nil)
	if len(nr) != 1 || nr[0] != g {
		t.Fatal("nr", nr)
	}
	if len(nw) != 1 || nw[0] != g {
		t.Fatal("nw", nw)
	}
	// with frame includes local
	nr, nw = cg.GetExternalNoReadsWrites([]*Variable{loc})
	if len(nr) != 2 {
		t.Fatal("frame nr", nr)
	}
	_ = nw
	// nil RW hole fails closed incomplete (not bare nil invent empty complete)
	cg2 := EmptyCGContext().WithSession(testAmbientSession).WithRW(&RWDirective{NoReadVars: []*Variable{nil, g}})
	nr, nw = cg2.GetExternalNoReadsWrites(nil)
	if VariablesComplete(nr) || VariablesComplete(nw) {
		t.Fatal("nil NoReadVars hole must fail closed incomplete", nr, nw)
	}
}

func TestFindMustUseArraysNilHole(t *testing.T) {
	rw := &RWDirective{MustReadVars: []*Variable{nil}}
	if rw.FindMustUseArraysSess(testAmbientSession) != nil {
		t.Fatal("nil must-use hole must fail closed")
	}
}

func TestFindRelatedPointToNilHole(t *testing.T) {
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	facts := []*FactPointTo{nil, MakeFactPointToSess(testAmbientSession, p, NullPtr)}
	if FindRelatedPointToSess(testAmbientSession, facts, p) != nil {
		t.Fatal("nil fact hole must fail closed (no invent skip to later)")
	}
	if FindRelatedUnionSess(testAmbientSession, []*FactUnion{nil}, p) != nil {
		t.Fatal("nil union fact hole must fail closed")
	}
}

func TestPtrModifiedInRhsNilPointees(t *testing.T) {
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	lhs := &Lhs{Var: p, Type: GetIntTypeSess(testAmbientSession)} // indir via type peel — set IndirectLevel path
	// force multi-level via Type pointer chain
	pt := PointerToSess(testAmbientSession, PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)))
	pp := CreateVariableScalarsSess(testAmbientSession, "g_pp", pt, true, false)
	lhs2 := &Lhs{Var: pp, Type: GetIntTypeSess(testAmbientSession)}
	// IndirectLevel for Lhs
	if lhs2.IndirectLevelSess(testAmbientSession) <= 1 {
		// still exercise MergePointees nil path via incomplete facts with hole
		_ = lhs
	}
	cg := EmptyCGContext().WithSession(testAmbientSession)
	// incomplete merge via nil in pointees of related fact
	facts := []*FactPointTo{{Var: pp, PointTo: []*Variable{nil}}}
	// for indir > 1 need multi-level pointer
	// Lhs IndirectLevel = var.Type.IndirectLevel - want.IndirectLevel
	// pp is **int, Type GetIntType → indir 2
	if lhs2.IndirectLevelSess(testAmbientSession) > 1 {
		if !cg.PtrModifiedInRhs(lhs2, facts) {
			t.Fatal("nil pointee hole must fail closed as modified")
		}
	}
}

func TestGetExternalNoWritesFromIV(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	g := CreateVariableScalarsSess(testAmbientSession, "g_i", GetIntTypeSess(testAmbientSession), false, false)
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.AddIVBound(g, 10)
	_, nw := cg.GetExternalNoReadsWrites(nil)
	if len(nw) != 1 || nw[0] != g {
		t.Fatal(nw)
	}
}

func TestBuildCalleeRWDirective(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	cg := EmptyCGContext().WithSession(testAmbientSession).WithRW(&RWDirective{NoWriteVars: []*Variable{g}})
	rwd := cg.BuildCalleeRWDirective(nil)
	if rwd == nil || len(rwd.NoWriteVars) != 1 {
		t.Fatal(rwd)
	}
}

func TestFindReachableFrameVarsCompleteEmpty(t *testing.T) {
	// complete empty must be non-nil empty (not invent nil==incomplete)
	cg := EmptyCGContext().WithSession(testAmbientSession)
	got := cg.FindReachableFrameVars(nil)
	if got == nil {
		t.Fatal("complete empty must be non-nil empty slice")
	}
	if len(got) != 0 {
		t.Fatal(got)
	}
	// incomplete fact map fails closed nil
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	if VariablesComplete(cg.FindReachableFrameVars([]*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr), nil})) {
		t.Fatal("incomplete facts must fail closed incomplete")
	}
}

func TestBuildCalleeRWDirectiveIncompleteFactsFailClosed(t *testing.T) {
	// soft invent: incomplete frame → nil RW (no restrictions)
	// fair: inherit full NoWrite without inventing unrestricted nil; sticky
	ClearErrorSess(testAmbientSession)
	g := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	cg := EmptyCGContext().WithSession(testAmbientSession).WithRW(&RWDirective{NoWriteVars: []*Variable{g}})
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	rwd := cg.BuildCalleeRWDirective([]*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr), nil})
	if rwd == nil {
		t.Fatal("incomplete must not invent nil unrestricted RW")
	}
	if len(rwd.NoWriteVars) != 1 || rwd.NoWriteVars[0] != g {
		t.Fatal(rwd)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete frame facts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestVisitFactsInvocationParams(t *testing.T) {
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), false, false)
	b := CreateVariableScalarsSess(testAmbientSession, "g_b", GetIntTypeSess(testAmbientSession), false, false)
	fi := &Invocation{
		Args: []*Expression{
			{Term: TermVariable, Var: a, ExprType: GetIntTypeSess(testAmbientSession)},
			{Term: TermVariable, Var: b, ExprType: GetIntTypeSess(testAmbientSession)},
		},
	}
	eff := EmptyEffect()
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.EffectAccum = &eff
	if !VisitFactsInvocation(fi, &cg, Defaults()) {
		t.Fatal("visit")
	}
	if !eff.IsReadSess(testAmbientSession, a) || !eff.IsReadSess(testAmbientSession, b) {
		t.Fatal("reads")
	}
}

func TestVisitFactsInvocationArgResidualSticky(t *testing.T) {
	// Visit residual soft invent was soft-continue later args invent visit success.
	ClearErrorSess(testAmbientSession)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), false, false)
	hole := &Expression{Term: TermConstant, Con: &Constant{Type: GetIntTypeSess(testAmbientSession)}}
	fi := &Invocation{
		Args: []*Expression{
			{Term: TermVariable, Var: a, ExprType: GetIntTypeSess(testAmbientSession)},
			hole,
		},
	}
	eff := EmptyEffect()
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.EffectAccum = &eff
	if VisitFactsInvocation(fi, &cg, Defaults()) {
		t.Fatal("arg visit residual must fail closed VisitFactsInvocation")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("arg visit residual must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestVisitFactsInvocationAlwaysRevisitsUser(t *testing.T) {
	// FunctionInvocation.cpp:530–551 — always revisit user callees in visit_facts.
	ClearErrorSess(testAmbientSession)
	SetProcessOptionsSess(testAmbientSession, Defaults())
	callee := &Function{Name: "c", ReturnType: GetIntTypeSess(testAmbientSession), BuildState: BuildBuilt, IsBuilt: true}
	callee.Body = &Block{StmID: 50, Func: callee, Stmts: nil}
	fm := callee.ensurePairedFactMgrSess(testAmbientSession)
	// NeedsRevisit false (no FactChanged / ptrs) — visit_facts still revisits
	if callee.NeedsRevisitSess(testAmbientSession) {
		t.Fatal("fixture must not NeedsRevisit; testing always-revisit gate")
	}
	fi := &Invocation{User: callee}
	caller := &Function{Name: "caller", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{StmID: 1, Func: caller}
	caller.Stack = []*Block{blk}
	// caller FM for GlobalFacts work set; revisit uses callee.PairedFactMgr
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, caller))
	cg.CurrentFunc = caller
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if !VisitFactsInvocation(fi, &cg, Defaults()) {
		t.Fatalf("always-revisit empty body must ok err=%v", HasErrorSess(testAmbientSession))
	}
	// body maps recorded by find_fixed_point
	if !FactsComplete(fm.GetMapFactsIn(50)) && !FactsComplete(fm.GetMapFactsOut(50)) {
		// empty complete is ok; incomplete would be wrong
		t.Fatal("revisit must install complete body fact maps")
	}
	ClearErrorSess(testAmbientSession)
}

// FunctionInvocation.cpp:536–541 — visit_facts builds
//
//	CGContext new_context(cg, callee, effect_context, &fresh_accum)
//
// before revisit. Parent CurrRHS / EffectAccum must not leak into nested body
// visit (Lhs.cpp:318–328 overlap uses curr_rhs; effect_accum shares would
// corrupt the outer StatementAssign analysis — seed-2 func_49 e37241).
func TestVisitFactsInvocationUsesFreshCalleeContext(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	SetProcessOptionsSess(testAmbientSession, Defaults())
	g := CreateVariableScalarsSess(testAmbientSession, "g_overlap", GetIntTypeSess(testAmbientSession), false, false)
	callee := &Function{Name: "c", ReturnType: GetIntTypeSess(testAmbientSession), BuildState: BuildBuilt, IsBuilt: true}
	// body: g_overlap = 1
	st := Stmt{
		Kind: StmtAssign, StmID: 51, AssignOp: AssignSimple,
		LhsVar: g, Lhs: &Lhs{Var: g, Type: GetIntTypeSess(testAmbientSession)},
		Expr: &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)},
	}
	callee.Body = &Block{StmID: 50, Func: callee, Stmts: []Stmt{st}}
	_ = callee.ensurePairedFactMgrSess(testAmbientSession)
	fi := &Invocation{User: callee}
	caller := &Function{Name: "caller", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{StmID: 1, Func: caller, LocalVars: nil}
	caller.Stack = []*Block{blk}
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, caller))
	cg.CurrentFunc = caller
	// Outer assign-like pollution that must not reach nested Lhs visit.
	outerEff := EmptyEffect().WriteVarSess(testAmbientSession, g)
	cg.EffectAccum = &outerEff
	cg.CurrRHS = &Expression{Term: TermVariable, Var: g, ExprType: GetIntTypeSess(testAmbientSession)}
	// Ambient context already has g written — nested body write of g would fail
	// CheckWriteVar if effect_context wrongly included that write via shared state.
	// C++ new_context keeps parent effect_context (same ambient) but fresh accum.
	// Ambient write of g makes body assign fail for both — so clear ambient, only pollute accum/rhs.
	cg.effectContext = EmptyEffect()
	if !VisitFactsInvocation(fi, &cg, Defaults()) {
		t.Fatalf("nested revisit with polluted parent CurrRHS/accum must still ok; err=%v", HasErrorSess(testAmbientSession))
	}
	// Parent CurrRHS must remain set (new_context is a clone).
	if cg.CurrRHS == nil || cg.CurrRHS.Var != g {
		t.Fatal("parent CurrRHS must be unchanged after nested revisit")
	}
	ClearErrorSess(testAmbientSession)
}

func TestVisitFactsInvocationConflict(t *testing.T) {
	// Legacy name kept: ambient write conflict on static path removed with always-revisit.
	// Soft analysis fail remains non-sticky (RevisitUserInvocation ClearError on body fail).
	ClearErrorSess(testAmbientSession)
	g := CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntTypeSess(testAmbientSession), false, false)
	callee := &Function{Name: "c", ReturnType: GetIntTypeSess(testAmbientSession), BuildState: BuildBuilt, IsBuilt: true}
	// body with *p write under may-null would fail; use incomplete as soft fail
	callee.Body = &Block{StmID: 50, Func: callee, Stmts: []Stmt{{Kind: StmtAssign, StmID: IncompleteStmID}}} // StmID 0 sticky fail
	_ = callee.ensurePairedFactMgrSess(testAmbientSession)
	fi := &Invocation{User: callee}
	caller := &Function{Name: "caller", ReturnType: GetIntTypeSess(testAmbientSession)}
	blk := &Block{StmID: 1, Func: caller}
	caller.Stack = []*Block{blk}
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(NewFactMgrSess(testAmbientSession, caller))
	cg.CurrentFunc = caller
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	_ = g
	if VisitFactsInvocation(fi, &cg, Defaults()) {
		t.Fatal("revisit of broken body must fail analysis")
	}
	// soft fail clears sticky ERROR
	ClearErrorSess(testAmbientSession)
}

func TestFactMgrMapStmEffect(t *testing.T) {
	fm := NewFactMgrSess(testAmbientSession, nil)
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	eff := EmptyEffect().WriteVarSess(testAmbientSession, v)
	fm.SetMapStmEffect(3, eff)
	got := fm.GetMapStmEffect(3)
	if !got.IsWrittenSess(testAmbientSession, v) {
		t.Fatal("map")
	}
	if fm.GetMapStmEffect(9).IsWrittenSess(testAmbientSession, v) {
		t.Fatal("empty")
	}
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	facts := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)}
	fm.SetMapFactsIn(1, facts)
	fm.SetMapFactsOut(1, facts)
	if len(fm.MapFactsIn[1]) != 1 || len(fm.MapFactsOut[1]) != 1 {
		t.Fatal("facts maps")
	}
}

func TestVisitFactsBlockRecordsMaps(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	st := Stmt{
		Kind: StmtAssign, StmID: 7, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntTypeSess(testAmbientSession)},
		Expr: &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)}, AssignOp: AssignSimple,
	}
	// Block::stm_id always live when FM bound
	b := &Block{StmID: 5, Stmts: []Stmt{st}}
	fm := NewFactMgrSess(testAmbientSession, nil)
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if !VisitFactsBlock(b, &cg, Defaults()) {
		t.Fatal("block")
	}
	if _, ok := fm.MapFactsIn[7]; !ok {
		t.Fatal("facts_in")
	}
	if _, ok := fm.MapFactsOut[7]; !ok {
		t.Fatal("facts_out")
	}
}

// TestVisitFactsInvocationIgnoresFailedFlag — FunctionInvocation.cpp:502–555.
// visit_facts does not consult failed; generation-time Failed is not analysis fail.
func TestVisitFactsInvocationIgnoresFailedFlag(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), false, false)
	fi := &Invocation{
		Failed: true,
		Args: []*Expression{
			{Term: TermVariable, Var: a, ExprType: GetIntTypeSess(testAmbientSession)},
		},
	}
	eff := EmptyEffect()
	cg := EmptyCGContext().WithSession(testAmbientSession)
	cg.EffectAccum = &eff
	if !VisitFactsInvocation(fi, &cg, Defaults()) {
		t.Fatalf("visit_facts must analyze despite Failed=true err=%v", HasErrorSess(testAmbientSession))
	}
	if !eff.IsReadSess(testAmbientSession, a) {
		t.Fatal("must still visit args when Failed")
	}
	ClearErrorSess(testAmbientSession)
}
