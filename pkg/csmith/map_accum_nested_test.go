package csmith

import "testing"

// Statement.cpp:563 — shortcut_analysis: map_accum_effect[this] = *get_effect_accum()
// after add_effect(map_stm_effect[this]). Live accum reads must be preserved in the snapshot.
func TestShortcutAnalysisPreservesLiveAccumReads(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgr(f)
	g1 := CreateVariableScalars("g_1", GetIntType(), false, false)
	g2 := CreateVariableScalars("g_2", GetIntType(), false, false)
	g3 := CreateVariableScalars("g_3", GetIntType(), false, false)

	st := Stmt{
		Kind: StmtAssign, AssignOp: AssignSimple, StmID: AllocStmID(),
		LhsVar: g1,
		Expr:   &Expression{Term: TermVariable, Var: g1, ExprType: GetIntType()},
	}
	blk := &Block{Func: f, StmID: AllocStmID(), Stmts: []Stmt{st}}
	f.Blocks = []*Block{blk}
	f.Body = blk

	fm.MapFactsIn = map[int][]*FactPointTo{st.StmID: {}}
	fm.MapFactsOut = map[int][]*FactPointTo{st.StmID: {}}
	fm.MapStmEffect = map[int]Effect{st.StmID: EmptyEffect().ReadVar(g1).WriteVar(g1)}
	fm.MapAccumEffect = map[int]Effect{st.StmID: EmptyEffect().ReadVar(g1)} // sparse gen-time
	fm.MapVisited = map[int]bool{st.StmID: true}
	fm.MapUnionFactsIn = map[int][]*FactUnion{st.StmID: {}}
	fm.MapUnionFactsOut = map[int][]*FactUnion{st.StmID: {}}
	fm.GlobalFacts = []*FactPointTo{}

	live := EmptyEffect().ReadVar(g1).ReadVar(g2).ReadVar(g3)
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &live
	cg.EffectStm = EmptyEffect()

	facts := []*FactPointTo{}
	sc := ShortcutAnalysis(&st, &facts, &cg, opts)
	if sc != ShortcutOK {
		t.Fatalf("want ShortcutOK got %d err=%v", sc, HasErrorSess(testAmbientSession))
	}
	got := fm.GetMapAccumEffect(st.StmID)
	names := mapAccumNamesOf(got.ReadVars())
	if len(got.ReadVars()) < 3 {
		t.Fatalf("shortcut must snapshot live accum reads, got %v", names)
	}
	// Must include g2/g3 from live before shortcut, not only gen-time g1
	has := map[string]bool{}
	for _, v := range got.ReadVars() {
		if v != nil {
			has[v.Name] = true
		}
	}
	if !has["g_2"] || !has["g_3"] {
		t.Fatalf("missing live reads in map_accum after shortcut: %v", names)
	}
	ClearErrorSess(testAmbientSession)
}

// StmVisitFacts always records map_accum_effect even when visit_facts fails.
// Statement.cpp:622.
func TestStmVisitFactsRecordsAccumEvenOnVisitFail(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgr(f)
	g1 := CreateVariableScalars("g_1", GetIntType(), false, false)
	g2 := CreateVariableScalars("g_2", GetIntType(), false, false)

	// Assign with nil Lhs will fail visit — still must record map_accum
	st := Stmt{Kind: StmtAssign, AssignOp: AssignSimple, StmID: AllocStmID()}
	fm.MapAccumEffect = map[int]Effect{st.StmID: EmptyEffect().ReadVar(g1)}
	fm.GlobalFacts = []*FactPointTo{}
	fm.MapUnionFactsIn = map[int][]*FactUnion{}
	fm.MapUnionFactsOut = map[int][]*FactUnion{}

	live := EmptyEffect().ReadVar(g1).ReadVar(g2)
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	cg.EffectAccum = &live
	cg.EffectStm = EmptyEffect()

	facts := []*FactPointTo{}
	ok := StmVisitFacts(&st, &facts, &cg, opts)
	if ok {
		t.Fatal("expected visit fail on incomplete assign")
	}
	// even on fail, map_accum should be live snapshot
	got := fm.GetMapAccumEffect(st.StmID)
	if len(got.ReadVars()) < 2 {
		t.Fatalf("StmVisitFacts must record live accum on fail: %v err=%v",
			mapAccumNamesOf(got.ReadVars()), HasErrorSess(testAmbientSession))
	}
	ClearErrorSess(testAmbientSession)
}

func TestMapAccumEffectStoreDetachedFromLiveAccum(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	g1 := CreateVariableScalars("g_1", GetIntType(), false, false)
	g2 := CreateVariableScalars("g_2", GetIntType(), false, false)
	live2 := EmptyEffect().ReadVar(g1)
	stored := live2.Clone()
	live2 = live2.ReadVar(g2)
	if len(stored.ReadVars()) != 1 {
		t.Fatalf("Clone store must stay frozen: %v", mapAccumNamesOf(stored.ReadVars()))
	}
	ClearErrorSess(testAmbientSession)
}

// Effect.cpp:84–89 — map_stm_effect assignment deep-copies vectors. Soft invent was
// shallow SetMapStmEffect(cg.EffectStm) so live EffectStm COW / AddEffect into
// GetMapStmEffect results shared maps with the store; Block::set_accumulated_effect
// then alias-corrupted snapshots used as generation ambient (seed-7 ChooseOKVar
// n=26 vs UP n=56 when effect_context seFree poisoned by shared write sets).
func TestMapStmEffectStoreDetachedFromLiveStm(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	fm := NewFactMgr(&Function{Name: "f", ReturnType: GetIntType()})
	g1 := CreateVariableScalars("g_1", GetIntType(), false, false)
	g2 := CreateVariableScalars("g_2", GetIntType(), false, false)
	g3 := CreateVariableScalars("g_3", GetIntType(), false, false)

	live := EmptyEffect().WriteVar(g1).WriteVar(g2)
	id := AllocStmID()
	fm.SetMapStmEffect(id, live)
	// grow live after store — must not appear in map snapshot
	live = live.WriteVar(g3)
	got := fm.GetMapStmEffect(id)
	if !got.IsWritten(g1) || !got.IsWritten(g2) {
		t.Fatalf("stored map_stm_effect missing g1/g2")
	}
	if got.IsWritten(g3) {
		t.Fatalf("map_stm_effect must not see post-store WriteVar on live Effect: %v",
			mapAccumNamesOf(got.WrittenVars()))
	}
	// Get returns detached: mutating returned Effect must not change map
	got2 := got.WriteVar(g3)
	_ = got2
	again := fm.GetMapStmEffect(id)
	if again.IsWritten(g3) {
		t.Fatal("GetMapStmEffect must return detached copy (map not mutated by caller WriteVar)")
	}
	// Block accum merge must not leave shared maps between statements
	id2 := AllocStmID()
	fm.SetMapStmEffect(id2, EmptyEffect().WriteVar(g3))
	merged := fm.GetMapStmEffect(id).AddEffect(fm.GetMapStmEffect(id2))
	fm.SetMapStmEffect(id, EmptyEffect().WriteVar(g1)) // replace id entry
	if !merged.IsWritten(g2) {
		t.Fatal("merged snapshot must keep g2 after map entry replaced")
	}
	ClearErrorSess(testAmbientSession)
}

func mapAccumNamesOf(vs []*Variable) []string {
	out := make([]string, 0, len(vs))
	for _, v := range vs {
		if v != nil {
			out = append(out, v.Name)
		}
	}
	return out
}
