package csmith

import "testing"

func TestUpdateFactsForDestDropsOOS(t *testing.T) {
	f := &Function{Name: "f", ReturnType: GetIntType()}
	outer := &Block{Func: f, LocalVars: []*Variable{
		CreateVariableScalars("l_1", GetIntType(), false, false),
	}}
	// mark local properly
	loc := outer.LocalVars[0]
	loc.Name = "l_1"
	// force local flag via name prefix used by IsLocal
	inner := &Block{Parent: outer, Func: f}
	f.Blocks = []*Block{outer, inner}
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	// fact: g_p points to local l_1 which is OOS at dest with nil parent (outside func stack)
	// destParent = nil means only globals visible → local is OOS
	in := []*FactPointTo{MakeFactPointToSet(p, []*Variable{loc})}
	// also a fact about the local itself
	// locals that are subjects get dropped as OOS
	var out []*FactPointTo
	UpdateFactsForDest(in, &out, f, nil)
	// p fact should remain but pointee marked dead/garbage
	if len(out) == 0 {
		t.Fatal("expected ptr fact kept")
	}
	// subject p is global → not OOS
	fp := FindRelatedPointTo(out, p)
	if fp == nil {
		t.Fatal("p gone")
	}
	// nil fact hole fails closed sticky — hole marker (not bare nil / empty complete)
	ClearError()
	var out2 []*FactPointTo
	UpdateFactsForDest([]*FactPointTo{MakeFactPointTo(p, NullPtr), nil}, &out2, f, nil)
	if FactsComplete(out2) {
		t.Fatal("nil hole must fail closed incomplete dest facts", out2)
	}
	if !HasError() {
		t.Fatal("nil hole UpdateFactsForDest must SetError sticky")
	}
	ClearError()
	// nil PointTo hole on live fact fails closed sticky
	var out3 []*FactPointTo
	bad := MakeFactPointTo(p, NullPtr)
	bad.PointTo = []*Variable{nil, loc}
	UpdateFactsForDest([]*FactPointTo{bad}, &out3, f, nil)
	if FactsComplete(out3) {
		t.Fatal("nil pointee hole must fail closed incomplete dest facts", out3)
	}
	if !HasError() {
		t.Fatal("nil pointee UpdateFactsForDest must SetError sticky")
	}
	ClearError()
	// missing function fails closed sticky incomplete (no invent empty dest)
	var out4 []*FactPointTo
	UpdateFactsForDest([]*FactPointTo{MakeFactPointTo(p, NullPtr)}, &out4, nil, nil)
	if FactsComplete(out4) {
		t.Fatal("nil func must fail closed incomplete dest facts", out4)
	}
	if !HasError() {
		t.Fatal("nil func UpdateFactsForDest must SetError sticky")
	}
	ClearError()
}

func TestClearMapVisited(t *testing.T) {
	fm := NewFactMgr(nil)
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
	fm := NewFactMgr(nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	f1 := MakeFactPointTo(p, NullPtr)
	fm.SetMapFactsIn(1, []*FactPointTo{f1})
	fm.SetMapFactsOut(1, []*FactPointTo{f1})
	fm.SetupInOutMaps(true)
	if len(fm.MapFactsInFinal[1]) != 1 {
		t.Fatal("first clone")
	}
	// second visit with wider fact
	f2 := MakeFactPointToSet(p, []*Variable{NullPtr, GarbagePtr})
	fm.SetMapFactsIn(1, []*FactPointTo{f2})
	fm.SetupInOutMaps(false)
	final := FindRelatedPointTo(fm.MapFactsInFinal[1], p)
	if final == nil || len(final.PointTo) < 2 {
		t.Fatal("combine", final)
	}
}

func TestSetupInOutMapsFirstTimeIncompleteFailClosed(t *testing.T) {
	// FactMgr.cpp:208–222 — first_time copy_facts; Fact* always live
	// incomplete hole must not invent cleaned final map entry
	fm := NewFactMgr(nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	// plant holes bypassing SetMapFacts* (CloneFactSlice strips holes)
	fm.MapFactsIn = map[int][]*FactPointTo{
		1: {MakeFactPointTo(p, NullPtr), nil},
	}
	fm.MapFactsOut = map[int][]*FactPointTo{
		2: {MakeFactPointTo(p, GarbagePtr), nil},
	}
	fm.SetupInOutMaps(true)
	if FactsComplete(fm.MapFactsInFinal[1]) {
		t.Fatal("incomplete MapFactsIn must not invent cleaned/complete InFinal")
	}
	if FactsComplete(fm.MapFactsOutFinal[2]) {
		t.Fatal("incomplete MapFactsOut must not invent cleaned/complete OutFinal")
	}
	// complete sibling still clones
	fm2 := NewFactMgr(nil)
	fm2.MapFactsIn = map[int][]*FactPointTo{
		3: {MakeFactPointTo(p, NullPtr)},
	}
	fm2.SetupInOutMaps(true)
	if len(fm2.MapFactsInFinal[3]) != 1 {
		t.Fatal("complete first_time must still clone")
	}
}

func TestSetupInOutMapsCombineIncompleteFailClosed(t *testing.T) {
	// second visit: incomplete current map must not invent join into final
	fm := NewFactMgr(nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	f1 := MakeFactPointTo(p, NullPtr)
	fm.SetMapFactsIn(1, []*FactPointTo{f1})
	fm.SetupInOutMaps(true)
	// plant incomplete current MapFactsIn
	fm.MapFactsIn[1] = []*FactPointTo{MakeFactPointToSet(p, []*Variable{NullPtr, GarbagePtr}), nil}
	fm.SetupInOutMaps(false)
	if FactsComplete(fm.MapFactsInFinal[1]) {
		t.Fatal("incomplete combine must fail closed incomplete final, not invent partial join")
	}
}

func TestBackupRestoreStmFactMaps(t *testing.T) {
	fm := NewFactMgr(nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	thenB := &Block{StmID: 20, Stmts: []Stmt{{StmID: 21}}}
	// StatementIf always has both arms
	st := &Stmt{Kind: StmtIfElse, StmID: 10, Then: thenB, Else: &Block{}}
	fm.SetMapFactsIn(10, []*FactPointTo{MakeFactPointTo(p, NullPtr)})
	fm.SetMapFactsOut(21, []*FactPointTo{MakeFactPointTo(p, GarbagePtr)})
	in := map[int][]*FactPointTo{}
	out := map[int][]*FactPointTo{}
	fm.BackupStmFactMaps(st, in, out)
	// mutate
	fm.SetMapFactsIn(10, nil)
	fm.SetMapFactsOut(21, nil)
	fm.RestoreStmFactMaps(st, in, out)
	if FindRelatedPointTo(fm.MapFactsIn[10], p) == nil {
		t.Fatal("restored in")
	}
	if FindRelatedPointTo(fm.MapFactsOut[21], p) == nil {
		t.Fatal("restored out")
	}
	// incomplete if — whole backup fail closed (no invent root-only complete tree)
	in2 := map[int][]*FactPointTo{}
	out2 := map[int][]*FactPointTo{}
	bad := &Stmt{Kind: StmtIfElse, StmID: 10, Then: thenB}
	fm.SetMapFactsIn(10, []*FactPointTo{MakeFactPointTo(p, NullPtr)})
	fm.SetMapFactsOut(21, []*FactPointTo{MakeFactPointTo(p, GarbagePtr)})
	fm.BackupStmFactMaps(bad, in2, out2)
	if _, ok := out2[21]; ok {
		t.Fatal("incomplete if must not invent nested backup past nil Else")
	}
	if FactsComplete(in2[10]) || FactsComplete(out2[10]) {
		t.Fatal("incomplete if must backup root as IncompleteFactSlice, not invent complete", in2[10], out2[10])
	}
}

func TestBackupStmFactMapsIncompleteFailClosed(t *testing.T) {
	// incomplete source maps must not invent cleaned backup clones
	fm := NewFactMgr(nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	st := &Stmt{Kind: StmtAssign, StmID: 15}
	fm.MapFactsIn = map[int][]*FactPointTo{
		15: {MakeFactPointTo(p, NullPtr), nil},
	}
	fm.MapFactsOut = map[int][]*FactPointTo{
		15: {MakeFactPointTo(p, GarbagePtr), nil},
	}
	in := map[int][]*FactPointTo{}
	out := map[int][]*FactPointTo{}
	fm.BackupStmFactMaps(st, in, out)
	if FactsComplete(in[15]) {
		t.Fatal("incomplete MapFactsIn must backup incomplete, not invent cleaned")
	}
	if FactsComplete(out[15]) {
		t.Fatal("incomplete MapFactsOut must backup incomplete, not invent cleaned")
	}
	// restore incomplete backup → incomplete maps (not invent cleaned)
	fm.MapFactsIn[15] = []*FactPointTo{MakeFactPointTo(p, GarbagePtr)}
	fm.MapFactsOut[15] = []*FactPointTo{MakeFactPointTo(p, GarbagePtr)}
	in[15] = []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	out[15] = []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	fm.RestoreStmFactMaps(st, in, out)
	if FactsComplete(fm.MapFactsIn[15]) || FactsComplete(fm.MapFactsOut[15]) {
		t.Fatal("restore incomplete backup must fail closed incomplete")
	}
}

func TestFindUpdatedFacts(t *testing.T) {
	fm := NewFactMgr(nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	fm.SetMapFactsIn(1, []*FactPointTo{MakeFactPointTo(p, NullPtr)})
	fm.SetMapFactsOut(1, []*FactPointTo{MakeFactPointTo(p, GarbagePtr)})
	u := fm.FindUpdatedFacts(1)
	if len(u) != 1 {
		t.Fatal(u)
	}
	// equal → no update
	fm.SetMapFactsOut(1, []*FactPointTo{MakeFactPointTo(p, NullPtr)})
	if len(fm.FindUpdatedFacts(1)) != 0 {
		t.Fatal("no change")
	}
	// FactMgr.cpp:660 assert(prev_f) — out-only fact without in match is not updated
	q := CreateVariableScalars("g_q", PointerTo(GetIntType()), true, false)
	fm.SetMapFactsOut(1, []*FactPointTo{MakeFactPointTo(q, NullPtr)})
	if len(fm.FindUpdatedFacts(1)) != 0 {
		t.Fatal("missing prev must fail closed, not invent as updated")
	}
	// nil fact hole fails closed at store and find_updated
	fm.SetMapFactsIn(1, []*FactPointTo{MakeFactPointTo(p, NullPtr)})
	fm.SetMapFactsOut(1, []*FactPointTo{MakeFactPointTo(p, GarbagePtr), nil})
	// incomplete store is hole marker (FactsComplete false), not cleaned list
	if FactsComplete(fm.MapFactsOut[1]) {
		t.Fatal("SetMapFactsOut must not invent cleaned/complete list from nil hole")
	}
	if FactsComplete(fm.FindUpdatedFacts(1)) {
		t.Fatal("incomplete out map must fail closed incomplete, not invent empty complete")
	}
	// direct find with hole in out (bypass Set)
	fm.MapFactsOut[1] = []*FactPointTo{MakeFactPointTo(p, GarbagePtr), nil}
	if FactsComplete(fm.FindUpdatedFacts(1)) {
		t.Fatal("nil fact hole in out must fail closed incomplete")
	}
	// final maps: incomplete → IncompleteFactSlice (not bare nil invent empty)
	fm.MapFactsInFinal = map[int][]*FactPointTo{1: {MakeFactPointTo(p, NullPtr)}}
	fm.MapFactsOutFinal = map[int][]*FactPointTo{1: {MakeFactPointTo(p, GarbagePtr), nil}}
	if FactsComplete(fm.FindUpdatedFinalFacts(1)) {
		t.Fatal("incomplete final out must fail closed incomplete")
	}
}

func TestRestoreFacts(t *testing.T) {
	ClearError()
	fm := NewFactMgr(nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	q := CreateVariableScalars("g_q", PointerTo(GetIntType()), true, false)
	old := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(p, GarbagePtr), MakeFactPointTo(q, NullPtr)}
	fm.RestoreFacts(old)
	// p restored to old; q added via makeup
	if FindRelatedPointTo(fm.GlobalFacts, p) == nil {
		t.Fatal("p")
	}
	if FindRelatedPointTo(fm.GlobalFacts, q) == nil {
		t.Fatal("makeup q")
	}
	// incomplete oldFacts fails closed sticky (no invent soft re-pick past wipe)
	hole := []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	fm.GlobalFacts = []*FactPointTo{MakeFactPointTo(q, NullPtr)}
	fm.RestoreFacts(hole)
	if FactsComplete(fm.GlobalFacts) {
		t.Fatal("incomplete oldFacts must wipe GlobalFacts incomplete")
	}
	if !HasError() {
		t.Fatal("incomplete oldFacts RestoreFacts must SetError sticky")
	}
	ClearError()
}

func TestMakeupNewVarFactsIncompleteFailClosed(t *testing.T) {
	// incomplete old/new maps must not invent partial makeup past holes — sticky
	ClearError()
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	q := CreateVariableScalars("g_q", PointerTo(GetIntType()), true, false)
	old := []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	newF := []*FactPointTo{MakeFactPointTo(q, NullPtr)}
	if MakeupNewVarFacts(&old, newF) {
		t.Fatal("incomplete oldFacts must fail closed false")
	}
	if FactsComplete(old) {
		t.Fatal("incomplete oldFacts must fail closed nil")
	}
	if !HasError() {
		t.Fatal("incomplete oldFacts must SetError sticky")
	}
	ClearError()
	old2 := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	new2 := []*FactPointTo{MakeFactPointTo(q, NullPtr), nil}
	if MakeupNewVarFacts(&old2, new2) {
		t.Fatal("incomplete newFacts must fail closed false")
	}
	if FactsComplete(old2) {
		t.Fatal("incomplete newFacts must fail closed incomplete oldFacts")
	}
	if !HasError() {
		t.Fatal("incomplete newFacts must SetError sticky")
	}
	ClearError()
}

func TestMakeupNewVarFactsAddNewHoleStopsLaterVars(t *testing.T) {
	// AddNewVarFactInto FieldVars hole clears *oldFacts; must not invent later vars
	ClearError()
	opts := Defaults()
	SetProcessOptions(opts)
	SetProcessProbabilities(NewProbabilities(opts))
	// non-pointer aggregate with nil field hole (global name)
	agg := CreateVariableScalars("g_agg", GetIntType(), true, false)
	if agg == nil {
		t.Fatal("agg")
	}
	agg.FieldVars = []*Variable{nil}
	later := CreateVariableScalars("g_later", PointerTo(GetIntType()), true, false)
	old := []*FactPointTo{}
	// both appear as new_facts subjects so makeup tries AddNewVarFactInto for each
	newF := []*FactPointTo{
		MakeFactPointTo(agg, NullPtr),
		MakeFactPointTo(later, NullPtr),
	}
	if MakeupNewVarFacts(&old, newF) {
		t.Fatal("FieldVars hole must fail closed false")
	}
	if FactsComplete(old) {
		t.Fatal("must not invent re-accumulate later pointer after hole", old)
	}
	if FindRelatedPointTo(old, later) != nil {
		t.Fatal("later var must not be made up past field hole")
	}
	ClearError()
}

func TestSetMapFactsOutGotoDest(t *testing.T) {
	// FactMgr.cpp:263–266 — update_facts_for_dest via StatementGoto::dest;
	// no soft invent RemoveFunctionLocalFacts when dest fields present.
	f := &Function{Name: "f", ReturnType: GetIntType()}
	// nested local not visible at body dest → OOS/garbage
	innerLoc := CreateVariableScalars("l_inner", PointerTo(GetIntType()), false, false)
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
	fm := NewFactMgr(f)
	g := CreateVariableScalars("g_p", PointerTo(GetIntType()), false, false)
	// global points to inner local; after goto to body, pointee is OOS → garbage
	facts := []*FactPointTo{
		MakeFactPointTo(g, innerLoc),
		MakeFactPointTo(innerLoc, NullPtr),
	}
	// SetMapFactsOutForStmt resolves GotoDestParent (no invent approx drop)
	fm.SetMapFactsOutForStmt(st, facts, inner)
	out := fm.MapFactsOut[3]
	if out == nil {
		t.Fatal("out set")
	}
	// global kept; subject innerLoc OOS at body dest → dropped from subjects
	if FindRelatedPointTo(out, g) == nil {
		t.Fatal("global lost", out)
	}
	// pointee of g marked dead (OOS local)
	gf := FindRelatedPointTo(out, g)
	if gf == nil || !gf.IsDead() {
		t.Fatalf("want g→garbage after OOS pointee, got %+v", gf)
	}
	// return uses s->parent stack walk, not invent f.Body-only
	ret := &Stmt{Kind: StmtReturn, StmID: 4}
	fm.SetMapFactsOutForStmt(ret, facts, inner)
	retOut := fm.MapFactsOut[4]
	// innerLoc on stack at return → subject dropped
	if FindRelatedPointTo(retOut, innerLoc) != nil {
		t.Fatal("return must drop stack local subject", retOut)
	}
}

func TestSetMapFactsIncompleteStoresNil(t *testing.T) {
	// incomplete facts must not invent cleaned MapFactsIn/Out entry
	// stored as hole marker (FactsComplete false), not bare nil (FactsComplete true)
	fm := NewFactMgr(nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	fm.SetMapFactsIn(1, []*FactPointTo{MakeFactPointTo(p, NullPtr), nil})
	if FactsComplete(fm.MapFactsIn[1]) {
		t.Fatal("SetMapFactsIn incomplete must not invent complete empty")
	}
	fm.SetMapFactsOut(2, []*FactPointTo{MakeFactPointTo(p, GarbagePtr), nil})
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
	// LocalVars nil hole fails closed (no invent skip partial OOS list)
	loop := &Block{Looping: true, LocalVars: []*Variable{
		CreateVariableScalars("l_1", GetIntType(), false, false),
		nil,
	}}
	if VariablesComplete(collectLoopLocalVars(loop)) {
		t.Fatal("nil LocalVars hole must fail closed incomplete")
	}
	// complete empty
	empty := &Block{Looping: true}
	got := collectLoopLocalVars(empty)
	if got == nil {
		t.Fatal("empty complete must be non-nil empty")
	}
	if len(got) != 0 {
		t.Fatal(got)
	}
	// incomplete facts on RemoveLoopLocalFacts sticky
	ClearError()
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	if FactsComplete(RemoveLoopLocalFacts([]*FactPointTo{MakeFactPointTo(p, NullPtr), nil}, empty)) {
		t.Fatal("incomplete facts RemoveLoopLocalFacts must fail closed")
	}
	if !HasError() {
		t.Fatal("incomplete facts RemoveLoopLocalFacts must SetError sticky")
	}
	ClearError()
}

func TestSetMapFactsOutForStmtIncompleteFailClosed(t *testing.T) {
	ClearError()
	fm := NewFactMgr(nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	st := &Stmt{Kind: StmtAssign, StmID: 5}
	fm.SetMapFactsOutForStmt(st, []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}, nil)
	if FactsComplete(fm.MapFactsOut[5]) {
		t.Fatal("incomplete set_fact_out must not invent complete empty")
	}
	if !HasError() {
		t.Fatal("incomplete set_fact_out must SetError sticky")
	}
	ClearError()
	// StmID 0 fails closed sticky (no invent silent set_fact_out)
	st0 := &Stmt{Kind: StmtAssign, StmID: 0}
	fm.SetMapFactsOutForStmt(st0, []*FactPointTo{MakeFactPointTo(p, NullPtr)}, nil)
	if !HasError() {
		t.Fatal("StmID 0 set_fact_out must SetError sticky")
	}
	ClearError()
}
