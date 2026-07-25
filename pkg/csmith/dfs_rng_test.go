package csmith

import "testing"

func TestDFSRandomChoiceFirstVisit(t *testing.T) {
	// DFSRndNumGenerator::random_choice first-visit path picks lowest valid v.
	ClearErrorSess(testAmbientSession)
	defer func() {
		RandomNumberDoFinalizationSess(testAmbientSession)
		ReinstallTestProcessSingletons()
		ClearErrorSess(testAmbientSession)
	}()
	o := Defaults()
	o.MaxExhaustiveDepth = 5
	o.DFSExhaustive = true
	o.RandomBased = false
	SetProcessOptionsSess(testAmbientSession, o)
	clearDFSImpl()
	r := NewDFSRng(1, o)
	if r == nil {
		t.Fatal("NewDFSRng")
	}
	// first choice bound 3 → 0
	v := r.RndUptoSess(testAmbientSession, 3)
	if HasErrorSess(testAmbientSession) || v != 0 {
		t.Fatalf("first upto got %d err=%d", v, GetErrorSess(testAmbientSession))
	}
	if r.DFSGetCurrentPos() != 0 {
		t.Fatal("pos", r.DFSGetCurrentPos())
	}
	// second choice
	v2 := r.RndUptoSess(testAmbientSession, 4)
	if HasErrorSess(testAmbientSession) || v2 != 0 {
		t.Fatalf("second %d", v2)
	}
	seq := r.GetSequenceSess(testAmbientSession)
	if seq != "0_0" {
		t.Fatal(seq)
	}
	// flipcoin p=100 forces 1
	ClearErrorSess(testAmbientSession)
	ok := r.RndFlipcoinSess(testAmbientSession, 100)
	if HasErrorSess(testAmbientSession) || !ok {
		t.Fatal("p100", ok, GetErrorSess(testAmbientSession))
	}
	// flipcoin p=0 forces 0
	ClearErrorSess(testAmbientSession)
	ok = r.RndFlipcoinSess(testAmbientSession, 0)
	if HasErrorSess(testAmbientSession) || ok {
		t.Fatal("p0", ok, GetErrorSess(testAmbientSession))
	}
}

func TestDFSRandomChoiceFilterRejects(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	o := Defaults()
	o.MaxExhaustiveDepth = 4
	r := NewDFSRng(1, o)
	// reject 0 and 1 → pick 2
	f := RejectEQ(0)
	// chain: need filter that rejects 0 and 1
	f = filterFunc(func(v uint32) bool { return v < 2 })
	v := r.RndUptoFilterSess(testAmbientSession, 4, f)
	if HasErrorSess(testAmbientSession) || v != 2 {
		t.Fatalf("got %d err=%d", v, GetErrorSess(testAmbientSession))
	}
}

func TestDFSBacktrackingExhaustsBranch(t *testing.T) {
	// After first choice, reset_state and re-walk with filter rejecting previous.
	ClearErrorSess(testAmbientSession)
	o := Defaults()
	o.MaxExhaustiveDepth = 2
	r := NewDFSRng(1, o)
	// depth 0: take 0 of bound 2
	if r.RndUptoSess(testAmbientSession, 2) != 0 || HasErrorSess(testAmbientSession) {
		t.Fatal("first")
	}
	// depth 1: take 0 of bound 1 → only value 0
	if r.RndUptoSess(testAmbientSession, 1) != 0 || HasErrorSess(testAmbientSession) {
		t.Fatal("second")
	}
	// reset for next program enumeration step
	r.DFSResetState()
	// re-enter depth 0: state still init from previous walk at pos 0?
	// reset_state only clears pos/seq/trace — does NOT clear states_.init
	// C++ reset_state same. Full re-search uses decision_depth backtracking.
	// Advance again: current_pos becomes 0, state.init true, decision_depth was 1.
	// current_pos(0) < decision_depth(1) && init → revisit returns same value 0.
	ClearErrorSess(testAmbientSession)
	v := r.RndUptoSess(testAmbientSession, 2)
	if HasErrorSess(testAmbientSession) || v != 0 {
		t.Fatalf("revisit got %d err=%d", v, GetErrorSess(testAmbientSession))
	}
}

func TestDFSExceedMaxDepth(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	o := Defaults()
	o.MaxExhaustiveDepth = 1
	r := NewDFSRng(1, o)
	_ = r.RndUptoSess(testAmbientSession, 2)
	if HasErrorSess(testAmbientSession) {
		t.Fatal("first ok")
	}
	// second choice current_pos=1 >= max 1 → EXCEED
	_ = r.RndUptoSess(testAmbientSession, 2)
	if GetErrorSess(testAmbientSession) != ErrExceedMaxDepth {
		t.Fatal("want exceed", GetErrorSess(testAmbientSession))
	}
	ClearErrorSess(testAmbientSession)
}

func TestDFSEagerBacktracking(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	o := Defaults()
	o.MaxExhaustiveDepth = 5
	r := NewDFSRng(1, o)
	// current_pos <= 0 → false
	if r.EagerBacktracking(10) {
		t.Fatal("pos<=0 no eager")
	}
	_ = r.RndUptoSess(testAmbientSession, 2) // pos=0
	if r.EagerBacktracking(10) {
		t.Fatal("pos==0 no eager")
	}
	_ = r.RndUptoSess(testAmbientSession, 2) // pos=1, decision=1
	// remain = 5-1=4; depth_needed 3 → ok
	if r.EagerBacktracking(3) {
		t.Fatal("enough remain")
	}
	// depth_needed 5 → remain 4 < 5 → backtrack
	if !r.EagerBacktracking(5) || GetErrorSess(testAmbientSession) != ErrBacktracking {
		t.Fatal("eager", GetErrorSess(testAmbientSession))
	}
	ClearErrorSess(testAmbientSession)
}

func TestDFSDebugSequence(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	o := Defaults()
	o.MaxExhaustiveDepth = 5
	o.DFSDebugSequence = "3_1_4"
	r := NewDFSRng(1, o)
	if r == nil || HasErrorSess(testAmbientSession) {
		t.Fatal("debug seq ctor")
	}
	// debug path: get_number_by_pos after ++current_pos; starts -1 → pos 0 → 3
	v0 := r.RndUptoSess(testAmbientSession, 10)
	if HasErrorSess(testAmbientSession) || v0 != 3 {
		t.Fatalf("v0=%d err=%d", v0, GetErrorSess(testAmbientSession))
	}
	v1 := r.RndUptoSess(testAmbientSession, 10)
	if v1 != 1 {
		t.Fatal(v1)
	}
	v2 := r.RndUptoSess(testAmbientSession, 10)
	if v2 != 4 {
		t.Fatal(v2)
	}
	if !r.DFSGetAllDone() {
		t.Fatal("all_done at last")
	}
}

func TestDFSGetPrefixedName(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	o := Defaults()
	o.MaxExhaustiveDepth = 3
	r := NewDFSRng(1, o)
	_ = r.RndUptoSess(testAmbientSession, 2)
	_ = r.RndUptoSess(testAmbientSession, 3)
	// sequence "0_0"
	got := r.GetPrefixedNameDFS("foo")
	if got != "p_0_0_foo" {
		t.Fatal(got)
	}
}

func TestDFSDepthGuardIntegration(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	defer func() {
		RandomNumberDoFinalizationSess(testAmbientSession)
		ReinstallTestProcessSingletons()
		ClearErrorSess(testAmbientSession)
	}()
	o := Defaults()
	o.DFSExhaustive = true
	o.RandomBased = false
	o.MaxExhaustiveDepth = 8
	SetProcessOptionsSess(testAmbientSession, o)
	clearDFSImpl()
	CreateRandomNumberInstanceSess(testAmbientSession, RngKindDFS, 2)
	if DepthGuardByDepthSess(testAmbientSession, o, 1) != GoodDepth || HasErrorSess(testAmbientSession) {
		t.Fatal("fresh GOOD")
	}
	// burn a few choices so pos advances
	r := GetRndNumGeneratorSess(testAmbientSession)
	_ = r.RndUptoSess(testAmbientSession, 2)
	_ = r.RndUptoSess(testAmbientSession, 2)
	// pos=1, remain=7; need 20 → BAD + BACKTRACKING
	if DepthGuardByDepthSess(testAmbientSession, o, 20) != BadDepth || GetErrorSess(testAmbientSession) != ErrBacktracking {
		t.Fatal("deep need BAD", GetErrorSess(testAmbientSession))
	}
	ClearErrorSess(testAmbientSession)
}
