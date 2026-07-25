package csmith

import "testing"

// TestForwardGotoRecomputesGotoOutFromLiveOtherMaps — StatementGoto.cpp:178–181.
//
// C++ binds goto_in as a reference to map_facts_in/out[other_stm]. After
// stm_visit_facts(dest) when dest contains other, those maps may have been
// updated; the recompute of goto_out re-reads via the live reference.
// Soft invent cloned gotoIn before visit and recomputed from the stale clone,
// so map_facts_out[goto] under-approximated (seed 11466719812903307384:
// g_124 stayed {g_106} on forward-goto out while live other map_out had
// {l_2181,g_106,l_2156} → contains_unfixed_goto false → pure-shortcut of
// need_revisit LCA → Func.Blocks n=37 vs UP n=3 at FindGoodJumpBlock).
func TestForwardGotoRecomputesGotoOutFromLiveOtherMaps(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	SetProcessOptionsSess(testAmbientSession, Defaults())
	f := &Function{Name: "func_t", ReturnType: GetIntType()}
	fm := NewFactMgrSess(testAmbientSession, f)
	body := &Block{StmID: 1, Func: f, Parent: nil}
	f.Body = body
	f.Blocks = []*Block{body}

	g124 := CreateVariableScalars("g_124", PointerTo(GetIntType()), true, false)
	g106 := CreateVariableScalars("g_106", GetIntType(), true, false)
	l2181 := CreateVariableScalars("l_2181", GetIntType(), false, false)
	l2156 := CreateVariableScalars("l_2156", GetIntType(), false, false)
	wide := MakeFactPointToSet(g124, []*Variable{l2181, g106, l2156})
	precise := MakeFactPointTo(g124, g106)

	// Simulate: pre-visit clone would capture precise; post-visit live map is wide.
	fm.SetMapFactsOutPair(10, []*FactPointTo{precise}, []*FactUnion{})
	fm.SetMapFactsOutPair(10, []*FactPointTo{wide}, []*FactUnion{})

	// Contract: re-fetch like C++ live reference must see wide lattice.
	gotoIn := CloneFactSlice(fm.GetMapFactsOut(10))
	if HasErrorSess(testAmbientSession) || !FactsComplete(gotoIn) {
		t.Fatalf("GetMapFactsOut: err=%v facts=%v", GetErrorSess(testAmbientSession), gotoIn)
	}
	got := FindRelatedPointTo(gotoIn, g124)
	if got == nil || len(got.PointTo) < 3 {
		t.Fatalf("re-fetch must see wide live lattice, got %v", ptsNamesGoto(got))
	}

	// update_facts_for_dest at dest parent must keep function-visible pointees
	var gotoOut []*FactPointTo
	UpdateFactsForDest(gotoIn, &gotoOut, f, body)
	if HasErrorSess(testAmbientSession) {
		t.Fatalf("UpdateFactsForDest sticky: %v", GetErrorSess(testAmbientSession))
	}
	outF := FindRelatedPointTo(gotoOut, g124)
	if outF == nil {
		t.Fatal("goto_out missing g_124")
	}
	names := ptsNamesGoto(outF)
	if !containsNameGoto(names, "l_2181") || !containsNameGoto(names, "g_106") {
		t.Fatalf("goto_out after dest filter must keep wide, got %v", names)
	}

	// Statement.cpp:797–800 — dest-in precise does not imply wide jump-src → unfixed
	destIn := []*FactPointTo{MakeFactPointTo(g124, g106)}
	df := FindRelatedPointTo(destIn, g124)
	sf := FindRelatedPointTo(gotoOut, g124)
	if df == nil || sf == nil {
		t.Fatal("need both g_124 facts")
	}
	if df.Imply(sf) {
		t.Fatal("precise dest must not imply wide jump src (contains_unfixed_goto path)")
	}
	ClearErrorSess(testAmbientSession)
}

func ptsNamesGoto(f *FactPointTo) []string {
	if f == nil {
		return nil
	}
	out := make([]string, 0, len(f.PointTo))
	for _, v := range f.PointTo {
		if v == nil {
			out = append(out, "?")
		} else {
			out = append(out, v.Name)
		}
	}
	return out
}

func containsNameGoto(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
