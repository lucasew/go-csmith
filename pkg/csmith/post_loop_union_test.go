package csmith

import "testing"

// StatementFor.cpp:355 — non-must_return: global_facts = map_facts_in[body] only.
// Soft invent rewrote from preUnion + makeup; fair path keeps map_in last_written.
func TestPostLoopKeepsMapInUnionLattice(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	fm := NewFactMgrSess(testAmbientSession, f)
	ut := &Type{isUnion: true, Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
		{Name: "f4", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	oldU := CreateVariableQferSess(testAmbientSession, "g_old", ut, NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false}))
	oldU.CreateFieldVarsSess(testAmbientSession)
	oldU.Init = MakeIntSess(testAmbientSession, 0)
	newU := CreateVariableQferSess(testAmbientSession, "g_new", ut, NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false}))
	newU.CreateFieldVarsSess(testAmbientSession)
	newU.Init = MakeIntSess(testAmbientSession, 0)
	// preUnion (make_iteration snap) differs from map_in — must not clobber map_in
	preU := []*FactUnion{MakeFactUnionSess(testAmbientSession, oldU, 0)}
	// map_in after fair FP: old last_write 4 + body-created new global
	mapIn := []*FactUnion{MakeFactUnionSess(testAmbientSession, oldU, 4), MakeFactUnionSess(testAmbientSession, newU, 0)}
	body := &Block{Func: f, Looping: true, StmID: AllocStmIDSess(testAmbientSession)}
	fm.SetMapFactsInPair(body.StmID, []*FactPointTo{}, mapIn)
	fm.UnionFacts = CloneUnionFactSliceDeepSess(testAmbientSession, mapIn)
	fm.GlobalFacts = []*FactPointTo{}
	if fm.MapStmEffect == nil {
		fm.MapStmEffect = make(map[int]Effect)
	}
	fm.MapStmEffect[body.StmID] = EmptyEffect()
	forSt := &Stmt{Kind: StmtFor, Then: body, StmID: AllocStmIDSess(testAmbientSession)}
	cg := WithFunc(f, EmptyEffect()).WithSession(testAmbientSession).WithFactMgr(fm)
	postLoopAnalysis(fm, forSt, body, []*FactPointTo{}, preU, EmptyEffect(), &cg)
	if HasErrorSess(testAmbientSession) {
		t.Fatal(GetErrorSess(testAmbientSession))
	}
	gotOld := FindRelatedUnionSess(testAmbientSession, fm.UnionFacts, oldU)
	if gotOld == nil || gotOld.LastWrittenFID != 4 {
		t.Fatalf("non-must_return must keep map_in last_write 4, got %#v", gotOld)
	}
	gotNew := FindRelatedUnionSess(testAmbientSession, fm.UnionFacts, newU)
	if gotNew == nil {
		t.Fatal("map_in body-created union must remain")
	}
}

// Block.cpp:703 facts_copy = map_facts_in full FactVec; find_fixed_point starts
// current_inputs from that entry — not the post-generation live lattice.
// Soft invent: FindFixedPointBlock currentUnions = live UnionFacts (BOTTOM after
// if-combine) while map_in still had entry last=0 → set_fact_in wrote BOTTOM;
// post_loop + break merge left g_721 nonreadable (seed-123 choose ok 36 vs 37).
func TestPostCreationFPStartsUnionFromMapInNotLive(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	SetProcessOptionsSess(testAmbientSession, Defaults())
	f := &Function{Name: "func_t", ReturnType: GetIntTypeSess(testAmbientSession)}
	f.Stack = []*Block{}
	ut := &Type{
		isUnion: true, StructName: "U_pc",
		Fields: []StructField{
			{Name: "f0", Type: GetSimpleTypeSess(testAmbientSession, EChar), BitWidth: -1},
			{Name: "f1", Type: GetSimpleTypeSess(testAmbientSession, EUInt), BitWidth: -1},
		},
	}
	uv := CreateVariableQferSess(testAmbientSession, "g_u_pc", ut, NewCVQualifiersSess(testAmbientSession, []bool{false}, []bool{false}))
	uv.CreateFieldVarsSess(testAmbientSession)
	entryU := MakeFactUnionSess(testAmbientSession, uv, 0)
	bottomU := MakeFactUnionSess(testAmbientSession, uv, 0)
	if entryU == nil || bottomU == nil {
		t.Fatal("facts")
	}
	bottomU.SetBottomSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, f)
	// outer function body (non-looping) so post_creation does not append return
	outer := &Block{StmID: AllocStmIDSess(testAmbientSession), Func: f, Looping: false}
	// looping for-body, empty stms → no self-back (FromTailToHead false when empty)
	body := &Block{StmID: AllocStmIDSess(testAmbientSession), Func: f, Looping: true, Parent: outer}
	f.Body = outer
	f.Stack = []*Block{outer, body}
	// map_facts_in = entry last=0 (Block::make_random set_fact_in at start)
	fm.SetMapFactsInPair(body.StmID, []*FactPointTo{}, []*FactUnion{entryU})
	// live = post-generation BOTTOM (if-combine only-in-else IV write)
	fm.UnionFacts = []*FactUnion{bottomU}
	fm.GlobalFacts = []*FactPointTo{}
	if fm.MapStmEffect == nil {
		fm.MapStmEffect = make(map[int]Effect)
	}
	fm.MapStmEffect[body.StmID] = EmptyEffect()
	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	cg.CurrentFunc = f
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	body.PostCreationAnalysis(&cg, Defaults(), EmptyEffect(), NewRngSess(testAmbientSession, 1), NewVariableSelector(testAmbientSession, Defaults()))
	if HasErrorSess(testAmbientSession) {
		t.Fatal("post_creation", GetErrorSess(testAmbientSession))
	}
	// map_facts_in must retain entry last=0 (no self-back merge of BOTTOM live)
	inU := fm.GetMapUnionFactsIn(body.StmID)
	got := FindRelatedUnionSess(testAmbientSession, inU, uv)
	if got == nil {
		t.Fatal("map_in must keep union subject")
	}
	if got.IsBottomSess(testAmbientSession) || got.LastWrittenFID != 0 {
		t.Fatalf("post_creation FP must seed from map_in entry last=0, got %#v", got)
	}
}
