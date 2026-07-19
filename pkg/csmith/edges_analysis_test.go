package csmith

import (
	"testing"
)

func TestMergeJumpFacts(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	facts := []*FactPointTo{MakeFactPointTo(p, a)}
	jump := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	if !MergeJumpFacts(&facts, jump) {
		t.Fatal("changed")
	}
	fp := FindRelatedPointTo(facts, p)
	// joined set should include both
	if fp == nil || len(fp.PointTo) < 2 {
		t.Fatal(fp)
	}
}

func TestMergeJumpFactsMissingIsGarbage(t *testing.T) {
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	facts := []*FactPointTo{MakeFactPointTo(p, a)}
	// jump has no fact for p → garbage join
	if !MergeJumpFacts(&facts, nil) {
		t.Fatal("expect garbage merge")
	}
	fp := FindRelatedPointTo(facts, p)
	if fp == nil || !fp.IsDead() {
		// garbage_ptr is IsDead
		if fp == nil || !IsVariableInSet(fp.PointTo, GarbagePtr) {
			t.Fatal(fp)
		}
	}
}

func TestFindEdgesIn(t *testing.T) {
	fm := NewFactMgr(nil)
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
	// StmID ≤0 incomplete key sticky
	ClearError()
	if got := fm.FindEdgesIn(0, false, false); got != nil {
		t.Fatal("destStmID 0 must fail closed", got)
	}
	if !HasError() {
		t.Fatal("destStmID 0 FindEdgesIn must SetError sticky")
	}
	ClearError()
	if got := fm.FindEdgesInToBlock(nil, false, false); got != nil {
		t.Fatal("nil dest block must fail closed", got)
	}
	if !HasError() {
		t.Fatal("nil dest FindEdgesInToBlock must SetError sticky")
	}
	ClearError()
}

func TestFindEdgesInNilHoleFailClosed(t *testing.T) {
	fm := NewFactMgr(nil)
	fm.CFGEdges = []*CFGEdge{
		{SrcID: 10, DestStmID: 20, BackLink: true},
		nil,
		{SrcID: 11, DestStmID: 20},
	}
	// nil hole fails closed sticky (no invent soft-skip / re-pick past holes)
	ClearError()
	if got := fm.FindEdgesIn(20, false, true); got != nil {
		t.Fatal("nil CFG hole must fail closed", got)
	}
	if !HasError() {
		t.Fatal("nil CFG hole FindEdgesIn must SetError sticky")
	}
	ClearError()
	if got := fm.FindEdgesIn(20, false, false); got != nil {
		t.Fatal("nil CFG hole must fail closed fwd", got)
	}
	if !HasError() {
		t.Fatal("nil CFG hole FindEdgesIn fwd must SetError sticky")
	}
	// HasEdgeIn must not invent false from len(nil)==0 (sticky already set)
	if !fm.HasEdgeIn(20, false, true) {
		t.Fatal("incomplete FindEdgesIn must HasEdgeIn true, not invent none")
	}
	ClearError()
	b := &Block{}
	fm.CFGEdges = []*CFGEdge{nil, {SrcID: 1, DestBlock: b, BackLink: true}}
	if got := fm.FindEdgesInToBlock(b, false, true); got != nil {
		t.Fatal("nil hole FindEdgesInToBlock", got)
	}
	if !HasError() {
		t.Fatal("nil hole FindEdgesInToBlock must SetError sticky")
	}
	ClearError()
}

func TestHasEdgeInNilFMFailClosed(t *testing.T) {
	// assert(fm) path — sticky has-edge (no invent no-edge without FactMgr)
	ClearError()
	var fm *FactMgr
	if !fm.HasEdgeIn(1, false, true) {
		t.Fatal("nil FM must HasEdgeIn true")
	}
	if !HasError() {
		t.Fatal("nil FM HasEdgeIn must SetError sticky")
	}
	ClearError()
}

func TestMergeJumpFactsNilHoleFailClosed(t *testing.T) {
	ClearError()
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	a := CreateVariableScalars("g_a", GetIntType(), false, false)
	facts := []*FactPointTo{MakeFactPointTo(p, a), nil}
	jump := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	// incomplete subject map must not soft-join past hole — sticky
	if MergeJumpFacts(&facts, jump) {
		t.Fatal("nil subject hole must fail closed")
	}
	// fail closed clears *facts — no invent leave partial / hole-bearing map
	if FactsComplete(facts) {
		t.Fatal("incomplete must clear facts", facts)
	}
	if !HasError() {
		t.Fatal("nil subject hole must SetError sticky")
	}
	ClearError()
	holeSubj := []*FactPointTo{MakeFactPointTo(p, a), nil}
	if _, ok := tryMergeJumpFacts(&holeSubj, jump); ok {
		t.Fatal("tryMerge incomplete subject must ok=false")
	}
	if FactsComplete(holeSubj) {
		t.Fatal("tryMerge must clear incomplete subject", holeSubj)
	}
	if !HasError() {
		t.Fatal("tryMerge incomplete must SetError sticky")
	}
	ClearError()
	facts2 := []*FactPointTo{MakeFactPointTo(p, a)}
	jumpHole := []*FactPointTo{nil}
	if MergeJumpFacts(&facts2, jumpHole) {
		t.Fatal("nil jump hole must fail closed")
	}
	if FactsComplete(facts2) {
		t.Fatal("incomplete jump must clear facts2", facts2)
	}
	if !HasError() {
		t.Fatal("nil jump hole must SetError sticky")
	}
	ClearError()
}

func TestVisitFactsStatementReturnIncompleteAssignFailClosed(t *testing.T) {
	// incomplete GlobalFacts after return update must not invent visit success
	f := &Function{Name: "f", ReturnType: GetIntType()}
	f.RV = CreateVariableScalars("f_rv", PointerTo(GetIntType()), false, false)
	fm := NewFactMgr(f)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	cg := EmptyCGContext().WithFactMgr(fm)
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
	ClearError()
	// Statement::stm_id always live; StmID 0 + FM fails closed sticky
	// (no invent soft-skip edge merge then validate as complete)
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	st := &Stmt{
		Kind: StmtAssign, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignSimple,
	}
	fm := NewFactMgr(nil)
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	facts := []*FactPointTo{}
	if AnalyzeWithEdgesIn(st, &facts, &cg, Defaults(), nil) {
		t.Fatal("StmID 0 must fail closed")
	}
	if !HasError() {
		t.Fatal("StmID 0 AnalyzeWithEdgesIn must SetError sticky")
	}
	ClearError()
	// incomplete call sticky (no invent true past nil Statement*)
	if AnalyzeWithEdgesIn(nil, &facts, &cg, Defaults(), nil) {
		t.Fatal("nil st must fail closed")
	}
	if !HasError() {
		t.Fatal("nil st AnalyzeWithEdgesIn must SetError sticky")
	}
	ClearError()
}

func TestAnalyzeWithEdgesInNilCFGFailClosed(t *testing.T) {
	ClearError()
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 20, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(0)}, AssignOp: AssignSimple,
	}
	fm := NewFactMgr(nil)
	fm.CFGEdges = []*CFGEdge{nil}
	facts := []*FactPointTo{}
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if AnalyzeWithEdgesIn(st, &facts, &cg, Defaults(), nil) {
		t.Fatal("incomplete CFG must fail closed analyze")
	}
	if !HasError() {
		t.Fatal("incomplete CFG analyze must SetError sticky")
	}
	ClearError()
}

func TestAnalyzeWithEdgesInMergesJump(t *testing.T) {
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 20, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(0)}, AssignOp: AssignSimple,
	}
	fm := NewFactMgr(nil)
	// prior goto edge from 10 → 20 with facts
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	fm.CreateCFGEdgeTo(10, &Block{}, 20, false, false)
	fm.MapVisited = map[int]bool{10: true}
	fm.MapFactsOut[10] = []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	// also seed p fact at dest
	facts := []*FactPointTo{MakeFactPointTo(p, CreateVariableScalars("g_a", GetIntType(), false, false))}
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if !AnalyzeWithEdgesIn(st, &facts, &cg, Defaults(), nil) {
		t.Fatal("analyze")
	}
	fp := FindRelatedPointTo(facts, p)
	if fp == nil || len(fp.PointTo) < 2 {
		t.Fatal("merged jump", fp)
	}
}

func TestAnalyzeWithEdgesInIncompleteOutFailClosed(t *testing.T) {
	// Statement.cpp:819 — merge_jump_facts always; incomplete out fails closed
	// (no invent skip merge when MapFactsOut has holes)
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	st := &Stmt{
		Kind: StmtAssign, StmID: 20, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(0)}, AssignOp: AssignSimple,
	}
	fm := NewFactMgr(nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	fm.CreateCFGEdgeTo(10, &Block{}, 20, false, false)
	fm.MapVisited = map[int]bool{10: true}
	fm.MapFactsOut = map[int][]*FactPointTo{
		10: {MakeFactPointTo(p, NullPtr), nil},
	}
	facts := []*FactPointTo{MakeFactPointTo(p, CreateVariableScalars("g_a", GetIntType(), false, false))}
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	if AnalyzeWithEdgesIn(st, &facts, &cg, Defaults(), nil) {
		t.Fatal("incomplete MapFactsOut on jump src must fail closed")
	}
	if !HasError() {
		t.Fatal("incomplete MapFactsOut AnalyzeWithEdgesIn must SetError sticky")
	}
	ClearError()
}

func TestFindFixedPointIncompleteBackOutFailClosed(t *testing.T) {
	// Block.cpp:535 — merge_facts(current, map_facts_out[src]) always
	// incomplete out fails closed (no invent skip soft-merge)
	ClearError()
	f := &Function{Name: "f"}
	b := &Block{StmID: 50, Func: f, Looping: true, Stmts: nil}
	fm := NewFactMgr(f)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	// mark visited so fixed-point enters back-edge merge path
	fm.MapVisited = map[int]bool{50: true}
	fm.CreateCFGEdgeTo(60, b, 50, false, true)
	fm.MapFactsOut = map[int][]*FactPointTo{
		60: {MakeFactPointTo(p, NullPtr), nil},
	}
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	_, _, ok := FindFixedPointBlock(b, []*FactPointTo{MakeFactPointTo(p, GarbagePtr)}, &cg, Defaults(), false)
	if ok {
		t.Fatal("incomplete back-edge MapFactsOut must fail closed")
	}
	if !HasError() {
		t.Fatal("expect sticky error on incomplete fixed-point merge")
	}
	ClearError()
}

func TestFindFixedPointBlock(t *testing.T) {
	ClearError()
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	// Block::stm_id always live when FM bound
	b := &Block{StmID: 10, Stmts: []Stmt{{
		Kind: StmtAssign, StmID: 1, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(3)}, AssignOp: AssignSimple,
	}}}
	fm := NewFactMgr(nil)
	cg := EmptyCGContext().WithFactMgr(fm)
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	out, _, ok := FindFixedPointBlock(b, nil, &cg, Defaults(), false)
	if !ok {
		t.Fatal("fp")
	}
	_ = out
	if !fm.MapVisited[1] {
		t.Fatal("visited")
	}
	// StmID 0 + FM fails closed (no invent soft single-pass success)
	ClearError()
	bad := &Block{Stmts: []Stmt{{
		Kind: StmtAssign, StmID: 2, LhsVar: v, Lhs: &Lhs{Var: v, Type: GetIntType()},
		Expr: &Expression{Term: TermConstant, Con: MakeInt(1)}, AssignOp: AssignSimple,
	}}}
	if _, _, ok := FindFixedPointBlock(bad, nil, &cg, Defaults(), false); ok {
		t.Fatal("block StmID 0 must fail closed")
	}
	if !HasError() {
		t.Fatal("expect sticky error on incomplete block id")
	}
	ClearError()
}

func TestSetAccumulatedEffectAfterBlock(t *testing.T) {
	ClearError()
	st := &Stmt{Kind: StmtIfElse, StmID: 7}
	fm := NewFactMgr(nil)
	cg := EmptyCGContext().WithFactMgr(fm)
	v := CreateVariableScalars("g_1", GetIntType(), false, false)
	SetAccumulatedEffectAfterBlock(st, EmptyEffect().WriteVar(v), &cg, EmptyEffect())
	if !fm.GetMapStmEffect(7).IsWritten(v) {
		t.Fatal("effect")
	}
	// StmID 0 — no invent soft no-op that leaves effect unrecorded as success
	st0 := &Stmt{Kind: StmtFor, StmID: 0}
	SetAccumulatedEffectAfterBlock(st0, EmptyEffect().WriteVar(v), &cg, EmptyEffect())
	// GetMapStmEffect(0) IncompleteEffect (IsWritten fail-closed true — use map entry probe)
	if _, ok := fm.MapStmEffect[0]; ok {
		t.Fatal("StmID 0 must not invent map effect key 0")
	}
	// GetMapStmEffect(0) IncompleteEffect sticky — not invent empty pure default
	ClearError()
	if EffectComplete(fm.GetMapStmEffect(0)) || fm.GetMapStmEffect(0).IsEmpty() {
		t.Fatal("StmID 0 GetMapStmEffect must IncompleteEffect")
	}
	if !HasError() {
		t.Fatal("StmID 0 GetMapStmEffect must SetError sticky")
	}
	ClearError()
	if EffectComplete(fm.GetMapAccumEffect(0)) || fm.GetMapAccumEffect(0).IsPure() {
		t.Fatal("StmID 0 GetMapAccumEffect must IncompleteEffect")
	}
	if !HasError() {
		t.Fatal("StmID 0 GetMapAccumEffect must SetError sticky")
	}
	// incomplete pre/block effect must sticky Incomplete map entry
	ClearError()
	st2 := &Stmt{Kind: StmtIfElse, StmID: 8}
	SetAccumulatedEffectAfterBlock(st2, IncompleteEffect(), &cg, EmptyEffect())
	if EffectComplete(fm.GetMapStmEffect(8)) {
		t.Fatal("incomplete block effect must map IncompleteEffect")
	}
	if !HasError() {
		t.Fatal("incomplete block effect must SetError sticky")
	}
	ClearError()
}

func TestFindEdgesInNilFMSticky(t *testing.T) {
	ClearError()
	if (*FactMgr)(nil).FindEdgesIn(1, false, false) != nil {
		t.Fatal("nil FM FindEdgesIn must fail closed")
	}
	if !HasError() {
		t.Fatal("nil FM FindEdgesIn must SetError sticky")
	}
	ClearError()
	if (*FactMgr)(nil).FindEdgesInToBlock(&Block{}, false, false) != nil {
		t.Fatal("nil FM FindEdgesInToBlock must fail closed")
	}
	if !HasError() {
		t.Fatal("nil FM FindEdgesInToBlock must SetError sticky")
	}
	ClearError()
}
