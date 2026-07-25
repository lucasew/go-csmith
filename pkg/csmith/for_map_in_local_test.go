// Upstream: StatementFor.cpp post_loop_analysis + FactMgr::merge_jump_facts.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import "testing"

// TestDropFactSubjectsByVarsKeepsEntryWithoutBodyLocals —
// map_facts_in[body] must not list body LocalVars. When break outs correctly
// lack those subjects (remove_loop_local), merge_jump must not invent garbage
// for them (FactMgr.cpp:575–579; seed-7 for 640 / l_1402).
func TestDropFactSubjectsByVarsKeepsEntryWithoutBodyLocals(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	outer := CreateVariableScalars("g_outer", GetIntType(), false, false)
	bodyLoc := CreateVariableScalars("l_body", PointerTo(GetIntType()), false, false)
	bodyLoc.InitExpr = &Expression{Term: TermVariable, Var: outer, ExprType: PointerTo(GetIntType())}
	in := []*FactPointTo{
		MakeFactPointTo(CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false), outer),
		MakeFactPointTo(bodyLoc, outer),
	}
	locals := []*Variable{bodyLoc}
	out := DropFactSubjectsByVars(in, locals)
	if !FactsComplete(out) || HasErrorSess(testAmbientSession) {
		t.Fatalf("drop must complete: out complete=%v err=%v", FactsComplete(out), HasErrorSess(testAmbientSession))
	}
	if FindRelatedPointTo(out, bodyLoc) != nil {
		t.Fatal("body local subject must be dropped from entry env")
	}
	if FindRelatedPointTo(out, in[0].Var) == nil {
		t.Fatal("unrelated subjects must remain")
	}
	// empty vars no-op
	same := DropFactSubjectsByVars(in, nil)
	if len(same) != len(in) {
		t.Fatal("nil vars must no-op")
	}
	ClearErrorSess(testAmbientSession)
}

// TestPostLoopBreakMergeNoInventBodyLocal —
// StatementFor.cpp:355–367 post_loop: restore map_facts_in then merge break outs.
// Body local in polluted map_in + break out without it must not invent garbage.
func TestPostLoopBreakMergeNoInventBodyLocal(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "func_t", ReturnType: GetIntType()}
	fm := NewFactMgrSess(testAmbientSession, f)
	// for body with local l_body
	body := &Block{StmID: 10, Func: f, Looping: true, BreakStmIDs: []int{20}}
	g := CreateVariableScalars("g_x", GetIntType(), false, false)
	lBody := CreateVariableScalars("l_body", PointerTo(GetIntType()), false, false)
	lBody.InitExpr = &Expression{Term: TermVariable, Var: g, ExprType: PointerTo(GetIntType())}
	body.LocalVars = []*Variable{lBody}
	// polluted map_facts_in[body] incorrectly includes body local (bug shape)
	gPtr := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	fm.MapFactsIn[body.StmID] = []*FactPointTo{
		MakeFactPointTo(gPtr, g),
		MakeFactPointTo(lBody, g),
	}
	fm.MapUnionFactsIn = map[int][]*FactUnion{body.StmID: {}}
	// break out: remove_loop_local already dropped l_body (48 facts shape: only g_p)
	fm.MapFactsOut[20] = []*FactPointTo{MakeFactPointTo(gPtr, g)}
	fm.MapUnionFactsOut = map[int][]*FactUnion{20: {}}
	// outer facts as post_loop starts (will AssignGlobalFactsFromMapIn)
	fm.GlobalFacts = CloneFactSlice(fm.MapFactsIn[body.StmID])
	fm.UnionFacts = []*FactUnion{}
	forSt := &Stmt{Kind: StmtFor, StmID: 30, Then: body}
	// run post_loop strip + break merge path
	// Call the production helper path via PostLoopAnalysis if exported, else inline contract:
	// after Assign + Drop, merge jump must not invent garbage on l_body.
	fm.AssignGlobalFactsFromMapIn(body.StmID)
	if len(body.LocalVars) > 0 {
		fm.GlobalFacts = DropFactSubjectsByVars(fm.GlobalFacts, body.LocalVars)
		fm.UnionFacts = DropUnionSubjectsByVars(fm.UnionFacts, body.LocalVars)
	}
	out := fm.GetMapFactsOut(20)
	if _, ok := tryMergeJumpFacts(&fm.GlobalFacts, out); !ok {
		t.Fatalf("merge must succeed: err=%v", HasErrorSess(testAmbientSession))
	}
	// l_body must not reappear as garbage invent
	if f := FindRelatedPointTo(fm.GlobalFacts, lBody); f != nil {
		t.Fatalf("body local must not be re-invented after break merge, got pointees=%v", pointToNames(f))
	}
	// outer g_p remains
	if FindRelatedPointTo(fm.GlobalFacts, gPtr) == nil {
		t.Fatal("outer pointer fact must remain after break merge")
	}
	// contrast: without drop, invent would create garbage subject
	polluted := []*FactPointTo{
		MakeFactPointTo(gPtr, g),
		MakeFactPointTo(lBody, g),
	}
	breakOut := []*FactPointTo{MakeFactPointTo(gPtr, g)}
	if _, ok := tryMergeJumpFacts(&polluted, breakOut); !ok {
		t.Fatal("raw invent path must complete")
	}
	if f := FindRelatedPointTo(polluted, lBody); f == nil || !f.IsDead() {
		t.Fatal("without drop, invent garbage is expected (documents FactMgr.cpp:575–579)")
	}
	_ = forSt
	ClearErrorSess(testAmbientSession)
}
