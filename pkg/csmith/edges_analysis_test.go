package csmith

import (
	"testing"
)

func TestMergeJumpFacts(t *testing.T) {
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), false, false)
	facts := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, a)}
	jump := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)}
	if !MergeJumpFactsSess(testAmbientSession, &facts, jump) {
		t.Fatal("changed")
	}
	fp := FindRelatedPointToSess(testAmbientSession, facts, p)
	// joined set should include both
	if fp == nil || len(fp.PointTo) < 2 {
		t.Fatal(fp)
	}
}

func TestMergeJumpFactsMissingIsGarbage(t *testing.T) {
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), false, false)
	facts := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, a)}
	// jump has no fact for p → garbage join
	if !MergeJumpFactsSess(testAmbientSession, &facts, nil) {
		t.Fatal("expect garbage merge")
	}
	fp := FindRelatedPointToSess(testAmbientSession, facts, p)
	if fp == nil || !fp.IsDeadSess(testAmbientSession) {
		// garbage_ptr is IsDead
		if fp == nil || !IsVariableInSet(fp.PointTo, GarbagePtr) {
			t.Fatal(fp)
		}
	}
}

func TestFindEdgesIn(t *testing.T) {
	fm := NewFactMgrSess(testAmbientSession, nil)
	blk := &Block{}
	fm.CreateCFGEdgeTo(10, blk, 20, false, true)
	fm.CreateCFGEdgeTo(11, blk, 20, false, false)
	back := fm.FindEdgesIn(20, false, true)
	if back == nil || len(back) != 1 || back[0].SrcID != 10 {
		t.Fatal(back)
	}
	fwd := fm.FindEdgesIn(20, false, false)
	if fwd == nil || len(fwd) != 1 || fwd[0].SrcID != 11 {
		t.Fatal(fwd)
	}
	if !fm.HasEdgeIn(20, false, true) {
		t.Fatal("has")
	}
	// empty complete scan is non-nil empty (not nil = incomplete)
	none := fm.FindEdgesIn(999, false, false)
	if none == nil || len(none) != 0 {
		t.Fatal("complete empty must be non-nil empty", none)
	}
	// IncompleteStmID key sticky; valid id 0 is a complete empty scan
	ClearErrorSess(testAmbientSession)
	if got := fm.FindEdgesIn(IncompleteStmID, false, false); got != nil {
		t.Fatal("IncompleteStmID must fail closed", got)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IncompleteStmID FindEdgesIn must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	none0 := fm.FindEdgesIn(0, false, false)
	if none0 == nil || len(none0) != 0 {
		t.Fatal("valid dest id 0 empty scan must be non-nil empty", none0)
	}
	ClearErrorSess(testAmbientSession)
	if got := fm.FindEdgesInToBlock(nil, false, false); got != nil {
		t.Fatal("nil dest block must fail closed", got)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil dest FindEdgesInToBlock must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestFindEdgesInNilHoleFailClosed(t *testing.T) {
	fm := NewFactMgrSess(testAmbientSession, nil)
	fm.CFGEdges = []*CFGEdge{
		{SrcID: 10, DestStmID: 20, BackLink: true},
		nil,
		{SrcID: 11, DestStmID: 20},
	}
	// nil hole fails closed sticky (no invent soft-skip / re-pick past holes)
	ClearErrorSess(testAmbientSession)
	if got := fm.FindEdgesIn(20, false, true); got != nil {
		t.Fatal("nil CFG hole must fail closed", got)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil CFG hole FindEdgesIn must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if got := fm.FindEdgesIn(20, false, false); got != nil {
		t.Fatal("nil CFG hole must fail closed fwd", got)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil CFG hole FindEdgesIn fwd must SetError sticky")
	}
	// HasEdgeIn must not invent false from len(nil)==0 (sticky already set)
	if !fm.HasEdgeIn(20, false, true) {
		t.Fatal("incomplete FindEdgesIn must HasEdgeIn true, not invent none")
	}
	ClearErrorSess(testAmbientSession)
	b := &Block{}
	fm.CFGEdges = []*CFGEdge{nil, {SrcID: 1, DestBlock: b, BackLink: true}}
	if got := fm.FindEdgesInToBlock(b, false, true); got != nil {
		t.Fatal("nil hole FindEdgesInToBlock", got)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil hole FindEdgesInToBlock must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestHasEdgeInNilFMFailClosed(t *testing.T) {
	// assert(fm) path — sticky has-edge (no invent no-edge without FactMgr)
	ClearErrorSess(testAmbientSession)
	var fm *FactMgr
	if !fm.HasEdgeIn(1, false, true) {
		t.Fatal("nil FM must HasEdgeIn true")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM HasEdgeIn must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMergeJumpFactsNilHoleFailClosed(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	a := CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), false, false)
	facts := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, a), nil}
	jump := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)}
	// incomplete subject map must not soft-join past hole — sticky
	if MergeJumpFactsSess(testAmbientSession, &facts, jump) {
		t.Fatal("nil subject hole must fail closed")
	}
	// fail closed clears *facts — no invent leave partial / hole-bearing map
	if FactsComplete(facts) {
		t.Fatal("incomplete must clear facts", facts)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil subject hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	holeSubj := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, a), nil}
	if _, ok := tryMergeJumpFactsSess(testAmbientSession, &holeSubj, jump); ok {
		t.Fatal("tryMerge incomplete subject must ok=false")
	}
	if FactsComplete(holeSubj) {
		t.Fatal("tryMerge must clear incomplete subject", holeSubj)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("tryMerge incomplete must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	facts2 := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, a)}
	jumpHole := []*FactPointTo{nil}
	if MergeJumpFactsSess(testAmbientSession, &facts2, jumpHole) {
		t.Fatal("nil jump hole must fail closed")
	}
	if FactsComplete(facts2) {
		t.Fatal("incomplete jump must clear facts2", facts2)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil jump hole must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// facts out always live; sticky (no invent soft-skip jump merge past hole)
	if MergeJumpFactsSess(testAmbientSession, nil, jump) {
		t.Fatal("nil facts MergeJumpFacts must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil facts MergeJumpFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if _, ok := tryMergeJumpFactsSess(testAmbientSession, nil, jump); ok {
		t.Fatal("nil facts tryMergeJumpFacts must ok=false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil facts tryMergeJumpFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestVisitFactsStatementReturnIncompleteAssignFailClosed(t *testing.T) {
	// incomplete GlobalFacts after return update must not invent visit success
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	f.RV = CreateVariableScalarsSess(testAmbientSession, "f_rv", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	fm := NewFactMgrSess(testAmbientSession, f)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr), nil}
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	cg.CurrentFunc = f
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	st := &Stmt{
		Kind: StmtReturn, StmID: 5,
		Expr: &Expression{Term: TermVariable, Var: f.RV, ExprType: f.RV.Type},
	}
	if VisitFactsStatementReturn(st, &cg, Defaults()) {
		t.Fatal("incomplete GlobalFacts return visit must fail closed")
	}
}

func TestAnalyzeWithEdgesInStmID0FailClosed(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	// Statement::stm_id always live; StmID 0 + FM fails closed sticky
	// (no invent soft-skip edge merge then validate as complete)
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: IncompleteStmID, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntTypeSess(testAmbientSession)},
		Expr: &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)}, AssignOp: AssignSimple,
	}
	fm := NewFactMgrSess(testAmbientSession, nil)
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	facts := []*FactPointTo{}
	if AnalyzeWithEdgesIn(st, &facts, &cg, Defaults(), nil) {
		t.Fatal("IncompleteStmID must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("StmID 0 AnalyzeWithEdgesIn must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete call sticky (no invent true past nil Statement*)
	if AnalyzeWithEdgesIn(nil, &facts, &cg, Defaults(), nil) {
		t.Fatal("nil st must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil st AnalyzeWithEdgesIn must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestAnalyzeWithEdgesInNilCFGFailClosed(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 20, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntTypeSess(testAmbientSession)},
		Expr: &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0)}, AssignOp: AssignSimple,
	}
	fm := NewFactMgrSess(testAmbientSession, nil)
	fm.CFGEdges = []*CFGEdge{nil}
	facts := []*FactPointTo{}
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if AnalyzeWithEdgesIn(st, &facts, &cg, Defaults(), nil) {
		t.Fatal("incomplete CFG must fail closed analyze")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete CFG analyze must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestAnalyzeWithEdgesInMergesJump(t *testing.T) {
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 20, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntTypeSess(testAmbientSession)},
		Expr: &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0)}, AssignOp: AssignSimple,
	}
	fm := NewFactMgrSess(testAmbientSession, nil)
	// prior goto edge from 10 → 20 with facts
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	fm.CreateCFGEdgeTo(10, &Block{}, 20, false, false)
	fm.MapVisited = map[int]bool{10: true}
	fm.MapFactsOut[10] = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)}
	// also seed p fact at dest
	facts := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), false, false))}
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if !AnalyzeWithEdgesIn(st, &facts, &cg, Defaults(), nil) {
		t.Fatal("analyze")
	}
	fp := FindRelatedPointToSess(testAmbientSession, facts, p)
	if fp == nil || len(fp.PointTo) < 2 {
		t.Fatal("merged jump", fp)
	}
}

func TestAnalyzeWithEdgesInIncompleteOutFailClosed(t *testing.T) {
	// Statement.cpp:819 — merge_jump_facts always; incomplete out fails closed
	// (no invent skip merge when MapFactsOut has holes)
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 20, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntTypeSess(testAmbientSession)},
		Expr: &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0)}, AssignOp: AssignSimple,
	}
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	fm.CreateCFGEdgeTo(10, &Block{}, 20, false, false)
	fm.MapVisited = map[int]bool{10: true}
	fm.MapFactsOut = map[int][]*FactPointTo{
		10: {MakeFactPointToSess(testAmbientSession, p, NullPtr), nil},
	}
	facts := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, CreateVariableScalarsSess(testAmbientSession, "g_a", GetIntTypeSess(testAmbientSession), false, false))}
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if AnalyzeWithEdgesIn(st, &facts, &cg, Defaults(), nil) {
		t.Fatal("incomplete MapFactsOut on jump src must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete MapFactsOut AnalyzeWithEdgesIn must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestFindFixedPointIncompleteBackOutFailClosed(t *testing.T) {
	// Block.cpp:535 — merge_facts(current, map_facts_out[src]) always
	// incomplete out fails closed (no invent skip soft-merge)
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f"}
	b := &Block{StmID: 50, Func: f, Looping: true, Stmts: nil}
	fm := NewFactMgrSess(testAmbientSession, f)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	// mark visited so fixed-point enters back-edge merge path
	fm.MapVisited = map[int]bool{50: true}
	fm.CreateCFGEdgeTo(60, b, 50, false, true)
	fm.MapFactsOut = map[int][]*FactPointTo{
		60: {MakeFactPointToSess(testAmbientSession, p, NullPtr), nil},
	}
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	_, _, _, ok := FindFixedPointBlock(b, []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, GarbagePtr)}, &cg, Defaults(), false)
	if ok {
		t.Fatal("incomplete back-edge MapFactsOut must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("expect sticky error on incomplete fixed-point merge")
	}
	ClearErrorSess(testAmbientSession)
}

func TestFindFixedPointBlock(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	// Block::stm_id always live when FM bound
	b := &Block{StmID: 10, Stmts: []Stmt{{
		Kind: StmtAssign, StmID: 1, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntTypeSess(testAmbientSession)},
		Expr: &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 3)}, AssignOp: AssignSimple,
	}}}
	fm := NewFactMgrSess(testAmbientSession, nil)
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	out, _, _, ok := FindFixedPointBlock(b, nil, &cg, Defaults(), false)
	if !ok {
		t.Fatal("fp")
	}
	_ = out
	if !fm.MapVisited[1] {
		t.Fatal("visited")
	}
	// IncompleteStmID on block + FM fails closed (no invent soft single-pass success)
	ClearErrorSess(testAmbientSession)
	bad := &Block{StmID: IncompleteStmID, Stmts: []Stmt{{
		Kind: StmtAssign, StmID: 2, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntTypeSess(testAmbientSession)},
		Expr: &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)}, AssignOp: AssignSimple,
	}}}
	if _, _, _, ok := FindFixedPointBlock(bad, nil, &cg, Defaults(), false); ok {
		t.Fatal("block IncompleteStmID must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("expect sticky error on incomplete block id")
	}
	ClearErrorSess(testAmbientSession)
}

// TestFindFixedPointBlockNoDoublePushStack — when block is already stack top
// (make_random post_creation), find_fixed_point must not push again.
// VisitFactsBlock still needs a push when the block is not on stack.
func TestFindFixedPointBlockNoDoublePushStack(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	outer := &Block{StmID: 1, Func: f}
	body := &Block{StmID: 10, Func: f, Parent: outer}
	f.Stack = []*Block{outer, body}
	f.Blocks = []*Block{outer, body}
	v := CreateVariableScalarsSess(testAmbientSession, "g_x", GetIntTypeSess(testAmbientSession), false, false)
	body.Stmts = []Stmt{{
		Kind: StmtAssign, StmID: 2, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntTypeSess(testAmbientSession)},
		Expr: &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 1)}, AssignOp: AssignSimple,
	}}
	fm := NewFactMgrSess(testAmbientSession, f)
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	cg.CurrentFunc = f
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	before := len(f.Stack)
	_, _, _, ok := FindFixedPointBlock(body, []*FactPointTo{}, &cg, Defaults(), true)
	if !ok {
		t.Fatalf("fp failed sticky=%v", HasErrorSess(testAmbientSession))
	}
	if len(f.Stack) != before {
		t.Fatalf("already-top block must not double-push stack: before=%d after=%d", before, len(f.Stack))
	}
	if len(f.Stack) != 2 || f.Stack[0] != outer || f.Stack[1] != body {
		t.Fatalf("stack identity must be unchanged")
	}
	// Off-stack visit: temporary push then pop.
	f.Stack = []*Block{outer}
	_, _, _, ok = FindFixedPointBlock(body, []*FactPointTo{}, &cg, Defaults(), true)
	if !ok {
		t.Fatalf("fp off-stack failed sticky=%v", HasErrorSess(testAmbientSession))
	}
	if len(f.Stack) != 1 || f.Stack[0] != outer {
		t.Fatalf("off-stack FP must restore stack to outer only: %#v", f.Stack)
	}
	ClearErrorSess(testAmbientSession)
}

// TestFindFixedPointBlockShortcutConflictFallthrough — Block.cpp:537–541.
// shortcut==1 (effect conflict) is commented out in C++; full re-analysis runs.
// Inventing fail-closed on ShortcutConflict emptied bodies during outer FP.
func TestFindFixedPointBlockShortcutConflictFallthrough(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	SetProcessOptionsSess(testAmbientSession, Defaults())
	w := CreateVariableScalarsSess(testAmbientSession, "g_w", GetIntTypeSess(testAmbientSession), false, false)
	// empty body: full visit after conflict is trivial success
	b := &Block{StmID: 1, Looping: true, Stmts: nil}
	fm := NewFactMgrSess(testAmbientSession, nil)
	entry := []*FactPointTo{}
	fm.SetMapFactsIn(1, entry)
	fm.SetMapFactsOut(1, entry)
	fm.MapVisited = map[int]bool{1: true}
	// prior block effect writes w; ambient effect_context writes w → InConflict
	fm.SetMapStmEffect(1, EmptyEffect().WriteVarSess(testAmbientSession, w))
	cg := WithEffectContext(EmptyEffect().WriteVarSess(testAmbientSession, w)).WithSession(testAmbientSession).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	// visitOnce false so shortcut is attempted; conflict must fall through
	out, _, idx, ok := FindFixedPointBlock(b, entry, &cg, Defaults(), false)
	if !ok {
		t.Fatalf("block shortcut conflict must fall through to full re-analysis, idx=%d err=%v", idx, HasErrorSess(testAmbientSession))
	}
	_ = out
	// full path sets maps for the block
	if !fm.MapVisited[1] {
		t.Fatal("full re-analysis must mark block visited")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSetAccumulatedEffectAfterBlock(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	st := &Stmt{Kind: StmtIfElse, StmID: 7}
	fm := NewFactMgrSess(testAmbientSession, nil)
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	v := CreateVariableScalarsSess(testAmbientSession, "g_1", GetIntTypeSess(testAmbientSession), false, false)
	SetAccumulatedEffectAfterBlock(st, EmptyEffect().WriteVarSess(testAmbientSession, v), &cg, EmptyEffect())
	if !fm.GetMapStmEffect(7).IsWrittenSess(testAmbientSession, v) {
		t.Fatal("effect")
	}
	// StmID 0 — no invent soft no-op that leaves effect unrecorded as success
	st0 := &Stmt{Kind: StmtFor, StmID: IncompleteStmID}
	SetAccumulatedEffectAfterBlock(st0, EmptyEffect().WriteVarSess(testAmbientSession, v), &cg, EmptyEffect())
	// GetMapStmEffect(IncompleteStmID) IncompleteEffect (IsWritten fail-closed true — use map entry probe)
	if _, ok := fm.MapStmEffect[0]; ok {
		t.Fatal("StmID 0 must not invent map effect key 0")
	}
	// GetMapStmEffect(IncompleteStmID) IncompleteEffect sticky — not invent empty pure default
	ClearErrorSess(testAmbientSession)
	if EffectComplete(fm.GetMapStmEffect(IncompleteStmID)) || fm.GetMapStmEffect(IncompleteStmID).IsEmptySess(testAmbientSession) {
		t.Fatal("StmID 0 GetMapStmEffect must IncompleteEffect")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("StmID 0 GetMapStmEffect must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if EffectComplete(fm.GetMapAccumEffect(IncompleteStmID)) || fm.GetMapAccumEffect(IncompleteStmID).IsPureSess(testAmbientSession) {
		t.Fatal("IncompleteStmID GetMapAccumEffect must IncompleteEffect")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IncompleteStmID GetMapAccumEffect must SetError sticky")
	}
	// incomplete pre/block effect must sticky Incomplete map entry
	ClearErrorSess(testAmbientSession)
	st2 := &Stmt{Kind: StmtIfElse, StmID: 8}
	SetAccumulatedEffectAfterBlock(st2, IncompleteEffect(), &cg, EmptyEffect())
	if EffectComplete(fm.GetMapStmEffect(8)) {
		t.Fatal("incomplete block effect must map IncompleteEffect")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete block effect must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// Statement + CGContext always live; sticky
	// Nil FM / StmID≤0 stay non-sticky soft re-pick
	SetAccumulatedEffectAfterBlock(nil, EmptyEffect().WriteVarSess(testAmbientSession, v), &cg, EmptyEffect())
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil stmt SetAccumulatedEffectAfterBlock must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	SetAccumulatedEffectAfterBlock(st, EmptyEffect().WriteVarSess(testAmbientSession, v), nil, EmptyEffect())
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil cg SetAccumulatedEffectAfterBlock must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	cgNoFM := EmptyCGContext().WithSession(testAmbientSession)
	SetAccumulatedEffectAfterBlock(st, EmptyEffect().WriteVarSess(testAmbientSession, v), &cgNoFM, EmptyEffect())
	if HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM SetAccumulatedEffectAfterBlock must stay non-sticky soft re-pick")
	}
	ClearErrorSess(testAmbientSession)
	SetAccumulatedEffectAfterBlock(st0, EmptyEffect().WriteVarSess(testAmbientSession, v), &cg, EmptyEffect())
	if HasErrorSess(testAmbientSession) {
		t.Fatal("StmID 0 SetAccumulatedEffectAfterBlock must stay non-sticky soft re-pick")
	}
	ClearErrorSess(testAmbientSession)
}

func TestFindEdgesInNilFMSticky(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	if (*FactMgr)(nil).FindEdgesIn(1, false, false) != nil {
		t.Fatal("nil FM FindEdgesIn must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM FindEdgesIn must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if (*FactMgr)(nil).FindEdgesInToBlock(&Block{}, false, false) != nil {
		t.Fatal("nil FM FindEdgesInToBlock must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM FindEdgesInToBlock must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

// TestCreateCFGEdgeBlockDestUsesStmID — FactMgr.cpp:597–598 CFGEdge dest is Statement*.
// Block* dest must record DestStmID so find_edges_in(e->dest==this) matches
// (no invent DestStmID 0 + FindEdgesInToBlock second pass).
func TestCreateCFGEdgeBlockDestUsesStmID(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, nil)
	b := &Block{StmID: 42}
	fm.CreateCFGEdge(7, b, false, true)
	if len(fm.CFGEdges) != 1 {
		t.Fatalf("want 1 edge, got %d", len(fm.CFGEdges))
	}
	e := fm.CFGEdges[0]
	if e.DestStmID != 42 || e.DestBlock != b || !e.BackLink {
		t.Fatalf("edge dest: stm=%d blk=%v back=%v want stm=42 DestBlock=b back", e.DestStmID, e.DestBlock == b, e.BackLink)
	}
	// has_edge_in / find_edges_in via DestStmID
	if !fm.HasEdgeIn(42, false, true) {
		t.Fatal("HasEdgeIn(block.StmID) must see self/continue-style block dest")
	}
	got := fm.FindEdgesIn(42, false, true)
	if len(got) != 1 {
		t.Fatalf("FindEdgesIn want 1, got %d", len(got))
	}
	ClearErrorSess(testAmbientSession)
}

func TestAnalyzeWithEdgesInMergesJumpUnions(t *testing.T) {
	// Statement.cpp:819–820 merge_jump_facts on full FactVec (eUnionWrite too).
	// Soft invent was PT-only tryMergeJumpFacts in AnalyzeWithEdgesIn.
	ClearErrorSess(testAmbientSession)
	ut := &Type{isUnion: true, StructName: "U_jmp", Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
		{Name: "f1", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	parent := CreateVariableScalarsSess(testAmbientSession, "g_uj", ut, false, false)
	parent.CreateFieldVarsSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, nil)
	// dest stmt already visited so back-edge merge runs
	dest := &Stmt{Kind: StmtAssign, StmID: 20}
	// live entry: field 0 last-write
	fm.UnionFacts = []*FactUnion{MakeFactUnionSess(testAmbientSession, parent, 0)}
	fm.GlobalFacts = []*FactPointTo{}
	// jump out: field 1 last-write
	fm.SetMapFactsOutPair(10, []*FactPointTo{}, []*FactUnion{MakeFactUnionSess(testAmbientSession, parent, 1)})
	fm.SetMapFactsInPair(20, []*FactPointTo{}, []*FactUnion{MakeFactUnionSess(testAmbientSession, parent, 0)})
	fm.SetMapStmEffect(20, EmptyEffect())
	fm.SetMapAccumEffect(10, EmptyEffect())
	fm.MapVisited = map[int]bool{10: true, 20: true}
	// back edge src→dest
	fm.CFGEdges = []*CFGEdge{{SrcID: 10, DestStmID: 20, PostDest: false, BackLink: true}}
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	facts := []*FactPointTo{}
	blk := &Block{StmID: 1}
	// validate may fail (empty assign shell) but jump merge runs first
	_ = AnalyzeWithEdgesIn(dest, &facts, &cg, Defaults(), blk)
	if !UnionFactsComplete(fm.UnionFacts) {
		t.Fatalf("union merge incomplete err=%v", GetErrorSess(testAmbientSession))
	}
	fu := FindRelatedUnionSess(testAmbientSession, fm.UnionFacts, parent)
	if fu == nil {
		t.Fatal("missing union fact after jump merge")
	}
	// 0 join 1 must not stay fid 0 (merge_jump_facts eUnionWrite)
	if fu.LastWrittenFID == 0 {
		t.Fatalf("jump out fid 1 must join entry 0; got still 0 (no union merge?)")
	}
	ClearErrorSess(testAmbientSession)
}
