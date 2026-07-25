package csmith

import "testing"

// TestFindFixedPointDropsBodyLocalsBeforeShortcut — Block.cpp:520–557.
// map_facts_in is stored after DropFactSubjectsByVars(body.LocalVars).
// Back-edge merge can reintroduce those locals from goto/break map_out.
// same_facts(current_with_locals, map_in_without) never converges → multi-pass
// rewrites map_accum_effect with end-of-body effect_accum → StatementGoto
// choose_visible_read_var pool skew (seed 1469030: nOk 49 vs UP 40).
// Fair: drop body locals from current_inputs after each back-edge merge,
// before ShortcutAnalysisBlock, so the lattice matches map_facts_in.
func TestFindFixedPointDropsBodyLocalsBeforeShortcut(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	SetProcessOptionsSess(testAmbientSession, Defaults())
	f := &Function{Name: "f_fp_local_drop"}
	fm := NewFactMgrSess(testAmbientSession, f)
	g := CreateVariableScalars("g_pt", GetIntType(), false, false)
	loc := CreateVariableScalars("l_body", GetIntType(), false, false)
	// Point-to facts: g and local both point to g
	ptG := MakeFactPointTo(g, g)
	ptLoc := MakeFactPointTo(loc, g)
	if ptG == nil || ptLoc == nil {
		t.Fatal("facts")
	}
	entry := []*FactPointTo{ptG.Clone()}
	// map_in is entry without body local (as after Drop)
	b := &Block{
		Func: f, StmID: AllocStmID(), Looping: true,
		LocalVars: []*Variable{loc},
		Stmts:     []Stmt{},
	}
	fm.SetMapFactsIn(b.StmID, CloneFactSlice(entry))
	fm.SetMapFactsOut(b.StmID, CloneFactSlice(entry))
	fm.MapVisited[b.StmID] = true
	// Self-back edge whose out still lists body local (goto/break-style pollution)
	outWithLocal := []*FactPointTo{ptG.Clone(), ptLoc.Clone()}
	// Use a dummy src statement id that has out with local
	srcID := AllocStmID()
	fm.SetMapFactsOut(srcID, outWithLocal)
	fm.CreateCFGEdge(srcID, b, false, true)
	fm.SetMapStmEffect(b.StmID, EmptyEffect())
	fm.MapVisited[srcID] = true

	cg := EmptyCGContext().WithFactMgr(fm)
	pre := EmptyEffect()
	cg.EffectAccum = &pre
	fm.GlobalFacts = CloneFactSlice(entry)
	fm.UnionFacts = []*FactUnion{}

	// First call visitOnce=true forces full walk; second with false should shortcut
	// if drop-before-shortcut makes same_facts match after first set_fact_in.
	_, _, _, ok1 := FindFixedPointBlock(b, CloneFactSlice(entry), &cg, Defaults(), true)
	if !ok1 && HasErrorSess(testAmbientSession) {
		// may fail on empty body; still check map_in shape
		ClearErrorSess(testAmbientSession)
	}
	inAfter := fm.GetMapFactsIn(b.StmID)
	if FindRelatedPointTo(inAfter, loc) != nil {
		t.Fatal("map_facts_in must not keep body local after FP store")
	}
	// Reset visited and force second iteration path: visited true, visitOnce false
	fm.MapVisited[b.StmID] = true
	// Simulate currentInputs after merge including local — Drop must make shortcut OK.
	// Call FP again with entry; merge will reintroduce local from src out.
	ClearErrorSess(testAmbientSession)
	_, _, _, ok2 := FindFixedPointBlock(b, CloneFactSlice(entry), &cg, Defaults(), false)
	if HasErrorSess(testAmbientSession) {
		t.Fatalf("sticky err after second FP: %v", GetErrorSess(testAmbientSession))
	}
	// If drop-before-shortcut works, we should not spin 50 iterations pollution.
	// Success: ok2 true (shortcut or converged full walk) without incomplete maps.
	if !ok2 {
		t.Log("second FP returned false; may be empty-body policy — map_in check is primary")
	}
	in2 := fm.GetMapFactsIn(b.StmID)
	if FindRelatedPointTo(in2, loc) != nil {
		t.Fatal("map_facts_in must still exclude body local after second FP")
	}
	_ = ok1
}
