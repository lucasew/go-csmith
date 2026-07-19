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
	// nil fact hole fails closed — no invent partial dest update
	var out2 []*FactPointTo
	UpdateFactsForDest([]*FactPointTo{MakeFactPointTo(p, NullPtr), nil}, &out2, f, nil)
	if out2 != nil {
		t.Fatal("nil hole must fail closed dest facts", out2)
	}
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
	if fm.MapFactsInFinal[1] != nil {
		t.Fatal("incomplete MapFactsIn must not invent cleaned InFinal")
	}
	if fm.MapFactsOutFinal[2] != nil {
		t.Fatal("incomplete MapFactsOut must not invent cleaned OutFinal")
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

func TestBackupRestoreStmFactMaps(t *testing.T) {
	fm := NewFactMgr(nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	thenB := &Block{StmID: 20, Stmts: []Stmt{{StmID: 21}}}
	st := &Stmt{Kind: StmtIfElse, StmID: 10, Then: thenB}
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
	if in[15] != nil {
		t.Fatal("incomplete MapFactsIn must backup as nil, not invent cleaned")
	}
	if out[15] != nil {
		t.Fatal("incomplete MapFactsOut must backup as nil, not invent cleaned")
	}
	// restore incomplete backup → nil maps (not invent cleaned)
	fm.MapFactsIn[15] = []*FactPointTo{MakeFactPointTo(p, GarbagePtr)}
	fm.MapFactsOut[15] = []*FactPointTo{MakeFactPointTo(p, GarbagePtr)}
	in[15] = []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	out[15] = []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	fm.RestoreStmFactMaps(st, in, out)
	if fm.MapFactsIn[15] != nil || fm.MapFactsOut[15] != nil {
		t.Fatal("restore incomplete backup must fail closed nil")
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
	// nil fact hole fails closed at clone store and find_updated
	fm.SetMapFactsIn(1, []*FactPointTo{MakeFactPointTo(p, NullPtr)})
	fm.SetMapFactsOut(1, []*FactPointTo{MakeFactPointTo(p, GarbagePtr), nil})
	// CloneFactSlice drops incomplete store → MapFactsOut[1] nil
	if fm.MapFactsOut[1] != nil {
		t.Fatal("SetMapFactsOut must not invent cleaned list from nil hole")
	}
	if fm.FindUpdatedFacts(1) != nil {
		t.Fatal("nil out map must fail closed")
	}
	// direct find with hole in out (bypass Set)
	fm.MapFactsOut[1] = []*FactPointTo{MakeFactPointTo(p, GarbagePtr), nil}
	if fm.FindUpdatedFacts(1) != nil {
		t.Fatal("nil fact hole in out must fail closed")
	}
}

func TestRestoreFacts(t *testing.T) {
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
}

func TestMakeupNewVarFactsIncompleteFailClosed(t *testing.T) {
	// incomplete old/new maps must not invent partial makeup past holes
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	q := CreateVariableScalars("g_q", PointerTo(GetIntType()), true, false)
	old := []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}
	newF := []*FactPointTo{MakeFactPointTo(q, NullPtr)}
	MakeupNewVarFacts(&old, newF)
	if old != nil {
		t.Fatal("incomplete oldFacts must fail closed nil")
	}
	old2 := []*FactPointTo{MakeFactPointTo(p, NullPtr)}
	new2 := []*FactPointTo{MakeFactPointTo(q, NullPtr), nil}
	MakeupNewVarFacts(&old2, new2)
	if old2 != nil {
		t.Fatal("incomplete newFacts must fail closed nil oldFacts")
	}
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
	fm := NewFactMgr(nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	fm.SetMapFactsIn(1, []*FactPointTo{MakeFactPointTo(p, NullPtr), nil})
	if fm.MapFactsIn[1] != nil {
		t.Fatal("SetMapFactsIn incomplete must store nil")
	}
	fm.SetMapFactsOut(2, []*FactPointTo{MakeFactPointTo(p, GarbagePtr), nil})
	if fm.MapFactsOut[2] != nil {
		t.Fatal("SetMapFactsOut incomplete must store nil")
	}
}

func TestCollectLoopLocalVarsNilHoleFailClosed(t *testing.T) {
	// LocalVars nil hole fails closed (no invent skip partial OOS list)
	loop := &Block{Looping: true, LocalVars: []*Variable{
		CreateVariableScalars("l_1", GetIntType(), false, false),
		nil,
	}}
	if collectLoopLocalVars(loop) != nil {
		t.Fatal("nil LocalVars hole must fail closed")
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
	// incomplete facts on RemoveLoopLocalFacts
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	if RemoveLoopLocalFacts([]*FactPointTo{MakeFactPointTo(p, NullPtr), nil}, empty) != nil {
		t.Fatal("incomplete facts RemoveLoopLocalFacts must fail closed")
	}
}

func TestSetMapFactsOutForStmtIncompleteFailClosed(t *testing.T) {
	fm := NewFactMgr(nil)
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	st := &Stmt{Kind: StmtAssign, StmID: 5}
	fm.SetMapFactsOutForStmt(st, []*FactPointTo{MakeFactPointTo(p, NullPtr), nil}, nil)
	if fm.MapFactsOut[5] != nil {
		t.Fatal("incomplete set_fact_out must store nil")
	}
}
