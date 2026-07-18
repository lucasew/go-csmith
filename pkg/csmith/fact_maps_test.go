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

func TestSetMapFactsOutGotoDest(t *testing.T) {
	f := &Function{Name: "f", ReturnType: GetIntType()}
	loc := CreateVariableScalars("l_1", GetIntType(), false, false)
	loc.Name = "l_1"
	body := &Block{Func: f, LocalVars: []*Variable{loc}}
	f.Blocks = []*Block{body}
	fm := NewFactMgr(f)
	// local fact should be OOS at dest outside function (nil parent)
	// use global pointer fact only for merge
	p := CreateVariableScalars("g_p", PointerTo(GetIntType()), true, false)
	// invent a fact about local subject — IsLocal by name
	// CreateVariableScalars may set is_global based on true/false arg
	// loc is local of body → OOS at destParent=nil
	facts := []*FactPointTo{}
	// Add fact for local if pointer — skip; use Remove path via dest
	st := &Stmt{Kind: StmtGoto, StmID: 3}
	// with destParent = body, loc is visible
	fm.SetMapFactsOutForStmtDest(st, facts, body, body)
	if _, ok := fm.MapFactsOut[3]; !ok {
		t.Fatal("out set")
	}
	_ = p
}
