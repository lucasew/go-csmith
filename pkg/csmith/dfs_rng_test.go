package csmith

import "testing"

func TestDFSRandomChoiceFirstVisit(t *testing.T) {
	// DFSRndNumGenerator::random_choice first-visit path picks lowest valid v.
	ClearError()
	prevO := ProcessOptions()
	prevR := ProcessRng()
	defer func() {
		RandomNumberDoFinalization()
		SetProcessOptions(prevO)
		SetProcessRng(prevR)
		ClearError()
	}()
	o := Defaults()
	o.MaxExhaustiveDepth = 5
	o.DFSExhaustive = true
	o.RandomBased = false
	SetProcessOptions(o)
	clearDFSImpl()
	r := NewDFSRng(1, o)
	if r == nil {
		t.Fatal("NewDFSRng")
	}
	// first choice bound 3 → 0
	v := r.RndUpto(3)
	if HasError() || v != 0 {
		t.Fatalf("first upto got %d err=%d", v, GetError())
	}
	if r.DFSGetCurrentPos() != 0 {
		t.Fatal("pos", r.DFSGetCurrentPos())
	}
	// second choice
	v2 := r.RndUpto(4)
	if HasError() || v2 != 0 {
		t.Fatalf("second %d", v2)
	}
	seq := r.GetSequence()
	if seq != "0_0" {
		t.Fatal(seq)
	}
	// flipcoin p=100 forces 1
	ClearError()
	ok := r.RndFlipcoin(100)
	if HasError() || !ok {
		t.Fatal("p100", ok, GetError())
	}
	// flipcoin p=0 forces 0
	ClearError()
	ok = r.RndFlipcoin(0)
	if HasError() || ok {
		t.Fatal("p0", ok, GetError())
	}
}

func TestDFSRandomChoiceFilterRejects(t *testing.T) {
	ClearError()
	o := Defaults()
	o.MaxExhaustiveDepth = 4
	r := NewDFSRng(1, o)
	// reject 0 and 1 → pick 2
	f := RejectEQ(0)
	// chain: need filter that rejects 0 and 1
	f = filterFunc(func(v uint32) bool { return v < 2 })
	v := r.RndUptoFilter(4, f)
	if HasError() || v != 2 {
		t.Fatalf("got %d err=%d", v, GetError())
	}
}

func TestDFSBacktrackingExhaustsBranch(t *testing.T) {
	// After first choice, reset_state and re-walk with filter rejecting previous.
	ClearError()
	o := Defaults()
	o.MaxExhaustiveDepth = 2
	r := NewDFSRng(1, o)
	// depth 0: take 0 of bound 2
	if r.RndUpto(2) != 0 || HasError() {
		t.Fatal("first")
	}
	// depth 1: take 0 of bound 1 → only value 0
	if r.RndUpto(1) != 0 || HasError() {
		t.Fatal("second")
	}
	// reset for next program enumeration step
	r.DFSResetState()
	// re-enter depth 0: state still init from previous walk at pos 0?
	// reset_state only clears pos/seq/trace — does NOT clear states_.init
	// C++ reset_state same. Full re-search uses decision_depth backtracking.
	// Advance again: current_pos becomes 0, state.init true, decision_depth was 1.
	// current_pos(0) < decision_depth(1) && init → revisit returns same value 0.
	ClearError()
	v := r.RndUpto(2)
	if HasError() || v != 0 {
		t.Fatalf("revisit got %d err=%d", v, GetError())
	}
}

func TestDFSExceedMaxDepth(t *testing.T) {
	ClearError()
	o := Defaults()
	o.MaxExhaustiveDepth = 1
	r := NewDFSRng(1, o)
	_ = r.RndUpto(2)
	if HasError() {
		t.Fatal("first ok")
	}
	// second choice current_pos=1 >= max 1 → EXCEED
	_ = r.RndUpto(2)
	if GetError() != ErrExceedMaxDepth {
		t.Fatal("want exceed", GetError())
	}
	ClearError()
}

func TestDFSEagerBacktracking(t *testing.T) {
	ClearError()
	o := Defaults()
	o.MaxExhaustiveDepth = 5
	r := NewDFSRng(1, o)
	// current_pos <= 0 → false
	if r.EagerBacktracking(10) {
		t.Fatal("pos<=0 no eager")
	}
	_ = r.RndUpto(2) // pos=0
	if r.EagerBacktracking(10) {
		t.Fatal("pos==0 no eager")
	}
	_ = r.RndUpto(2) // pos=1, decision=1
	// remain = 5-1=4; depth_needed 3 → ok
	if r.EagerBacktracking(3) {
		t.Fatal("enough remain")
	}
	// depth_needed 5 → remain 4 < 5 → backtrack
	if !r.EagerBacktracking(5) || GetError() != ErrBacktracking {
		t.Fatal("eager", GetError())
	}
	ClearError()
}

func TestDFSDebugSequence(t *testing.T) {
	ClearError()
	o := Defaults()
	o.MaxExhaustiveDepth = 5
	o.DFSDebugSequence = "3_1_4"
	r := NewDFSRng(1, o)
	if r == nil || HasError() {
		t.Fatal("debug seq ctor")
	}
	// debug path: get_number_by_pos after ++current_pos; starts -1 → pos 0 → 3
	v0 := r.RndUpto(10)
	if HasError() || v0 != 3 {
		t.Fatalf("v0=%d err=%d", v0, GetError())
	}
	v1 := r.RndUpto(10)
	if v1 != 1 {
		t.Fatal(v1)
	}
	v2 := r.RndUpto(10)
	if v2 != 4 {
		t.Fatal(v2)
	}
	if !r.DFSGetAllDone() {
		t.Fatal("all_done at last")
	}
}

func TestDFSGetPrefixedName(t *testing.T) {
	ClearError()
	o := Defaults()
	o.MaxExhaustiveDepth = 3
	r := NewDFSRng(1, o)
	_ = r.RndUpto(2)
	_ = r.RndUpto(3)
	// sequence "0_0"
	got := r.GetPrefixedNameDFS("foo")
	if got != "p_0_0_foo" {
		t.Fatal(got)
	}
}

func TestDFSDepthGuardIntegration(t *testing.T) {
	ClearError()
	prevO := ProcessOptions()
	prevR := ProcessRng()
	defer func() {
		RandomNumberDoFinalization()
		SetProcessOptions(prevO)
		SetProcessRng(prevR)
		ClearError()
	}()
	o := Defaults()
	o.DFSExhaustive = true
	o.RandomBased = false
	o.MaxExhaustiveDepth = 8
	SetProcessOptions(o)
	clearDFSImpl()
	CreateRandomNumberInstance(RngKindDFS, 2)
	if DepthGuardByDepth(o, 1) != GoodDepth || HasError() {
		t.Fatal("fresh GOOD")
	}
	// burn a few choices so pos advances
	r := GetRndNumGenerator()
	_ = r.RndUpto(2)
	_ = r.RndUpto(2)
	// pos=1, remain=7; need 20 → BAD + BACKTRACKING
	if DepthGuardByDepth(o, 20) != BadDepth || GetError() != ErrBacktracking {
		t.Fatal("deep need BAD", GetError())
	}
	ClearError()
}
