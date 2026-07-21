package csmith

import "testing"

// StatementFor.cpp:355 map_facts_in entry FactVec; makeup FactMgr.cpp:494–508.
func TestPostLoopRestoresEntryUnionAndMakeupNewGlobals(t *testing.T) {
	ClearError()
	f := &Function{Name: "f", ReturnType: GetIntType()}
	fm := NewFactMgr(f)
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: GetIntType(), BitWidth: -1},
		{Name: "f4", Type: GetIntType(), BitWidth: -1},
	}}
	oldU := CreateVariableQfer("g_old", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	oldU.CreateFieldVars()
	oldU.Init = MakeInt(0)
	newU := CreateVariableQfer("g_new", ut, NewCVQualifiers([]bool{false}, []bool{false}))
	newU.CreateFieldVars()
	newU.Init = MakeInt(0)
	// preUnion: only old at fid 0
	preU := []*FactUnion{MakeFactUnion(oldU, 0)}
	// map_in / live polluted: old at 4, plus new global created in body
	polluted := []*FactUnion{MakeFactUnion(oldU, 4), MakeFactUnion(newU, 0)}
	body := &Block{Func: f, Looping: true, StmID: AllocStmID()}
	fm.SetMapFactsInPair(body.StmID, []*FactPointTo{}, polluted)
	fm.UnionFacts = CloneUnionFactSlice(polluted)
	fm.GlobalFacts = []*FactPointTo{}
	if fm.MapStmEffect == nil {
		fm.MapStmEffect = make(map[int]Effect)
	}
	fm.MapStmEffect[body.StmID] = EmptyEffect()
	forSt := &Stmt{Kind: StmtFor, Then: body, StmID: AllocStmID()}
	cg := WithFunc(f, EmptyEffect()).WithFactMgr(fm)
	postLoopAnalysis(fm, forSt, body, []*FactPointTo{}, preU, EmptyEffect(), &cg)
	if HasError() {
		t.Fatal(GetError())
	}
	gotOld := FindRelatedUnion(fm.UnionFacts, oldU)
	if gotOld == nil || gotOld.LastWrittenFID != 0 {
		t.Fatalf("old want fid 0 (entry), got %#v", gotOld)
	}
	gotNew := FindRelatedUnion(fm.UnionFacts, newU)
	if gotNew == nil {
		t.Fatal("new global union fact must be makeup from body")
	}
}
