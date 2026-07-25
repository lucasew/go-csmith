package csmith

import "testing"

func TestUpdateFactsForDestDropsOOS(t *testing.T) {
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	outer := &Block{Func: f, LocalVars: []*Variable{
		CreateVariableScalarsSess(testAmbientSession, "l_1", GetIntTypeSess(testAmbientSession), false, false),
	}}
	// mark local properly
	loc := outer.LocalVars[0]
	loc.Name = "l_1"
	// force local flag via name prefix used by IsLocal
	inner := &Block{Parent: outer, Func: f}
	f.Blocks = []*Block{outer, inner}
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	// fact: g_p points to local l_1 which is OOS at dest with nil parent (outside func stack)
	// destParent = nil means only globals visible → local is OOS
	in := []*FactPointTo{MakeFactPointToSetSess(testAmbientSession, p, []*Variable{loc})}
	// also a fact about the local itself
	// locals that are subjects get dropped as OOS
	var out []*FactPointTo
	UpdateFactsForDestSess(testAmbientSession, in, &out, f, nil)
	// p fact should remain but pointee marked dead/garbage
	if len(out) == 0 {
		t.Fatal("expected ptr fact kept")
	}
	// subject p is global → not OOS
	fp := FindRelatedPointToSess(testAmbientSession, out, p)
	if fp == nil {
		t.Fatal("p gone")
	}
	// nil fact hole fails closed sticky — hole marker (not bare nil / empty complete)
	ClearErrorSess(testAmbientSession)
	var out2 []*FactPointTo
	UpdateFactsForDestSess(testAmbientSession, []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr), nil}, &out2, f, nil)
	if FactsComplete(out2) {
		t.Fatal("nil hole must fail closed incomplete dest facts", out2)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil hole UpdateFactsForDest must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nil PointTo hole on live fact fails closed sticky
	var out3 []*FactPointTo
	bad := MakeFactPointToSess(testAmbientSession, p, NullPtr)
	bad.PointTo = []*Variable{nil, loc}
	UpdateFactsForDestSess(testAmbientSession, []*FactPointTo{bad}, &out3, f, nil)
	if FactsComplete(out3) {
		t.Fatal("nil pointee hole must fail closed incomplete dest facts", out3)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil pointee UpdateFactsForDest must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// missing function fails closed sticky incomplete (no invent empty dest)
	var out4 []*FactPointTo
	UpdateFactsForDestSess(testAmbientSession, []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)}, &out4, nil, nil)
	if FactsComplete(out4) {
		t.Fatal("nil func must fail closed incomplete dest facts", out4)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil func UpdateFactsForDest must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// factsOut always live; sticky (no invent soft-skip dest update past hole)
	UpdateFactsForDestSess(testAmbientSession, []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)}, nil, f, nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil factsOut UpdateFactsForDest must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// IsVarOOS residual (Blocks hole before match): soft invent was soft-skip then merge.
	// Fair: sticky IncompleteFactSlice fail closed whole dest update.
	fHole := &Function{Name: "fh", ReturnType: GetIntTypeSess(testAmbientSession)}
	locH := &Variable{Name: "l_h", Type: GetIntTypeSess(testAmbientSession)}
	fHole.Blocks = []*Block{nil, {Func: fHole, LocalVars: []*Variable{locH}}}
	var outH []*FactPointTo
	UpdateFactsForDestSess(testAmbientSession, []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, locH)}, &outH, fHole, nil)
	if FactsComplete(outH) {
		t.Fatal("IsVarOOS residual must fail closed incomplete dest facts", outH)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("IsVarOOS residual UpdateFactsForDest must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestClearMapVisited(t *testing.T) {
	fm := NewFactMgrSess(testAmbientSession, nil)
	fm.MapVisited[1] = true
	fm.MapVisited[2] = true
	fm.ClearMapVisited()
	if fm.MapVisited[1] || fm.MapVisited[2] {
		t.Fatal(fm.MapVisited)
	}
	if _, ok := fm.MapVisited[1]; !ok {
		t.Fatal("keys kept")
	}
}

func TestSetupInOutMaps(t *testing.T) {
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	f1 := MakeFactPointToSess(testAmbientSession, p, NullPtr)
	fm.SetMapFactsIn(1, []*FactPointTo{f1})
	fm.SetMapFactsOut(1, []*FactPointTo{f1})
	fm.SetupInOutMaps(true)
	if len(fm.MapFactsInFinal[1]) != 1 {
		t.Fatal("first clone")
	}
	// second visit with wider fact
	f2 := MakeFactPointToSetSess(testAmbientSession, p, []*Variable{NullPtr, GarbagePtr})
	fm.SetMapFactsIn(1, []*FactPointTo{f2})
	fm.SetupInOutMaps(false)
	final := FindRelatedPointToSess(testAmbientSession, fm.MapFactsInFinal[1], p)
	if final == nil || len(final.PointTo) < 2 {
		t.Fatal("combine", final)
	}
}

func TestSetupInOutMapsFirstTimeIncompleteFailClosed(t *testing.T) {
	// FactMgr.cpp:208–222 — first_time copy_facts; Fact* always live
	// incomplete hole must not invent cleaned final map entry — sticky
	ClearErrorSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	// plant holes bypassing SetMapFacts* (CloneFactSlice strips holes)
	fm.MapFactsIn = map[int][]*FactPointTo{
		1: {MakeFactPointToSess(testAmbientSession, p, NullPtr), nil},
	}
	fm.MapFactsOut = map[int][]*FactPointTo{
		2: {MakeFactPointToSess(testAmbientSession, p, GarbagePtr), nil},
	}
	fm.SetupInOutMaps(true)
	if FactsComplete(fm.MapFactsInFinal[1]) {
		t.Fatal("incomplete MapFactsIn must not invent cleaned/complete InFinal")
	}
	// residual soft-continue invents later MapFactsOut complete clone past In hole
	// fair: sticky fail closed wipe finals (OutFinal must not invent complete entry 2)
	if out2, ok := fm.MapFactsOutFinal[2]; ok && FactsComplete(out2) && len(out2) > 0 && out2[0] != nil {
		t.Fatal("incomplete MapFactsIn residual must not invent complete OutFinal[2]", out2)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete first_time SetupInOutMaps must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// incomplete Out only: still sticky
	fmOut := NewFactMgrSess(testAmbientSession, nil)
	fmOut.MapFactsOut = map[int][]*FactPointTo{
		2: {MakeFactPointToSess(testAmbientSession, p, GarbagePtr), nil},
	}
	fmOut.SetupInOutMaps(true)
	if FactsComplete(fmOut.MapFactsOutFinal[2]) {
		t.Fatal("incomplete MapFactsOut must not invent cleaned/complete OutFinal")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete MapFactsOut SetupInOutMaps must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete sibling still clones
	fm2 := NewFactMgrSess(testAmbientSession, nil)
	fm2.MapFactsIn = map[int][]*FactPointTo{
		3: {MakeFactPointToSess(testAmbientSession, p, NullPtr)},
	}
	fm2.SetupInOutMaps(true)
	if len(fm2.MapFactsInFinal[3]) != 1 {
		t.Fatal("complete first_time must still clone")
	}
}

func TestSetupInOutMapsSiblingResidualSticky(t *testing.T) {
	// incomplete id soft invent was continue then clone later complete sibling final.
	// Fair: sticky fail closed whole SetupInOutMaps — wipe finals (no invent partial complete).
	ClearErrorSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	good := MakeFactPointToSess(testAmbientSession, p, NullPtr)
	fm.MapFactsIn = map[int][]*FactPointTo{
		1: {good, nil}, // incomplete
		2: {good},      // complete sibling
	}
	fm.SetupInOutMaps(true)
	// whole setup wiped — sibling must not invent complete final under any map order
	if in2, ok := fm.MapFactsInFinal[2]; ok && FactsComplete(in2) && len(in2) > 0 && in2[0] != nil {
		t.Fatal("sibling residual must not invent complete InFinal[2]", in2)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("sibling residual SetupInOutMaps must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSetupInOutMapsCombineIncompleteFailClosed(t *testing.T) {
	// second visit: incomplete current map must not invent join into final — sticky
	ClearErrorSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	f1 := MakeFactPointToSess(testAmbientSession, p, NullPtr)
	fm.SetMapFactsIn(1, []*FactPointTo{f1})
	fm.SetupInOutMaps(true)
	// plant incomplete current MapFactsIn
	fm.MapFactsIn[1] = []*FactPointTo{MakeFactPointToSetSess(testAmbientSession, p, []*Variable{NullPtr, GarbagePtr}), nil}
	fm.SetupInOutMaps(false)
	if FactsComplete(fm.MapFactsInFinal[1]) {
		t.Fatal("incomplete combine must fail closed incomplete final, not invent partial join")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete combine SetupInOutMaps must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestBackupRestoreStmFactMaps(t *testing.T) {
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	thenB := &Block{StmID: 20, Stmts: []Stmt{{StmID: 21}}}
	// StatementIf always has both arms
	st := &Stmt{Kind: StmtIfElse, StmID: 10, Then: thenB, Else: &Block{}}
	fm.SetMapFactsIn(10, []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)})
	fm.SetMapFactsOut(21, []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, GarbagePtr)})
	in := map[int][]*FactPointTo{}
	out := map[int][]*FactPointTo{}
	fm.BackupStmFactMaps(st, in, out, map[int][]*FactUnion{}, map[int][]*FactUnion{})
	// mutate
	fm.SetMapFactsIn(10, nil)
	fm.SetMapFactsOut(21, nil)
	fm.RestoreStmFactMaps(st, in, out, map[int][]*FactUnion{}, map[int][]*FactUnion{})
	if FindRelatedPointToSess(testAmbientSession, fm.MapFactsIn[10], p) == nil {
		t.Fatal("restored in")
	}
	if FindRelatedPointToSess(testAmbientSession, fm.MapFactsOut[21], p) == nil {
		t.Fatal("restored out")
	}
	// incomplete if — whole backup fail closed sticky (no invent root-only complete tree)
	ClearErrorSess(testAmbientSession)
	in2 := map[int][]*FactPointTo{}
	out2 := map[int][]*FactPointTo{}
	bad := &Stmt{Kind: StmtIfElse, StmID: 10, Then: thenB}
	fm.SetMapFactsIn(10, []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)})
	fm.SetMapFactsOut(21, []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, GarbagePtr)})
	fm.BackupStmFactMaps(bad, in2, out2, map[int][]*FactUnion{}, map[int][]*FactUnion{})
	if _, ok := out2[21]; ok {
		t.Fatal("incomplete if must not invent nested backup past nil Else")
	}
	if FactsComplete(in2[10]) || FactsComplete(out2[10]) {
		t.Fatal("incomplete if must backup root as IncompleteFactSlice, not invent complete", in2[10], out2[10])
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete if BackupStmFactMaps must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestBackupRestoreStmFactMapsUnionPartition(t *testing.T) {
	// FactMgr.cpp:516–548 — map_facts_in/out are full FactVec (ePointTo + eUnionWrite).
	// Soft invent was PT-only backup: restore left MapUnionFacts* at post-mutate state.
	ClearErrorSess(testAmbientSession)
	ut := &Type{isUnion: true, StructName: "U0", Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
		{Name: "f1", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	parent := CreateVariableScalarsSess(testAmbientSession, "g_u", ut, false, false)
	parent.CreateFieldVarsSess(testAmbientSession)
	entryU := MakeFactUnionSess(testAmbientSession, parent, 0)
	exitU := MakeFactUnionSess(testAmbientSession, parent, 1)
	if entryU == nil || exitU == nil {
		t.Fatal("MakeFactUnion")
	}
	fm := NewFactMgrSess(testAmbientSession, nil)
	st := &Stmt{Kind: StmtAssign, StmID: 15}
	fm.SetMapFactsInPair(15, []*FactPointTo{}, []*FactUnion{entryU})
	fm.SetMapFactsOutPair(15, []*FactPointTo{}, []*FactUnion{exitU})
	in := map[int][]*FactPointTo{}
	out := map[int][]*FactPointTo{}
	uin := map[int][]*FactUnion{}
	uout := map[int][]*FactUnion{}
	fm.BackupStmFactMaps(st, in, out, uin, uout)
	// mutate union maps after backup
	fm.SetMapFactsInPair(15, []*FactPointTo{}, []*FactUnion{exitU})
	fm.SetMapFactsOutPair(15, []*FactPointTo{}, []*FactUnion{entryU})
	fm.RestoreStmFactMaps(st, in, out, uin, uout)
	if HasErrorSess(testAmbientSession) {
		t.Fatal(GetErrorSess(testAmbientSession))
	}
	gotIn := fm.GetMapUnionFactsIn(15)
	gotOut := fm.GetMapUnionFactsOut(15)
	if len(gotIn) != 1 || gotIn[0] == nil || gotIn[0].LastWrittenFID != 0 {
		t.Fatalf("restored union in want fid 0, got %#v", gotIn)
	}
	if len(gotOut) != 1 || gotOut[0] == nil || gotOut[0].LastWrittenFID != 1 {
		t.Fatalf("restored union out want fid 1, got %#v", gotOut)
	}
	ClearErrorSess(testAmbientSession)
}

func TestBackupStmFactMapsIncompleteFailClosed(t *testing.T) {
	// incomplete source maps must not invent cleaned backup clones
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	st := &Stmt{Kind: StmtAssign, StmID: 15}
	fm.MapFactsIn = map[int][]*FactPointTo{
		15: {MakeFactPointToSess(testAmbientSession, p, NullPtr), nil},
	}
	fm.MapFactsOut = map[int][]*FactPointTo{
		15: {MakeFactPointToSess(testAmbientSession, p, GarbagePtr), nil},
	}
	in := map[int][]*FactPointTo{}
	out := map[int][]*FactPointTo{}
	fm.BackupStmFactMaps(st, in, out, map[int][]*FactUnion{}, map[int][]*FactUnion{})
	if FactsComplete(in[15]) {
		t.Fatal("incomplete MapFactsIn must backup incomplete, not invent cleaned")
	}
	if FactsComplete(out[15]) {
		t.Fatal("incomplete MapFactsOut must backup incomplete, not invent cleaned")
	}
	// restore incomplete backup → incomplete maps (not invent cleaned)
	fm.MapFactsIn[15] = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, GarbagePtr)}
	fm.MapFactsOut[15] = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, GarbagePtr)}
	in[15] = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr), nil}
	out[15] = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr), nil}
	fm.RestoreStmFactMaps(st, in, out, map[int][]*FactUnion{}, map[int][]*FactUnion{})
	if FactsComplete(fm.MapFactsIn[15]) || FactsComplete(fm.MapFactsOut[15]) {
		t.Fatal("restore incomplete backup must fail closed incomplete")
	}
}

func TestFindUpdatedFacts(t *testing.T) {
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	fm.SetMapFactsIn(1, []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)})
	fm.SetMapFactsOut(1, []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, GarbagePtr)})
	u := fm.FindUpdatedFacts(1)
	if len(u) != 1 {
		t.Fatal(u)
	}
	// FactMgr always live; sticky IncompleteFactSlice (no invent empty complete)
	ClearErrorSess(testAmbientSession)
	if FactsComplete((*FactMgr)(nil).FindUpdatedFacts(1)) {
		t.Fatal("nil FM FindUpdatedFacts must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM FindUpdatedFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if FactsComplete(fm.FindUpdatedFacts(IncompleteStmID)) {
		t.Fatal("stmID 0 FindUpdatedFacts must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("stmID 0 FindUpdatedFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	if FactsComplete((*FactMgr)(nil).FindUpdatedFinalFacts(1)) {
		t.Fatal("nil FM FindUpdatedFinalFacts must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM FindUpdatedFinalFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// equal → no update
	fm.SetMapFactsOut(1, []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)})
	if len(fm.FindUpdatedFacts(1)) != 0 {
		t.Fatal("no change")
	}
	// FactMgr.cpp:660 assert(prev_f) — out-only fact without in match is not updated
	q := CreateVariableScalarsSess(testAmbientSession, "g_q", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	fm.SetMapFactsOut(1, []*FactPointTo{MakeFactPointToSess(testAmbientSession, q, NullPtr)})
	if len(fm.FindUpdatedFacts(1)) != 0 {
		t.Fatal("missing prev must fail closed, not invent as updated")
	}
	// Equal residual: PointTo nil hole soft invent was continue then partial updated list
	ClearErrorSess(testAmbientSession)
	goodIn := MakeFactPointToSess(testAmbientSession, p, NullPtr)
	badOut := &FactPointTo{Var: p, PointTo: []*Variable{GarbagePtr, nil}} // hole for Equal
	goodOutOther := MakeFactPointToSess(testAmbientSession, q, GarbagePtr)
	// plant complete maps with Equal residual on p then later q would invent partial
	fm.MapFactsIn[1] = []*FactPointTo{goodIn, MakeFactPointToSess(testAmbientSession, q, NullPtr)}
	fm.MapFactsOut[1] = []*FactPointTo{badOut, goodOutOther}
	if FactsComplete(fm.FindUpdatedFacts(1)) {
		t.Fatal("Equal residual must fail closed incomplete, not invent partial updated")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("Equal residual FindUpdatedFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nil fact hole fails closed sticky at store and find_updated
	ClearErrorSess(testAmbientSession)
	fm.SetMapFactsIn(1, []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)})
	fm.SetMapFactsOut(1, []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, GarbagePtr), nil})
	// incomplete store is hole marker (FactsComplete false), not cleaned list
	if FactsComplete(fm.MapFactsOut[1]) {
		t.Fatal("SetMapFactsOut must not invent cleaned/complete list from nil hole")
	}
	if FactsComplete(fm.FindUpdatedFacts(1)) {
		t.Fatal("incomplete out map must fail closed incomplete, not invent empty complete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete out FindUpdatedFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// direct find with hole in out (bypass Set)
	fm.MapFactsOut[1] = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, GarbagePtr), nil}
	if FactsComplete(fm.FindUpdatedFacts(1)) {
		t.Fatal("nil fact hole in out must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil hole FindUpdatedFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// final maps: incomplete → sticky IncompleteFactSlice (not bare nil invent empty)
	fm.MapFactsInFinal = map[int][]*FactPointTo{1: {MakeFactPointToSess(testAmbientSession, p, NullPtr)}}
	fm.MapFactsOutFinal = map[int][]*FactPointTo{1: {MakeFactPointToSess(testAmbientSession, p, GarbagePtr), nil}}
	if FactsComplete(fm.FindUpdatedFinalFacts(1)) {
		t.Fatal("incomplete final out must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete final FindUpdatedFinalFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestRestoreFacts(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	q := CreateVariableScalarsSess(testAmbientSession, "g_q", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	old := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)}
	fm.GlobalFacts = []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, GarbagePtr), MakeFactPointToSess(testAmbientSession, q, NullPtr)}
	fm.RestoreFacts(old)
	// p restored to old; q added via makeup
	if FindRelatedPointToSess(testAmbientSession, fm.GlobalFacts, p) == nil {
		t.Fatal("p")
	}
	if FindRelatedPointToSess(testAmbientSession, fm.GlobalFacts, q) == nil {
		t.Fatal("makeup q")
	}
	// incomplete oldFacts fails closed sticky (no invent soft re-pick past wipe)
	hole := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr), nil}
	fm.GlobalFacts = []*FactPointTo{MakeFactPointToSess(testAmbientSession, q, NullPtr)}
	fm.RestoreFacts(hole)
	if FactsComplete(fm.GlobalFacts) {
		t.Fatal("incomplete oldFacts must wipe GlobalFacts incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete oldFacts RestoreFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// FactMgr always live; sticky no invent soft-skip restore past hole
	(*FactMgr)(nil).RestoreFacts(old)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil FM RestoreFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

// TestRestoreFactsDoesNotReinjectLiveMayNull: FactMgr.cpp:489–492 is makeup +
// assign only. Do not invent re-join of live may-null into the restored snapshot
// (SPEC: no invent may-null reinject).
func TestRestoreFactsDoesNotReinjectLiveMayNull(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, &Function{Name: "f"})
	g := CreateVariableScalarsSess(testAmbientSession, "g_127", PointerToSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EShort)), false, false)
	p := CreateVariableScalarsSess(testAmbientSession, "l_233", PointerToSess(testAmbientSession, PointerToSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EShort))), false, false)
	p.IsArray = true
	snap := []*FactPointTo{MakeFactPointToSetSess(testAmbientSession, p, []*Variable{g})}
	fm.GlobalFacts = []*FactPointTo{MakeFactPointToSetSess(testAmbientSession, p, []*Variable{NullPtr, g})}
	fm.RestoreFacts(snap)
	if !FactsComplete(fm.GlobalFacts) {
		t.Fatal("restore incomplete", HasErrorSess(testAmbientSession))
	}
	fp := FindRelatedPointToSess(testAmbientSession, fm.GlobalFacts, p)
	if fp == nil {
		t.Fatal("missing fact after restore")
	}
	if fp.IsNullSess(testAmbientSession) {
		t.Fatalf("restore must not reinject live may-null, pts=%v", fp.PointTo)
	}
}

func TestMakeupNewVarFactsIncompleteFailClosed(t *testing.T) {
	// incomplete old/new maps must not invent partial makeup past holes — sticky
	ClearErrorSess(testAmbientSession)
	if MakeupNewVarFactsSess(testAmbientSession, nil, nil) {
		t.Fatal("nil oldFacts must fail closed false")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil oldFacts MakeupNewVarFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	q := CreateVariableScalarsSess(testAmbientSession, "g_q", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	old := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr), nil}
	newF := []*FactPointTo{MakeFactPointToSess(testAmbientSession, q, NullPtr)}
	if MakeupNewVarFactsSess(testAmbientSession, &old, newF) {
		t.Fatal("incomplete oldFacts must fail closed false")
	}
	if FactsComplete(old) {
		t.Fatal("incomplete oldFacts must fail closed nil")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete oldFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	old2 := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)}
	new2 := []*FactPointTo{MakeFactPointToSess(testAmbientSession, q, NullPtr), nil}
	if MakeupNewVarFactsSess(testAmbientSession, &old2, new2) {
		t.Fatal("incomplete newFacts must fail closed false")
	}
	if FactsComplete(old2) {
		t.Fatal("incomplete newFacts must fail closed incomplete oldFacts")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete newFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeupNewVarFactsAddNewHoleStopsLaterVars(t *testing.T) {
	// AddNewVarFactInto FieldVars hole clears *oldFacts; must not invent later vars
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	SetProcessOptionsSess(testAmbientSession, opts)
	SetProcessProbabilitiesSess(testAmbientSession, NewProbabilities(opts))
	// non-pointer aggregate with nil field hole (global name)
	agg := CreateVariableScalarsSess(testAmbientSession, "g_agg", GetIntTypeSess(testAmbientSession), true, false)
	if agg == nil {
		t.Fatal("agg")
	}
	agg.FieldVars = []*Variable{nil}
	later := CreateVariableScalarsSess(testAmbientSession, "g_later", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	old := []*FactPointTo{}
	// both appear as new_facts subjects so makeup tries AddNewVarFactInto for each
	newF := []*FactPointTo{
		MakeFactPointToSess(testAmbientSession, agg, NullPtr),
		MakeFactPointToSess(testAmbientSession, later, NullPtr),
	}
	if MakeupNewVarFactsSess(testAmbientSession, &old, newF) {
		t.Fatal("FieldVars hole must fail closed false")
	}
	if FactsComplete(old) {
		t.Fatal("must not invent re-accumulate later pointer after hole", old)
	}
	if FindRelatedPointToSess(testAmbientSession, old, later) != nil {
		t.Fatal("later var must not be made up past field hole")
	}
	ClearErrorSess(testAmbientSession)
}

func TestMakeupNewVarFactsAmbientResidualSticky(t *testing.T) {
	// Ambient residual ERROR soft invent was soft-continue makeup later complete vars.
	// Fair: sticky wipe IncompleteFactSlice whole MakeupNewVarFacts.
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	old := []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)}
	SetErrorSess(testAmbientSession, ErrGeneric)
	q := CreateVariableScalarsSess(testAmbientSession, "g_q", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	newF := []*FactPointTo{MakeFactPointToSess(testAmbientSession, q, NullPtr)}
	if MakeupNewVarFactsSess(testAmbientSession, &old, newF) {
		t.Fatal("ambient residual must fail closed MakeupNewVarFacts")
	}
	if FactsComplete(old) {
		t.Fatal("ambient residual must wipe IncompleteFactSlice, not invent later makeup")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("ambient residual MakeupNewVarFacts must keep SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSetMapFactsOutGotoDest(t *testing.T) {
	// FactMgr.cpp:263–266 — update_facts_for_dest via StatementGoto::dest;
	// no soft invent RemoveFunctionLocalFacts when dest fields present.
	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	// nested local not visible at body dest → OOS/garbage
	innerLoc := CreateVariableScalarsSess(testAmbientSession, "l_inner", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	inner := &Block{Func: f, LocalVars: []*Variable{innerLoc}}
	body := &Block{Func: f, LocalVars: nil}
	inner.Parent = body
	f.Body = body
	f.Blocks = []*Block{body, inner}
	// dest stmt at body level
	dest := &Stmt{Kind: StmtAssign, StmID: 10}
	body.Stmts = []Stmt{*dest}
	// goto in inner jumps to dest in body
	st := &Stmt{
		Kind: StmtGoto, StmID: 3,
		GotoDestStmID:  10,
		GotoDestParent: body,
	}
	inner.Stmts = []Stmt{*st}
	fm := NewFactMgrSess(testAmbientSession, f)
	g := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), false, false)
	// global points to inner local; after goto to body, pointee is OOS → garbage
	facts := []*FactPointTo{
		MakeFactPointToSess(testAmbientSession, g, innerLoc),
		MakeFactPointToSess(testAmbientSession, innerLoc, NullPtr),
	}
	// SetMapFactsOutForStmt resolves GotoDestParent (no invent approx drop)
	fm.SetMapFactsOutForStmt(st, facts, inner)
	out := fm.MapFactsOut[3]
	if out == nil {
		t.Fatal("out set")
	}
	// global kept; subject innerLoc OOS at body dest → dropped from subjects
	if FindRelatedPointToSess(testAmbientSession, out, g) == nil {
		t.Fatal("global lost", out)
	}
	// pointee of g marked dead (OOS local)
	gf := FindRelatedPointToSess(testAmbientSession, out, g)
	if gf == nil || !gf.IsDeadSess(testAmbientSession) {
		t.Fatalf("want g→garbage after OOS pointee, got %+v", gf)
	}
	// return uses s->parent stack walk, not invent f.Body-only
	ret := &Stmt{Kind: StmtReturn, StmID: 4}
	fm.SetMapFactsOutForStmt(ret, facts, inner)
	retOut := fm.MapFactsOut[4]
	// innerLoc on stack at return → subject dropped
	if FindRelatedPointToSess(testAmbientSession, retOut, innerLoc) != nil {
		t.Fatal("return must drop stack local subject", retOut)
	}
}

func TestSetMapFactsIncompleteStoresNil(t *testing.T) {
	// incomplete facts must not invent cleaned MapFactsIn/Out entry
	// stored as hole marker (FactsComplete false), not bare nil (FactsComplete true)
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	fm.SetMapFactsIn(1, []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr), nil})
	if FactsComplete(fm.MapFactsIn[1]) {
		t.Fatal("SetMapFactsIn incomplete must not invent complete empty")
	}
	fm.SetMapFactsOut(2, []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, GarbagePtr), nil})
	if FactsComplete(fm.MapFactsOut[2]) {
		t.Fatal("SetMapFactsOut incomplete must not invent complete empty")
	}
	// complete empty is non-nil empty slice (FactsComplete true)
	fm.SetMapFactsIn(3, nil)
	if !FactsComplete(fm.MapFactsIn[3]) || fm.MapFactsIn[3] == nil {
		t.Fatal("complete empty must store non-nil empty, not hole/bare nil", fm.MapFactsIn[3])
	}
}

func TestCollectLoopLocalVarsNilHoleFailClosed(t *testing.T) {
	// LocalVars nil hole fails closed sticky (no invent skip partial OOS list)
	ClearErrorSess(testAmbientSession)
	loop := &Block{Looping: true, LocalVars: []*Variable{
		CreateVariableScalarsSess(testAmbientSession, "l_1", GetIntTypeSess(testAmbientSession), false, false),
		nil,
	}}
	if VariablesComplete(collectLoopLocalVarsSess(testAmbientSession, loop)) {
		t.Fatal("nil LocalVars hole must fail closed incomplete")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil LocalVars collectLoopLocalVars must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// complete empty
	empty := &Block{Looping: true}
	got := collectLoopLocalVarsSess(testAmbientSession, empty)
	if got == nil {
		t.Fatal("empty complete must be non-nil empty")
	}
	if len(got) != 0 {
		t.Fatal(got)
	}
	// incomplete facts on RemoveLoopLocalFacts sticky
	ClearErrorSess(testAmbientSession)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	if FactsComplete(RemoveLoopLocalFactsSess(testAmbientSession, []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr), nil}, empty)) {
		t.Fatal("incomplete facts RemoveLoopLocalFacts must fail closed")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete facts RemoveLoopLocalFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// nil blk soft invent was complete passthrough keep-all-facts (including loop locals)
	// fair: sticky IncompleteFactSlice so break/continue cannot invent cleaned out map
	prior := MakeFactPointToSess(testAmbientSession, p, NullPtr)
	gotNil := RemoveLoopLocalFactsSess(testAmbientSession, []*FactPointTo{prior}, nil)
	if FactsComplete(gotNil) {
		t.Fatal("nil blk RemoveLoopLocalFacts must fail closed, not keep-all passthrough", gotNil)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil blk RemoveLoopLocalFacts must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// RemoveLoopLocalFactsForStmt with nil parent (non-block) same sticky hole
	br := &Stmt{Kind: StmtBreak, StmID: 9}
	gotFor := RemoveLoopLocalFactsForStmtSess(testAmbientSession, []*FactPointTo{prior}, br, nil)
	if FactsComplete(gotFor) {
		t.Fatal("nil parent RemoveLoopLocalFactsForStmt must fail closed", gotFor)
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("nil parent RemoveLoopLocalFactsForStmt must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestSetMapFactsOutForStmtIncompleteFailClosed(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	fm := NewFactMgrSess(testAmbientSession, nil)
	p := CreateVariableScalarsSess(testAmbientSession, "g_p", PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession)), true, false)
	st := &Stmt{Kind: StmtAssign, StmID: 5}
	fm.SetMapFactsOutForStmt(st, []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr), nil}, nil)
	if FactsComplete(fm.MapFactsOut[5]) {
		t.Fatal("incomplete set_fact_out must not invent complete empty")
	}
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("incomplete set_fact_out must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
	// StmID 0 fails closed sticky (no invent silent set_fact_out)
	st0 := &Stmt{Kind: StmtAssign, StmID: IncompleteStmID}
	fm.SetMapFactsOutForStmt(st0, []*FactPointTo{MakeFactPointToSess(testAmbientSession, p, NullPtr)}, nil)
	if !HasErrorSess(testAmbientSession) {
		t.Fatal("StmID 0 set_fact_out must SetError sticky")
	}
	ClearErrorSess(testAmbientSession)
}

func TestArrayPointerAssignMergesNotRenews(t *testing.T) {
	// FactMgr.cpp:378 — array LHS merges.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	SetProcessOptionsSess(testAmbientSession, opts)
	SetProcessProbabilitiesSess(testAmbientSession, NewProbabilities(opts))
	SetProcessRngSess(testAmbientSession, NewRngSess(testAmbientSession, 1))

	g := CreateVariableScalarsSess(testAmbientSession, "g_127", PointerToSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EShort)), false, false)
	elem := PointerToSess(testAmbientSession, PointerToSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EShort)))
	base := CreateVariableScalarsSess(testAmbientSession, "l_233", elem, false, false)
	av := &ArrayVariable{Variable: *base, Sizes: []int{10}}
	av.IsArray = true
	av.AsArray = av
	av.Name = "l_233"
	av.Type = elem

	fm := NewFactMgrSess(testAmbientSession, &Function{Name: "f"})
	fm.GlobalFacts = []*FactPointTo{MakeFactPointToSess(testAmbientSession, &av.Variable, NullPtr)}
	rhs := &Expression{Term: TermVariable, Var: g, ExprType: elem}
	_ = fm.UpdateFactForAssign(&av.Variable, 0, rhs)
	fp := FindRelatedPointToSess(testAmbientSession, fm.GlobalFacts, &av.Variable)
	if fp == nil || !fp.IsNullSess(testAmbientSession) {
		t.Fatalf("merge keep null; fp=%v", fp)
	}
}

func TestAbstractFactForVarInitArrayPointerMergesAlts(t *testing.T) {
	// Fact.cpp:97–109 — primary init + more_init_values merge.
	// Primary &g plus Constant 0 alt must leave may-null.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	SetProcessOptionsSess(testAmbientSession, opts)
	SetProcessProbabilitiesSess(testAmbientSession, NewProbabilities(opts))
	SetProcessRngSess(testAmbientSession, NewRngSess(testAmbientSession, 2))

	g := CreateVariableScalarsSess(testAmbientSession, "g_127", PointerToSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EShort)), false, false)
	elem := PointerToSess(testAmbientSession, PointerToSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EShort)))
	base := CreateVariableScalarsSess(testAmbientSession, "l_233", elem, false, false)
	av := &ArrayVariable{Variable: *base, Sizes: []int{10}}
	av.IsArray = true
	av.AsArray = av
	av.Name = "l_233"
	av.Type = elem
	// primary = address-of g
	av.InitExpr = &Expression{Term: TermVariable, Var: g, ExprType: elem}
	// alt = null constant
	av.InitExprs = []*Expression{{Term: TermConstant, Con: &Constant{Type: elem, Value: "0"}, ExprType: elem}}

	pt, _ := AbstractFactForVarInitSess(testAmbientSession, &av.Variable)
	if !FactsComplete(pt) || len(pt) != 1 {
		t.Fatalf("abstract incomplete n=%d err=%v", len(pt), HasErrorSess(testAmbientSession))
	}
	if !pt[0].IsNullSess(testAmbientSession) {
		names := []string{}
		for _, p := range pt[0].PointTo {
			if p != nil {
				names = append(names, p.Name)
			}
		}
		t.Fatalf("want may-null after merge alts, PointTo=%v", names)
	}
}

func TestEqualsIntZeroPointer(t *testing.T) {
	e := &Expression{Term: TermConstant, Con: MakeIntSess(testAmbientSession, 0), ExprType: PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))}
	if !e.EqualsIntSess(testAmbientSession, 0) {
		t.Fatalf("EqualsInt(0) false for MakeIntSess(testAmbientSession, 0) pointer expr; Con=%v", e.Con)
	}
}

func TestAbstractFactAssignConstant0Pointer(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	elem := PointerToSess(testAmbientSession, GetIntTypeSess(testAmbientSession))
	base := CreateVariableScalarsSess(testAmbientSession, "p", elem, false, false)
	av := &ArrayVariable{Variable: *base, Sizes: []int{2}}
	av.IsArray = true
	av.AsArray = av
	av.Name = "p"
	av.Type = elem
	rhs := &Expression{Term: TermConstant, Con: &Constant{Type: elem, Value: "0"}, ExprType: elem}
	pt, _ := AbstractFactForAssignSess(testAmbientSession, nil, &av.Variable, 0, rhs)
	if !FactsComplete(pt) || len(pt) != 1 {
		t.Fatalf("n=%d complete=%v err=%v", len(pt), FactsComplete(pt), HasErrorSess(testAmbientSession))
	}
	if !pt[0].IsNullSess(testAmbientSession) {
		names := []string{}
		for _, x := range pt[0].PointTo {
			if x != nil {
				names = append(names, x.Name)
			}
		}
		t.Fatalf("const0 must be null, pts=%v", names)
	}
}

func TestUpdateFactArrayAssignKeepsMayNull(t *testing.T) {
	// After may-null lattice, definitive &g assign to array must merge not wipe null.
	// FactMgr.cpp:378–388 — isArray → merge_fact not renew_fact.
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	SetProcessOptionsSess(testAmbientSession, opts)
	SetProcessProbabilitiesSess(testAmbientSession, NewProbabilities(opts))
	SetProcessRngSess(testAmbientSession, NewRngSess(testAmbientSession, 1))

	g := CreateVariableScalarsSess(testAmbientSession, "g_127", PointerToSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EShort)), false, false)
	elem := PointerToSess(testAmbientSession, PointerToSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EShort)))
	base := CreateVariableScalarsSess(testAmbientSession, "l_233", elem, false, false)
	av := &ArrayVariable{Variable: *base, Sizes: []int{10}}
	av.IsArray = true
	av.AsArray = av
	av.Name = "l_233"
	av.Type = elem
	// primary init &g
	av.InitExpr = &Expression{Term: TermVariable, Var: g, ExprType: elem}

	fm := NewFactMgrSess(testAmbientSession, &Function{Name: "f"})
	// seed may-null: null + g_127
	fm.GlobalFacts = []*FactPointTo{MakeFactPointToSetSess(testAmbientSession, &av.Variable, []*Variable{NullPtr, g})}
	// assign l_233 = &g_127 (address-of as ExpressionVariable with pointer type)
	rhs := &Expression{Term: TermVariable, Var: g, ExprType: elem}
	if !fm.UpdateFactForAssign(&av.Variable, 0, rhs) {
		t.Fatalf("update failed err=%v", HasErrorSess(testAmbientSession))
	}
	fp := FindRelatedPointToSess(testAmbientSession, fm.GlobalFacts, &av.Variable)
	if fp == nil {
		t.Fatal("missing fact")
	}
	if !fp.IsNullSess(testAmbientSession) {
		names := []string{}
		for _, p := range fp.PointTo {
			if p != nil {
				names = append(names, p.Name)
			}
		}
		t.Fatalf("array assign must merge keep may-null, pts=%v", names)
	}
}

// TestFixedPointBlockReintroducesMayNull: loop body fixed-point must re-apply
// assigns that introduce null so map_facts_in (after back-edge merge) and
// post_loop restore can keep may-null (seed-2 l_233 / first_div 10107).
func TestFixedPointBlockReintroducesMayNull(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	opts := Defaults()
	SetProcessOptionsSess(testAmbientSession, opts)
	SetProcessProbabilitiesSess(testAmbientSession, NewProbabilities(opts))
	SetProcessRngSess(testAmbientSession, NewRngSess(testAmbientSession, 1))

	f := &Function{Name: "f", ReturnType: GetIntTypeSess(testAmbientSession)}
	fm := NewFactMgrSess(testAmbientSession, f)
	g := CreateVariableScalarsSess(testAmbientSession, "g_127", PointerToSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EShort)), false, false)
	elem := PointerToSess(testAmbientSession, PointerToSess(testAmbientSession, GetSimpleTypeSess(testAmbientSession, EShort)))
	base := CreateVariableScalarsSess(testAmbientSession, "l_233", elem, false, false)
	av := &ArrayVariable{Variable: *base, Sizes: []int{10}}
	av.IsArray = true
	av.AsArray = av
	av.Name = "l_233"
	av.Type = elem
	// Entry: only g_127 (as if map_facts_in taken before body assign)
	entry := []*FactPointTo{MakeFactPointToSetSess(testAmbientSession, &av.Variable, []*Variable{g})}
	// Body statement: l_233 = null (Constant 0 pointer)
	nullRHS := &Expression{Term: TermConstant, Con: &Constant{Type: elem, Value: "0"}, ExprType: elem}
	st := &Stmt{
		Kind: StmtAssign, StmID: 2, LhsVar: &av.Variable,
		Lhs:  &Lhs{Var: &av.Variable, Type: elem},
		Expr: nullRHS, AssignOp: AssignSimple,
	}
	body := &Block{
		Func: f, StmID: 1, Looping: true, Parent: nil,
		Stmts: []Stmt{*st},
	}
	f.Blocks = []*Block{body}
	f.Stack = []*Block{body}
	// Entry facts in map (body start)
	fm.SetMapFactsIn(body.StmID, entry)
	// Self back-edge (loop)
	fm.CreateCFGEdge(body.StmID, body, false, true)

	cg := EmptyCGContext().WithSession(testAmbientSession).WithFactMgr(fm)
	cg.CurrentFunc = f
	eff := EmptyEffect()
	cg.EffectAccum = &eff
	opts2 := Defaults()

	out, _, failIdx, ok := FindFixedPointBlock(body, CloneFactSliceSess(testAmbientSession, entry), &cg, opts2, true)
	if !ok {
		t.Fatalf("fixed-point failed idx=%d err=%v", failIdx, HasErrorSess(testAmbientSession))
	}
	// After fixed-point, map_facts_in should have absorbed may-null from out via
	// back-edge merge on iteration 2+ (or out itself has may-null).
	inAfter := fm.GetMapFactsIn(body.StmID)
	fpIn := FindRelatedPointToSess(testAmbientSession, inAfter, &av.Variable)
	fpOut := FindRelatedPointToSess(testAmbientSession, out, &av.Variable)
	if fpOut == nil || !fpOut.IsNullSess(testAmbientSession) {
		t.Fatalf("map_facts_out/return must be may-null after null assign; out=%v", fpOut)
	}
	// post_loop uses map_facts_in — must eventually include null after ≥2 iters
	if fpIn == nil || !fpIn.IsNullSess(testAmbientSession) {
		// document actual state for diagnosis
		inPts, outPts := []string{}, []string{}
		if fpIn != nil {
			for _, p := range fpIn.PointTo {
				if p != nil {
					inPts = append(inPts, p.Name)
				}
			}
		}
		if fpOut != nil {
			for _, p := range fpOut.PointTo {
				if p != nil {
					outPts = append(outPts, p.Name)
				}
			}
		}
		t.Fatalf("map_facts_in must gain may-null after back-edge merge; in=%v out=%v", inPts, outPts)
	}
}

// FactMgr.cpp:629–639 — remove_loop_local_facts is full FactVec (ePointTo + eUnionWrite).
// Soft invent was PT-only OOS on continue/break map_out: parent-block eUnionWrite
// subjects (l_810) survived into map_union_out and polluted for-body back-edge merge.
func TestRemoveLoopLocalUnionFactsDropsParentLocals(t *testing.T) {
	ClearErrorSess(testAmbientSession)
	defer ClearErrorSess(testAmbientSession)
	ut := &Type{isUnion: true, StructName: "U2", Fields: []StructField{
		{Name: "f0", Type: GetIntTypeSess(testAmbientSession), BitWidth: -1},
	}}
	loop := &Block{Looping: true, StmID: 333, LocalVars: []*Variable{}}
	parent := &Block{Parent: loop, Looping: false, StmID: 336}
	l810 := CreateVariableScalarsSess(testAmbientSession, "l_810", ut, false, false)
	parent.LocalVars = []*Variable{l810}
	inner := &Block{Parent: parent, Looping: false, StmID: 367, LocalVars: []*Variable{}}
	// continue lives in inner; walk collects inner + parent (l_810) + loop locals
	fm := NewFactMgrSess(testAmbientSession, &Function{Name: "func_t", ReturnType: GetIntTypeSess(testAmbientSession)})
	fm.UnionFacts = []*FactUnion{
		MakeFactUnionSess(testAmbientSession, CreateVariableScalarsSess(testAmbientSession, "g_25", ut, true, false), 0),
		MakeFactUnionSess(testAmbientSession, l810, 0),
	}
	cont := &Stmt{Kind: StmtContinue, StmID: 379}
	fm.SetMapFactsOutForStmt(cont, []*FactPointTo{}, inner)
	if HasErrorSess(testAmbientSession) {
		t.Fatal("set_fact_out continue sticky", GetErrorSess(testAmbientSession))
	}
	outU := fm.GetMapUnionFactsOut(379)
	if !UnionFactsComplete(outU) {
		t.Fatal("map_union_out incomplete", outU)
	}
	if FindRelatedUnionSess(testAmbientSession, outU, l810) != nil {
		t.Fatalf("continue map_out must OOS parent-block union l_810, got %v", outU)
	}
	// global union subject must remain
	if FindRelatedUnionSess(testAmbientSession, outU, fm.UnionFacts[0].Var) == nil {
		t.Fatal("must keep non-loop-local union subject", outU)
	}
}
