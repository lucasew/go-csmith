package csmith

import "testing"

// CHECKLIST: Filter.cpp::*, VectorFilter.cpp::*

func TestFilterCtorEnablesAllKinds(t *testing.T) {
	// Filter.cpp:40 — kinds_.set() all true
	SetProcessOptionsSess(testAmbientSession, Defaults())
	f := NewVectorFilterSess(testAmbientSession, nil)
	if !f.ValidFilter() {
		t.Fatal("default ctor valid_filter true in random mode")
	}
	if f.CurrentKind() != FilterKindDefault {
		t.Fatalf("current_kind random: got %d", f.CurrentKind())
	}
}

func TestFilterDisableDefaultInvalidatesRandom(t *testing.T) {
	// Filter.cpp:55–57 disable; 74–79 valid_filter
	SetProcessOptionsSess(testAmbientSession, Defaults()) // RandomBased
	f := NewVectorFilterSess(testAmbientSession, nil)
	f.Disable(FilterKindDefault)
	if f.ValidFilter() {
		t.Fatal("after disable(fDefault), valid_filter false in random mode")
	}
	// VectorFilter.cpp:59–60 — invalid filter never rejects
	if f.Filter(0) || f.Filter(99) {
		t.Fatal("invalid filter must return false (accept)")
	}
	f.Enable(FilterKindDefault)
	if !f.ValidFilter() {
		t.Fatal("enable restores valid_filter")
	}
}

func TestFilterCurrentKindDFS(t *testing.T) {
	o := Defaults()
	o.RandomBased = false
	o.DFSExhaustive = true
	SetProcessOptionsSess(testAmbientSession, o)
	defer SetProcessOptionsSess(testAmbientSession, Defaults())
	f := NewVectorFilterSess(testAmbientSession, nil)
	if f.CurrentKind() != FilterKindDFS {
		t.Fatalf("dfs current_kind: got %d", f.CurrentKind())
	}
	f.Disable(FilterKindDFS)
	if f.ValidFilter() {
		t.Fatal("disable fDFS must invalidate in dfs mode")
	}
}

func TestVectorFilterFilterOutWithItems(t *testing.T) {
	// VectorFilter.cpp:58–66 FilterOut: reject if in set
	SetProcessOptionsSess(testAmbientSession, Defaults())
	f := NewVectorFilterItemsSess(testAmbientSession, []int{3, 7}, FilterModeOut)
	if !f.Filter(3) {
		t.Fatal("FilterOut must reject 3")
	}
	if f.Filter(4) {
		t.Fatal("FilterOut must accept 4")
	}
}

func TestVectorFilterKeepMode(t *testing.T) {
	// Keep: reject if NOT in set
	SetProcessOptionsSess(testAmbientSession, Defaults())
	f := NewVectorFilterItemsSess(testAmbientSession, []int{3}, FilterModeKeep)
	if f.Filter(3) {
		t.Fatal("Keep must accept 3")
	}
	if !f.Filter(4) {
		t.Fatal("Keep must reject 4")
	}
}

func TestVectorFilterAddDedup(t *testing.T) {
	// VectorFilter.cpp:68–72 — add only if not present
	f := NewVectorFilterSess(testAmbientSession, nil)
	f.AddSess(testAmbientSession, 1).AddSess(testAmbientSession, 1).AddSess(testAmbientSession, 2)
	if len(f.items) != 2 {
		t.Fatalf("items after dedup: %v", f.items)
	}
}

func TestVectorFilterLookupWithTable(t *testing.T) {
	// lookup through DistributionTable when valid
	SetProcessOptionsSess(testAmbientSession, Defaults())
	tab := &DistributionTable{}
	tab.AddEntrySess(testAmbientSession, 10, 50) // key 10 weight 50 → rnd 0..49 → 10
	tab.AddEntrySess(testAmbientSession, 20, 50) // key 20 weight 50 → rnd 50..99 → 20
	f := NewVectorFilterSess(testAmbientSession, tab)
	f.AddSess(testAmbientSession, 10) // filter out key 10
	// raw 0 → key 10 → reject
	if !f.Filter(0) {
		t.Fatal("raw 0 → key 10 should reject")
	}
	// raw 50 → key 20 → accept
	if f.Filter(50) {
		t.Fatal("raw 50 → key 20 should accept")
	}
	if f.MaxProb() != 100 {
		t.Fatalf("MaxProb: got %d want 100", f.MaxProb())
	}
}

func TestBlockProbabilityMatchesDisabledKeepFilter(t *testing.T) {
	// Block.cpp:87–93 — disable fDefault → uniform rnd_upto(block_size)
	SetProcessOptionsSess(testAmbientSession, Defaults())
	r := NewRngSess(testAmbientSession, 2)
	// first genrand % 4 == 1959434203 % 4
	want := int(NewRngSess(testAmbientSession, 2).RndUptoSess(testAmbientSession, 4))
	r = NewRngSess(testAmbientSession, 2)
	got := BlockProbabilitySess(testAmbientSession, 4, r)
	if got != want {
		t.Fatalf("BlockProbability(4) seed2: got %d want %d (uniform)", got, want)
	}
	if got < 0 || got >= 4 {
		t.Fatalf("out of range: %d", got)
	}
}

func TestFilterKindConstants(t *testing.T) {
	// Filter.h enum order
	if FilterKindDefault != 0 || FilterKindDFS != 1 || FilterKindMax != 2 {
		t.Fatalf("FilterKind order: %d %d %d", FilterKindDefault, FilterKindDFS, FilterKindMax)
	}
}
